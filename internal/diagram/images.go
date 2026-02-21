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
	Image      string `json:"image"`      // registry/repo (without tag)
	Tag        string `json:"tag"`        // tag or digest
	Type       string `json:"type"`       // "app" | "init"
	Namespaces string `json:"namespaces"` // comma-separated unique namespaces
	Pods       int    `json:"pods"`       // count of pods using this image:tag
	Registry   string `json:"registry"`   // extracted registry hostname
	Latest     string `json:"latest"`     // latest tag with same variant pattern
	Outdated   bool   `json:"outdated"`   // true if latest != current tag
}

// imageKey uniquely identifies an image ref + container type.
type imageKey struct {
	image    string // registry/repo (no tag)
	tag      string
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

	agg := make(map[imageKey]*imageAgg)

	for _, p := range data.Pods {
		registry, repo, tag := parseImageRef(p.Image)
		image := registry + "/" + repo

		key := imageKey{image: image, tag: tag, initContainer: p.InitContainer}

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
		if checker != nil {
			if v := checker.GetLatest(key.image, key.tag); v != "" {
				latest = v
				if latest != "-" && latest != key.tag {
					outdated = true
				}
			}
		}

		rows = append(rows, ImageRow{
			Image:      key.image,
			Tag:        key.tag,
			Type:       typ,
			Namespaces: strings.Join(ns, ", "),
			Pods:       len(a.pods),
			Registry:   a.registry,
			Latest:     latest,
			Outdated:   outdated,
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

// parseImageRef splits a container image reference into registry, repo, and tag.
// Examples:
//
//	"ghcr.io/foo/bar:v1.2" → "ghcr.io", "foo/bar", "v1.2"
//	"nginx:latest"         → "docker.io", "library/nginx", "latest"
//	"nginx"                → "docker.io", "library/nginx", "latest"
//	"myregistry:5000/app:v1" → "myregistry:5000", "app", "v1"
func parseImageRef(ref string) (registry, repo, tag string) {
	// Handle @sha256: digest references
	if idx := strings.Index(ref, "@"); idx != -1 {
		tag = ref[idx+1:]
		ref = ref[:idx]
	}

	// Split off tag
	if tag == "" {
		if idx := strings.LastIndex(ref, ":"); idx != -1 {
			// Make sure the colon is after the last slash (not a port in registry)
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

	// Determine registry vs repo
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		// No slash — Docker Hub official image
		return "docker.io", "library/" + parts[0], tag
	}

	// Check if first part looks like a registry (has dot or colon, or is "localhost")
	first := parts[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first, parts[1], tag
	}

	// No registry indicator — Docker Hub user image
	return "docker.io", ref, tag
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
