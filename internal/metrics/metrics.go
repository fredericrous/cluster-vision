// Package metrics exposes Prometheus collectors for cluster-vision's
// security signals. The HTTP handler is wired in server.go at /metrics.
package metrics

import (
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ImageKEVCount: number of CVEs in this image listed on the CISA KEV
	// catalog. Per-(cluster, namespace, image) so alerts can target the
	// specific workload location. Reset between refreshes so a fixed image
	// disappears from the metric.
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
)

// EmitImageVulnMetrics emits gauges keyed by (cluster, namespace, image).
// `pods` provides the namespace dimension that ImageVuln deliberately
// drops; PodImageInfo carries Cluster (stamped at parse time), so the
// join across multi-cluster data stays attributable.
func EmitImageVulnMetrics(pods []model.PodImageInfo, vulns []model.ImageVuln) {
	// Reset to drop labels from previous refreshes (otherwise a fixed
	// image would keep its stale gauge series forever).
	ImageKEVCount.Reset()
	ImageMaxEPSS.Reset()

	if len(vulns) == 0 || len(pods) == 0 {
		return
	}

	type vKey struct{ cluster, image string }
	vulnByCI := make(map[vKey]*model.ImageVuln, len(vulns))
	for i := range vulns {
		vulnByCI[vKey{cluster: vulns[i].Cluster, image: vulns[i].Image}] = &vulns[i]
	}

	type triple struct{ cluster, namespace, image string }
	seen := make(map[triple]struct{}, len(pods))

	for _, p := range pods {
		v, ok := vulnByCI[vKey{cluster: p.Cluster, image: p.Image}]
		if !ok {
			continue
		}
		t := triple{cluster: p.Cluster, namespace: p.Namespace, image: p.Image}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		ImageKEVCount.WithLabelValues(p.Cluster, p.Namespace, p.Image).Set(float64(v.KEVCount))
		ImageMaxEPSS.WithLabelValues(p.Cluster, p.Namespace, p.Image).Set(v.MaxEPSS)
	}
}
