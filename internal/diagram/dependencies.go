package diagram

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// extractLayer returns the real Flux layer directory name from a kustomization path.
// Path format: ./kubernetes/<cluster>/<layer>/... → returns "<layer>".
func extractLayer(path string) string {
	path = strings.TrimPrefix(path, "./")
	parts := strings.Split(path, "/")
	// kubernetes/<cluster>/<layer>
	if len(parts) >= 3 {
		return parts[2]
	}
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}
	return "unknown"
}

// FlowNode represents a node in the interactive flow diagram.
type FlowNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Cluster string `json:"cluster"`
	Layer   string `json:"layer"`
}

// FlowEdge represents an edge in the interactive flow diagram.
type FlowEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	CrossCluster bool   `json:"crossCluster,omitempty"`
}

// FlowData holds the complete flow diagram data.
type FlowData struct {
	Nodes []FlowNode `json:"nodes"`
	Edges []FlowEdge `json:"edges"`
}

// transitiveReduce removes redundant edges from a dependency graph.
// An edge A→B is redundant if there is a longer path A→...→B through other nodes.
func transitiveReduce(graph map[string]map[string]bool) map[string]map[string]bool {
	reduced := make(map[string]map[string]bool, len(graph))
	for node, deps := range graph {
		reduced[node] = make(map[string]bool, len(deps))
		for d := range deps {
			reduced[node][d] = true
		}
	}

	for node, deps := range graph {
		for dep := range deps {
			// DFS: can we reach dep from node without the direct edge?
			visited := make(map[string]bool)
			var stack []string
			for other := range deps {
				if other != dep {
					stack = append(stack, other)
				}
			}
			found := false
			for len(stack) > 0 && !found {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if cur == dep {
					found = true
					break
				}
				if !visited[cur] {
					visited[cur] = true
					for next := range graph[cur] {
						stack = append(stack, next)
					}
				}
			}
			if found {
				delete(reduced[node], dep)
			}
		}
	}
	return reduced
}

// discoverCrossClusterEdges finds implicit dependencies between clusters
// by inspecting MESH_EXTERNAL ServiceEntries with network labels.
//
// Algorithm:
//  1. Build cluster name set from Flux kustomizations.
//  2. Map network label → cluster name (strip "-network" suffix, case-insensitive match).
//  3. For each MESH_EXTERNAL SE with a network label, find best matching kustomizations
//     in source (consumer) and target (provider) clusters.
//  4. Create edge: target-kust → source-kust (provider before consumer).
//  5. Deduplicate bidirectional SEs.
func discoverCrossClusterEdges(data *model.ClusterData, idSet map[string]bool) []FlowEdge {
	// 1. Build cluster name set
	clusterNames := make(map[string]bool)
	for _, k := range data.Flux {
		clusterNames[k.Cluster] = true
	}

	// 2. Map network label → cluster name
	networkToCluster := make(map[string]string)
	for name := range clusterNames {
		// e.g. "nas-network" → "NAS", "homelab-network" → "Homelab"
		networkToCluster[strings.ToLower(name)+"-network"] = name
	}

	// Helper: find best kustomization in a cluster containing a service name
	findBestKust := func(cluster, svcName string) string {
		svcLower := strings.ToLower(svcName)
		var bestID string
		var bestScore int
		for _, k := range data.Flux {
			if k.Cluster != cluster {
				continue
			}
			nameLower := strings.ToLower(k.Name)
			if strings.Contains(nameLower, svcLower) {
				// Prefer exact or closer match (shorter name = more specific)
				score := 100 - len(nameLower)
				if score > bestScore || bestID == "" {
					bestScore = score
					bestID = k.Cluster + "/" + k.Name
				}
			}
		}
		// Fallback: look for "platform" in name
		if bestID == "" {
			for _, k := range data.Flux {
				if k.Cluster != cluster {
					continue
				}
				if strings.Contains(strings.ToLower(k.Name), "platform") {
					bestID = k.Cluster + "/" + k.Name
					break
				}
			}
		}
		return bestID
	}

	// 3. Process MESH_EXTERNAL ServiceEntries
	seen := make(map[string]bool) // deduplicate edges
	var edges []FlowEdge

	for _, se := range data.ServiceEntries {
		if se.Location != "MESH_EXTERNAL" || se.Network == "" {
			continue
		}

		targetCluster, ok := networkToCluster[strings.ToLower(se.Network)]
		if !ok {
			continue
		}

		sourceCluster := se.Cluster
		if sourceCluster == targetCluster {
			continue
		}

		// Extract service name from SE name by stripping target cluster prefix
		svcName := se.Name
		prefix := strings.ToLower(targetCluster) + "-"
		if strings.HasPrefix(strings.ToLower(svcName), prefix) {
			svcName = svcName[len(prefix):]
		}

		sourceKust := findBestKust(sourceCluster, svcName)
		targetKust := findBestKust(targetCluster, svcName)

		if sourceKust == "" || targetKust == "" {
			continue
		}
		if !idSet[sourceKust] || !idSet[targetKust] {
			continue
		}

		// Deduplicate: canonical key is sorted pair
		pairKey := targetKust + "→" + sourceKust
		if targetKust > sourceKust {
			pairKey = sourceKust + "→" + targetKust
		}
		if seen[pairKey] {
			continue
		}
		seen[pairKey] = true

		edgeID := fmt.Sprintf("xc:%s->%s", targetKust, sourceKust)
		edges = append(edges, FlowEdge{
			ID:           edgeID,
			Source:       targetKust,
			Target:       sourceKust,
			CrossCluster: true,
		})
	}

	return edges
}

// GenerateDependencies produces a JSON flow diagram of Flux Kustomization dependencies.
//
// Uses transitive reduction to remove redundant edges (e.g. if A→B→C exists,
// the direct A→C edge is dropped). Returns type "flow" with JSON content
// containing nodes and edges for @xyflow/react rendering.
func GenerateDependencies(data *model.ClusterData) model.DiagramResult {
	if len(data.Flux) == 0 {
		empty := FlowData{Nodes: []FlowNode{}, Edges: []FlowEdge{}}
		content, _ := json.Marshal(empty)
		return model.DiagramResult{
			ID:      "dependencies",
			Title:   "Flux Dependencies",
			Type:    "flow",
			Content: string(content),
		}
	}

	// Build node ID set. IDs use {Cluster}/{Name} to disambiguate cross-cluster.
	idSet := make(map[string]bool)
	for _, k := range data.Flux {
		idSet[k.Cluster+"/"+k.Name] = true
	}

	// Build dependency graph with cluster-qualified IDs
	depGraph := make(map[string]map[string]bool)
	for _, k := range data.Flux {
		id := k.Cluster + "/" + k.Name
		deps := make(map[string]bool)
		for _, d := range k.DependsOn {
			depID := k.Cluster + "/" + d
			if idSet[depID] {
				deps[depID] = true
			}
		}
		depGraph[id] = deps
	}

	// Transitive reduction
	reduced := transitiveReduce(depGraph)

	// Build nodes with real layer from path
	var nodes []FlowNode
	for _, k := range data.Flux {
		id := k.Cluster + "/" + k.Name
		nodes = append(nodes, FlowNode{
			ID:      id,
			Label:   k.Name,
			Cluster: k.Cluster,
			Layer:   extractLayer(k.Path),
		})
	}

	// Sort nodes for deterministic output
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// Build edges from reduced graph
	var edges []FlowEdge
	var allIDs []string
	for id := range depGraph {
		allIDs = append(allIDs, id)
	}
	sort.Strings(allIDs)

	for _, id := range allIDs {
		deps := reduced[id]
		var sortedDeps []string
		for d := range deps {
			sortedDeps = append(sortedDeps, d)
		}
		sort.Strings(sortedDeps)

		for _, dep := range sortedDeps {
			edges = append(edges, FlowEdge{
				ID:     fmt.Sprintf("%s->%s", dep, id),
				Source: dep,
				Target: id,
			})
		}
	}

	// Discover cross-cluster edges from ServiceEntries (skip transitive reduction for these)
	crossEdges := discoverCrossClusterEdges(data, idSet)
	edges = append(edges, crossEdges...)

	flowData := FlowData{Nodes: nodes, Edges: edges}
	content, _ := json.Marshal(flowData)

	return model.DiagramResult{
		ID:      "dependencies",
		Title:   "Flux Dependencies",
		Type:    "flow",
		Content: string(content),
	}
}
