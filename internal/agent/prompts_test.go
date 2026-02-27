package agent

import (
	"strings"
	"testing"
)

func TestBuildCapabilityPrompt(t *testing.T) {
	apps := []AppContext{
		{Name: "grafana", Namespace: "monitoring", ChartName: "grafana", Images: []string{"grafana/grafana:10.0.0"}},
		{Name: "authelia", Namespace: "security", ChartName: "authelia"},
	}

	got := BuildCapabilityPrompt(apps)

	if !strings.Contains(got, "Applications:") {
		t.Error("prompt should contain 'Applications:'")
	}
	if !strings.Contains(got, "grafana") {
		t.Error("prompt should contain 'grafana'")
	}
	if !strings.Contains(got, "authelia") {
		t.Error("prompt should contain 'authelia'")
	}
	if !strings.Contains(got, "monitoring") {
		t.Error("prompt should contain namespace 'monitoring'")
	}
	if !strings.Contains(got, "chart: grafana") {
		t.Error("prompt should contain 'chart: grafana'")
	}
	if !strings.Contains(got, "grafana/grafana:10.0.0") {
		t.Error("prompt should contain image reference")
	}
}

func TestBuildCapabilityPromptEmpty(t *testing.T) {
	got := BuildCapabilityPrompt(nil)
	if got != "Applications:\n" {
		t.Errorf("BuildCapabilityPrompt(nil) = %q, want %q", got, "Applications:\n")
	}
}

func TestBuildEnrichmentPrompt(t *testing.T) {
	app := AppContext{
		Name:         "grafana",
		Namespace:    "monitoring",
		ChartName:    "grafana",
		ChartVersion: "7.0.0",
		Images:       []string{"grafana/grafana:10.0.0"},
		VulnCritical: 2,
		VulnHigh:     5,
	}

	got := BuildEnrichmentPrompt(app)

	if !strings.Contains(got, `"grafana"`) {
		t.Error("prompt should contain app name")
	}
	if !strings.Contains(got, `"monitoring"`) {
		t.Error("prompt should contain namespace")
	}
	if !strings.Contains(got, "v7.0.0") {
		t.Error("prompt should contain chart version")
	}
	if !strings.Contains(got, "2 critical") {
		t.Error("prompt should contain critical vuln count")
	}
	if !strings.Contains(got, "5 high") {
		t.Error("prompt should contain high vuln count")
	}
}

func TestBuildEnrichmentPromptMinimal(t *testing.T) {
	app := AppContext{
		Name:      "myapp",
		Namespace: "default",
	}

	got := BuildEnrichmentPrompt(app)

	if !strings.Contains(got, `"myapp"`) {
		t.Error("prompt should contain app name")
	}
	// No chart → no "Chart:" line
	if strings.Contains(got, "Chart:") {
		t.Error("prompt should not contain Chart line for app without chart")
	}
	// No vulns → no "Vulnerabilities:" line
	if strings.Contains(got, "Vulnerabilities:") {
		t.Error("prompt should not contain Vulnerabilities line for app without vulns")
	}
}

func TestBuildDependencyPrompt(t *testing.T) {
	apps := []AppContext{
		{Name: "web", Namespace: "default", Images: []string{"web:v1"}},
		{Name: "db", Namespace: "default", Images: []string{"postgres:15"}},
	}

	got := BuildDependencyPrompt(apps)

	if !strings.Contains(got, "Applications:") {
		t.Error("prompt should contain 'Applications:'")
	}
	// Should be valid JSON containing app data
	if !strings.Contains(got, `"name": "web"`) {
		t.Error("prompt should contain JSON with app name 'web'")
	}
	if !strings.Contains(got, `"name": "db"`) {
		t.Error("prompt should contain JSON with app name 'db'")
	}
}
