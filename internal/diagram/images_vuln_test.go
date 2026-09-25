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
