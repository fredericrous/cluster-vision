-- KEV/EPSS exploit-risk enrichment cache.
-- Populated by internal/versions/exploit_enrichment.go on a 24h ticker
-- (see internal/server/server.go enrichmentLoop). The Trivy parser stores
-- per-image CVE lists in memory only; this table holds the global
-- per-CVE intel that lookups join against.
CREATE TABLE IF NOT EXISTS cve_enrichment (
    cve_id          TEXT PRIMARY KEY,
    kev_listed      BOOLEAN NOT NULL DEFAULT FALSE,
    kev_added_date  DATE,
    kev_due_date    DATE,
    kev_short_name  TEXT,
    epss_score      DOUBLE PRECISION,
    epss_percentile DOUBLE PRECISION,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS cve_enrichment_kev_listed_idx
    ON cve_enrichment(kev_listed) WHERE kev_listed = TRUE;

CREATE INDEX IF NOT EXISTS cve_enrichment_epss_idx
    ON cve_enrichment(epss_score DESC) WHERE epss_score > 0.5;
