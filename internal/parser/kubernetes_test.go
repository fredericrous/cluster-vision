package parser

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// Node roles come from a label map; they must not follow map order.
func TestParseNodesSortsRoles(t *testing.T) {
	labels := map[string]string{"kubernetes.io/hostname": "n1"}
	for _, r := range []string{"worker", "control-plane", "etcd", "master", "gpu", "storage", "ingress", "infra"} {
		labels["node-role.kubernetes.io/"+r] = ""
	}
	p := &KubernetesParser{
		typed:       fake.NewClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n1", Labels: labels}}),
		clusterName: "c",
	}
	for i := 0; i < 10; i++ {
		nodes := p.parseNodes(context.Background())
		if len(nodes) != 1 {
			t.Fatalf("nodes = %d", len(nodes))
		}
		if !slices.IsSorted(nodes[0].Roles) || len(nodes[0].Roles) != 8 {
			t.Fatalf("roles not sorted: %v", nodes[0].Roles)
		}
	}
}

var vulnGVR = schema.GroupVersionResource{Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "vulnerabilityreports"}

func vulnReport(name, repo, tag string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "aquasecurity.github.io/v1alpha1",
		"kind":       "VulnerabilityReport",
		"metadata":   map[string]interface{}{"name": name, "namespace": "ns"},
		"report": map[string]interface{}{
			"registry": map[string]interface{}{"server": "ghcr.io"},
			"artifact": map[string]interface{}{"repository": repo, "tag": tag},
			"summary":  map[string]interface{}{"criticalCount": int64(1)},
		},
	}}
}

// The reports are merged through a map; the result must still come out in
// a stable order.
func TestParseVulnReportsIsSorted(t *testing.T) {
	var objs []runtime.Object
	for _, r := range []string{"zeta", "alpha", "mu", "beta", "omega", "kappa", "gamma", "delta", "epsilon"} {
		objs = append(objs, vulnReport(r, "x/"+r, "1.0"))
	}
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{vulnGVR: "VulnerabilityReportList"}, objs...)
	p := &KubernetesParser{dynamic: dyn, clusterName: "c"}
	for i := 0; i < 10; i++ {
		got := p.parseVulnReports(context.Background())
		if len(got) != len(objs) {
			t.Fatalf("got %d reports, want %d", len(got), len(objs))
		}
		if !slices.IsSortedFunc(got, func(a, b model.ImageVuln) int { return strings.Compare(a.Image, b.Image) }) {
			t.Fatalf("reports not sorted by image")
		}
	}
}
