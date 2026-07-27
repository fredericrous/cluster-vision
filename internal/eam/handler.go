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

// RegisterRoutes registers the EAM routes on the given mux.
//
// The surface is read-only: cluster-vision is the OBSERVED sensor in the
// federation with application-landscape, which owns the declared model and
// all authoring. The response shapes below are frozen — application-landscape
// polls /applications, /graph, /applications/{id}/versions and
// /applications/{id}/k8s (see its adr-cluster-vision-federation).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Applications
	mux.HandleFunc("GET /api/eam/applications", h.listApplications)
	mux.HandleFunc("GET /api/eam/applications/{id}", h.getApplication)

	// Application relationships
	mux.HandleFunc("GET /api/eam/applications/{id}/dependencies", h.listAppDependencies)
	mux.HandleFunc("GET /api/eam/applications/{id}/components", h.listAppComponents)
	mux.HandleFunc("GET /api/eam/applications/{id}/capabilities", h.listAppCapabilities)
	mux.HandleFunc("GET /api/eam/applications/{id}/k8s", h.listAppK8sSources)
	mux.HandleFunc("GET /api/eam/applications/{id}/versions", h.listAppVersionHistory)

	// IT Components
	mux.HandleFunc("GET /api/eam/components", h.listComponents)
	mux.HandleFunc("GET /api/eam/components/{id}", h.getComponent)

	// Business Capabilities
	mux.HandleFunc("GET /api/eam/capabilities/tree", h.getCapabilityTree)
	mux.HandleFunc("GET /api/eam/capabilities", h.listCapabilities)
	mux.HandleFunc("GET /api/eam/capabilities/{id}", h.getCapability)

	// Aggregated views
	mux.HandleFunc("GET /api/eam/graph", h.getGraph)

	// Coordinate → CI slug resolution
	mux.HandleFunc("GET /api/eam/resolve", h.resolveCI)
}
