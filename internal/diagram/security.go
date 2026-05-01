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

	// Build client mTLS map: cluster/sectionName → optional from ClientTrafficPolicies
	ctpBySection := make(map[string]bool)
	for _, ctp := range data.ClientTrafficPolicies {
		ctpBySection[ctp.Cluster+"/"+ctp.SectionName] = ctp.Optional
	}

	// Cross-reference HTTPRoutes with CTPs for client mTLS and ingress exposure
	ingressNS := make(map[string]bool)    // cluster/namespace → has HTTPRoute
	clientMTLS := make(map[string]string) // cluster/namespace → "yes"|"optional"
	for _, route := range data.HTTPRoutes {
		cluster := route.Cluster
		if cluster == "" {
			cluster = data.PrimaryCluster
		}
		key := cluster + "/" + route.Namespace
		ingressNS[key] = true

		if route.SectionName == "" {
			continue
		}
		optional, hasCTP := ctpBySection[cluster+"/"+route.SectionName]
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
	fmt.Fprintf(&b, "  \"Ingress\" : %d\n", ingressCount)
	fmt.Fprintf(&b, "  \"Istio Ambient\" : %d\n", ambientCount)
	fmt.Fprintf(&b, "  \"Velero Backup\" : %d\n", backupCount)
	fmt.Fprintf(&b, "  \"Ext Auth\" : %d\n", authCount)
	fmt.Fprintf(&b, "  \"mTLS Mesh\" : %d\n", mtlsCount)
	fmt.Fprintf(&b, "  \"mTLS Client\" : %d\n", clientMTLSCount)

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

// vulnRisk derives a security risk level and human-readable summary from vulnerability counts.
func vulnRisk(v model.ImageVuln) (risk, summary string) {
	if v.Critical > 0 || v.High > 0 {
		risk = "critical"
	} else if v.Medium > 0 || v.Low > 0 {
		risk = "warning"
	} else {
		risk = "none"
	}

	var parts []string
	if v.Critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", v.Critical))
	}
	if v.High > 0 {
		parts = append(parts, fmt.Sprintf("%d high", v.High))
	}
	if v.Medium > 0 {
		parts = append(parts, fmt.Sprintf("%d medium", v.Medium))
	}
	if v.Low > 0 {
		parts = append(parts, fmt.Sprintf("%d low", v.Low))
	}
	summary = strings.Join(parts, ", ")

	return risk, summary
}

// vulnExploitRisk maps KEV/EPSS enrichment to a UI badge tier.
//
// Tier rules (highest wins):
//   - "kev"       → at least one CVE listed on CISA KEV (actively exploited)
//   - "high-epss" → max EPSS > 0.5 (>50% probability of exploitation in 30d)
//   - "low-epss"  → max EPSS > 0.1 (some exploit signal, watch list)
//   - "none"      → zero KEV, EPSS <= 0.1 (or no enrichment data)
//
// Empty risk + summary means "no Trivy report at all" — caller decides
// whether to render or hide the column for that row.
func vulnExploitRisk(v model.ImageVuln) (risk, summary string) {
	switch {
	case v.KEVCount > 0:
		risk = "kev"
		first := ""
		if len(v.KEVCVEs) > 0 {
			first = v.KEVCVEs[0]
		}
		if v.KEVCount == 1 && first != "" {
			summary = fmt.Sprintf("CISA KEV: %s", first)
		} else {
			summary = fmt.Sprintf("%d KEV-listed CVEs (e.g. %s)", v.KEVCount, first)
		}
	case v.MaxEPSS > 0.5:
		risk = "high-epss"
		summary = fmt.Sprintf("EPSS %.2f — %s", v.MaxEPSS, v.MaxEPSSCVE)
	case v.MaxEPSS > 0.1:
		risk = "low-epss"
		summary = fmt.Sprintf("EPSS %.2f — %s", v.MaxEPSS, v.MaxEPSSCVE)
	default:
		risk = "none"
	}
	return risk, summary
}
