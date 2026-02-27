package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BusinessCapability struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	ParentID    *uuid.UUID `json:"parent_id"`
	Level       int        `json:"level"`
	SortOrder   int        `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CapabilityTreeNode struct {
	BusinessCapability
	Children []CapabilityTreeNode `json:"children"`
	AppCount int                  `json:"app_count"`
}

func (db *DB) ListCapabilities(ctx context.Context) ([]BusinessCapability, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id, name, description, parent_id, level, sort_order, created_at, updated_at
		FROM business_capabilities ORDER BY level, sort_order, name`)
	if err != nil {
		return nil, fmt.Errorf("listing capabilities: %w", err)
	}
	defer rows.Close()

	var caps []BusinessCapability
	for rows.Next() {
		var c BusinessCapability
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.ParentID, &c.Level, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning capability: %w", err)
		}
		caps = append(caps, c)
	}
	return caps, nil
}

func (db *DB) GetCapabilityTree(ctx context.Context) ([]CapabilityTreeNode, error) {
	rows, err := db.Pool.Query(ctx, `SELECT bc.id, bc.name, bc.description, bc.parent_id, bc.level, bc.sort_order,
		bc.created_at, bc.updated_at, COALESCE(ac.cnt, 0) as app_count
		FROM business_capabilities bc
		LEFT JOIN (SELECT capability_id, count(*) as cnt FROM app_capabilities GROUP BY capability_id) ac ON ac.capability_id = bc.id
		ORDER BY bc.level, bc.sort_order, bc.name`)
	if err != nil {
		return nil, fmt.Errorf("getting capability tree: %w", err)
	}
	defer rows.Close()

	var all []CapabilityTreeNode
	for rows.Next() {
		var n CapabilityTreeNode
		if err := rows.Scan(&n.ID, &n.Name, &n.Description, &n.ParentID, &n.Level, &n.SortOrder,
			&n.CreatedAt, &n.UpdatedAt, &n.AppCount); err != nil {
			return nil, fmt.Errorf("scanning capability tree node: %w", err)
		}
		n.Children = []CapabilityTreeNode{}
		all = append(all, n)
	}

	return buildTree(all), nil
}

func buildTree(nodes []CapabilityTreeNode) []CapabilityTreeNode {
	byID := make(map[uuid.UUID]*CapabilityTreeNode)

	for i := range nodes {
		byID[nodes[i].ID] = &nodes[i]
	}

	// First pass: assign children to parents via pointers
	for i := range nodes {
		if nodes[i].ParentID != nil {
			if parent, ok := byID[*nodes[i].ParentID]; ok {
				parent.Children = append(parent.Children, nodes[i])
			}
		}
	}

	// Second pass: collect roots (copies now include children)
	var roots []CapabilityTreeNode
	for i := range nodes {
		if nodes[i].ParentID == nil {
			roots = append(roots, nodes[i])
		} else if _, ok := byID[*nodes[i].ParentID]; !ok {
			roots = append(roots, nodes[i])
		}
	}

	return roots
}

func (db *DB) GetCapability(ctx context.Context, id uuid.UUID) (*BusinessCapability, error) {
	var c BusinessCapability
	err := db.Pool.QueryRow(ctx, `SELECT id, name, description, parent_id, level, sort_order, created_at, updated_at
		FROM business_capabilities WHERE id = $1`, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.ParentID, &c.Level, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting capability: %w", err)
	}
	return &c, nil
}

func (db *DB) CreateCapability(ctx context.Context, c *BusinessCapability) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	_, err := db.Pool.Exec(ctx, `INSERT INTO business_capabilities (id, name, description, parent_id, level, sort_order, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		c.ID, c.Name, c.Description, c.ParentID, c.Level, c.SortOrder, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating capability: %w", err)
	}
	return nil
}

func (db *DB) UpdateCapability(ctx context.Context, c *BusinessCapability) error {
	c.UpdatedAt = time.Now()
	_, err := db.Pool.Exec(ctx, `UPDATE business_capabilities SET
		name=$2, description=$3, parent_id=$4, level=$5, sort_order=$6, updated_at=$7
		WHERE id=$1`,
		c.ID, c.Name, c.Description, c.ParentID, c.Level, c.SortOrder, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("updating capability: %w", err)
	}
	return nil
}

func (db *DB) DeleteCapability(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM business_capabilities WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting capability: %w", err)
	}
	return nil
}

// GetCapabilityByName finds a capability by exact name match.
func (db *DB) GetCapabilityByName(ctx context.Context, name string) (*BusinessCapability, error) {
	var c BusinessCapability
	err := db.Pool.QueryRow(ctx, `SELECT id, name, description, parent_id, level, sort_order, created_at, updated_at
		FROM business_capabilities WHERE name = $1`, name).Scan(
		&c.ID, &c.Name, &c.Description, &c.ParentID, &c.Level, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting capability by name: %w", err)
	}
	return &c, nil
}
