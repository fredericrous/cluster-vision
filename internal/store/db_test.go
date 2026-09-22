package store

import (
	"context"
	"strings"
	"testing"
	"time"
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
