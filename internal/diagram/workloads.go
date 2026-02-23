package diagram

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// WorkloadRow represents a single row in the workloads table.
type WorkloadRow struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	Cluster        string `json:"cluster"`
	Kind           string `json:"kind"`
	Replicas       string `json:"replicas"` // "ready/desired"
	UpdateStrategy string `json:"updateStrategy"`
	Images         string `json:"images"` // comma-separated
	Age            string `json:"age"`
}

// GenerateWorkloads produces a table of cluster workloads.
func GenerateWorkloads(data *model.ClusterData) model.DiagramResult {
	if len(data.Workloads) == 0 {
		return model.DiagramResult{
			ID:      "workloads",
			Title:   "Workloads",
			Type:    "markdown",
			Content: "*No workload data available.*",
		}
	}

	var rows []WorkloadRow
	for _, w := range data.Workloads {
		replicas := ""
		if w.Kind == "CronJob" {
			replicas = "-"
		} else {
			replicas = fmt.Sprintf("%d/%d", w.ReadyReplicas, w.Replicas)
		}

		rows = append(rows, WorkloadRow{
			Name:           w.Name,
			Namespace:      w.Namespace,
			Cluster:        w.Cluster,
			Kind:           w.Kind,
			Replicas:       replicas,
			UpdateStrategy: w.UpdateStrategy,
			Images:         strings.Join(w.Images, ", "),
			Age:            w.CreatedAt,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "workloads",
		Title:   "Workloads",
		Type:    "table",
		Content: string(tableJSON),
	}
}

