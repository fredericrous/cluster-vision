package diff

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
)

func table(id string, rows ...map[string]any) model.DiagramResult {
	b, _ := json.Marshal(rows)
	return model.DiagramResult{ID: id, Title: id, Type: "table", Content: string(b)}
}

func flow(nodes []flowNode, edges []flowEdge) model.DiagramResult {
	b, _ := json.Marshal(flowData{Nodes: nodes, Edges: edges})
	return model.DiagramResult{ID: "dependencies", Title: "Dependencies", Type: "flow", Content: string(b)}
}

func TestDiffTable_AddRemoveChange(t *testing.T) {
	a := table("certificates",
		map[string]any{"cluster": "Homelab", "namespace": "a", "name": "one", "issuer": "x", "ready": "yes"},
		map[string]any{"cluster": "Homelab", "namespace": "a", "name": "gone", "issuer": "x", "ready": "yes"},
	)
	b := table("certificates",
		map[string]any{"cluster": "Homelab", "namespace": "a", "name": "one", "issuer": "y", "ready": "yes"},
		map[string]any{"cluster": "Homelab", "namespace": "a", "name": "new", "issuer": "x", "ready": "no"},
	)
	d := Diagram(a, b)
	if d.Summary != (Summary{Added: 1, Removed: 1, Changed: 1}) {
		t.Fatalf("summary = %+v", d.Summary)
	}
	if got := d.KeyFields; len(got) != 3 || got[2] != "name" {
		t.Fatalf("key fields = %v", got)
	}
	var changed *Change
	for i := range d.Changes {
		if d.Changes[i].Op == "changed" {
			changed = &d.Changes[i]
		}
	}
	if changed == nil || changed.ID != "Homelab/a/one" || len(changed.Fields) != 1 || changed.Fields[0] != (FieldChange{"issuer", "x", "y"}) {
		t.Fatalf("changed = %+v", changed)
	}
	if changed.Label != "a / one" {
		t.Fatalf("label = %q", changed.Label)
	}
}

func TestDiffTable_AdvisoryNotCounted(t *testing.T) {
	a := table("images", map[string]any{"image": "nginx", "tag": "1.0", "type": "app", "pods": 2, "latest": "1.0", "outdated": false})
	b := table("images", map[string]any{"image": "nginx", "tag": "1.0", "type": "app", "pods": 2, "latest": "1.1", "outdated": true})
	d := Diagram(a, b)
	if d.Summary.Total() != 0 {
		t.Fatalf("advisory change counted: %+v", d.Changes)
	}
	if len(d.Advisory) != 1 || len(d.Advisory[0].Fields) != 2 {
		t.Fatalf("advisory = %+v", d.Advisory)
	}
	if !bytes.Equal(ObservedHash([]model.DiagramResult{a}, nil), ObservedHash([]model.DiagramResult{b}, nil)) {
		t.Fatal("advisory field changed the observed hash")
	}
	c := table("images", map[string]any{"image": "nginx", "tag": "1.0", "type": "app", "pods": 3, "latest": "1.1", "outdated": true})
	if bytes.Equal(ObservedHash([]model.DiagramResult{b}, nil), ObservedHash([]model.DiagramResult{c}, nil)) {
		t.Fatal("observed field did not change the hash")
	}
}

func TestDiffTable_NoDataMarkdownIsEmptyTable(t *testing.T) {
	a := model.DiagramResult{ID: "velero", Title: "Backups", Type: "markdown", Content: "*No data*"}
	b := table("velero", map[string]any{"cluster": "Homelab", "namespace": "velero", "name": "daily"})
	d := Diagram(a, b)
	if d.Type != "table" || d.Summary != (Summary{Added: 1}) {
		t.Fatalf("diff = %+v", d)
	}
}

func TestDiffTable_NoSpecKeysOnWholeRow(t *testing.T) {
	a := table("unknown", map[string]any{"x": 1})
	b := table("unknown", map[string]any{"x": 2})
	d := Diagram(a, b)
	if d.Summary != (Summary{Added: 1, Removed: 1}) {
		t.Fatalf("summary = %+v", d.Summary)
	}
}

func TestDiffFlow(t *testing.T) {
	a := flow(
		[]flowNode{{ID: "Homelab/a", Label: "a", Cluster: "Homelab", Layer: "apps"}, {ID: "Homelab/b", Label: "b", Cluster: "Homelab", Layer: "apps"}},
		[]flowEdge{{ID: "Homelab/a->Homelab/b", Source: "Homelab/a", Target: "Homelab/b"}},
	)
	b := flow(
		[]flowNode{{ID: "Homelab/a", Label: "a", Cluster: "Homelab", Layer: "platform"}, {ID: "Homelab/c", Label: "c", Cluster: "Homelab", Layer: "apps"}},
		[]flowEdge{{ID: "Homelab/a->Homelab/c", Source: "Homelab/a", Target: "Homelab/c"}},
	)
	d := Diagram(a, b)
	if d.Summary != (Summary{Added: 2, Removed: 2, Changed: 1}) {
		t.Fatalf("summary = %+v changes=%+v", d.Summary, d.Changes)
	}
}

func TestContentOnlyDiagrams(t *testing.T) {
	a := model.DiagramResult{ID: "topology", Type: "mermaid", Content: "graph TD; a-->b"}
	b := model.DiagramResult{ID: "topology", Type: "mermaid", Content: "graph TD; a-->c"}
	if d := Diagram(a, b); d.Summary.Changed != 1 || d.Changes[0].Kind != "content" {
		t.Fatalf("diff = %+v", d)
	}
	if d := Diagram(a, a); d.Summary.Total() != 0 {
		t.Fatalf("identical content produced changes: %+v", d)
	}
}

func TestAll_PairsByIDAndSorts(t *testing.T) {
	a := []model.DiagramResult{table("crds", map[string]any{"cluster": "H", "name": "x"}), {ID: "topology", Type: "mermaid", Content: "g"}}
	b := []model.DiagramResult{{ID: "topology", Type: "mermaid", Content: "g"}, table("velero", map[string]any{"cluster": "H", "namespace": "v", "name": "d"})}
	ds := All(a, b)
	if len(ds) != 3 || ds[0].DiagramID != "crds" || ds[1].DiagramID != "topology" || ds[2].DiagramID != "velero" {
		t.Fatalf("ids = %v", ds)
	}
	if ds[0].Summary.Removed != 1 || ds[2].Summary.Added != 1 || ds[1].Summary.Total() != 0 {
		t.Fatalf("summaries = %+v %+v %+v", ds[0].Summary, ds[1].Summary, ds[2].Summary)
	}
}

func TestRevisionsOf(t *testing.T) {
	data := &model.ClusterData{Flux: []model.FluxKustomization{
		{Name: "flux-system", Cluster: "Homelab", SourceKind: "GitRepository", LastAppliedRevision: "main@sha1:abc123"},
		{Name: "apps", Cluster: "Homelab", SourceKind: "GitRepository", DependsOn: []string{"flux-system"}, LastAppliedRevision: "main@sha1:abc123"},
		{Name: "flux-system", Cluster: "NAS", SourceKind: "OCIRepository", LastAppliedRevision: "latest@sha256:def"},
		{Name: "helm-only", Cluster: "NAS", SourceKind: "HelmRepository", LastAppliedRevision: "x"},
	}}
	revs := RevisionsOf(data)
	if len(revs) != 2 || revs[0].Cluster != "Homelab" || revs[0].SHA != "abc123" || revs[1].SHA != "def" {
		t.Fatalf("revs = %+v", revs)
	}
	if !SameRevisions(revs, RevisionsOf(data)) {
		t.Fatal("SameRevisions false for identical input")
	}
	h1 := ObservedHash(nil, revs)
	revs[0].Revision = "main@sha1:zzz"
	if bytes.Equal(h1, ObservedHash(nil, revs)) {
		t.Fatal("revision change did not change the hash")
	}
}

func TestParseSHA(t *testing.T) {
	for in, want := range map[string]string{"main@sha1:abc": "abc", "latest@sha256:def": "def", "main/abc": "abc", "abc": "abc"} {
		if got := ParseSHA(in); got != want {
			t.Errorf("ParseSHA(%q) = %q, want %q", in, got, want)
		}
	}
}
