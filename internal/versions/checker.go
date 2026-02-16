package versions

import (
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
	mu            sync.RWMutex
	latest        map[string]string // "repoURL/chartName" → latest version
	tokenCache    map[string]string // host → bearer token (for paginated requests)
	interval      time.Duration
	registryProxy string // e.g. "192.168.1.43:5000" — if set, OCI URLs through this host are resolved to upstream
	client        *http.Client
}

// NewChecker creates a version checker with the given check interval.
// registryProxy is the host:port of a local OCI proxy (e.g. Zot); empty disables proxy resolution.
func NewChecker(interval time.Duration, registryProxy string) *Checker {
	return &Checker{
		latest:        make(map[string]string),
		interval:      interval,
		registryProxy: registryProxy,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
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
			slog.Warn("version check failed", "repo", ch.repoURL, "chart", ch.chartName, "error", err)
			continue
		}

		if version != "" {
			results[key] = version
		}

		// Rate limit: max 1 request/second
		time.Sleep(time.Second)
	}

	c.mu.Lock()
	for k, v := range results {
		c.latest[k] = v
	}
	c.mu.Unlock()

	slog.Info("version check complete", "checked", len(checks), "resolved", len(results))
}

// GetLatest returns the latest known version for a repo+chart combination.
func (c *Checker) GetLatest(repoURL, chartName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest[repoURL+"/"+chartName]
}

// resolveUpstream converts a proxy OCI URL to the upstream registry.
// e.g. "oci://192.168.1.43:5000/ghcr.io/grafana/helm-charts" → ("ghcr.io", "grafana/helm-charts")
// If not a proxy URL, returns the host and path as-is.
func (c *Checker) resolveUpstream(repoURL string) (host, path string) {
	addr := strings.TrimPrefix(repoURL, "oci://")
	parts := strings.SplitN(addr, "/", 2)
	host = parts[0]
	if len(parts) > 1 {
		path = parts[1]
	}

	// If the host matches our proxy, the first path segment is the upstream registry
	if c.registryProxy != "" && host == c.registryProxy {
		pathParts := strings.SplitN(path, "/", 2)
		if len(pathParts) >= 1 && strings.Contains(pathParts[0], ".") {
			host = pathParts[0]
			path = ""
			if len(pathParts) > 1 {
				path = pathParts[1]
			}
		}
	}

	// docker.io → registry-1.docker.io
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}

	return host, path
}

// checkOCI queries an OCI registry for the latest tag of a chart.
// Follows pagination (Link headers) to collect all tags.
func (c *Checker) checkOCI(repoURL, chartName string) (string, error) {
	host, path := c.resolveUpstream(repoURL)

	imagePath := chartName
	if path != "" {
		imagePath = path + "/" + chartName
	}

	var allTags []string
	url := fmt.Sprintf("https://%s/v2/%s/tags/list?n=1000", host, imagePath)

	for url != "" {
		body, nextURL, err := c.fetchWithAuthPaginated(url)
		if err != nil {
			return "", err
		}

		var tagList struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(body, &tagList); err != nil {
			return "", fmt.Errorf("parsing tags: %w", err)
		}

		allTags = append(allTags, tagList.Tags...)
		url = nextURL
	}

	return highestStableSemver(allTags), nil
}

// fetchWithAuthPaginated performs an HTTP GET with OCI token auth, returning the body
// and the next page URL (from Link header) if any.
func (c *Checker) fetchWithAuthPaginated(url string) (body []byte, nextURL string, err error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	// If 401, try token auth
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("Www-Authenticate")
		if challenge == "" {
			return nil, "", fmt.Errorf("401 with no WWW-Authenticate header")
		}

		token, err := c.getToken(challenge)
		if err != nil {
			return nil, "", fmt.Errorf("getting auth token: %w", err)
		}

		// Cache token for subsequent paginated requests
		c.mu.Lock()
		if c.tokenCache == nil {
			c.tokenCache = make(map[string]string)
		}
		c.tokenCache[extractHost(url)] = token
		c.mu.Unlock()

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp2, err := c.client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("authenticated request: %w", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("registry returned %d after auth", resp2.StatusCode)
		}

		b, err := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
		return b, parseLinkNext(resp2.Header.Get("Link"), url), err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return b, parseLinkNext(resp.Header.Get("Link"), url), err
}

// extractHost returns the scheme+host portion of a URL for token cache keying.
func extractHost(rawURL string) string {
	if idx := strings.Index(rawURL, "//"); idx >= 0 {
		rest := rawURL[idx+2:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			return rawURL[:idx+2+slash]
		}
	}
	return rawURL
}

// parseLinkNext extracts the next page URL from an OCI Link header.
// Format: </v2/.../tags/list?n=100&last=xxx>; rel="next"
func parseLinkNext(link, currentURL string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.IndexByte(part, '<')
		end := strings.IndexByte(part, '>')
		if start < 0 || end < 0 || end <= start {
			continue
		}
		nextPath := part[start+1 : end]
		// If relative URL, make absolute using current URL's base
		if strings.HasPrefix(nextPath, "/") {
			base := extractHost(currentURL)
			return base + nextPath
		}
		return nextPath
	}
	return ""
}

// getToken parses a WWW-Authenticate Bearer challenge and fetches an anonymous token.
// Challenge format: Bearer realm="https://...",service="...",scope="..."
func (c *Checker) getToken(challenge string) (string, error) {
	challenge = strings.TrimPrefix(challenge, "Bearer ")

	params := parseAuthParams(challenge)
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("no realm in challenge: %s", challenge)
	}

	tokenURL := realm
	sep := "?"
	if service := params["service"]; service != "" {
		tokenURL += sep + "service=" + service
		sep = "&"
	}
	if scope := params["scope"]; scope != "" {
		tokenURL += sep + "scope=" + scope
	}

	resp, err := c.client.Get(tokenURL)
	if err != nil {
		return "", fmt.Errorf("fetching token from %s: %w", tokenURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	if tokenResp.Token != "" {
		return tokenResp.Token, nil
	}
	return tokenResp.AccessToken, nil
}

// parseAuthParams parses key="value" pairs from a WWW-Authenticate header value.
func parseAuthParams(s string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(part[:eq])
		val := strings.TrimSpace(part[eq+1:])
		val = strings.Trim(val, "\"")
		params[key] = val
	}
	return params
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

	return highestStableSemver(versions), nil
}

type helmIndex struct {
	Entries map[string][]helmEntry `yaml:"entries"`
}

type helmEntry struct {
	Version string `yaml:"version"`
}

// highestStableSemver returns the highest stable (non-pre-release) semantic version.
func highestStableSemver(versions []string) string {
	var semvers []semver
	for _, v := range versions {
		sv, ok := parseSemver(v)
		if !ok || sv.pre != "" {
			continue
		}
		semvers = append(semvers, sv)
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
