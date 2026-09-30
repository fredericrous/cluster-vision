package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewWithoutClusterStillServes pins the failure mode of a process that has
// no reachable API server: no kubeconfig and no in-cluster token. New used to
// return an error there and the process exited before listening — which is
// how the in-cluster migration check (this binary against a prod-data clone,
// in a pod that mounts no service account token) could never come up, and
// how a broken RBAC binding would crash-loop the pod that owns the database.
// The server must boot, serve /api/config, and refresh with empty data.
func TestNewWithoutClusterStillServes(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	s, err := New(Config{Kubeconfig: "", ClusterName: "nowhere", RefreshInterval: time.Minute})
	if err != nil {
		t.Fatalf("New must not fail without a cluster: %v", err)
	}
	if len(s.k8sParsers) != 0 {
		t.Fatalf("expected no parsers, got %d", len(s.k8sParsers))
	}

	// refresh must tolerate the missing primary parser.
	s.refresh(context.Background())

	rr := httptest.NewRecorder()
	s.handleConfig(rr, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("/api/config = %d", rr.Code)
	}
	var cfg struct {
		EAM bool `json:"eam"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.EAM {
		t.Fatal("eam must be false without a DATABASE_URL")
	}
}
