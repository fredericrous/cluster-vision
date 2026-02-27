package discovery

import (
	"context"
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
	AppsCreated       int
	AppsUpdated       int
	ComponentsCreated int
	Errors            []string
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

	apps, components := MapClusterData(data)

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

		// Upsert K8s source
		existingSource, _ := s.db.FindK8sSource(ctx, app.ID, da.Cluster, da.Namespace, da.HelmRelease)
		src := BuildK8sSource(*app, da)
		if existingSource != nil {
			if existingSource.ManualOverride {
				continue
			}
			src.ID = existingSource.ID
		}
		if err := s.db.UpsertK8sSource(ctx, src); err != nil {
			slog.Error("failed to upsert k8s source", "app", da.Name, "error", err)
			result.Errors = append(result.Errors, err.Error())
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

	// Sync IT components
	for _, dc := range components {
		_, created, err := s.db.UpsertComponentByNameType(ctx, dc.Name, dc.Type, func(c *store.ITComponent) {
			c.Version = dc.Version
			c.Provider = dc.Provider
		})
		if err != nil {
			slog.Error("failed to upsert component", "name", dc.Name, "error", err)
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		if created {
			result.ComponentsCreated++
		}
	}

	// Finish sync log
	syncLog.AppsCreated = result.AppsCreated
	syncLog.AppsUpdated = result.AppsUpdated
	syncLog.ComponentsCreated = result.ComponentsCreated
	syncLog.Errors = result.Errors
	if err := s.db.FinishSyncLog(ctx, syncLog); err != nil {
		slog.Error("failed to finish sync log", "error", err)
	}

	slog.Info("EAM sync complete",
		"apps_created", result.AppsCreated,
		"apps_updated", result.AppsUpdated,
		"components_created", result.ComponentsCreated,
		"errors", len(result.Errors))

	return result
}
