package agent

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeEnum(t *testing.T) {
	tests := []struct {
		value    string
		fallback string
		want     string
	}{
		{"high", "medium", "high"},
		{"medium", "medium", "medium"},
		{"low", "medium", "low"},
		{"invalid", "medium", "medium"},
		{"", "low", "low"},
		{"HIGH", "medium", "medium"}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.value+"_fallback_"+tt.fallback, func(t *testing.T) {
			got := normalizeEnum(tt.value, tt.fallback)
			if got != tt.want {
				t.Errorf("normalizeEnum(%q, %q) = %q, want %q", tt.value, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestNormalizeTimeCategory(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"tolerate", "tolerate"},
		{"invest", "invest"},
		{"migrate", "migrate"},
		{"eliminate", "eliminate"},
		{"", ""},
		{"bad", ""},
		{"TOLERATE", ""},
	}

	for _, tt := range tests {
		t.Run("value_"+tt.value, func(t *testing.T) {
			got := normalizeTimeCategory(tt.value)
			if got != tt.want {
				t.Errorf("normalizeTimeCategory(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeConfidence(t *testing.T) {
	tests := []struct {
		in   float64
		want float32
	}{
		{0.85, 0.85},
		{1, 1},
		{85, 0.85},   // percentage scale
		{100, 1},     // percentage scale, upper bound
		{250, 1},     // nonsense: clamped
		{-0.5, 0.01}, // negative: floor
		{0, 0.01},    // missing/zero must not read as "never enriched"
		{math.NaN(), 0.01},
		{math.Inf(1), 0.01},
	}
	for _, tt := range tests {
		got := normalizeConfidence(tt.in)
		if math.Abs(float64(got-tt.want)) > 1e-6 {
			t.Errorf("normalizeConfidence(%v) = %v, want %v", tt.in, got, tt.want)
		}
		if got == 0 {
			t.Errorf("normalizeConfidence(%v) = 0: EnrichNew would re-enrich forever", tt.in)
		}
	}
}

func TestPlanDependencies(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	ids := map[string]uuid.UUID{"a": a, "b": b, "c": c}
	plan, dropped := planDependencies([]InferredDependency{
		{Source: "a", Target: "b", Reason: "calls"},
		{Source: "a", Target: "b", Reason: "duplicate"},
		{Source: "a", Target: "a", Reason: "self-loop"},
		{Source: "a", Target: "ghost", Reason: "unknown target"},
		{Source: "ghost", Target: "c", Reason: "unknown source"},
		{Source: "b", Target: "c", Reason: "stores in"},
	}, ids)
	if dropped != 4 {
		t.Errorf("dropped = %d, want 4", dropped)
	}
	if len(plan[a]) != 1 || plan[a][0].TargetAppID != b || *plan[a][0].Description != "calls" {
		t.Errorf("plan[a] = %+v", plan[a])
	}
	if len(plan[b]) != 1 || plan[b][0].TargetAppID != c {
		t.Errorf("plan[b] = %+v", plan[b])
	}
	if len(plan[c]) != 0 {
		t.Errorf("plan[c] = %+v, want none", plan[c])
	}
}
