package diagram

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// NodeRow represents a single row in the cluster nodes table.
type NodeRow struct {
	Name             string `json:"name"`
	Cluster          string `json:"cluster"`
	Type             string `json:"type"`     // "node" | "load-balancer"
	Roles            string `json:"roles"`
	IP               string `json:"ip"`
	OS               string `json:"os"`
	OSVersion        string `json:"osVersion"`
	LatestOS         string `json:"latestOS"`
	OSOutdated       bool   `json:"osOutdated"`
	Kubelet          string `json:"kubelet"`
	LatestKubelet    string `json:"latestKubelet"`
	KubeletOutdated  bool   `json:"kubeletOutdated"`
	ContainerRuntime string `json:"containerRuntime"`
	Kernel           string `json:"kernel"`
	CPU              string `json:"cpu"`
	Memory           string `json:"memory"`
	Arch             string `json:"arch"`
	Provider         string `json:"provider"` // e.g. "proxmox"
	GPU              string `json:"gpu"`
	OSDisk           string `json:"osDisk"`   // e.g. "32 GB"
	DataDisk         string `json:"dataDisk"` // e.g. "100 GB"
}

// formatDiskGB formats a disk size in GB for display, omitting zero values.
func formatDiskGB(gb int) string {
	if gb == 0 {
		return ""
	}
	return fmt.Sprintf("%d GB", gb)
}

// GenerateNodes produces a table of cluster nodes with OS and kubelet version info,
// enriched with Terraform data and load-balancer entries.
func GenerateNodes(data *model.ClusterData, checker *versions.NodeChecker) model.DiagramResult {
	if len(data.Nodes) == 0 && len(data.EastWestGateways) == 0 {
		return model.DiagramResult{
			ID:      "nodes",
			Title:   "Cluster Nodes",
			Type:    "markdown",
			Content: "*No node data available.*",
		}
	}

	// Build TF lookup map keyed by node name.
	tfByName := make(map[string]model.TerraformNode)
	for _, src := range data.InfraSources {
		for _, tfn := range src.TerraformNodes {
			tfByName[tfn.Name] = tfn
		}
	}

	var rows []NodeRow
	for _, n := range data.Nodes {
		distro, osVer := versions.ParseOSImage(n.OSImage)

		latestOS := ""
		osOutdated := false
		if checker != nil {
			if v := checker.GetLatestOS(n.OSImage); v != "" {
				latestOS = v
				cleanLatest := strings.TrimPrefix(latestOS, "v")
				cleanCurrent := strings.TrimPrefix(osVer, "v")
				if cleanLatest != "" && cleanCurrent != "" && cleanLatest != cleanCurrent {
					osOutdated = true
				}
			}
		}

		latestKubelet := ""
		kubeletOutdated := false
		if checker != nil {
			if v := checker.GetLatestKubelet(n.KubeletVersion); v != "" {
				latestKubelet = v
				if latestKubelet != n.KubeletVersion {
					kubeletOutdated = true
				}
			}
		}

		osName := distro
		if osName == "" {
			osName = n.OSImage
		}

		row := NodeRow{
			Name:             n.Name,
			Cluster:          n.Cluster,
			Type:             "node",
			Roles:            strings.Join(n.Roles, ", "),
			IP:               n.IP,
			OS:               osName,
			OSVersion:        osVer,
			LatestOS:         latestOS,
			OSOutdated:       osOutdated,
			Kubelet:          n.KubeletVersion,
			LatestKubelet:    latestKubelet,
			KubeletOutdated:  kubeletOutdated,
			ContainerRuntime: n.ContainerRuntime,
			Kernel:           n.KernelVersion,
			CPU:              n.CPU,
			Memory:           n.Memory,
			Arch:             n.Architecture,
		}

		// Enrich with Terraform data.
		if tfn, ok := tfByName[n.Name]; ok {
			row.Provider = tfn.Provider
			row.GPU = tfn.GPU
			row.OSDisk = formatDiskGB(tfn.OSDiskGB)
			row.DataDisk = formatDiskGB(tfn.DataDiskGB)
		}

		// GPU fallback: check K8s node labels.
		if row.GPU == "" {
			if gpu, ok := n.Labels["gpu"]; ok && gpu != "" {
				row.GPU = gpu
			}
		}

		rows = append(rows, row)
	}

	// Append load-balancer entries from east-west gateways.
	for _, gw := range data.EastWestGateways {
		rows = append(rows, NodeRow{
			Name:    gw.Name,
			Cluster: data.PrimaryCluster,
			Type:    "load-balancer",
			Roles:   "load-balancer",
			IP:      gw.IP,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type != rows[j].Type {
			return rows[i].Type < rows[j].Type // "load-balancer" before "node"
		}
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)

	return model.DiagramResult{
		ID:      "nodes",
		Title:   "Cluster Nodes",
		Type:    "table",
		Content: string(tableJSON),
	}
}
