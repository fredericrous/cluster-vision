package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
)

func TestCORSAllowsNoCrossSiteMutation(t *testing.T) {
	reached := false
	h := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))

	// Preflight for a cross-origin POST: POST must not be offered.
	pre := httptest.NewRequest(http.MethodOptions, "/api/eam/sync/trigger", nil)
	pre.Header.Set("Origin", "https://evil.example")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, pre)
	if m := rr.Header().Get("Access-Control-Allow-Methods"); strings.Contains(m, "POST") {
		t.Fatalf("preflight allows %q; POST must not be allowed cross-origin", m)
	}

	// The "simple" POSTs a page can send without preflight never reach a handler.
	for _, ct := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		reached = false
		req := httptest.NewRequest(http.MethodPost, "/api/eam/sync/trigger", strings.NewReader("{}"))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnsupportedMediaType || reached {
			t.Errorf("POST with Content-Type %q = %d (reached=%v), want 415 before the handler", ct, rr.Code, reached)
		}
	}

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/diagrams", nil),
		func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/api/cve/enrichment", strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json; charset=utf-8")
			return r
		}(),
	} {
		reached = false
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !reached {
			t.Errorf("%s %s did not reach the handler", req.Method, req.URL.Path)
		}
	}
}

func TestSyncTriggerIsSingleFlight(t *testing.T) {
	s := newTestServer()
	s.clusterData = &model.ClusterData{}
	s.eam.Store(&eamState{}) // the syncer is never reached
	s.syncRunning.Store(true)

	rr := httptest.NewRecorder()
	s.handleSyncTrigger(rr, httptest.NewRequest(http.MethodPost, "/api/eam/sync/trigger", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("sync trigger during a sync = %d, want 409", rr.Code)
	}
}

func TestEnrichIsSingleFlightAndBounded(t *testing.T) {
	s := newTestServer()
	release := make(chan struct{})
	gotDeadline := make(chan bool, 1)
	if !s.startEnrichment("test", func(ctx context.Context) error {
		_, ok := ctx.Deadline()
		gotDeadline <- ok
		<-release
		return nil
	}) {
		t.Fatal("the first enrichment must start")
	}
	if !<-gotDeadline {
		t.Fatal("enrichment must run under a deadline")
	}

	if s.startEnrichment("test", func(context.Context) error { return nil }) {
		t.Fatal("a second enrichment started while one was running")
	}
	s.eam.Store(&eamState{})
	rr := httptest.NewRecorder()
	s.handleEnrich(rr, httptest.NewRequest(http.MethodPost, "/api/eam/enrich", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("enrich during an enrichment = %d, want 409", rr.Code)
	}

	close(release)
	waitFor(t, "the enrichment to finish", func() bool { return !s.enrichRunning.Load() })
	done := make(chan struct{})
	if !s.startEnrichment("test", func(context.Context) error { close(done); return nil }) {
		t.Fatal("an enrichment must start again once the previous finished")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("enrichment did not run")
	}
}
