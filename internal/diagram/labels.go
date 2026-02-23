package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// LabelRow represents a unique label key with aggregated stats.
type LabelRow struct {
	Key            string `json:"key"`
	DistinctValues int    `json:"distinctValues"`
	ResourceCount  int    `json:"resourceCount"`
	ResourceKinds  string `json:"resourceKinds"`
}

// GenerateLabels harvests labels from all parsed resources and produces a taxonomy table.
func GenerateLabels(data *model.ClusterData) model.DiagramResult {
	type labelStats struct {
		values map[string]bool
		count  int
		kinds  map[string]bool
	}

	stats := make(map[string]*labelStats)

	record := func(labels map[string]string, kind string) {
		for k, v := range labels {
			s, ok := stats[k]
			if !ok {
				s = &labelStats{
					values: make(map[string]bool),
					kinds:  make(map[string]bool),
				}
				stats[k] = s
			}
			s.values[v] = true
			s.count++
			s.kinds[kind] = true
		}
	}

	for _, n := range data.Nodes {
		record(n.Labels, "Node")
	}
	for _, w := range data.Workloads {
		record(w.Labels, w.Kind)
	}
	for _, s := range data.Services {
		record(s.Selector, "Service")
	}

	if len(stats) == 0 {
		return model.DiagramResult{
			ID:      "labels",
			Title:   "Labels & Annotations",
			Type:    "markdown",
			Content: "*No label data available.*",
		}
	}

	var rows []LabelRow
	for key, s := range stats {
		kinds := sortedKeys(s.kinds)
		rows = append(rows, LabelRow{
			Key:            key,
			DistinctValues: len(s.values),
			ResourceCount:  s.count,
			ResourceKinds:  strings.Join(kinds, ", "),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Key < rows[j].Key
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "labels",
		Title:   "Labels & Annotations",
		Type:    "table",
		Content: string(tableJSON),
	}
}
