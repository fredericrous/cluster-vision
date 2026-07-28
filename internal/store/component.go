package store

// it_components is READ-ONLY at runtime. The authoring API was removed when
// cluster-vision became the observed sensor, and infrastructure is no longer
// mapped into components (a node is context for a CI, not a peer CI), so
// nothing writes this table. The read paths stay: they are part of the
// published federation contract and answer "no components tracked" honestly.

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ITComponent struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Version     *string   `json:"version"`
	Provider    *string   `json:"provider"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (db *DB) ListComponents(ctx context.Context, componentType string) ([]ITComponent, error) {
	query := `SELECT id, name, type, version, provider, description, status, tags, created_at, updated_at
		FROM it_components`
	var args []any
	if componentType != "" {
		query += " WHERE type = $1"
		args = append(args, componentType)
	}
	query += " ORDER BY type, name"

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing components: %w", err)
	}
	defer rows.Close()

	var components []ITComponent
	for rows.Next() {
		var c ITComponent
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Version, &c.Provider, &c.Description,
			&c.Status, &c.Tags, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning component: %w", err)
		}
		components = append(components, c)
	}
	return components, nil
}

func (db *DB) GetComponent(ctx context.Context, id uuid.UUID) (*ITComponent, error) {
	var c ITComponent
	err := db.Pool.QueryRow(ctx, `SELECT id, name, type, version, provider, description, status, tags, created_at, updated_at
		FROM it_components WHERE id = $1`, id).Scan(
		&c.ID, &c.Name, &c.Type, &c.Version, &c.Provider, &c.Description,
		&c.Status, &c.Tags, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting component: %w", err)
	}
	return &c, nil
}
