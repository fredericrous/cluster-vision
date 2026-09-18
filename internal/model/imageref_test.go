package model

import "testing"

func TestSplitImageRef(t *testing.T) {
	d := "sha256:" + repeat("a", 64)
	cases := []struct {
		ref                         string
		registry, repo, tag, digest string
	}{
		{"ghcr.io/foo/bar:1.2.3", "ghcr.io", "foo/bar", "1.2.3", ""},
		{"ghcr.io/foo/bar:1.2.3@" + d, "ghcr.io", "foo/bar", "1.2.3", d},
		{"ghcr.io/foo/bar@" + d, "ghcr.io", "foo/bar", "", d},
		{"nginx", "docker.io", "library/nginx", "latest", ""},
		{"nginx:1.27", "docker.io", "library/nginx", "1.27", ""},
		{"rook/ceph:v1.19.1", "docker.io", "rook/ceph", "v1.19.1", ""},
		{"registry.local:5000/foo:1.0", "registry.local:5000", "foo", "1.0", ""},
		{"registry.local:5000/foo", "registry.local:5000", "foo", "latest", ""},
		{"registry.local:5000/foo@" + d, "registry.local:5000", "foo", "", d},
		{"localhost/x/y:t", "localhost", "x/y", "t", ""},
	}
	for _, c := range cases {
		reg, repo, tag, digest := SplitImageRef(c.ref)
		if reg != c.registry || repo != c.repo || tag != c.tag || digest != c.digest {
			t.Errorf("SplitImageRef(%q) = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
				c.ref, reg, repo, tag, digest, c.registry, c.repo, c.tag, c.digest)
		}
	}
}

func TestDigestOf(t *testing.T) {
	d := "sha256:" + repeat("b", 64)
	for in, want := range map[string]string{
		"docker.io/foo/bar@" + d:             d,
		"docker-pullable://ghcr.io/x/y@" + d: d,
		d:                                    d,
		"sha256:short":                       "",
		"":                                   "",
		"ghcr.io/x/y:tag":                    "",
	} {
		if got := DigestOf(in); got != want {
			t.Errorf("DigestOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}
