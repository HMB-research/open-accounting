package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse flags
	var (
		dbURL          = flag.String("db", "", "Database URL (or set DATABASE_URL env)")
		migrationsPath = flag.String("path", "migrations", "Path to migrations directory")
		direction      = flag.String("direction", "up", "Migration direction: up or down")
		steps          = flag.Int("steps", 0, "Number of migrations to apply (0 = all)")
	)
	flag.Parse()

	// Get database URL
	databaseURL := *dbURL
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		log.Fatal().Msg("Database URL required. Use -db flag or set DATABASE_URL env")
	}

	ctx := context.Background()

	// Connect to database
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to ping database")
	}
	log.Info().Msg("Connected to database")

	// Ensure migrations table exists
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		log.Fatal().Err(err).Msg("Failed to create migrations table")
	}

	// Run migrations
	switch *direction {
	case "up":
		if err := migrateUp(ctx, pool, *migrationsPath, *steps); err != nil {
			log.Fatal().Err(err).Msg("Migration up failed")
		}
	case "down":
		if err := migrateDown(ctx, pool, *migrationsPath, *steps); err != nil {
			log.Fatal().Err(err).Msg("Migration down failed")
		}
	default:
		log.Fatal().Str("direction", *direction).Msg("Invalid direction. Use 'up' or 'down'")
	}

	log.Info().Msg("Migration completed successfully")
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func getAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
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

func migrateUp(ctx context.Context, pool *pgxpool.Pool, path string, steps int) error {
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

func migrateDown(ctx context.Context, pool *pgxpool.Pool, path string, steps int) error {
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
