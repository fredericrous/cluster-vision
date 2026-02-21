package versions

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// ImageChecker periodically checks container image registries for latest tags.
type ImageChecker struct {
	mu        sync.RWMutex
	latest    map[string]string // "image|tag" → latest tag
	lastCheck time.Time
	checking  atomic.Bool
	client    *http.Client
	insecure  *http.Client // for HTTP-only registries
}

// NewImageChecker creates a new ImageChecker.
func NewImageChecker() *ImageChecker {
	return &ImageChecker{
		latest: make(map[string]string),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		insecure: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
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

// skipRegistry returns true for registries we can't reach from inside the cluster
// or that don't support the Docker v2 API.
func skipRegistry(registry string) bool {
	return strings.Contains(registry, ".svc.cluster.local") ||
		strings.HasSuffix(registry, ".local") ||
		strings.HasPrefix(registry, "localhost")
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

	skipRegistries := make(map[string]bool) // registries that returned 429
	checked := 0
	resolved := 0

	for image, ri := range repos {
		if skipRegistry(ri.registry) {
			ic.setResults(image, ri.tags, "-")
			checked++
			continue
		}

		if skipRegistries[ri.registry] {
			ic.setResults(image, ri.tags, "-")
			checked++
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
			ic.setResults(image, ri.tags, "-")
			checked++
			time.Sleep(2 * time.Second)
			continue
		}

		// For each deployed tag, find the highest matching tag with the same variant.
		results := make(map[string]string)
		for tag := range ri.tags {
			latest := highestMatchingTag(tag, allTags)
			results[tag] = latest
		}

		// Write results incrementally so partial data is visible.
		ic.mu.Lock()
		for tag, latest := range results {
			ic.latest[image+"|"+tag] = latest
		}
		ic.mu.Unlock()

		checked++
		resolved++
		time.Sleep(2 * time.Second)
	}

	ic.mu.Lock()
	ic.lastCheck = time.Now()
	ic.mu.Unlock()

	slog.Info("image check complete", "repos", checked, "resolved", resolved)
}

// setResults writes "-" for all tags of an image (used for errors/skips).
func (ic *ImageChecker) setResults(image string, tags map[string]bool, value string) {
	ic.mu.Lock()
	for tag := range tags {
		ic.latest[image+"|"+tag] = value
	}
	ic.mu.Unlock()
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
func (ic *ImageChecker) listTags(registry, imagePath string) ([]string, error) {
	host := registry
	// docker.io → registry-1.docker.io
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}

	var allTags []string
	tagURL := fmt.Sprintf("https://%s/v2/%s/tags/list?n=1000", host, imagePath)

	for tagURL != "" {
		body, nextURL, err := ic.fetchWithAuth(tagURL, host)
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
		tagURL = nextURL
	}

	return allTags, nil
}

// fetchWithAuth performs an HTTP GET, handling 401 Bearer challenge auth.
// Each request gets a fresh token scoped to the correct repository.
func (ic *ImageChecker) fetchWithAuth(reqURL, registryHost string) (body []byte, nextURL string, err error) {
	resp, err := ic.client.Get(reqURL)
	if err != nil {
		// HTTPS failed — try HTTP for registries with port (likely internal)
		if strings.Contains(registryHost, ":") {
			httpURL := strings.Replace(reqURL, "https://", "http://", 1)
			resp, err = ic.insecure.Get(httpURL)
			if err != nil {
				return nil, "", fmt.Errorf("fetching %s: %w", reqURL, err)
			}
		} else {
			return nil, "", fmt.Errorf("fetching %s: %w", reqURL, err)
		}
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

		token, tokenErr := ic.getToken(challenge)
		if tokenErr != nil {
			return nil, "", fmt.Errorf("getting auth token: %w", tokenErr)
		}

		req, reqErr := http.NewRequest("GET", reqURL, nil)
		if reqErr != nil {
			return nil, "", reqErr
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp2, doErr := ic.client.Do(req)
		if doErr != nil {
			return nil, "", fmt.Errorf("authenticated request: %w", doErr)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("registry returned %d after auth", resp2.StatusCode)
		}

		b, readErr := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
		return b, parseLinkNext(resp2.Header.Get("Link"), reqURL), readErr
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return b, parseLinkNext(resp.Header.Get("Link"), reqURL), err
}

// getToken parses a WWW-Authenticate Bearer challenge and fetches an anonymous token.
func (ic *ImageChecker) getToken(challenge string) (string, error) {
	challenge = strings.TrimPrefix(challenge, "Bearer ")

	params := parseAuthParams(challenge)
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("no realm in challenge: %s", challenge)
	}

	// Build token URL with properly encoded query parameters.
	u, err := url.Parse(realm)
	if err != nil {
		return "", fmt.Errorf("invalid realm URL %q: %w", realm, err)
	}
	q := u.Query()
	if service := params["service"]; service != "" {
		q.Set("service", service)
	}
	if scope := params["scope"]; scope != "" {
		q.Set("scope", scope)
	}
	u.RawQuery = q.Encode()

	resp, err := ic.client.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("fetching token from %s: %w", u.String(), err)
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
