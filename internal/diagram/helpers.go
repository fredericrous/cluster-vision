package diagram

import (
	"regexp"
	"strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sanitizeID converts a name to a valid Mermaid node ID.
func sanitizeID(name string) string {
	return nonAlphaNum.ReplaceAllString(name, "_")
}

// mermaidEscaper turns free text into something safe inside a quoted
// Mermaid label (`id["…"]`, `-->|"…"|`). A `"` would end the label and
// break the whole diagram; `<`/`>` would be read as HTML; a "`" at the
// start switches the label to markdown. Mermaid decodes `#name;` and
// `#code;` back into the character, so each becomes an entity — `#`
// first, so text that already looks like an entity stays literal.
var mermaidEscaper = strings.NewReplacer(
	"#", "#35;",
	`"`, "#quot;",
	"<", "#lt;",
	">", "#gt;",
	"`", "#96;",
	"\r\n", " ",
	"\n", " ",
	"\r", " ",
)

// mermaidText escapes free text (names from docker-compose, tfstate,
// cluster objects) for a Mermaid label. Markup the generator adds itself,
// such as <br/>, goes around the escaped parts, never through this.
func mermaidText(s string) string {
	return mermaidEscaper.Replace(s)
}
