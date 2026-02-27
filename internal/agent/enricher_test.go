package agent

import "testing"

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
