package diagram

import (
	"fmt"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// GenerateNetwork produces a Mermaid diagram of external ingress routing.
//
// A route hangs off every ingress Gateway it attaches to: the Gateways its
// parentRefs name (namespace and sectionName included), or — for data
// recorded before parentRefs were — the Gateways with a listener whose
// hostname intersects the route's. Routes that attach to no ingress Gateway
// (mesh routes on a waypoint, a typo'd parentRef) are drawn in a group of
// their own rather than dropped.
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

	clusterOf := func(c string) string {
		if c == "" {
			return data.PrimaryCluster
		}
		return c
	}

	fmt.Fprint(&b, "graph LR\n")
	fmt.Fprint(&b, "  internet((\"Internet\"))\n")

	attached := make([]bool, len(data.HTTPRoutes))
	declared := make(map[string]bool)
	declare := func(r model.HTTPRouteInfo, indent string) string {
		id := routeNodeID(clusterOf(r.Cluster), r.Namespace, r.Name)
		if !declared[id] {
			declared[id] = true
			fmt.Fprintf(&b, "%s%s[\"%s\"]\n", indent, id, routeLabel(r, clusterOf(r.Cluster)))
		}
		return id
	}

	// One node per ingress gateway. Waypoints and east-west gateways are
	// mesh-internal: nothing reaches them from the Internet.
	for gi, gw := range data.Gateways {
		if meshInternalGatewayClasses[gw.GatewayClassName] {
			continue
		}
		gwID := fmt.Sprintf("gw%d", gi)
		gwCluster := clusterOf(gw.Cluster)
		fmt.Fprintf(&b, "  %s{\"%s<br/>%s<br/>%s\"}\n", gwID, mermaidText(gw.Name), mermaidText(gw.Namespace), mermaidText(gwCluster))
		fmt.Fprintf(&b, "  internet -->|HTTPS| %s\n\n", gwID)

		for ri, r := range data.HTTPRoutes {
			if clusterOf(r.Cluster) != gwCluster {
				continue
			}
			listener, ok := routeAttaches(r, gw)
			if !ok {
				continue
			}
			attached[ri] = true
			routeID := declare(r, "  ")

			edgeLabel := r.Name
			switch {
			case len(r.Hostnames) > 0:
				edgeLabel = r.Hostnames[0]
			case listener != "":
				edgeLabel = listener
			}
			fmt.Fprintf(&b, "  %s -->|\"%s\"| %s\n", gwID, mermaidText(edgeLabel), routeID)
		}
	}

	// Routes no ingress Gateway picked up.
	var unattached []model.HTTPRouteInfo
	for ri, r := range data.HTTPRoutes {
		if !attached[ri] {
			unattached = append(unattached, r)
		}
	}
	if len(unattached) > 0 {
		b.WriteString("  subgraph unattached[\"Not attached to an ingress Gateway\"]\n")
		for _, r := range unattached {
			declare(r, "    ")
		}
		b.WriteString("  end\n")
	}

	return model.DiagramResult{
		ID:      "network",
		Title:   "Network & Ingress",
		Type:    "mermaid",
		Content: b.String(),
	}
}

var meshInternalGatewayClasses = map[string]bool{
	"istio-waypoint":  true,
	"istio-east-west": true,
}

func routeLabel(r model.HTTPRouteInfo, cluster string) string {
	if len(r.Hostnames) > 0 {
		return fmt.Sprintf("%s<br/><small>%s</small><br/><small>%s</small>", mermaidText(r.Name), mermaidText(r.Hostnames[0]), mermaidText(cluster))
	}
	return fmt.Sprintf("%s<br/><small>%s</small>", mermaidText(r.Name), mermaidText(cluster))
}

// routeAttaches reports whether r attaches to gw, and through which
// listener when one is singled out. The caller has matched the cluster.
func routeAttaches(r model.HTTPRouteInfo, gw model.GatewayInfo) (listener string, ok bool) {
	if len(r.ParentRefs) == 0 {
		// Recorded before parentRefs were: the first ref's sectionName, if
		// any, and the hostnames are all there is to go on.
		return listenerFor(r, gw, r.SectionName)
	}
	for _, pr := range r.ParentRefs {
		if pr.Group != "" && pr.Group != "gateway.networking.k8s.io" {
			continue
		}
		if pr.Kind != "" && pr.Kind != "Gateway" {
			continue // a mesh Service parent, not an ingress Gateway
		}
		ns := pr.Namespace
		if ns == "" {
			ns = r.Namespace
		}
		if pr.Name != gw.Name || ns != gw.Namespace {
			continue
		}
		if l, ok := listenerFor(r, gw, pr.SectionName); ok {
			return l, true
		}
	}
	return "", false
}

// listenerFor finds a listener of gw (the one named section, when given)
// whose hostname intersects the route's. A gateway recorded without
// listeners accepts the route: there is nothing to contradict it.
func listenerFor(r model.HTTPRouteInfo, gw model.GatewayInfo, section string) (string, bool) {
	if len(gw.Listeners) == 0 {
		return section, true
	}
	for _, l := range gw.Listeners {
		if section != "" && l.Name != section {
			continue
		}
		if hostnamesIntersect(l.Hostname, r.Hostnames) {
			return l.Name, true
		}
	}
	return "", false
}

// hostnamesIntersect applies the Gateway API rule: a listener without a
// hostname, or a route without hostnames, matches anything; otherwise some
// route hostname must match the listener's, either side possibly a
// "*." wildcard standing for one or more labels.
func hostnamesIntersect(listener string, routeHosts []string) bool {
	if listener == "" || len(routeHosts) == 0 {
		return true
	}
	for _, h := range routeHosts {
		if hostnameMatch(listener, h) {
			return true
		}
	}
	return false
}

func hostnameMatch(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return true
	}
	wildcardCovers := func(wild, host string) bool {
		suffix, ok := strings.CutPrefix(wild, "*")
		return ok && strings.HasPrefix(suffix, ".") && strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return wildcardCovers(a, b) || wildcardCovers(b, a)
}

// routeNodeID is a Mermaid node ID unique per cluster/namespace/name.
// sanitizeID alone is not: "foo-bar"/"x" and "foo"/"bar-x" both became
// foo_bar_x, and the cluster was left out. Each part keeps its letters and
// digits and spells every other byte as _hh; parts are joined by "__",
// which no encoded part contains.
func routeNodeID(cluster, namespace, name string) string {
	return "route_" + idPart(cluster) + "__" + idPart(namespace) + "__" + idPart(name)
}

func idPart(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "_%02x", c)
		}
	}
	return b.String()
}
