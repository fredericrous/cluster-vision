package eam

import (
	"encoding/json"
	"net/http"
)

// resolveCI maps k8s coordinates (from alert labels, webhooks, agents) to a
// CI slug. Consumers: ticket-vision's Alertmanager auto-link and
// sre-agent's KB writer.
//
// Responses:
//   - 200 {"match": {...}}                 exactly one application matches
//   - 404 {"error":"no match"}             nothing matches
//   - 409 {"error":"ambiguous","matches":[...]}  narrower coordinates needed
func (h *Handler) resolveCI(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cluster := q.Get("cluster")
	namespace := q.Get("namespace")
	helmRelease := q.Get("helm_release")
	workload := q.Get("workload")

	if cluster == "" && namespace == "" && helmRelease == "" && workload == "" {
		http.Error(w, `{"error":"at least one of cluster, namespace, helm_release, workload is required"}`, http.StatusBadRequest)
		return
	}

	matches, err := h.db.ResolveCoordinates(r.Context(), cluster, namespace, helmRelease, workload)
	if err != nil {
		http.Error(w, jsonErr(err), http.StatusInternalServerError)
		return
	}

	switch len(matches) {
	case 0:
		http.Error(w, `{"error":"no match"}`, http.StatusNotFound)
	case 1:
		writeJSON(w, map[string]any{"match": matches[0]})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "ambiguous", "matches": matches})
	}
}
