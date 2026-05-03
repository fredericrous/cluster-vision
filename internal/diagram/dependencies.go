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
	// Label is the Service name (Cilium global) or Istio host that
	// drives this edge. Empty for plain Flux dependency edges.
	Label string `json:"label,omitempty"`
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

		// Use the SE host (or first host) as the edge label so the same
		// kust pair can carry multiple distinct cross-cluster edges (e.g.
		// vault + nas-vault both flow Homelab→NAS).
		label := se.Name
		if len(se.Hosts) > 0 {
			label = se.Hosts[0]
		}
		// Dedup on (sorted-pair, label) so symmetric SEs collapse but
		// distinct hosts/labels keep their own edge.
		pairKey := targetKust + "→" + sourceKust
		if targetKust > sourceKust {
			pairKey = sourceKust + "→" + targetKust
		}
		dedupKey := pairKey + "|" + label
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		edgeID := fmt.Sprintf("xc-se:%s->%s:%s", targetKust, sourceKust, label)
		edges = append(edges, FlowEdge{
			ID:           edgeID,
			Source:       targetKust,
			Target:       sourceKust,
			CrossCluster: true,
			Label:        label,
		})
	}

	return edges
}

// discoverCiliumMeshEdges finds Services annotated
// `service.cilium.io/global=true` and pairs each with the matching
// Service of the same name+namespace in another cluster, producing one
// cross-cluster edge per pair labeled with `<name>.<namespace>`.
//
// Cilium ClusterMesh aggregates remote endpoints into the local
// (selectorless or stub) Service via a name+namespace match across
// clusters whenever both sides carry the global annotation, so any
// such pair represents a real cross-cluster traffic path independent
// of Istio. Each edge is keyed off the kustomization that owns the
// namespace on each side, using the same name-match heuristic as the
// Istio cross-cluster discovery.
func discoverCiliumMeshEdges(data *model.ClusterData, idSet map[string]bool) []FlowEdge {
	// Index global Services by namespace/name -> []ServiceInfo
	type svcKey struct{ ns, name string }
	byKey := make(map[svcKey][]model.ServiceInfo)
	for _, s := range data.Services {
		if s.Annotations["service.cilium.io/global"] != "true" {
			continue
		}
		k := svcKey{ns: s.Namespace, name: s.Name}
		byKey[k] = append(byKey[k], s)
	}

	// Reuse the per-name kust matcher from the SE path. Local copy to
	// avoid carrying a closure across helpers.
	findBestKust := func(cluster, hint string) string {
		hintLower := strings.ToLower(hint)
		var bestID string
		var bestScore int
		for _, k := range data.Flux {
			if k.Cluster != cluster {
				continue
			}
			nameLower := strings.ToLower(k.Name)
			if strings.Contains(nameLower, hintLower) {
				score := 100 - len(nameLower)
				if score > bestScore || bestID == "" {
					bestScore = score
					bestID = k.Cluster + "/" + k.Name
				}
			}
		}
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

	var edges []FlowEdge
	seen := make(map[string]bool)
	for k, peers := range byKey {
		if len(peers) < 2 {
			continue
		}
		// All ordered pairs (a, b) where a != b — gives one edge per
		// direction. Cilium-mesh consumers stub the Service locally, so
		// the directionality matches "consumer (stub) → provider (real
		// pods)" but we emit both for visibility; the dedup below
		// collapses the symmetric pair when both sides see each other.
		for i, a := range peers {
			for j, b := range peers {
				if i == j {
					continue
				}
				// Heuristic: provider is the one with a non-empty
				// selector (real backends); consumer is selectorless or
				// has `service.cilium.io/shared=false`.
				provider, consumer := a, b
				if len(a.Selector) == 0 && len(b.Selector) > 0 {
					provider, consumer = b, a
				}
				if provider.Cluster == consumer.Cluster {
					continue
				}
				pairKey := provider.Cluster + "→" + consumer.Cluster + ":" + k.ns + "/" + k.name
				if seen[pairKey] {
					continue
				}
				seen[pairKey] = true

				hint := k.name
				providerKust := findBestKust(provider.Cluster, hint)
				consumerKust := findBestKust(consumer.Cluster, hint)
				if providerKust == "" {
					providerKust = findBestKust(provider.Cluster, k.ns)
				}
				if consumerKust == "" {
					consumerKust = findBestKust(consumer.Cluster, k.ns)
				}
				if providerKust == "" || consumerKust == "" {
					continue
				}
				if !idSet[providerKust] || !idSet[consumerKust] {
					continue
				}
				label := k.name + "." + k.ns
				edges = append(edges, FlowEdge{
					ID:           fmt.Sprintf("xc-cm:%s->%s:%s", providerKust, consumerKust, label),
					Source:       providerKust,
					Target:       consumerKust,
					CrossCluster: true,
					Label:        label,
				})
			}
		}
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

	// Discover Cilium ClusterMesh per-Service edges (one per
	// `service.cilium.io/global=true` Service paired across clusters).
	ciliumEdges := discoverCiliumMeshEdges(data, idSet)
	edges = append(edges, ciliumEdges...)

	flowData := FlowData{Nodes: nodes, Edges: edges}
	content, _ := json.Marshal(flowData)

	return model.DiagramResult{
		ID:      "dependencies",
		Title:   "Flux Dependencies",
		Type:    "flow",
		Content: string(content),
	}
}
