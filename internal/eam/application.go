package eam

import (
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
