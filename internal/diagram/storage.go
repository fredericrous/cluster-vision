package diagram

import (
	"encoding/json"
	"sort"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// StorageRow represents a single row in the storage inventory table.
type StorageRow struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Cluster       string `json:"cluster"`
	Kind          string `json:"kind"`
	Capacity      string `json:"capacity"`
	AccessModes   string `json:"accessModes"`
	Status        string `json:"status"`
	StorageClass  string `json:"storageClass"`
	ReclaimPolicy string `json:"reclaimPolicy"`
	BoundTo       string `json:"boundTo"`
}

// GenerateStorage produces a table of storage resources.
func GenerateStorage(data *model.ClusterData) model.DiagramResult {
	if len(data.Storage) == 0 {
		return model.DiagramResult{
			ID:      "storage",
			Title:   "Storage",
			Type:    "markdown",
			Content: "*No storage data available.*",
		}
	}

	var rows []StorageRow
	for _, s := range data.Storage {
		rows = append(rows, StorageRow{
			Name:          s.Name,
			Namespace:     s.Namespace,
			Cluster:       s.Cluster,
			Kind:          s.Kind,
			Capacity:      s.Capacity,
			AccessModes:   s.AccessModes,
			Status:        s.Status,
			StorageClass:  s.StorageClass,
			ReclaimPolicy: s.ReclaimPolicy,
			BoundTo:       s.BoundTo,
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
		ID:      "storage",
		Title:   "Storage",
		Type:    "table",
		Content: string(tableJSON),
	}
}
