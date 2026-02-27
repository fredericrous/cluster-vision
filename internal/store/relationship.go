package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AppDependency struct {
	SourceAppID uuid.UUID `json:"source_app_id"`
	TargetAppID uuid.UUID `json:"target_app_id"`
	Description *string   `json:"description"`
}

type K8sSource struct {
	ID             uuid.UUID `json:"id"`
	AppID          uuid.UUID `json:"app_id"`
	Cluster        string    `json:"cluster"`
	Namespace      string    `json:"namespace"`
	HelmRelease    *string   `json:"helm_release"`
	WorkloadName   *string   `json:"workload_name"`
	WorkloadKind   *string   `json:"workload_kind"`
	ChartName      *string   `json:"chart_name"`
	ChartVersion   *string   `json:"chart_version"`
	Images         []string  `json:"images"`
	LastSyncAt     time.Time `json:"last_sync_at"`
	ManualOverride bool      `json:"manual_override"`
}

// Dependencies

func (db *DB) ListDependencies(ctx context.Context, appID uuid.UUID) ([]AppDependency, error) {
	rows, err := db.Pool.Query(ctx, `SELECT source_app_id, target_app_id, description
		FROM app_dependencies WHERE source_app_id = $1 OR target_app_id = $1`, appID)
	if err != nil {
		return nil, fmt.Errorf("listing dependencies: %w", err)
	}
	defer rows.Close()

	var deps []AppDependency
	for rows.Next() {
		var d AppDependency
		if err := rows.Scan(&d.SourceAppID, &d.TargetAppID, &d.Description); err != nil {
			return nil, fmt.Errorf("scanning dependency: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, nil
}

func (db *DB) AddDependency(ctx context.Context, d *AppDependency) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO app_dependencies (source_app_id, target_app_id, description)
		VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, d.SourceAppID, d.TargetAppID, d.Description)
	if err != nil {
		return fmt.Errorf("adding dependency: %w", err)
	}
	return nil
}

func (db *DB) RemoveDependency(ctx context.Context, sourceID, targetID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM app_dependencies WHERE source_app_id = $1 AND target_app_id = $2", sourceID, targetID)
	if err != nil {
		return fmt.Errorf("removing dependency: %w", err)
	}
	return nil
}

// App-Component links

func (db *DB) ListAppComponents(ctx context.Context, appID uuid.UUID) ([]ITComponent, error) {
	rows, err := db.Pool.Query(ctx, `SELECT c.id, c.name, c.type, c.version, c.provider, c.description, c.status, c.tags, c.created_at, c.updated_at
		FROM it_components c JOIN app_components ac ON ac.component_id = c.id WHERE ac.app_id = $1
		ORDER BY c.type, c.name`, appID)
	if err != nil {
		return nil, fmt.Errorf("listing app components: %w", err)
	}
	defer rows.Close()

	var components []ITComponent
	for rows.Next() {
		var c ITComponent
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Version, &c.Provider, &c.Description,
			&c.Status, &c.Tags, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning app component: %w", err)
		}
		components = append(components, c)
	}
	return components, nil
}

func (db *DB) LinkAppComponent(ctx context.Context, appID, componentID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO app_components (app_id, component_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, appID, componentID)
	if err != nil {
		return fmt.Errorf("linking app component: %w", err)
	}
	return nil
}

func (db *DB) UnlinkAppComponent(ctx context.Context, appID, componentID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM app_components WHERE app_id = $1 AND component_id = $2", appID, componentID)
	if err != nil {
		return fmt.Errorf("unlinking app component: %w", err)
	}
	return nil
}

// App-Capability links

func (db *DB) ListAppCapabilities(ctx context.Context, appID uuid.UUID) ([]BusinessCapability, error) {
	rows, err := db.Pool.Query(ctx, `SELECT bc.id, bc.name, bc.description, bc.parent_id, bc.level, bc.sort_order, bc.created_at, bc.updated_at
		FROM business_capabilities bc JOIN app_capabilities ac ON ac.capability_id = bc.id WHERE ac.app_id = $1
		ORDER BY bc.level, bc.sort_order, bc.name`, appID)
	if err != nil {
		return nil, fmt.Errorf("listing app capabilities: %w", err)
	}
	defer rows.Close()

	var caps []BusinessCapability
	for rows.Next() {
		var c BusinessCapability
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.ParentID, &c.Level, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning app capability: %w", err)
		}
		caps = append(caps, c)
	}
	return caps, nil
}

func (db *DB) LinkAppCapability(ctx context.Context, appID, capabilityID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO app_capabilities (app_id, capability_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, appID, capabilityID)
	if err != nil {
		return fmt.Errorf("linking app capability: %w", err)
	}
	return nil
}

func (db *DB) UnlinkAppCapability(ctx context.Context, appID, capabilityID uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM app_capabilities WHERE app_id = $1 AND capability_id = $2", appID, capabilityID)
	if err != nil {
		return fmt.Errorf("unlinking app capability: %w", err)
	}
	return nil
}

// K8s Sources

func (db *DB) ListK8sSources(ctx context.Context, appID uuid.UUID) ([]K8sSource, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id, app_id, cluster, namespace, helm_release, workload_name, workload_kind,
		chart_name, chart_version, images, last_sync_at, manual_override
		FROM k8s_sources WHERE app_id = $1 ORDER BY cluster, namespace`, appID)
	if err != nil {
		return nil, fmt.Errorf("listing k8s sources: %w", err)
	}
	defer rows.Close()

	var sources []K8sSource
	for rows.Next() {
		var s K8sSource
		if err := rows.Scan(&s.ID, &s.AppID, &s.Cluster, &s.Namespace, &s.HelmRelease, &s.WorkloadName, &s.WorkloadKind,
			&s.ChartName, &s.ChartVersion, &s.Images, &s.LastSyncAt, &s.ManualOverride); err != nil {
			return nil, fmt.Errorf("scanning k8s source: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (db *DB) UpsertK8sSource(ctx context.Context, s *K8sSource) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.LastSyncAt = time.Now()

	_, err := db.Pool.Exec(ctx, `INSERT INTO k8s_sources (id, app_id, cluster, namespace, helm_release, workload_name, workload_kind,
		chart_name, chart_version, images, last_sync_at, manual_override)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
		helm_release=EXCLUDED.helm_release, workload_name=EXCLUDED.workload_name, workload_kind=EXCLUDED.workload_kind,
		chart_name=EXCLUDED.chart_name, chart_version=EXCLUDED.chart_version, images=EXCLUDED.images,
		last_sync_at=EXCLUDED.last_sync_at`,
		s.ID, s.AppID, s.Cluster, s.Namespace, s.HelmRelease, s.WorkloadName, s.WorkloadKind,
		s.ChartName, s.ChartVersion, s.Images, s.LastSyncAt, s.ManualOverride)
	if err != nil {
		return fmt.Errorf("upserting k8s source: %w", err)
	}
	return nil
}

// FindK8sSource looks up existing k8s_source by app_id + cluster + namespace + helm_release.
func (db *DB) FindK8sSource(ctx context.Context, appID uuid.UUID, cluster, namespace string, helmRelease *string) (*K8sSource, error) {
	query := `SELECT id, app_id, cluster, namespace, helm_release, workload_name, workload_kind,
		chart_name, chart_version, images, last_sync_at, manual_override
		FROM k8s_sources WHERE app_id = $1 AND cluster = $2 AND namespace = $3`
	args := []any{appID, cluster, namespace}

	if helmRelease != nil {
		query += " AND helm_release = $4"
		args = append(args, *helmRelease)
	} else {
		query += " AND helm_release IS NULL"
	}

	var s K8sSource
	err := db.Pool.QueryRow(ctx, query, args...).Scan(&s.ID, &s.AppID, &s.Cluster, &s.Namespace,
		&s.HelmRelease, &s.WorkloadName, &s.WorkloadKind,
		&s.ChartName, &s.ChartVersion, &s.Images, &s.LastSyncAt, &s.ManualOverride)
	if err != nil {
		return nil, nil // not found
	}
	return &s, nil
}

// AllDependencies returns all dependencies for the graph view.
func (db *DB) AllDependencies(ctx context.Context) ([]AppDependency, error) {
	rows, err := db.Pool.Query(ctx, "SELECT source_app_id, target_app_id, description FROM app_dependencies")
	if err != nil {
		return nil, fmt.Errorf("listing all dependencies: %w", err)
	}
	defer rows.Close()

	var deps []AppDependency
	for rows.Next() {
		var d AppDependency
		if err := rows.Scan(&d.SourceAppID, &d.TargetAppID, &d.Description); err != nil {
			return nil, fmt.Errorf("scanning dependency: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, nil
}
