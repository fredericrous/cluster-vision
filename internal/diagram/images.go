package diagram

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/versions"
)

// ImageRow represents a single row in the container images table.
type ImageRow struct {
	Image        string `json:"image"`        // registry/repo (without tag)
	Tag          string `json:"tag"`          // tag or digest
	Type         string `json:"type"`         // "app" | "init"
	Namespaces   string `json:"namespaces"`   // comma-separated unique namespaces
	Pods         int    `json:"pods"`         // count of pods using this image:tag
	Registry     string `json:"registry"`     // extracted registry hostname
	Latest       string `json:"latest"`       // latest tag with same variant pattern
	Outdated     bool   `json:"outdated"`     // true if latest != current tag
	SecurityRisk string `json:"securityRisk"` // "critical" | "warning" | "none" | ""
	VulnSummary  string `json:"vulnSummary"`  // human-readable tooltip
	// Exploit-risk badge driven by CISA KEV + FIRST EPSS. Empty for
	// images with no Trivy report. See vulnExploitRisk() for tier rules.
	ExploitRisk    string `json:"exploitRisk"`    // "kev" | "high-epss" | "low-epss" | "none" | ""
	ExploitSummary string `json:"exploitSummary"` // e.g. "1 KEV (CVE-2024-12345)" or "EPSS 0.87 (CVE-…)"
	KEVCVEs        string `json:"kevCVEs"`        // comma-separated for tooltip
	// Pin state. Pinned: the reference carries @sha256 (exact bytes).
	// TagMoved: pinned, but the registry now serves another digest for the
	// tag (upstream re-tag; the pin is stale). RegistryDigest: what the
	// registry serves for the tag now, "" if unknown.
	Pinned         bool   `json:"pinned"`
	Digest         string `json:"digest"`         // pinned digest, "" when pull-by-tag
	RegistryDigest string `json:"registryDigest"` // registry's current digest for the tag
	TagMoved       bool   `json:"tagMoved"`
}

// imageKey uniquely identifies an image ref + container type. The digest
// is part of it: a pod pinned to tag@sha256 and one pulling the bare tag
// are different rows, or the row's pin status would depend on pod order.
type imageKey struct {
	image         string // registry/repo (no tag)
	tag           string
	digest        string // @sha256 the reference pins, "" when pull-by-tag
	initContainer bool
}

type imageAgg struct {
	namespaces map[string]bool
	pods       map[string]bool // namespace/podName for dedup
	registry   string
}

// GenerateImages produces a table of container images running across the cluster.
func GenerateImages(data *model.ClusterData, checker *versions.ImageChecker) model.DiagramResult {
	if len(data.Pods) == 0 {
		return model.DiagramResult{
			ID:      "images",
			Title:   "Container Images",
			Type:    "markdown",
			Content: "*No pod data available.*",
		}
	}

	vulns := model.NewVulnIndex(data.ImageVulns)

	agg := make(map[imageKey]*imageAgg)

	for _, p := range data.Pods {
		registry, repo, tag := parseImageRef(p.Image)
		_, _, _, digest := model.SplitImageRef(p.Image)
		image := registry + "/" + repo

		key := imageKey{image: image, tag: tag, digest: digest, initContainer: p.InitContainer}

		a, ok := agg[key]
		if !ok {
			a = &imageAgg{
				namespaces: make(map[string]bool),
				pods:       make(map[string]bool),
				registry:   registry,
			}
			agg[key] = a
		}
		a.namespaces[p.Namespace] = true
		a.pods[p.Namespace+"/"+p.PodName] = true
	}

	var rows []ImageRow
	for key, a := range agg {
		ns := sortedKeys(a.namespaces)

		typ := "app"
		if key.initContainer {
			typ = "init"
		}

		latest := "-"
		outdated := false
		registryDigest := ""
		if checker != nil {
			if v := checker.GetLatest(key.image, key.tag); v != "" {
				latest = v
				if latest != "-" && latest != key.tag {
					outdated = true
				}
			}
			if d, ok := checker.GetDigest(key.image, key.tag); ok {
				registryDigest = d
			}
		}
		pinned := key.digest != ""
		tagMoved := pinned && registryDigest != "" && registryDigest != key.digest && key.tag != key.digest

		// Security risk from trivy VulnerabilityReports
		secRisk := ""
		vulnSum := ""
		exploitRisk := ""
		exploitSum := ""
		kevList := ""
		// Rows span clusters, so the worst report any cluster has.
		imageRef := key.image + ":" + key.tag
		if strings.HasPrefix(key.tag, "sha256:") {
			imageRef = key.image + "@" + key.tag
		} else if key.digest != "" {
			imageRef += "@" + key.digest
		}
		if v, ok := vulns.Lookup("", imageRef); ok {
			secRisk, vulnSum = vulnRisk(v)
			exploitRisk, exploitSum = vulnExploitRisk(v)
			kevList = strings.Join(v.KEVCVEs, ",")
		}

		rows = append(rows, ImageRow{
			Image:          key.image,
			Tag:            key.tag,
			Type:           typ,
			Namespaces:     strings.Join(ns, ", "),
			Pods:           len(a.pods),
			Registry:       a.registry,
			Latest:         latest,
			Outdated:       outdated,
			SecurityRisk:   secRisk,
			VulnSummary:    vulnSum,
			ExploitRisk:    exploitRisk,
			ExploitSummary: exploitSum,
			KEVCVEs:        kevList,
			Pinned:         pinned,
			Digest:         key.digest,
			RegistryDigest: registryDigest,
			TagMoved:       tagMoved,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Registry != rows[j].Registry {
			return rows[i].Registry < rows[j].Registry
		}
		if rows[i].Image != rows[j].Image {
			return rows[i].Image < rows[j].Image
		}
		if rows[i].Tag != rows[j].Tag {
			return rows[i].Tag < rows[j].Tag
		}
		if rows[i].Digest != rows[j].Digest {
			return rows[i].Digest < rows[j].Digest
		}
		return rows[i].Type < rows[j].Type
	})

	tableJSON, _ := json.Marshal(rows)

	return model.DiagramResult{
		ID:      "images",
		Title:   "Container Images",
		Type:    "table",
		Content: string(tableJSON),
	}
}

// parseImageRef splits a container image reference into registry, repo, and
// tag. For a bare digest reference the "tag" column shows the digest itself
// (the page's Tag cell already renders sha256 values shortened); a pinned
// `repo:tag@sha256:…` shows its tag, with the digest carried on ImageRow.
func parseImageRef(ref string) (registry, repo, tag string) {
	registry, repo, tag, digest := model.SplitImageRef(ref)
	if tag == "" {
		tag = digest
	}
	return registry, repo, tag
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
