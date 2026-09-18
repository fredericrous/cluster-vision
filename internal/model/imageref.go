package model

import "strings"

// SplitImageRef splits a container image reference into its parts.
//
//	ghcr.io/foo/bar:1.2.3                 → ghcr.io, foo/bar, 1.2.3, ""
//	ghcr.io/foo/bar:1.2.3@sha256:<hex>    → ghcr.io, foo/bar, 1.2.3, sha256:<hex>
//	ghcr.io/foo/bar@sha256:<hex>          → ghcr.io, foo/bar, "",    sha256:<hex>
//	nginx                                  → docker.io, library/nginx, latest, ""
//	registry.local:5000/foo:1.0            → registry.local:5000, foo, 1.0, ""
//
// A pinned reference keeps BOTH its tag and its digest: the tag is what the
// version checker compares against the registry, the digest is what the
// cluster actually pulls. Only a reference with neither defaults to
// "latest" — a bare digest has an empty tag, not "latest".
func SplitImageRef(ref string) (registry, repo, tag, digest string) {
	if idx := strings.Index(ref, "@"); idx != -1 {
		digest = ref[idx+1:]
		ref = ref[:idx]
	}

	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		// A ":" after the last "/" is the tag separator; before it, a port.
		if idx > strings.LastIndex(ref, "/") {
			tag = ref[idx+1:]
			ref = ref[:idx]
		}
	}
	if tag == "" && digest == "" {
		tag = "latest"
	}

	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		return "docker.io", "library/" + parts[0], tag, digest
	}

	first := parts[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first, parts[1], tag, digest
	}

	return "docker.io", ref, tag, digest
}

// DigestOf extracts the sha256 digest from a kubelet imageID
// ("docker.io/foo/bar@sha256:<hex>", "docker-pullable://…@sha256:<hex>" or a
// bare "sha256:<hex>"). Empty when the string carries none.
func DigestOf(imageID string) string {
	if idx := strings.Index(imageID, "sha256:"); idx != -1 {
		d := imageID[idx:]
		if len(d) == len("sha256:")+64 {
			return d
		}
	}
	return ""
}
