package store

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
)

func TestWithConnectTimeout(t *testing.T) {
	got, err := withConnectTimeout("postgres://u:p%40ss@host:5432/db?sslmode=disable", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "connect_timeout=10") || !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("connect_timeout not added: %s", got)
	}
	if !strings.Contains(got, "u:p%40ss@host") {
		t.Fatalf("userinfo must survive: %s", got)
	}
	kept, err := withConnectTimeout("postgres://u@host/db?connect_timeout=3", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kept, "connect_timeout=3") || strings.Contains(kept, "connect_timeout=10") {
		t.Fatalf("an explicit connect_timeout must be kept: %s", kept)
	}
}

func TestNewWithRetryGivesUpWithinBudget(t *testing.T) {
	// Nothing listens on this port: every attempt fails fast with a refusal.
	start := time.Now()
	_, err := NewWithRetry(context.Background(), "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1", 5*time.Second)
	if err == nil {
		t.Fatal("expected an error against a closed port")
	}
	if !strings.Contains(err.Error(), "attempt(s)") {
		t.Fatalf("error should count attempts: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 12*time.Second {
		t.Fatalf("retry must respect the budget, took %s", elapsed)
	}
}

// On a deadline awaitMigration must ask the migrator to stop and then wait
// for Up to return: returning while Up still runs lets the deferred Close
// cut a migration mid-statement and leaves the schema dirty.
func TestAwaitMigrationWaitsForUpAfterDeadline(t *testing.T) {
	stop := make(chan bool, 1)
	release := make(chan struct{})
	var upReturned atomic.Bool
	up := func() error {
		<-release
		upReturned.Store(true)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- awaitMigration(ctx, up, stop) }()

	cancel()
	select {
	case <-stop:
	case <-time.After(5 * time.Second):
		t.Fatal("awaitMigration did not ask the migrator to stop")
	}
	select {
	case err := <-errc:
		t.Fatalf("awaitMigration returned (%v) while Up was still running", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	err := <-errc
	if !upReturned.Load() {
		t.Fatal("returned before Up did")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a stopped migration must report the deadline, got %v", err)
	}
}

func TestAwaitMigrationNoChangeIsSuccess(t *testing.T) {
	err := awaitMigration(context.Background(), func() error { return migrate.ErrNoChange }, make(chan bool, 1))
	if err != nil {
		t.Fatalf("ErrNoChange must be success, got %v", err)
	}
}
