package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fredericrous/cluster-vision/internal/diff"
	cvmetrics "github.com/fredericrous/cluster-vision/internal/metrics"
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

// captureSnapshot persists the observed cluster state if it differs from
// the latest snapshot. Runs in its own goroutine after every refresh.
func (s *Server) captureSnapshot(parent context.Context, data *model.ClusterData, diagrams []model.DiagramResult, partial bool) {
	if partial {
		cvmetrics.SnapshotsSkipped.WithLabelValues("partial_parse").Inc()
		slog.Warn("snapshot skipped: partial parse")
		return
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	revs := diff.RevisionsOf(data)
	hash := diff.ObservedHash(diagrams, revs)

	prev, err := s.db.LatestSnapshot(ctx)
	if err != nil && !errors.Is(err, store.ErrNoSnapshot) {
		cvmetrics.SnapshotsSkipped.WithLabelValues("error").Inc()
		slog.Error("snapshot: loading latest failed", "error", err)
		return
	}
	if prev != nil && bytes.Equal(prev.ObservedHash, hash) {
		cvmetrics.SnapshotsSkipped.WithLabelValues("unchanged").Inc()
		return
	}

	snap := &store.Snapshot{
		TakenAt:      time.Now(),
		ObservedHash: hash,
		Revisions:    revs,
		Summary:      store.SnapshotSummary{Diagrams: map[string]diff.Summary{}},
	}

	if prev != nil {
		prevData, err := s.db.GetSnapshotData(ctx, prev.ID)
		if err != nil {
			slog.Warn("snapshot: previous data unreadable, summary will be empty", "id", prev.ID, "error", err)
		} else {
			id := prev.ID
			snap.Summary.PreviousID = &id
			for _, d := range diff.All(s.generate(prevData), diagrams) {
				if d.Summary.Total() == 0 {
					continue
				}
				snap.Summary.Diagrams[d.DiagramID] = d.Summary
				snap.Summary.Total.Added += d.Summary.Added
				snap.Summary.Total.Removed += d.Summary.Removed
				snap.Summary.Total.Changed += d.Summary.Changed
			}
			snap.Summary.Drift = diff.SameRevisions(prev.Revisions, revs) && snap.Summary.Total.Total() > 0
		}
	}

	if err := s.db.InsertSnapshot(ctx, snap, data); err != nil {
		cvmetrics.SnapshotsSkipped.WithLabelValues("error").Inc()
		slog.Error("snapshot: insert failed", "error", err)
		return
	}
	cvmetrics.SnapshotsTotal.Inc()
	if snap.Summary.Drift {
		cvmetrics.SnapshotDrift.Set(1)
	} else {
		cvmetrics.SnapshotDrift.Set(0)
	}
	slog.Info("snapshot written", "id", snap.ID, "changes", snap.Summary.Total.Total(), "drift", snap.Summary.Drift, "revisions", len(revs))
}

// snapshotRetentionLoop prunes old snapshots once a day.
func (s *Server) snapshotRetentionLoop(ctx context.Context) {
	full, daily := s.cfg.SnapshotFullRetention, s.cfg.SnapshotDailyRetention
	if full <= 0 {
		full = 7 * 24 * time.Hour
	}
	if daily < full {
		daily = 90 * 24 * time.Hour
	}
	prune := func() {
		pctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		n, err := s.db.PruneSnapshots(pctx, full, daily)
		if err != nil {
			slog.Warn("snapshot retention failed", "error", err)
			return
		}
		if n > 0 {
			slog.Info("snapshot retention", "deleted", n)
		}
	}
	// First pass shortly after boot, then daily.
	select {
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Minute):
		prune()
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}

// ---- selectors ----

// side is one resolved end of a comparison.
type side struct {
	snap     diff.Snapshot
	data     *model.ClusterData
	diagrams []model.DiagramResult
	hash     []byte
}

var shaRe = regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`)

type selectorError struct {
	selector string
	reason   string
}

func (e *selectorError) Error() string { return fmt.Sprintf("selector %q: %s", e.selector, e.reason) }

// current returns the in-memory state as a side.
func (s *Server) current() (*side, error) {
	s.mu.RLock()
	data, diagrams, gen, partial := s.clusterData, s.data, s.lastGen, s.lastPartial
	s.mu.RUnlock()
	if data == nil {
		return nil, &selectorError{"now", "no cluster data yet"}
	}
	revs := diff.RevisionsOf(data)
	sd := &side{
		snap:     diff.Snapshot{ID: "now", TakenAt: gen, Revisions: revs},
		data:     data,
		diagrams: diagrams,
		hash:     diff.ObservedHash(diagrams, revs),
	}
	_ = partial
	return sd, nil
}

func (s *Server) sideFromSnapshot(ctx context.Context, snap *store.Snapshot) (*side, error) {
	data, err := s.db.GetSnapshotData(ctx, snap.ID)
	if err != nil {
		return nil, err
	}
	return &side{
		snap:     diff.Snapshot{ID: snap.ID.String(), TakenAt: snap.TakenAt, Revisions: snap.Revisions},
		data:     data,
		diagrams: s.generate(data),
		hash:     snap.ObservedHash,
	}, nil
}

// resolveTo resolves the "after" selector: now (default) | uuid | sha |
// RFC3339 time.
func (s *Server) resolveTo(ctx context.Context, sel string) (*side, error) {
	sel = strings.TrimSpace(sel)
	if sel == "" || sel == "now" || sel == "latest" {
		return s.current()
	}
	snap, err := s.lookup(ctx, sel)
	if err != nil {
		return nil, err
	}
	return s.sideFromSnapshot(ctx, snap)
}

// resolveFrom resolves the "before" selector. Besides the absolute forms
// it accepts the relative "prev" (the snapshot before `to`, the default)
// and "deploy" (the last snapshot observed under the previous revision
// set — "since last deploy").
func (s *Server) resolveFrom(ctx context.Context, sel string, to *side) (*side, error) {
	sel = strings.TrimSpace(sel)
	switch sel {
	case "", "prev", "previous":
		var snap *store.Snapshot
		var err error
		if to.snap.ID == "now" {
			// The latest stored snapshot is usually the current state
			// itself (dedup); "prev" means the one before that.
			snap, err = s.db.LatestSnapshot(ctx)
			if err == nil && bytes.Equal(snap.ObservedHash, to.hash) {
				snap, err = s.db.SnapshotBefore(ctx, snap.TakenAt)
			}
		} else {
			snap, err = s.db.SnapshotBefore(ctx, to.snap.TakenAt)
		}
		if err != nil {
			if errors.Is(err, store.ErrNoSnapshot) {
				return nil, &selectorError{"prev", "no earlier snapshot — change history starts now"}
			}
			return nil, err
		}
		return s.sideFromSnapshot(ctx, snap)
	case "deploy", "since-deploy":
		anchor := &store.Snapshot{TakenAt: to.snap.TakenAt, Revisions: to.snap.Revisions}
		if to.snap.ID == "now" {
			anchor.TakenAt = time.Now()
		}
		snap, err := s.db.SnapshotBeforeRevisionChange(ctx, anchor)
		if err != nil {
			if errors.Is(err, store.ErrNoSnapshot) {
				return nil, &selectorError{"deploy", "no snapshot from a previous revision on record"}
			}
			return nil, err
		}
		return s.sideFromSnapshot(ctx, snap)
	case "now", "latest":
		return s.current()
	}
	snap, err := s.lookup(ctx, sel)
	if err != nil {
		return nil, err
	}
	return s.sideFromSnapshot(ctx, snap)
}

// lookup resolves an absolute selector to a stored snapshot.
func (s *Server) lookup(ctx context.Context, sel string) (*store.Snapshot, error) {
	if id, err := uuid.Parse(sel); err == nil {
		snap, err := s.db.GetSnapshot(ctx, id)
		if errors.Is(err, store.ErrNoSnapshot) {
			return nil, &selectorError{sel, "no snapshot with that id"}
		}
		return snap, err
	}
	if t, err := time.Parse(time.RFC3339, sel); err == nil {
		snap, err := s.db.SnapshotAt(ctx, t)
		if errors.Is(err, store.ErrNoSnapshot) {
			return nil, &selectorError{sel, "no snapshot at or before that time"}
		}
		return snap, err
	}
	if shaRe.MatchString(sel) {
		snap, err := s.db.SnapshotBySHA(ctx, strings.ToLower(sel))
		if errors.Is(err, store.ErrNoSnapshot) {
			return nil, &selectorError{sel, "no snapshot observed at a revision with that sha"}
		}
		return snap, err
	}
	return nil, &selectorError{sel, "expected a snapshot id, a git sha, an RFC3339 time, now, prev or deploy"}
}

// ---- compare links ----

// CompareLink points at the forge's compare view for one cluster's
// revision change.
type CompareLink struct {
	Cluster string `json:"cluster"`
	FromSHA string `json:"from_sha"`
	ToSHA   string `json:"to_sha"`
	URL     string `json:"url"`
}

func compareLinks(from, to *side) []CompareLink {
	links := []CompareLink{}
	if to.data == nil {
		return links
	}
	fromRev := map[string]diff.Revision{}
	for _, r := range from.snap.Revisions {
		fromRev[r.Cluster+"/"+r.Kustomization] = r
	}
	for _, r := range to.snap.Revisions {
		if r.SourceKind != "GitRepository" {
			continue
		}
		prev, ok := fromRev[r.Cluster+"/"+r.Kustomization]
		if !ok || prev.SHA == r.SHA || prev.SourceKind != "GitRepository" {
			continue
		}
		base := gitRepoWebURL(to.data, r.Cluster, r.Kustomization)
		if base == "" {
			continue
		}
		links = append(links, CompareLink{
			Cluster: r.Cluster,
			FromSHA: prev.SHA,
			ToSHA:   r.SHA,
			URL:     fmt.Sprintf("%s/compare/%s...%s", base, prev.SHA, r.SHA),
		})
	}
	return links
}

// gitRepoWebURL finds the GitRepository a root kustomization reconciles
// from and normalizes its URL to https://host/owner/repo.
func gitRepoWebURL(data *model.ClusterData, cluster, kustomization string) string {
	sourceName := ""
	for _, k := range data.Flux {
		if k.Cluster == cluster && k.Name == kustomization {
			sourceName = k.SourceName
			break
		}
	}
	for _, g := range data.GitRepositories {
		if g.Cluster != cluster {
			continue
		}
		if sourceName != "" && g.Name != sourceName {
			continue
		}
		return normalizeGitURL(g.URL)
	}
	return ""
}

func normalizeGitURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, ".git")
	switch {
	case strings.HasPrefix(u, "ssh://"):
		u = strings.TrimPrefix(u, "ssh://")
		if i := strings.Index(u, "@"); i >= 0 {
			u = u[i+1:]
		}
		return "https://" + u
	case strings.HasPrefix(u, "git@"):
		u = strings.TrimPrefix(u, "git@")
		return "https://" + strings.Replace(u, ":", "/", 1)
	case strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "https://"):
		return u
	}
	return ""
}

// ---- handlers ----

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var se *selectorError
	code := http.StatusInternalServerError
	if errors.As(err, &se) || errors.Is(err, store.ErrNoSnapshot) {
		code = http.StatusNotFound
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// handleListSnapshots — GET /api/snapshots?from=&to=&limit=
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeErr(w, &selectorError{v, "from must be RFC3339"})
			return
		}
		from = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeErr(w, &selectorError{v, "to must be RFC3339"})
			return
		}
		to = &t
	}
	limit, _ := strconv.Atoi(q.Get("limit"))

	snaps, err := s.db.ListSnapshots(r.Context(), from, to, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Mark the first snapshot at each revision set so the UI can flag
	// "deploy" points. Computed over the page only — good enough for a
	// picker; the deploy selector uses the DB for the real answer.
	type item struct {
		store.Snapshot
		NewRevision bool `json:"new_revision"`
	}
	items := make([]item, len(snaps))
	for i := range snaps {
		items[i] = item{Snapshot: snaps[i]}
		if i+1 < len(snaps) {
			items[i].NewRevision = !diff.SameRevisions(snaps[i].Revisions, snaps[i+1].Revisions)
		}
	}
	writeJSON(w, map[string]any{"snapshots": items})
}

// handleSnapshotDiagrams — GET /api/snapshots/{id}/diagrams
// Same shape as /api/diagrams, regenerated from the stored model.
func (s *Server) handleSnapshotDiagrams(w http.ResponseWriter, r *http.Request) {
	sd, err := s.resolveTo(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, struct {
		Diagrams    []model.DiagramResult `json:"diagrams"`
		GeneratedAt time.Time             `json:"generated_at"`
		Snapshot    diff.Snapshot         `json:"snapshot"`
	}{sd.diagrams, sd.snap.TakenAt, sd.snap})
}

// DiffResponse is the cluster-wide comparison.
type DiffResponse struct {
	From         diff.Snapshot      `json:"from"`
	To           diff.Snapshot      `json:"to"`
	Diagrams     []diff.DiagramDiff `json:"diagrams"`
	Total        diff.Summary       `json:"total"`
	Drift        bool               `json:"drift"`
	CompareLinks []CompareLink      `json:"compare_links"`
}

func (s *Server) resolvePair(r *http.Request) (*side, *side, error) {
	q := r.URL.Query()
	to, err := s.resolveTo(r.Context(), q.Get("to"))
	if err != nil {
		return nil, nil, err
	}
	from, err := s.resolveFrom(r.Context(), q.Get("from"), to)
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}

func (s *Server) buildDiff(from, to *side, only string) DiffResponse {
	resp := DiffResponse{From: from.snap, To: to.snap, Diagrams: []diff.DiagramDiff{}, CompareLinks: compareLinks(from, to)}
	same := diff.SameRevisions(from.snap.Revisions, to.snap.Revisions)
	for _, d := range diff.All(from.diagrams, to.diagrams) {
		if only != "" && d.DiagramID != only {
			continue
		}
		d.From, d.To = from.snap, to.snap
		d.Drift = same && d.Summary.Total() > 0
		resp.Diagrams = append(resp.Diagrams, d)
		resp.Total.Added += d.Summary.Added
		resp.Total.Removed += d.Summary.Removed
		resp.Total.Changed += d.Summary.Changed
	}
	resp.Drift = same && resp.Total.Total() > 0
	return resp
}

// handleDiffAll — GET /api/diff?from=&to=
func (s *Server) handleDiffAll(w http.ResponseWriter, r *http.Request) {
	from, to, err := s.resolvePair(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, s.buildDiff(from, to, ""))
}

// handleDiffDiagram — GET /api/diagrams/{id}/diff?from=&to=
func (s *Server) handleDiffDiagram(w http.ResponseWriter, r *http.Request) {
	from, to, err := s.resolvePair(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	resp := s.buildDiff(from, to, id)
	if len(resp.Diagrams) == 0 {
		writeErr(w, &selectorError{id, "no such diagram"})
		return
	}
	d := resp.Diagrams[0]
	writeJSON(w, struct {
		diff.DiagramDiff
		CompareLinks []CompareLink `json:"compare_links"`
	}{d, resp.CompareLinks})
}
