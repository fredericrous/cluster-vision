package diagram

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// HelmWorkloadRow represents a Helm release with its managed workloads.
type HelmWorkloadRow struct {
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
	Cluster   string `json:"cluster"`
	Workload  string `json:"workload"`
	Kind      string `json:"kind"`
	Replicas  string `json:"replicas"`
}

// GenerateHelmWorkloads correlates HelmReleases with the Workloads they
// installed (HelmReleaseInfo.Owns).
func GenerateHelmWorkloads(data *model.ClusterData) model.DiagramResult {
	if len(data.HelmReleases) == 0 || len(data.Workloads) == 0 {
		return model.DiagramResult{
			ID:      "helm-workloads",
			Title:   "Helm to Workloads",
			Type:    "markdown",
			Content: "*No Helm release or workload data available for correlation.*",
		}
	}

	var rows []HelmWorkloadRow
	for _, hr := range data.HelmReleases {
		var workloads []model.WorkloadInfo
		for _, w := range data.Workloads {
			if hr.Owns(w) {
				workloads = append(workloads, w)
			}
		}

		if len(workloads) == 0 {
			rows = append(rows, HelmWorkloadRow{
				Release:   hr.Name,
				Namespace: hr.Namespace,
				Cluster:   hr.Cluster,
				Workload:  "(none found)",
				Kind:      "",
				Replicas:  "",
			})
			continue
		}

		for _, w := range workloads {
			replicas := ""
			if w.Kind == "CronJob" {
				replicas = "-"
			} else {
				replicas = fmt.Sprintf("%d/%d", w.ReadyReplicas, w.Replicas)
			}
			rows = append(rows, HelmWorkloadRow{
				Release:   hr.Name,
				Namespace: hr.Namespace,
				Cluster:   hr.Cluster,
				Workload:  w.Name,
				Kind:      w.Kind,
				Replicas:  replicas,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if rows[i].Release != rows[j].Release {
			return rows[i].Release < rows[j].Release
		}
		return rows[i].Workload < rows[j].Workload
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "helm-workloads",
		Title:   "Helm to Workloads",
		Type:    "table",
		Content: string(tableJSON),
	}
}
