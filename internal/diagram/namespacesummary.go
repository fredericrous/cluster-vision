package diagram

import (
	"encoding/json"
	"sort"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// NamespaceSummaryRow aggregates resource counts per namespace.
type NamespaceSummaryRow struct {
	Namespace       string `json:"namespace"`
	Cluster         string `json:"cluster"`
	Workloads       int    `json:"workloads"`
	Services        int    `json:"services"`
	ConfigMaps      int    `json:"configMaps"`
	Secrets         int    `json:"secrets"`
	PVCs            int    `json:"pvcs"`
	Certificates    int    `json:"certificates"`
	NetworkPolicies int    `json:"networkPolicies"`
	HelmReleases    int    `json:"helmReleases"`
}

// GenerateNamespaceSummary aggregates all ClusterData per namespace.
func GenerateNamespaceSummary(data *model.ClusterData) model.DiagramResult {
	type nsKey struct{ cluster, namespace string }
	counts := make(map[nsKey]*NamespaceSummaryRow)

	ensure := func(cluster, ns string) *NamespaceSummaryRow {
		key := nsKey{cluster, ns}
		if r, ok := counts[key]; ok {
			return r
		}
		r := &NamespaceSummaryRow{Namespace: ns, Cluster: cluster}
		counts[key] = r
		return r
	}

	for _, w := range data.Workloads {
		ensure(w.Cluster, w.Namespace).Workloads++
	}
	for _, s := range data.Services {
		ensure(s.Cluster, s.Namespace).Services++
	}
	for _, c := range data.Configs {
		r := ensure(c.Cluster, c.Namespace)
		if c.Kind == "ConfigMap" {
			r.ConfigMaps++
		} else {
			r.Secrets++
		}
	}
	for _, s := range data.Storage {
		if s.Kind == "PersistentVolumeClaim" && s.Namespace != "" {
			ensure(s.Cluster, s.Namespace).PVCs++
		}
	}
	for _, c := range data.Certificates {
		ensure(c.Cluster, c.Namespace).Certificates++
	}
	for _, np := range data.NetworkPolicies {
		ensure(np.Cluster, np.Namespace).NetworkPolicies++
	}
	for _, hr := range data.HelmReleases {
		ensure(hr.Cluster, hr.Namespace).HelmReleases++
	}

	var rows []NamespaceSummaryRow
	for _, r := range counts {
		rows = append(rows, *r)
	}

	if len(rows) == 0 {
		return model.DiagramResult{
			ID:      "namespace-summary",
			Title:   "Namespace Summary",
			Type:    "markdown",
			Content: "*No namespace data available.*",
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		return rows[i].Namespace < rows[j].Namespace
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "namespace-summary",
		Title:   "Namespace Summary",
		Type:    "table",
		Content: string(tableJSON),
	}
}
