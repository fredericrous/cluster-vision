package store

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"grafana", "grafana"},
		{"kube-prometheus-stack", "kube-prometheus-stack"},
		{"My App", "my-app"},
		{"Cert Manager (webhook)", "cert-manager-webhook"},
		{"--weird__name--", "weird-name"},
		{"UPPER.case.chart", "upper-case-chart"},
		{"héllo wörld", "h-llo-w-rld"},
		{"", ""},
		{"!!!", ""},
	}

	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
