package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// VeleroRow represents a single row in the Velero schedules table.
type VeleroRow struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Cluster    string `json:"cluster"`
	Schedule   string `json:"schedule"`
	IncludedNS string `json:"includedNS"`
	ExcludedNS string `json:"excludedNS"`
	TTL        string `json:"ttl"`
	Phase      string `json:"phase"`
}

// GenerateVelero produces a table of Velero backup schedules.
func GenerateVelero(data *model.ClusterData) model.DiagramResult {
	if len(data.VeleroSchedules) == 0 {
		return model.DiagramResult{
			ID:      "velero",
			Title:   "Backup Schedules",
			Type:    "markdown",
			Content: "*No Velero schedule data available.*",
		}
	}

	var rows []VeleroRow
	for _, v := range data.VeleroSchedules {
		rows = append(rows, VeleroRow{
			Name:       v.Name,
			Namespace:  v.Namespace,
			Cluster:    v.Cluster,
			Schedule:   v.Schedule,
			IncludedNS: strings.Join(v.IncludedNS, ", "),
			ExcludedNS: strings.Join(v.ExcludedNS, ", "),
			TTL:        v.TTL,
			Phase:      v.Phase,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "velero",
		Title:   "Backup Schedules",
		Type:    "table",
		Content: string(tableJSON),
	}
}
