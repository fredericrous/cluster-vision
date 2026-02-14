package diagram

import (
	"fmt"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// GenerateNetwork produces a Mermaid diagram of external ingress routing.
func GenerateNetwork(data *model.ClusterData) model.DiagramResult {
	var b strings.Builder

	if len(data.Gateways) == 0 && len(data.HTTPRoutes) == 0 {
		return model.DiagramResult{
			ID:      "network",
			Title:   "Network & Ingress",
			Type:    "mermaid",
			Content: "graph LR\n  empty[\"No Gateway or HTTPRoute resources found\"]\n",
		}
	}

	b.WriteString("graph LR\n")
	b.WriteString("  internet((\"Internet\"))\n")

	// One subgraph per gateway
	for gi, gw := range data.Gateways {
		gwID := fmt.Sprintf("gw%d", gi)
		b.WriteString(fmt.Sprintf("  %s{\"%s<br/>%s\"}\n", gwID, gw.Name, gw.Namespace))
		b.WriteString(fmt.Sprintf("  internet -->|HTTPS| %s\n\n", gwID))

		// Build hostname → listener mapping
		hostToListener := make(map[string]string)
		for _, l := range gw.Listeners {
			if l.Hostname != "" {
				hostToListener[l.Hostname] = l.Name
			}
		}

		// Group routes by namespace for visual clarity
		nsByRoute := make(map[string][]model.HTTPRouteInfo)
		for _, r := range data.HTTPRoutes {
			// Only include routes targeting this gateway
			nsByRoute[r.Namespace] = append(nsByRoute[r.Namespace], r)
		}

		// Collect routes that match this gateway's listeners
		var matched []model.HTTPRouteInfo
		for _, r := range data.HTTPRoutes {
			for _, h := range r.Hostnames {
				if _, ok := hostToListener[h]; ok {
					matched = append(matched, r)
					break
				}
			}
		}

		// Render route nodes and edges
		seen := make(map[string]bool)
		for _, r := range matched {
			routeID := sanitizeID(r.Namespace + "_" + r.Name)
			if seen[routeID] {
				continue
			}
			seen[routeID] = true

			hostname := ""
			if len(r.Hostnames) > 0 {
				hostname = r.Hostnames[0]
			}

			label := r.Name
			if hostname != "" {
				label = fmt.Sprintf("%s<br/><small>%s</small>", r.Name, hostname)
			}

			b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", routeID, label))

			edgeLabel := hostname
			if edgeLabel == "" {
				edgeLabel = r.Name
			}
			b.WriteString(fmt.Sprintf("  %s -->|\"%s\"| %s\n", gwID, edgeLabel, routeID))
		}
	}

	// Routes not matching any gateway (standalone)
	if len(data.Gateways) == 0 {
		for _, r := range data.HTTPRoutes {
			routeID := sanitizeID(r.Namespace + "_" + r.Name)
			hostname := ""
			if len(r.Hostnames) > 0 {
				hostname = r.Hostnames[0]
			}
			label := fmt.Sprintf("%s<br/><small>%s</small>", r.Name, hostname)
			b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", routeID, label))
		}
	}

	return model.DiagramResult{
		ID:      "network",
		Title:   "Network & Ingress",
		Type:    "mermaid",
		Content: b.String(),
	}
}
