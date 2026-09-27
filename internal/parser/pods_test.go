package parser

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// The pod's image is its spec reference, whatever the runtime reports in
// status.image: containerd names an image by the first reference it stored
// that image ID under, so a digest-identical image moved from ghcr.io to
// git.daddyshome.fr kept reporting ghcr.io (live, 2026-09-26), and a
// digest-pinned spec often reports only the bare "sha256:<id>". status.imageID
// still records what the node runs.
func TestParsePodsUsesSpecImage(t *testing.T) {
	const id = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const pinned = "quay.io/cilium/cilium:v1.20.2@" + id
	const moved = "git.daddyshome.fr/fredericrous/agent-console:0.0.1@" + id
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init", Image: pinned}},
			Containers: []corev1.Container{
				{Name: "agent", Image: pinned},
				{Name: "console", Image: moved},
				{Name: "sidecar", Image: "nginx:1.25"},
			},
		},
		Status: corev1.PodStatus{
			Phase:                 corev1.PodRunning,
			InitContainerStatuses: []corev1.ContainerStatus{{Name: "init", Image: id, ImageID: "quay.io/cilium/cilium@" + id}},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "agent", Image: id, ImageID: "quay.io/cilium/cilium@" + id},
				{Name: "console", Image: "ghcr.io/fredericrous/agent-console:0.0.1", ImageID: "ghcr.io/fredericrous/agent-console@" + id},
				{Name: "sidecar", Image: "docker.io/library/nginx:1.25", ImageID: "docker.io/library/nginx@" + id},
			},
		},
	}
	p := &KubernetesParser{typed: fake.NewClientset(pod), clusterName: "c"}

	want := map[string]string{"init": pinned, "agent": pinned, "console": moved, "sidecar": "nginx:1.25"}
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
