package diff_test

import (
	"testing"

	"github.com/fredericrous/cluster-vision/internal/diagram"
	"github.com/fredericrous/cluster-vision/internal/diff"
	"github.com/fredericrous/cluster-vision/internal/model"
)

// Two LoadBalancer Services of the same name in different namespaces are
// two rows; keyed on cluster/type/name they collapsed, and a diff of a
// cluster against itself reported phantom changes.
func TestNodesTableKeysLoadBalancersPerNamespace(t *testing.T) {
	data := func(ip string) *model.ClusterData {
		return &model.ClusterData{LoadBalancers: []model.LoadBalancerService{
			{Name: "gateway", Namespace: "public", Cluster: "c", IP: ip},
			{Name: "gateway", Namespace: "internal", Cluster: "c", IP: "10.0.0.2"},
		}}
	}
	a := diagram.GenerateNodes(data("10.0.0.1"), nil, nil)
	if d := diff.Diagram(a, diagram.GenerateNodes(data("10.0.0.1"), nil, nil)); d.Summary.Total() != 0 {
		t.Fatalf("self-diff reported changes: %+v", d.Changes)
	}
	d := diff.Diagram(a, diagram.GenerateNodes(data("10.0.0.9"), nil, nil))
	if d.Summary != (diff.Summary{Changed: 1}) || d.Changes[0].ID != "c/load-balancer/public/gateway" {
		t.Fatalf("changes = %+v", d.Changes)
	}
}
