package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
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
// New is what the in-cluster migration check exercises — a PR touching
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

// connectTimeout bounds one attempt of New: pool creation, ping and
// migrations together. Generous for a real migration set, far below anything
// a readiness probe or a supervisor would wait for a process that has not
// started listening.
const connectTimeout = 2 * time.Minute

// NewWithRetry calls New until it succeeds or budget runs out, backing off
// from 2 s to 15 s between attempts. A database that is up but not yet
// stable — a freshly restored CNPG clone whose instance manager marks it
// down for minutes while the copy-on-write volume warms, a primary mid
// failover — refuses connections for a while and then serves normally; one
// attempt at boot turns that into a process that runs EAM-less until it is
// restarted by hand. Each attempt logs so a persistent refusal is visible.
func NewWithRetry(ctx context.Context, databaseURL string, budget time.Duration) (*DB, error) {
	deadline := time.Now().Add(budget)
	wait := 2 * time.Second
	for attempt := 1; ; attempt++ {
		db, err := New(ctx, databaseURL)
		if err == nil {
			return db, nil
		}
		if time.Now().Add(wait).After(deadline) {
			return nil, fmt.Errorf("after %d attempt(s): %w", attempt, err)
		}
		slog.Warn("EAM database not ready, retrying", "attempt", attempt, "retry_in", wait, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		if wait < 15*time.Second {
			wait *= 2
			if wait > 15*time.Second {
				wait = 15 * time.Second
			}
		}
	}
}

func runMigrations(ctx context.Context, databaseURL string) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	// The migrator opens its own database/sql connection. sql.Open is lazy;
	// the first dial happens inside mpg.WithInstance, which runs a query and
	// has no context — a dial that never completes (a server restarting
	// mid-handshake, the state a freshly restored CNPG clone passes through)
	// blocks here forever, past any deadline the caller holds. Give the
	// connection its own connect_timeout so the dial can fail instead of
	// hang, and ping it under ctx before handing it to the migrator.
	migrateURL, err := withConnectTimeout(databaseURL, 10)
	if err != nil {
		return fmt.Errorf("migration database URL: %w", err)
	}
	db, err := sql.Open("pgx", migrateURL)
	if err != nil {
		return fmt.Errorf("opening database for migrations: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("pinging database for migrations: %w", err)
	}

	driver, err := mpg.WithInstance(db, &mpg.Config{})
	if err != nil {
		return fmt.Errorf("creating migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	// Closing the migrator before db (defers run last-in first-out) hands
	// its connection, and the advisory lock it holds, back first.
	defer func() { _, _ = m.Close() }()

	return awaitMigration(ctx, m.Up, m.GracefulStop)
}

// awaitMigration runs up and honours ctx the only way golang-migrate allows:
// Up has no context, so on cancellation it asks the migrator to stop at the
// next migration boundary and then WAITS for Up to return. Returning early
// instead would let the caller's deferred Close cut the connection under a
// migration that is still executing, leaving schema_migrations dirty and
// every later boot refusing to migrate until someone forces the version.
func awaitMigration(ctx context.Context, up func() error, stop chan<- bool) error {
	done := make(chan error, 1)
	go func() { done <- up() }()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		slog.Warn("EAM database: migration deadline reached, stopping after the current migration")
		select {
		case stop <- true:
		default: // a stop is already pending
		}
		err = <-done
		if err == nil || errors.Is(err, migrate.ErrNoChange) {
			// Up finished (or stopped cleanly) anyway; the deadline still
			// failed this attempt, so report it rather than half-success.
			return fmt.Errorf("applying migrations: %w", ctx.Err())
		}
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

// withConnectTimeout sets libpq's connect_timeout (seconds) on a database URL
// unless the caller already chose one.
func withConnectTimeout(databaseURL string, seconds int) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Get("connect_timeout") == "" {
		q.Set("connect_timeout", strconv.Itoa(seconds))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
