package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fredericrous/cluster-vision/internal/diff"
	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ModelVersion is stamped on every snapshot. Bump it when model.ClusterData
// changes shape in a way old snapshots cannot be read through (renamed or
// re-typed fields). Additive changes are fine: missing fields decode to
// their zero value.
const ModelVersion = 1

// ErrNoSnapshot is returned when a selector matches nothing.
var ErrNoSnapshot = errors.New("no such snapshot")

// Snapshot is the metadata of one persisted cluster state. The data itself
// is loaded separately with GetSnapshotData.
type Snapshot struct {
	ID           uuid.UUID       `json:"id"`
	TakenAt      time.Time       `json:"taken_at"`
	ObservedHash []byte          `json:"-"`
	ModelVersion int             `json:"model_version"`
	Revisions    []diff.Revision `json:"revisions"`
	Summary      SnapshotSummary `json:"summary"`
}

// SnapshotSummary is the change count of a snapshot against the snapshot
// before it, computed once at insert time.
type SnapshotSummary struct {
	PreviousID *uuid.UUID              `json:"previous_id"`
	Total      diff.Summary            `json:"total"`
	Diagrams   map[string]diff.Summary `json:"diagrams"`
	Drift      bool                    `json:"drift"`
}

const snapshotCols = `id, taken_at, observed_hash, model_version, revisions, summary`

func scanSnapshot(row pgx.Row) (*Snapshot, error) {
	var s Snapshot
	var revs, summary []byte
	if err := row.Scan(&s.ID, &s.TakenAt, &s.ObservedHash, &s.ModelVersion, &revs, &summary); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoSnapshot
		}
		return nil, err
	}
	if err := json.Unmarshal(revs, &s.Revisions); err != nil {
		return nil, fmt.Errorf("decoding revisions: %w", err)
	}
	if err := json.Unmarshal(summary, &s.Summary); err != nil {
		return nil, fmt.Errorf("decoding summary: %w", err)
	}
	if s.Revisions == nil {
		s.Revisions = []diff.Revision{}
	}
	return &s, nil
}

// LatestSnapshot returns the most recent snapshot, or ErrNoSnapshot.
func (db *DB) LatestSnapshot(ctx context.Context) (*Snapshot, error) {
	return scanSnapshot(db.Pool.QueryRow(ctx,
		`SELECT `+snapshotCols+` FROM snapshots ORDER BY taken_at DESC LIMIT 1`))
}

// GetSnapshot returns one snapshot by id.
func (db *DB) GetSnapshot(ctx context.Context, id uuid.UUID) (*Snapshot, error) {
	return scanSnapshot(db.Pool.QueryRow(ctx,
		`SELECT `+snapshotCols+` FROM snapshots WHERE id = $1`, id))
}

// SnapshotAt returns the latest snapshot taken at or before t.
func (db *DB) SnapshotAt(ctx context.Context, t time.Time) (*Snapshot, error) {
	return scanSnapshot(db.Pool.QueryRow(ctx,
		`SELECT `+snapshotCols+` FROM snapshots WHERE taken_at <= $1 ORDER BY taken_at DESC LIMIT 1`, t))
}

// SnapshotBefore returns the latest snapshot strictly older than t.
func (db *DB) SnapshotBefore(ctx context.Context, t time.Time) (*Snapshot, error) {
	return scanSnapshot(db.Pool.QueryRow(ctx,
		`SELECT `+snapshotCols+` FROM snapshots WHERE taken_at < $1 ORDER BY taken_at DESC LIMIT 1`, t))
}

// SnapshotBySHA returns the first (earliest) snapshot observed at a
// revision whose digest starts with prefix. Prefixes shorter than 4
// characters are rejected to avoid accidental matches.
func (db *DB) SnapshotBySHA(ctx context.Context, prefix string) (*Snapshot, error) {
	if len(prefix) < 4 {
		return nil, ErrNoSnapshot
	}
	return scanSnapshot(db.Pool.QueryRow(ctx,
		`SELECT `+snapshotCols+` FROM snapshots s
		 WHERE EXISTS (SELECT 1 FROM snapshot_revisions r WHERE r.snapshot_id = s.id AND r.sha LIKE $1 || '%')
		 ORDER BY taken_at ASC LIMIT 1`, prefix))
}

// SnapshotBeforeRevisionChange returns the latest snapshot older than s
// whose revision set differs from s's — i.e. the last state observed
// before the current deploy. ErrNoSnapshot when s is the first revision
// on record.
func (db *DB) SnapshotBeforeRevisionChange(ctx context.Context, s *Snapshot) (*Snapshot, error) {
	revs, err := json.Marshal(s.Revisions)
	if err != nil {
		return nil, err
	}
	return scanSnapshot(db.Pool.QueryRow(ctx,
		`SELECT `+snapshotCols+` FROM snapshots
		 WHERE taken_at < $1 AND revisions <> $2::jsonb ORDER BY taken_at DESC LIMIT 1`, s.TakenAt, revs))
}

// ListSnapshots returns snapshots newest first within the optional time
// window.
func (db *DB) ListSnapshots(ctx context.Context, from, to *time.Time, limit int) ([]Snapshot, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + snapshotCols + ` FROM snapshots WHERE true`
	args := []any{}
	if from != nil {
		args = append(args, *from)
		q += fmt.Sprintf(" AND taken_at >= $%d", len(args))
	}
	if to != nil {
		args = append(args, *to)
		q += fmt.Sprintf(" AND taken_at <= $%d", len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY taken_at DESC LIMIT $%d", len(args))

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	defer rows.Close()

	out := []Snapshot{}
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning snapshot: %w", err)
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// GetSnapshotData loads the cluster model of a snapshot.
func (db *DB) GetSnapshotData(ctx context.Context, id uuid.UUID) (*model.ClusterData, error) {
	var raw []byte
	err := db.Pool.QueryRow(ctx, `SELECT data FROM snapshots WHERE id = $1`, id).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoSnapshot
		}
		return nil, fmt.Errorf("loading snapshot data: %w", err)
	}
	var data model.ClusterData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding snapshot data: %w", err)
	}
	return &data, nil
}

// InsertSnapshot persists a snapshot and its revision rows in one
// transaction. Dedup against the latest hash is the caller's job (it needs
// the diagrams anyway to compute the summary).
func (db *DB) InsertSnapshot(ctx context.Context, s *Snapshot, data *model.ClusterData) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.TakenAt.IsZero() {
		s.TakenAt = time.Now()
	}
	s.ModelVersion = ModelVersion
	if s.Revisions == nil {
		s.Revisions = []diff.Revision{}
	}
	if s.Summary.Diagrams == nil {
		s.Summary.Diagrams = map[string]diff.Summary{}
	}

	revs, err := json.Marshal(s.Revisions)
	if err != nil {
		return err
	}
	summary, err := json.Marshal(s.Summary)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encoding cluster data: %w", err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Single writer: the chart runs one replica, but if it ever scales,
	// serialize snapshot writers so two pods can't both pass the
	// "hash differs from latest" check.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('cluster_vision_snapshots'))`); err != nil {
		return fmt.Errorf("taking snapshot lock: %w", err)
	}

	_, err = tx.Exec(ctx, `INSERT INTO snapshots (id, taken_at, observed_hash, model_version, revisions, summary, data)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, s.TakenAt, s.ObservedHash, s.ModelVersion, revs, summary, raw)
	if err != nil {
		return fmt.Errorf("inserting snapshot: %w", err)
	}
	for _, r := range s.Revisions {
		_, err = tx.Exec(ctx, `INSERT INTO snapshot_revisions (snapshot_id, cluster, kustomization, source_kind, revision, sha)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			s.ID, r.Cluster, r.Kustomization, r.SourceKind, r.Revision, r.SHA)
		if err != nil {
			return fmt.Errorf("inserting snapshot revision: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// PruneSnapshots applies the retention policy: keep everything younger
// than full; between full and daily keep one per day; older than daily
// keep nothing — except the first snapshot observed at each revision set,
// which is always kept. Returns the number of rows deleted.
func (db *DB) PruneSnapshots(ctx context.Context, full, daily time.Duration) (int64, error) {
	now := time.Now()
	tag, err := db.Pool.Exec(ctx, `
		WITH first_at_revision AS (
			SELECT DISTINCT ON (revisions) id FROM snapshots ORDER BY revisions, taken_at ASC
		),
		first_of_day AS (
			SELECT DISTINCT ON (date_trunc('day', taken_at)) id FROM snapshots ORDER BY date_trunc('day', taken_at), taken_at ASC
		)
		DELETE FROM snapshots s
		WHERE s.taken_at < $1
		  AND s.id NOT IN (SELECT id FROM first_at_revision)
		  AND (s.taken_at < $2 OR s.id NOT IN (SELECT id FROM first_of_day))`,
		now.Add(-full), now.Add(-daily))
	if err != nil {
		return 0, fmt.Errorf("pruning snapshots: %w", err)
	}
	return tag.RowsAffected(), nil
}
