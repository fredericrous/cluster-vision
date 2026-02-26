package diagram

import "regexp"

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sanitizeID converts a name to a valid Mermaid node ID.
func sanitizeID(name string) string {
	return nonAlphaNum.ReplaceAllString(name, "_")
}
