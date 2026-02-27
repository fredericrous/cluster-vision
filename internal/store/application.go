package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Application struct {
	ID                        uuid.UUID  `json:"id"`
	Name                      string     `json:"name"`
	DisplayName               *string    `json:"display_name"`
	Description               *string    `json:"description"`
	DescriptionSource         string     `json:"description_source"`
	Status                    string     `json:"status"`
	BusinessCriticality       string     `json:"business_criticality"`
	BusinessCriticalitySource string     `json:"business_criticality_source"`
	TechnicalRisk             string     `json:"technical_risk"`
	TechnicalRiskSource       string     `json:"technical_risk_source"`
	TechnicalRiskReasoning    *string    `json:"technical_risk_reasoning"`
	LifecyclePhase            string     `json:"lifecycle_phase"`
	TimeCategory              *string    `json:"time_category"`
	TimeCategorySource        string     `json:"time_category_source"`
	TimeCategoryReasoning     *string    `json:"time_category_reasoning"`
	EndOfLifeDate             *string    `json:"end_of_life_date"`
	Tags                      []string   `json:"tags"`
	AIConfidence              float32    `json:"ai_confidence"`
	ManualOverride            bool       `json:"manual_override"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type ApplicationFilter struct {
	Status  string
	Risk    string
	Cluster string
	Search  string
	Limit   int
	Offset  int
}

func (db *DB) ListApplications(ctx context.Context, f ApplicationFilter) ([]Application, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if f.Status != "" {
		conditions = append(conditions, fmt.Sprintf("a.status = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	}
	if f.Risk != "" {
		conditions = append(conditions, fmt.Sprintf("a.technical_risk = $%d", argIdx))
		args = append(args, f.Risk)
		argIdx++
	}
	if f.Cluster != "" {
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM k8s_sources k WHERE k.app_id = a.id AND k.cluster = $%d)", argIdx))
		args = append(args, f.Cluster)
		argIdx++
	}
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(a.name ILIKE $%d OR a.display_name ILIKE $%d OR a.description ILIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+f.Search+"%")
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT count(*) FROM applications a %s", where)
	var total int
	if err := db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting applications: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`SELECT id, name, display_name, description, description_source,
		status, business_criticality, business_criticality_source,
		technical_risk, technical_risk_source, technical_risk_reasoning,
		lifecycle_phase, time_category, time_category_source, time_category_reasoning,
		end_of_life_date, tags, ai_confidence, manual_override, created_at, updated_at
		FROM applications a %s ORDER BY name LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing applications: %w", err)
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var a Application
		if err := rows.Scan(&a.ID, &a.Name, &a.DisplayName, &a.Description, &a.DescriptionSource,
			&a.Status, &a.BusinessCriticality, &a.BusinessCriticalitySource,
			&a.TechnicalRisk, &a.TechnicalRiskSource, &a.TechnicalRiskReasoning,
			&a.LifecyclePhase, &a.TimeCategory, &a.TimeCategorySource, &a.TimeCategoryReasoning,
			&a.EndOfLifeDate, &a.Tags, &a.AIConfidence, &a.ManualOverride, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning application: %w", err)
		}
		apps = append(apps, a)
	}
	return apps, total, nil
}

func (db *DB) GetApplication(ctx context.Context, id uuid.UUID) (*Application, error) {
	var a Application
	err := db.Pool.QueryRow(ctx, `SELECT id, name, display_name, description, description_source,
		status, business_criticality, business_criticality_source,
		technical_risk, technical_risk_source, technical_risk_reasoning,
		lifecycle_phase, time_category, time_category_source, time_category_reasoning,
		end_of_life_date, tags, ai_confidence, manual_override, created_at, updated_at
		FROM applications WHERE id = $1`, id).Scan(
		&a.ID, &a.Name, &a.DisplayName, &a.Description, &a.DescriptionSource,
		&a.Status, &a.BusinessCriticality, &a.BusinessCriticalitySource,
		&a.TechnicalRisk, &a.TechnicalRiskSource, &a.TechnicalRiskReasoning,
		&a.LifecyclePhase, &a.TimeCategory, &a.TimeCategorySource, &a.TimeCategoryReasoning,
		&a.EndOfLifeDate, &a.Tags, &a.AIConfidence, &a.ManualOverride, &a.CreatedAt, &a.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting application: %w", err)
	}
	return &a, nil
}

func (db *DB) GetApplicationByName(ctx context.Context, name string) (*Application, error) {
	var a Application
	err := db.Pool.QueryRow(ctx, `SELECT id, name, display_name, description, description_source,
		status, business_criticality, business_criticality_source,
		technical_risk, technical_risk_source, technical_risk_reasoning,
		lifecycle_phase, time_category, time_category_source, time_category_reasoning,
		end_of_life_date, tags, ai_confidence, manual_override, created_at, updated_at
		FROM applications WHERE name = $1`, name).Scan(
		&a.ID, &a.Name, &a.DisplayName, &a.Description, &a.DescriptionSource,
		&a.Status, &a.BusinessCriticality, &a.BusinessCriticalitySource,
		&a.TechnicalRisk, &a.TechnicalRiskSource, &a.TechnicalRiskReasoning,
		&a.LifecyclePhase, &a.TimeCategory, &a.TimeCategorySource, &a.TimeCategoryReasoning,
		&a.EndOfLifeDate, &a.Tags, &a.AIConfidence, &a.ManualOverride, &a.CreatedAt, &a.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting application by name: %w", err)
	}
	return &a, nil
}

func (db *DB) CreateApplication(ctx context.Context, a *Application) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now

	_, err := db.Pool.Exec(ctx, `INSERT INTO applications
		(id, name, display_name, description, description_source, status,
		business_criticality, business_criticality_source,
		technical_risk, technical_risk_source, technical_risk_reasoning,
		lifecycle_phase, time_category, time_category_source, time_category_reasoning,
		end_of_life_date, tags, ai_confidence, manual_override, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		a.ID, a.Name, a.DisplayName, a.Description, a.DescriptionSource, a.Status,
		a.BusinessCriticality, a.BusinessCriticalitySource,
		a.TechnicalRisk, a.TechnicalRiskSource, a.TechnicalRiskReasoning,
		a.LifecyclePhase, a.TimeCategory, a.TimeCategorySource, a.TimeCategoryReasoning,
		a.EndOfLifeDate, a.Tags, a.AIConfidence, a.ManualOverride, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating application: %w", err)
	}
	return nil
}

func (db *DB) UpdateApplication(ctx context.Context, a *Application) error {
	a.UpdatedAt = time.Now()
	_, err := db.Pool.Exec(ctx, `UPDATE applications SET
		display_name=$2, description=$3, description_source=$4, status=$5,
		business_criticality=$6, business_criticality_source=$7,
		technical_risk=$8, technical_risk_source=$9, technical_risk_reasoning=$10,
		lifecycle_phase=$11, time_category=$12, time_category_source=$13, time_category_reasoning=$14,
		end_of_life_date=$15, tags=$16, ai_confidence=$17, manual_override=$18, updated_at=$19
		WHERE id=$1`,
		a.ID, a.DisplayName, a.Description, a.DescriptionSource, a.Status,
		a.BusinessCriticality, a.BusinessCriticalitySource,
		a.TechnicalRisk, a.TechnicalRiskSource, a.TechnicalRiskReasoning,
		a.LifecyclePhase, a.TimeCategory, a.TimeCategorySource, a.TimeCategoryReasoning,
		a.EndOfLifeDate, a.Tags, a.AIConfidence, a.ManualOverride, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("updating application: %w", err)
	}
	return nil
}

func (db *DB) DeleteApplication(ctx context.Context, id uuid.UUID) error {
	_, err := db.Pool.Exec(ctx, "DELETE FROM applications WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting application: %w", err)
	}
	return nil
}

// UpsertApplicationByName creates or updates an application by name (for auto-discovery).
// Returns the application and whether it was newly created.
func (db *DB) UpsertApplicationByName(ctx context.Context, name string, update func(*Application)) (*Application, bool, error) {
	existing, err := db.GetApplicationByName(ctx, name)
	if err != nil {
		return nil, false, err
	}

	if existing != nil {
		if !existing.ManualOverride {
			update(existing)
			if err := db.UpdateApplication(ctx, existing); err != nil {
				return nil, false, err
			}
		}
		return existing, false, nil
	}

	a := &Application{
		Name:                      name,
		DescriptionSource:         "auto-discovered",
		Status:                    "active",
		BusinessCriticality:       "medium",
		BusinessCriticalitySource: "auto-discovered",
		TechnicalRisk:             "medium",
		TechnicalRiskSource:       "auto-discovered",
		LifecyclePhase:            "active",
		TimeCategorySource:        "auto-discovered",
		Tags:                      []string{},
	}
	update(a)
	if err := db.CreateApplication(ctx, a); err != nil {
		return nil, false, err
	}
	return a, true, nil
}
