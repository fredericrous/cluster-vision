package versions

import (
	"slices"
	"testing"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
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
		{"1.2.3+build", true, 1, 2, 3, ""},
		{"v1.31.4+k3s1", true, 1, 31, 4, ""},
		{"1.2.3-rc.1+build.5", true, 1, 2, 3, "-rc.1"},
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
		name     string
		proxy    string
		repoURL  string
		wantHost string
		wantPath string
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
		{"1.0.0-alpha", "1.0.0-alpha.1", true},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", true},
		{"1.0.0-beta.2", "1.0.0-beta.11", true},
		{"1.0.0-rc.1", "1.0.0-beta.11", false},
		{"1.0.0+build.2", "1.0.0+build.1", false}, // build metadata ignored
		{"1.0.0+k3s1", "1.0.0", false},
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

// Two clusters may each have a HelmRepository with the same
// namespace/name pointing at different URLs; each release must be checked
// against its own cluster's.
func TestChartChecksKeyRepositoriesByCluster(t *testing.T) {
	repos := []model.HelmRepositoryInfo{
		{Name: "charts", Namespace: "flux-system", Cluster: "home", Type: "default", URL: "https://home.example/charts"},
		{Name: "charts", Namespace: "flux-system", Cluster: "nas", Type: "oci", URL: "oci://nas.example/charts"},
	}
	releases := []model.HelmReleaseInfo{
		{Name: "app", Cluster: "home", ChartName: "app", RepoName: "charts", RepoNS: "flux-system"},
		{Name: "app", Cluster: "nas", ChartName: "app", RepoName: "charts", RepoNS: "flux-system"},
		{Name: "orphan", Cluster: "cloud", ChartName: "app", RepoName: "charts", RepoNS: "flux-system"},
	}
	got := chartChecks(repos, releases)
	want := []chartRef{
		{repoURL: "https://home.example/charts", repoType: "default", chartName: "app"},
		{repoURL: "oci://nas.example/charts", repoType: "oci", chartName: "app"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("checks = %+v, want %+v", got, want)
	}
}

func TestOutdated(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.31.4+k3s1", "v1.31.4", false},
		{"v1.31.4+k3s1", "v1.31.5", true},
		{"v1.2.3", "1.2.3", false},
		{"1.2.3", "v1.2.4", true},
		{"2.0.0-rc.1", "1.9.0", false}, // deployed pre-release ahead of stable
		{"2.0.0-rc.1", "2.0.0", true},
		{"1.2.3", "1.2.3", false},
		{"1.3.0", "1.2.9", false}, // ahead of the index
		{"", "1.0.0", false},
		{"2024.01", "2024.01", false},
		{"weird", "other", true},
	}
	for _, c := range cases {
		if got := Outdated(c.current, c.latest); got != c.want {
			t.Errorf("Outdated(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
