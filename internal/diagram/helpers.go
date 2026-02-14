package diagram

import (
	"fmt"
	"regexp"
	"strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sanitizeID converts a name to a valid Mermaid node ID.
func sanitizeID(name string) string {
	return nonAlphaNum.ReplaceAllString(name, "_")
}

// quote wraps a string in double quotes for Mermaid labels.
func quote(s string) string {
	return fmt.Sprintf(`"%s"`, s)
}

// inferLayer determines the Flux dependency layer from the spec.path
// or from a cluster-vision.io/layer annotation.
func inferLayer(path string) string {
	lower := strings.ToLower(path)

	switch {
	case strings.Contains(lower, "/crds"):
		return "Foundation"
	case strings.Contains(lower, "/controllers"):
		return "Platform"
	case strings.Contains(lower, "/platform") || strings.Contains(lower, "/foundation"):
		return "Platform"
	case strings.Contains(lower, "/networking"):
		return "Foundation"
	case strings.Contains(lower, "/security") ||
		strings.Contains(lower, "/monitoring") ||
		strings.Contains(lower, "/identity") ||
		strings.Contains(lower, "/backup"):
		return "Middleware"
	case strings.Contains(lower, "/apps"):
		return "Apps"
	}
	return ""
}
