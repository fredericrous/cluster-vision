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

	// Build set of namespaces with ext-auth policies
	extAuthNS := make(map[string]bool)
	for _, sp := range data.SecurityPolicies {
		extAuthNS[sp.Namespace] = true
	}

	sorted := make([]model.NamespaceInfo, len(data.Namespaces))
	copy(sorted, data.Namespaces)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var b strings.Builder

	// Table
	b.WriteString("| Namespace | Istio Ambient | mTLS | Ext Auth | Backup | Pod Security |\n")
	b.WriteString("|-----------|:---:|:---:|:---:|:---:|:---:|\n")

	var ambientCount, mtlsCount, authCount, backupCount int

	for _, ns := range sorted {
		ambient := boolIcon(ns.Ambient)
		mtls := boolIcon(ns.MTLS)
		auth := boolIcon(extAuthNS[ns.Name])
		backup := boolIcon(ns.Backup)
		podSec := ns.PodSecurity
		if podSec == "" {
			podSec = "-"
		}

		if ns.Ambient {
			ambientCount++
		}
		if ns.MTLS {
			mtlsCount++
		}
		if extAuthNS[ns.Name] {
			authCount++
		}
		if ns.Backup {
			backupCount++
		}

		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			ns.Name, ambient, mtls, auth, backup, podSec))
	}

	// Coverage pie chart
	b.WriteString("\n```mermaid\n")
	b.WriteString("pie title Security Coverage\n")
	b.WriteString(fmt.Sprintf("  \"Istio Ambient\" : %d\n", ambientCount))
	b.WriteString(fmt.Sprintf("  \"Velero Backup\" : %d\n", backupCount))
	b.WriteString(fmt.Sprintf("  \"Ext Auth\" : %d\n", authCount))
	b.WriteString(fmt.Sprintf("  \"mTLS Sync\" : %d\n", mtlsCount))
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
