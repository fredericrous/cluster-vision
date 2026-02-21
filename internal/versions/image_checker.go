package versions

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// ImageChecker periodically checks container image registries for latest tags.
type ImageChecker struct {
	mu         sync.RWMutex
	latest     map[string]string // "image|tag" → latest tag
	tokenCache map[string]string // registry host → bearer token
	lastCheck  time.Time
	checking   atomic.Bool
	client     *http.Client
}

// NewImageChecker creates a new ImageChecker.
func NewImageChecker() *ImageChecker {
	return &ImageChecker{
		latest:     make(map[string]string),
		tokenCache: make(map[string]string),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// variant represents a tag's decomposed structure: prefix + semver + suffix.
type variant struct {
	prefix string
	suffix string
}

var semverInTagRe = regexp.MustCompile(`^(.*?)(\d+\.\d+(?:\.\d+)?)(.*?)$`)

// extractVariant splits a tag into its variant pattern and semver portion.
// Returns the variant and the parsed semver. ok=false if the tag has no semver.
func extractVariant(tag string) (v variant, sv semver, ok bool) {
	m := semverInTagRe.FindStringSubmatch(tag)
	if m == nil {
		return variant{}, semver{}, false
	}
	v = variant{prefix: m[1], suffix: m[3]}
	sv, ok = parseSemver(m[2])
	return v, sv, ok
}

// variantKey returns a string key that identifies a variant pattern.
func (v variant) key() string {
	return v.prefix + "|" + v.suffix
}

// Check fetches latest tags for all unique image repos used by pods.
// Single-flight: returns immediately if already checking.
// Interval gate: skips if last check was less than 15 minutes ago.
func (ic *ImageChecker) Check(pods []model.PodImageInfo) {
	if !ic.checking.CompareAndSwap(false, true) {
		return
	}
	defer ic.checking.Store(false)

	ic.mu.RLock()
	tooSoon := time.Since(ic.lastCheck) < 15*time.Minute
	ic.mu.RUnlock()
	if tooSoon {
		return
	}

	// Dedup: group deployed tags by image repo (registry/path).
	type repoInfo struct {
		registry string
		path     string
		tags     map[string]bool // all deployed tags for this repo
	}
	repos := make(map[string]*repoInfo) // key = "registry/path"

	for _, p := range pods {
		registry, repo, tag := parseImageRef(p.Image)
		image := registry + "/" + repo
		ri, ok := repos[image]
		if !ok {
			ri = &repoInfo{
				registry: registry,
				path:     repo,
				tags:     make(map[string]bool),
			}
			repos[image] = ri
		}
		ri.tags[tag] = true
	}

	results := make(map[string]string)
	skipRegistries := make(map[string]bool) // registries that returned 429

	for image, ri := range repos {
		if skipRegistries[ri.registry] {
			for tag := range ri.tags {
				results[image+"|"+tag] = "-"
			}
			continue
		}

		allTags, err := ic.listTags(ri.registry, ri.path)
		if err != nil {
			if strings.Contains(err.Error(), "429") {
				slog.Warn("image check: rate limited, skipping registry", "registry", ri.registry)
				skipRegistries[ri.registry] = true
			} else {
				slog.Warn("image check: failed to list tags", "image", image, "error", err)
			}
			for tag := range ri.tags {
				results[image+"|"+tag] = "-"
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// For each deployed tag, find the highest matching tag with the same variant.
		for tag := range ri.tags {
			latest := highestMatchingTag(tag, allTags)
			results[image+"|"+latest] = latest // self-reference is fine
			results[image+"|"+tag] = latest
		}

		time.Sleep(2 * time.Second)
	}

	ic.mu.Lock()
	for k, v := range results {
		ic.latest[k] = v
	}
	ic.lastCheck = time.Now()
	ic.mu.Unlock()

	slog.Info("image check complete", "repos", len(repos), "results", len(results))
}

// GetLatest returns the cached latest tag for a given image+tag combination.
func (ic *ImageChecker) GetLatest(image, tag string) string {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.latest[image+"|"+tag]
}

// highestMatchingTag finds the tag with the highest semver that matches
// the same variant pattern (prefix + suffix) as the deployed tag.
func highestMatchingTag(deployedTag string, allTags []string) string {
	deployedVariant, deployedSV, ok := extractVariant(deployedTag)
	if !ok {
		return "-"
	}

	bestTag := deployedTag
	bestSV := deployedSV

	for _, t := range allTags {
		v, sv, ok := extractVariant(t)
		if !ok {
			continue
		}
		if v.key() != deployedVariant.key() {
			continue
		}
		// Skip pre-release versions
		if sv.pre != "" {
			continue
		}
		if bestSV.less(sv) {
			bestSV = sv
			bestTag = t
		}
	}

	return bestTag
}

// listTags fetches the tag list for an image from an OCI registry.
// Uses proactive auth from token cache and follows pagination.
func (ic *ImageChecker) listTags(registry, imagePath string) ([]string, error) {
	host := registry
	// docker.io → registry-1.docker.io
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}

	var allTags []string
	url := fmt.Sprintf("https://%s/v2/%s/tags/list?n=1000", host, imagePath)

	for url != "" {
		body, nextURL, err := ic.fetchWithAuth(url)
		if err != nil {
			return nil, err
		}

		var tagList struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(body, &tagList); err != nil {
			return nil, fmt.Errorf("parsing tags: %w", err)
		}

		allTags = append(allTags, tagList.Tags...)
		url = nextURL
	}

	return allTags, nil
}

// fetchWithAuth performs an HTTP GET with proactive bearer auth from cache,
// falling back to 401 challenge auth. Returns body and next page URL.
func (ic *ImageChecker) fetchWithAuth(url string) (body []byte, nextURL string, err error) {
	host := extractHost(url)

	// Try with cached token first
	ic.mu.RLock()
	token := ic.tokenCache[host]
	ic.mu.RUnlock()

	if token != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := ic.client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("fetching %s: %w", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			return b, parseLinkNext(resp.Header.Get("Link"), url), err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, "", fmt.Errorf("429 rate limited")
		}

		// Token expired or invalid — fall through to unauthenticated request
	}

	// Unauthenticated request (or cached token failed)
	resp, err := ic.client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, "", fmt.Errorf("429 rate limited")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("Www-Authenticate")
		if challenge == "" {
			return nil, "", fmt.Errorf("401 with no WWW-Authenticate header")
		}

		newToken, err := ic.getToken(challenge)
		if err != nil {
			return nil, "", fmt.Errorf("getting auth token: %w", err)
		}

		ic.mu.Lock()
		ic.tokenCache[host] = newToken
		ic.mu.Unlock()

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+newToken)

		resp2, err := ic.client.Do(req)
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

// getToken parses a WWW-Authenticate Bearer challenge and fetches an anonymous token.
func (ic *ImageChecker) getToken(challenge string) (string, error) {
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

	resp, err := ic.client.Get(tokenURL)
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

// parseImageRef splits a container image reference into registry, repo, and tag.
// Duplicated from diagram package to avoid circular imports.
func parseImageRef(ref string) (registry, repo, tag string) {
	if idx := strings.Index(ref, "@"); idx != -1 {
		tag = ref[idx+1:]
		ref = ref[:idx]
	}

	if tag == "" {
		if idx := strings.LastIndex(ref, ":"); idx != -1 {
			slashIdx := strings.LastIndex(ref, "/")
			if idx > slashIdx {
				tag = ref[idx+1:]
				ref = ref[:idx]
			}
		}
		if tag == "" {
			tag = "latest"
		}
	}

	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		return "docker.io", "library/" + parts[0], tag
	}

	first := parts[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first, parts[1], tag
	}

	return "docker.io", ref, tag
}
