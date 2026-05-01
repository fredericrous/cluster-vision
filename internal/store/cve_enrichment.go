package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CVEEnrichmentRow is one row of the cve_enrichment table — the global
// per-CVE intel we cache from CISA KEV and FIRST EPSS.
type CVEEnrichmentRow struct {
	CVEID          string
	KEVListed      bool
	KEVAddedDate   *time.Time
	KEVDueDate     *time.Time
	KEVShortName   *string
	EPSSScore      *float64
	EPSSPercentile *float64
}

// UpsertCVEEnrichmentBatch inserts or updates rows in a single transaction.
// Used by the daily refresh — the input slice is the union of CISA KEV +
// FIRST EPSS records, with KEV-only and EPSS-only rows mixed.
func (db *DB) UpsertCVEEnrichmentBatch(ctx context.Context, rows []CVEEnrichmentRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `INSERT INTO cve_enrichment
		(cve_id, kev_listed, kev_added_date, kev_due_date, kev_short_name,
		 epss_score, epss_percentile, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (cve_id) DO UPDATE SET
		  kev_listed       = EXCLUDED.kev_listed OR cve_enrichment.kev_listed,
		  kev_added_date   = COALESCE(EXCLUDED.kev_added_date, cve_enrichment.kev_added_date),
		  kev_due_date     = COALESCE(EXCLUDED.kev_due_date, cve_enrichment.kev_due_date),
		  kev_short_name   = COALESCE(EXCLUDED.kev_short_name, cve_enrichment.kev_short_name),
		  epss_score       = COALESCE(EXCLUDED.epss_score, cve_enrichment.epss_score),
		  epss_percentile  = COALESCE(EXCLUDED.epss_percentile, cve_enrichment.epss_percentile),
		  fetched_at       = NOW()`

	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(q,
			r.CVEID, r.KEVListed, r.KEVAddedDate, r.KEVDueDate, r.KEVShortName,
			r.EPSSScore, r.EPSSPercentile)
	}
	br := tx.SendBatch(ctx, batch)
	for range rows {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("exec batch: %w", err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("close batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// LoadAllCVEEnrichment returns the entire cve_enrichment table — called
// once at startup so the in-memory cache can answer Lookup() before the
// first refresh completes.
func (db *DB) LoadAllCVEEnrichment(ctx context.Context) ([]CVEEnrichmentRow, error) {
	rows, err := db.Pool.Query(ctx, `SELECT cve_id, kev_listed, kev_added_date, kev_due_date,
		kev_short_name, epss_score, epss_percentile FROM cve_enrichment`)
	if err != nil {
		return nil, fmt.Errorf("query cve_enrichment: %w", err)
	}
	defer rows.Close()

	var out []CVEEnrichmentRow
	for rows.Next() {
		var r CVEEnrichmentRow
		if err := rows.Scan(&r.CVEID, &r.KEVListed, &r.KEVAddedDate, &r.KEVDueDate,
			&r.KEVShortName, &r.EPSSScore, &r.EPSSPercentile); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
