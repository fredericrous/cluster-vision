package versions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	lastCheck time.Time
	checking  atomic.Bool
	client    *http.Client
}

// NewSecurityChecker creates a new SecurityChecker.
func NewSecurityChecker() *SecurityChecker {
	return &SecurityChecker{
		cache: make(map[string]SecurityResult),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
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

	// Build OSV batch request
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
	for _, q := range deduped {
		oq := osvQuery{Version: q.Version}
		oq.Package.Name = q.Package
		oq.Package.Ecosystem = q.Ecosystem
		req.Queries = append(req.Queries, oq)
	}

	body, err := json.Marshal(req)
	if err != nil {
		slog.Warn("security check: failed to marshal request", "error", err)
		return
	}

	httpReq, err := http.NewRequest("POST", "https://api.osv.dev/v1/querybatch", bytes.NewReader(body))
	if err != nil {
		slog.Warn("security check: failed to create request", "error", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := sc.client.Do(httpReq)
	if err != nil {
		slog.Warn("security check: request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("security check: OSV API returned non-200", "status", resp.StatusCode)
		return
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		slog.Warn("security check: failed to read response", "error", err)
		return
	}

	var batchResp osvBatchResp
	if err := json.Unmarshal(respBody, &batchResp); err != nil {
		slog.Warn("security check: failed to parse response", "error", err)
		return
	}

	// Process results — one result per query
	for i, q := range deduped {
		if i >= len(batchResp.Results) {
			break
		}

		result := batchResp.Results[i]
		counts := map[string]int{
			"CRITICAL": 0,
			"HIGH":     0,
			"MEDIUM":   0,
			"LOW":      0,
		}

		for _, vuln := range result.Vulns {
			severity := extractSeverity(vuln.Severity, vuln.DatabaseSpecific.Severity)
			counts[severity]++
		}

		risk := SecurityRiskNone
		if counts["CRITICAL"] > 0 || counts["HIGH"] > 0 {
			risk = SecurityRiskCritical
		} else if counts["MEDIUM"] > 0 || counts["LOW"] > 0 {
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

	slog.Info("security check complete", "queries", len(deduped))
}

// OSV API response types.
type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type osvDBSpecific struct {
	Severity []osvSeverity `json:"severity"`
}

type osvVuln struct {
	ID               string        `json:"id"`
	DatabaseSpecific osvDBSpecific  `json:"database_specific"`
	Severity         []osvSeverity  `json:"severity"`
}

type osvResult struct {
	Vulns []osvVuln `json:"vulns"`
}

type osvBatchResp struct {
	Results []osvResult `json:"results"`
}

// extractSeverity determines the severity level from OSV severity data.
// It checks CVSS v3 score first, then falls back to CVSS v2.
func extractSeverity(vulnSeverity, dbSeverity []osvSeverity) string {
	// Try to extract from CVSS score
	for _, sev := range append(vulnSeverity, dbSeverity...) {
		if sev.Score == "" {
			continue
		}
		// CVSS vector strings contain the score — extract base score
		if strings.Contains(sev.Score, "CVSS:") {
			score := extractCVSSBaseScore(sev.Score)
			if score >= 9.0 {
				return "CRITICAL"
			} else if score >= 7.0 {
				return "HIGH"
			} else if score >= 4.0 {
				return "MEDIUM"
			} else if score > 0 {
				return "LOW"
			}
		}
	}

	// Default to MEDIUM if we have a vuln but can't determine severity
	return "MEDIUM"
}

// extractCVSSBaseScore parses a CVSS v3 or v2 vector string to extract the base score.
// For simplicity, we derive severity from the vector's AV/AC/PR/UI metrics.
func extractCVSSBaseScore(vector string) float64 {
	// Try to find an explicit score at the end (some formats include it)
	// CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H → 9.8
	// We estimate from attack complexity and impact instead
	parts := strings.Split(vector, "/")

	attackVector := ""
	attackComplexity := ""
	confidentiality := ""
	integrity := ""
	availability := ""

	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "AV":
			attackVector = kv[1]
		case "AC":
			attackComplexity = kv[1]
		case "C":
			confidentiality = kv[1]
		case "I":
			integrity = kv[1]
		case "A":
			availability = kv[1]
		}
	}

	// Simple heuristic scoring based on vector components
	score := 5.0

	// Attack vector: N(etwork) > A(djacent) > L(ocal) > P(hysical)
	switch attackVector {
	case "N":
		score += 2.0
	case "A":
		score += 1.0
	case "L":
		score += 0.5
	}

	// Attack complexity: L(ow) adds more risk
	if attackComplexity == "L" {
		score += 1.0
	}

	// Impact: H(igh) on any CIA adds risk
	for _, impact := range []string{confidentiality, integrity, availability} {
		if impact == "H" {
			score += 0.5
		}
	}

	if score > 10.0 {
		score = 10.0
	}
	return score
}

// buildVulnSummary creates a human-readable summary from severity counts.
func buildVulnSummary(counts map[string]int) string {
	var parts []string
	for _, level := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		if c := counts[level]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, strings.ToLower(level)))
		}
	}
	return strings.Join(parts, ", ")
}
