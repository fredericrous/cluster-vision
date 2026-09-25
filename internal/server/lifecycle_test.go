package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// startServing runs s.serve on a loopback port and returns its base URL and
// a stop function that cancels it and returns serve's error.
func startServing(t *testing.T, s *Server) (string, func() error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- s.serve(ctx, ln) }()
	stopped := false
	stop := func() error {
		if stopped {
			return nil
		}
		stopped = true
		cancel()
		select {
		case err := <-errc:
			return err
		case <-time.After(3 * shutdownTimeout):
			t.Fatal("serve did not return after cancel")
			return nil
		}
	}
	t.Cleanup(func() { _ = stop() })
	return "http://" + ln.Addr().String(), stop
}

func get(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The listener must be up, and liveness green, before the first refresh
// (and the DB connect) finish: a boot that did them first failed liveness
// and crash-looped. Readiness stays red until the first refresh publishes.
func TestServeAnswersLivenessBeforeFirstRefresh(t *testing.T) {
	p := &fakeParser{name: "test", gate: make(chan struct{})}
	s := newTestServer(p)
	base, stop := startServing(t, s)

	if code, _ := get(t, base+"/api/health/live"); code != http.StatusOK {
		t.Fatalf("liveness = %d while the first refresh runs, want 200", code)
	}
	waitFor(t, "the first refresh to start", func() bool { return p.calls.Load() > 0 })
	if code, body := get(t, base+"/api/health"); code != http.StatusServiceUnavailable || body["status"] != "initializing" {
		t.Fatalf("readiness = %d %v before the first refresh, want 503 initializing", code, body)
	}

	close(p.gate)
	waitFor(t, "readiness", func() bool {
		code, _ := get(t, base+"/api/health")
		return code == http.StatusOK
	})

	if err := stop(); err != nil {
		t.Fatalf("a normal shutdown must return nil, got %v", err)
	}
}

// EAM routes exist from the start. Without a DATABASE_URL they are a 404
// that says so; with one that is still connecting they are a 503 (and
// readiness waits for the connect).
func TestEAMRoutesBeforeDatabase(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		s := newTestServer(&fakeParser{name: "test"})
		base, _ := startServing(t, s)
		if code, _ := get(t, base+"/api/eam/applications"); code != http.StatusNotFound {
			t.Fatalf("/api/eam/applications = %d without a database, want 404", code)
		}
		if code, _ := get(t, base+"/api/snapshots"); code != http.StatusNotFound {
			t.Fatalf("/api/snapshots = %d without a database, want 404", code)
		}
	})

	t.Run("connecting", func(t *testing.T) {
		s := newTestServer(&fakeParser{name: "test"})
		// Nothing listens there: the connect keeps retrying for its budget.
		s.cfg.DatabaseURL = "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"
		s.dbPending.Store(true)
		base, stop := startServing(t, s)

		if code, body := get(t, base+"/api/eam/applications"); code != http.StatusServiceUnavailable {
			t.Fatalf("/api/eam/applications = %d %v while connecting, want 503", code, body)
		}
		waitFor(t, "the first refresh", s.ready.Load)
		if code, body := get(t, base+"/api/health"); code != http.StatusServiceUnavailable || body["status"] != "db_connecting" {
			t.Fatalf("readiness = %d %v while the DB connects, want 503 db_connecting", code, body)
		}
		if _, body := get(t, base+"/api/config"); body["eam"] != false {
			t.Fatalf("/api/config eam = %v before the DB connected", body["eam"])
		}

		// Shutdown must not wait out the five-minute connect budget.
		start := time.Now()
		if err := stop(); err != nil {
			t.Fatalf("shutdown = %v, want nil", err)
		}
		if d := time.Since(start); d > 2*shutdownTimeout {
			t.Fatalf("shutdown took %s", d)
		}
	})
}

// With a real database the EAM routes come up once the background connect
// finishes, and shutdown closes the pool only after HTTP has drained.
func TestEAMComesUpAfterConnect(t *testing.T) {
	url := os.Getenv("CV_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CV_TEST_DATABASE_URL not set")
	}
	s := newTestServer(&fakeParser{name: "test"})
	s.cfg.DatabaseURL = url
	s.dbPending.Store(true)
	base, stop := startServing(t, s)

	waitFor(t, "EAM", func() bool {
		_, body := get(t, base+"/api/config")
		return body["eam"] == true
	})
	waitFor(t, "readiness", func() bool {
		code, _ := get(t, base+"/api/health")
		return code == http.StatusOK
	})
	resp, err := http.Get(base + "/api/eam/applications")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/eam/applications = %d once connected", resp.StatusCode)
	}
	if err := stop(); err != nil {
		t.Fatalf("shutdown = %v", err)
	}
	if err := s.database().Pool.Ping(context.Background()); err == nil {
		t.Fatal("the pool must be closed after shutdown")
	}
}
