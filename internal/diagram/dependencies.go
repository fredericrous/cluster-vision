package diagram

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

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
			// BFS: can we reach dep from node without the direct edge?
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

// GenerateDependencies produces a Mermaid DAG of Flux Kustomization dependencies.
//
// Uses transitive reduction to remove redundant edges (e.g. if A→B→C exists,
// the direct A→C edge is dropped). Dependencies shared by most apps (like
// "security" and "data-storage") are elided from individual edges and noted
// in the Apps subgraph title.
func GenerateDependencies(data *model.ClusterData) model.DiagramResult {
	var b strings.Builder

	if len(data.Flux) == 0 {
		return model.DiagramResult{
			ID:      "dependencies",
			Title:   "Flux Dependencies",
			Type:    "mermaid",
			Content: "graph LR\n  empty[\"No Flux Kustomizations found\"]\n",
		}
	}

	b.WriteString("graph TD\n")

	// Categorize kustomizations into layers
	layers := map[string][]model.FluxKustomization{
		"Foundation": {},
		"Platform":   {},
		"Middleware": {},
		"Apps":       {},
		"Other":      {},
	}
	nameSet := make(map[string]bool)
	for _, k := range data.Flux {
		nameSet[k.Name] = true
	}

	appNames := make(map[string]bool)
	for _, k := range data.Flux {
		layer := inferLayer(k.Path)
		if layer == "" {
			if len(k.DependsOn) == 0 {
				layer = "Foundation"
			} else {
				layer = "Other"
			}
		}
		layers[layer] = append(layers[layer], k)
		if layer == "Apps" {
			appNames[k.Name] = true
		}
	}

	// Build dependency graph with only valid edges
	depGraph := make(map[string]map[string]bool)
	for _, k := range data.Flux {
		deps := make(map[string]bool)
		for _, d := range k.DependsOn {
			if nameSet[d] {
				deps[d] = true
			}
		}
		depGraph[k.Name] = deps
	}

	// Transitive reduction
	reduced := transitiveReduce(depGraph)

	// Identify common infra/middleware deps shared by >=5 apps.
	// These are elided from individual app edges and noted on the subgraph.
	appDepCounts := make(map[string]int)
	for name := range appNames {
		for dep := range reduced[name] {
			if !appNames[dep] {
				appDepCounts[dep]++
			}
		}
	}
	commonDeps := make(map[string]bool)
	for dep, count := range appDepCounts {
		if count >= 5 {
			commonDeps[dep] = true
		}
	}

	// Render subgraphs in order
	layerOrder := []string{"Foundation", "Platform", "Middleware", "Apps", "Other"}
	for _, layer := range layerOrder {
		kustomizations := layers[layer]
		if len(kustomizations) == 0 {
			continue
		}

		sort.Slice(kustomizations, func(i, j int) bool {
			return kustomizations[i].Name < kustomizations[j].Name
		})

		sgID := "sg_" + sanitizeID(strings.ToLower(layer))

		// Annotate Apps subgraph with common deps
		if layer == "Apps" && len(commonDeps) > 0 {
			var depList []string
			for d := range commonDeps {
				depList = append(depList, d)
			}
			sort.Strings(depList)
			b.WriteString(fmt.Sprintf("  subgraph %s[\"%s<br/><i>all apps depend on: %s</i>\"]\n",
				sgID, layer, strings.Join(depList, ", ")))
		} else {
			b.WriteString(fmt.Sprintf("  subgraph %s[\"%s\"]\n", sgID, layer))
		}

		for _, k := range kustomizations {
			nodeID := "k_" + sanitizeID(k.Name)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, k.Name))
		}
		b.WriteString("  end\n")
	}

	// Render edges — reduced graph, eliding common app→infra deps
	// Sort for deterministic output
	var allNames []string
	for name := range depGraph {
		allNames = append(allNames, name)
	}
	sort.Strings(allNames)

	for _, name := range allNames {
		deps := reduced[name]
		var sortedDeps []string
		for d := range deps {
			sortedDeps = append(sortedDeps, d)
		}
		sort.Strings(sortedDeps)

		dst := "k_" + sanitizeID(name)
		for _, dep := range sortedDeps {
			// Skip common deps for apps (noted on subgraph title)
			if appNames[name] && commonDeps[dep] {
				continue
			}
			src := "k_" + sanitizeID(dep)
			b.WriteString(fmt.Sprintf("  %s --> %s\n", src, dst))
		}
	}

	return model.DiagramResult{
		ID:      "dependencies",
		Title:   "Flux Dependencies",
		Type:    "mermaid",
		Content: b.String(),
	}
}
