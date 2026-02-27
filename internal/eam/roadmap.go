package eam

import (
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
)

type RoadmapApplication struct {
	store.Application
	VersionHistory []store.VersionHistoryEntry `json:"version_history"`
}

func (h *Handler) getRoadmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	apps, _, err := h.db.ListApplications(ctx, store.ApplicationFilter{Limit: 1000})
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	var roadmap []RoadmapApplication
	for _, app := range apps {
		history, _ := h.db.GetVersionHistory(ctx, app.ID, nil, nil)
		roadmap = append(roadmap, RoadmapApplication{
			Application:    app,
			VersionHistory: history,
		})
	}

	writeJSON(w, roadmap)
}
