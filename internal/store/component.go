package store

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

func (db *DB) CreateComponent(ctx context.Context, c *ITComponent) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	_, err := db.Pool.Exec(ctx, `INSERT INTO it_components (id, name, type, version, provider, description, status, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID, c.Name, c.Type, c.Version, c.Provider, c.Description, c.Status, c.Tags, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating component: %w", err)
	}
	return nil
}

func (db *DB) UpdateComponent(ctx context.Context, c *ITComponent) error {
	c.UpdatedAt = time.Now()
	_, err := db.Pool.Exec(ctx, `UPDATE it_components SET
		name=$2, type=$3, version=$4, provider=$5, description=$6, status=$7, tags=$8, updated_at=$9
		WHERE id=$1`,
		c.ID, c.Name, c.Type, c.Version, c.Provider, c.Description, c.Status, c.Tags, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("updating component: %w", err)
	}
	return nil
}

func (db *DB) DeleteComponent(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM it_components WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting component: %w", err)
	}
	return nil
}

// UpsertComponentByNameType creates or updates a component by name+type (for auto-discovery).
func (db *DB) UpsertComponentByNameType(ctx context.Context, name, cType string, update func(*ITComponent)) (*ITComponent, bool, error) {
	var c ITComponent
	err := db.Pool.QueryRow(ctx, `SELECT id, name, type, version, provider, description, status, tags, created_at, updated_at
		FROM it_components WHERE name = $1 AND type = $2`, name, cType).Scan(
		&c.ID, &c.Name, &c.Type, &c.Version, &c.Provider, &c.Description,
		&c.Status, &c.Tags, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		c = ITComponent{
			Name:   name,
			Type:   cType,
			Status: "active",
			Tags:   []string{},
		}
		update(&c)
		if err := db.CreateComponent(ctx, &c); err != nil {
			return nil, false, err
		}
		return &c, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("looking up component: %w", err)
	}

	update(&c)
	if err := db.UpdateComponent(ctx, &c); err != nil {
		return nil, false, err
	}
	return &c, false, nil
}
