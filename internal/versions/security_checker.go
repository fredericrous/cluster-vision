package versions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// SecurityRisk represents the severity level of known vulnerabilities.
type SecurityRisk string

const (
	SecurityRiskCritical SecurityRisk = "critical" // CRITICAL or HIGH
	SecurityRiskWarning  SecurityRisk = "warning"  // MEDIUM or LOW
	SecurityRiskNone     SecurityRisk = "none"     // no vulns
	SecurityRiskUnknown  SecurityRisk = ""         // not checked
)

// SecurityQuery represents a package to check for vulnerabilities.
type SecurityQuery struct {
	Ecosystem string
	Package   string
	Version   string
}

// SecurityResult holds the vulnerability check result for a package+version.
type SecurityResult struct {
	Risk    SecurityRisk
	Summary string // e.g. "2 critical, 1 high"
}

// knownDistroModules maps distro names to their Go module paths for OSV lookup.
var knownDistroModules = map[string]string{
	"talos": "github.com/siderolabs/talos",
}

// KnownDistroModule returns the Go module path for a given distro, if known.
func KnownDistroModule(distro string) (string, bool) {
	mod, ok := knownDistroModules[distro]
	return mod, ok
}

// SecurityChecker queries OSV.dev for known vulnerabilities in node packages.
type SecurityChecker struct {
	mu        sync.RWMutex
	cache     map[string]SecurityResult // key: "ecosystem/pkg@version"
	records   map[string]osvRecord      // full OSV records by ID
	lastCheck time.Time
	checking  atomic.Bool
	client    *http.Client
	// osvBaseURL is https://api.osv.dev; tests point it at a fake.
	osvBaseURL string
}

// osvFetchConcurrency bounds the parallel GET /v1/vulns/{id} requests.
const osvFetchConcurrency = 8

// NewSecurityChecker creates a new SecurityChecker.
func NewSecurityChecker() *SecurityChecker {
	return &SecurityChecker{
		cache:   make(map[string]SecurityResult),
		records: make(map[string]osvRecord),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		osvBaseURL: "https://api.osv.dev",
	}
}

func securityCacheKey(ecosystem, pkg, version string) string {
	return ecosystem + "/" + pkg + "@" + version
}

// GetResult returns the cached security result for a given package.
func (sc *SecurityChecker) GetResult(ecosystem, pkg, version string) SecurityResult {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.cache[securityCacheKey(ecosystem, pkg, version)]
}

// NodeSecurityQueries builds SecurityQuery entries from node data.
func NodeSecurityQueries(nodes []model.NodeInfo) []SecurityQuery {
	seen := make(map[string]bool)
	var queries []SecurityQuery

	for _, n := range nodes {
		// Distro query
		distro, ver := ParseOSImage(n.OSImage)
		if mod, ok := knownDistroModules[distro]; ok && ver != "" {
			key := "Go/" + mod + "@" + ver
			if !seen[key] {
				seen[key] = true
				queries = append(queries, SecurityQuery{
					Ecosystem: "Go",
					Package:   mod,
					Version:   "v" + strings.TrimPrefix(ver, "v"),
				})
			}
		}

		// Kubelet query
		if n.KubeletVersion != "" {
			ver := n.KubeletVersion
			key := "Go/k8s.io/kubernetes@" + ver
			if !seen[key] {
				seen[key] = true
				queries = append(queries, SecurityQuery{
					Ecosystem: "Go",
					Package:   "k8s.io/kubernetes",
					Version:   strings.TrimPrefix(ver, "v"),
				})
			}
		}
	}

	return queries
}

// Check queries the OSV.dev batch API for vulnerabilities.
// Single-flight: returns immediately if already checking.
// Interval gate: skips if last check was less than 15 minutes ago.
func (sc *SecurityChecker) Check(queries []SecurityQuery) {
	if len(queries) == 0 {
		return
	}

	if !sc.checking.CompareAndSwap(false, true) {
		return
	}
	defer sc.checking.Store(false)

	sc.mu.RLock()
	tooSoon := time.Since(sc.lastCheck) < 15*time.Minute
	sc.mu.RUnlock()
	if tooSoon {
		return
	}

	// Dedup queries
	seen := make(map[string]bool)
	var deduped []SecurityQuery
	for _, q := range queries {
		key := securityCacheKey(q.Ecosystem, q.Package, q.Version)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, q)
		}
	}

	batchResp, err := sc.queryBatch(deduped)
	if err != nil {
		slog.Warn("security check: OSV batch query failed", "error", err)
		return
	}

	// The batch endpoint returns only {id, modified} per vuln; severity and
	// aliases live in the full record, fetched (and cached) per ID.
	var refs []osvVulnRef
	for _, r := range batchResp.Results {
		refs = append(refs, r.Vulns...)
	}
	sc.fetchRecords(refs)

	// Process results — one result per query
	for i, q := range deduped {
		if i >= len(batchResp.Results) {
			break
		}

		counts := map[string]int{}
		for _, sev := range sc.issueSeverities(batchResp.Results[i].Vulns) {
			counts[sev]++
		}

		risk := SecurityRiskNone
		if counts["CRITICAL"] > 0 || counts["HIGH"] > 0 {
			risk = SecurityRiskCritical
		} else if len(counts) > 0 {
			risk = SecurityRiskWarning
		}

		summary := buildVulnSummary(counts)

		key := securityCacheKey(q.Ecosystem, q.Package, q.Version)
		sc.mu.Lock()
		sc.cache[key] = SecurityResult{Risk: risk, Summary: summary}
		sc.mu.Unlock()
	}

	sc.mu.Lock()
	sc.lastCheck = time.Now()
	sc.mu.Unlock()

	slog.Info("security check complete", "queries", len(deduped), "vulns", len(refs))
}

// queryBatch posts the queries to OSV's querybatch endpoint.
func (sc *SecurityChecker) queryBatch(queries []SecurityQuery) (*osvBatchResp, error) {
	type osvQuery struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Version string `json:"version"`
	}

	type osvBatchReq struct {
		Queries []osvQuery `json:"queries"`
	}

	req := osvBatchReq{}
	for _, q := range queries {
		oq := osvQuery{Version: q.Version}
		oq.Package.Name = q.Package
		oq.Package.Ecosystem = q.Ecosystem
		req.Queries = append(req.Queries, oq)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", sc.osvBaseURL+"/v1/querybatch", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var batchResp osvBatchResp
	if err := sc.doJSON(httpReq, &batchResp); err != nil {
		return nil, err
	}
	return &batchResp, nil
}

// fetchRecords makes sure the full record of every ref is cached, refetching
// one whose modified timestamp moved. Failures are logged and leave the
// record missing; its vuln is then counted with an unknown severity.
func (sc *SecurityChecker) fetchRecords(refs []osvVulnRef) {
	var ids []string
	seen := make(map[string]bool)
	sc.mu.RLock()
	for _, ref := range refs {
		if seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		if rec, ok := sc.records[ref.ID]; !ok || rec.Modified != ref.Modified {
			ids = append(ids, ref.ID)
		}
	}
	sc.mu.RUnlock()
	sc.fetchIDs(ids)
}

// fetchIDs fetches and caches the given records, a few at a time.
func (sc *SecurityChecker) fetchIDs(ids []string) {
	sem := make(chan struct{}, osvFetchConcurrency)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			rec, err := sc.fetchRecord(id)
			if err != nil {
				slog.Warn("security check: failed to fetch OSV record", "id", id, "error", err)
				return
			}
			sc.mu.Lock()
			sc.records[id] = rec
			sc.mu.Unlock()
		}(id)
	}
	wg.Wait()
}

func (sc *SecurityChecker) fetchRecord(id string) (osvRecord, error) {
	var rec osvRecord
	httpReq, err := http.NewRequest("GET", sc.osvBaseURL+"/v1/vulns/"+url.PathEscape(id), nil)
	if err != nil {
		return rec, err
	}
	err = sc.doJSON(httpReq, &rec)
	return rec, err
}

func (sc *SecurityChecker) doJSON(req *http.Request, out any) error {
	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OSV API returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

// issueSeverities returns one severity per distinct issue among refs.
//
// OSV lists the same issue once per database that tracks it (a Go vulndb
// GO-… record and its GHSA-…, linked through aliases), so refs are first
// merged into groups of mutual aliases and each group counts once. A group
// takes the highest severity any member states. Go vulndb records carry no
// severity at all, so a group whose members are all silent has its aliases
// fetched too, GHSA before CVE; if none of them says either, the issue is
// counted as UNKNOWN rather than given a made-up rating.
func (sc *SecurityChecker) issueSeverities(refs []osvVulnRef) []string {
	parent := make(map[string]string)
	var find func(string) string
	find = func(id string) string {
		if p, ok := parent[id]; ok && p != id {
			root := find(p)
			parent[id] = root
			return root
		}
		parent[id] = id
		return id
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	sc.mu.RLock()
	records := make(map[string]osvRecord, len(refs))
	for _, ref := range refs {
		find(ref.ID)
		if rec, ok := sc.records[ref.ID]; ok {
			records[ref.ID] = rec
			for _, alias := range rec.Aliases {
				union(ref.ID, alias)
			}
		}
	}
	sc.mu.RUnlock()

	groups := make(map[string][]string) // root -> member IDs present in refs
	var roots []string
	for _, ref := range refs {
		root := find(ref.ID)
		if _, ok := groups[root]; !ok {
			roots = append(roots, root)
		}
		groups[root] = append(groups[root], ref.ID)
	}

	// Groups no present member rates consult their aliases, one database
	// tier at a time so a GHSA that answers spares the CVE fetch.
	sev := make(map[string]string, len(roots))
	for _, root := range roots {
		sev[root] = groupSeverity(groups[root], records)
	}
	for _, prefix := range []string{"GHSA-", "CVE-"} {
		consult := make(map[string][]string) // root -> aliases of this tier
		var missing []string
		sc.mu.RLock()
		for _, root := range roots {
			if sev[root] != "" {
				continue
			}
			for _, id := range groups[root] {
				for _, alias := range records[id].Aliases {
					if !strings.HasPrefix(alias, prefix) {
						continue
					}
					consult[root] = append(consult[root], alias)
					if _, cached := sc.records[alias]; !cached {
						missing = append(missing, alias)
					}
				}
			}
		}
		sc.mu.RUnlock()
		if len(consult) == 0 {
			continue
		}
		sc.fetchIDs(dedupStrings(missing))

		sc.mu.RLock()
		for root, aliases := range consult {
			aliasRecords := make(map[string]osvRecord, len(aliases))
			for _, alias := range aliases {
				if rec, ok := sc.records[alias]; ok {
					aliasRecords[alias] = rec
				}
			}
			sev[root] = groupSeverity(aliases, aliasRecords)
		}
		sc.mu.RUnlock()
	}

	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if sev[root] == "" {
			sev[root] = "UNKNOWN"
		}
		out = append(out, sev[root])
	}
	return out
}

// groupSeverity returns the highest severity any of ids' records states,
// or "" when none does.
func groupSeverity(ids []string, records map[string]osvRecord) string {
	best := ""
	for _, id := range ids {
		rec, ok := records[id]
		if !ok {
			continue
		}
		if sev := extractSeverity(rec); severityRank[sev] > severityRank[best] {
			best = sev
		}
	}
	return best
}

func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// OSV API response types.
type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

// osvRecord is the part of a full OSV record (GET /v1/vulns/{id}) used here.
type osvRecord struct {
	ID       string        `json:"id"`
	Modified string        `json:"modified"`
	Aliases  []string      `json:"aliases"`
	Severity []osvSeverity `json:"severity"`
	// database_specific is free-form per database. GHSA records put a
	// rating string in its "severity" field; others use another shape or
	// nothing, so it is decoded lazily and tolerantly.
	DatabaseSpecific json.RawMessage `json:"database_specific"`
}

// osvVulnRef is what the querybatch endpoint returns per vuln.
type osvVulnRef struct {
	ID       string `json:"id"`
	Modified string `json:"modified"`
}

type osvResult struct {
	Vulns []osvVulnRef `json:"vulns"`
}

type osvBatchResp struct {
	Results []osvResult `json:"results"`
}

var severityRank = map[string]int{"": 0, "UNKNOWN": 1, "LOW": 2, "MEDIUM": 3, "HIGH": 4, "CRITICAL": 5}

// extractSeverity rates a full OSV record: the CVSS v3 base score when the
// record has a v3 vector, else the database's own rating (GHSA's
// CRITICAL/HIGH/MODERATE/LOW), else "" for unrated. CVSS v2 and v4 vectors
// are not scored here; GHSA records that carry only v4 still have a rating.
func extractSeverity(rec osvRecord) string {
	for _, sev := range rec.Severity {
		if sev.Type != "CVSS_V3" {
			continue
		}
		if score, ok := cvss3BaseScore(sev.Score); ok && score > 0 {
			return cvssSeverity(score)
		}
	}

	var dbSpecific struct {
		Severity any `json:"severity"`
	}
	if len(rec.DatabaseSpecific) > 0 && json.Unmarshal(rec.DatabaseSpecific, &dbSpecific) == nil {
		if s, ok := dbSpecific.Severity.(string); ok {
			switch strings.ToUpper(s) {
			case "CRITICAL":
				return "CRITICAL"
			case "HIGH":
				return "HIGH"
			case "MODERATE", "MEDIUM":
				return "MEDIUM"
			case "LOW":
				return "LOW"
			}
		}
	}
	return ""
}

// buildVulnSummary creates a human-readable summary from severity counts.
func buildVulnSummary(counts map[string]int) string {
	var parts []string
	for _, level := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
		if c := counts[level]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, strings.ToLower(level)))
		}
	}
	return strings.Join(parts, ", ")
}
