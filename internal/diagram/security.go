package diagram

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// SecurityRow represents a single row in the security table.
type SecurityRow struct {
	Cluster     string `json:"cluster"`
	Namespace   string `json:"namespace"`
	Ingress     string `json:"ingress"`
	Ambient     string `json:"ambient"`
	MTLS        string `json:"mtls"`
	MTLSClient  string `json:"mtlsClient"`
	ExtAuth     string `json:"extAuth"`
	Backup      string `json:"backup"`
	PodSecurity string `json:"podSecurity"`
}

// GenerateSecurity produces a table diagram and a coverage pie chart.
func GenerateSecurity(data *model.ClusterData) []model.DiagramResult {
	if len(data.Namespaces) == 0 {
		return []model.DiagramResult{{
			ID:      "security",
			Title:   "Security Matrix",
			Type:    "markdown",
			Content: "*No namespace data available.*",
		}}
	}

	// Build set of namespaces with ext-auth policies (keyed by cluster/namespace)
	extAuthNS := make(map[string]bool)
	for _, sp := range data.SecurityPolicies {
		extAuthNS[sp.Cluster+"/"+sp.Namespace] = true
	}

	// Build client mTLS map: sectionName → optional from ClientTrafficPolicies
	ctpBySection := make(map[string]bool) // sectionName → optional
	for _, ctp := range data.ClientTrafficPolicies {
		ctpBySection[ctp.SectionName] = ctp.Optional
	}

	// Cross-reference HTTPRoutes with CTPs for client mTLS and ingress exposure
	// HTTPRoutes are only from the primary cluster
	ingressNS := make(map[string]bool)   // cluster/namespace → has HTTPRoute
	clientMTLS := make(map[string]string) // cluster/namespace → "yes"|"optional"
	for _, route := range data.HTTPRoutes {
		key := data.PrimaryCluster + "/" + route.Namespace
		ingressNS[key] = true

		if route.SectionName == "" {
			continue
		}
		optional, hasCTP := ctpBySection[route.SectionName]
		if !hasCTP {
			continue
		}
		if !optional {
			clientMTLS[key] = "yes"
		} else if clientMTLS[key] != "yes" {
			clientMTLS[key] = "optional"
		}
	}

	sorted := make([]model.NamespaceInfo, len(data.Namespaces))
	copy(sorted, data.Namespaces)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Cluster != sorted[j].Cluster {
			return sorted[i].Cluster < sorted[j].Cluster
		}
		return sorted[i].Name < sorted[j].Name
	})

	var rows []SecurityRow
	var ingressCount, ambientCount, mtlsCount, clientMTLSCount, authCount, backupCount int

	for _, ns := range sorted {
		nsKey := ns.Cluster + "/" + ns.Name
		cmtls := clientMTLS[nsKey]
		if cmtls == "" {
			cmtls = "no"
		}

		podSec := ns.PodSecurity
		if podSec == "" {
			podSec = "-"
		}

		if ingressNS[nsKey] {
			ingressCount++
		}
		if ns.Ambient {
			ambientCount++
		}
		if ns.MTLS {
			mtlsCount++
		}
		if cmtls == "yes" {
			clientMTLSCount++
		}
		if extAuthNS[nsKey] {
			authCount++
		}
		if ns.Backup {
			backupCount++
		}

		rows = append(rows, SecurityRow{
			Cluster:     ns.Cluster,
			Namespace:   ns.Name,
			Ingress:     boolIcon(ingressNS[nsKey]),
			Ambient:     boolIcon(ns.Ambient),
			MTLS:        boolIcon(ns.MTLS),
			MTLSClient:  cmtls,
			ExtAuth:     boolIcon(extAuthNS[nsKey]),
			Backup:      boolIcon(ns.Backup),
			PodSecurity: podSec,
		})
	}

	tableJSON, _ := json.Marshal(rows)

	// Coverage pie chart
	var b strings.Builder
	b.WriteString("pie title Security Coverage\n")
	b.WriteString(fmt.Sprintf("  \"Ingress\" : %d\n", ingressCount))
	b.WriteString(fmt.Sprintf("  \"Istio Ambient\" : %d\n", ambientCount))
	b.WriteString(fmt.Sprintf("  \"Velero Backup\" : %d\n", backupCount))
	b.WriteString(fmt.Sprintf("  \"Ext Auth\" : %d\n", authCount))
	b.WriteString(fmt.Sprintf("  \"mTLS Mesh\" : %d\n", mtlsCount))
	b.WriteString(fmt.Sprintf("  \"mTLS Client\" : %d\n", clientMTLSCount))

	return []model.DiagramResult{
		{
			ID:      "security",
			Title:   "Security Matrix",
			Type:    "table",
			Content: string(tableJSON),
		},
		{
			ID:      "security-chart",
			Title:   "Security Coverage",
			Type:    "mermaid",
			Content: b.String(),
		},
	}
}

func boolIcon(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
