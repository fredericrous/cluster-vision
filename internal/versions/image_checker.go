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
	"strconv"
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
	digests   map[string]string // "image|tag" → what the registry serves for the tag NOW ("sha256:…"), "" if unknown
	lastCheck time.Time
	checking  atomic.Bool
	client    *http.Client
	insecure  *http.Client // for HTTP-only registries
}

// NewImageChecker creates a new ImageChecker.
func NewImageChecker() *ImageChecker {
	return &ImageChecker{
		latest:  make(map[string]string),
		digests: make(map[string]string),
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
// The suffix keeps its letters and punctuation but not its numbers, which
// are carried separately: "1.2.3-debian-12-r4" and "1.2.4-debian-12-r0" are
// the same variant ("-debian-#-r#"), as are "16.4-alpine3.20" and
// "16.4-alpine3.21" ("-alpine#.#").
type variant struct {
	prefix     string
	suffix     string // numbers replaced by "#"
	suffixNums []int  // the numbers, in order, to rank equal semvers
}

var (
	semverInTagRe = regexp.MustCompile(`^(.*?)(\d+\.\d+(?:\.\d+)?)(.*?)$`)
	digitsRe      = regexp.MustCompile(`\d+`)
)

// extractVariant splits a tag into its variant pattern and semver portion.
// Returns the variant and the parsed semver. ok=false if the tag has no semver.
func extractVariant(tag string) (v variant, sv semver, ok bool) {
	m := semverInTagRe.FindStringSubmatch(tag)
	if m == nil {
		return variant{}, semver{}, false
	}
	v = variant{prefix: m[1], suffix: digitsRe.ReplaceAllString(m[3], "#")}
	for _, d := range digitsRe.FindAllString(m[3], -1) {
		n, err := strconv.Atoi(d)
		if err != nil {
			return variant{}, semver{}, false // absurdly long digit run
		}
		v.suffixNums = append(v.suffixNums, n)
	}
	sv, ok = parseSemver(m[2])
	return v, sv, ok
}

// variantKey returns a string key that identifies a variant pattern.
func (v variant) key() string {
	return v.prefix + "|" + v.suffix
}

// newerTag orders two tags of one variant: by semver, then by the numbers
// in the suffix (a rebuild "-r5" after "-r4", a base image "alpine3.21"
// after "alpine3.20").
func newerTag(sv semver, v variant, thanSV semver, than variant) bool {
	if c := sv.compare(thanSV); c != 0 {
		return c > 0
	}
	for i := 0; i < len(v.suffixNums) && i < len(than.suffixNums); i++ {
		if v.suffixNums[i] != than.suffixNums[i] {
			return v.suffixNums[i] > than.suffixNums[i]
		}
	}
	return false
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

		// For each deployed tag, find the highest matching tag with the same
		// variant, and what the registry serves for the deployed tag RIGHT NOW.
		// The latter feeds the pin findings (unpinned / tag moved / drifted):
		// a manifest pinned to digest X while the registry now serves Y for
		// the same tag is an upstream re-tag the cluster has not seen; an
		// unpinned pod running Y' while the registry serves Y is a mutable
		// tag that moved underneath the running workload.
		results := make(map[string]string)
		digests := make(map[string]string)
		for tag := range ri.tags {
			if tag == "" {
				// Bare-digest reference: no tag to compare against.
				results[tag] = "-"
				continue
			}
			results[tag] = highestMatchingTag(tag, allTags)
			d, derr := ic.headDigest(ri.registry, ri.path, tag)
			if derr != nil {
				slog.Debug("image check: digest lookup failed", "image", image, "tag", tag, "error", derr)
				continue
			}
			digests[tag] = d
		}

		// Write results incrementally so partial data is visible.
		ic.mu.Lock()
		for tag, latest := range results {
			ic.latest[image+"|"+tag] = latest
		}
		for tag, d := range digests {
			ic.digests[image+"|"+tag] = d
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

// GetDigest returns what the registry served for image:tag at the last
// check ("sha256:…"), and false when it is unknown (never checked, registry
// skipped or the manifest request failed).
func (ic *ImageChecker) GetDigest(image, tag string) (string, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	d, ok := ic.digests[image+"|"+tag]
	return d, ok && d != ""
}

// headDigest resolves what the registry serves for image:tag with the
// multi-arch-capable Accept set — the image INDEX for a multi-arch publish,
// the single manifest otherwise. That is exactly what containerd (and a
// pull-through mirror with preserveDigest) resolves the tag to, so it is
// the value a `tag@digest` pin in the manifests must equal.
func (ic *ImageChecker) headDigest(registry, imagePath, tag string) (string, error) {
	host := registry
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}
	reqURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, imagePath, tag)
	resp, err := ic.doWithAuth(http.MethodHead, reqURL, host, manifestAccept)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %d", resp.StatusCode)
	}
	d := strings.TrimSpace(resp.Header.Get("Docker-Content-Digest"))
	if !strings.HasPrefix(d, "sha256:") || len(d) != len("sha256:")+64 {
		return "", fmt.Errorf("no usable Docker-Content-Digest header (%q)", d)
	}
	return d, nil
}

// manifestAccept lists every manifest media type in preference order so the
// registry answers with the index for multi-arch images.
const manifestAccept = "application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json"

// doWithAuth issues one request, handling the 401 Bearer challenge the same
// way fetchWithAuth does but for an arbitrary method — a HEAD carries no body
// worth reading, only headers. The caller owns resp.Body.
func (ic *ImageChecker) doWithAuth(method, reqURL, registryHost, accept string) (*http.Response, error) {
	build := func(u, token string) (*http.Request, error) {
		req, err := http.NewRequest(method, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", accept)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return req, nil
	}
	req, err := build(reqURL, "")
	if err != nil {
		return nil, err
	}
	resp, err := ic.client.Do(req)
	if err != nil {
		if !strings.Contains(registryHost, ":") {
			return nil, fmt.Errorf("%s %s: %w", method, reqURL, err)
		}
		// HTTPS failed — try HTTP for registries with a port (likely internal).
		req, err = build(strings.Replace(reqURL, "https://", "http://", 1), "")
		if err != nil {
			return nil, err
		}
		if resp, err = ic.insecure.Do(req); err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, reqURL, err)
		}
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	challenge := resp.Header.Get("Www-Authenticate")
	_ = resp.Body.Close()
	if challenge == "" {
		return nil, fmt.Errorf("401 with no WWW-Authenticate header")
	}
	token, err := ic.getToken(challenge)
	if err != nil {
		return nil, fmt.Errorf("getting auth token: %w", err)
	}
	req, err = build(reqURL, token)
	if err != nil {
		return nil, err
	}
	resp, err = ic.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authenticated request: %w", err)
	}
	return resp, nil
}

// GetLatest returns the cached latest tag for a given image+tag combination.
func (ic *ImageChecker) GetLatest(image, tag string) string {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.latest[image+"|"+tag]
}

// highestMatchingTag finds the tag with the highest semver that matches
// the same variant pattern (prefix + suffix) as the deployed tag.
//
// Pre-releases need no filter of their own: "1.2.4-rc.1" is a different
// variant ("-rc.#") from a deployed "1.2.3", so it never matches one.
func highestMatchingTag(deployedTag string, allTags []string) string {
	deployedVariant, deployedSV, ok := extractVariant(deployedTag)
	if !ok {
		return "-"
	}

	bestTag := deployedTag
	bestSV := deployedSV
	bestVariant := deployedVariant

	for _, t := range allTags {
		v, sv, ok := extractVariant(t)
		if !ok {
			continue
		}
		if v.key() != deployedVariant.key() {
			continue
		}
		if newerTag(sv, v, bestSV, bestVariant) {
			bestSV, bestVariant, bestTag = sv, v, t
		}
	}

	return bestTag
}

// maxTagPages bounds how many Link-header pages a tag listing follows. At
// n=1000 per page that is far past any real repository; a registry that
// keeps answering with a next link (or links back to itself) stops here.
const maxTagPages = 50

// listTags fetches the tag list for an image from an OCI registry.
func (ic *ImageChecker) listTags(registry, imagePath string) ([]string, error) {
	host := registry
	// docker.io → registry-1.docker.io
	if host == "docker.io" {
		host = "registry-1.docker.io"
	}

	var allTags []string
	tagURL := fmt.Sprintf("https://%s/v2/%s/tags/list?n=1000", host, imagePath)

	for page := 0; tagURL != ""; page++ {
		if page == maxTagPages {
			slog.Warn("image check: tag list truncated", "image", host+"/"+imagePath, "pages", maxTagPages)
			break
		}
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
	defer func() { _ = resp.Body.Close() }()

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
		defer func() { _ = resp2.Body.Close() }()

		if resp2.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("registry returned %d after auth", resp2.StatusCode)
		}

		b, readErr := readCapped(resp2.Body, maxRegistryResponseBytes)
		return b, parseLinkNext(resp2.Header.Get("Link"), reqURL), readErr
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	b, err := readCapped(resp.Body, maxRegistryResponseBytes)
	return b, parseLinkNext(resp.Header.Get("Link"), reqURL), err
}

// maxRegistryResponseBytes bounds one registry response. Tag lists are not
// small: registry.k8s.io redirects to Artifact Registry, which ignores n=
// and returns every tag in one page together with a "manifest" map of every
// digest — 1.37 MB for kube-apiserver in 2026-09. The old 1 MiB cap cut
// that JSON mid-object ("unexpected end of JSON input").
const maxRegistryResponseBytes = 32 << 20

// readCapped reads r to the end, failing — rather than silently truncating,
// as io.LimitReader does — when it holds more than limit bytes.
func readCapped(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("registry response exceeds %d bytes", limit)
	}
	return b, nil
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
	defer func() { _ = resp.Body.Close() }()

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

// parseImageRef splits a container image reference into registry, repo, and
// tag. A digest-pinned reference (`repo:tag@sha256:…`) yields its TAG —
// that is what the registry's tag list is compared against; the digest is
// returned separately by model.SplitImageRef. A bare digest yields "".
func parseImageRef(ref string) (registry, repo, tag string) {
	registry, repo, tag, _ = model.SplitImageRef(ref)
	return registry, repo, tag
}
