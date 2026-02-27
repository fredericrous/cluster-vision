package eam

import (
	"net/http"

	"github.com/fredericrous/cluster-vision/internal/store"
)

// Handler holds EAM HTTP handlers.
type Handler struct {
	db *store.DB
}

// NewHandler creates a new EAM handler.
func NewHandler(db *store.DB) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes registers all EAM routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Applications
	mux.HandleFunc("GET /api/eam/applications", h.listApplications)
	mux.HandleFunc("POST /api/eam/applications", h.createApplication)
	mux.HandleFunc("GET /api/eam/applications/{id}", h.getApplication)
	mux.HandleFunc("PUT /api/eam/applications/{id}", h.updateApplication)
	mux.HandleFunc("DELETE /api/eam/applications/{id}", h.deleteApplication)

	// Application relationships
	mux.HandleFunc("GET /api/eam/applications/{id}/dependencies", h.listAppDependencies)
	mux.HandleFunc("POST /api/eam/applications/{id}/dependencies", h.addAppDependency)
	mux.HandleFunc("DELETE /api/eam/applications/{id}/dependencies/{targetId}", h.removeAppDependency)
	mux.HandleFunc("GET /api/eam/applications/{id}/components", h.listAppComponents)
	mux.HandleFunc("POST /api/eam/applications/{id}/components", h.linkAppComponent)
	mux.HandleFunc("DELETE /api/eam/applications/{id}/components/{componentId}", h.unlinkAppComponent)
	mux.HandleFunc("GET /api/eam/applications/{id}/capabilities", h.listAppCapabilities)
	mux.HandleFunc("POST /api/eam/applications/{id}/capabilities", h.linkAppCapability)
	mux.HandleFunc("DELETE /api/eam/applications/{id}/capabilities/{capabilityId}", h.unlinkAppCapability)
	mux.HandleFunc("GET /api/eam/applications/{id}/k8s", h.listAppK8sSources)
	mux.HandleFunc("GET /api/eam/applications/{id}/versions", h.listAppVersionHistory)

	// IT Components
	mux.HandleFunc("GET /api/eam/components", h.listComponents)
	mux.HandleFunc("POST /api/eam/components", h.createComponent)
	mux.HandleFunc("GET /api/eam/components/{id}", h.getComponent)
	mux.HandleFunc("PUT /api/eam/components/{id}", h.updateComponent)
	mux.HandleFunc("DELETE /api/eam/components/{id}", h.deleteComponent)

	// Business Capabilities
	mux.HandleFunc("GET /api/eam/capabilities/tree", h.getCapabilityTree)
	mux.HandleFunc("GET /api/eam/capabilities", h.listCapabilities)
	mux.HandleFunc("POST /api/eam/capabilities", h.createCapability)
	mux.HandleFunc("GET /api/eam/capabilities/{id}", h.getCapability)
	mux.HandleFunc("PUT /api/eam/capabilities/{id}", h.updateCapability)
	mux.HandleFunc("DELETE /api/eam/capabilities/{id}", h.deleteCapability)

	// Aggregated views
	mux.HandleFunc("GET /api/eam/landscape", h.getLandscape)
	mux.HandleFunc("GET /api/eam/roadmap", h.getRoadmap)
	mux.HandleFunc("GET /api/eam/graph", h.getGraph)
}
