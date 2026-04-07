package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MIGRATE_HELPER") != "1" {
		return
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			break
		}
	}
	main()
	os.Exit(0)
}

func TestGetMigrationFilesAndExtractVersion(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"002_second.up.sql",
		"001_first.up.sql",
		"003_third.down.sql",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- noop"), 0o644); err != nil {
			t.Fatalf("failed to write migration file %s: %v", name, err)
		}
	}

	upFiles, err := getMigrationFiles(dir, ".up")
	if err != nil {
		t.Fatalf("getMigrationFiles failed: %v", err)
	}
	if len(upFiles) != 2 {
		t.Fatalf("expected 2 up migrations, got %d", len(upFiles))
	}
	if !strings.HasSuffix(upFiles[0], "001_first.up.sql") || !strings.HasSuffix(upFiles[1], "002_second.up.sql") {
		t.Fatalf("expected sorted up migrations, got %v", upFiles)
	}

	if version := extractVersion(filepath.Join(dir, "001_first.up.sql")); version != "001_first" {
		t.Fatalf("unexpected extracted version: %s", version)
	}
	if version := extractVersion(filepath.Join(dir, "003_third.down.sql")); version != "003_third" {
		t.Fatalf("unexpected extracted version: %s", version)
	}
}

func TestMigrationLifecycle(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations failed: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("expected no applied migrations at start, got %v", applied)
	}

	dir := t.TempDir()
	version := fmt.Sprintf("999999_%d_test_table", time.Now().UnixNano())
	tableName := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())

	writeMigration(t, dir, version+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, tableName))
	writeMigration(t, dir, version+".down.sql", fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName))

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp failed: %v", err)
	}

	if !tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected table %s to exist after migrateUp", tableName)
	}

	applied, err = getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations after up failed: %v", err)
	}
	if !applied[version] {
		t.Fatalf("expected version %s to be recorded as applied", version)
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown failed: %v", err)
	}

	if tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected table %s to be removed after migrateDown", tableName)
	}

	applied, err = getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations after down failed: %v", err)
	}
	if applied[version] {
		t.Fatalf("expected version %s to be removed after rollback", version)
	}
}

func TestMainRunsMigrateUp(t *testing.T) {
	pool := setupMigrationTestDB(t)
	dir := t.TempDir()
	version := fmt.Sprintf("999998_%d_main_up", time.Now().UnixNano())
	tableName := fmt.Sprintf("migration_main_%d", time.Now().UnixNano())
	writeMigration(t, dir, version+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, tableName))

	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--", "-db", connStringFromPool(pool), "-path", dir, "-direction", "up", "-steps", "1")
	cmd.Env = append(os.Environ(), "GO_WANT_MIGRATE_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main helper failed: %v\n%s", err, string(out))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if !tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected table %s to exist after main helper", tableName)
	}
}

func TestMainRejectsInvalidDirection(t *testing.T) {
	pool := setupMigrationTestDB(t)
	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--", "-db", connStringFromPool(pool), "-direction", "sideways")
	cmd.Env = append(os.Environ(), "GO_WANT_MIGRATE_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected invalid direction helper to fail, output: %s", string(out))
	}
	if !strings.Contains(string(out), "Invalid direction") {
		t.Fatalf("expected invalid direction error, got: %s", string(out))
	}
}

func TestMainRequiresDatabaseURL(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--")
	cmd.Env = []string{"GO_WANT_MIGRATE_HELPER=1"}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected helper to fail without db url, output: %s", string(out))
	}
	if !strings.Contains(string(out), "Database URL required") {
		t.Fatalf("expected missing database error, got: %s", string(out))
	}
}

func TestMigrateUpHonorsStepsAndSkipsApplied(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	dir := t.TempDir()
	version1 := fmt.Sprintf("999997_%d_step_one", time.Now().UnixNano())
	version2 := fmt.Sprintf("999996_%d_step_two", time.Now().UnixNano())
	table1 := fmt.Sprintf("migration_step_one_%d", time.Now().UnixNano())
	table2 := fmt.Sprintf("migration_step_two_%d", time.Now().UnixNano())
	writeMigration(t, dir, version1+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, table1))
	writeMigration(t, dir, version2+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, table2))

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp step-limited failed: %v", err)
	}
	if !tableExists(t, ctx, pool, table2) {
		t.Fatalf("expected first sorted migration table %s to exist", table2)
	}
	if tableExists(t, ctx, pool, table1) {
		t.Fatalf("expected second migration table %s to be deferred", table1)
	}

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp applying remaining migrations failed: %v", err)
	}
	if !tableExists(t, ctx, pool, table1) {
		t.Fatalf("expected deferred migration table %s to exist", table1)
	}

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp no-op failed: %v", err)
	}
}

func TestMigrateDownDefaultsToSingleRollback(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	dir := t.TempDir()
	version1 := fmt.Sprintf("999995_%d_down_one", time.Now().UnixNano())
	version2 := fmt.Sprintf("999994_%d_down_two", time.Now().UnixNano())
	table1 := fmt.Sprintf("migration_down_one_%d", time.Now().UnixNano())
	table2 := fmt.Sprintf("migration_down_two_%d", time.Now().UnixNano())
	writeMigration(t, dir, version1+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, table1))
	writeMigration(t, dir, version1+".down.sql", fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, table1))
	writeMigration(t, dir, version2+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, table2))
	writeMigration(t, dir, version2+".down.sql", fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, table2))

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp failed: %v", err)
	}

	if err := migrateDown(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateDown default step failed: %v", err)
	}

	if tableExists(t, ctx, pool, table1) == tableExists(t, ctx, pool, table2) {
		t.Fatalf("expected only one migration to be rolled back by default")
	}
}

func writeMigration(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write migration %s: %v", name, err)
	}
}

func tableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tableName string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1
	)`, tableName).Scan(&exists); err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	return exists
}

func connStringFromPool(pool *pgxpool.Pool) string {
	cfg := pool.Config().ConnConfig
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
}

func setupMigrationTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		return testutil.SetupTestDB(t)
	}

	adminURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("failed to parse DATABASE_URL: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, adminURL.String())
	if err != nil {
		t.Fatalf("failed to connect admin pool: %v", err)
	}

	dbName := fmt.Sprintf("migrate_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		adminPool.Close()
		t.Fatalf("failed to create migration test database %s: %v", dbName, err)
	}

	testURL := *adminURL
	testURL.Path = "/" + dbName

	pool, err := pgxpool.New(ctx, testURL.String())
	if err != nil {
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
		adminPool.Close()
		t.Fatalf("failed to connect migration test database %s: %v", dbName, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
		adminPool.Close()
		t.Fatalf("failed to ping migration test database %s: %v", dbName, err)
	}

	t.Cleanup(func() {
		pool.Close()

		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		if _, err := adminPool.Exec(cleanupCtx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName)); err != nil {
			t.Logf("warning: failed to drop migration test database %s: %v", dbName, err)
		}
		adminPool.Close()
	})

	return pool
}
