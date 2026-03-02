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

func TestCreateApplicationBadJSON(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("POST", "/api/eam/applications", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateApplicationMissingName(t *testing.T) {
	mux := newTestMux()
	body, _ := json.Marshal(map[string]string{"description": "no name"})
	req := httptest.NewRequest("POST", "/api/eam/applications", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "name is required" {
		t.Errorf("error = %q, want %q", errResp["error"], "name is required")
	}
}

func TestDeleteApplicationBadUUID(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("DELETE", "/api/eam/applications/invalid", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateApplicationBadUUID(t *testing.T) {
	mux := newTestMux()
	req := httptest.NewRequest("PUT", "/api/eam/applications/invalid", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
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
		{"POST", "/api/eam/applications"},
		{"GET", "/api/eam/applications/" + uuid.New().String()},
		{"PUT", "/api/eam/applications/" + uuid.New().String()},
		{"DELETE", "/api/eam/applications/" + uuid.New().String()},
		{"GET", "/api/eam/components"},
		{"POST", "/api/eam/components"},
		{"GET", "/api/eam/capabilities/tree"},
		{"GET", "/api/eam/capabilities"},
		{"POST", "/api/eam/capabilities"},
		{"GET", "/api/eam/landscape"},
		{"GET", "/api/eam/roadmap"},
		{"GET", "/api/eam/graph"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			var body *bytes.Buffer
			if rt.method == "POST" || rt.method == "PUT" {
				body = bytes.NewBufferString("{}")
			} else {
				body = &bytes.Buffer{}
			}
			req := httptest.NewRequest(rt.method, rt.path, body)
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

func TestComponentBadUUID(t *testing.T) {
	mux := newTestMux()

	for _, method := range []string{"GET", "PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			var body *bytes.Buffer
			if method == "PUT" {
				body = bytes.NewBufferString("{}")
			} else {
				body = &bytes.Buffer{}
			}
			req := httptest.NewRequest(method, "/api/eam/components/not-uuid", body)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("%s /api/eam/components/not-uuid = %d, want 400", method, w.Code)
			}
		})
	}
}

func TestCapabilityBadUUID(t *testing.T) {
	mux := newTestMux()

	for _, method := range []string{"GET", "PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			var body *bytes.Buffer
			if method == "PUT" {
				body = bytes.NewBufferString("{}")
			} else {
				body = &bytes.Buffer{}
			}
			req := httptest.NewRequest(method, "/api/eam/capabilities/not-uuid", body)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("%s /api/eam/capabilities/not-uuid = %d, want 400", method, w.Code)
			}
		})
	}
}
