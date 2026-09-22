package store

import (
	"strings"
	"testing"
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
