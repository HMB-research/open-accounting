//go:build integration

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

// GetTestContainer returns a shared PostgreSQL container for integration tests.
// If DATABASE_URL is set, it uses that instead of starting a container.
// The container is shared across all tests to avoid startup overhead.
func GetTestContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Check if DATABASE_URL is set - use external database if so
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return setupExternalDB(t, dbURL)
	}

	// Use testcontainers
	if containerInstance == nil {
		containerInstance = startContainer(t)
	}

	return containerInstance.Pool
}

// setupExternalDB connects to an external database specified by DATABASE_URL
func setupExternalDB(t *testing.T, dbURL string) *pgxpool.Pool {
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

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
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
		container.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		container.Terminate(ctx)
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
		container.Terminate(ctx)
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
	for _, migration := range migrations {
		migrationPath := filepath.Join(migrationsDir, migration)
		content, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", migration, err)
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration, err)
		}

		t.Logf("Applied migration: %s", migration)
	}

	return nil
}

// CleanupContainer cleans up the container (call from TestMain if needed)
func CleanupContainer() {
	if containerInstance != nil && containerInstance.Container != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		containerInstance.Pool.Close()
		containerInstance.Container.Terminate(ctx)
		containerInstance = nil
	}
}
