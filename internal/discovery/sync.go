package discovery

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fredericrous/cluster-vision/internal/model"
	"github.com/fredericrous/cluster-vision/internal/store"
)

// Syncer orchestrates auto-discovery from ClusterData into the EAM store.
type Syncer struct {
	db *store.DB
}

// NewSyncer creates a new discovery syncer.
func NewSyncer(db *store.DB) *Syncer {
	return &Syncer{db: db}
}

// SyncResult holds counts from a sync operation.
type SyncResult struct {
	AppsCreated int
	AppsUpdated int
	Errors      []string
}

// Sync maps ClusterData to EAM entities and persists them.
func (s *Syncer) Sync(ctx context.Context, data *model.ClusterData) *SyncResult {
	result := &SyncResult{}

	syncLog, err := s.db.CreateSyncLog(ctx)
	if err != nil {
		slog.Error("failed to create sync log", "error", err)
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	apps := MapClusterData(data)

	// Sync applications
	for _, da := range apps {
		app, created, err := s.db.UpsertApplicationByName(ctx, da.Name, func(a *store.Application) {
			// Only update auto-discovered fields
			if a.Tags == nil {
				a.Tags = []string{}
			}
		})
		if err != nil {
			slog.Error("failed to upsert application", "name", da.Name, "error", err)
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		if created {
			result.AppsCreated++
		} else {
			result.AppsUpdated++
		}

		// Upsert K8s source. A failed lookup is not "no row": inserting
		// anyway would add a duplicate next to the row the lookup missed.
		manual, err := s.syncK8sSource(ctx, app, da)
		if err != nil {
			slog.Error("failed to sync k8s source", "app", da.Name, "error", err)
			result.Errors = append(result.Errors, err.Error())
		}
		if manual {
			// A hand-maintained source: leave its history alone too.
			continue
		}

		// Record version history
		entry := &store.VersionHistoryEntry{
			AppID:        app.ID,
			ChartVersion: da.ChartVersion,
			ImageTag:     PrimaryImageTag(da.Images),
			VulnCritical: da.VulnCritical,
			VulnHigh:     da.VulnHigh,
		}
		if da.VulnCritical > 0 || da.VulnHigh > 0 {
			entry.Outdated = true
		}
		if err := s.db.InsertVersionHistory(ctx, entry); err != nil {
			slog.Error("failed to insert version history", "app", da.Name, "error", err)
		}
	}

	// Finish sync log
	syncLog.AppsCreated = result.AppsCreated
	syncLog.AppsUpdated = result.AppsUpdated
	syncLog.Errors = result.Errors
	if err := s.db.FinishSyncLog(ctx, syncLog); err != nil {
		slog.Error("failed to finish sync log", "error", err)
	}

	slog.Info("EAM sync complete",
		"apps_created", result.AppsCreated,
		"apps_updated", result.AppsUpdated,
		"errors", len(result.Errors))

	return result
}

// syncK8sSource records where an application runs. It never writes when the
// lookup of the existing row failed, and leaves manual overrides alone
// (reported through manual=true).
func (s *Syncer) syncK8sSource(ctx context.Context, app *store.Application, da DiscoveredApp) (manual bool, err error) {
	existing, err := s.db.FindK8sSource(ctx, app.ID, da.Cluster, da.Namespace, da.HelmRelease)
	if err != nil {
		return false, fmt.Errorf("k8s source for %s: %w", da.Name, err)
	}
	if existing == nil && da.LegacyNamespace != "" {
		// Recorded under the HelmRelease object's namespace before;
		// reuse that row so the upsert moves it instead of leaving a
		// stale duplicate behind.
		existing, err = s.db.FindK8sSource(ctx, app.ID, da.Cluster, da.LegacyNamespace, da.HelmRelease)
		if err != nil {
			return false, fmt.Errorf("k8s source for %s (legacy namespace): %w", da.Name, err)
		}
	}
	src := BuildK8sSource(*app, da)
	if existing != nil {
		if existing.ManualOverride {
			return true, nil
		}
		src.ID = existing.ID
	}
	if err := s.db.UpsertK8sSource(ctx, src); err != nil {
		return false, fmt.Errorf("k8s source for %s: %w", da.Name, err)
	}
	return false, nil
}
