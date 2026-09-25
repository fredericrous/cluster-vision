package diagram

import (
	"encoding/json"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// Trivy names Docker Hub "index.docker.io"; the pods below name the same
// images as written in their specs. Every row must still get its report.
func TestGenerateImagesJoinsTrivyReports(t *testing.T) {
	const d = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	data := &model.ClusterData{
		Pods: []model.PodImageInfo{
			{Cluster: "c", Namespace: "web", PodName: "a", Image: "nginx:1.25"},
			{Cluster: "c", Namespace: "backup", PodName: "b", Image: "velero/velero:v1.17.2"},
			{Cluster: "c", Namespace: "x", PodName: "c", Image: "ghcr.io/foo/bar@" + d},
		},
		ImageVulns: []model.ImageVuln{
			{Cluster: "c", Image: model.ImageKey("index.docker.io/library/nginx:1.25"), Critical: 1},
			{Cluster: "c", Image: model.ImageKey("index.docker.io/velero/velero:v1.17.2"), High: 2},
			{Cluster: "c", Image: model.ImageKey("ghcr.io/foo/bar@" + d), Medium: 3},
		},
	}

	res := GenerateImages(data, nil)
	var rows []ImageRow
	if err := json.Unmarshal([]byte(res.Content), &rows); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, res.Content)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows; want 3", len(rows))
	}
	for _, r := range rows {
		if r.SecurityRisk == "" {
			t.Errorf("%s:%s has no security risk; the Trivy report did not join", r.Image, r.Tag)
		}
	}
}

// A pod pinned to tag@digest and one pulling the bare tag are two rows,
// each with its own pin status, whatever order the pods come in.
func TestGenerateImagesSplitsPinnedFromUnpinned(t *testing.T) {
	const d = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	pinned := model.PodImageInfo{Cluster: "c", Namespace: "a", PodName: "p1", Image: "ghcr.io/x/api:1.2.3@" + d}
	unpinned := model.PodImageInfo{Cluster: "c", Namespace: "b", PodName: "p2", Image: "ghcr.io/x/api:1.2.3"}

	var first string
	for _, pods := range [][]model.PodImageInfo{{pinned, unpinned}, {unpinned, pinned}} {
		res := GenerateImages(&model.ClusterData{Pods: pods}, nil)
		var rows []ImageRow
		if err := json.Unmarshal([]byte(res.Content), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2: %+v", len(rows), rows)
		}
		for _, r := range rows {
			wantPinned := r.Namespaces == "a"
			if r.Pinned != wantPinned || (r.Digest == d) != wantPinned {
				t.Errorf("row %+v: pinned = %v, want %v", r, r.Pinned, wantPinned)
			}
		}
		if first == "" {
			first = res.Content
		} else if res.Content != first {
			t.Errorf("output depends on pod order")
		}
	}
}
