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
