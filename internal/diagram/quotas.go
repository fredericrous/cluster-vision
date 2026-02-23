package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// QuotaRow represents a single row in the quotas table.
type QuotaRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Cluster   string `json:"cluster"`
	Kind      string `json:"kind"`
	Resources string `json:"resources"` // formatted key=value pairs
}

// GenerateQuotas produces a table of ResourceQuotas and LimitRanges.
func GenerateQuotas(data *model.ClusterData) model.DiagramResult {
	if len(data.Quotas) == 0 {
		return model.DiagramResult{
			ID:      "quotas",
			Title:   "Resource Quotas & Limits",
			Type:    "markdown",
			Content: "*No quota or limit range data available.*",
		}
	}

	var rows []QuotaRow
	for _, q := range data.Quotas {
		// Format resources as sorted key=value pairs
		var parts []string
		for k, v := range q.Resources {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)

		rows = append(rows, QuotaRow{
			Name:      q.Name,
			Namespace: q.Namespace,
			Cluster:   q.Cluster,
			Kind:      q.Kind,
			Resources: strings.Join(parts, ", "),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "quotas",
		Title:   "Resource Quotas & Limits",
		Type:    "table",
		Content: string(tableJSON),
	}
}
