package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type migrationDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

type migrationPool interface {
	migrationDB
	Ping(ctx context.Context) error
	Close()
}

var newMigrationPool = func(ctx context.Context, databaseURL string) (migrationPool, error) {
	return pgxpool.New(ctx, databaseURL)
}

var fatalMigrationError = func(err error) {
	log.Fatal().Err(err).Msg("Migration failed")
}

func main() {
	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if err := runMigrationCLI(context.Background(), os.Args[1:], os.Getenv); err != nil {
		fatalMigrationError(err)
	}
}

func runMigrationCLI(ctx context.Context, args []string, getenv func(string) string) error {
	// Parse flags
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	var (
		dbURL          = flags.String("db", "", "Database URL (or set DATABASE_URL env)")
		migrationsPath = flags.String("path", "migrations", "Path to migrations directory")
		direction      = flags.String("direction", "up", "Migration direction: up or down")
		steps          = flags.Int("steps", 0, "Number of migrations to apply (0 = all)")
	)
	if err := flags.Parse(args); err != nil {
		return err
	}

	// Get database URL
	databaseURL := *dbURL
	if databaseURL == "" {
		databaseURL = getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		return fmt.Errorf("Database URL required. Use -db flag or set DATABASE_URL env")
	}

	// Connect to database
	pool, err := newMigrationPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	log.Info().Msg("Connected to database")

	// Ensure migrations table exists
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Run migrations
	switch *direction {
	case "up":
		if err := migrateUp(ctx, pool, *migrationsPath, *steps); err != nil {
			return fmt.Errorf("migration up failed: %w", err)
		}
	case "down":
		if err := migrateDown(ctx, pool, *migrationsPath, *steps); err != nil {
			return fmt.Errorf("migration down failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid direction %q: use 'up' or 'down'", *direction)
	}

	log.Info().Msg("Migration completed successfully")
	return nil
}

func ensureMigrationsTable(ctx context.Context, pool migrationDB) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func getAppliedMigrations(ctx context.Context, pool migrationDB) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
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

func getMigrationFiles(path, suffix string) ([]string, error) {
	pattern := filepath.Join(path, fmt.Sprintf("*%s.sql", suffix))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func extractVersion(filename string) string {
	base := filepath.Base(filename)
	// Remove .up.sql or .down.sql suffix
	base = strings.TrimSuffix(base, ".up.sql")
	base = strings.TrimSuffix(base, ".down.sql")
	return base
}

func migrateUp(ctx context.Context, pool migrationDB, path string, steps int) error {
	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("get applied migrations: %w", err)
	}

	files, err := getMigrationFiles(path, ".up")
	if err != nil {
		return fmt.Errorf("get migration files: %w", err)
	}

	count := 0
	for _, file := range files {
		version := extractVersion(file)
		if applied[version] {
			log.Debug().Str("version", version).Msg("Already applied, skipping")
			continue
		}

		if steps > 0 && count >= steps {
			break
		}

		log.Info().Str("version", version).Str("file", file).Msg("Applying migration")

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}

		log.Info().Str("version", version).Msg("Migration applied successfully")
		count++
	}

	if count == 0 {
		log.Info().Msg("No migrations to apply")
	} else {
		log.Info().Int("count", count).Msg("Migrations applied")
	}

	return nil
}

func migrateDown(ctx context.Context, pool migrationDB, path string, steps int) error {
	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		return fmt.Errorf("get applied migrations: %w", err)
	}

	files, err := getMigrationFiles(path, ".down")
	if err != nil {
		return fmt.Errorf("get migration files: %w", err)
	}

	// Reverse order for down migrations
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	if steps == 0 {
		steps = 1 // Default to rolling back one migration
	}

	count := 0
	for _, file := range files {
		version := extractVersion(file)
		if !applied[version] {
			log.Debug().Str("version", version).Msg("Not applied, skipping")
			continue
		}

		if count >= steps {
			break
		}

		log.Info().Str("version", version).Str("file", file).Msg("Rolling back migration")

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute rollback %s: %w", version, err)
		}

		if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("remove migration record %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit rollback %s: %w", version, err)
		}

		log.Info().Str("version", version).Msg("Migration rolled back successfully")
		count++
	}

	if count == 0 {
		log.Info().Msg("No migrations to roll back")
	} else {
		log.Info().Int("count", count).Msg("Migrations rolled back")
	}

	return nil
}
