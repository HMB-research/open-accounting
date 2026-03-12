package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a testcontainers postgres instance
type PostgresContainer struct {
	Container testcontainers.Container
	Pool      *pgxpool.Pool
	ConnStr   string
}

// containerInstance is a singleton for the test container
var containerInstance *PostgresContainer

const migrationAdvisoryLockKey = 23456789

// GetTestContainer returns a shared PostgreSQL container for integration tests.
// If DATABASE_URL is set, it uses that instead of starting a container.
// The container is shared across all tests to avoid startup overhead.
func GetTestContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Check if DATABASE_URL is set - use external database if so
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if containerInstance != nil && containerInstance.ConnStr == dbURL && containerInstance.Pool != nil {
			return containerInstance.Pool
		}
		containerInstance = setupExternalDB(t, dbURL)
		return containerInstance.Pool
	}

	// Use testcontainers
	if containerInstance == nil {
		containerInstance = startContainer(t)
	}

	return containerInstance.Pool
}

// setupExternalDB connects to an external database specified by DATABASE_URL
func setupExternalDB(t *testing.T, dbURL string) *PostgresContainer {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping database: %v", err)
	}

	if err := runMigrations(t, pool); err != nil {
		pool.Close()
		t.Fatalf("failed to prepare external database: %v", err)
	}

	return &PostgresContainer{
		Pool:    pool,
		ConnStr: dbURL,
	}
}

// startContainer starts a new PostgreSQL container and runs migrations
func startContainer(t *testing.T) *PostgresContainer {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		if terminateErr := container.Terminate(ctx); terminateErr != nil {
			t.Fatalf("failed to get connection string: %v (terminate container: %v)", err, terminateErr)
		}
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		if terminateErr := container.Terminate(ctx); terminateErr != nil {
			t.Fatalf("failed to create pool: %v (terminate container: %v)", err, terminateErr)
		}
		t.Fatalf("failed to create pool: %v", err)
	}

	// Wait for connection
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Run migrations
	if err := runMigrations(t, pool); err != nil {
		pool.Close()
		if terminateErr := container.Terminate(ctx); terminateErr != nil {
			t.Fatalf("failed to run migrations: %v (terminate container: %v)", err, terminateErr)
		}
		t.Fatalf("failed to run migrations: %v", err)
	}

	pc := &PostgresContainer{
		Container: container,
		Pool:      pool,
		ConnStr:   connStr,
	}

	// Note: We don't cleanup the container here because it's shared across tests
	// The container will be cleaned up when the process exits

	return pc
}

// runMigrations runs all .up.sql migrations from the migrations directory
func runMigrations(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()

	// Find migrations directory relative to this file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get current file path")
	}

	// Navigate from internal/testutil to project root
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	migrationsDir := filepath.Join(projectRoot, "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Collect and sort migration files
	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			migrations = append(migrations, entry.Name())
		}
	}
	sort.Strings(migrations)

	ctx := context.Background()
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration lock connection: %w", err)
	}
	defer lockConn.Release()

	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = lockConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockKey)
	}()

	if err := ensureBootstrapMigrationsTable(ctx, pool); err != nil {
		return fmt.Errorf("ensure bootstrap migrations table: %w", err)
	}

	migratorApplied, err := getAppliedSchemaMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("get applied schema migrations: %w", err)
	}
	if len(migratorApplied) > 0 {
		if err := backfillBootstrapMigrations(ctx, pool); err != nil {
			return fmt.Errorf("backfill bootstrap migrations: %w", err)
		}
	}

	bootstrapApplied, err := getAppliedBootstrapMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("get applied bootstrap migrations: %w", err)
	}

	applied := make(map[string]bool, len(migratorApplied)+len(bootstrapApplied))
	for version := range migratorApplied {
		applied[version] = true
	}
	for version := range bootstrapApplied {
		applied[version] = true
	}

	for _, migration := range migrations {
		version := strings.TrimSuffix(migration, ".up.sql")
		if applied[version] {
			continue
		}

		migrationPath := filepath.Join(migrationsDir, migration)
		content, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migration, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration, err)
		}

		if _, err = tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %s: %w", migration, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO testutil_bootstrap_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", migration, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration, err)
		}

		t.Logf("Applied migration: %s", migration)
	}

	return nil
}

// CleanupContainer cleans up the container (call from TestMain if needed)
func CleanupContainer() {
	if containerInstance != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if containerInstance.Pool != nil {
			containerInstance.Pool.Close()
		}
		if containerInstance.Container != nil {
			if err := containerInstance.Container.Terminate(ctx); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "failed to terminate test container: %v\n", err)
			}
		}
		containerInstance = nil
	}
}

func ensureBootstrapMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS testutil_bootstrap_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func getAppliedBootstrapMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM testutil_bootstrap_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func getAppliedSchemaMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return map[string]bool{}, nil
	}

	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func backfillBootstrapMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO testutil_bootstrap_migrations (version)
		SELECT version FROM schema_migrations
		ON CONFLICT (version) DO NOTHING
	`)
	return err
}
