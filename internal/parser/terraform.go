package parser

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// tfState represents the top-level Terraform state structure.
type tfState struct {
	Version   int          `json:"version"`
	Resources []tfResource `json:"resources"`
}

type tfResource struct {
	Mode      string       `json:"mode"`
	Type      string       `json:"type"`
	Name      string       `json:"name"`
	Provider  string       `json:"provider"`
	Module    string       `json:"module"`
	Instances []tfInstance `json:"instances"`
}

type tfInstance struct {
	IndexKey   interface{}            `json:"index_key"`
	Attributes map[string]interface{} `json:"attributes"`
}

// ParseTerraformState reads a terraform.tfstate file and extracts VM nodes.
// Returns nil if the file doesn't exist or can't be parsed.
func ParseTerraformState(path string) []model.TerraformNode {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("no terraform state file found", "path", path)
		} else {
			slog.Warn("failed to read terraform state", "path", path, "error", err)
		}
		return nil
	}

	var state tfState
	if err := json.Unmarshal(data, &state); err != nil {
		slog.Warn("failed to parse terraform state", "error", err)
		return nil
	}

	var nodes []model.TerraformNode
	for _, res := range state.Resources {
		if res.Mode != "managed" {
			continue
		}
		switch res.Type {
		case "proxmox_vm_qemu":
			nodes = append(nodes, parseProxmoxTelmate(res)...)
		case "proxmox_virtual_environment_vm":
			nodes = append(nodes, parseProxmoxBPG(res)...)
		}
	}

	return nodes
}

// parseProxmoxTelmate handles VMs from the telmate/proxmox provider.
func parseProxmoxTelmate(res tfResource) []model.TerraformNode {
	var nodes []model.TerraformNode
	for _, inst := range res.Instances {
		a := inst.Attributes
		node := model.TerraformNode{
			Name:     strAttr(a, "name"),
			IP:       strAttr(a, "default_ipv4_address"),
			Cores:    intAttr(a, "cores"),
			MemoryMB: intAttr(a, "memory"),
			Provider: "proxmox",
		}

		// Infer role from resource name or tags
		node.Role = inferRole(res.Name, node.Name)

		// Disk sizes — telmate stores disks in a list
		if disks, ok := a["disk"].([]interface{}); ok {
			for i, d := range disks {
				if dm, ok := d.(map[string]interface{}); ok {
					size := intAttr(dm, "size")
					if i == 0 {
						node.OSDiskGB = size
					} else if i == 1 {
						node.DataDiskGB = size
					}
				}
			}
		}

		// GPU — check hostpci devices
		if hostpci, ok := a["hostpci"].([]interface{}); ok {
			for _, h := range hostpci {
				if hm, ok := h.(map[string]interface{}); ok {
					if device := strAttr(hm, "device"); device != "" {
						node.GPU = device
					}
				}
			}
		}

		// GPU from tags (e.g. "gpu=nvidia-rtx-4060")
		if tags := strAttr(a, "tags"); tags != "" {
			for _, tag := range strings.Split(tags, ";") {
				if strings.HasPrefix(tag, "gpu=") {
					node.GPU = strings.TrimPrefix(tag, "gpu=")
				}
			}
		}

		nodes = append(nodes, node)
	}
	return nodes
}

// parseProxmoxBPG handles VMs from the bpg/proxmox provider.
func parseProxmoxBPG(res tfResource) []model.TerraformNode {
	var nodes []model.TerraformNode
	for _, inst := range res.Instances {
		a := inst.Attributes
		node := model.TerraformNode{
			Name:     strAttr(a, "name"),
			Provider: "proxmox",
			Role:     inferRole(res.Name, strAttr(a, "name")),
		}

		// CPU
		if cpu, ok := a["cpu"].([]interface{}); ok && len(cpu) > 0 {
			if cpuMap, ok := cpu[0].(map[string]interface{}); ok {
				node.Cores = intAttr(cpuMap, "cores")
			}
		}

		// Memory
		if mem, ok := a["memory"].([]interface{}); ok && len(mem) > 0 {
			if memMap, ok := mem[0].(map[string]interface{}); ok {
				node.MemoryMB = intAttr(memMap, "dedicated")
			}
		}

		// IP — first IPv4 address
		if addrs, ok := a["ipv4_addresses"].([]interface{}); ok {
			for _, addrList := range addrs {
				if al, ok := addrList.([]interface{}); ok {
					for _, addr := range al {
						if s, ok := addr.(string); ok && s != "" && s != "127.0.0.1" {
							node.IP = s
							break
						}
					}
				}
				if node.IP != "" {
					break
				}
			}
		}

		// Disks
		if disks, ok := a["disk"].([]interface{}); ok {
			for i, d := range disks {
				if dm, ok := d.(map[string]interface{}); ok {
					size := intAttr(dm, "size")
					if i == 0 {
						node.OSDiskGB = size
					} else if i == 1 {
						node.DataDiskGB = size
					}
				}
			}
		}

		nodes = append(nodes, node)
	}
	return nodes
}

func inferRole(resourceName, vmName string) string {
	lower := strings.ToLower(resourceName + " " + vmName)
	if strings.Contains(lower, "controlplane") || strings.Contains(lower, "control-plane") ||
		strings.Contains(lower, "master") || strings.Contains(lower, "-cp-") {
		return "controlplane"
	}
	return "worker"
}

func strAttr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func intAttr(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return 0
}
