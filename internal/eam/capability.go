package eam

import (
	"net/http"

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
