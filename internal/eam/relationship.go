package eam

import (
	"encoding/json"
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
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

func (h *Handler) addAppDependency(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		TargetAppID uuid.UUID `json:"target_app_id"`
		Description *string   `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	dep := &store.AppDependency{
		SourceAppID: id,
		TargetAppID: body.TargetAppID,
		Description: body.Description,
	}
	if err := h.db.AddDependency(r.Context(), dep); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, dep)
}

func (h *Handler) removeAppDependency(w http.ResponseWriter, r *http.Request) {
	sourceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid source id"}`, http.StatusBadRequest)
		return
	}
	targetID, err := uuid.Parse(r.PathValue("targetId"))
	if err != nil {
		http.Error(w, `{"error":"invalid target id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.RemoveDependency(r.Context(), sourceID, targetID); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *Handler) linkAppComponent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		ComponentID uuid.UUID `json:"component_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.LinkAppComponent(r.Context(), id, body.ComponentID); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) unlinkAppComponent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	componentID, err := uuid.Parse(r.PathValue("componentId"))
	if err != nil {
		http.Error(w, `{"error":"invalid component id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.UnlinkAppComponent(r.Context(), id, componentID); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *Handler) linkAppCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		CapabilityID uuid.UUID `json:"capability_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.LinkAppCapability(r.Context(), id, body.CapabilityID); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) unlinkAppCapability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	capabilityID, err := uuid.Parse(r.PathValue("capabilityId"))
	if err != nil {
		http.Error(w, `{"error":"invalid capability id"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.UnlinkAppCapability(r.Context(), id, capabilityID); err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
