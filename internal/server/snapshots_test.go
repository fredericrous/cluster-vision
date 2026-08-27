package server

import (
	"context"
	"testing"
	"time"

	"github.com/fredericrous/cluster-vision/internal/diff"
	cvmetrics "github.com/fredericrous/cluster-vision/internal/metrics"
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// A partial parse must never reach the database. The server here has no
// db at all, so any attempt to persist would nil-deref — the test passing
// means the guard fired first.
func TestCaptureSnapshotSkipsPartialParse(t *testing.T) {
	s := &Server{}
	before := testutil.ToFloat64(cvmetrics.SnapshotsSkipped.WithLabelValues("partial_parse"))
	s.captureSnapshot(context.Background(), &model.ClusterData{}, nil, true)
	after := testutil.ToFloat64(cvmetrics.SnapshotsSkipped.WithLabelValues("partial_parse"))
	if after != before+1 {
		t.Fatalf("partial_parse counter = %v, want %v", after, before+1)
	}
}

func TestNormalizeGitURL(t *testing.T) {
	cases := map[string]string{
		"ssh://git@github.com/fredericrous/homelab.git": "https://github.com/fredericrous/homelab",
		"git@git.example.com:owner/repo.git":            "https://git.example.com/owner/repo",
		"https://github.com/owner/repo.git":             "https://github.com/owner/repo",
		"https://github.com/owner/repo":                 "https://github.com/owner/repo",
		"oci://ghcr.io/owner/repo":                      "",
	}
	for in, want := range cases {
		if got := normalizeGitURL(in); got != want {
			t.Errorf("normalizeGitURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompareLinks(t *testing.T) {
	data := &model.ClusterData{
		Flux:            []model.FluxKustomization{{Name: "flux-system", Cluster: "Homelab", SourceKind: "GitRepository", SourceName: "flux-system"}},
		GitRepositories: []model.GitRepositoryInfo{{Name: "flux-system", Cluster: "Homelab", URL: "ssh://git@github.com/o/r.git"}},
	}
	from := &side{snap: diff.Snapshot{Revisions: []diff.Revision{
		{Cluster: "Homelab", Kustomization: "flux-system", SourceKind: "GitRepository", SHA: "aaa111"},
		{Cluster: "NAS", Kustomization: "flux-system", SourceKind: "OCIRepository", SHA: "111"},
	}}}
	to := &side{data: data, snap: diff.Snapshot{TakenAt: time.Now(), Revisions: []diff.Revision{
		{Cluster: "Homelab", Kustomization: "flux-system", SourceKind: "GitRepository", SHA: "bbb222"},
		{Cluster: "NAS", Kustomization: "flux-system", SourceKind: "OCIRepository", SHA: "222"},
	}}}
	links := compareLinks(from, to)
	if len(links) != 1 || links[0].URL != "https://github.com/o/r/compare/aaa111...bbb222" {
		t.Fatalf("links = %+v", links)
	}
	// Same sha → no link.
	if got := compareLinks(to, to); len(got) != 0 {
		t.Fatalf("expected no links for identical revisions, got %+v", got)
	}
}

func TestBuildDiffFlagsDrift(t *testing.T) {
	s := &Server{}
	revs := []diff.Revision{{Cluster: "Homelab", Kustomization: "flux-system", Revision: "main@sha1:abc", SHA: "abc"}}
	from := &side{snap: diff.Snapshot{ID: "a", Revisions: revs}, diagrams: []model.DiagramResult{{ID: "topology", Type: "mermaid", Content: "x"}}}
	to := &side{snap: diff.Snapshot{ID: "b", Revisions: revs}, diagrams: []model.DiagramResult{{ID: "topology", Type: "mermaid", Content: "y"}}, data: &model.ClusterData{}}
	resp := s.buildDiff(from, to, "")
	if !resp.Drift || resp.Total.Changed != 1 || !resp.Diagrams[0].Drift {
		t.Fatalf("resp = %+v", resp)
	}
	if resp := s.buildDiff(from, to, "nope"); len(resp.Diagrams) != 0 {
		t.Fatalf("filter by id leaked diagrams: %+v", resp.Diagrams)
	}
}
