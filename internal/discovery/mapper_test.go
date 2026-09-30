package discovery

import (
	"strings"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

func strPtr(s string) *string { return &s }

func TestMapClusterData(t *testing.T) {
	t.Run("helm releases produce apps", func(t *testing.T) {
		data := &model.ClusterData{
			HelmReleases: []model.HelmReleaseInfo{
				{Name: "grafana", Namespace: "monitoring", Cluster: "homelab", ChartName: "grafana", Version: "7.0.0"},
			},
			Workloads: []model.WorkloadInfo{
				{Name: "grafana", Namespace: "monitoring", Cluster: "homelab", Images: []string{"grafana/grafana:10.0.0"},
					Labels: map[string]string{"app.kubernetes.io/instance": "grafana"}},
			},
			Nodes: []model.NodeInfo{
				{Name: "node-1", Cluster: "homelab", KubeletVersion: "v1.30.0", Platform: "proxmox"},
			},
		}

		apps := MapClusterData(data)
		if len(apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(apps))
		}
		if apps[0].Name != "grafana" {
			t.Errorf("app name = %q, want %q", apps[0].Name, "grafana")
		}
		if apps[0].Namespace != "monitoring" {
			t.Errorf("namespace = %q, want %q", apps[0].Namespace, "monitoring")
		}
		if *apps[0].ChartName != "grafana" {
			t.Errorf("chart name = %q, want %q", *apps[0].ChartName, "grafana")
		}
		if *apps[0].ChartVersion != "7.0.0" {
			t.Errorf("chart version = %q, want %q", *apps[0].ChartVersion, "7.0.0")
		}
		if len(apps[0].Images) != 1 || apps[0].Images[0] != "grafana/grafana:10.0.0" {
			t.Errorf("images = %v, want [grafana/grafana:10.0.0]", apps[0].Images)
		}
	})

	t.Run("empty ClusterData returns empty results", func(t *testing.T) {
		data := &model.ClusterData{}
		apps := MapClusterData(data)
		if len(apps) != 0 {
			t.Errorf("expected 0 apps, got %d", len(apps))
		}
	})
}

// Infrastructure is context for a CI, not a peer CI: nodes and storage classes
// were once mapped into it_components, duplicating what the core Nodes/Storage
// views already render live. Guards against reintroducing that write path.
func TestInfrastructureAloneYieldsNoCIs(t *testing.T) {
	data := &model.ClusterData{
		Nodes: []model.NodeInfo{
			{Name: "node-1", Cluster: "homelab", KubeletVersion: "v1.30.0", Platform: "proxmox"},
			{Name: "node-2", Cluster: "homelab", KubeletVersion: "v1.30.0"},
		},
		Storage: []model.StorageInfo{
			{Name: "ceph-block", Kind: "StorageClass"},
			{Name: "my-pvc", Kind: "PersistentVolumeClaim"},
		},
	}

	if apps := MapClusterData(data); len(apps) != 0 {
		t.Errorf("nodes/storage produced %d CIs, want 0 — infrastructure is an attribute of an app, not its own CI", len(apps))
	}
}

func TestMapHelmReleases(t *testing.T) {
	t.Run("single release with chart and images", func(t *testing.T) {
		data := &model.ClusterData{
			HelmReleases: []model.HelmReleaseInfo{
				{Name: "loki", Namespace: "monitoring", Cluster: "homelab", ChartName: "loki", Version: "5.0.0"},
			},
			Workloads: []model.WorkloadInfo{
				{Name: "loki", Namespace: "monitoring", Cluster: "homelab", Images: []string{"grafana/loki:2.9.0"},
					Labels: map[string]string{"app.kubernetes.io/instance": "loki"}},
				{Name: "loki-canary", Namespace: "monitoring", Cluster: "homelab", Images: []string{"grafana/loki-canary:2.9.0"},
					Labels: map[string]string{"app.kubernetes.io/instance": "loki"}},
			},
		}

		apps := mapHelmReleases(data)
		if len(apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(apps))
		}
		if apps[0].HelmRelease == nil || *apps[0].HelmRelease != "loki" {
			t.Errorf("helm release = %v, want %q", apps[0].HelmRelease, "loki")
		}
		if len(apps[0].Images) != 2 {
			t.Errorf("expected 2 images, got %d", len(apps[0].Images))
		}
	})

	t.Run("multiple releases", func(t *testing.T) {
		data := &model.ClusterData{
			HelmReleases: []model.HelmReleaseInfo{
				{Name: "a", Namespace: "ns-a", Cluster: "c"},
				{Name: "b", Namespace: "ns-b", Cluster: "c"},
			},
		}
		apps := mapHelmReleases(data)
		if len(apps) != 2 {
			t.Fatalf("expected 2 apps, got %d", len(apps))
		}
	})
}

func TestMapStandaloneWorkloads(t *testing.T) {
	t.Run("groups by app label and skips helm namespaces", func(t *testing.T) {
		helmApps := []DiscoveredApp{
			{Name: "grafana", Namespace: "monitoring", Cluster: "homelab"},
		}

		data := &model.ClusterData{
			Workloads: []model.WorkloadInfo{
				// In helm namespace — should be skipped
				{Name: "grafana-deploy", Namespace: "monitoring", Cluster: "homelab", Kind: "Deployment", Labels: map[string]string{"app.kubernetes.io/name": "grafana"}},
				// Standalone — should be included
				{Name: "my-api-deploy", Namespace: "default", Cluster: "homelab", Kind: "Deployment", Labels: map[string]string{"app.kubernetes.io/name": "my-api"}, Images: []string{"myapi:v1"}},
				{Name: "my-api-worker", Namespace: "default", Cluster: "homelab", Kind: "Deployment", Labels: map[string]string{"app.kubernetes.io/name": "my-api"}, Images: []string{"myapi:v1", "redis:7"}},
			},
		}

		apps := mapStandaloneWorkloads(data, helmApps)
		if len(apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(apps))
		}
		if apps[0].Name != "my-api" {
			t.Errorf("name = %q, want %q", apps[0].Name, "my-api")
		}
		if apps[0].WorkloadName == nil || *apps[0].WorkloadName != "my-api" {
			t.Errorf("workload name = %v, want %q", apps[0].WorkloadName, "my-api")
		}
	})

	t.Run("falls back to workload name when no label", func(t *testing.T) {
		data := &model.ClusterData{
			Workloads: []model.WorkloadInfo{
				{Name: "orphan-deploy", Namespace: "default", Cluster: "homelab", Kind: "Deployment", Labels: map[string]string{}, Images: []string{"orphan:v1"}},
			},
		}

		apps := mapStandaloneWorkloads(data, nil)
		if len(apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(apps))
		}
		if apps[0].Name != "orphan-deploy" {
			t.Errorf("name = %q, want %q", apps[0].Name, "orphan-deploy")
		}
	})
}

func TestEnrichWithVulns(t *testing.T) {
	t.Run("matches images to vulns and aggregates", func(t *testing.T) {
		apps := []DiscoveredApp{
			{Name: "app1", Images: []string{"img1:v1", "img2:v2"}},
			{Name: "app2", Images: []string{"img3:v3"}},
		}
		vulns := []model.ImageVuln{
			{Image: "img1:v1", Critical: 2, High: 3},
			{Image: "img2:v2", Critical: 1, High: 0},
		}

		enrichWithVulns(apps, vulns)
		if apps[0].VulnCritical != 3 {
			t.Errorf("app1 critical = %d, want 3", apps[0].VulnCritical)
		}
		if apps[0].VulnHigh != 3 {
			t.Errorf("app1 high = %d, want 3", apps[0].VulnHigh)
		}
		if apps[1].VulnCritical != 0 {
			t.Errorf("app2 critical = %d, want 0", apps[1].VulnCritical)
		}
	})

	t.Run("no match means zero vulns", func(t *testing.T) {
		apps := []DiscoveredApp{
			{Name: "clean-app", Images: []string{"clean:v1"}},
		}
		vulns := []model.ImageVuln{
			{Image: "other:v1", Critical: 5, High: 10},
		}

		enrichWithVulns(apps, vulns)
		if apps[0].VulnCritical != 0 || apps[0].VulnHigh != 0 {
			t.Errorf("expected 0 vulns, got critical=%d, high=%d", apps[0].VulnCritical, apps[0].VulnHigh)
		}
	})
}

func TestMapHelmReleasesImageAttribution(t *testing.T) {
	flux := func(hr string) map[string]string {
		return map[string]string{"helm.toolkit.fluxcd.io/name": hr, "helm.toolkit.fluxcd.io/namespace": "flux-system"}
	}
	data := &model.ClusterData{
		HelmReleases: []model.HelmReleaseInfo{
			// Kept in flux-system, installed into monitoring.
			{Name: "grafana", Namespace: "flux-system", TargetNamespace: "monitoring", ReleaseName: "monitoring-grafana", Cluster: "a"},
			{Name: "loki", Namespace: "flux-system", TargetNamespace: "monitoring", ReleaseName: "monitoring-loki", Cluster: "a"},
			// Unlabelled by Flux: matched on instance label + namespace.
			{Name: "redis", Namespace: "cache", ReleaseName: "redis", Cluster: "a"},
		},
		Workloads: []model.WorkloadInfo{
			{Name: "grafana", Namespace: "monitoring", Cluster: "a", Images: []string{"grafana/grafana:11"}, Labels: flux("grafana")},
			{Name: "loki", Namespace: "monitoring", Cluster: "a", Images: []string{"grafana/loki:3"}, Labels: flux("loki")},
			// Same namespace name in another cluster: not grafana's.
			{Name: "grafana", Namespace: "monitoring", Cluster: "b", Images: []string{"grafana/grafana:9"}, Labels: flux("grafana")},
			// The Flux controllers the old namespace match attributed to
			// every HelmRelease kept in flux-system.
			{Name: "helm-controller", Namespace: "flux-system", Cluster: "a", Images: []string{"ghcr.io/fluxcd/helm-controller:v1"}},
			{Name: "redis", Namespace: "cache", Cluster: "a", Images: []string{"redis:7"}, Labels: map[string]string{"app.kubernetes.io/instance": "redis"}},
			{Name: "other", Namespace: "cache", Cluster: "a", Images: []string{"memcached:1"}, Labels: map[string]string{"app.kubernetes.io/instance": "other"}},
		},
	}

	want := map[string]struct {
		ns, legacy string
		images     []string
	}{
		"grafana": {"monitoring", "flux-system", []string{"grafana/grafana:11"}},
		"loki":    {"monitoring", "flux-system", []string{"grafana/loki:3"}},
		"redis":   {"cache", "", []string{"redis:7"}},
	}
	for _, app := range mapHelmReleases(data) {
		w := want[app.Name]
		if app.Namespace != w.ns || app.LegacyNamespace != w.legacy {
			t.Errorf("%s: namespace %q legacy %q; want %q, %q", app.Name, app.Namespace, app.LegacyNamespace, w.ns, w.legacy)
		}
		if len(app.Images) != len(w.images) || (len(w.images) > 0 && app.Images[0] != w.images[0]) {
			t.Errorf("%s: images %v; want %v", app.Name, app.Images, w.images)
		}
	}

	// grafana's standalone-looking workload in monitoring is covered by
	// its release, now that the release counts as living there.
	for _, app := range mapStandaloneWorkloads(data, mapHelmReleases(data)) {
		if app.Cluster == "a" && app.Namespace == "monitoring" {
			t.Errorf("release workload %s duplicated as a standalone app", app.Name)
		}
	}
}

func TestPrimaryImageTag(t *testing.T) {
	tests := []struct {
		name   string
		images []string
		want   *string
	}{
		{"tag from image", []string{"repo:v1.2.3"}, strPtr("v1.2.3")},
		{"no colon returns nil", []string{"repo"}, nil},
		{"empty slice returns nil", nil, nil},
		{"uses first image only", []string{"a:v1", "b:v2"}, strPtr("v1")},
		{"sha tag", []string{"repo:sha-abc123"}, strPtr("sha-abc123")},
		{"registry port", []string{"registry.local:5000/team/app:1.4.2"}, strPtr("1.4.2")},
		{"registry port, no tag", []string{"registry.local:5000/team/app"}, nil},
		{"tag and digest", []string{"ghcr.io/x/y:1.2.3@sha256:" + strings.Repeat("a", 64)}, strPtr("1.2.3")},
		{"digest only", []string{"ghcr.io/x/y@sha256:" + strings.Repeat("a", 64)}, strPtr("sha256:" + strings.Repeat("a", 64))},
		{"explicit latest", []string{"nginx:latest"}, strPtr("latest")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrimaryImageTag(tt.images)
			if tt.want == nil && got != nil {
				t.Errorf("PrimaryImageTag(%v) = %q, want nil", tt.images, *got)
			}
			if tt.want != nil && (got == nil || *got != *tt.want) {
				t.Errorf("PrimaryImageTag(%v) = %v, want %q", tt.images, got, *tt.want)
			}
		})
	}
}

func TestDedup(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  int
	}{
		{"removes duplicates", []string{"a", "b", "a", "c", "b"}, 3},
		{"preserves order", []string{"c", "a", "b"}, 3},
		{"empty input", nil, 0},
		{"single element", []string{"x"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedup(tt.input)
			if len(got) != tt.want {
				t.Errorf("dedup(%v) len = %d, want %d", tt.input, len(got), tt.want)
			}
		})
	}

	// Test order preservation explicitly
	t.Run("order is preserved", func(t *testing.T) {
		got := dedup([]string{"c", "a", "b", "a"})
		if got[0] != "c" || got[1] != "a" || got[2] != "b" {
			t.Errorf("dedup order = %v, want [c a b]", got)
		}
	})
}

func TestBuildK8sSource(t *testing.T) {
	appID := uuid.New()
	app := store.Application{ID: appID}

	helmRelease := "grafana"
	chartName := "grafana"
	chartVersion := "7.0.0"

	da := DiscoveredApp{
		Name:         "grafana",
		Namespace:    "monitoring",
		Cluster:      "homelab",
		HelmRelease:  &helmRelease,
		ChartName:    &chartName,
		ChartVersion: &chartVersion,
		Images:       []string{"grafana/grafana:10.0.0"},
	}

	src := BuildK8sSource(app, da)

	if src.AppID != appID {
		t.Errorf("AppID = %v, want %v", src.AppID, appID)
	}
	if src.Cluster != "homelab" {
		t.Errorf("Cluster = %q, want %q", src.Cluster, "homelab")
	}
	if src.Namespace != "monitoring" {
		t.Errorf("Namespace = %q, want %q", src.Namespace, "monitoring")
	}
	if src.HelmRelease == nil || *src.HelmRelease != "grafana" {
		t.Errorf("HelmRelease = %v, want %q", src.HelmRelease, "grafana")
	}
	if src.ChartName == nil || *src.ChartName != "grafana" {
		t.Errorf("ChartName = %v, want %q", src.ChartName, "grafana")
	}
	if src.ChartVersion == nil || *src.ChartVersion != "7.0.0" {
		t.Errorf("ChartVersion = %v, want %q", src.ChartVersion, "7.0.0")
	}
	if len(src.Images) != 1 || src.Images[0] != "grafana/grafana:10.0.0" {
		t.Errorf("Images = %v, want [grafana/grafana:10.0.0]", src.Images)
	}
}

// Standalone apps are grouped through a map; they must come out in a stable
// order.
func TestMapStandaloneWorkloadsIsSorted(t *testing.T) {
	data := &model.ClusterData{}
	for _, n := range []string{"zeta", "alpha", "mu", "beta", "omega", "kappa", "gamma", "delta", "epsilon"} {
		data.Workloads = append(data.Workloads, model.WorkloadInfo{Name: n, Namespace: "default", Cluster: "homelab", Kind: "Deployment"})
	}
	for i := 0; i < 10; i++ {
		apps := mapStandaloneWorkloads(data, nil)
		for j := 1; j < len(apps); j++ {
			if apps[j-1].Name > apps[j].Name {
				t.Fatalf("apps not sorted: %s before %s", apps[j-1].Name, apps[j].Name)
			}
		}
	}
}
