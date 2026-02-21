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

// knownDistros maps OS distro names (lowercased) to their GitHub repo for release checking.
var knownDistros = map[string]string{
	"talos": "siderolabs/talos",
	"k3s":   "k3s-io/k3s",
}

// osImageRe extracts the distro name and version from an OS image string.
// Examples: "Talos (v1.9.0)" → ("talos", "v1.9.0"), "Ubuntu 22.04" → ("ubuntu", "22.04")
var osImageRe = regexp.MustCompile(`(?i)^(\S+)\s*\(?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)\)?`)

// NodeChecker checks for latest OS and kubelet versions for cluster nodes.
type NodeChecker struct {
	mu        sync.RWMutex
	latestOS  map[string]string // "distro" → latest version
	latestK8s map[string]string // "major.minor" → latest patch version
	lastCheck time.Time
	checking  atomic.Bool
	client    *http.Client
}

// NewNodeChecker creates a new NodeChecker.
func NewNodeChecker() *NodeChecker {
	return &NodeChecker{
		latestOS:  make(map[string]string),
		latestK8s: make(map[string]string),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ParseOSImage extracts the distro name and version from an OSImage string.
func ParseOSImage(osImage string) (distro, version string) {
	m := osImageRe.FindStringSubmatch(osImage)
	if m == nil {
		return "", ""
	}
	return strings.ToLower(m[1]), m[2]
}

// Check fetches latest OS and kubelet versions for the given nodes.
// Single-flight: returns immediately if already checking.
// Interval gate: skips if last check was less than 15 minutes ago.
func (nc *NodeChecker) Check(nodes []model.NodeInfo) {
	if !nc.checking.CompareAndSwap(false, true) {
		return
	}
	defer nc.checking.Store(false)

	nc.mu.RLock()
	tooSoon := time.Since(nc.lastCheck) < 15*time.Minute
	nc.mu.RUnlock()
	if tooSoon {
		return
	}

	// Collect unique distros and kubelet minor versions
	distros := make(map[string]bool)
	minorVersions := make(map[string]bool)

	for _, n := range nodes {
		distro, _ := ParseOSImage(n.OSImage)
		if distro != "" {
			distros[distro] = true
		}
		if n.KubeletVersion != "" {
			minor := kubeletMinor(n.KubeletVersion)
			if minor != "" {
				minorVersions[minor] = true
			}
		}
	}

	// Check OS distro versions
	for distro := range distros {
		repo, ok := knownDistros[distro]
		if !ok {
			continue
		}
		latest, err := nc.fetchLatestGitHubRelease(repo)
		if err != nil {
			slog.Warn("node version check: failed to get latest OS release", "distro", distro, "error", err)
			continue
		}
		nc.mu.Lock()
		nc.latestOS[distro] = latest
		nc.mu.Unlock()
		time.Sleep(time.Second)
	}

	// Check kubelet versions (latest patch in each minor series)
	for minor := range minorVersions {
		latest, err := nc.fetchLatestK8sPatch(minor)
		if err != nil {
			slog.Warn("node version check: failed to get latest k8s patch", "minor", minor, "error", err)
			continue
		}
		nc.mu.Lock()
		nc.latestK8s[minor] = latest
		nc.mu.Unlock()
		time.Sleep(time.Second)
	}

	nc.mu.Lock()
	nc.lastCheck = time.Now()
	nc.mu.Unlock()

	slog.Info("node version check complete", "distros", len(distros), "k8sMinors", len(minorVersions))
}

// GetLatestOS returns the latest known version for a given OS distro.
func (nc *NodeChecker) GetLatestOS(osImage string) string {
	distro, _ := ParseOSImage(osImage)
	if distro == "" {
		return ""
	}
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.latestOS[distro]
}

// GetLatestKubelet returns the latest known patch version for a given kubelet version's minor series.
func (nc *NodeChecker) GetLatestKubelet(kubeletVersion string) string {
	minor := kubeletMinor(kubeletVersion)
	if minor == "" {
		return ""
	}
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.latestK8s[minor]
}

// kubeletMinor extracts the major.minor from a kubelet version string.
// e.g. "v1.32.0" → "1.32"
func kubeletMinor(version string) string {
	v := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}

// fetchLatestGitHubRelease fetches the latest release tag from a GitHub repo.
func (nc *NodeChecker) fetchLatestGitHubRelease(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := nc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d for %s", resp.StatusCode, repo)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("parsing release: %w", err)
	}

	return release.TagName, nil
}

// fetchLatestK8sPatch fetches the latest patch release for a given Kubernetes minor version.
func (nc *NodeChecker) fetchLatestK8sPatch(minor string) (string, error) {
	url := "https://api.github.com/repos/kubernetes/kubernetes/releases?per_page=100"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := nc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching k8s releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parsing releases: %w", err)
	}

	prefix := "v" + minor + "."
	var best semver
	var bestTag string

	for _, r := range releases {
		if r.Prerelease || r.Draft {
			continue
		}
		if !strings.HasPrefix(r.TagName, prefix) {
			continue
		}
		sv, ok := parseSemver(r.TagName)
		if !ok || sv.pre != "" {
			continue
		}
		if bestTag == "" || best.less(sv) {
			best = sv
			bestTag = r.TagName
		}
	}

	if bestTag == "" {
		return "", fmt.Errorf("no stable release found for v%s.x", minor)
	}
	return bestTag, nil
}
