package diagram

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// FlowNode represents a node in the interactive flow diagram.
type FlowNode struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Cluster string `json:"cluster"`
	Layer   string `json:"layer"`
}

// FlowEdge represents an edge in the interactive flow diagram.
type FlowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
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

	// Build nodes with layer classification
	var nodes []FlowNode
	for _, k := range data.Flux {
		id := k.Cluster + "/" + k.Name
		layer := inferLayer(k.Path)
		if layer == "" {
			if len(k.DependsOn) == 0 {
				layer = "Foundation"
			} else {
				layer = "Uncategorized"
			}
		}
		nodes = append(nodes, FlowNode{
			ID:      id,
			Label:   k.Name,
			Cluster: k.Cluster,
			Layer:   layer,
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

	flowData := FlowData{Nodes: nodes, Edges: edges}
	content, _ := json.Marshal(flowData)

	return model.DiagramResult{
		ID:      "dependencies",
		Title:   "Flux Dependencies",
		Type:    "flow",
		Content: string(content),
	}
}
