package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// fakeParser stands in for a cluster. ParseAll blocks on gate when set.
type fakeParser struct {
	name  string
	data  func() *model.ClusterData
	err   error
	gate  chan struct{}
	calls atomic.Int32
}

func (f *fakeParser) ClusterName() string { return f.name }

func (f *fakeParser) ParseAll(ctx context.Context) (*model.ClusterData, error) {
	f.calls.Add(1)
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return &model.ClusterData{}, ctx.Err()
		}
	}
	if f.data == nil {
		return &model.ClusterData{}, f.err
	}
	return f.data(), f.err
}

// newTestServer builds a Server around fake parsers, with real (idle)
// checkers: the fake cluster has no repos, images or nodes to look up.
func newTestServer(parsers ...clusterParser) *Server {
	s := &Server{
		cfg:             Config{ClusterName: "test", RefreshInterval: time.Minute},
		k8sParsers:      parsers,
		checker:         versions.NewChecker(time.Minute, ""),
		imageChecker:    versions.NewImageChecker(),
		nodeChecker:     versions.NewNodeChecker(),
		securityChecker: versions.NewSecurityChecker(),
		kick:            make(chan struct{}, 1),
	}
	s.exploit.Store(versions.NewExploitEnricher(nil))
	return s
}

// Concurrent refreshes, background re-renders and every reader of s.data
// must be race-free (run with -race): readers hash and encode the slice
// after dropping the lock, so nothing may write into a published slice.
func TestRefreshAndReadersAreRaceFree(t *testing.T) {
	s := newTestServer(&fakeParser{name: "test"})
	s.refresh(context.Background())

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				rr := httptest.NewRecorder()
				s.handleDiagrams(rr, httptest.NewRequest(http.MethodGet, "/api/diagrams", nil))
				if _, err := s.current(); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for i := 0; i < 5; i++ {
		s.refresh(context.Background())
		s.updateDiagram("charts", func(cd *model.ClusterData) model.DiagramResult {
			return model.DiagramResult{ID: "charts", Title: "re-rendered"}
		})
	}
	close(stop)
	wg.Wait()
}

func TestUpdateDiagramIsCopyOnWrite(t *testing.T) {
	s := newTestServer()
	s.data = []model.DiagramResult{{ID: "a", Content: "old"}, {ID: "b", Content: "old"}}
	s.clusterData = &model.ClusterData{}
	held := s.data // what a reader took under RLock

	if !s.updateDiagram("b", func(*model.ClusterData) model.DiagramResult {
		return model.DiagramResult{ID: "b", Content: "new"}
	}) {
		t.Fatal("update of the current generation must apply")
	}
	if held[1].Content != "old" {
		t.Fatal("a slice a reader holds was modified in place")
	}
	if s.data[1].Content != "new" {
		t.Fatalf("published data = %+v, want b updated", s.data)
	}
}

// A re-render that finishes after a newer refresh published must not
// overwrite the newer generation's diagram.
func TestUpdateDiagramDropsOlderGeneration(t *testing.T) {
	s := newTestServer()
	s.data = []model.DiagramResult{{ID: "charts", Content: "gen1"}}
	s.clusterData = &model.ClusterData{}
	s.gen = 1

	applied := s.updateDiagram("charts", func(*model.ClusterData) model.DiagramResult {
		// A refresh publishes while this render runs.
		s.mu.Lock()
		s.gen = 2
		s.data = []model.DiagramResult{{ID: "charts", Content: "gen2"}}
		s.mu.Unlock()
		return model.DiagramResult{ID: "charts", Content: "stale"}
	})
	if applied {
		t.Fatal("a result rendered for generation 1 was applied to generation 2")
	}
	if s.data[0].Content != "gen2" {
		t.Fatalf("data = %+v, want gen2 untouched", s.data)
	}
}

func TestRunCheckIsSingleFlight(t *testing.T) {
	s := newTestServer()
	s.clusterData = &model.ClusterData{}
	var flag atomic.Bool
	release := make(chan struct{})
	var runs atomic.Int32
	done := make(chan struct{}, 2)
	check := func() {
		runs.Add(1)
		<-release
	}
	render := func(*model.ClusterData) model.DiagramResult {
		done <- struct{}{}
		return model.DiagramResult{ID: "x"}
	}

	s.runCheck(&flag, "t", check, "x", render)
	// Wait until the first check is actually running.
	deadline := time.Now().Add(5 * time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	s.runCheck(&flag, "t", check, "x", render) // slow upstream: must not start a second
	close(release)
	<-done
	if n := runs.Load(); n != 1 {
		t.Fatalf("check ran %d times while one was in flight, want 1", n)
	}

	// Once finished, the next refresh may run it again.
	s.runCheck(&flag, "t", func() { runs.Add(1) }, "x", render)
	<-done
	if n := runs.Load(); n != 2 {
		t.Fatalf("check ran %d times, want 2 after the first finished", n)
	}
}

func TestRefreshTimeoutBounds(t *testing.T) {
	for in, want := range map[time.Duration]time.Duration{
		0:                time.Minute,
		30 * time.Second: time.Minute,
		5 * time.Minute:  5 * time.Minute,
		time.Hour:        10 * time.Minute,
	} {
		s := &Server{cfg: Config{RefreshInterval: in}}
		if got := s.refreshTimeout(); got != want {
			t.Errorf("refreshTimeout(%s) = %s, want %s", in, got, want)
		}
	}
}

// A hung API server must not hang refresh: the parse runs under a context
// derived from refresh's, and an interrupted parse is published as partial.
func TestRefreshReturnsWhenParseHangs(t *testing.T) {
	p := &fakeParser{name: "test", gate: make(chan struct{})} // never released
	s := newTestServer(p)
	s.cfg.RefreshInterval = 0 // refreshTimeout floors at one minute

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { s.refresh(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not return when its context expired")
	}
	s.mu.RLock()
	partial := s.lastPartial
	s.mu.RUnlock()
	if !partial {
		t.Fatal("an interrupted parse must be published as partial")
	}
}
