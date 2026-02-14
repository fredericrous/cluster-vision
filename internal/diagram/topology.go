package diagram

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/fredericrous/cluster-vision/internal/model"
)

var titleCaser = cases.Title(language.English)

// GenerateTopology produces a Mermaid graph of the physical infrastructure.
// Uses Terraform state for detailed VM specs, falls back to K8s node info.
func GenerateTopology(data *model.ClusterData) model.DiagramResult {
	var b strings.Builder
	b.WriteString("graph TB\n")

	hasTF := len(data.TerraformNodes) > 0

	if hasTF {
		writeTerraformTopology(&b, data)
	} else if len(data.Nodes) > 0 {
		writeKubernetesTopology(&b, data)
	} else {
		b.WriteString("  empty[\"No node information available\"]\n")
	}

	return model.DiagramResult{
		ID:      "topology",
		Title:   "Physical Topology",
		Type:    "mermaid",
		Content: b.String(),
	}
}

func writeTerraformTopology(b *strings.Builder, data *model.ClusterData) {
	// Group nodes by provider
	b.WriteString("  subgraph cluster[\"Kubernetes Cluster\"]\n")
	b.WriteString("    direction TB\n")

	for i, node := range data.TerraformNodes {
		id := fmt.Sprintf("tf%d", i)
		memGB := float64(node.MemoryMB) / 1024.0

		var details []string
		if node.Cores > 0 {
			details = append(details, fmt.Sprintf("%d cores", node.Cores))
		}
		if memGB > 0 {
			details = append(details, fmt.Sprintf("%.1f GB RAM", memGB))
		}
		if node.OSDiskGB > 0 {
			details = append(details, fmt.Sprintf("OS: %d GB", node.OSDiskGB))
		}
		if node.DataDiskGB > 0 {
			details = append(details, fmt.Sprintf("Data: %d GB", node.DataDiskGB))
		}
		if node.GPU != "" {
			details = append(details, fmt.Sprintf("GPU: %s", node.GPU))
		}

		role := node.Role
		if role == "" {
			role = "worker"
		}

		label := fmt.Sprintf("%s<br/>%s<br/>%s",
			node.Name,
			titleCaser.String(role),
			strings.Join(details, " / "),
		)
		if node.IP != "" {
			label += "<br/>" + node.IP
		}

		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
	}

	b.WriteString("  end\n")

	// Add K8s nodes not in terraform (e.g. NAS cluster nodes)
	tfNames := make(map[string]bool)
	for _, n := range data.TerraformNodes {
		tfNames[n.Name] = true
	}
	var extra []model.NodeInfo
	for _, n := range data.Nodes {
		if !tfNames[n.Name] {
			extra = append(extra, n)
		}
	}
	if len(extra) > 0 {
		b.WriteString("\n  subgraph other[\"Other Nodes\"]\n")
		for i, n := range extra {
			id := fmt.Sprintf("ex%d", i)
			label := fmt.Sprintf("%s<br/>%s / %s<br/>%s", n.Name, n.CPU, n.Memory, n.IP)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
		b.WriteString("  end\n")
	}
}

func writeKubernetesTopology(b *strings.Builder, data *model.ClusterData) {
	b.WriteString("  subgraph cluster[\"Kubernetes Cluster\"]\n")
	b.WriteString("    direction TB\n")

	for i, node := range data.Nodes {
		id := fmt.Sprintf("n%d", i)
		role := "Worker"
		for _, r := range node.Roles {
			if r == "control-plane" || r == "master" {
				role = "Control Plane"
				break
			}
		}

		label := fmt.Sprintf("%s<br/>%s<br/>CPU: %s / Mem: %s<br/>%s",
			node.Name, role, node.CPU, node.Memory, node.IP)

		// Check for GPU label
		for k, v := range node.Labels {
			if strings.Contains(strings.ToLower(k), "gpu") {
				label += fmt.Sprintf("<br/>GPU: %s", v)
			}
		}

		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
	}

	b.WriteString("  end\n")
}
