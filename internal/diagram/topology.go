package diagram

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/fredericrous/cluster-vision/internal/model"
)

var titleCaser = cases.Title(language.English)

// GenerateTopologySections produces one DiagramResult per InfraSource,
// falling back to a single K8s-only diagram if no sources are configured.
func GenerateTopologySections(data *model.ClusterData) []model.DiagramResult {
	if len(data.InfraSources) == 0 {
		return []model.DiagramResult{generateK8sOnlyTopology(data)}
	}

	var results []model.DiagramResult

	// Mesh topology first (east-west gateways + cross-cluster services)
	if mesh := generateMeshTopology(data); mesh != nil {
		results = append(results, *mesh)
	}

	for _, src := range data.InfraSources {
		id := "topology-" + sanitizeID(src.Name)
		switch src.Type {
		case "tfstate":
			results = append(results, generateTFSourceDiagram(id, src, data))
		case "docker-compose":
			results = append(results, generateDockerComposeDiagram(id, src))
		}
	}

	// Append K8s nodes not covered by any tfstate source
	if extra := extraK8sNodes(data); len(extra) > 0 {
		var b strings.Builder
		b.WriteString("graph TB\n")
		b.WriteString("  subgraph other[\"Other Kubernetes Nodes\"]\n")
		b.WriteString("    direction TB\n")
		for i, n := range extra {
			id := fmt.Sprintf("ex%d", i)
			label := fmt.Sprintf("%s<br/>%s / %s<br/>%s", n.Name, n.CPU, n.Memory, n.IP)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}
		b.WriteString("  end\n")
		results = append(results, model.DiagramResult{
			ID:      "topology-other",
			Title:   "Other Nodes",
			Type:    "mermaid",
			Content: b.String(),
		})
	}

	return results
}

func generateTFSourceDiagram(id string, src model.InfraSource, data *model.ClusterData) model.DiagramResult {
	var b strings.Builder
	b.WriteString("graph TB\n")
	b.WriteString(fmt.Sprintf("  subgraph cluster[\"%s\"]\n", src.Name))
	b.WriteString("    direction TB\n")

	for i, node := range src.TerraformNodes {
		nodeID := fmt.Sprintf("tf%d", i)
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

		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, label))
	}

	b.WriteString("  end\n")

	return model.DiagramResult{
		ID:      id,
		Title:   src.Name + " — Physical Topology",
		Type:    "mermaid",
		Content: b.String(),
	}
}

func generateDockerComposeDiagram(id string, src model.InfraSource) model.DiagramResult {
	dc := src.DockerCompose
	var b strings.Builder
	b.WriteString("graph TB\n")
	b.WriteString(fmt.Sprintf("  subgraph host[\"%s\"]\n", src.Name))
	b.WriteString("    direction TB\n")

	for i, svc := range dc.Services {
		svcID := fmt.Sprintf("svc%d", i)

		var details []string
		if svc.Image != "" {
			details = append(details, svc.Image)
		}
		if svc.IP != "" {
			details = append(details, svc.IP)
		}
		if len(svc.Ports) > 0 {
			details = append(details, "Ports: "+strings.Join(svc.Ports, ", "))
		}
		if svc.Privileged {
			details = append(details, "privileged")
		}

		hostname := svc.Hostname
		if hostname == "" {
			hostname = svc.Name
		}

		label := hostname
		if len(details) > 0 {
			label += "<br/>" + strings.Join(details, "<br/>")
		}
		if len(svc.Volumes) > 0 {
			// Show volume count to avoid overly long labels
			label += fmt.Sprintf("<br/>%d volume(s)", len(svc.Volumes))
		}

		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", svcID, label))
	}

	b.WriteString("  end\n")

	return model.DiagramResult{
		ID:      id,
		Title:   src.Name + " — Docker Compose",
		Type:    "mermaid",
		Content: b.String(),
	}
}

func generateK8sOnlyTopology(data *model.ClusterData) model.DiagramResult {
	var b strings.Builder
	b.WriteString("graph TB\n")

	if len(data.Nodes) == 0 {
		b.WriteString("  empty[\"No node information available\"]\n")
	} else {
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

			for k, v := range node.Labels {
				if strings.Contains(strings.ToLower(k), "gpu") {
					label += fmt.Sprintf("<br/>GPU: %s", v)
				}
			}

			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		}

		b.WriteString("  end\n")
	}

	return model.DiagramResult{
		ID:      "topology",
		Title:   "Physical Topology",
		Type:    "mermaid",
		Content: b.String(),
	}
}

func generateMeshTopology(data *model.ClusterData) *model.DiagramResult {
	// Filter to MESH_EXTERNAL service entries (cross-cluster)
	var crossCluster []model.ServiceEntryInfo
	for _, se := range data.ServiceEntries {
		if se.Location == "MESH_EXTERNAL" && se.Network != "" {
			crossCluster = append(crossCluster, se)
		}
	}

	if len(data.EastWestGateways) == 0 && len(crossCluster) == 0 {
		return nil
	}

	// Build network-to-name map from InfraSources
	networkName := func(network string) string {
		for _, src := range data.InfraSources {
			if sanitizeID(src.Name) == sanitizeID(network) || strings.EqualFold(src.Name, network) {
				return src.Name
			}
		}
		// Derive friendly name: "nas-network" → "NAS"
		name := strings.TrimSuffix(network, "-network")
		return strings.ToUpper(name)
	}

	// Determine local network from east-west gateways
	localNetwork := ""
	for _, gw := range data.EastWestGateways {
		localNetwork = gw.Network
		break
	}

	// Collect remote networks from service entries
	remoteNetworks := make(map[string]string) // network → gateway IP
	for _, se := range crossCluster {
		if se.Network != localNetwork {
			remoteNetworks[se.Network] = se.EndpointAddress
		}
	}

	var b strings.Builder
	b.WriteString("graph TB\n")

	hasLocalGW := len(data.EastWestGateways) > 0

	// Local cluster subgraph (only if gateways exist)
	if hasLocalGW {
		localName := networkName(localNetwork)
		if localName == "" {
			localName = "Local"
		}
		b.WriteString(fmt.Sprintf("  subgraph local[\"%s\"]\n", localName))
		for i, gw := range data.EastWestGateways {
			gwID := fmt.Sprintf("ewgw_l%d", i)
			label := fmt.Sprintf("East-West Gateway<br/>%s:%d", gw.IP, gw.Port)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", gwID, label))
		}
		b.WriteString("  end\n")
	}

	// Remote cluster subgraphs
	remoteIdx := 0
	remoteGwIDs := make(map[string]string) // network → mermaid ID
	for network, ip := range remoteNetworks {
		remoteName := networkName(network)
		subID := fmt.Sprintf("remote%d", remoteIdx)
		gwID := fmt.Sprintf("ewgw_r%d", remoteIdx)
		remoteGwIDs[network] = gwID

		b.WriteString(fmt.Sprintf("  subgraph %s[\"%s\"]\n", subID, remoteName))
		label := fmt.Sprintf("East-West Gateway<br/>%s:15443", ip)
		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", gwID, label))
		b.WriteString("  end\n")
		remoteIdx++
	}

	// mTLS tunnel links between local and remote gateways
	if hasLocalGW {
		for _, remoteGwID := range remoteGwIDs {
			b.WriteString(fmt.Sprintf("  ewgw_l0 <-->|\"mTLS tunnel<br/>port 15443\"| %s\n", remoteGwID))
		}
	}

	// Cross-cluster services subgraph
	if len(crossCluster) > 0 {
		b.WriteString("  subgraph xcluster[\"Cross-Cluster Services\"]\n")
		for i, se := range crossCluster {
			seID := fmt.Sprintf("se%d", i)
			host := strings.Join(se.Hosts, ", ")
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", seID, host))
		}
		b.WriteString("  end\n")

		// Arrows: local gateway → service → remote gateway
		for i, se := range crossCluster {
			seID := fmt.Sprintf("se%d", i)
			if hasLocalGW {
				b.WriteString(fmt.Sprintf("  ewgw_l0 --> %s\n", seID))
			}
			if rgw, ok := remoteGwIDs[se.Network]; ok {
				b.WriteString(fmt.Sprintf("  %s --> %s\n", seID, rgw))
			}
		}
	}

	return &model.DiagramResult{
		ID:      "topology-mesh",
		Title:   "Mesh Topology",
		Type:    "mermaid",
		Content: b.String(),
	}
}

// extraK8sNodes returns K8s nodes not present in any tfstate source.
func extraK8sNodes(data *model.ClusterData) []model.NodeInfo {
	tfNames := make(map[string]bool)
	for _, src := range data.InfraSources {
		for _, n := range src.TerraformNodes {
			tfNames[n.Name] = true
		}
	}
	var extra []model.NodeInfo
	for _, n := range data.Nodes {
		if !tfNames[n.Name] {
			extra = append(extra, n)
		}
	}
	return extra
}
