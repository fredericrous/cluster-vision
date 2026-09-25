package metrics

import (
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSeriesReplaceDeletesOnlyStale(t *testing.T) {
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "t_replace"}, []string{"cluster", "image"})
	s := newSeries(vec)

	b := newBatch()
	b.set(1, "a", "x")
	b.set(2, "a", "y")
	b.set(3, "b", "z")
	s.replace(b, nil)
	if n := testutil.CollectAndCount(vec); n != 3 {
		t.Fatalf("series = %d, want 3", n)
	}

	// y is fixed; x changes value. The kept series must never disappear.
	b = newBatch()
	b.set(5, "a", "x")
	b.set(3, "b", "z")
	s.replace(b, nil)
	if n := testutil.CollectAndCount(vec); n != 2 {
		t.Fatalf("series = %d, want 2 after y went away", n)
	}
	if v := testutil.ToFloat64(vec.WithLabelValues("a", "x")); v != 5 {
		t.Fatalf("x = %v, want 5", v)
	}
}

func TestSeriesReplaceKeepsPartialClusters(t *testing.T) {
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "t_partial"}, []string{"cluster", "image"})
	s := newSeries(vec)

	b := newBatch()
	b.set(1, "a", "x")
	b.set(1, "b", "z")
	s.replace(b, nil)

	// Cluster b's list calls failed: it reports nothing this refresh.
	b = newBatch()
	b.set(1, "a", "x")
	s.replace(b, map[string]bool{"b": true})
	if n := testutil.CollectAndCount(vec); n != 2 {
		t.Fatalf("series = %d, want 2 (b's kept through the partial refresh)", n)
	}

	// b recovers and z is really gone now.
	b = newBatch()
	b.set(1, "a", "x")
	s.replace(b, nil)
	if n := testutil.CollectAndCount(vec); n != 1 {
		t.Fatalf("series = %d, want 1 once b reports again", n)
	}
}

func TestEmitImageVulnMetricsKeepsPartialCluster(t *testing.T) {
	pods := []model.PodImageInfo{
		{Cluster: "c1", Namespace: "ns", Image: "img:1"},
		{Cluster: "c2", Namespace: "ns", Image: "img:2"},
	}
	vulns := []model.ImageVuln{
		{Cluster: "c1", Image: "img:1", KEVCount: 1},
		{Cluster: "c2", Image: "img:2", KEVCount: 2},
	}
	EmitImageVulnMetrics(pods, vulns, nil)
	if n := testutil.CollectAndCount(ImageKEVCount); n != 2 {
		t.Fatalf("kev series = %d, want 2", n)
	}
	// c2 failed to list pods this time.
	EmitImageVulnMetrics(pods[:1], vulns[:1], map[string]bool{"c2": true})
	if v := testutil.ToFloat64(ImageKEVCount.WithLabelValues("c2", "ns", "img:2")); v != 2 {
		t.Fatalf("c2 series = %v, want kept at 2", v)
	}
	EmitImageVulnMetrics(nil, nil, nil)
	if n := testutil.CollectAndCount(ImageKEVCount); n != 0 {
		t.Fatalf("kev series = %d, want 0", n)
	}
}
