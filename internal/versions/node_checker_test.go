package versions

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The latest patch of a minor comes from its release marker, which also
// answers for minors too old to be in the newest page of GitHub releases.
func TestFetchLatestK8sPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release/stable-1.31.txt":
			_, _ = w.Write([]byte("v1.31.14\n"))
		case "/release/stable-1.24.txt":
			_, _ = w.Write([]byte("v1.24.17"))
		case "/release/stable-1.40.txt":
			_, _ = w.Write([]byte("v1.40.0-rc.1"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	nc := NewNodeChecker()
	nc.client = srv.Client()
	nc.k8sReleaseBase = srv.URL + "/release"

	for minor, want := range map[string]string{"1.31": "v1.31.14", "1.24": "v1.24.17"} {
		got, err := nc.fetchLatestK8sPatch(minor)
		if err != nil || got != want {
			t.Errorf("fetchLatestK8sPatch(%s) = %q, %v; want %q", minor, got, err, want)
		}
	}
	for _, minor := range []string{"1.40", "1.99"} {
		if got, err := nc.fetchLatestK8sPatch(minor); err == nil {
			t.Errorf("fetchLatestK8sPatch(%s) = %q, want an error", minor, got)
		}
	}
}
