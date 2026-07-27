package eam

import (
	"net/http"

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
