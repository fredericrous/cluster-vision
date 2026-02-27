package eam

import (
	"encoding/json"
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

func (h *Handler) listComponents(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")
	components, err := h.db.ListComponents(r.Context(), typeFilter)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, components)
}

func (h *Handler) getComponent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.db.GetComponent(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, c)
}

func (h *Handler) createComponent(w http.ResponseWriter, r *http.Request) {
	var c store.ITComponent
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if c.Name == "" || c.Type == "" {
		http.Error(w, `{"error":"name and type are required"}`, http.StatusBadRequest)
		return
	}
	if c.Status == "" {
		c.Status = "active"
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if err := h.db.CreateComponent(r.Context(), &c); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, c)
}

func (h *Handler) updateComponent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	existing, err := h.db.GetComponent(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var update store.ITComponent
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	existing.Name = update.Name
	existing.Type = update.Type
	existing.Version = update.Version
	existing.Provider = update.Provider
	existing.Description = update.Description
	existing.Status = update.Status
	if update.Tags != nil {
		existing.Tags = update.Tags
	}

	if err := h.db.UpdateComponent(r.Context(), existing); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, existing)
}

func (h *Handler) deleteComponent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteComponent(r.Context(), id); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
