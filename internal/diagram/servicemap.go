package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// ServiceMapRow represents a service with its target workloads.
type ServiceMapRow struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Cluster    string `json:"cluster"`
	Type       string `json:"type"`
	Ports      string `json:"ports"`
	Targets    string `json:"targets"` // matched workload names
}

// GenerateServiceMap correlates Services with Workloads via selector matching.
func GenerateServiceMap(data *model.ClusterData) model.DiagramResult {
	if len(data.Services) == 0 {
		return model.DiagramResult{
			ID:      "service-map",
			Title:   "Service Mapping",
			Type:    "markdown",
			Content: "*No service data available.*",
		}
	}

	// Index workloads by cluster+namespace for selector matching
	type nsKey struct{ cluster, namespace string }
	workloadsByNS := make(map[nsKey][]model.WorkloadInfo)
	for _, w := range data.Workloads {
		key := nsKey{w.Cluster, w.Namespace}
		workloadsByNS[key] = append(workloadsByNS[key], w)
	}

	var rows []ServiceMapRow
	for _, svc := range data.Services {
		if len(svc.Selector) == 0 {
			continue
		}

		// Find workloads whose labels match the service selector
		key := nsKey{svc.Cluster, svc.Namespace}
		var matched []string
		for _, w := range workloadsByNS[key] {
			if labelsMatch(svc.Selector, w.Labels) {
				matched = append(matched, w.Kind+"/"+w.Name)
			}
		}
		sort.Strings(matched)

		rows = append(rows, ServiceMapRow{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Cluster:   svc.Cluster,
			Type:      svc.Type,
			Ports:     svc.Ports,
			Targets:   strings.Join(matched, ", "),
		})
	}

	if len(rows) == 0 {
		return model.DiagramResult{
			ID:      "service-map",
			Title:   "Service Mapping",
			Type:    "markdown",
			Content: "*No services with selectors found.*",
		}
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
		ID:      "service-map",
		Title:   "Service Mapping",
		Type:    "table",
		Content: string(tableJSON),
	}
}

// labelsMatch checks if all selector key/values exist in the labels map.
func labelsMatch(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
