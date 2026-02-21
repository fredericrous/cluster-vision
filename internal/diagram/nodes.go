package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// NodeRow represents a single row in the cluster nodes table.
type NodeRow struct {
	Name             string `json:"name"`
	Cluster          string `json:"cluster"`
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
}

// GenerateNodes produces a table of cluster nodes with OS and kubelet version info.
func GenerateNodes(data *model.ClusterData, checker *versions.NodeChecker) model.DiagramResult {
	if len(data.Nodes) == 0 {
		return model.DiagramResult{
			ID:      "nodes",
			Title:   "Cluster Nodes",
			Type:    "markdown",
			Content: "*No node data available.*",
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
				// Compare: strip leading "v" for comparison
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

		rows = append(rows, NodeRow{
			Name:             n.Name,
			Cluster:          n.Cluster,
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
		})
	}

	sort.Slice(rows, func(i, j int) bool {
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
