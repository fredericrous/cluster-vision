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

	fmt.Fprint(&b, "graph LR\n")
	fmt.Fprint(&b, "  internet((\"Internet\"))\n")

	// One subgraph per gateway (skip mesh-internal waypoints)
	for gi, gw := range data.Gateways {
		if gw.GatewayClassName == "istio-waypoint" {
			continue
		}
		gwID := fmt.Sprintf("gw%d", gi)
		clusterLabel := gw.Cluster
		if clusterLabel == "" {
			clusterLabel = data.PrimaryCluster
		}
		fmt.Fprintf(&b, "  %s{\"%s<br/>%s<br/>%s\"}\n", gwID, gw.Name, gw.Namespace, clusterLabel)
		fmt.Fprintf(&b, "  internet -->|HTTPS| %s\n\n", gwID)

		// Build hostname → listener mapping
		hostToListener := make(map[string]string)
		for _, l := range gw.Listeners {
			if l.Hostname != "" {
				hostToListener[l.Hostname] = l.Name
			}
		}

		// Collect routes that match this gateway's listeners in the same cluster.
		var matched []model.HTTPRouteInfo
		for _, r := range data.HTTPRoutes {
			if r.Cluster != "" && gw.Cluster != "" && r.Cluster != gw.Cluster {
				continue
			}
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
			routeCluster := r.Cluster
			if routeCluster == "" {
				routeCluster = data.PrimaryCluster
			}

			var label string
			if hostname != "" {
				label = fmt.Sprintf("%s<br/><small>%s</small><br/><small>%s</small>", r.Name, hostname, routeCluster)
			} else {
				label = fmt.Sprintf("%s<br/><small>%s</small>", r.Name, routeCluster)
			}

			fmt.Fprintf(&b, "  %s[\"%s\"]\n", routeID, label)

			edgeLabel := hostname
			if edgeLabel == "" {
				edgeLabel = r.Name
			}
			fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", gwID, edgeLabel, routeID)
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
			fmt.Fprintf(&b, "  %s[\"%s\"]\n", routeID, label)
		}
	}

	return model.DiagramResult{
		ID:      "network",
		Title:   "Network & Ingress",
		Type:    "mermaid",
		Content: b.String(),
	}
}
