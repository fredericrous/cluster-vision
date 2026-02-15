package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fredericrous/cluster-vision/internal/diagram"
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/parser"
)

// Config holds server configuration.
type Config struct {
	Port            int
	Kubeconfig      string
	DataSources     []model.DataSource
	RefreshInterval time.Duration
}

// Server serves the diagram API.
type Server struct {
	cfg     Config
	k8s     *parser.KubernetesParser
	mu      sync.RWMutex
	data    []model.DiagramResult
	lastGen time.Time
}

// New creates a new Server.
func New(cfg Config) (*Server, error) {
	k8s, err := parser.NewKubernetesParser(cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("creating k8s parser: %w", err)
	}
	return &Server{cfg: cfg, k8s: k8s}, nil
}

// Start begins serving HTTP and starts the background refresh loop.
func (s *Server) Start(ctx context.Context) error {
	// Initial generation
	s.refresh(ctx)

	// Background refresh
	go s.refreshLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/diagrams", s.handleDiagrams)
	mux.HandleFunc("GET /api/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	slog.Info("starting server", "addr", addr, "refresh", s.cfg.RefreshInterval, "dataSources", len(s.cfg.DataSources))

	srv := &http.Server{Addr: addr, Handler: withCORS(mux)}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	return srv.ListenAndServe()
}

func (s *Server) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

func (s *Server) refresh(ctx context.Context) {
	slog.Info("refreshing cluster data")
	start := time.Now()

	clusterData := s.k8s.ParseAll(ctx)

	// Resolve each data source
	for _, ds := range s.cfg.DataSources {
		src, err := resolveDataSource(ds)
		if err != nil {
			slog.Warn("failed to resolve data source", "name", ds.Name, "error", err)
			continue
		}
		if src != nil {
			clusterData.InfraSources = append(clusterData.InfraSources, *src)
		}
	}

	diagrams := diagram.GenerateTopologySections(clusterData)
	diagrams = append(diagrams,
		diagram.GenerateDependencies(clusterData),
		diagram.GenerateNetwork(clusterData),
		diagram.GenerateSecurity(clusterData),
	)

	s.mu.Lock()
	s.data = diagrams
	s.lastGen = time.Now()
	s.mu.Unlock()

	slog.Info("refresh complete", "duration", time.Since(start))
}

// resolveDataSource fetches and parses a single data source.
func resolveDataSource(ds model.DataSource) (*model.InfraSource, error) {
	data, err := fetchSourceData(ds)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	src := &model.InfraSource{
		Name: ds.Name,
		Type: ds.Type,
	}

	switch ds.Type {
	case "tfstate":
		nodes := parser.ParseTerraformStateBytes(data)
		if len(nodes) == 0 {
			return nil, nil
		}
		src.TerraformNodes = nodes
	case "docker-compose":
		dc, err := parser.ParseDockerCompose(data)
		if err != nil {
			return nil, fmt.Errorf("parsing docker-compose: %w", err)
		}
		if dc == nil {
			return nil, nil
		}
		src.DockerCompose = dc
	default:
		return nil, fmt.Errorf("unknown data source type: %s", ds.Type)
	}

	return src, nil
}

// fetchSourceData reads raw bytes from a mounted file.
func fetchSourceData(ds model.DataSource) ([]byte, error) {
	if ds.Path == "" {
		return nil, fmt.Errorf("data source %q has no path configured", ds.Name)
	}
	return os.ReadFile(ds.Path)
}

func (s *Server) handleDiagrams(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := struct {
		Diagrams    []model.DiagramResult `json:"diagrams"`
		GeneratedAt time.Time             `json:"generated_at"`
	}{
		Diagrams:    s.data,
		GeneratedAt: s.lastGen,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	hasData := len(s.data) > 0
	s.mu.RUnlock()

	if !hasData {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"initializing"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
