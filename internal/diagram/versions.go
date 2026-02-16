package diagram

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// GenerateVersions produces a markdown table of deployed HelmRelease versions.
func GenerateVersions(data *model.ClusterData, checker *versions.Checker) model.DiagramResult {
	if len(data.HelmReleases) == 0 {
		return model.DiagramResult{
			ID:      "versions",
			Title:   "Component Versions",
			Type:    "markdown",
			Content: "*No HelmRelease data available.*",
		}
	}

	// Build repo lookup: "namespace/name" → HelmRepositoryInfo
	repoByKey := make(map[string]model.HelmRepositoryInfo)
	for _, r := range data.HelmRepositories {
		repoByKey[r.Namespace+"/"+r.Name] = r
	}

	// Sort releases by namespace, then name
	sorted := make([]model.HelmReleaseInfo, len(data.HelmReleases))
	copy(sorted, data.HelmReleases)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return sorted[i].Name < sorted[j].Name
	})

	var b strings.Builder

	b.WriteString("| Release | Namespace | Chart | Version | Latest | Repo Type | Repository |\n")
	b.WriteString("|---------|-----------|-------|---------|--------|-----------|------------|\n")

	var outdatedCount int

	for _, rel := range sorted {
		repo := repoByKey[rel.RepoNS+"/"+rel.RepoName]
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
		if checker != nil {
			if v := checker.GetLatest(repo.URL, rel.ChartName); v != "" {
				latest = v
				if latest != rel.Version && rel.Version != "" {
					latest = fmt.Sprintf("**%s**", latest)
					outdatedCount++
				}
			}
		}

		version := rel.Version
		if version == "" {
			version = "-"
		}

		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			rel.Name, rel.Namespace, rel.ChartName, version, latest, repoType, repoURL))
	}

	if outdatedCount > 0 {
		b.WriteString(fmt.Sprintf("\n*%d release(s) have newer versions available (shown in **bold**).*\n", outdatedCount))
	}

	return model.DiagramResult{
		ID:      "versions",
		Title:   "Component Versions",
		Type:    "markdown",
		Content: b.String(),
	}
}
