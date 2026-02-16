package versions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"

	"gopkg.in/yaml.v3"
)

// Checker periodically fetches latest chart versions from Helm repositories.
type Checker struct {
	mu       sync.RWMutex
	latest   map[string]string // "repoURL/chartName" → latest version
	interval time.Duration
	client   *http.Client
}

// NewChecker creates a version checker with the given check interval.
func NewChecker(interval time.Duration) *Checker {
	return &Checker{
		latest:   make(map[string]string),
		interval: interval,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start launches the background checking goroutine.
func (c *Checker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Check is called from refresh(), not from here.
				// The ticker just exists to allow future autonomous re-checks.
			}
		}
	}()
}

// Check fetches latest versions for all unique repo+chart combinations.
func (c *Checker) Check(repos []model.HelmRepositoryInfo, releases []model.HelmReleaseInfo) {
	// Build repo lookup: "namespace/name" → HelmRepositoryInfo
	repoByKey := make(map[string]model.HelmRepositoryInfo)
	for _, r := range repos {
		repoByKey[r.Namespace+"/"+r.Name] = r
	}

	// Collect unique chart+repo pairs
	type chartRef struct {
		repoURL   string
		repoType  string
		chartName string
	}
	seen := make(map[string]bool)
	var checks []chartRef

	for _, rel := range releases {
		repo, ok := repoByKey[rel.RepoNS+"/"+rel.RepoName]
		if !ok {
			continue
		}

		key := repo.URL + "/" + rel.ChartName
		if seen[key] {
			continue
		}
		seen[key] = true
		checks = append(checks, chartRef{
			repoURL:   repo.URL,
			repoType:  repo.Type,
			chartName: rel.ChartName,
		})
	}

	results := make(map[string]string)

	for _, ch := range checks {
		key := ch.repoURL + "/" + ch.chartName

		var version string
		var err error

		if ch.repoType == "oci" {
			version, err = c.checkOCI(ch.repoURL, ch.chartName)
		} else {
			version, err = c.checkHTTP(ch.repoURL, ch.chartName)
		}

		if err != nil {
			slog.Debug("version check failed", "repo", ch.repoURL, "chart", ch.chartName, "error", err)
			continue
		}

		results[key] = version

		// Rate limit: max 1 request/second
		time.Sleep(time.Second)
	}

	c.mu.Lock()
	for k, v := range results {
		c.latest[k] = v
	}
	c.mu.Unlock()
}

// GetLatest returns the latest known version for a repo+chart combination.
func (c *Checker) GetLatest(repoURL, chartName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest[repoURL+"/"+chartName]
}

// checkOCI queries an OCI registry for the latest tag of a chart.
// OCI repos have URLs like oci://host/path — we query /v2/<path>/<chart>/tags/list
func (c *Checker) checkOCI(repoURL, chartName string) (string, error) {
	// Strip oci:// prefix
	addr := strings.TrimPrefix(repoURL, "oci://")

	// Split into host and path
	parts := strings.SplitN(addr, "/", 2)
	host := parts[0]
	path := ""
	if len(parts) > 1 {
		path = parts[1]
	}

	// Build full image path
	imagePath := chartName
	if path != "" {
		imagePath = path + "/" + chartName
	}

	// Try HTTPS first, fall back to HTTP (for insecure registries)
	var resp *http.Response
	var err error

	for _, scheme := range []string{"https", "http"} {
		url := fmt.Sprintf("%s://%s/v2/%s/tags/list", scheme, host, imagePath)
		resp, err = c.client.Get(url)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("fetching tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var tagList struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &tagList); err != nil {
		return "", fmt.Errorf("parsing tags: %w", err)
	}

	return highestSemver(tagList.Tags), nil
}

// checkHTTP fetches a Helm HTTP repo's index.yaml and finds the latest chart version.
func (c *Checker) checkHTTP(repoURL, chartName string) (string, error) {
	url := strings.TrimRight(repoURL, "/") + "/index.yaml"

	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("index returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return "", fmt.Errorf("reading index: %w", err)
	}

	var index helmIndex
	if err := yaml.Unmarshal(body, &index); err != nil {
		return "", fmt.Errorf("parsing index: %w", err)
	}

	entries, ok := index.Entries[chartName]
	if !ok || len(entries) == 0 {
		return "", fmt.Errorf("chart %q not found in index", chartName)
	}

	var versions []string
	for _, e := range entries {
		if e.Version != "" {
			versions = append(versions, e.Version)
		}
	}

	return highestSemver(versions), nil
}

type helmIndex struct {
	Entries map[string][]helmEntry `yaml:"entries"`
}

type helmEntry struct {
	Version string `yaml:"version"`
}

// highestSemver returns the highest semantic version from a list of version strings.
func highestSemver(versions []string) string {
	var semvers []semver
	for _, v := range versions {
		sv, ok := parseSemver(v)
		if ok {
			semvers = append(semvers, sv)
		}
	}

	if len(semvers) == 0 {
		return ""
	}

	sort.Slice(semvers, func(i, j int) bool {
		return semvers[j].less(semvers[i]) // descending
	})

	return semvers[0].original
}

type semver struct {
	major, minor, patch int
	pre                 string
	original            string
}

func parseSemver(s string) (semver, bool) {
	v := semver{original: s}
	s = strings.TrimPrefix(s, "v")

	// Split off pre-release
	if idx := strings.IndexAny(s, "-+"); idx >= 0 {
		v.pre = s[idx:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semver{}, false
	}

	var err error
	v.major, err = strconv.Atoi(parts[0])
	if err != nil {
		return semver{}, false
	}
	v.minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return semver{}, false
	}
	if len(parts) == 3 {
		v.patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return semver{}, false
		}
	}

	return v, true
}

func (a semver) less(b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	if a.patch != b.patch {
		return a.patch < b.patch
	}
	// Pre-release versions have lower precedence than release
	if a.pre != "" && b.pre == "" {
		return true
	}
	if a.pre == "" && b.pre != "" {
		return false
	}
	return a.pre < b.pre
}
