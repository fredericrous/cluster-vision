package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type VersionHistoryEntry struct {
	ID            uuid.UUID `json:"id"`
	AppID         uuid.UUID `json:"app_id"`
	ChartVersion  *string   `json:"chart_version"`
	ImageTag      *string   `json:"image_tag"`
	LatestVersion *string   `json:"latest_version"`
	Outdated      bool      `json:"outdated"`
	VulnCritical  int       `json:"vuln_critical"`
	VulnHigh      int       `json:"vuln_high"`
	RecordedAt    time.Time `json:"recorded_at"`
}

// InsertVersionHistory records a version snapshot for an application.
// Only inserts if the latest entry differs (deduplication).
func (db *DB) InsertVersionHistory(ctx context.Context, e *VersionHistoryEntry) error {
	// Check if latest entry is identical
	var lastChart, lastTag *string
	err := db.Pool.QueryRow(ctx, `SELECT chart_version, image_tag FROM version_history
		WHERE app_id = $1 ORDER BY recorded_at DESC LIMIT 1`, e.AppID).Scan(&lastChart, &lastTag)
	if err == nil {
		if ptrEqual(lastChart, e.ChartVersion) && ptrEqual(lastTag, e.ImageTag) {
			return nil // no change
		}
	}

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.RecordedAt = time.Now()

	_, err = db.Pool.Exec(ctx, `INSERT INTO version_history
		(id, app_id, chart_version, image_tag, latest_version, outdated, vuln_critical, vuln_high, recorded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID, e.AppID, e.ChartVersion, e.ImageTag, e.LatestVersion, e.Outdated, e.VulnCritical, e.VulnHigh, e.RecordedAt)
	if err != nil {
		return fmt.Errorf("inserting version history: %w", err)
	}
	return nil
}

// GetVersionHistory returns version history for an app, optionally filtered by time range.
func (db *DB) GetVersionHistory(ctx context.Context, appID uuid.UUID, from, to *time.Time) ([]VersionHistoryEntry, error) {
	query := `SELECT id, app_id, chart_version, image_tag, latest_version, outdated, vuln_critical, vuln_high, recorded_at
		FROM version_history WHERE app_id = $1`
	args := []any{appID}
	idx := 2

	if from != nil {
		query += fmt.Sprintf(" AND recorded_at >= $%d", idx)
		args = append(args, *from)
		idx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND recorded_at <= $%d", idx)
		args = append(args, *to)
	}
	query += " ORDER BY recorded_at DESC"

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting version history: %w", err)
	}
	defer rows.Close()

	var entries []VersionHistoryEntry
	for rows.Next() {
		var e VersionHistoryEntry
		if err := rows.Scan(&e.ID, &e.AppID, &e.ChartVersion, &e.ImageTag, &e.LatestVersion,
			&e.Outdated, &e.VulnCritical, &e.VulnHigh, &e.RecordedAt); err != nil {
			return nil, fmt.Errorf("scanning version history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func ptrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// SyncLog

type SyncLog struct {
	ID                uuid.UUID  `json:"id"`
	AppsCreated       int        `json:"apps_created"`
	AppsUpdated       int        `json:"apps_updated"`
	ComponentsCreated int        `json:"components_created"`
	Errors            []string   `json:"errors"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
}

func (db *DB) CreateSyncLog(ctx context.Context) (*SyncLog, error) {
	sl := &SyncLog{
		ID:        uuid.New(),
		Errors:    []string{},
		StartedAt: time.Now(),
	}
	_, err := db.Pool.Exec(ctx, `INSERT INTO sync_logs (id, errors, started_at) VALUES ($1,$2,$3)`,
		sl.ID, sl.Errors, sl.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("creating sync log: %w", err)
	}
	return sl, nil
}

func (db *DB) FinishSyncLog(ctx context.Context, sl *SyncLog) error {
	now := time.Now()
	sl.FinishedAt = &now
	_, err := db.Pool.Exec(ctx, `UPDATE sync_logs SET apps_created=$2, apps_updated=$3,
		components_created=$4, errors=$5, finished_at=$6 WHERE id=$1`,
		sl.ID, sl.AppsCreated, sl.AppsUpdated, sl.ComponentsCreated, sl.Errors, sl.FinishedAt)
	if err != nil {
		return fmt.Errorf("finishing sync log: %w", err)
	}
	return nil
}

func (db *DB) ListSyncLogs(ctx context.Context, limit int) ([]SyncLog, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Pool.Query(ctx, `SELECT id, apps_created, apps_updated, components_created, errors, started_at, finished_at
		FROM sync_logs ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing sync logs: %w", err)
	}
	defer rows.Close()

	var logs []SyncLog
	for rows.Next() {
		var sl SyncLog
		if err := rows.Scan(&sl.ID, &sl.AppsCreated, &sl.AppsUpdated, &sl.ComponentsCreated, &sl.Errors, &sl.StartedAt, &sl.FinishedAt); err != nil {
			return nil, fmt.Errorf("scanning sync log: %w", err)
		}
		logs = append(logs, sl)
	}
	return logs, nil
}
