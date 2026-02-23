package diagram

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// CertificateRow represents a single row in the certificates table.
type CertificateRow struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Cluster     string `json:"cluster"`
	DNSNames    string `json:"dnsNames"`
	Issuer      string `json:"issuer"` // "kind/name"
	NotAfter    string `json:"notAfter"`
	RenewalTime string `json:"renewalTime"`
	Ready       string `json:"ready"`
	ExpiryDays  int    `json:"expiryDays"`  // days until expiry, -1 if unknown
	ExpiryLevel string `json:"expiryLevel"` // "ok", "warning", "critical"
}

// GenerateCertificates produces a table of cert-manager certificates.
func GenerateCertificates(data *model.ClusterData) model.DiagramResult {
	if len(data.Certificates) == 0 {
		return model.DiagramResult{
			ID:      "certificates",
			Title:   "Certificates",
			Type:    "markdown",
			Content: "*No certificate data available.*",
		}
	}

	var rows []CertificateRow
	for _, c := range data.Certificates {
		ready := "no"
		if c.Ready {
			ready = "yes"
		}

		expiryDays := -1
		expiryLevel := "ok"
		if c.NotAfter != "" {
			if t, err := time.Parse(time.RFC3339, c.NotAfter); err == nil {
				expiryDays = int(time.Until(t).Hours() / 24)
				if expiryDays < 30 {
					expiryLevel = "critical"
				} else if expiryDays < 90 {
					expiryLevel = "warning"
				}
			}
		}

		issuer := c.IssuerName
		if c.IssuerKind != "" && c.IssuerKind != "Issuer" {
			issuer = c.IssuerKind + "/" + c.IssuerName
		}

		rows = append(rows, CertificateRow{
			Name:        c.Name,
			Namespace:   c.Namespace,
			Cluster:     c.Cluster,
			DNSNames:    strings.Join(c.DNSNames, ", "),
			Issuer:      issuer,
			NotAfter:    c.NotAfter,
			RenewalTime: c.RenewalTime,
			Ready:       ready,
			ExpiryDays:  expiryDays,
			ExpiryLevel: expiryLevel,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cluster != rows[j].Cluster {
			return rows[i].Cluster < rows[j].Cluster
		}
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})

	tableJSON, _ := json.Marshal(rows)
	return model.DiagramResult{
		ID:      "certificates",
		Title:   "Certificates",
		Type:    "table",
		Content: string(tableJSON),
	}
}
