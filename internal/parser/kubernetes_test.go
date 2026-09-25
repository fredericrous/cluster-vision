package parser

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

var (
	securityPolicyGVR = schema.GroupVersionResource{Group: "gateway.envoyproxy.io", Version: "v1alpha1", Resource: "securitypolicies"}
	ctpGVR            = schema.GroupVersionResource{Group: "gateway.envoyproxy.io", Version: "v1alpha1", Resource: "clienttrafficpolicies"}
)

// A failed policy list (timeout, 403) must mark the parse partial like
// every other list does; otherwise the snapshot records every policy as
// removed. A cluster without Envoy Gateway's CRDs is not a failure.
func TestPolicyListFailuresMarkParsePartial(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantPartial bool
	}{
		{"forbidden", apierrors.NewForbidden(securityPolicyGVR.GroupResource(), "", errors.New("rbac")), true},
		{"timeout", apierrors.NewTimeoutError("slow", 1), true},
		{"crd not installed", apierrors.NewNotFound(securityPolicyGVR.GroupResource(), ""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, parse := range []func(*KubernetesParser) int{
				func(p *KubernetesParser) int { return len(p.parseSecurityPolicies(context.Background())) },
				func(p *KubernetesParser) int { return len(p.parseClientTrafficPolicies(context.Background())) },
			} {
				dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
					securityPolicyGVR: "SecurityPolicyList", ctpGVR: "ClientTrafficPolicyList",
				})
				dyn.PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, tc.err
				})
				p := &KubernetesParser{dynamic: dyn, clusterName: "c"}
				if n := parse(p); n != 0 {
					t.Fatalf("got %d policies from a failed list", n)
				}
				if partial := p.failed.Load() > 0; partial != tc.wantPartial {
					t.Fatalf("partial = %v, want %v", partial, tc.wantPartial)
				}
			}
		})
	}
}
