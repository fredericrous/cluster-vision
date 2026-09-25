package parser

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// containerd reports status.image as the bare image ID for a digest-pinned
// spec (seen live on cilium, forgejo-runner, gtm-agent). The pod's image must
// stay the spec reference, not "sha256:…" — which the image checker would
// otherwise look up as docker.io/library/sha256.
func TestParsePodsIgnoresBareImageIDStatus(t *testing.T) {
	const id = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const pinned = "quay.io/cilium/cilium:v1.20.2@" + id
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "cilium-x", Namespace: "kube-system"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init", Image: pinned}},
			Containers: []corev1.Container{
				{Name: "agent", Image: pinned},
				{Name: "sidecar", Image: "nginx:1.25"},
			},
		},
		Status: corev1.PodStatus{
			Phase:                 corev1.PodRunning,
			InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", Image: id, ImageID: "quay.io/cilium/cilium@" + id}},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "agent", Image: id, ImageID: "quay.io/cilium/cilium@" + id},
				// A real resolved reference still wins over the spec.
				{Name: "sidecar", Image: "docker.io/library/nginx:1.25", ImageID: "docker.io/library/nginx@" + id},
			},
		},
	}
	p := &KubernetesParser{typed: fake.NewClientset(pod), clusterName: "c"}

	want := map[string]string{
		"init":    pinned,
		"agent":   pinned,
		"sidecar": "docker.io/library/nginx:1.25",
	}
	got := p.parsePods(context.Background())
	if len(got) != len(want) {
		t.Fatalf("got %d pod images; want %d", len(got), len(want))
	}
	for _, pi := range got {
		if pi.Image != want[pi.Container] {
			t.Errorf("%s: image %q; want %q", pi.Container, pi.Image, want[pi.Container])
		}
		if pi.ImageID == "" {
			t.Errorf("%s: imageID dropped", pi.Container)
		}
	}
}

func TestIsBareImageID(t *testing.T) {
	for s, want := range map[string]bool{
		"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef": true,
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef":        true,
		"nginx:1.25":                          false,
		"docker.io/library/nginx@sha256:0123": false,
		"sha256:XYZ":                          false,
		"":                                    false,
	} {
		if got := isBareImageID(s); got != want {
			t.Errorf("isBareImageID(%q) = %v; want %v", s, got, want)
		}
	}
}
