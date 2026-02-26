package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// ConfigRow represents a single row in the ConfigMaps/Secrets table.
type ConfigRow struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Cluster      string `json:"cluster"`
	Kind         string `json:"kind"`
	KeyCount     int    `json:"keyCount"`
	ReferencedBy string `json:"referencedBy"` // comma-separated workload names
}

// GenerateConfigs produces a table of ConfigMaps and Secrets.
// It cross-references workloads to find which configs are in use.
func GenerateConfigs(data *model.ClusterData) model.DiagramResult {
	if len(data.Configs) == 0 {
		return model.DiagramResult{
			ID:      "configs",
			Title:   "ConfigMaps & Secrets",
			Type:    "markdown",
			Content: "*No configmap or secret data available.*",
		}
	}

	var rows []ConfigRow
	for _, c := range data.Configs {
		rows = append(rows, ConfigRow{
			Name:         c.Name,
			Namespace:    c.Namespace,
			Cluster:      c.Cluster,
			Kind:         c.Kind,
			KeyCount:     c.KeyCount,
			ReferencedBy: strings.Join(c.ReferencedBy, ", "),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "configs",
		Title:   "ConfigMaps & Secrets",
		Type:    "table",
		Content: string(tableJSON),
	}
}
