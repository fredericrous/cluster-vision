// Package metrics exposes Prometheus collectors for cluster-vision's
// security signals. The HTTP handler is wired in server.go at /metrics.
package metrics

import (
	"strings"
	"sync"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ImageKEVCount: number of CVEs in this image listed on the CISA KEV
	// catalog. Per-(cluster, namespace, image) so alerts can target the
	// specific workload location. Stale series are deleted between refreshes
	// so a fixed image disappears from the metric.
	ImageKEVCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_vision_image_kev_count",
		Help: "Number of CVEs in the image listed on CISA KEV (Known Exploited Vulnerabilities).",
	}, []string{"cluster", "namespace", "image"})

	// ImageMaxEPSS: highest FIRST EPSS score across the image's CVEs. EPSS
	// gives the probability of exploitation in the next 30 days (0..1).
	ImageMaxEPSS = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_vision_image_max_epss",
		Help: "Highest FIRST EPSS score across CVEs in the image (0..1).",
	}, []string{"cluster", "namespace", "image"})

	// Image pin findings, per (cluster, namespace, image) like the vuln
	// gauges so the same alert → playbook plumbing applies. Consumed by the
	// ImageUnpinned / ImageTagMoved / ImageTagDrifted rules and, through
	// them, sre-agent's image_digest_pin playbook. Stale series are deleted
	// between refreshes.
	//
	// ImageUnpinned: the pod spec pulls by tag only (no @sha256). Whatever
	// the tag points at tomorrow is what the next pull gets.
	ImageUnpinned = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_vision_image_unpinned",
		Help: "1 when the workload's image reference carries no @sha256 digest (pull-by-tag).",
	}, []string{"cluster", "namespace", "image"})

	// ImageTagMoved: the reference IS pinned, but the registry now serves a
	// different digest for that tag — upstream re-tagged. The cluster still
	// runs the pinned bytes; the pin is stale, not wrong.
	ImageTagMoved = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_vision_image_tag_moved",
		Help: "1 when a digest-pinned tag now resolves to a different digest upstream.",
	}, []string{"cluster", "namespace", "image", "pinned_digest", "registry_digest"})

	// ImageTagDrifted: unpinned, and the RUNNING digest (kubelet imageID) is
	// not what the registry serves for the tag — a mutable tag moved under
	// a running workload; the next pod restart pulls different bytes than
	// its siblings. The strongest argument for pinning, measured.
	ImageTagDrifted = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_vision_image_tag_drifted",
		Help: "1 when an unpinned tag's running digest differs from what the registry serves now.",
	}, []string{"cluster", "namespace", "image", "running_digest", "registry_digest"})

	// EnrichmentLastFetch: unix timestamp of the most recent successful
	// KEV/EPSS feed fetch — drives the staleness alert.
	EnrichmentLastFetch = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_vision_enrichment_last_fetch_timestamp_seconds",
		Help: "Unix timestamp of the last successful CISA KEV / FIRST EPSS refresh.",
	})

	// EnrichmentCVETotal: per-source count of cached CVEs (kev=true count
	// and epss>0 count). Sanity gauge so we can spot a feed-format change.
	EnrichmentCVETotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_vision_enrichment_cve_total",
		Help: "Number of CVEs currently cached, by source.",
	}, []string{"source"})

	// SnapshotsTotal counts cluster snapshots persisted (a new observed
	// state, or a new revision).
	SnapshotsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cluster_vision_snapshots_total",
		Help: "Number of cluster snapshots written.",
	})

	// SnapshotsSkipped counts refresh ticks that did not produce a
	// snapshot, by reason: unchanged (same observed hash), partial_parse
	// (a cluster list failed, data would be incomplete), error (DB write).
	SnapshotsSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cluster_vision_snapshots_skipped_total",
		Help: "Refresh ticks that did not write a snapshot, by reason.",
	}, []string{"reason"})

	// SnapshotDrift is 1 when the latest snapshot changed without a new
	// desired-state revision — something moved outside GitOps.
	SnapshotDrift = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_vision_snapshot_drift",
		Help: "1 if the most recent snapshot changed with no new Flux revision.",
	})
)

// EmitImageVulnMetrics emits gauges keyed by (cluster, namespace, image).
// `pods` provides the namespace dimension that ImageVuln deliberately
// drops; PodImageInfo carries Cluster (stamped at parse time), so the
// join across multi-cluster data stays attributable.
//
// Series that are no longer observed are deleted AFTER the new values are
// set (never Reset-then-refill, which lets a scrape see an empty set and
// resolves every alert for a moment). Clusters in `partial` — whose list
// calls failed this refresh — keep their previous series: missing data is
// not evidence that an image was fixed.
func EmitImageVulnMetrics(pods []model.PodImageInfo, vulns []model.ImageVuln, partial map[string]bool) {
	kev, epss := newBatch(), newBatch()
	if len(vulns) > 0 && len(pods) > 0 {
		idx := model.NewVulnIndex(vulns)
		for _, p := range pods {
			v, ok := idx.Lookup(p.Cluster, p.Image)
			if !ok {
				continue
			}
			kev.set(float64(v.KEVCount), p.Cluster, p.Namespace, p.Image)
			epss.set(v.MaxEPSS, p.Cluster, p.Namespace, p.Image)
		}
	}
	kevSeries.replace(kev, partial)
	epssSeries.replace(epss, partial)
}

// DigestLookup answers "what does the registry serve for image:tag right
// now" — ImageChecker.GetDigest in production. ok=false means unknown.
type DigestLookup func(image, tag string) (digest string, ok bool)

// EmitImagePinMetrics emits the pin findings for every running image.
// `image` is the pod spec reference as written (with its @sha256 when
// pinned), so an alert's label is what the GitOps manifest contains — the
// binding sre-agent's playbook hands to patch-agent. Stale series are
// deleted after the new ones are set, except for clusters in `partial`
// (see EmitImageVulnMetrics).
func EmitImagePinMetrics(pods []model.PodImageInfo, lookup DigestLookup, partial map[string]bool) {
	unpinned, moved, drifted := newBatch(), newBatch(), newBatch()
	for _, p := range pods {
		registry, repo, tag, pinned := model.SplitImageRef(p.Image)
		if tag == "" {
			// Bare digest: pinned, and no tag for the registry to move.
			continue
		}
		var upstream string
		if lookup != nil {
			upstream, _ = lookup(registry+"/"+repo, tag)
		}
		if pinned == "" {
			unpinned.set(1, p.Cluster, p.Namespace, p.Image)
			running := model.DigestOf(p.ImageID)
			if upstream != "" && running != "" && running != upstream {
				drifted.set(1, p.Cluster, p.Namespace, p.Image, running, upstream)
			}
			continue
		}
		if upstream != "" && upstream != pinned {
			moved.set(1, p.Cluster, p.Namespace, p.Image, pinned, upstream)
		}
	}
	unpinnedSeries.replace(unpinned, partial)
	movedSeries.replace(moved, partial)
	driftedSeries.replace(drifted, partial)
}

// ---- stale-series tracking ----

var (
	kevSeries      = newSeries(ImageKEVCount)
	epssSeries     = newSeries(ImageMaxEPSS)
	unpinnedSeries = newSeries(ImageUnpinned)
	movedSeries    = newSeries(ImageTagMoved)
	driftedSeries  = newSeries(ImageTagDrifted)
)

// batch is one refresh's worth of values for a GaugeVec, keyed by the
// joined label values. The first label of every vec here is "cluster".
type batch map[string]sample

type sample struct {
	labels []string
	value  float64
}

func newBatch() batch { return batch{} }

func (b batch) set(v float64, labels ...string) {
	b[strings.Join(labels, "\x00")] = sample{labels: labels, value: v}
}

// series remembers which label sets it published last time, so the next
// publish can delete exactly the ones that disappeared.
type series struct {
	mu   sync.Mutex
	vec  *prometheus.GaugeVec
	last batch
}

func newSeries(vec *prometheus.GaugeVec) *series { return &series{vec: vec, last: batch{}} }

// replace publishes next: every value is set first, then series absent from
// next are deleted — unless their cluster is in keep, in which case they
// are carried over untouched.
func (s *series) replace(next batch, keep map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, smp := range next {
		s.vec.WithLabelValues(smp.labels...).Set(smp.value)
	}
	for k, smp := range s.last {
		if _, still := next[k]; still {
			continue
		}
		if keep[smp.labels[0]] {
			next[k] = smp
			continue
		}
		s.vec.DeleteLabelValues(smp.labels...)
	}
	s.last = next
}
