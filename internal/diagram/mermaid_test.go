package diagram

import (
	"strings"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
)

func TestMermaidText(t *testing.T) {
	cases := map[string]string{
		`plain-name 10.0.0.1`:   `plain-name 10.0.0.1`,
		`say "hi"`:              `say #quot;hi#quot;`,
		`<img src=x onerror=1>`: `#lt;img src=x onerror=1#gt;`,
		"`md`":                  "#96;md#96;",
		`#quot; stays literal`:  `#35;quot; stays literal`,
		"two\nlines":            "two lines",
	}
	for in, want := range cases {
		if got := mermaidText(in); got != want {
			t.Errorf("mermaidText(%q) = %q, want %q", in, got, want)
		}
	}
}

// labelQuotesBalanced reports whether every line of a Mermaid flowchart has
// an even number of double quotes: an odd count is a label a free-text `"`
// cut short.
func labelQuotesBalanced(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.Count(line, `"`)%2 != 0 {
			return false
		}
	}
	return true
}

// Free text from docker-compose, tfstate and cluster objects must not be
// able to close a label and break the diagram.
func TestTopologyEscapesFreeText(t *testing.T) {
	evil := `a"b<i>`
	data := &model.ClusterData{
		InfraSources: []model.InfraSource{
			{Name: evil, Type: "docker-compose", DockerCompose: &model.DockerCompose{Services: []model.DockerService{
				{Name: "svc", Hostname: evil, Image: evil, IP: evil, Ports: []string{evil}, Volumes: []string{evil}},
			}}},
			{Name: evil, Type: "tfstate", TerraformNodes: []model.TerraformNode{
				{Name: evil, IP: evil, GPU: evil, Role: evil, Cores: 2},
			}},
		},
		Nodes:            []model.NodeInfo{{Name: "extra" + evil, CPU: evil, Memory: evil, IP: evil}},
		EastWestGateways: []model.EastWestGateway{{Name: "ew", IP: evil, Port: 15443, Network: evil}},
		ServiceEntries: []model.ServiceEntryInfo{
			{Name: "se", Hosts: []string{evil}, Location: "MESH_EXTERNAL", EndpointAddress: evil, Network: "remote" + evil},
		},
	}
	sections := GenerateTopologySections(data)
	sections = append(sections, generateK8sOnlyTopology(&model.ClusterData{
		Nodes: []model.NodeInfo{{Name: evil, CPU: evil, Memory: evil, IP: evil, Labels: map[string]string{"gpu": evil}}},
	}))
	for _, d := range sections {
		if !labelQuotesBalanced(d.Content) || strings.Contains(d.Content, "<i>") {
			t.Errorf("%s: free text not escaped:\n%s", d.ID, d.Content)
		}
	}
}
