// Package diff compares two sets of generated diagrams and reports what
// changed between them. It is the engine behind cluster snapshots: both
// sides are always rendered by the current generators, so a change here is
// a change in the observed cluster, never a change in how we draw it.
//
// Identity and "advisory" fields are declared per diagram in specs.go.
// Observed fields participate in the content hash and in Changes; advisory
// fields (registry "latest" tags, vuln DB counts — anything derived from a
// source other than the cluster) are reported separately and never make a
// snapshot look different.
package diff

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// Revision is the desired-state revision a cluster was observed under, as
// reported by a root Flux Kustomization.
type Revision struct {
	Cluster       string `json:"cluster"`
	Kustomization string `json:"kustomization"`
	SourceKind    string `json:"source_kind"` // GitRepository | OCIRepository | Bucket
	Revision      string `json:"revision"`    // e.g. main@sha1:abc123…
	SHA           string `json:"sha"`         // digest part of Revision
}

// Snapshot identifies one side of a diff.
type Snapshot struct {
	ID        string     `json:"id"` // uuid, or "now" for the in-memory state
	TakenAt   time.Time  `json:"taken_at"`
	Revisions []Revision `json:"revisions"`
}

// Change is one added / removed / changed element.
type Change struct {
	Kind   string        `json:"kind"` // node | edge | row | content
	Op     string        `json:"op"`   // added | removed | changed
	ID     string        `json:"id"`
	Label  string        `json:"label"`
	Fields []FieldChange `json:"fields,omitempty"`
}

// FieldChange is one field of a changed element.
type FieldChange struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
}

// Summary counts changes by operation.
type Summary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}

// Total returns the number of changes in the summary.
func (s Summary) Total() int { return s.Added + s.Removed + s.Changed }

// DiagramDiff is the delta of one diagram between two snapshots.
type DiagramDiff struct {
	DiagramID string   `json:"diagram_id"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	KeyFields []string `json:"key_fields,omitempty"` // table identity, for the UI to key rows
	From      Snapshot `json:"from"`
	To        Snapshot `json:"to"`
	Changes   []Change `json:"changes"`
	Advisory  []Change `json:"advisory"` // never counted in Summary or Drift
	Summary   Summary  `json:"summary"`
	// Drift is true when the revisions on both sides are identical but the
	// observed content differs: something moved outside GitOps.
	Drift bool `json:"drift"`
}

// Diagram diffs one diagram. From/To/Drift are left for the caller to
// fill in — the differ knows nothing about snapshots.
func Diagram(a, b model.DiagramResult) DiagramDiff {
	d := DiagramDiff{DiagramID: b.ID, Title: b.Title, Type: b.Type, Changes: []Change{}, Advisory: []Change{}}
	if d.DiagramID == "" {
		d.DiagramID, d.Title, d.Type = a.ID, a.Title, a.Type
	}

	switch {
	case a.Type == "flow" || b.Type == "flow":
		d.Type = "flow"
		d.Changes = diffFlow(a.Content, b.Content)
	case a.Type == "table" || b.Type == "table":
		d.Type = "table"
		spec := specFor(d.DiagramID)
		d.KeyFields = spec.Keys
		d.Changes, d.Advisory = diffTable(tableRows(a), tableRows(b), spec)
	default:
		if a.Content != b.Content {
			d.Changes = []Change{{Kind: "content", Op: "changed", ID: d.DiagramID, Label: d.Title}}
		}
	}

	for _, c := range d.Changes {
		switch c.Op {
		case "added":
			d.Summary.Added++
		case "removed":
			d.Summary.Removed++
		default:
			d.Summary.Changed++
		}
	}
	return d
}

// All diffs two diagram sets, pairing by diagram ID. A diagram present on
// one side only is diffed against the zero value (everything added or
// removed).
func All(a, b []model.DiagramResult) []DiagramDiff {
	byID := func(ds []model.DiagramResult) map[string]model.DiagramResult {
		m := make(map[string]model.DiagramResult, len(ds))
		for _, d := range ds {
			m[d.ID] = d
		}
		return m
	}
	am, bm := byID(a), byID(b)
	ids := make([]string, 0, len(bm))
	for id := range bm {
		ids = append(ids, id)
	}
	for id := range am {
		if _, ok := bm[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	out := make([]DiagramDiff, 0, len(ids))
	for _, id := range ids {
		out = append(out, Diagram(am[id], bm[id]))
	}
	return out
}

// SameRevisions reports whether two revision sets are identical.
func SameRevisions(a, b []Revision) bool {
	if len(a) != len(b) {
		return false
	}
	key := func(r Revision) string { return r.Cluster + "\x00" + r.Kustomization + "\x00" + r.Revision }
	seen := make(map[string]bool, len(a))
	for _, r := range a {
		seen[key(r)] = true
	}
	for _, r := range b {
		if !seen[key(r)] {
			return false
		}
	}
	return true
}

// ObservedHash hashes the observed content of a diagram set plus the
// revisions it was observed under. Two refreshes with the same hash are
// the same snapshot; the hash ignores advisory fields so registry lookups
// and vuln DB updates never produce a new snapshot.
func ObservedHash(diagrams []model.DiagramResult, revs []Revision) []byte {
	h := sha256.New()

	sorted := append([]model.DiagramResult(nil), diagrams...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, d := range sorted {
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00", d.ID, d.Type)
		if d.Type == "table" {
			spec := specFor(d.ID)
			for _, row := range tableRows(d) {
				_, _ = fmt.Fprintf(h, "%s\n", canonicalRow(row, spec.Advisory))
			}
		} else {
			_, _ = h.Write([]byte(d.Content))
		}
		_, _ = h.Write([]byte{0})
	}

	rs := append([]Revision(nil), revs...)
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Cluster != rs[j].Cluster {
			return rs[i].Cluster < rs[j].Cluster
		}
		return rs[i].Kustomization < rs[j].Kustomization
	})
	for _, r := range rs {
		_, _ = fmt.Fprintf(h, "rev\x00%s\x00%s\x00%s\n", r.Cluster, r.Kustomization, r.Revision)
	}
	return h.Sum(nil)
}

// RevisionsOf extracts the desired-state revisions from cluster data: one
// per root Flux Kustomization (no dependsOn, backed by a Git/OCI source).
// Clusters without Flux contribute nothing.
func RevisionsOf(data *model.ClusterData) []Revision {
	if data == nil {
		return nil
	}
	var out []Revision
	seen := map[string]bool{}
	for _, k := range data.Flux {
		if k.LastAppliedRevision == "" || len(k.DependsOn) > 0 {
			continue
		}
		switch k.SourceKind {
		case "GitRepository", "OCIRepository", "Bucket":
		default:
			continue
		}
		key := k.Cluster + "/" + k.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Revision{
			Cluster:       k.Cluster,
			Kustomization: k.Name,
			SourceKind:    k.SourceKind,
			Revision:      k.LastAppliedRevision,
			SHA:           ParseSHA(k.LastAppliedRevision),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cluster != out[j].Cluster {
			return out[i].Cluster < out[j].Cluster
		}
		return out[i].Kustomization < out[j].Kustomization
	})
	return out
}

// ParseSHA extracts the digest from a Flux revision string:
// "main@sha1:abc123" → "abc123", "latest@sha256:def" → "def",
// legacy "main/abc123" → "abc123". Unknown shapes return the input.
func ParseSHA(rev string) string {
	if i := strings.LastIndex(rev, ":"); i >= 0 {
		return rev[i+1:]
	}
	if i := strings.LastIndex(rev, "/"); i >= 0 {
		return rev[i+1:]
	}
	return rev
}

// ---- flow ----

type flowNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Cluster string `json:"cluster"`
	Layer   string `json:"layer"`
}

type flowEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	CrossCluster bool   `json:"crossCluster"`
	Label        string `json:"label"`
}

type flowData struct {
	Nodes []flowNode `json:"nodes"`
	Edges []flowEdge `json:"edges"`
}

func parseFlow(content string) flowData {
	var f flowData
	if content != "" {
		_ = json.Unmarshal([]byte(content), &f)
	}
	return f
}

func diffFlow(aContent, bContent string) []Change {
	a, b := parseFlow(aContent), parseFlow(bContent)
	var out []Change

	an := map[string]flowNode{}
	for _, n := range a.Nodes {
		an[n.ID] = n
	}
	bn := map[string]flowNode{}
	for _, n := range b.Nodes {
		bn[n.ID] = n
	}
	for _, n := range b.Nodes {
		old, ok := an[n.ID]
		if !ok {
			out = append(out, Change{Kind: "node", Op: "added", ID: n.ID, Label: n.Label})
			continue
		}
		var fields []FieldChange
		if old.Label != n.Label {
			fields = append(fields, FieldChange{"label", old.Label, n.Label})
		}
		if old.Cluster != n.Cluster {
			fields = append(fields, FieldChange{"cluster", old.Cluster, n.Cluster})
		}
		if old.Layer != n.Layer {
			fields = append(fields, FieldChange{"layer", old.Layer, n.Layer})
		}
		if len(fields) > 0 {
			out = append(out, Change{Kind: "node", Op: "changed", ID: n.ID, Label: n.Label, Fields: fields})
		}
	}
	for _, n := range a.Nodes {
		if _, ok := bn[n.ID]; !ok {
			out = append(out, Change{Kind: "node", Op: "removed", ID: n.ID, Label: n.Label})
		}
	}

	edgeLabel := func(e flowEdge) string {
		if e.Label != "" {
			return e.Source + " → " + e.Target + " (" + e.Label + ")"
		}
		return e.Source + " → " + e.Target
	}
	ae := map[string]flowEdge{}
	for _, e := range a.Edges {
		ae[e.ID] = e
	}
	be := map[string]flowEdge{}
	for _, e := range b.Edges {
		be[e.ID] = e
	}
	for _, e := range b.Edges {
		old, ok := ae[e.ID]
		if !ok {
			out = append(out, Change{Kind: "edge", Op: "added", ID: e.ID, Label: edgeLabel(e)})
			continue
		}
		var fields []FieldChange
		if old.Source != e.Source {
			fields = append(fields, FieldChange{"source", old.Source, e.Source})
		}
		if old.Target != e.Target {
			fields = append(fields, FieldChange{"target", old.Target, e.Target})
		}
		if old.CrossCluster != e.CrossCluster {
			fields = append(fields, FieldChange{"crossCluster", fmt.Sprint(old.CrossCluster), fmt.Sprint(e.CrossCluster)})
		}
		if old.Label != e.Label {
			fields = append(fields, FieldChange{"label", old.Label, e.Label})
		}
		if len(fields) > 0 {
			out = append(out, Change{Kind: "edge", Op: "changed", ID: e.ID, Label: edgeLabel(e), Fields: fields})
		}
	}
	for _, e := range a.Edges {
		if _, ok := be[e.ID]; !ok {
			out = append(out, Change{Kind: "edge", Op: "removed", ID: e.ID, Label: edgeLabel(e)})
		}
	}
	return out
}

// ---- table ----

type row = map[string]any

func tableRows(d model.DiagramResult) []row {
	if d.Type != "table" || d.Content == "" {
		return nil
	}
	var rows []row
	if err := json.Unmarshal([]byte(d.Content), &rows); err != nil {
		return nil
	}
	return rows
}

func fieldString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// RowKey builds the identity of a row from the spec's key fields. With no
// key fields the whole observed row is the key, so such rows can only be
// added or removed, never "changed".
func RowKey(r row, spec tableSpec) string {
	if len(spec.Keys) == 0 {
		return canonicalRow(r, spec.Advisory)
	}
	parts := make([]string, len(spec.Keys))
	for i, k := range spec.Keys {
		parts[i] = fieldString(r[k])
	}
	return strings.Join(parts, "/")
}

func canonicalRow(r row, advisory []string) string {
	skip := make(map[string]bool, len(advisory))
	for _, a := range advisory {
		skip[a] = true
	}
	keys := make([]string, 0, len(r))
	for k := range r {
		if !skip[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(fieldString(r[k]))
		sb.WriteByte('\x1f')
	}
	return sb.String()
}

func rowLabel(r row, spec tableSpec) string {
	fields := spec.Label
	if len(fields) == 0 {
		fields = spec.Keys
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if v := fieldString(r[f]); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " / ")
}

func diffTable(a, b []row, spec tableSpec) (changes, advisory []Change) {
	advisorySet := make(map[string]bool, len(spec.Advisory))
	for _, f := range spec.Advisory {
		advisorySet[f] = true
	}

	am := make(map[string]row, len(a))
	for _, r := range a {
		am[RowKey(r, spec)] = r
	}
	bm := make(map[string]row, len(b))
	for _, r := range b {
		bm[RowKey(r, spec)] = r
	}

	for _, r := range b {
		key := RowKey(r, spec)
		old, ok := am[key]
		if !ok {
			changes = append(changes, Change{Kind: "row", Op: "added", ID: key, Label: rowLabel(r, spec)})
			continue
		}
		names := map[string]bool{}
		for k := range old {
			names[k] = true
		}
		for k := range r {
			names[k] = true
		}
		sortedNames := make([]string, 0, len(names))
		for k := range names {
			sortedNames = append(sortedNames, k)
		}
		sort.Strings(sortedNames)

		var obs, adv []FieldChange
		for _, k := range sortedNames {
			from, to := fieldString(old[k]), fieldString(r[k])
			if from == to {
				continue
			}
			fc := FieldChange{Name: k, From: from, To: to}
			if advisorySet[k] {
				adv = append(adv, fc)
			} else {
				obs = append(obs, fc)
			}
		}
		if len(obs) > 0 {
			changes = append(changes, Change{Kind: "row", Op: "changed", ID: key, Label: rowLabel(r, spec), Fields: obs})
		}
		if len(adv) > 0 {
			advisory = append(advisory, Change{Kind: "row", Op: "changed", ID: key, Label: rowLabel(r, spec), Fields: adv})
		}
	}
	for _, r := range a {
		key := RowKey(r, spec)
		if _, ok := bm[key]; !ok {
			changes = append(changes, Change{Kind: "row", Op: "removed", ID: key, Label: rowLabel(r, spec)})
		}
	}
	return changes, advisory
}
