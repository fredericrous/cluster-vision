package eam

import (
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

type GraphNode struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	DisplayName   *string   `json:"display_name"`
	Status        string    `json:"status"`
	TechnicalRisk string    `json:"technical_risk"`
	Criticality   string    `json:"criticality"`
	Namespace     string    `json:"namespace"`
	Cluster       string    `json:"cluster"`
	Capabilities  []string  `json:"capabilities"`
}

type GraphEdge struct {
	Source      uuid.UUID `json:"source"`
	Target      uuid.UUID `json:"target"`
	Description *string   `json:"description"`
}

func (h *Handler) getGraph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	apps, _, err := h.db.ListApplications(ctx, store.ApplicationFilter{Limit: 1000})
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	deps, err := h.db.AllDependencies(ctx)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	var nodes []GraphNode
	for _, app := range apps {
		k8s, _ := h.db.ListK8sSources(ctx, app.ID)
		caps, _ := h.db.ListAppCapabilities(ctx, app.ID)

		node := GraphNode{
			ID:            app.ID,
			Name:          app.Name,
			DisplayName:   app.DisplayName,
			Status:        app.Status,
			TechnicalRisk: app.TechnicalRisk,
			Criticality:   app.BusinessCriticality,
			Capabilities:  make([]string, 0, len(caps)),
		}

		if len(k8s) > 0 {
			node.Namespace = k8s[0].Namespace
			node.Cluster = k8s[0].Cluster
		}

		for _, c := range caps {
			node.Capabilities = append(node.Capabilities, c.Name)
		}

		nodes = append(nodes, node)
	}

	var edges []GraphEdge
	for _, d := range deps {
		edges = append(edges, GraphEdge{
			Source:      d.SourceAppID,
			Target:      d.TargetAppID,
			Description: d.Description,
		})
	}

	writeJSON(w, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}
