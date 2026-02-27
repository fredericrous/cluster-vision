package eam

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

func (h *Handler) listApplications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	f := store.ApplicationFilter{
		Status:  q.Get("status"),
		Risk:    q.Get("risk"),
		Cluster: q.Get("cluster"),
		Search:  q.Get("search"),
		Limit:   limit,
		Offset:  offset,
	}

	apps, total, err := h.db.ListApplications(r.Context(), f)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"items": apps,
		"total": total,
	})
}

func (h *Handler) getApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.db.GetApplication(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	if app == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	// Enrich with relationships
	deps, _ := h.db.ListDependencies(r.Context(), id)
	components, _ := h.db.ListAppComponents(r.Context(), id)
	capabilities, _ := h.db.ListAppCapabilities(r.Context(), id)
	k8s, _ := h.db.ListK8sSources(r.Context(), id)

	writeJSON(w, map[string]any{
		"application":  app,
		"dependencies": deps,
		"components":   components,
		"capabilities": capabilities,
		"k8s_sources":  k8s,
	})
}

func (h *Handler) createApplication(w http.ResponseWriter, r *http.Request) {
	var app store.Application
	if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if app.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	if app.Status == "" {
		app.Status = "active"
	}
	if app.BusinessCriticality == "" {
		app.BusinessCriticality = "medium"
	}
	if app.TechnicalRisk == "" {
		app.TechnicalRisk = "medium"
	}
	if app.LifecyclePhase == "" {
		app.LifecyclePhase = "active"
	}
	if app.Tags == nil {
		app.Tags = []string{}
	}
	app.DescriptionSource = "human-verified"
	app.BusinessCriticalitySource = "human-verified"
	app.TechnicalRiskSource = "human-verified"
	app.TimeCategorySource = "human-verified"
	app.ManualOverride = true

	if err := h.db.CreateApplication(r.Context(), &app); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, app)
}

func (h *Handler) updateApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.db.GetApplication(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var update store.Application
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Apply updates (preserve auto fields if not provided)
	existing.DisplayName = update.DisplayName
	existing.Description = update.Description
	existing.Status = update.Status
	existing.BusinessCriticality = update.BusinessCriticality
	existing.TechnicalRisk = update.TechnicalRisk
	existing.TechnicalRiskReasoning = update.TechnicalRiskReasoning
	existing.LifecyclePhase = update.LifecyclePhase
	existing.TimeCategory = update.TimeCategory
	existing.TimeCategoryReasoning = update.TimeCategoryReasoning
	existing.EndOfLifeDate = update.EndOfLifeDate
	if update.Tags != nil {
		existing.Tags = update.Tags
	}

	// Mark as human-verified
	existing.DescriptionSource = "human-verified"
	existing.BusinessCriticalitySource = "human-verified"
	existing.TechnicalRiskSource = "human-verified"
	existing.TimeCategorySource = "human-verified"
	existing.ManualOverride = true

	if err := h.db.UpdateApplication(r.Context(), existing); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, existing)
}

func (h *Handler) deleteApplication(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteApplication(r.Context(), id); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listAppVersionHistory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	q := r.URL.Query()
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			from = &t
		}
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			to = &t
		}
	}

	entries, err := h.db.GetVersionHistory(r.Context(), id, from, to)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, entries)
}
