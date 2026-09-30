package parser

import (
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// Report fields as trivy-operator writes them (checked against the live
// homelab cluster), each paired with the pod image that must join it.
func TestTrivyImageKeyMatchesPodImages(t *testing.T) {
	const d = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		server, repository, tag, digest string
		podImage                        string
	}{
		{"index.docker.io", "library/nginx", "1.29.5-alpine", d, "nginx:1.29.5-alpine"},
		{"index.docker.io", "library/busybox", "1.36", d, "busybox:1.36"},
		{"index.docker.io", "velero/velero", "v1.17.2", d, "velero/velero:v1.17.2"},
		{"index.docker.io", "istio/proxyv2", "1.31.1", d, "docker.io/istio/proxyv2:1.31.1"},
		{"ghcr.io", "fredericrous/cluster-vision", "0.24.0", d, "ghcr.io/fredericrous/cluster-vision:0.24.0@" + d},
		{"ghcr.io", "foo/bar", "", d, "ghcr.io/foo/bar@" + d},
		// Digest-pinned pods: trivy-operator writes the pod ref into tag.
		{"index.docker.io", "library/haproxy", "docker.io/library/haproxy:3.2.23-alpine", d, "docker.io/library/haproxy:3.2.23-alpine@" + d},
		{"index.docker.io", "coturn/coturn", "docker.io/coturn/coturn", d, "docker.io/coturn/coturn@" + d},
	}
	for _, tt := range tests {
		got := trivyImageKey(tt.server, tt.repository, tt.tag, tt.digest)
		if want := model.ImageKey(tt.podImage); got != want {
			t.Errorf("trivyImageKey(%s, %s, %s) = %q; pod %q keys as %q", tt.server, tt.repository, tt.tag, got, tt.podImage, want)
		}
	}
}
