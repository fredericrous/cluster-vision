package versions

import "testing"

func TestHighestSemver(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
	}{
		{"simple", []string{"1.0.0", "2.0.0", "1.5.0"}, "2.0.0"},
		{"with v prefix", []string{"v1.0.0", "v2.1.0", "v1.5.3"}, "v2.1.0"},
		{"pre-release lower", []string{"1.0.0", "1.0.1-rc1", "1.0.1"}, "1.0.1"},
		{"patch ordering", []string{"1.2.3", "1.2.10", "1.2.9"}, "1.2.10"},
		{"empty list", []string{}, ""},
		{"non-semver ignored", []string{"latest", "main", "1.0.0"}, "1.0.0"},
		{"mixed", []string{"0.1.0", "0.2.0", "0.1.5"}, "0.2.0"},
		{"two part", []string{"1.0", "2.0", "1.5"}, "2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highestSemver(tt.versions)
			if got != tt.want {
				t.Errorf("highestSemver(%v) = %q, want %q", tt.versions, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
		major int
		minor int
		patch int
		pre   string
	}{
		{"1.2.3", true, 1, 2, 3, ""},
		{"v1.2.3", true, 1, 2, 3, ""},
		{"1.2.3-rc1", true, 1, 2, 3, "-rc1"},
		{"1.2.3+build", true, 1, 2, 3, "+build"},
		{"1.2", true, 1, 2, 0, ""},
		{"latest", false, 0, 0, 0, ""},
		{"1", false, 0, 0, 0, ""},
		{"1.2.3.4", false, 0, 0, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sv, ok := parseSemver(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseSemver(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if sv.major != tt.major || sv.minor != tt.minor || sv.patch != tt.patch || sv.pre != tt.pre {
				t.Errorf("parseSemver(%q) = {%d, %d, %d, %q}, want {%d, %d, %d, %q}",
					tt.input, sv.major, sv.minor, sv.patch, sv.pre, tt.major, tt.minor, tt.patch, tt.pre)
			}
		})
	}
}

func TestSemverLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.0.0", false},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "1.0.1", true},
		{"1.0.0-rc1", "1.0.0", true},  // pre-release < release
		{"1.0.0", "1.0.0-rc1", false}, // release > pre-release
		{"1.0.0", "1.0.0", false},     // equal
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a, _ := parseSemver(tt.a)
			b, _ := parseSemver(tt.b)
			got := a.less(b)
			if got != tt.want {
				t.Errorf("(%q).less(%q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
