package metrics

import (
	"strings"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func hex(c byte) string { return strings.Repeat(string(c), 64) }

func TestEmitImagePinMetrics(t *testing.T) {
	pinnedOld, pinnedNow := "sha256:"+hex('a'), "sha256:"+hex('b')
	runningOld := "sha256:" + hex('c')
	pods := []model.PodImageInfo{
		// unpinned, running what the registry serves → unpinned only
		{Cluster: "homelab", Namespace: "seerr", PodName: "p1", Image: "ghcr.io/seerr-team/seerr:v3.4.1",
			ImageID: "ghcr.io/seerr-team/seerr@" + pinnedNow},
		// unpinned, running an older build of the tag → unpinned + drifted
		{Cluster: "homelab", Namespace: "radarr", PodName: "p2", Image: "ghcr.io/x/radarr:5",
			ImageID: "ghcr.io/x/radarr@" + runningOld},
		// pinned at a digest the registry no longer serves for the tag → moved
		{Cluster: "homelab", Namespace: "kb", PodName: "p3", Image: "ghcr.io/x/kb:1.0@" + pinnedOld},
		// pinned and current → nothing
		{Cluster: "homelab", Namespace: "ok", PodName: "p4", Image: "ghcr.io/x/ok:1.0@" + pinnedNow},
		// bare digest → nothing (pinned, no tag to move)
		{Cluster: "homelab", Namespace: "bare", PodName: "p5", Image: "ghcr.io/x/bare@" + pinnedOld},
		// duplicate pod of the first image in the same namespace → still one series
		{Cluster: "homelab", Namespace: "seerr", PodName: "p6", Image: "ghcr.io/seerr-team/seerr:v3.4.1",
			ImageID: "ghcr.io/seerr-team/seerr@" + pinnedNow},
	}
	lookup := func(image, tag string) (string, bool) {
		switch image {
		case "ghcr.io/seerr-team/seerr", "ghcr.io/x/radarr", "ghcr.io/x/kb", "ghcr.io/x/ok":
			return pinnedNow, true
		}
		return "", false
	}

	EmitImagePinMetrics(pods, lookup, nil)

	if n := testutil.CollectAndCount(ImageUnpinned); n != 2 {
		t.Fatalf("unpinned series = %d, want 2 (seerr, radarr)", n)
	}
	if v := testutil.ToFloat64(ImageUnpinned.WithLabelValues("homelab", "seerr", "ghcr.io/seerr-team/seerr:v3.4.1")); v != 1 {
		t.Fatalf("seerr unpinned = %v", v)
	}
	if n := testutil.CollectAndCount(ImageTagDrifted); n != 1 {
		t.Fatalf("drifted series = %d, want 1 (radarr)", n)
	}
	if v := testutil.ToFloat64(ImageTagDrifted.WithLabelValues("homelab", "radarr", "ghcr.io/x/radarr:5", runningOld, pinnedNow)); v != 1 {
		t.Fatalf("radarr drifted = %v", v)
	}
	if n := testutil.CollectAndCount(ImageTagMoved); n != 1 {
		t.Fatalf("moved series = %d, want 1 (kb)", n)
	}
	if v := testutil.ToFloat64(ImageTagMoved.WithLabelValues("homelab", "kb", "ghcr.io/x/kb:1.0@"+pinnedOld, pinnedOld, pinnedNow)); v != 1 {
		t.Fatalf("kb moved = %v", v)
	}

	// A refresh with the seerr image now pinned drops its series.
	pods[0].Image, pods[5].Image = "ghcr.io/seerr-team/seerr:v3.4.1@"+pinnedNow, "ghcr.io/seerr-team/seerr:v3.4.1@"+pinnedNow
	EmitImagePinMetrics(pods, lookup, nil)
	if n := testutil.CollectAndCount(ImageUnpinned); n != 1 {
		t.Fatalf("after repin: unpinned series = %d, want 1", n)
	}
	if n := testutil.CollectAndCount(ImageTagMoved); n != 1 {
		t.Fatalf("after repin: moved series = %d, want 1", n)
	}
}

func TestEmitImagePinMetricsWithoutLookup(t *testing.T) {
	pods := []model.PodImageInfo{
		{Cluster: "c", Namespace: "n", PodName: "p", Image: "ghcr.io/x/y:1", ImageID: "ghcr.io/x/y@sha256:" + hex('d')},
	}
	EmitImagePinMetrics(pods, nil, nil)
	if n := testutil.CollectAndCount(ImageUnpinned); n != 1 {
		t.Fatalf("unpinned = %d", n)
	}
	// No registry answer → no drift/moved claims.
	if testutil.CollectAndCount(ImageTagDrifted)+testutil.CollectAndCount(ImageTagMoved) != 0 {
		t.Fatal("drift/moved must need a registry digest")
	}
}
