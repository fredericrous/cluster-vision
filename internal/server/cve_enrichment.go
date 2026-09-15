package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/fredericrous/cluster-vision/internal/versions"
)

const (
	// maxEnrichmentCVEs bounds one request. Callers chunk larger scans.
	maxEnrichmentCVEs = 500
	// maxEnrichmentBody bounds the request body independently of the id
	// count — 500 ids is ~10KiB, so 64KiB leaves room without letting an
	// unbounded body into memory.
	maxEnrichmentBody = 64 << 10
	// enrichmentStaleAfter is how old the intel may be before a caller
	// must stop treating "no KEV hit" as evidence of anything. The feeds
	// refresh daily; 72h tolerates two missed refreshes.
	enrichmentStaleAfter = 72 * time.Hour
)

// cveIntelSource is the slice of the exploit enricher this endpoint needs.
// It exists because Server.exploit is a concrete *versions.ExploitEnricher:
// the test seam belongs on the handler, not on the server struct.
type cveIntelSource interface {
	LookupOK(string) (versions.Enrichment, bool)
	LastFetch() time.Time
	CacheSize() (kev, epss int)
}

// cveIntel is the per-CVE answer. EPSS is the raw probability (0..1).
type cveIntel struct {
	CVE  string  `json:"cve"`
	KEV  bool    `json:"kev"`
	EPSS float64 `json:"epss"`
}

type enrichmentRequest struct {
	CVEs []string `json:"cves"`
}

// enrichmentResponse carries the cache's own state alongside the results.
// Without FetchedAt/KEVTotal/Stale a caller cannot tell "this image has no
// exploitable CVEs" from "this cache is empty", and would read a cold or
// broken enricher as a clean image.
type enrichmentResponse struct {
	Results   []cveIntel `json:"results"` // known CVEs only
	Unknown   []string   `json:"unknown"`
	FetchedAt time.Time  `json:"fetched_at"`
	KEVTotal  int        `json:"kev_total"`
	EPSSTotal int        `json:"epss_total"`
	Stale     bool       `json:"stale"`
}

// handleCVEEnrichment answers POST /api/cve/enrichment from the in-memory
// KEV/EPSS cache — no SQL, so it keeps answering while Postgres is down.
//
// Ids that do not look like a CVE (GHSA, DSA, ALAS, …) are not looked up;
// they come back under "unknown" rather than 400, so a caller that passes a
// scanner's raw id list still gets a usable verdict for the rest.
func handleCVEEnrichment(src cveIntelSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The route is registered as "POST /api/cve/enrichment", so the mux
		// already rejects other methods; this keeps the handler correct on
		// its own (and is what the test exercises).
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxEnrichmentBody)
		var req enrichmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if len(req.CVEs) == 0 {
			writeJSONError(w, http.StatusBadRequest, "cves must not be empty")
			return
		}
		if len(req.CVEs) > maxEnrichmentCVEs {
			writeJSONError(w, http.StatusBadRequest, "too many cves (max 500)")
			return
		}

		results := make([]cveIntel, 0, len(req.CVEs))
		unknown := make([]string, 0)
		seen := make(map[string]struct{}, len(req.CVEs))
		for _, raw := range req.CVEs {
			id := strings.ToUpper(strings.TrimSpace(raw))
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}

			if !strings.HasPrefix(id, "CVE-") {
				unknown = append(unknown, id)
				continue
			}
			en, ok := src.LookupOK(id)
			if !ok {
				unknown = append(unknown, id)
				continue
			}
			results = append(results, cveIntel{CVE: id, KEV: en.KEV, EPSS: en.EPSSScore})
		}

		kevTotal, epssTotal := src.CacheSize()
		fetchedAt := src.LastFetch()
		resp := enrichmentResponse{
			Results:   results,
			Unknown:   unknown,
			FetchedAt: fetchedAt,
			KEVTotal:  kevTotal,
			EPSSTotal: epssTotal,
			Stale:     fetchedAt.IsZero() || time.Since(fetchedAt) > enrichmentStaleAfter,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{msg})
}
