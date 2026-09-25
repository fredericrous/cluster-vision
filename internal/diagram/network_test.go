package diagram

import (
	"strings"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
)

func gwRef(name, section string) model.ParentRef {
	return model.ParentRef{Group: "gateway.networking.k8s.io", Kind: "Gateway", Namespace: "gw-system", Name: name, SectionName: section}
}

// edgesTo returns the gateway IDs with an edge to routeID.
func edgesTo(content, routeID string) []string {
	var from []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "| "+routeID) {
			from = append(from, strings.Fields(line)[0])
		}
	}
	return from
}

func TestNetworkAttachesRoutesByParentRef(t *testing.T) {
	data := &model.ClusterData{
		PrimaryCluster: "home",
		Gateways: []model.GatewayInfo{
			{Name: "public", Namespace: "gw-system", Cluster: "home", GatewayClassName: "eg", Listeners: []model.ListenerInfo{
				{Name: "http", Protocol: "HTTP", Port: 80},
				{Name: "git-https", Hostname: "git.example.com", Protocol: "HTTPS", Port: 443},
				{Name: "preview", Hostname: "*.preview.example.com", Protocol: "HTTPS", Port: 443},
			}},
			{Name: "internal", Namespace: "gw-system", Cluster: "home", GatewayClassName: "eg", Listeners: []model.ListenerInfo{
				{Name: "git-internal", Hostname: "git.example.com", Protocol: "HTTPS", Port: 443},
			}},
			{Name: "waypoint", Namespace: "app", Cluster: "home", GatewayClassName: "istio-waypoint"},
			{Name: "eastwest", Namespace: "istio-system", Cluster: "home", GatewayClassName: "istio-east-west"},
		},
		HTTPRoutes: []model.HTTPRouteInfo{
			// Same hostname as a listener of both gateways; parentRef decides.
			{Name: "git", Namespace: "git", Cluster: "home", Hostnames: []string{"git.example.com"}, ParentRefs: []model.ParentRef{gwRef("public", "git-https")}},
			{Name: "git-internal", Namespace: "git", Cluster: "home", Hostnames: []string{"git.example.com"}, ParentRefs: []model.ParentRef{gwRef("internal", "")}},
			// Wildcard route on a listener without a hostname.
			{Name: "redirect", Namespace: "gw-system", Cluster: "home", Hostnames: []string{"*.example.com"}, ParentRefs: []model.ParentRef{{Name: "public", SectionName: "http"}}},
			// A preview host under the wildcard listener.
			{Name: "pr-42", Namespace: "previews", Cluster: "home", Hostnames: []string{"pr-42.preview.example.com"}, ParentRefs: []model.ParentRef{gwRef("public", "")}},
			// Mesh route on a waypoint, and a route naming a missing gateway.
			{Name: "mesh", Namespace: "app", Cluster: "home", ParentRefs: []model.ParentRef{{Group: "", Kind: "Service", Name: "api"}}},
			{Name: "typo", Namespace: "app", Cluster: "home", Hostnames: []string{"git.example.com"}, ParentRefs: []model.ParentRef{gwRef("pubic", "")}},
		},
	}
	out := GenerateNetwork(data).Content
	if strings.Contains(out, "waypoint<br/>") || strings.Contains(out, "eastwest<br/>") {
		t.Errorf("mesh-internal gateways drawn as ingress:\n%s", out)
	}

	want := map[string][]string{
		routeNodeID("home", "git", "git"):            {"gw0"},
		routeNodeID("home", "git", "git-internal"):   {"gw1"},
		routeNodeID("home", "gw-system", "redirect"): {"gw0"},
		routeNodeID("home", "previews", "pr-42"):     {"gw0"},
		routeNodeID("home", "app", "mesh"):           nil,
		routeNodeID("home", "app", "typo"):           nil,
	}
	for id, gws := range want {
		if got := edgesTo(out, id); strings.Join(got, ",") != strings.Join(gws, ",") {
			t.Errorf("%s: edges from %v, want %v\n%s", id, got, gws, out)
		}
		if strings.Count(out, "    "+id+"[") == 0 && strings.Count(out, "  "+id+"[") == 0 {
			t.Errorf("%s not drawn:\n%s", id, out)
		}
	}
	unattached := out[strings.Index(out, "subgraph unattached"):]
	for _, name := range []string{"mesh", "typo"} {
		if !strings.Contains(unattached, routeNodeID("home", "app", name)+"[") {
			t.Errorf("route %s missing from the unattached group:\n%s", name, out)
		}
	}
}

// Routes recorded before parentRefs were parsed fall back to hostnames,
// wildcards and hostname-less listeners included.
func TestNetworkFallsBackToHostnames(t *testing.T) {
	data := &model.ClusterData{
		Gateways: []model.GatewayInfo{
			{Name: "gw", Namespace: "gw-system", Listeners: []model.ListenerInfo{{Name: "preview", Hostname: "*.preview.example.com"}}},
			{Name: "catchall", Namespace: "gw-system", Listeners: []model.ListenerInfo{{Name: "http"}}},
		},
		HTTPRoutes: []model.HTTPRouteInfo{
			{Name: "pr", Namespace: "p", Hostnames: []string{"pr.preview.example.com"}, SectionName: "preview"},
			{Name: "apex", Namespace: "p", Hostnames: []string{"preview.example.com"}, SectionName: "preview"},
		},
	}
	out := GenerateNetwork(data).Content
	if got := edgesTo(out, routeNodeID("", "p", "pr")); strings.Join(got, ",") != "gw0" {
		t.Errorf("pr: edges from %v, want gw0\n%s", got, out)
	}
	// "*.preview.example.com" does not cover the apex, and the section
	// pins it to that listener, so the catch-all does not take it either.
	if got := edgesTo(out, routeNodeID("", "p", "apex")); len(got) != 0 {
		t.Errorf("apex: edges from %v, want none\n%s", got, out)
	}
	if !strings.Contains(out, "subgraph unattached") {
		t.Errorf("apex route dropped:\n%s", out)
	}
}

func TestRouteNodeIDIsCollisionFree(t *testing.T) {
	ids := map[string]string{}
	for _, parts := range [][3]string{
		{"home", "foo-bar", "x"},
		{"home", "foo", "bar-x"},
		{"home", "foo_bar", "x"},
		{"nas", "foo-bar", "x"},
		{"home", "foo", "bar_x"},
		{"home", "fo", "o-bar-x"},
	} {
		id := routeNodeID(parts[0], parts[1], parts[2])
		if prev, dup := ids[id]; dup {
			t.Fatalf("%v and %s share ID %s", parts, prev, id)
		}
		ids[id] = strings.Join(parts[:], "/")
		if sanitizeID(id) != id {
			t.Fatalf("ID %q is not a plain Mermaid identifier", id)
		}
	}
}

func TestHostnameMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"git.example.com", "git.example.com", true},
		{"GIT.example.com", "git.EXAMPLE.com", true},
		{"*.example.com", "git.example.com", true},
		{"*.example.com", "a.b.example.com", true},
		{"*.example.com", "example.com", false},
		{"git.example.com", "*.example.com", true},
		{"*.example.com", "*.a.example.com", true},
		{"*.example.com", "git.example.org", false},
		{"*example.com", "badexample.com", false},
	}
	for _, c := range cases {
		if got := hostnameMatch(c.a, c.b); got != c.want {
			t.Errorf("hostnameMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
