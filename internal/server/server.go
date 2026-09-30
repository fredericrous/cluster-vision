package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fredericrous/cluster-vision/internal/agent"
	"github.com/fredericrous/cluster-vision/internal/diagram"
	"github.com/fredericrous/cluster-vision/internal/discovery"
	"github.com/fredericrous/cluster-vision/internal/eam"
	cvmetrics "github.com/fredericrous/cluster-vision/internal/metrics"
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/parser"
	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/fredericrous/cluster-vision/internal/versions"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds server configuration.
type Config struct {
	Port            int
	Kubeconfig      string
	ClusterName     string
	DataSources     []model.DataSource
	RefreshInterval time.Duration
	RegistryProxy   string // host:port of local OCI proxy (e.g. Zot) for upstream resolution
	// Snapshot retention (needs DatabaseURL). Every snapshot is kept for
	// SnapshotFullRetention, one per day up to SnapshotDailyRetention, and
	// the first snapshot at each revision forever.
	SnapshotFullRetention  time.Duration
	SnapshotDailyRetention time.Duration
	// EAM (all optional)
	DatabaseURL  string // enables EAM features
	LiteLLMURL   string // enables AI enrichment
	LiteLLMKey   string // API key for LiteLLM
	LiteLLMModel string // default model
}

// clusterParser is what refresh needs from a cluster: the production
// implementation is *parser.KubernetesParser; tests substitute fakes.
type clusterParser interface {
	ParseAll(ctx context.Context) (*model.ClusterData, error)
	ClusterName() string
}

// Server serves the diagram API.
//
// Concurrency: data, clusterData, gen and friends are guarded by mu and are
// copy-on-write. A published data slice (and the ClusterData behind it) is
// never modified again — readers take the slice under RLock and use it after
// unlocking (encoding, hashing, diffing), so a writer must build a new slice
// and swap it in whole.
type Server struct {
	cfg             Config
	k8sParsers      []clusterParser
	checker         *versions.Checker
	imageChecker    *versions.ImageChecker
	nodeChecker     *versions.NodeChecker
	securityChecker *versions.SecurityChecker
	// CISA KEV + FIRST EPSS. Starts in-memory; replaced once by the
	// DB-backed one when the database connects. Read through intel().
	exploit         atomic.Pointer[versions.ExploitEnricher]
	mu              sync.RWMutex
	data            []model.DiagramResult // copy-on-write, see above
	gen             uint64                // bumped by every published refresh
	lastGen         time.Time
	lastPartial     bool               // last refresh had a failed list call somewhere
	partialClusters map[string]bool    // clusters whose list calls failed last refresh
	clusterData     *model.ClusterData // cached for EAM sync-on-demand
	// Single-flight guards for the slow upstream checks refresh starts in
	// the background: one of each at a time, however slow the upstream.
	chartsCheck, imagesCheck, nodesCheck, securityCheck atomic.Bool

	// EAM: nil until the database has connected. The connect runs in the
	// background after the listener is up (it may retry for minutes), so
	// every reader goes through eam.Load().
	eam       atomic.Pointer[eamState]
	dbPending atomic.Bool // DATABASE_URL set, connect attempt still running
	ready     atomic.Bool // the first refresh has published

	kick  chan struct{}   // asks refreshLoop for an immediate refresh
	bg    sync.WaitGroup  // background work Start waits for before closing the DB
	bgCtx context.Context // set by serve before any request; see backgroundContext

	// Single-flight for EAM work: one sync and one enrichment at a time,
	// whether started by a refresh or by a POST.
	syncRunning, enrichRunning atomic.Bool
}

// eamState is everything that exists only once the EAM database is up.
type eamState struct {
	db       *store.DB
	syncer   *discovery.Syncer
	enricher *agent.Enricher // nil without LITELLM_URL
	routes   http.Handler    // the DB-backed routes, see routes()
}

// database returns the EAM store, or nil while there is none.
func (s *Server) database() *store.DB {
	if st := s.eam.Load(); st != nil {
		return st.db
	}
	return nil
}

// intel returns the current KEV/EPSS enricher.
func (s *Server) intel() *versions.ExploitEnricher { return s.exploit.Load() }

// dbConnectBudget is how long the background connect keeps retrying the
// EAM database before giving up on it. Five minutes covers a CNPG clone's
// post-restore instability (observed 2.5 min on 2026-09-22) and a primary
// failover.
const dbConnectBudget = 5 * time.Minute

// shutdownTimeout bounds each shutdown phase: draining HTTP requests, then
// waiting for background work. Both fit in the kubelet's default 30s grace.
const shutdownTimeout = 10 * time.Second

// New creates a new Server. It does no I/O that can block: the database is
// connected by Start, in the background, after the listener is up.
func New(cfg Config) (*Server, error) {
	if cfg.ClusterName == "" {
		cfg.ClusterName = "Homelab"
	}

	// The primary cluster is optional, like the EAM database below: a process
	// with no reachable API server (no kubeconfig, no in-cluster token) still
	// boots, serves /api/config and the EAM routes, and reports empty cluster
	// data. Making it fatal meant the in-cluster migration check — which runs
	// this binary against a prod-data clone in a pod that mounts no service
	// account token on purpose — could never come up (2026-09-22), and it is
	// the wrong failure mode for a Deployment too: a broken RBAC binding should
	// degrade the diagrams, not crash-loop the pod that owns the database.
	var parsers []clusterParser
	if k8s, err := parser.NewKubernetesParser(cfg.Kubeconfig, cfg.ClusterName, ""); err != nil {
		slog.Error("no primary cluster: k8s parser unavailable — cluster discovery disabled", "error", err)
	} else {
		parsers = append(parsers, k8s)
	}

	for _, ds := range cfg.DataSources {
		if ds.Type != "kubernetes" {
			continue
		}
		if _, err := os.Stat(ds.Path); err != nil {
			slog.Warn("skipping kubernetes data source: kubeconfig not readable", "name", ds.Name, "path", ds.Path, "error", err)
			continue
		}
		p, err := parser.NewKubernetesParser(ds.Path, ds.Name, ds.Platform)
		if err != nil {
			slog.Warn("skipping kubernetes data source: failed to create parser", "name", ds.Name, "error", err)
			continue
		}
		parsers = append(parsers, p)
		slog.Info("added kubernetes data source", "name", ds.Name)
	}

	s := &Server{
		cfg:             cfg,
		k8sParsers:      parsers,
		checker:         versions.NewChecker(cfg.RefreshInterval, cfg.RegistryProxy),
		imageChecker:    versions.NewImageChecker(),
		nodeChecker:     versions.NewNodeChecker(),
		securityChecker: versions.NewSecurityChecker(),
		kick:            make(chan struct{}, 1),
	}
	// In-memory until (and unless) the database connects.
	s.exploit.Store(versions.NewExploitEnricher(nil))
	s.dbPending.Store(cfg.DatabaseURL != "")
	return s, nil
}

// Start listens, then brings everything else up in the background: the
// first refresh, the EAM database connect, the enrichment and retention
// loops. Nothing slow runs before the listener, so /api/health/live answers
// from the first second — a boot that waited on the database (up to
// dbConnectBudget) and a full cluster refresh first failed its liveness
// probe and crash-looped.
//
// Start returns once ctx is cancelled and shutdown has finished: HTTP
// drained first, then background work stopped, then the database closed.
// A normal shutdown returns nil.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("listening: %w", err)
	}
	return s.serve(ctx, ln)
}

func (s *Server) serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: withCORS(s.routes()), ReadHeaderTimeout: 10 * time.Second}
	slog.Info("starting server", "addr", ln.Addr().String(), "refresh", s.cfg.RefreshInterval, "dataSources", len(s.cfg.DataSources))

	bgCtx, stopBackground := context.WithCancel(ctx)
	defer stopBackground()
	s.bgCtx = bgCtx // before Serve: handlers read it

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	s.goBackground(func() { s.bootstrap(bgCtx) })

	var err error
	select {
	case err = <-serveErr:
		// The listener failed on its own; nothing to drain.
		slog.Error("http server stopped", "error", err)
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		err = srv.Shutdown(shutdownCtx)
		cancel()
		// Serve returns ErrServerClosed as soon as Shutdown starts; the
		// drain itself is what Shutdown waited for.
		if serr := <-serveErr; err == nil && !errors.Is(serr, http.ErrServerClosed) {
			err = serr
		}
	}

	// Requests are done; now stop the loops and wait for in-flight
	// background work (snapshot writes, syncs) before closing the pool
	// under it.
	stopBackground()
	s.waitBackground(shutdownTimeout)
	if db := s.database(); db != nil {
		db.Close()
	}
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	slog.Info("server stopped", "error", err)
	return err
}

// goBackground runs f as tracked background work. Every caller is itself
// tracked work or an HTTP handler, so the group is never at zero here while
// serve is waiting on it.
func (s *Server) goBackground(f func()) {
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		f()
	}()
}

// waitBackground waits for tracked background work, at most d: an
// upstream check stuck in a slow registry call must not hold the process.
func (s *Server) waitBackground(d time.Duration) {
	done := make(chan struct{})
	go func() {
		s.bg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		slog.Warn("background work still running at shutdown, closing anyway", "waited", d)
	}
}

// bootstrap starts the refresh loop, then connects the EAM database (if
// configured) and starts what depends on it.
func (s *Server) bootstrap(ctx context.Context) {
	// The refresh needs nothing from the database, and readiness waits on
	// it: start it first.
	s.goBackground(func() { s.refreshLoop(ctx) })

	if s.cfg.DatabaseURL != "" {
		s.connectEAM(ctx)
		s.dbPending.Store(false)
	}
	if ctx.Err() != nil {
		return
	}

	// Warm the KEV/EPSS cache from the persisted table so lookups have data
	// before the daily fetch completes. Best-effort: a fresh database has an
	// empty table, and without a database this is a no-op.
	if err := s.intel().LoadFromDB(ctx); err != nil {
		slog.Warn("exploit enrichment LoadFromDB failed — running with empty cache", "error", err)
	}
	// Publish what the warmed cache knows straight away. The staleness
	// alert reads these gauges, and leaving them at zero until the first
	// fetch completes would fire it on every restart.
	s.publishEnrichmentMetrics()
	s.goBackground(func() { s.exploitEnrichmentLoop(ctx) })

	if s.eam.Load() != nil {
		s.goBackground(func() { s.snapshotRetentionLoop(ctx) })
		// A refresh that ran before the database was up skipped the EAM
		// sync and the snapshot, and saw a cold KEV cache: redo it now.
		s.requestRefresh()
	}
}

// connectEAM connects and migrates the EAM database, retrying within
// dbConnectBudget, and installs the EAM state on success. On failure the
// process keeps running without EAM, as before.
func (s *Server) connectEAM(ctx context.Context) {
	db, err := store.NewWithRetry(ctx, s.cfg.DatabaseURL, dbConnectBudget)
	if err != nil {
		slog.Error("failed to connect EAM database — EAM features disabled", "error", err)
		return
	}
	st := &eamState{db: db, syncer: discovery.NewSyncer(db)}
	if s.cfg.LiteLLMURL != "" {
		client := agent.NewClient(s.cfg.LiteLLMURL, s.cfg.LiteLLMKey, s.cfg.LiteLLMModel)
		st.enricher = agent.NewEnricher(client, db)
		slog.Info("AI enrichment enabled", "model", s.cfg.LiteLLMModel)
	}
	st.routes = s.eamRoutes(st)
	// DB-backed enricher: its cache survives restarts. Swapped before the
	// enrichment loop starts, so no fetched data is lost with the old one.
	s.exploit.Store(versions.NewExploitEnricher(db))
	s.eam.Store(st)
	slog.Info("EAM features enabled")
}

// routes builds the public mux. The DB-backed routes are registered
// unconditionally and answer 503 until the database is ready (404 when no
// database is configured): the connect is asynchronous, so registering them
// only if it had succeeded at boot would never register them.
func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/diagrams", s.handleDiagrams)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/health/live", s.handleHealthLive)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	// Per-CVE KEV/EPSS intel. Not DB-gated: it is served from the in-memory
	// cache, and a caller that gates a deployment on it must not get an
	// error just because Postgres is unreachable.
	mux.HandleFunc("POST /api/cve/enrichment", func(w http.ResponseWriter, r *http.Request) {
		handleCVEEnrichment(s.intel()).ServeHTTP(w, r)
	})
	// Prometheus scrape endpoint — no auth (cluster-internal only via the
	// new `api` Service port; not on the public Gateway).
	mux.Handle("GET /metrics", promhttp.Handler())

	db := http.HandlerFunc(s.handleDBRoute)
	mux.Handle("/api/eam/", db)
	mux.Handle("GET /api/snapshots", db)
	mux.Handle("GET /api/snapshots/{id}/diagrams", db)
	mux.Handle("GET /api/diff", db)
	mux.Handle("GET /api/diagrams/{id}/diff", db)
	return mux
}

// eamRoutes is the mux behind handleDBRoute once the database is up.
func (s *Server) eamRoutes(st *eamState) http.Handler {
	mux := http.NewServeMux()
	eam.NewHandler(st.db).RegisterRoutes(mux)
	// Manual sync trigger
	mux.HandleFunc("POST /api/eam/sync/trigger", s.handleSyncTrigger)
	mux.HandleFunc("GET /api/eam/sync/logs", s.handleSyncLogs)
	// AI enrichment (only if enricher configured)
	if st.enricher != nil {
		mux.HandleFunc("POST /api/eam/enrich", s.handleEnrich)
	}
	// Cluster snapshots + diffs
	mux.HandleFunc("GET /api/snapshots", s.handleListSnapshots)
	mux.HandleFunc("GET /api/snapshots/{id}/diagrams", s.handleSnapshotDiagrams)
	mux.HandleFunc("GET /api/diff", s.handleDiffAll)
	mux.HandleFunc("GET /api/diagrams/{id}/diff", s.handleDiffDiagram)
	return mux
}

// handleDBRoute forwards to the EAM routes, or says why there are none.
func (s *Server) handleDBRoute(w http.ResponseWriter, r *http.Request) {
	if st := s.eam.Load(); st != nil {
		st.routes.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case s.cfg.DatabaseURL == "":
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "EAM is not configured (no DATABASE_URL)"})
	case s.dbPending.Load():
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "EAM database is still connecting"})
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "EAM database unavailable"})
	}
}

// requestRefresh asks refreshLoop for a refresh now, without blocking; a
// request already pending covers this one.
func (s *Server) requestRefresh() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// refreshLoop runs the first refresh at once, then one per interval and
// one per requestRefresh.
func (s *Server) refreshLoop(ctx context.Context) {
	s.refresh(ctx)

	ticker := time.NewTicker(s.cfg.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		case <-s.kick:
			s.refresh(ctx)
		}
	}
}

// publishEnrichmentMetrics mirrors the enricher's current state onto the
// gauges. Called after the DB warm-up as well as after every successful
// refresh, so the gauges describe the cache being served rather than only
// the fetches this process happened to perform.
func (s *Server) publishEnrichmentMetrics() {
	kevN, epssN := s.intel().CacheSize()
	// A zero time.Time is a large negative Unix value; report 0 instead,
	// which reads as "never fetched" to the staleness alert.
	var last float64
	if t := s.intel().LastFetch(); !t.IsZero() {
		last = float64(t.Unix())
	}
	cvmetrics.EnrichmentLastFetch.Set(last)
	cvmetrics.EnrichmentCVETotal.WithLabelValues("kev").Set(float64(kevN))
	cvmetrics.EnrichmentCVETotal.WithLabelValues("epss").Set(float64(epssN))
}

// exploitEnrichmentLoop refreshes the KEV/EPSS cache once a day. The first
// fetch is skipped when the DB-warmed cache is already younger than the
// interval — the feeds are ~5MB and a restart is not a reason to
// re-download them. A failed refresh is retried in an hour rather than a
// day: the data behind it gates deployments.
func (s *Server) exploitEnrichmentLoop(ctx context.Context) {
	const (
		interval = 24 * time.Hour
		retry    = time.Hour
	)

	doRefresh := func() bool {
		fctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := s.intel().Refresh(fctx); err != nil {
			slog.Warn("exploit enrichment refresh failed — retrying", "error", err, "retry_in", retry)
			return false
		}
		s.publishEnrichmentMetrics()
		return true
	}

	// Fire immediately only if the cache is empty or older than a day.
	next := interval
	if last := s.intel().LastFetch(); last.IsZero() || time.Since(last) > interval {
		next = 0
	}
	timer := time.NewTimer(next)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			d := interval
			if !doRefresh() {
				d = retry
			}
			timer.Reset(d)
		}
	}
}

// enrichImageVulns populates KEVCount, KEVCVEs, MaxEPSS, MaxEPSSCVE on
// each ImageVuln by looking up its captured CVE list against the
// in-memory enrichment cache. Image-level (no namespace) — the metric
// layer recovers (cluster, namespace) at emit time from PodImageInfo.
func (s *Server) enrichImageVulns(vulns []model.ImageVuln) {
	intel := s.intel()
	for i := range vulns {
		v := &vulns[i]
		v.KEVCount = 0
		v.KEVCVEs = v.KEVCVEs[:0]
		v.MaxEPSS = 0
		v.MaxEPSSCVE = ""
		for _, cve := range v.CVEs {
			en := intel.Lookup(cve)
			if en.KEV {
				v.KEVCount++
				v.KEVCVEs = append(v.KEVCVEs, cve)
			}
			if en.EPSSScore > v.MaxEPSS {
				v.MaxEPSS = en.EPSSScore
				v.MaxEPSSCVE = cve
			}
		}
	}
}

// generate renders every diagram from a cluster model. It is the single
// place that knows the diagram list, so live refreshes, historical views
// and diffs all draw with the same code — which is what makes a diff mean
// "the cluster changed" rather than "the generator changed".
func (s *Server) generate(clusterData *model.ClusterData) []model.DiagramResult {
	diagrams := diagram.GenerateTopologySections(clusterData)
	diagrams = append(diagrams,
		diagram.GenerateDependencies(clusterData),
		diagram.GenerateNetwork(clusterData),
	)
	diagrams = append(diagrams, diagram.GenerateSecurity(clusterData)...)
	diagrams = append(diagrams, diagram.GenerateImages(clusterData, s.imageChecker))
	diagrams = append(diagrams, diagram.GenerateVersions(clusterData, s.checker))
	diagrams = append(diagrams, diagram.GenerateNodes(clusterData, s.nodeChecker, s.securityChecker))
	diagrams = append(diagrams,
		diagram.GenerateWorkloads(clusterData),
		diagram.GenerateStorage(clusterData),
		diagram.GenerateCRDs(clusterData),
		diagram.GenerateQuotas(clusterData),
		diagram.GenerateCertificates(clusterData),
		diagram.GenerateNetworkPolicies(clusterData),
		diagram.GenerateConfigs(clusterData),
		diagram.GenerateHelmWorkloads(clusterData),
		diagram.GenerateServiceMap(clusterData),
		diagram.GenerateNamespaceSummary(clusterData),
		diagram.GenerateRBAC(clusterData),
		diagram.GenerateLabels(clusterData),
		diagram.GenerateVelero(clusterData),
	)
	return diagrams
}

// refreshTimeout bounds one refresh's cluster parsing. The per-request
// k8s timeout already stops a single hung call; this stops a refresh made
// of many slow ones from outliving its interval. Never below a minute, so
// a short test interval does not starve a real cluster.
func (s *Server) refreshTimeout() time.Duration {
	d := s.cfg.RefreshInterval
	if d < time.Minute {
		d = time.Minute
	}
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	return d
}

// refresh re-parses every cluster, regenerates the diagrams and publishes
// them as a new generation. Background work (snapshot, EAM sync, upstream
// checks) is started with ctx, not with the parse deadline.
func (s *Server) refresh(ctx context.Context) {
	slog.Info("refreshing cluster data")
	start := time.Now()
	parseCtx, cancelParse := context.WithTimeout(ctx, s.refreshTimeout())
	defer cancelParse()

	// All Kubernetes clusters get the same parsing treatment.
	// The first parser remains the primary cluster for UI semantics.
	// A failed list call anywhere makes the whole refresh "partial": the
	// diagrams are still served (stale-but-visible beats blank), but the
	// state must not be persisted as a snapshot or it would record a
	// mass delete followed by a mass add.
	partial := false
	// Clusters whose list calls failed: their metric series are kept
	// rather than dropped for lack of data.
	partialClusters := map[string]bool{}
	clusterData := &model.ClusterData{}
	if len(s.k8sParsers) == 0 {
		// No cluster at all (see New): nothing to discover, nothing to persist.
		partial = true
	} else {
		var err error
		clusterData, err = s.k8sParsers[0].ParseAll(parseCtx)
		if clusterData == nil {
			clusterData = &model.ClusterData{}
		}
		if err != nil {
			slog.Warn("partial parse", "error", err)
			partial = true
			partialClusters[s.k8sParsers[0].ClusterName()] = true
		}
	}
	clusterData.PrimaryCluster = s.cfg.ClusterName

	// Merge full data from secondary clusters.
	for i, p := range s.k8sParsers {
		if i == 0 {
			continue
		}
		secondary, err := p.ParseAll(parseCtx)
		if err != nil {
			slog.Warn("partial parse", "error", err)
			partial = true
			partialClusters[p.ClusterName()] = true
		}
		if secondary == nil {
			continue
		}
		clusterData.Nodes = append(clusterData.Nodes, secondary.Nodes...)
		clusterData.Flux = append(clusterData.Flux, secondary.Flux...)
		clusterData.Gateways = append(clusterData.Gateways, secondary.Gateways...)
		clusterData.HTTPRoutes = append(clusterData.HTTPRoutes, secondary.HTTPRoutes...)
		clusterData.Namespaces = append(clusterData.Namespaces, secondary.Namespaces...)
		clusterData.SecurityPolicies = append(clusterData.SecurityPolicies, secondary.SecurityPolicies...)
		clusterData.ClientTrafficPolicies = append(clusterData.ClientTrafficPolicies, secondary.ClientTrafficPolicies...)
		clusterData.InfraSources = append(clusterData.InfraSources, secondary.InfraSources...)
		clusterData.ServiceEntries = append(clusterData.ServiceEntries, secondary.ServiceEntries...)
		clusterData.EastWestGateways = append(clusterData.EastWestGateways, secondary.EastWestGateways...)
		clusterData.LoadBalancers = append(clusterData.LoadBalancers, secondary.LoadBalancers...)
		clusterData.HelmReleases = append(clusterData.HelmReleases, secondary.HelmReleases...)
		clusterData.HelmRepositories = append(clusterData.HelmRepositories, secondary.HelmRepositories...)
		clusterData.GitRepositories = append(clusterData.GitRepositories, secondary.GitRepositories...)
		clusterData.Pods = append(clusterData.Pods, secondary.Pods...)
		clusterData.Workloads = append(clusterData.Workloads, secondary.Workloads...)
		clusterData.Storage = append(clusterData.Storage, secondary.Storage...)
		clusterData.CRDs = append(clusterData.CRDs, secondary.CRDs...)
		clusterData.Quotas = append(clusterData.Quotas, secondary.Quotas...)
		clusterData.Certificates = append(clusterData.Certificates, secondary.Certificates...)
		clusterData.NetworkPolicies = append(clusterData.NetworkPolicies, secondary.NetworkPolicies...)
		clusterData.Configs = append(clusterData.Configs, secondary.Configs...)
		clusterData.Services = append(clusterData.Services, secondary.Services...)
		clusterData.RBACBindings = append(clusterData.RBACBindings, secondary.RBACBindings...)
		clusterData.VeleroSchedules = append(clusterData.VeleroSchedules, secondary.VeleroSchedules...)
		clusterData.ImageVulns = append(clusterData.ImageVulns, secondary.ImageVulns...)
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

	// Cross-reference each image's CVEs with the cached KEV/EPSS data.
	// Pure in-memory map lookups — sub-millisecond even with thousands
	// of CVEs across hundreds of images.
	s.enrichImageVulns(clusterData.ImageVulns)

	// Emit Prometheus metrics keyed on (cluster, namespace, image) by
	// joining ImageVulns × Pods. Stale series are dropped inside the call,
	// except a partial cluster's.
	cvmetrics.EmitImageVulnMetrics(clusterData.Pods, clusterData.ImageVulns, partialClusters)

	diagrams := s.generate(clusterData)

	s.mu.Lock()
	s.gen++
	s.data = diagrams
	s.lastGen = time.Now()
	s.clusterData = clusterData
	s.lastPartial = partial
	s.partialClusters = partialClusters
	s.mu.Unlock()
	s.ready.Store(true)

	slog.Info("refresh complete", "duration", time.Since(start), "partial", partial)

	if st := s.eam.Load(); st != nil {
		// Persist a snapshot off the refresh path. Never under s.mu, never
		// fatal, skipped entirely when the parse was partial.
		s.goBackground(func() { s.captureSnapshot(ctx, clusterData, diagrams, partial) })

		// Run EAM discovery sync asynchronously, then AI enrichment for new
		// apps. A sync still running (manual trigger, slow DB) is left to
		// finish; the next refresh syncs again.
		s.goBackground(func() {
			result, ran := s.runSync(ctx, st, clusterData)
			if !ran {
				slog.Info("EAM sync already running, skipping this refresh's")
				return
			}
			if st.enricher != nil && result.AppsCreated > 0 {
				s.startEnrichment("refresh", st.enricher.EnrichNew)
			}
		})
	}

	s.startChecks(clusterData)
}

// startChecks runs the slow upstream lookups (chart and image registries,
// node OS feeds, OSV.dev) in the background. Updates arrive on a later page
// load. Each check is single-flight: while one is still running — a slow
// upstream — the next refresh does not start another, so they cannot pile
// up; the running one's result is rendered against whatever generation is
// current when it finishes.
func (s *Server) startChecks(cd *model.ClusterData) {
	s.runCheck(&s.chartsCheck, "charts", func() {
		s.checker.Check(cd.HelmRepositories, cd.HelmReleases)
	}, "charts", func(cur *model.ClusterData) model.DiagramResult {
		return diagram.GenerateVersions(cur, s.checker)
	})

	s.runCheck(&s.imagesCheck, "images", func() {
		s.imageChecker.Check(cd.Pods)
		// Pin findings need the registry digests the check just refreshed,
		// and describe the cluster as it is now, not as it was at launch.
		s.mu.RLock()
		cur, partial := s.clusterData, s.partialClusters
		s.mu.RUnlock()
		cvmetrics.EmitImagePinMetrics(cur.Pods, s.imageChecker.GetDigest, partial)
	}, "images", func(cur *model.ClusterData) model.DiagramResult {
		return diagram.GenerateImages(cur, s.imageChecker)
	})

	renderNodes := func(cur *model.ClusterData) model.DiagramResult {
		return diagram.GenerateNodes(cur, s.nodeChecker, s.securityChecker)
	}
	s.runCheck(&s.nodesCheck, "nodes", func() {
		s.nodeChecker.Check(cd.Nodes)
	}, "nodes", renderNodes)
	s.runCheck(&s.securityCheck, "node-security", func() {
		s.securityChecker.Check(versions.NodeSecurityQueries(cd.Nodes))
	}, "nodes", renderNodes)
}

// runCheck starts check in the background unless the previous run guarded
// by flag is still going, then re-renders diagram id through updateDiagram.
func (s *Server) runCheck(flag *atomic.Bool, name string, check func(), id string, render func(*model.ClusterData) model.DiagramResult) {
	if !flag.CompareAndSwap(false, true) {
		slog.Debug("upstream check still running, not starting another", "check", name)
		return
	}
	s.goBackground(func() {
		defer flag.Store(false)
		check()
		s.updateDiagram(id, render)
	})
}

// updateDiagram re-renders one diagram from the current generation and
// swaps it in — as a new slice, never in place (see Server). If another
// refresh publishes while render runs, the result belongs to an older
// generation and is dropped: the newer refresh rendered that diagram from
// newer data with the same checker state.
func (s *Server) updateDiagram(id string, render func(*model.ClusterData) model.DiagramResult) bool {
	s.mu.RLock()
	gen, cur := s.gen, s.clusterData
	s.mu.RUnlock()
	if cur == nil {
		return false
	}

	result := render(cur)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return false
	}
	for i, d := range s.data {
		if d.ID != id {
			continue
		}
		next := make([]model.DiagramResult, len(s.data))
		copy(next, s.data)
		next[i] = result
		s.data = next
		return true
	}
	return false
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
	// Copy-on-write: the slice taken here is never modified afterwards, so
	// it can be encoded without holding the lock.
	s.mu.RLock()
	resp := struct {
		Diagrams    []model.DiagramResult `json:"diagrams"`
		GeneratedAt time.Time             `json:"generated_at"`
	}{
		Diagrams:    s.data,
		GeneratedAt: s.lastGen,
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleHealth is the readiness probe: 503 until the first refresh has
// published, while the EAM database is still connecting, and whenever the
// connected database stops answering a ping. pgxpool's own health-check
// loop usually self-heals stuck connections, but if it can't, dropping the
// pod from Service endpoints lets kubelet recover.
//
// The chart's liveness probe is /api/health/live. The homelab release
// points liveness here instead, with a 120s initial delay and a 10-minute
// failure window; the first refresh and the (budgeted, five-minute) DB
// connect both fit in it.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"initializing"}`))
		return
	}
	if s.dbPending.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"db_connecting"}`))
		return
	}
	if db := s.database(); db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := db.Pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_down"}`))
			return
		}
	}
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleHealthLive is the liveness probe — cheap, process-only, answering
// from the moment the listener is up. Kept separate from /api/health so a
// transient DB outage drops the pod from the Service via readiness without
// also tripping liveness and restarting it while the pool's own
// health-check loop is mid-recovery.
func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	eamUp := s.eam.Load() != nil
	resp := struct {
		EAM       bool `json:"eam"`
		AI        bool `json:"ai"`
		Snapshots bool `json:"snapshots"`
	}{
		EAM:       eamUp,
		AI:        s.cfg.LiteLLMURL != "",
		Snapshots: eamUp,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Bounds for the EAM work started from a refresh or a manual POST. The
// sync is a few hundred upserts; enrichment is one LLM call per app at a
// concurrency of five, plus two batch prompts.
const (
	syncTimeout   = 5 * time.Minute
	enrichTimeout = 30 * time.Minute
)

// backgroundContext is the context work started from a request runs
// under: it outlives the request (a client that disconnects does not abort
// a sync halfway) but not the server.
func (s *Server) backgroundContext() context.Context {
	if s.bgCtx != nil {
		return s.bgCtx
	}
	return context.Background()
}

// runSync runs one EAM discovery sync, bounded by syncTimeout, unless one
// is already running (ran=false). Refresh-triggered and manual syncs share
// the guard: two concurrent syncs race each other's upserts.
func (s *Server) runSync(parent context.Context, st *eamState, cd *model.ClusterData) (result *discovery.SyncResult, ran bool) {
	if !s.syncRunning.CompareAndSwap(false, true) {
		return nil, false
	}
	defer s.syncRunning.Store(false)
	ctx, cancel := context.WithTimeout(parent, syncTimeout)
	defer cancel()
	return st.syncer.Sync(ctx, cd), true
}

// startEnrichment runs one AI enrichment pass in the background, bounded
// by enrichTimeout, unless one is already running (returns false).
func (s *Server) startEnrichment(what string, run func(ctx context.Context) error) bool {
	if !s.enrichRunning.CompareAndSwap(false, true) {
		return false
	}
	s.goBackground(func() {
		defer s.enrichRunning.Store(false)
		ctx, cancel := context.WithTimeout(s.backgroundContext(), enrichTimeout)
		defer cancel()
		if err := run(ctx); err != nil {
			slog.Error("ai enrichment failed", "trigger", what, "error", err)
		}
	})
	return true
}

// handleSyncTrigger — POST /api/eam/sync/trigger: one sync now, 409 while
// another is running.
func (s *Server) handleSyncTrigger(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cd := s.clusterData
	s.mu.RUnlock()
	if cd == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "no cluster data available yet"})
		return
	}

	st := s.eam.Load()
	result, ran := s.runSync(s.backgroundContext(), st, cd)
	if !ran {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "a sync is already running"})
		return
	}

	// Enrich the apps this sync created, unless an enrichment is running
	// already (it will pick them up: EnrichNew selects by state).
	if st.enricher != nil && result.AppsCreated > 0 {
		s.startEnrichment("sync", st.enricher.EnrichNew)
	}
	writeJSON(w, result)
}

// handleEnrich — POST /api/eam/enrich: a full enrichment pass in the
// background, 409 while one is running.
func (s *Server) handleEnrich(w http.ResponseWriter, r *http.Request) {
	st := s.eam.Load()
	if !s.startEnrichment("manual", st.enricher.EnrichAll) {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "an enrichment is already running"})
		return
	}
	writeJSONStatus(w, http.StatusAccepted, map[string]string{"status": "enrichment started"})
}

// handleSyncLogs — GET /api/eam/sync/logs: the 20 most recent syncs.
func (s *Server) handleSyncLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.database().ListSyncLogs(r.Context(), 20)
	if err != nil {
		writeErr(w, err)
		return
	}
	if logs == nil {
		logs = []store.SyncLog{}
	}
	writeJSON(w, logs)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// withCORS keeps the API readable cross-origin and makes it impossible to
// mutate cross-site. The browser never calls this API (web/ fetches it
// server-side, from the SSR loaders), so nothing legitimate needs more:
//
//   - preflight allows GET and HEAD only, so a browser will not send a
//     cross-origin POST with a JSON body;
//   - a POST must carry Content-Type: application/json. The "simple"
//     requests a page can send without preflight (form posts, text/plain)
//     are rejected with 415 before they reach a handler.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
			return
		case http.MethodGet, http.MethodHead:
		default:
			if !isJSONContent(r.Header.Get("Content-Type")) {
				writeJSONStatus(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isJSONContent(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	return err == nil && mt == "application/json"
}
