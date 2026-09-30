package model

import "testing"

func TestImageKey(t *testing.T) {
	const d = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct{ in, want string }{
		{"nginx:1.25", "docker.io/library/nginx:1.25"},
		{"nginx", "docker.io/library/nginx:latest"},
		{"docker.io/nginx:1.25", "docker.io/library/nginx:1.25"},
		{"docker.io/library/nginx:1.25", "docker.io/library/nginx:1.25"},
		{"index.docker.io/library/nginx:1.25", "docker.io/library/nginx:1.25"},
		{"registry-1.docker.io/library/nginx:1.25", "docker.io/library/nginx:1.25"},
		{"velero/velero:v1.17.2", "docker.io/velero/velero:v1.17.2"},
		{"index.docker.io/velero/velero:v1.17.2", "docker.io/velero/velero:v1.17.2"},
		{"nginx:1.25@" + d, "docker.io/library/nginx:1.25"},
		{"ghcr.io/foo/bar@" + d, "ghcr.io/foo/bar@" + d},
		{"registry.local:5000/foo:1.0", "registry.local:5000/foo:1.0"},
		{"quay.io/cilium/cilium:v1.20.2", "quay.io/cilium/cilium:v1.20.2"},
	}
	for _, tt := range tests {
		if got := ImageKey(tt.in); got != tt.want {
			t.Errorf("ImageKey(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestVulnIndex(t *testing.T) {
	vulns := []ImageVuln{
		{Image: "docker.io/library/nginx:1.25", Cluster: "b", High: 1},
		{Image: "docker.io/library/nginx:1.25", Cluster: "a", Critical: 2},
		{Image: "docker.io/library/nginx:1.25", Cluster: "c", Critical: 2},
	}

	idx := NewVulnIndex(vulns)
	if v, ok := idx.Lookup("b", "nginx:1.25"); !ok || v.Cluster != "b" {
		t.Errorf("Lookup(b) = %+v, %v; want cluster b's report", v, ok)
	}
	if _, ok := idx.Lookup("d", "nginx:1.25"); ok {
		t.Error("Lookup(d) found a report; cluster d has none")
	}
	if _, ok := idx.Lookup("b", "nginx:1.26"); ok {
		t.Error("Lookup(nginx:1.26) found a report for another tag")
	}

	// Across clusters: the worst report, and the same one whatever the order.
	for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}} {
		var in []ImageVuln
		for _, i := range order {
			in = append(in, vulns[i])
		}
		v, ok := NewVulnIndex(in).Lookup("", "docker.io/nginx:1.25")
		if !ok || v.Cluster != "a" {
			t.Errorf("order %v: Lookup(\"\") = %+v, %v; want cluster a's report", order, v, ok)
		}
	}
}

func TestVulnIndexDigest(t *testing.T) {
	const d1 = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	const d2 = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	idx := NewVulnIndex([]ImageVuln{
		{Image: "docker.io/library/haproxy:3.2", Digest: d1, Cluster: "c", High: 1},
	})
	for _, ref := range []string{"haproxy:3.2", "haproxy@" + d1, "docker.io/library/haproxy:3.2@" + d1} {
		if _, ok := idx.Lookup("c", ref); !ok {
			t.Errorf("Lookup(%q) found nothing", ref)
		}
	}
	// A pod pinned to another digest of the tag still gets the tag's report.
	if _, ok := idx.Lookup("c", "haproxy:3.2@"+d2); !ok {
		t.Error("Lookup by tag with another digest found nothing")
	}
	if _, ok := idx.Lookup("c", "haproxy@"+d2); ok {
		t.Error("Lookup of an unscanned digest found a report")
	}
}
