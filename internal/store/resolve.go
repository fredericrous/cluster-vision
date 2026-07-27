package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ResolveMatch is one application matching a set of k8s coordinates.
type ResolveMatch struct {
	ApplicationID uuid.UUID `json:"application_id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Cluster       string    `json:"cluster"`
	Namespace     string    `json:"namespace"`
	HelmRelease   *string   `json:"helm_release"`
	WorkloadName  *string   `json:"workload_name"`
}

// ResolveCoordinates maps k8s coordinates to CI slugs. Every non-empty
// parameter must match; workload matches either the workload name or the
// helm release, since alert labels rarely distinguish the two. Results are
// deduplicated per application.
func (db *DB) ResolveCoordinates(ctx context.Context, cluster, namespace, helmRelease, workload string) ([]ResolveMatch, error) {
	conditions := []string{"1=1"}
	var args []any
	argIdx := 1

	add := func(cond string, val any) {
		conditions = append(conditions, fmt.Sprintf(cond, argIdx))
		args = append(args, val)
		argIdx++
	}

	if cluster != "" {
		add("k.cluster = $%d", cluster)
	}
	if namespace != "" {
		add("k.namespace = $%d", namespace)
	}
	if helmRelease != "" {
		add("k.helm_release = $%d", helmRelease)
	}
	if workload != "" {
		conditions = append(conditions, fmt.Sprintf("(k.workload_name = $%d OR k.helm_release = $%d)", argIdx, argIdx))
		args = append(args, workload)
		argIdx++
	}

	query := fmt.Sprintf(`SELECT DISTINCT ON (a.id) a.id, a.slug, a.name,
		k.cluster, k.namespace, k.helm_release, k.workload_name
		FROM k8s_sources k
		JOIN applications a ON a.id = k.app_id
		WHERE %s
		ORDER BY a.id, k.last_sync_at DESC`, strings.Join(conditions, " AND "))

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("resolving coordinates: %w", err)
	}
	defer rows.Close()

	var matches []ResolveMatch
	for rows.Next() {
		var m ResolveMatch
		if err := rows.Scan(&m.ApplicationID, &m.Slug, &m.Name,
			&m.Cluster, &m.Namespace, &m.HelmRelease, &m.WorkloadName); err != nil {
			return nil, fmt.Errorf("scanning resolve match: %w", err)
		}
		matches = append(matches, m)
	}
	return matches, nil
}
