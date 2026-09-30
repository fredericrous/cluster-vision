package model

import "strings"

// dockerHubHosts are the names Docker Hub goes by. Pod specs say
// "nginx:1.25" or "docker.io/library/nginx:1.25"; trivy-operator reports
// the same image with registry.server "index.docker.io".
var dockerHubHosts = map[string]bool{
	"docker.io":            true,
	"index.docker.io":      true,
	"registry-1.docker.io": true,
}

// ImageKey is the canonical form of an image reference, the key vulnerability
// data is joined on: Docker Hub under one host name with its implicit
// "library/" namespace spelled out, then ":tag", or "@digest" for a
// reference that has no tag.
//
//	nginx:1.25                             → docker.io/library/nginx:1.25
//	index.docker.io/library/nginx:1.25     → docker.io/library/nginx:1.25
//	docker.io/nginx:1.25@sha256:<hex>      → docker.io/library/nginx:1.25
//	ghcr.io/foo/bar@sha256:<hex>           → ghcr.io/foo/bar@sha256:<hex>
//
// A pull-through cache in front of the registries (a containerd mirror)
// does not change either side: pods and Trivy both name the upstream.
func ImageKey(ref string) string {
	registry, repo, tag, digest := SplitImageRef(ref)
	if dockerHubHosts[registry] {
		registry = "docker.io"
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
	}
	if tag == "" {
		return registry + "/" + repo + "@" + digest
	}
	return registry + "/" + repo + ":" + tag
}

// VulnIndex looks up ImageVulns by image reference, in any spelling
// ImageKey reconciles.
type VulnIndex struct {
	byCluster map[vulnIndexKey]ImageVuln
	worst     map[string]ImageVuln
}

type vulnIndexKey struct{ cluster, image string }

// NewVulnIndex indexes vulns by (cluster, key) and by key alone, where the
// keys of a report are its ImageKey and, when it has a digest, its
// repository@digest: a digest-pinned pod is joined on what it runs.
func NewVulnIndex(vulns []ImageVuln) VulnIndex {
	idx := VulnIndex{
		byCluster: make(map[vulnIndexKey]ImageVuln, len(vulns)),
		worst:     make(map[string]ImageVuln, len(vulns)),
	}
	for _, v := range vulns {
		keys := []string{ImageKey(v.Image)}
		if v.Digest != "" {
			keys = append(keys, digestKey(v.Image, v.Digest))
		}
		for _, key := range keys {
			idx.byCluster[vulnIndexKey{v.Cluster, key}] = v
			if cur, ok := idx.worst[key]; !ok || worseVuln(v, cur) {
				idx.worst[key] = v
			}
		}
	}
	return idx
}

// digestKey is the ImageKey of ref's repository at digest.
func digestKey(ref, digest string) string {
	registry, repo, _, _ := SplitImageRef(ref)
	return ImageKey(registry + "/" + repo + "@" + digest)
}

// Lookup returns the report for ref in cluster. With cluster "" it returns
// the worst report any cluster has for the image: the same image scanned in
// two clusters can differ (vulnerability database age), and a caller
// aggregating across clusters must get the same answer on every refresh,
// whatever order the reports came in.
//
// A ref pinned by digest is looked up by that digest first, then by its tag.
func (x VulnIndex) Lookup(cluster, ref string) (ImageVuln, bool) {
	keys := []string{ImageKey(ref)}
	if _, _, _, digest := SplitImageRef(ref); digest != "" {
		keys = append([]string{digestKey(ref, digest)}, keys...)
	}
	for _, key := range keys {
		var v ImageVuln
		var ok bool
		if cluster != "" {
			v, ok = x.byCluster[vulnIndexKey{cluster, key}]
		} else {
			v, ok = x.worst[key]
		}
		if ok {
			return v, true
		}
	}
	return ImageVuln{}, false
}

// worseVuln orders reports by severity counts, then cluster name so the
// choice is total.
func worseVuln(a, b ImageVuln) bool {
	for _, d := range [][2]int{
		{a.Critical, b.Critical}, {a.High, b.High}, {a.Medium, b.Medium},
		{a.Low, b.Low}, {a.KEVCount, b.KEVCount},
	} {
		if d[0] != d[1] {
			return d[0] > d[1]
		}
	}
	if a.MaxEPSS != b.MaxEPSS {
		return a.MaxEPSS > b.MaxEPSS
	}
	return a.Cluster < b.Cluster
}
