package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/fredericrous/cluster-vision/internal/diagram"
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/parser"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// Config holds server configuration.
type Config struct {
	Port            int
	Kubeconfig      string
	ClusterName     string
	DataSources     []model.DataSource
	RefreshInterval time.Duration
	RegistryProxy   string // host:port of local OCI proxy (e.g. Zot) for upstream resolution
}

// Server serves the diagram API.
type Server struct {
	cfg        Config
	k8sParsers []*parser.KubernetesParser
	checker    *versions.Checker
	mu         sync.RWMutex
	data       []model.DiagramResult
	lastGen    time.Time
}

// New creates a new Server.
func New(cfg Config) (*Server, error) {
	if cfg.ClusterName == "" {
		cfg.ClusterName = "Homelab"
	}

	k8s, err := parser.NewKubernetesParser(cfg.Kubeconfig, cfg.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("creating k8s parser: %w", err)
	}

	parsers := []*parser.KubernetesParser{k8s}

	for _, ds := range cfg.DataSources {
		if ds.Type != "kubernetes" {
			continue
		}
		if _, err := os.Stat(ds.Path); err != nil {
			slog.Warn("skipping kubernetes data source: kubeconfig not readable", "name", ds.Name, "path", ds.Path, "error", err)
			continue
		}
		p, err := parser.NewKubernetesParser(ds.Path, ds.Name)
		if err != nil {
			slog.Warn("skipping kubernetes data source: failed to create parser", "name", ds.Name, "error", err)
			continue
		}
		parsers = append(parsers, p)
		slog.Info("added kubernetes data source", "name", ds.Name)
	}

	checker := versions.NewChecker(cfg.RefreshInterval, cfg.RegistryProxy)

	return &Server{cfg: cfg, k8sParsers: parsers, checker: checker}, nil
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

	// Primary cluster — full data
	clusterData := s.k8sParsers[0].ParseAll(ctx)
	clusterData.PrimaryCluster = s.cfg.ClusterName

	// Additional clusters — security data only
	for _, p := range s.k8sParsers[1:] {
		ns, sp := p.ParseSecurity(ctx)
		clusterData.Namespaces = append(clusterData.Namespaces, ns...)
		clusterData.SecurityPolicies = append(clusterData.SecurityPolicies, sp...)
	}

	// Sort namespaces and security policies deterministically
	sort.Slice(clusterData.Namespaces, func(i, j int) bool {
		if clusterData.Namespaces[i].Cluster != clusterData.Namespaces[j].Cluster {
			return clusterData.Namespaces[i].Cluster < clusterData.Namespaces[j].Cluster
		}
		return clusterData.Namespaces[i].Name < clusterData.Namespaces[j].Name
	})
	sort.Slice(clusterData.SecurityPolicies, func(i, j int) bool {
		if clusterData.SecurityPolicies[i].Cluster != clusterData.SecurityPolicies[j].Cluster {
			return clusterData.SecurityPolicies[i].Cluster < clusterData.SecurityPolicies[j].Cluster
		}
		if clusterData.SecurityPolicies[i].Namespace != clusterData.SecurityPolicies[j].Namespace {
			return clusterData.SecurityPolicies[i].Namespace < clusterData.SecurityPolicies[j].Namespace
		}
		return clusterData.SecurityPolicies[i].Name < clusterData.SecurityPolicies[j].Name
	})

	// Resolve each infra data source (tfstate, docker-compose)
	for _, ds := range s.cfg.DataSources {
		if ds.Type == "kubernetes" {
			continue
		}
		src, err := resolveDataSource(ds)
		if err != nil {
			slog.Warn("failed to resolve data source", "name", ds.Name, "error", err)
			continue
		}
		if src != nil {
			clusterData.InfraSources = append(clusterData.InfraSources, *src)
		}
	}

	// Check for latest versions in background
	s.checker.Check(clusterData.HelmRepositories, clusterData.HelmReleases)

	diagrams := diagram.GenerateTopologySections(clusterData)
	diagrams = append(diagrams,
		diagram.GenerateDependencies(clusterData),
		diagram.GenerateNetwork(clusterData),
		diagram.GenerateSecurity(clusterData),
		diagram.GenerateVersions(clusterData, s.checker),
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
