package store

import (
	"context"
	"testing"
)

func strp(s string) *string { return &s }

func TestVersionHistoryKeyIncludesVulnCounts(t *testing.T) {
	base := &VersionHistoryEntry{ChartVersion: strp("1.0.0"), ImageTag: strp("v1"), VulnCritical: 0, VulnHigh: 2}
	same := &VersionHistoryEntry{ChartVersion: strp("1.0.0"), ImageTag: strp("v1"), VulnCritical: 0, VulnHigh: 2}
	if !keyOf(base).equal(keyOf(same)) {
		t.Fatal("identical entries must dedup")
	}
	for name, e := range map[string]*VersionHistoryEntry{
		"critical": {ChartVersion: strp("1.0.0"), ImageTag: strp("v1"), VulnCritical: 1, VulnHigh: 2},
		"high":     {ChartVersion: strp("1.0.0"), ImageTag: strp("v1"), VulnCritical: 0, VulnHigh: 0},
		"tag":      {ChartVersion: strp("1.0.0"), ImageTag: strp("v2"), VulnCritical: 0, VulnHigh: 2},
		"chart":    {ChartVersion: nil, ImageTag: strp("v1"), VulnCritical: 0, VulnHigh: 2},
	} {
		if keyOf(base).equal(keyOf(e)) {
			t.Errorf("a changed %s must not dedup", name)
		}
	}
}

func TestInsertVersionHistoryRecordsVulnChange(t *testing.T) {
	db := testDB(t)
	app := testApp(t, db)
	ctx := context.Background()
	insert := func(critical int) {
		t.Helper()
		if err := db.InsertVersionHistory(ctx, &VersionHistoryEntry{AppID: app.ID, ImageTag: strp("v1"), VulnCritical: critical}); err != nil {
			t.Fatal(err)
		}
	}
	insert(0)
	insert(0) // duplicate: dropped
	insert(3) // same tag, new critical count: recorded
	got, err := db.GetVersionHistory(ctx, app.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].VulnCritical != 3 {
		t.Fatalf("want 2 entries with the latest at 3 critical, got %+v", got)
	}
}
