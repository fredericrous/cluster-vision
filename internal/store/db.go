package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	mpg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps a pgxpool and provides EAM data access.
type DB struct {
	Pool *pgxpool.Pool
}

// New connects to PostgreSQL and runs migrations.
// New is also what the in-cluster migration check exercises: a PR touching
// this file or migrations/ builds the PR image, and the preview operator boots
// /api against a prod-data clone until /api/config reports eam:true.
func New(ctx context.Context, databaseURL string) (*DB, error) {
	// ParseConfig so we can stamp application_name = cluster-vision before
	// dialing — shows up in pg_stat_activity, which is how we triaged the
	// kb-vision pool-wedge incident. pgxpool's defaults already include a
	// 1h max connection lifetime + 1m health check loop, so we don't
	// override those.
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "cluster-vision"

	// Bring-up is bounded: New runs before the HTTP listener exists, so a
	// connect or migration that never returns keeps the whole process from
	// serving anything (seen 2026-09-22 on the in-cluster migration check:
	// the log stopped after the parser line and the API never listened). A
	// deadline turns that into a logged, EAM-disabled boot instead.
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	slog.Info("EAM database: pinging")
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	slog.Info("EAM database: applying migrations")
	if err := runMigrations(ctx, databaseURL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	slog.Info("EAM database connected and migrated")
	return &DB{Pool: pool}, nil
}

// Close shuts down the connection pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// connectTimeout bounds New: pool creation, ping and migrations together.
// Generous for a real migration set, far below anything a readiness probe or
// a supervisor would wait for a process that has not started listening.
const connectTimeout = 2 * time.Minute

func runMigrations(ctx context.Context, databaseURL string) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("opening database for migrations: %w", err)
	}
	defer func() { _ = db.Close() }()

	driver, err := mpg.WithInstance(db, &mpg.Config{})
	if err != nil {
		return fmt.Errorf("creating migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	// golang-migrate has no context on Up; honour the deadline by racing it
	// and asking the migrator to stop at the next boundary if time runs out.
	done := make(chan error, 1)
	go func() { done <- m.Up() }()
	select {
	case err := <-done:
		if err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("applying migrations: %w", err)
		}
		return nil
	case <-ctx.Done():
		m.GracefulStop <- true
		return fmt.Errorf("applying migrations: %w", ctx.Err())
	}
}
