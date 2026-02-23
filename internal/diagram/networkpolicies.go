package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// NetworkPolicyRow represents a single row in the network policies table.
type NetworkPolicyRow struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	Cluster        string `json:"cluster"`
	PodSelector    string `json:"podSelector"`
	PolicyTypes    string `json:"policyTypes"`
	IngressSummary string `json:"ingressSummary"`
	EgressSummary  string `json:"egressSummary"`
}

// GenerateNetworkPolicies produces a table of Kubernetes NetworkPolicies.
func GenerateNetworkPolicies(data *model.ClusterData) model.DiagramResult {
	if len(data.NetworkPolicies) == 0 {
		return model.DiagramResult{
			ID:      "network-policies",
			Title:   "Network Policies",
			Type:    "markdown",
			Content: "*No network policy data available.*",
		}
	}

	var rows []NetworkPolicyRow
	for _, np := range data.NetworkPolicies {
		rows = append(rows, NetworkPolicyRow{
			Name:           np.Name,
			Namespace:      np.Namespace,
			Cluster:        np.Cluster,
			PodSelector:    np.PodSelector,
			PolicyTypes:    strings.Join(np.PolicyTypes, ", "),
			IngressSummary: np.IngressSummary,
			EgressSummary:  np.EgressSummary,
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
		ID:      "network-policies",
		Title:   "Network Policies",
		Type:    "table",
		Content: string(tableJSON),
	}
}
