package eam

import (
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

type LandscapeCapability struct {
	ID       uuid.UUID              `json:"id"`
	Name     string                 `json:"name"`
	Level    int                    `json:"level"`
	Children []LandscapeCapability  `json:"children"`
	Apps     []LandscapeApplication `json:"apps"`
}

type LandscapeApplication struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	DisplayName   *string   `json:"display_name"`
	Status        string    `json:"status"`
	TechnicalRisk string    `json:"technical_risk"`
	VulnCritical  int       `json:"vuln_critical"`
	VulnHigh      int       `json:"vuln_high"`
}

func (h *Handler) getLandscape(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get capability tree
	tree, err := h.db.GetCapabilityTree(ctx)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	// Get all applications with their capabilities
	apps, _, err := h.db.ListApplications(ctx, store.ApplicationFilter{Limit: 1000})
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	// Build app-to-capabilities map
	appCaps := make(map[uuid.UUID][]uuid.UUID)
	for _, app := range apps {
		caps, _ := h.db.ListAppCapabilities(ctx, app.ID)
		for _, c := range caps {
			appCaps[app.ID] = append(appCaps[app.ID], c.ID)
		}
	}

	// Get latest vuln data per app from version_history
	appVulns := make(map[uuid.UUID][2]int) // [critical, high]
	for _, app := range apps {
		entries, _ := h.db.GetVersionHistory(ctx, app.ID, nil, nil)
		if len(entries) > 0 {
			appVulns[app.ID] = [2]int{entries[0].VulnCritical, entries[0].VulnHigh}
		}
	}

	// Build landscape capabilities with mapped apps
	var landscape []LandscapeCapability
	for _, cap := range tree {
		lc := buildLandscapeCapability(cap, apps, appCaps, appVulns)
		landscape = append(landscape, lc)
	}

	// Also include unmapped apps
	var unmapped []LandscapeApplication
	for _, app := range apps {
		if len(appCaps[app.ID]) == 0 {
			vulns := appVulns[app.ID]
			unmapped = append(unmapped, LandscapeApplication{
				ID:            app.ID,
				Name:          app.Name,
				DisplayName:   app.DisplayName,
				Status:        app.Status,
				TechnicalRisk: app.TechnicalRisk,
				VulnCritical:  vulns[0],
				VulnHigh:      vulns[1],
			})
		}
	}

	writeJSON(w, map[string]any{
		"capabilities": landscape,
		"unmapped":     unmapped,
	})
}

func buildLandscapeCapability(cap store.CapabilityTreeNode, apps []store.Application, appCaps map[uuid.UUID][]uuid.UUID, appVulns map[uuid.UUID][2]int) LandscapeCapability {
	lc := LandscapeCapability{
		ID:       cap.ID,
		Name:     cap.Name,
		Level:    cap.Level,
		Children: []LandscapeCapability{},
		Apps:     []LandscapeApplication{},
	}

	for _, app := range apps {
		for _, capID := range appCaps[app.ID] {
			if capID == cap.ID {
				vulns := appVulns[app.ID]
				lc.Apps = append(lc.Apps, LandscapeApplication{
					ID:            app.ID,
					Name:          app.Name,
					DisplayName:   app.DisplayName,
					Status:        app.Status,
					TechnicalRisk: app.TechnicalRisk,
					VulnCritical:  vulns[0],
					VulnHigh:      vulns[1],
				})
				break
			}
		}
	}

	for _, child := range cap.Children {
		lc.Children = append(lc.Children, buildLandscapeCapability(child, apps, appCaps, appVulns))
	}

	return lc
}
