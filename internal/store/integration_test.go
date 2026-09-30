package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/google/uuid"
)

// Integration tests run only against a real, disposable PostgreSQL:
//
//	CV_TEST_DATABASE_URL=postgres://u:p@localhost:5432/db?sslmode=disable go test ./internal/store/
//
// Every test creates its own rows (fresh applications, unique hashes), so
// they tolerate a database that already has the migrations applied.
func testDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("CV_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CV_TEST_DATABASE_URL not set")
	}
	db, err := New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func testApp(t *testing.T, db *DB) *Application {
	t.Helper()
	a, _, err := db.UpsertApplicationByName(context.Background(), "it-"+uuid.NewString(), func(*Application) {})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFindK8sSourceMissIsNilNil(t *testing.T) {
	db := testDB(t)
	app := testApp(t, db)
	hr := "nope"
	got, err := db.FindK8sSource(context.Background(), app.ID, "c", "ns", &hr)
	if err != nil || got != nil {
		t.Fatalf("a miss must be (nil, nil), got (%v, %v)", got, err)
	}
}

func TestFindK8sSourceErrorIsNotAMiss(t *testing.T) {
	db := testDB(t)
	app := testApp(t, db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := db.FindK8sSource(ctx, app.ID, "c", "ns", nil)
	if err == nil {
		t.Fatalf("a failed lookup must return an error, got (%v, nil)", got)
	}
}

func TestK8sSourceIdentityIsUnique(t *testing.T) {
	db := testDB(t)
	app := testApp(t, db)
	ctx := context.Background()
	hr := "hr"
	if err := db.UpsertK8sSource(ctx, &K8sSource{AppID: app.ID, Cluster: "c", Namespace: "ns", HelmRelease: &hr}); err != nil {
		t.Fatal(err)
	}
	// A second row with the same identity and a fresh id is exactly the
	// duplicate a swallowed lookup error used to create.
	if err := db.UpsertK8sSource(ctx, &K8sSource{AppID: app.ID, Cluster: "c", Namespace: "ns", HelmRelease: &hr}); err == nil {
		t.Fatal("expected the unique index to reject a duplicate k8s source")
	}
}

func TestInsertSnapshotRejectsUnchangedHashUnderLock(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	hash := []byte(uuid.NewString())
	if err := db.InsertSnapshot(ctx, &Snapshot{ObservedHash: hash}, &model.ClusterData{}); err != nil {
		t.Fatal(err)
	}
	err := db.InsertSnapshot(ctx, &Snapshot{ObservedHash: hash}, &model.ClusterData{})
	if !errors.Is(err, ErrSnapshotUnchanged) {
		t.Fatalf("second insert of the same hash = %v, want ErrSnapshotUnchanged", err)
	}
}

func TestReplaceAIDependenciesPrunesOnlyInferred(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	src, keep, stale, legacy := testApp(t, db), testApp(t, db), testApp(t, db), testApp(t, db)

	// A dependency recorded before origin was tracked (origin NULL).
	if _, err := db.Pool.Exec(ctx, `INSERT INTO app_dependencies (source_app_id, target_app_id) VALUES ($1, $2)`, src.ID, legacy.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceAIDependencies(ctx, src.ID, []AppDependency{{TargetAppID: keep.ID}, {TargetAppID: stale.ID}}); err != nil {
		t.Fatal(err)
	}
	// Re-enrichment no longer infers `stale`, and proposes a self-loop.
	if err := db.ReplaceAIDependencies(ctx, src.ID, []AppDependency{{TargetAppID: keep.ID}, {TargetAppID: src.ID}}); err != nil {
		t.Fatal(err)
	}

	deps, err := db.ListDependencies(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, d := range deps {
		got[d.TargetAppID] = true
	}
	if !got[keep.ID] || !got[legacy.ID] || got[stale.ID] || got[src.ID] || len(got) != 2 {
		t.Fatalf("want {keep, legacy}, got %v", got)
	}
}
