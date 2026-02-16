package diagram

import (
	"encoding/json"
	"sort"

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
	RepoType  string `json:"repoType"`
	RepoURL   string `json:"repoUrl"`
}

// GenerateVersions produces a table of deployed HelmRelease versions.
func GenerateVersions(data *model.ClusterData, checker *versions.Checker) model.DiagramResult {
	if len(data.HelmReleases) == 0 {
		return model.DiagramResult{
			ID:      "versions",
			Title:   "Component Versions",
			Type:    "markdown",
			Content: "*No HelmRelease data available.*",
		}
	}

	// Build repo lookup: "cluster/namespace/name" → HelmRepositoryInfo
	repoByKey := make(map[string]model.HelmRepositoryInfo)
	for _, r := range data.HelmRepositories {
		repoByKey[r.Cluster+"/"+r.Namespace+"/"+r.Name] = r
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

		rows = append(rows, VersionRow{
			Cluster:   rel.Cluster,
			Release:   rel.Name,
			Namespace: rel.Namespace,
			Chart:     rel.ChartName,
			Version:   version,
			Latest:    latest,
			Outdated:  outdated,
			RepoType:  repoType,
			RepoURL:   repoURL,
		})
	}

	tableJSON, _ := json.Marshal(rows)

	return model.DiagramResult{
		ID:      "versions",
		Title:   "Component Versions",
		Type:    "table",
		Content: string(tableJSON),
	}
}
