package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// CRDRow represents a single row in the CRD inventory table.
type CRDRow struct {
	Name     string `json:"name"`
	Group    string `json:"group"`
	Versions string `json:"versions"`
	Scope    string `json:"scope"`
	Cluster  string `json:"cluster"`
}

// GenerateCRDs produces a table of installed CustomResourceDefinitions.
func GenerateCRDs(data *model.ClusterData) model.DiagramResult {
	if len(data.CRDs) == 0 {
		return model.DiagramResult{
			ID:      "crds",
			Title:   "Custom Resource Definitions",
			Type:    "markdown",
			Content: "*No CRD data available.*",
		}
	}

	var rows []CRDRow
	for _, c := range data.CRDs {
		rows = append(rows, CRDRow{
			Name:     c.Name,
			Group:    c.Group,
			Versions: strings.Join(c.Versions, ", "),
			Scope:    c.Scope,
			Cluster:  c.Cluster,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Group != rows[j].Group {
			return rows[i].Group < rows[j].Group
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "crds",
		Title:   "Custom Resource Definitions",
		Type:    "table",
		Content: string(tableJSON),
	}
}
