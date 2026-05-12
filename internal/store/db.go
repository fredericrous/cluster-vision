package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

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

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := runMigrations(databaseURL); err != nil {
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

func runMigrations(databaseURL string) error {
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

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
