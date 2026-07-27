package eam

import (
	"net/http"

	"github.com/google/uuid"
)

// Dependencies

func (h *Handler) listAppDependencies(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	deps, err := h.db.ListDependencies(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, deps)
}

// Components

func (h *Handler) listAppComponents(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	comps, err := h.db.ListAppComponents(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, comps)
}

// Capabilities

func (h *Handler) listAppCapabilities(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	caps, err := h.db.ListAppCapabilities(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, caps)
}

// K8s Sources

func (h *Handler) listAppK8sSources(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	sources, err := h.db.ListK8sSources(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, sources)
}
