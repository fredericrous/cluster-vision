package diagram

import (
	"testing"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// Snapshots hash and diff generator output, so generating twice from the
// same cluster model must yield byte-identical diagrams. This guards
// against unsorted map iteration and wall-clock reads creeping into a
// generator.
func TestGeneratorsAreDeterministic(t *testing.T) {
	data := fixture()
	gen := func() []model.DiagramResult {
		out := GenerateTopologySections(data)
		out = append(out, GenerateDependencies(data), GenerateNetwork(data))
		out = append(out, GenerateSecurity(data)...)
		out = append(out, GenerateImages(data, versions.NewImageChecker()))
		out = append(out, GenerateVersions(data, versions.NewChecker(time.Hour, "")))
		out = append(out, GenerateNodes(data, versions.NewNodeChecker(), versions.NewSecurityChecker()))
		out = append(out,
			GenerateWorkloads(data), GenerateStorage(data), GenerateCRDs(data), GenerateQuotas(data),
			GenerateCertificates(data), GenerateNetworkPolicies(data), GenerateConfigs(data),
			GenerateHelmWorkloads(data), GenerateServiceMap(data), GenerateNamespaceSummary(data),
			GenerateRBAC(data), GenerateLabels(data), GenerateVelero(data),
		)
		return out
	}
	a, b := gen(), gen()
	if len(a) != len(b) {
		t.Fatalf("diagram count differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("diagram %q is not deterministic:\n--- run 1\n%s\n--- run 2\n%s", a[i].ID, a[i].Content, b[i].Content)
		}
	}
}

func fixture() *model.ClusterData {
	return &model.ClusterData{
		PrimaryCluster: "Homelab",
		Nodes: []model.NodeInfo{
			{Name: "node-b", Cluster: "Homelab", IP: "10.0.0.2"},
			{Name: "node-a", Cluster: "Homelab", IP: "10.0.0.1"},
		},
		Flux: []model.FluxKustomization{
			{Name: "flux-system", Namespace: "flux-system", Cluster: "Homelab", Path: "./clusters/homelab", SourceKind: "GitRepository", SourceName: "flux-system", LastAppliedRevision: "main@sha1:abc123"},
			{Name: "apps", Namespace: "flux-system", Cluster: "Homelab", Path: "./apps", DependsOn: []string{"controllers"}},
			{Name: "controllers", Namespace: "flux-system", Cluster: "Homelab", Path: "./controllers", DependsOn: []string{"crds"}},
			{Name: "crds", Namespace: "flux-system", Cluster: "Homelab", Path: "./crds"},
		},
		Namespaces: []model.NamespaceInfo{{Name: "b", Cluster: "Homelab"}, {Name: "a", Cluster: "Homelab"}},
		HelmReleases: []model.HelmReleaseInfo{
			{Name: "istio", Namespace: "istio-system", Cluster: "Homelab", ChartName: "istiod", Version: "1.22.1", RepoName: "istio", RepoNS: "flux-system"},
		},
		HelmRepositories: []model.HelmRepositoryInfo{{Name: "istio", Namespace: "flux-system", Cluster: "Homelab", Type: "default", URL: "https://istio-release.storage.googleapis.com/charts"}},
		Pods: []model.PodImageInfo{
			{Cluster: "Homelab", Namespace: "a", PodName: "api-1", Container: "api", Image: "ghcr.io/x/api:1.2.3"},
			{Cluster: "Homelab", Namespace: "b", PodName: "web-1", Container: "web", Image: "nginx:1.25"},
		},
		Workloads: []model.WorkloadInfo{
			{Name: "api", Namespace: "a", Cluster: "Homelab", Kind: "Deployment", Replicas: 2, ReadyReplicas: 2, Images: []string{"ghcr.io/x/api:1.2.3"}, Labels: map[string]string{"app": "api", "tier": "backend"}, CreatedAt: "2026-01-01T00:00:00Z"},
			{Name: "web", Namespace: "b", Cluster: "Homelab", Kind: "Deployment", Replicas: 1, ReadyReplicas: 1, Images: []string{"nginx:1.25"}, Labels: map[string]string{"app": "web"}, CreatedAt: "2026-01-02T00:00:00Z"},
		},
		Certificates: []model.CertificateInfo{
			{Name: "web-tls", Namespace: "b", Cluster: "Homelab", DNSNames: []string{"web.example.com"}, IssuerName: "le", IssuerKind: "ClusterIssuer", NotAfter: "2026-12-01T00:00:00Z", Ready: true},
		},
		RBACBindings:    []model.RBACBindingInfo{{SubjectName: "admin", SubjectKind: "User", RoleName: "cluster-admin", RoleKind: "ClusterRole", Cluster: "Homelab"}},
		VeleroSchedules: []model.VeleroScheduleInfo{{Name: "daily", Namespace: "velero", Cluster: "Homelab"}},
	}
}
