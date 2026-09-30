package versions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCVSS3BaseScore(t *testing.T) {
	tests := []struct {
		vector string
		want   float64
	}{
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L", 5.3},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", 10.0},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:C/C:L/I:L/A:N", 6.1},
		{"CVSS:3.1/AV:N/AC:L/PR:H/UI:N/S:U/C:H/I:H/A:N", 6.5},
		{"CVSS:3.1/AV:L/AC:H/PR:L/UI:N/S:C/C:H/I:H/A:N", 7.5},
		{"CVSS:3.0/AV:L/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H", 7.8},
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N", 0},
	}
	for _, tt := range tests {
		got, ok := cvss3BaseScore(tt.vector)
		if !ok || got != tt.want {
			t.Errorf("cvss3BaseScore(%q) = %v, %v; want %v, true", tt.vector, got, ok, tt.want)
		}
	}

	for _, bad := range []string{
		"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N",
		"AV:N/AC:L/Au:N/C:P/I:P/A:P",
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H", // no A
	} {
		if _, ok := cvss3BaseScore(bad); ok {
			t.Errorf("cvss3BaseScore(%q) ok = true; want false", bad)
		}
	}
}

func TestExtractSeverity(t *testing.T) {
	tests := []struct {
		name string
		rec  string
		want string
	}{
		{"cvss v3 wins over rating", `{"severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"}],"database_specific":{"severity":"MODERATE"}}`, "CRITICAL"},
		{"ghsa rating", `{"severity":[{"type":"CVSS_V4","score":"CVSS:4.0/AV:N"}],"database_specific":{"severity":"MODERATE"}}`, "MEDIUM"},
		{"low vector", `{"severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:P/AC:H/PR:H/UI:R/S:U/C:L/I:N/A:N"}]}`, "LOW"},
		{"go vulndb record has nothing", `{"id":"GO-2025-3465"}`, ""},
		{"database_specific of another shape", `{"database_specific":{"severity":[{"type":"x"}]}}`, ""},
	}
	for _, tt := range tests {
		var rec osvRecord
		if err := json.Unmarshal([]byte(tt.rec), &rec); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := extractSeverity(rec); got != tt.want {
			t.Errorf("%s: extractSeverity = %q; want %q", tt.name, got, tt.want)
		}
	}
}

// fakeOSV serves querybatch the way api.osv.dev does — {id, modified} only —
// and full records from records. It counts record fetches per ID.
func fakeOSV(t *testing.T, batch [][]string, records map[string]string) (*httptest.Server, map[string]int) {
	t.Helper()
	fetches := map[string]int{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/querybatch":
			var resp osvBatchResp
			for _, ids := range batch {
				var res osvResult
				for _, id := range ids {
					res.Vulns = append(res.Vulns, osvVulnRef{ID: id, Modified: "2026-01-01T00:00:00Z"})
				}
				resp.Results = append(resp.Results, res)
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/vulns/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/vulns/")
			mu.Lock()
			fetches[id]++
			mu.Unlock()
			rec, ok := records[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(rec))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, fetches
}

func TestCheckRatesDistinctIssues(t *testing.T) {
	const modified = `"modified":"2026-01-01T00:00:00Z"`
	records := map[string]string{
		// A Go vulndb record and its GHSA twin: one issue, rated by the GHSA.
		"GO-2025-0001":        `{"id":"GO-2025-0001",` + modified + `,"aliases":["CVE-2025-0001","GHSA-aaaa-aaaa-aaaa"]}`,
		"GHSA-aaaa-aaaa-aaaa": `{"id":"GHSA-aaaa-aaaa-aaaa",` + modified + `,"aliases":["CVE-2025-0001","GO-2025-0001"],"severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"}]}`,
		// A Go vulndb record alone in the batch; its GHSA alias is fetched.
		"GO-2025-0002":        `{"id":"GO-2025-0002",` + modified + `,"aliases":["CVE-2025-0002","GHSA-bbbb-bbbb-bbbb"]}`,
		"GHSA-bbbb-bbbb-bbbb": `{"id":"GHSA-bbbb-bbbb-bbbb","aliases":["CVE-2025-0002"],"database_specific":{"severity":"HIGH"}}`,
		// Nothing anywhere rates this one.
		"GO-2025-0003": `{"id":"GO-2025-0003",` + modified + `}`,
		// A low one, to prove the old 5.0+ floor is gone.
		"GHSA-cccc-cccc-cccc": `{"id":"GHSA-cccc-cccc-cccc",` + modified + `,"severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:P/AC:H/PR:H/UI:R/S:U/C:L/I:N/A:N"}]}`,
	}
	batch := [][]string{
		{"GO-2025-0001", "GHSA-aaaa-aaaa-aaaa", "GO-2025-0002", "GO-2025-0003"},
		{"GHSA-cccc-cccc-cccc"},
		{},
	}
	srv, fetches := fakeOSV(t, batch, records)

	sc := NewSecurityChecker()
	sc.osvBaseURL = srv.URL
	queries := []SecurityQuery{
		{Ecosystem: "Go", Package: "k8s.io/kubernetes", Version: "1.31.0"},
		{Ecosystem: "Go", Package: "github.com/siderolabs/talos", Version: "v1.8.0"},
		{Ecosystem: "Go", Package: "example.com/clean", Version: "v1.0.0"},
	}
	sc.Check(queries)

	got := sc.GetResult("Go", "k8s.io/kubernetes", "1.31.0")
	want := SecurityResult{Risk: SecurityRiskCritical, Summary: "1 critical, 1 high, 1 unknown"}
	if got != want {
		t.Errorf("kubernetes = %+v; want %+v", got, want)
	}
	got = sc.GetResult("Go", "github.com/siderolabs/talos", "v1.8.0")
	want = SecurityResult{Risk: SecurityRiskWarning, Summary: "1 low"}
	if got != want {
		t.Errorf("talos = %+v; want %+v", got, want)
	}
	got = sc.GetResult("Go", "example.com/clean", "v1.0.0")
	want = SecurityResult{Risk: SecurityRiskNone}
	if got != want {
		t.Errorf("clean = %+v; want %+v", got, want)
	}

	// A second check with unchanged modified timestamps refetches nothing.
	before := map[string]int{}
	for k, v := range fetches {
		before[k] = v
	}
	sc.lastCheck = sc.lastCheck.Add(-16 * time.Minute)
	sc.Check(queries)
	for id, n := range fetches {
		if n != before[id] {
			t.Errorf("record %s refetched although unchanged (%d -> %d)", id, before[id], n)
		}
	}
}
