package discovery

import (
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/store"
)

// DiscoveredApp holds a mapped application from K8s data.
type DiscoveredApp struct {
	Name         string
	Namespace    string
	Cluster      string
	HelmRelease  *string
	ChartName    *string
	ChartVersion *string
	WorkloadName *string
	WorkloadKind *string
	Images       []string
	VulnCritical int
	VulnHigh     int
}

// MapClusterData extracts EAM entities from parsed ClusterData.
//
// Applications only. Nodes and StorageClasses were once mapped into
// it_components, but that persisted a second, staler copy of what the core
// Nodes and Storage views already render live from ClusterData (with kubelet
// version and CVE enrichment this copy never carried) — and nothing read it.
// Where an app runs is an attribute of the app, carried on k8s_sources; a node
// is context for a CI, not a peer CI.
func MapClusterData(data *model.ClusterData) []DiscoveredApp {
	apps := mapHelmReleases(data)
	apps = append(apps, mapStandaloneWorkloads(data, apps)...)
	enrichWithVulns(apps, data.ImageVulns)
	return apps
}

func mapHelmReleases(data *model.ClusterData) []DiscoveredApp {
	var apps []DiscoveredApp
	for _, hr := range data.HelmReleases {
		name := hr.Name
		chartName := hr.ChartName
		chartVersion := hr.Version
		helmRelease := hr.Name

		// Collect images for this helm release's namespace
		images := collectImagesForNamespace(data.Pods, hr.Namespace, hr.Cluster)

		apps = append(apps, DiscoveredApp{
			Name:         name,
			Namespace:    hr.Namespace,
			Cluster:      hr.Cluster,
			HelmRelease:  &helmRelease,
			ChartName:    &chartName,
			ChartVersion: &chartVersion,
			Images:       images,
		})
	}
	return apps
}

func mapStandaloneWorkloads(data *model.ClusterData, helmApps []DiscoveredApp) []DiscoveredApp {
	// Build set of namespaces already covered by HelmReleases
	helmNS := make(map[string]bool)
	for _, a := range helmApps {
		helmNS[a.Cluster+"/"+a.Namespace] = true
	}

	// Group workloads by app.kubernetes.io/name label, skip those in helm namespaces
	type wlGroup struct {
		name      string
		namespace string
		cluster   string
		kind      string
		images    []string
	}
	groups := make(map[string]*wlGroup)

	for _, w := range data.Workloads {
		key := w.Cluster + "/" + w.Namespace
		if helmNS[key] {
			continue
		}

		appName := w.Labels["app.kubernetes.io/name"]
		if appName == "" {
			appName = w.Name
		}

		groupKey := w.Cluster + "/" + w.Namespace + "/" + appName
		g, ok := groups[groupKey]
		if !ok {
			g = &wlGroup{name: appName, namespace: w.Namespace, cluster: w.Cluster, kind: w.Kind}
			groups[groupKey] = g
		}
		g.images = append(g.images, w.Images...)
	}

	var apps []DiscoveredApp
	for _, g := range groups {
		wlName := g.name
		wlKind := g.kind
		apps = append(apps, DiscoveredApp{
			Name:         g.name,
			Namespace:    g.namespace,
			Cluster:      g.cluster,
			WorkloadName: &wlName,
			WorkloadKind: &wlKind,
			Images:       dedup(g.images),
		})
	}
	return apps
}

func enrichWithVulns(apps []DiscoveredApp, vulns []model.ImageVuln) {
	vulnMap := make(map[string]*model.ImageVuln)
	for i := range vulns {
		vulnMap[vulns[i].Image] = &vulns[i]
	}

	for i := range apps {
		for _, img := range apps[i].Images {
			if v, ok := vulnMap[img]; ok {
				apps[i].VulnCritical += v.Critical
				apps[i].VulnHigh += v.High
			}
		}
	}
}

func collectImagesForNamespace(pods []model.PodImageInfo, namespace, cluster string) []string {
	seen := make(map[string]bool)
	var images []string
	for _, p := range pods {
		if p.Namespace == namespace {
			if !seen[p.Image] {
				seen[p.Image] = true
				images = append(images, p.Image)
			}
		}
	}
	return images
}

// BuildK8sSource creates a K8sSource from a DiscoveredApp.
func BuildK8sSource(appID store.Application, da DiscoveredApp) *store.K8sSource {
	return &store.K8sSource{
		AppID:        appID.ID,
		Cluster:      da.Cluster,
		Namespace:    da.Namespace,
		HelmRelease:  da.HelmRelease,
		WorkloadName: da.WorkloadName,
		WorkloadKind: da.WorkloadKind,
		ChartName:    da.ChartName,
		ChartVersion: da.ChartVersion,
		Images:       da.Images,
	}
}

func dedup(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// PrimaryImageTag extracts the main image tag from a list of images.
func PrimaryImageTag(images []string) *string {
	if len(images) == 0 {
		return nil
	}
	parts := strings.SplitN(images[0], ":", 2)
	if len(parts) == 2 {
		return &parts[1]
	}
	return nil
}
