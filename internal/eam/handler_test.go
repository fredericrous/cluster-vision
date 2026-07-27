package eam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func newTestMux() *http.ServeMux {
	h := NewHandler(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestGetApplicationBadUUID(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("GET", "/api/eam/applications/not-a-uuid", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "invalid id" {
		t.Errorf("error = %q, want %q", body["error"], "invalid id")
	}
}

func TestVersionHistoryBadUUID(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("GET", "/api/eam/applications/not-valid/versions", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRouteRegistration(t *testing.T) {
	mux := newTestMux()

	// These routes should all be registered (not return 404).
	// Routes that require DB will return 500 (nil pointer) — that's expected.
	// Routes that validate UUID first return 400 for bad UUIDs.
	// We just check they exist by verifying they don't 404.
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/eam/applications"},
		{"GET", "/api/eam/applications/" + uuid.New().String()},
		{"GET", "/api/eam/applications/" + uuid.New().String() + "/versions"},
		{"GET", "/api/eam/applications/" + uuid.New().String() + "/k8s"},
		{"GET", "/api/eam/components"},
		{"GET", "/api/eam/capabilities/tree"},
		{"GET", "/api/eam/capabilities"},
		{"GET", "/api/eam/graph"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, &bytes.Buffer{})
			w := httptest.NewRecorder()

			// Use recover to handle nil DB panics — we just want to confirm route exists
			func() {
				defer func() { _ = recover() }()
				mux.ServeHTTP(w, req)
			}()

			// If we got a 404 or 405 WITHOUT panicking, the route isn't registered
			if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
				t.Errorf("%s %s returned %d, route not registered", rt.method, rt.path, w.Code)
			}
		})
	}
}

// The authoring surface moved to application-landscape; cluster-vision is a
// read-only observed sensor. Guard against mutation routes creeping back in.
func TestMutationRoutesNotRegistered(t *testing.T) {
	mux := newTestMux()

	routes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/eam/applications"},
		{"PUT", "/api/eam/applications/" + uuid.New().String()},
		{"DELETE", "/api/eam/applications/" + uuid.New().String()},
		{"POST", "/api/eam/components"},
		{"PUT", "/api/eam/components/" + uuid.New().String()},
		{"DELETE", "/api/eam/components/" + uuid.New().String()},
		{"POST", "/api/eam/capabilities"},
		{"PUT", "/api/eam/capabilities/" + uuid.New().String()},
		{"DELETE", "/api/eam/capabilities/" + uuid.New().String()},
		{"POST", "/api/eam/applications/" + uuid.New().String() + "/dependencies"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, bytes.NewBufferString("{}"))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
				t.Errorf("%s %s returned %d, want 404/405 (mutation routes are removed)", rt.method, rt.path, w.Code)
			}
		})
	}
}

func TestComponentBadUUID(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("GET", "/api/eam/components/not-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GET /api/eam/components/not-uuid = %d, want 400", w.Code)
	}
}

func TestCapabilityBadUUID(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("GET", "/api/eam/capabilities/not-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GET /api/eam/capabilities/not-uuid = %d, want 400", w.Code)
	}
}
