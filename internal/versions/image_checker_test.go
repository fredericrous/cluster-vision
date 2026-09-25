package versions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// headDigest: the registry's Docker-Content-Digest for the tag, through the
// 401 Bearer dance, with the index media types requested.
func TestHeadDigest(t *testing.T) {
	digest := "sha256:" + strings.Repeat("e", 64)
	var mux http.ServeMux
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "repository:foo/bar:pull" {
			t.Errorf("scope = %q", r.URL.Query().Get("scope"))
		}
		_, _ = w.Write([]byte(`{"token":"t0k"}`))
	})
	srv := httptest.NewTLSServer(&mux)
	defer srv.Close()
	mux.HandleFunc("/v2/foo/bar/manifests/1.2.3", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Accept"), "application/vnd.oci.image.index.v1+json") {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Authorization") != "Bearer t0k" {
			w.Header().Set("Www-Authenticate", `Bearer realm="`+srv.URL+`/token",service="reg",scope="repository:foo/bar:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Docker-Content-Digest", digest)
		w.WriteHeader(http.StatusOK)
	})

	ic := NewImageChecker()
	ic.client = srv.Client()
	host := strings.TrimPrefix(srv.URL, "https://")

	got, err := ic.headDigest(host, "foo/bar", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got != digest {
		t.Fatalf("digest = %q, want %q", got, digest)
	}
	if _, err := ic.headDigest(host, "foo/bar", "missing"); err == nil {
		t.Fatal("a 404 must be an error, never an empty digest")
	}
}

func TestParseImageRefKeepsTheTagOfAPinnedReference(t *testing.T) {
	d := "sha256:" + strings.Repeat("f", 64)
	reg, repo, tag := parseImageRef("ghcr.io/x/y:1.2.3@" + d)
	if reg != "ghcr.io" || repo != "x/y" || tag != "1.2.3" {
		t.Fatalf("got (%q, %q, %q)", reg, repo, tag)
	}
	if _, _, tag := parseImageRef("ghcr.io/x/y@" + d); tag != "" {
		t.Fatalf("bare digest must have no tag, got %q", tag)
	}
}

func TestHighestMatchingTag(t *testing.T) {
	cases := []struct {
		name     string
		deployed string
		tags     []string
		want     string
	}{
		{"plain semver", "1.2.3", []string{"1.2.3", "1.2.4", "1.3.0", "2.0.0-rc.1", "latest"}, "1.3.0"},
		{"v prefix", "v1.2.3", []string{"v1.2.3", "v1.10.0", "1.20.0"}, "v1.10.0"},
		{"bitnami revision", "1.2.3-debian-12-r4", []string{"1.2.3-debian-12-r4", "1.2.4-debian-12-r0", "1.2.4", "1.2.5-debian-12-r0-rc"}, "1.2.4-debian-12-r0"},
		{"bitnami rebuild of the same version", "1.2.3-debian-12-r4", []string{"1.2.3-debian-12-r4", "1.2.3-debian-12-r11"}, "1.2.3-debian-12-r11"},
		{"alpine base bump", "16.4-alpine3.20", []string{"16.4-alpine3.20", "16.4-alpine3.21", "16.4", "17.0-alpine3.21"}, "17.0-alpine3.21"},
		{"alpine same version newer base", "16.4-alpine3.20", []string{"16.4-alpine3.21", "16.4-bookworm"}, "16.4-alpine3.21"},
		{"pre-release is another variant", "1.2.3", []string{"1.2.4-rc.1", "1.3.0-beta2"}, "1.2.3"},
		{"deployed is newest", "2.0.0", []string{"1.9.0", "2.0.0"}, "2.0.0"},
		{"no semver", "latest", []string{"1.0.0"}, "-"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := highestMatchingTag(c.deployed, c.tags); got != c.want {
				t.Errorf("highestMatchingTag(%q) = %q, want %q", c.deployed, got, c.want)
			}
		})
	}
}

// A registry that never stops sending a next link must not keep the
// checker paging forever.
func TestListTagsCapsPagination(t *testing.T) {
	var requests int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Link", `</v2/foo/bar/tags/list?n=1000&last=x>; rel="next"`)
		_, _ = w.Write([]byte(`{"tags":["1.0.0"]}`))
	}))
	defer srv.Close()

	ic := NewImageChecker()
	ic.client = srv.Client()
	tags, err := ic.listTags(strings.TrimPrefix(srv.URL, "https://"), "foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if requests != maxTagPages || len(tags) != maxTagPages {
		t.Fatalf("requests = %d, tags = %d; want %d pages", requests, len(tags), maxTagPages)
	}
}
