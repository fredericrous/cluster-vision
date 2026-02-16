package diagram

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// GenerateSecurity produces a markdown security matrix table and coverage pie chart.
func GenerateSecurity(data *model.ClusterData) model.DiagramResult {
	if len(data.Namespaces) == 0 {
		return model.DiagramResult{
			ID:      "security",
			Title:   "Security Matrix",
			Type:    "markdown",
			Content: "*No namespace data available.*",
		}
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

	var b strings.Builder

	// Table
	b.WriteString("| Cluster | Namespace | Ingress | Istio Ambient | mTLS | mTLS Client | Ext Auth | Backup | Pod Security |\n")
	b.WriteString("|---------|-----------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")

	var ingressCount, ambientCount, mtlsCount, clientMTLSCount, authCount, backupCount int

	for _, ns := range sorted {
		nsKey := ns.Cluster + "/" + ns.Name
		ambient := boolIcon(ns.Ambient)
		mtls := boolIcon(ns.MTLS)
		auth := boolIcon(extAuthNS[nsKey])
		backup := boolIcon(ns.Backup)
		podSec := ns.PodSecurity
		if podSec == "" {
			podSec = "-"
		}

		cmtls := clientMTLS[nsKey]
		if cmtls == "" {
			cmtls = "no"
		}

		ingress := boolIcon(ingressNS[nsKey])

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

		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			ns.Cluster, ns.Name, ingress, ambient, mtls, cmtls, auth, backup, podSec))
	}

	// Coverage pie chart
	b.WriteString("\n```mermaid\n")
	b.WriteString("pie title Security Coverage\n")
	b.WriteString(fmt.Sprintf("  \"Ingress\" : %d\n", ingressCount))
	b.WriteString(fmt.Sprintf("  \"Istio Ambient\" : %d\n", ambientCount))
	b.WriteString(fmt.Sprintf("  \"Velero Backup\" : %d\n", backupCount))
	b.WriteString(fmt.Sprintf("  \"Ext Auth\" : %d\n", authCount))
	b.WriteString(fmt.Sprintf("  \"mTLS Mesh\" : %d\n", mtlsCount))
	b.WriteString(fmt.Sprintf("  \"mTLS Client\" : %d\n", clientMTLSCount))
	b.WriteString("```\n")

	return model.DiagramResult{
		ID:      "security",
		Title:   "Security Matrix",
		Type:    "markdown",
		Content: b.String(),
	}
}

func boolIcon(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
