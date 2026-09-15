package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fredericrous/cluster-vision/internal/versions"
)

// fakeIntel is a cveIntelSource with no moving parts: the handler's whole
// job is reading a cache and reporting its state, so the cache is a literal.
type fakeIntel struct {
	entries   map[string]versions.Enrichment
	lastFetch time.Time
	kev       int
	epss      int
}

func (f fakeIntel) LookupOK(id string) (versions.Enrichment, bool) {
	en, ok := f.entries[strings.ToUpper(id)]
	return en, ok
}
func (f fakeIntel) LastFetch() time.Time  { return f.lastFetch }
func (f fakeIntel) CacheSize() (int, int) { return f.kev, f.epss }

func warmIntel() fakeIntel {
	return fakeIntel{
		entries: map[string]versions.Enrichment{
			"CVE-2024-0001": {KEV: true, EPSSScore: 0.94, EPSSPercentile: 0.99},
			"CVE-2024-0002": {EPSSScore: 0.004},
			"CVE-2024-0003": {},
		},
		lastFetch: time.Now().Add(-2 * time.Hour),
		kev:       1200,
		epss:      280000,
	}
}

func postEnrichment(t *testing.T, src cveIntelSource, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/cve/enrichment", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleCVEEnrichment(src).ServeHTTP(rec, req)
	return rec
}

func decodeEnrichment(t *testing.T, rec *httptest.ResponseRecorder) enrichmentResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	var resp enrichmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	return resp
}

func TestCVEEnrichmentHitAndMiss(t *testing.T) {
	// Lowercase input must still hit: scanners are not consistent about case.
	resp := decodeEnrichment(t, postEnrichment(t, warmIntel(),
		`{"cves":["cve-2024-0001","CVE-2024-0003","CVE-2030-9999"]}`))

	if len(resp.Results) != 2 {
		t.Fatalf("results = %+v, want 2", resp.Results)
	}
	if resp.Results[0] != (cveIntel{CVE: "CVE-2024-0001", KEV: true, EPSS: 0.94}) {
		t.Errorf("results[0] = %+v", resp.Results[0])
	}
	// Known-and-clean is a result, not an unknown — that distinction is the
	// whole reason LookupOK exists.
	if resp.Results[1] != (cveIntel{CVE: "CVE-2024-0003"}) {
		t.Errorf("results[1] = %+v", resp.Results[1])
	}
	if len(resp.Unknown) != 1 || resp.Unknown[0] != "CVE-2030-9999" {
		t.Errorf("unknown = %v", resp.Unknown)
	}
	if resp.Stale {
		t.Error("stale = true for a cache fetched 2h ago")
	}
	if resp.KEVTotal != 1200 || resp.EPSSTotal != 280000 {
		t.Errorf("totals = %d/%d", resp.KEVTotal, resp.EPSSTotal)
	}
}

func TestCVEEnrichmentStale(t *testing.T) {
	src := warmIntel()
	src.lastFetch = time.Now().Add(-73 * time.Hour)
	if resp := decodeEnrichment(t, postEnrichment(t, src, `{"cves":["CVE-2024-0001"]}`)); !resp.Stale {
		t.Error("stale = false for intel older than 72h")
	}

	// 71h is inside the window.
	src.lastFetch = time.Now().Add(-71 * time.Hour)
	if resp := decodeEnrichment(t, postEnrichment(t, src, `{"cves":["CVE-2024-0001"]}`)); resp.Stale {
		t.Error("stale = true for intel 71h old")
	}
}

func TestCVEEnrichmentEmptyCache(t *testing.T) {
	// Never fetched: every id is unknown and the envelope must say so
	// loudly enough that a caller cannot read it as a clean image.
	resp := decodeEnrichment(t, postEnrichment(t, fakeIntel{}, `{"cves":["CVE-2024-0001"]}`))
	if len(resp.Results) != 0 {
		t.Errorf("results = %+v, want none", resp.Results)
	}
	if len(resp.Unknown) != 1 {
		t.Errorf("unknown = %v", resp.Unknown)
	}
	if !resp.Stale || resp.KEVTotal != 0 || !resp.FetchedAt.IsZero() {
		t.Errorf("empty cache reported as stale=%v kev_total=%d fetched_at=%v",
			resp.Stale, resp.KEVTotal, resp.FetchedAt)
	}
}

func TestCVEEnrichmentDropsNonCVEIDs(t *testing.T) {
	// GHSA/DSA ids are never looked up, but they are reported rather than
	// rejected, so a caller passing a scanner's raw list still gets a
	// verdict for the CVEs in it.
	resp := decodeEnrichment(t, postEnrichment(t, warmIntel(),
		`{"cves":["GHSA-xxxx-yyyy-zzzz","CVE-2024-0001","  ","DSA-5555-1"]}`))
	if len(resp.Results) != 1 || resp.Results[0].CVE != "CVE-2024-0001" {
		t.Fatalf("results = %+v", resp.Results)
	}
	want := []string{"GHSA-XXXX-YYYY-ZZZZ", "DSA-5555-1"}
	if len(resp.Unknown) != len(want) {
		t.Fatalf("unknown = %v, want %v", resp.Unknown, want)
	}
	for i, id := range want {
		if resp.Unknown[i] != id {
			t.Errorf("unknown[%d] = %q, want %q", i, resp.Unknown[i], id)
		}
	}

	// An all-filtered list is still a 200: leniency here beats a 400 the
	// caller's error ladder would read as terminal.
	resp = decodeEnrichment(t, postEnrichment(t, warmIntel(), `{"cves":["GHSA-a-b-c"]}`))
	if len(resp.Results) != 0 || len(resp.Unknown) != 1 {
		t.Fatalf("all-filtered list: results=%+v unknown=%v", resp.Results, resp.Unknown)
	}
}

func TestCVEEnrichmentDeduplicates(t *testing.T) {
	resp := decodeEnrichment(t, postEnrichment(t, warmIntel(),
		`{"cves":["CVE-2024-0001","cve-2024-0001","CVE-2030-1","CVE-2030-1"]}`))
	if len(resp.Results) != 1 || len(resp.Unknown) != 1 {
		t.Fatalf("results=%+v unknown=%v", resp.Results, resp.Unknown)
	}
}

func TestCVEEnrichmentBadRequests(t *testing.T) {
	cases := map[string]string{
		"empty list":  `{"cves":[]}`,
		"no field":    `{}`,
		"broken json": `{"cves":`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := postEnrichment(t, warmIntel(), body); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestCVEEnrichmentTooManyIDs(t *testing.T) {
	ids := make([]string, maxEnrichmentCVEs+1)
	for i := range ids {
		ids[i] = "CVE-2024-0001"
	}
	body, err := json.Marshal(enrichmentRequest{CVEs: ids})
	if err != nil {
		t.Fatal(err)
	}
	rec := postEnrichment(t, warmIntel(), string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	// Exactly at the limit is accepted.
	body, err = json.Marshal(enrichmentRequest{CVEs: ids[:maxEnrichmentCVEs]})
	if err != nil {
		t.Fatal(err)
	}
	if rec := postEnrichment(t, warmIntel(), string(body)); rec.Code != http.StatusOK {
		t.Fatalf("status at limit = %d, want 200", rec.Code)
	}
}

func TestCVEEnrichmentOversizedBody(t *testing.T) {
	body := `{"cves":["CVE-2024-0001","` + strings.Repeat("A", maxEnrichmentBody) + `"]}`
	if rec := postEnrichment(t, warmIntel(), body); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCVEEnrichmentMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/cve/enrichment", nil)
	rec := httptest.NewRecorder()
	handleCVEEnrichment(warmIntel()).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Errorf("Allow = %q", got)
	}
}

// The route must not live behind the DB-conditional block: a caller that
// gates a deployment on this data would otherwise read a Postgres outage as
// a 404 and, depending on its ladder, as a clean image.
func TestCVEEnrichmentRegisteredWithoutDB(t *testing.T) {
	s := &Server{exploit: versions.NewExploitEnricher(nil)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/cve/enrichment", handleCVEEnrichment(s.exploit))

	req := httptest.NewRequest(http.MethodPost, "/api/cve/enrichment",
		strings.NewReader(`{"cves":["CVE-2024-0001"]}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !decodeEnrichment(t, rec).Stale {
		t.Error("a never-warmed enricher must report stale")
	}
}
