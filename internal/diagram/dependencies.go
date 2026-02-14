package diagram

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// GenerateDependencies produces a Mermaid DAG of Flux Kustomization dependencies.
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

	b.WriteString("graph LR\n")

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

	for _, k := range data.Flux {
		layer := inferLayer(k.Path)
		if layer == "" {
			// Infer from graph position: no dependencies = Foundation
			if len(k.DependsOn) == 0 {
				layer = "Foundation"
			} else {
				layer = "Other"
			}
		}
		layers[layer] = append(layers[layer], k)
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
		b.WriteString(fmt.Sprintf("  subgraph %s[\"%s\"]\n", sgID, layer))
		for _, k := range kustomizations {
			nodeID := "k_" + sanitizeID(k.Name)
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, k.Name))
		}
		b.WriteString("  end\n")
	}

	// Render edges
	for _, k := range data.Flux {
		dst := "k_" + sanitizeID(k.Name)
		for _, dep := range k.DependsOn {
			if nameSet[dep] {
				src := "k_" + sanitizeID(dep)
				b.WriteString(fmt.Sprintf("  %s --> %s\n", src, dst))
			}
		}
	}

	return model.DiagramResult{
		ID:      "dependencies",
		Title:   "Flux Dependencies",
		Type:    "mermaid",
		Content: b.String(),
	}
}
