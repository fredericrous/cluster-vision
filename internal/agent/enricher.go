package agent

import (
	"context"
	"log/slog"
	"math"
	"sync"

	"github.com/fredericrous/cluster-vision/internal/store"
	"github.com/google/uuid"
)

// Enricher orchestrates AI-powered enrichment of EAM entities.
type Enricher struct {
	client *Client
	db     *store.DB
}

// NewEnricher creates a new AI enrichment orchestrator.
func NewEnricher(client *Client, db *store.DB) *Enricher {
	return &Enricher{client: client, db: db}
}

// EnrichAll runs the full AI enrichment pipeline for all non-overridden applications.
// It runs capability inference (batch), per-app enrichment (parallel), and dependency inference (batch).
func (e *Enricher) EnrichAll(ctx context.Context) error {
	apps, _, err := e.db.ListApplications(ctx, store.ApplicationFilter{Limit: store.MaxApplicationsLimit})
	if err != nil {
		return err
	}

	if len(apps) == 0 {
		slog.Info("ai enricher: no applications to enrich")
		return nil
	}

	// Build app contexts with K8s metadata
	appContexts := e.buildAppContexts(ctx, apps)

	// Step 1: Capability inference (batch — one prompt for all apps)
	if err := e.inferCapabilities(ctx, appContexts); err != nil {
		slog.Error("ai enricher: capability inference failed", "error", err)
	}

	// Step 2: Per-app enrichment (parallel)
	e.enrichApps(ctx, apps, appContexts)

	// Step 3: Dependency inference (batch)
	if err := e.inferDependencies(ctx, appContexts); err != nil {
		slog.Error("ai enricher: dependency inference failed", "error", err)
	}

	slog.Info("ai enricher: enrichment complete", "apps", len(apps))
	return nil
}

// EnrichNew runs AI enrichment only for apps that haven't been enriched yet (ai_confidence = 0).
func (e *Enricher) EnrichNew(ctx context.Context) error {
	apps, _, err := e.db.ListApplications(ctx, store.ApplicationFilter{Limit: store.MaxApplicationsLimit})
	if err != nil {
		return err
	}

	// Filter to only unenriched, non-overridden apps
	var newApps []store.Application
	for _, a := range apps {
		if a.AIConfidence == 0 && !a.ManualOverride {
			newApps = append(newApps, a)
		}
	}

	if len(newApps) == 0 {
		slog.Info("ai enricher: no new applications to enrich")
		return nil
	}

	appContexts := e.buildAppContexts(ctx, newApps)

	// Run capability inference for all apps (needs full list for context)
	allApps, _, _ := e.db.ListApplications(ctx, store.ApplicationFilter{Limit: store.MaxApplicationsLimit})
	allContexts := e.buildAppContexts(ctx, allApps)
	if err := e.inferCapabilities(ctx, allContexts); err != nil {
		slog.Error("ai enricher: capability inference failed", "error", err)
	}

	// Enrich only new apps
	e.enrichApps(ctx, newApps, appContexts)

	// Re-run dependency inference with all apps
	if err := e.inferDependencies(ctx, allContexts); err != nil {
		slog.Error("ai enricher: dependency inference failed", "error", err)
	}

	slog.Info("ai enricher: new app enrichment complete", "apps", len(newApps))
	return nil
}

func (e *Enricher) buildAppContexts(ctx context.Context, apps []store.Application) []AppContext {
	contexts := make([]AppContext, 0, len(apps))
	for _, app := range apps {
		ac := AppContext{
			Name: app.Name,
		}

		// Get K8s source data
		sources, err := e.db.ListK8sSources(ctx, app.ID)
		if err == nil && len(sources) > 0 {
			src := sources[0] // use primary source
			ac.Namespace = src.Namespace
			ac.Cluster = src.Cluster
			if src.ChartName != nil {
				ac.ChartName = *src.ChartName
			}
			if src.ChartVersion != nil {
				ac.ChartVersion = *src.ChartVersion
			}
			ac.Images = src.Images
		}

		// Get latest vuln data from version history
		history, err := e.db.GetVersionHistory(ctx, app.ID, nil, nil)
		if err == nil && len(history) > 0 {
			ac.VulnCritical = history[0].VulnCritical
			ac.VulnHigh = history[0].VulnHigh
		}

		contexts = append(contexts, ac)
	}
	return contexts
}

func (e *Enricher) inferCapabilities(ctx context.Context, appContexts []AppContext) error {
	slog.Info("ai enricher: inferring capabilities", "apps", len(appContexts))

	var result CapabilityInference
	userPrompt := BuildCapabilityPrompt(appContexts)
	if err := e.client.CompleteJSON(ctx, capabilitySystemPrompt, userPrompt, &result); err != nil {
		return err
	}

	// Create capabilities in DB
	capMap := make(map[string]uuid.UUID) // name → ID
	for _, cap := range result.Capabilities {
		existing, _ := e.db.GetCapabilityByName(ctx, cap.Name)
		var parentID uuid.UUID
		if existing != nil {
			parentID = existing.ID
		} else {
			bc := &store.BusinessCapability{
				Name:  cap.Name,
				Level: 1,
			}
			if cap.Description != "" {
				bc.Description = &cap.Description
			}
			if err := e.db.CreateCapability(ctx, bc); err != nil {
				slog.Error("ai enricher: failed to create L1 capability", "name", cap.Name, "error", err)
				continue
			}
			parentID = bc.ID
		}
		capMap[cap.Name] = parentID

		// Create L2 children
		for i, child := range cap.Children {
			existing, _ := e.db.GetCapabilityByName(ctx, child.Name)
			if existing != nil {
				capMap[child.Name] = existing.ID
				continue
			}
			bc := &store.BusinessCapability{
				Name:      child.Name,
				ParentID:  &parentID,
				Level:     2,
				SortOrder: i,
			}
			if child.Description != "" {
				bc.Description = &child.Description
			}
			if err := e.db.CreateCapability(ctx, bc); err != nil {
				slog.Error("ai enricher: failed to create L2 capability", "name", child.Name, "error", err)
				continue
			}
			capMap[child.Name] = bc.ID
		}
	}

	// Apply mappings
	for _, m := range result.Mappings {
		capID, ok := capMap[m.CapabilityName]
		if !ok {
			slog.Warn("ai enricher: capability not found for mapping", "capability", m.CapabilityName, "app", m.AppName)
			continue
		}

		app, err := e.db.GetApplicationByName(ctx, m.AppName)
		if err != nil || app == nil {
			continue
		}

		if err := e.db.LinkAppCapability(ctx, app.ID, capID); err != nil {
			slog.Error("ai enricher: failed to link app capability", "app", m.AppName, "capability", m.CapabilityName, "error", err)
		}
	}

	slog.Info("ai enricher: capabilities created", "capabilities", len(capMap), "mappings", len(result.Mappings))
	return nil
}

func (e *Enricher) enrichApps(ctx context.Context, apps []store.Application, appContexts []AppContext) {
	slog.Info("ai enricher: enriching apps", "count", len(apps))

	// Build name → context map
	contextMap := make(map[string]AppContext)
	for _, ac := range appContexts {
		contextMap[ac.Name] = ac
	}

	// Enrich in parallel with bounded concurrency
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for i := range apps {
		if apps[i].ManualOverride {
			continue
		}

		ac, ok := contextMap[apps[i].Name]
		if !ok {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(app store.Application, ac AppContext) {
			defer wg.Done()
			defer func() { <-sem }()

			var enrichment AppEnrichment
			userPrompt := BuildEnrichmentPrompt(ac)
			if err := e.client.CompleteJSON(ctx, enrichmentSystemPrompt, userPrompt, &enrichment); err != nil {
				slog.Error("ai enricher: failed to enrich app", "app", app.Name, "error", err)
				return
			}

			// Apply enrichment to application
			desc := enrichment.Description
			app.Description = &desc
			app.DescriptionSource = "ai-inferred"
			app.BusinessCriticality = normalizeEnum(enrichment.BusinessCriticality, "medium")
			app.BusinessCriticalitySource = "ai-inferred"
			app.TechnicalRisk = normalizeEnum(enrichment.TechnicalRisk, "medium")
			app.TechnicalRiskSource = "ai-inferred"
			if enrichment.RiskReason != "" {
				app.TechnicalRiskReasoning = &enrichment.RiskReason
			}
			tc := normalizeTimeCategory(enrichment.TimeCategory)
			if tc != "" {
				app.TimeCategory = &tc
				app.TimeCategorySource = "ai-inferred"
				if enrichment.TimeCategoryReason != "" {
					app.TimeCategoryReasoning = &enrichment.TimeCategoryReason
				}
			}
			app.AIConfidence = normalizeConfidence(enrichment.Confidence)

			if err := e.db.UpdateApplication(ctx, &app); err != nil {
				slog.Error("ai enricher: failed to save enrichment", "app", app.Name, "error", err)
			}
		}(apps[i], ac)
	}

	wg.Wait()
}

func (e *Enricher) inferDependencies(ctx context.Context, appContexts []AppContext) error {
	slog.Info("ai enricher: inferring dependencies", "apps", len(appContexts))

	var result DependencyInference
	userPrompt := BuildDependencyPrompt(appContexts)
	if err := e.client.CompleteJSON(ctx, dependencySystemPrompt, userPrompt, &result); err != nil {
		return err
	}

	apps, _, err := e.db.ListApplications(ctx, store.ApplicationFilter{Limit: store.MaxApplicationsLimit})
	if err != nil {
		return err
	}
	ids := make(map[string]uuid.UUID, len(apps))
	for _, a := range apps {
		ids[a.Name] = a.ID
	}

	// An empty answer for a non-trivial landscape is far more likely a bad
	// completion than a real "nothing depends on anything"; pruning on it
	// would wipe every inferred edge.
	if len(result.Dependencies) == 0 && len(appContexts) > 1 {
		slog.Warn("ai enricher: model inferred no dependencies — keeping the existing ones")
		return nil
	}

	plan, dropped := planDependencies(result.Dependencies, ids)

	// Replace the inferred edges of every app that was part of the prompt,
	// including the ones the model now says depend on nothing: that is how
	// an edge the model stopped inferring goes away.
	created := 0
	for _, ac := range appContexts {
		srcID, ok := ids[ac.Name]
		if !ok {
			continue
		}
		deps := plan[srcID]
		if err := e.db.ReplaceAIDependencies(ctx, srcID, deps); err != nil {
			slog.Error("ai enricher: failed to store dependencies", "source", ac.Name, "error", err)
			continue
		}
		created += len(deps)
	}

	slog.Info("ai enricher: dependencies inferred", "stored", created, "dropped", dropped, "total_inferred", len(result.Dependencies))
	return nil
}

// planDependencies turns the model's answer into edges per source app. It
// drops edges naming an unknown application, self-loops (an app does not
// depend on itself; the schema rejects them too) and duplicates. dropped
// counts what was discarded.
func planDependencies(inferred []InferredDependency, ids map[string]uuid.UUID) (plan map[uuid.UUID][]store.AppDependency, dropped int) {
	plan = make(map[uuid.UUID][]store.AppDependency)
	seen := make(map[[2]uuid.UUID]bool)
	for _, dep := range inferred {
		src, ok1 := ids[dep.Source]
		dst, ok2 := ids[dep.Target]
		if !ok1 || !ok2 || src == dst || seen[[2]uuid.UUID{src, dst}] {
			dropped++
			continue
		}
		seen[[2]uuid.UUID{src, dst}] = true
		reason := dep.Reason
		plan[src] = append(plan[src], store.AppDependency{SourceAppID: src, TargetAppID: dst, Description: &reason})
	}
	return plan, dropped
}

// minEnrichedConfidence is the floor stored for a successful enrichment.
// EnrichNew treats ai_confidence = 0 as "never enriched", so an app whose
// enrichment is stored with 0 would be re-sent to the model on every sync
// that creates an app, forever.
const minEnrichedConfidence = 0.01

// normalizeConfidence maps the model's self-reported confidence onto the
// 0..1 the column means. Models regularly answer on a percentage scale
// (85 for 0.85), so 1 < c <= 100 is read as a percentage; anything else
// out of range, NaN or infinite is clamped. The result is never below
// minEnrichedConfidence.
func normalizeConfidence(c float64) float32 {
	switch {
	case math.IsNaN(c) || math.IsInf(c, 0) || c < 0:
		c = 0
	case c > 1 && c <= 100:
		c /= 100
	case c > 100:
		c = 1
	}
	if c < minEnrichedConfidence {
		c = minEnrichedConfidence
	}
	return float32(c)
}

func normalizeEnum(value, fallback string) string {
	switch value {
	case "high", "medium", "low":
		return value
	default:
		return fallback
	}
}

func normalizeTimeCategory(value string) string {
	switch value {
	case "tolerate", "invest", "migrate", "eliminate":
		return value
	default:
		return ""
	}
}
