package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
)

// Integration tests run only against a real, disposable PostgreSQL:
//
//	CV_TEST_DATABASE_URL=postgres://u:p@localhost:5432/db?sslmode=disable go test ./internal/store/
//
// Every test works in its own schema-wide data (fresh application rows), so
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
