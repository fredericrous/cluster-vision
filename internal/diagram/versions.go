package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// VersionRow represents a single row in the versions table.
type VersionRow struct {
	Cluster   string `json:"cluster"`
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
	Chart     string `json:"chart"`
	Version   string `json:"version"`
	Latest    string `json:"latest"`
	Outdated  bool   `json:"outdated"`
	RepoType     string `json:"repoType"`
	RepoURL      string `json:"repoUrl"`
	SecurityRisk string `json:"securityRisk"` // "critical" | "warning" | "none" | ""
	VulnSummary  string `json:"vulnSummary"`  // human-readable tooltip
}

// GenerateVersions produces a table of deployed HelmRelease versions.
func GenerateVersions(data *model.ClusterData, checker *versions.Checker) model.DiagramResult {
	if len(data.HelmReleases) == 0 {
		return model.DiagramResult{
			ID:      "charts",
			Title:   "Helm Charts",
			Type:    "markdown",
			Content: "*No HelmRelease data available.*",
		}
	}

	// Build repo lookup: "cluster/namespace/name" → HelmRepositoryInfo
	repoByKey := make(map[string]model.HelmRepositoryInfo)
	for _, r := range data.HelmRepositories {
		repoByKey[r.Cluster+"/"+r.Namespace+"/"+r.Name] = r
	}

	// Build vulnerability lookup: imageRef → ImageVuln
	vulnByImage := make(map[string]model.ImageVuln)
	for _, v := range data.ImageVulns {
		vulnByImage[v.Image] = v
	}

	// Build release → images mapping via workload labels
	// key: "cluster/namespace/releaseName" → set of image refs
	releaseImages := make(map[string]map[string]bool)
	for _, w := range data.Workloads {
		relName := w.Labels["app.kubernetes.io/instance"]
		if relName == "" {
			continue
		}
		key := w.Cluster + "/" + w.Namespace + "/" + relName
		if releaseImages[key] == nil {
			releaseImages[key] = make(map[string]bool)
		}
		for _, img := range w.Images {
			releaseImages[key][img] = true
		}
	}

	// Sort releases by cluster, namespace, then name
	sorted := make([]model.HelmReleaseInfo, len(data.HelmReleases))
	copy(sorted, data.HelmReleases)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Cluster != sorted[j].Cluster {
			return sorted[i].Cluster < sorted[j].Cluster
		}
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return sorted[i].Name < sorted[j].Name
	})

	var rows []VersionRow

	for _, rel := range sorted {
		repo := repoByKey[rel.Cluster+"/"+rel.RepoNS+"/"+rel.RepoName]
		repoType := repo.Type
		if repoType == "oci" {
			repoType = "OCI"
		} else if repoType != "" {
			repoType = "HTTP"
		} else {
			repoType = "-"
		}

		repoURL := repo.URL
		if repoURL == "" {
			repoURL = "-"
		}

		latest := "-"
		outdated := false
		if checker != nil {
			if v := checker.GetLatest(repo.URL, rel.ChartName); v != "" {
				latest = v
				if latest != rel.Version && rel.Version != "" {
					outdated = true
				}
			}
		}

		version := rel.Version
		if version == "" {
			version = "-"
		}

		// Aggregate security risk across all images in this release's workloads
		secRisk := ""
		vulnSum := ""
		relKey := rel.Cluster + "/" + rel.Namespace + "/" + rel.Name
		if images, ok := releaseImages[relKey]; ok {
			worstRisk := ""
			var summaryParts []string
			for img := range images {
				if v, ok := vulnByImage[img]; ok {
					r, s := vulnRisk(v)
					if worstRisk == "" || vulnRiskPriority(r) > vulnRiskPriority(worstRisk) {
						worstRisk = r
					}
					if s != "" {
						summaryParts = append(summaryParts, s)
					}
				}
			}
			secRisk = worstRisk
			vulnSum = strings.Join(summaryParts, "; ")
		}

		rows = append(rows, VersionRow{
			Cluster:      rel.Cluster,
			Release:      rel.Name,
			Namespace:    rel.Namespace,
			Chart:        rel.ChartName,
			Version:      version,
			Latest:       latest,
			Outdated:     outdated,
			RepoType:     repoType,
			RepoURL:      repoURL,
			SecurityRisk: secRisk,
			VulnSummary:  vulnSum,
		})
	}

	tableJSON, _ := json.Marshal(rows)

	return model.DiagramResult{
		ID:      "charts",
		Title:   "Helm Charts",
		Type:    "table",
		Content: string(tableJSON),
	}
}

// vulnRiskPriority returns a numeric priority for string risk levels (higher = worse).
func vulnRiskPriority(risk string) int {
	switch risk {
	case "critical":
		return 3
	case "warning":
		return 2
	case "none":
		return 1
	default:
		return 0
	}
}
