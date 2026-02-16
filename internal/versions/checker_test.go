package versions

import (
	"testing"
	"time"
)

func TestHighestStableSemver(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
	}{
		{"simple", []string{"1.0.0", "2.0.0", "1.5.0"}, "2.0.0"},
		{"with v prefix", []string{"v1.0.0", "v2.1.0", "v1.5.3"}, "v2.1.0"},
		{"pre-release skipped", []string{"1.0.0", "1.0.1-rc1", "1.0.1"}, "1.0.1"},
		{"only pre-release", []string{"1.0.0-rc1", "1.0.0-alpha", "1.0.0-beta"}, ""},
		{"rc higher than stable ignored", []string{"3.1.0-rc.2", "3.5.2", "3.0.0"}, "3.5.2"},
		{"patch ordering", []string{"1.2.3", "1.2.10", "1.2.9"}, "1.2.10"},
		{"empty list", []string{}, ""},
		{"non-semver ignored", []string{"latest", "main", "1.0.0"}, "1.0.0"},
		{"mixed", []string{"0.1.0", "0.2.0", "0.1.5"}, "0.2.0"},
		{"two part", []string{"1.0", "2.0", "1.5"}, "2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highestStableSemver(tt.versions)
			if got != tt.want {
				t.Errorf("highestStableSemver(%v) = %q, want %q", tt.versions, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
		major int
		minor int
		patch int
		pre   string
	}{
		{"1.2.3", true, 1, 2, 3, ""},
		{"v1.2.3", true, 1, 2, 3, ""},
		{"1.2.3-rc1", true, 1, 2, 3, "-rc1"},
		{"1.2.3+build", true, 1, 2, 3, "+build"},
		{"1.2", true, 1, 2, 0, ""},
		{"latest", false, 0, 0, 0, ""},
		{"1", false, 0, 0, 0, ""},
		{"1.2.3.4", false, 0, 0, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sv, ok := parseSemver(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseSemver(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if sv.major != tt.major || sv.minor != tt.minor || sv.patch != tt.patch || sv.pre != tt.pre {
				t.Errorf("parseSemver(%q) = {%d, %d, %d, %q}, want {%d, %d, %d, %q}",
					tt.input, sv.major, sv.minor, sv.patch, sv.pre, tt.major, tt.minor, tt.patch, tt.pre)
			}
		})
	}
}

func TestResolveUpstream(t *testing.T) {
	tests := []struct {
		name      string
		proxy     string
		repoURL   string
		wantHost  string
		wantPath  string
	}{
		{
			"ghcr through proxy",
			"192.168.1.43:5000",
			"oci://192.168.1.43:5000/ghcr.io/grafana/helm-charts",
			"ghcr.io",
			"grafana/helm-charts",
		},
		{
			"docker.io through proxy",
			"192.168.1.43:5000",
			"oci://192.168.1.43:5000/docker.io/giteacharts",
			"registry-1.docker.io",
			"giteacharts",
		},
		{
			"gcr through proxy",
			"192.168.1.43:5000",
			"oci://192.168.1.43:5000/gcr.io/istio-release/charts",
			"gcr.io",
			"istio-release/charts",
		},
		{
			"direct oci (no proxy)",
			"",
			"oci://ghcr.io/fredericrous/charts",
			"ghcr.io",
			"fredericrous/charts",
		},
		{
			"different host not resolved",
			"192.168.1.43:5000",
			"oci://other-registry:5000/myrepo",
			"other-registry:5000",
			"myrepo",
		},
		{
			"registry.k8s.io through proxy",
			"192.168.1.43:5000",
			"oci://192.168.1.43:5000/registry.k8s.io/nfd/charts",
			"registry.k8s.io",
			"nfd/charts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(time.Minute, tt.proxy)
			host, path := c.resolveUpstream(tt.repoURL)
			if host != tt.wantHost || path != tt.wantPath {
				t.Errorf("resolveUpstream(%q) = (%q, %q), want (%q, %q)",
					tt.repoURL, host, path, tt.wantHost, tt.wantPath)
			}
		})
	}
}

func TestParseAuthParams(t *testing.T) {
	input := `realm="https://ghcr.io/token",service="ghcr.io",scope="repository:grafana/helm-charts/grafana:pull"`
	params := parseAuthParams(input)

	if params["realm"] != "https://ghcr.io/token" {
		t.Errorf("realm = %q", params["realm"])
	}
	if params["service"] != "ghcr.io" {
		t.Errorf("service = %q", params["service"])
	}
	if params["scope"] != "repository:grafana/helm-charts/grafana:pull" {
		t.Errorf("scope = %q", params["scope"])
	}
}

func TestParseLinkNext(t *testing.T) {
	tests := []struct {
		name    string
		link    string
		current string
		want    string
	}{
		{
			"ghcr pagination",
			`</v2/kyverno/charts/kyverno/tags/list?n=1000&last=3.0.0>; rel="next"`,
			"https://ghcr.io/v2/kyverno/charts/kyverno/tags/list?n=1000",
			"https://ghcr.io/v2/kyverno/charts/kyverno/tags/list?n=1000&last=3.0.0",
		},
		{"empty", "", "https://ghcr.io/foo", ""},
		{"no next rel", `</v2/foo>; rel="prev"`, "https://ghcr.io/foo", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkNext(tt.link, tt.current)
			if got != tt.want {
				t.Errorf("parseLinkNext(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}

func TestSemverLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.0.0", false},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "1.0.1", true},
		{"1.0.0-rc1", "1.0.0", true},  // pre-release < release
		{"1.0.0", "1.0.0-rc1", false}, // release > pre-release
		{"1.0.0", "1.0.0", false},     // equal
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a, _ := parseSemver(tt.a)
			b, _ := parseSemver(tt.b)
			got := a.less(b)
			if got != tt.want {
				t.Errorf("(%q).less(%q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
