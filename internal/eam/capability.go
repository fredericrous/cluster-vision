package eam

import (
	"encoding/json"
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

func (h *Handler) listCapabilities(w http.ResponseWriter, r *http.Request) {
	caps, err := h.db.ListCapabilities(r.Context())
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, caps)
}

func (h *Handler) getCapabilityTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.db.GetCapabilityTree(r.Context())
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, tree)
}

func (h *Handler) getCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.db.GetCapability(r.Context(), id)
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

func (h *Handler) createCapability(w http.ResponseWriter, r *http.Request) {
	var c store.BusinessCapability
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if c.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if c.Level == 0 {
		c.Level = 1
	}
	if err := h.db.CreateCapability(r.Context(), &c); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, c)
}

func (h *Handler) updateCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	existing, err := h.db.GetCapability(r.Context(), id)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var update store.BusinessCapability
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	existing.Name = update.Name
	existing.Description = update.Description
	existing.ParentID = update.ParentID
	existing.Level = update.Level
	existing.SortOrder = update.SortOrder

	if err := h.db.UpdateCapability(r.Context(), existing); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, existing)
}

func (h *Handler) deleteCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteCapability(r.Context(), id); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
