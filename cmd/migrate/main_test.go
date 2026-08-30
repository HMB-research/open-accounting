//go:build integration

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

func TestMainRunsMigrateDownWithDatabaseURLEnv(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	dir := t.TempDir()
	version := fmt.Sprintf("999998_%d_main_down", time.Now().UnixNano())
	tableName := fmt.Sprintf("migration_main_down_%d", time.Now().UnixNano())
	writeMigration(t, dir, version+".up.sql", fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id INT PRIMARY KEY);`, tableName))
	writeMigration(t, dir, version+".down.sql", fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName))

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp setup failed: %v", err)
	}
	if !tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected table %s to exist before main helper rollback", tableName)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess", "--", "-path", dir, "-direction", "down", "-steps", "1")
	cmd.Env = append(os.Environ(), "GO_WANT_MIGRATE_HELPER=1", "DATABASE_URL="+connStringFromPool(pool))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main helper down failed: %v\n%s", err, string(out))
	}

	if tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected table %s to be removed after main helper rollback", tableName)
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
	if !strings.Contains(string(out), "invalid direction") {
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
	if !strings.Contains(string(out), "missing database URL") {
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

func TestMigrateUpRollsBackFailedMigration(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	dir := t.TempDir()
	version := fmt.Sprintf("999993_%d_failed_up", time.Now().UnixNano())
	tableName := fmt.Sprintf("migration_failed_up_%d", time.Now().UnixNano())
	writeMigration(t, dir, version+".up.sql", fmt.Sprintf(`
		CREATE TABLE %s (id INT PRIMARY KEY);
		SELECT open_accounting_missing_function();
	`, tableName))

	err := migrateUp(ctx, pool, dir, 0)
	if err == nil {
		t.Fatal("expected failed migration error")
	}
	if !strings.Contains(err.Error(), "execute migration") {
		t.Fatalf("expected execute migration error, got: %v", err)
	}
	if tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected failed migration table %s to be rolled back", tableName)
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations failed: %v", err)
	}
	if applied[version] {
		t.Fatalf("expected failed migration %s not to be recorded", version)
	}
}

func TestMigrateDownRollsBackFailedRollback(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	dir := t.TempDir()
	version := fmt.Sprintf("999992_%d_failed_down", time.Now().UnixNano())
	tableName := fmt.Sprintf("migration_failed_down_%d", time.Now().UnixNano())
	writeMigration(t, dir, version+".up.sql", fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY);`, tableName))
	writeMigration(t, dir, version+".down.sql", fmt.Sprintf(`
		DROP TABLE %s;
		SELECT open_accounting_missing_function();
	`, tableName))

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp setup failed: %v", err)
	}
	if !tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected table %s to exist before failed rollback", tableName)
	}

	err := migrateDown(ctx, pool, dir, 1)
	if err == nil {
		t.Fatal("expected failed rollback error")
	}
	if !strings.Contains(err.Error(), "execute rollback") {
		t.Fatalf("expected execute rollback error, got: %v", err)
	}
	if !tableExists(t, ctx, pool, tableName) {
		t.Fatalf("expected failed rollback table %s to remain", tableName)
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations failed: %v", err)
	}
	if !applied[version] {
		t.Fatalf("expected failed rollback migration %s to remain recorded", version)
	}
}

func TestMigrateDownSkipsUnappliedMigration(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	dir := t.TempDir()
	version := fmt.Sprintf("999991_%d_unapplied_down", time.Now().UnixNano())
	tableName := fmt.Sprintf("migration_unapplied_down_%d", time.Now().UnixNano())
	writeMigration(t, dir, version+".down.sql", fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName))

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown should skip unapplied migration: %v", err)
	}
}

func TestTenantFeatureMigrationsHandlePartialSchemas(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			schema_name TEXT NOT NULL UNIQUE,
			is_active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE SCHEMA tenant_partial;
		CREATE SCHEMA tenant_complete;
		INSERT INTO tenants (schema_name, is_active)
		VALUES ('tenant_partial', true), ('tenant_complete', true);

		CREATE TABLE tenant_complete.email_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			template_type VARCHAR(50) NOT NULL,
			subject TEXT,
			body_html TEXT,
			body_text TEXT,
			UNIQUE (tenant_id, template_type)
		);
		CREATE TABLE tenant_complete.orders (id UUID PRIMARY KEY DEFAULT gen_random_uuid());
		CREATE TABLE tenant_complete.products (id UUID PRIMARY KEY DEFAULT gen_random_uuid());
		CREATE TABLE tenant_complete.warehouses (id UUID PRIMARY KEY DEFAULT gen_random_uuid());
	`)

	dir := t.TempDir()
	copyRepositoryMigration(t, dir, "021_reminder_rules.up.sql")
	copyRepositoryMigration(t, dir, "021_reminder_rules.down.sql")
	copyRepositoryMigration(t, dir, "033_order_stock_reservations.up.sql")
	copyRepositoryMigration(t, dir, "033_order_stock_reservations.down.sql")

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp with partial tenant schemas failed: %v", err)
	}

	if !schemaTableExists(t, ctx, pool, "tenant_partial", "reminder_rules") {
		t.Fatalf("expected reminder_rules to be created for partial schema")
	}
	if schemaTableExists(t, ctx, pool, "tenant_partial", "order_stock_reservations") {
		t.Fatalf("expected order_stock_reservations to be skipped for schema missing order inventory tables")
	}
	if !schemaTableExists(t, ctx, pool, "tenant_complete", "order_stock_reservations") {
		t.Fatalf("expected order_stock_reservations to be created for complete schema")
	}

	if err := migrateDown(ctx, pool, dir, 2); err != nil {
		t.Fatalf("migrateDown with partial tenant schemas failed: %v", err)
	}
	if schemaTableExists(t, ctx, pool, "tenant_partial", "reminder_rules") {
		t.Fatalf("expected reminder_rules to be removed after rollback")
	}
	if schemaTableExists(t, ctx, pool, "tenant_complete", "order_stock_reservations") {
		t.Fatalf("expected order_stock_reservations to be removed after rollback")
	}
}

func TestJournalEntryPostReasonMigrationUpdatesTenantBootstrap(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		CREATE SCHEMA tenant_existing_post_reason;
		CREATE TABLE tenant_existing_post_reason.journal_entries (
			id UUID PRIMARY KEY
		);

		CREATE OR REPLACE FUNCTION create_accounting_tables(schema_name TEXT) RETURNS VOID AS $$
		BEGIN
			EXECUTE format('CREATE TABLE IF NOT EXISTS %I.journal_entries (id UUID PRIMARY KEY)', schema_name);
		END;
		$$ LANGUAGE plpgsql;

		DO $$
		DECLARE
			bootstrap_function TEXT;
		BEGIN
			FOREACH bootstrap_function IN ARRAY ARRAY[
				'add_vat_columns_to_journal_lines',
				'add_payment_reversal_columns',
				'add_reconciliation_tables_to_schema',
				'add_recurring_tables_to_schema',
				'add_quotes_and_orders_tables',
				'add_fixed_assets_tables',
				'add_fixed_asset_disposal_journal_links',
				'create_inventory_tables',
				'add_inventory_movement_tracking_metadata',
				'add_inventory_lot_reservations',
				'add_payroll_tables',
				'add_leave_management_tables',
				'create_email_tables_only',
				'add_kmd_tables_to_schema',
				'fix_email_log_schema',
				'add_reminder_rules_to_schema',
				'sync_email_template_type_constraint',
				'add_interest_tables',
				'add_document_tables',
				'add_document_review_workflow',
				'add_bank_transaction_review_columns',
				'add_close_pack_document_entity',
				'add_order_stock_reservations',
				'add_journal_entry_evidence_requirement',
				'add_journal_entry_templates',
				'add_journal_entry_template_recurrence',
				'add_bank_match_rules',
				'add_invoice_vat_treatment',
				'add_expense_tables',
				'add_commercial_document_entities',
				'add_leave_record_document_entity',
				'add_tax_declaration_document_entities',
				'add_document_lifecycle_workflow',
				'add_document_legal_hold_workflow',
				'add_document_lifecycle_integrity',
				'add_cost_center_tables',
				'add_migration_execution_run_tables'
			]
			LOOP
				EXECUTE format(
					'CREATE OR REPLACE FUNCTION %I(schema_name TEXT) RETURNS VOID AS $fn$ BEGIN END; $fn$ LANGUAGE plpgsql',
					bootstrap_function
				);
			END LOOP;
		END $$;
	`)

	dir := t.TempDir()
	copyRepositoryMigration(t, dir, "061_journal_entry_post_reason.up.sql")
	copyRepositoryMigration(t, dir, "061_journal_entry_post_reason.down.sql")

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp failed: %v", err)
	}

	if !schemaColumnExists(t, ctx, pool, "tenant_existing_post_reason", "journal_entries", "post_reason") {
		t.Fatalf("expected existing tenant journal_entries to receive post_reason")
	}

	execSQL(t, ctx, pool, `SELECT create_tenant_schema('tenant_future_post_reason')`)

	if !schemaColumnExists(t, ctx, pool, "tenant_future_post_reason", "journal_entries", "post_reason") {
		t.Fatalf("expected future tenant bootstrap journal_entries to receive post_reason")
	}
}

func TestImportSessionLedgerVerificationAndPlanInputMigrationsUpgradeAndBootstrap(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			schema_name TEXT NOT NULL UNIQUE
		);
		CREATE SCHEMA tenant_import_legacy;
		CREATE SCHEMA tenant_import_already_upgraded;
		INSERT INTO tenants (schema_name) VALUES ('tenant_import_legacy');
		INSERT INTO tenants (schema_name) VALUES ('tenant_import_already_upgraded');
	`)

	dir := t.TempDir()
	for _, name := range []string{
		"064_import_sessions.up.sql",
		"064_import_sessions.down.sql",
		"065_import_session_ledger_verification.up.sql",
		"065_import_session_ledger_verification.down.sql",
		"066_import_session_ledger_plan_input.up.sql",
		"066_import_session_ledger_plan_input.down.sql",
		"067_smartaccounts_sync_controls.up.sql",
		"067_smartaccounts_sync_controls.down.sql",
	} {
		copyRepositoryMigration(t, dir, name)
	}

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp 064 failed: %v", err)
	}
	if schemaColumnExists(t, ctx, pool, "tenant_import_legacy", "import_sessions", "ledger_verification") {
		t.Fatal("deployed migration 064 must retain the initial receipt schema")
	}
	execSQL(t, ctx, pool, `
		ALTER TABLE tenant_import_already_upgraded.import_sessions
		ADD COLUMN ledger_verification JSONB NOT NULL DEFAULT '{}'::JSONB;
	`)

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp 065 failed: %v", err)
	}
	if !schemaColumnExists(t, ctx, pool, "tenant_import_legacy", "import_sessions", "ledger_verification") {
		t.Fatal("expected 065 to upgrade the existing tenant receipt table")
	}
	if !schemaColumnExists(t, ctx, pool, "tenant_import_already_upgraded", "import_sessions", "ledger_verification") {
		t.Fatal("expected 065 to safely retain an already upgraded receipt table")
	}
	if schemaColumnExists(t, ctx, pool, "tenant_import_legacy", "import_sessions", "ledger_plan_input") {
		t.Fatal("ledger plan input must remain in forward-only migration 066")
	}

	// The helper itself must tolerate a fresh DB whose initial schema already
	// contains the column, and a retry against an upgraded tenant.
	execSQL(t, ctx, pool, `
		SELECT add_import_session_ledger_verification('tenant_import_legacy');
		SELECT add_import_session_ledger_verification('tenant_import_legacy');
	`)

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp 066 failed: %v", err)
	}
	for _, schemaName := range []string{"tenant_import_legacy", "tenant_import_already_upgraded"} {
		if !schemaColumnExists(t, ctx, pool, schemaName, "import_sessions", "ledger_plan_input") {
			t.Fatalf("expected 066 to upgrade %s with normalized ledger plan input", schemaName)
		}
	}
	execSQL(t, ctx, pool, `
		SELECT add_import_session_ledger_plan_input('tenant_import_legacy');
		SELECT add_import_session_ledger_plan_input('tenant_import_legacy');
	`)

	createImportSessionBootstrapStubs(t, ctx, pool)
	execSQL(t, ctx, pool, `SELECT create_tenant_schema('tenant_import_future')`)
	if !schemaColumnExists(t, ctx, pool, "tenant_import_future", "import_sessions", "ledger_verification") {
		t.Fatal("expected 065 tenant bootstrap to include ledger verification after 066")
	}
	if !schemaColumnExists(t, ctx, pool, "tenant_import_future", "import_sessions", "ledger_plan_input") {
		t.Fatal("expected 066 tenant bootstrap to include ledger plan input")
	}

	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp 067 failed: %v", err)
	}
	if !tableExists(t, ctx, pool, "smartaccounts_sync_controls") {
		t.Fatal("expected 067 to create the public SmartAccounts sync-control table")
	}
	execSQL(t, ctx, pool, `
		INSERT INTO smartaccounts_sync_controls (tenant_id, source_company_id, source_company_name, secret_reference)
		VALUES
			((SELECT id FROM tenants WHERE schema_name = 'tenant_import_legacy'), 'sa-company-a', 'Source A', 'secret-ref://sa-bridge/a'),
			((SELECT id FROM tenants WHERE schema_name = 'tenant_import_legacy'), 'sa-company-b', 'Source B', 'secret-ref://sa-bridge/b');
	`)
	if _, err := pool.Exec(ctx, `
		INSERT INTO smartaccounts_sync_controls (tenant_id, source_company_id, source_company_name, secret_reference)
		VALUES ((SELECT id FROM tenants WHERE schema_name = 'tenant_import_legacy'), 'sa-company-a', 'Duplicate', 'secret-ref://sa-bridge/duplicate');
	`); err == nil {
		t.Fatal("expected 067 composite tenant/source binding to reject a duplicate source")
	}
	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown 067 failed: %v", err)
	}
	if tableExists(t, ctx, pool, "smartaccounts_sync_controls") {
		t.Fatal("expected 067 rollback to remove only the public sync-control table")
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown 066 failed: %v", err)
	}
	if schemaColumnExists(t, ctx, pool, "tenant_import_legacy", "import_sessions", "ledger_plan_input") {
		t.Fatal("expected 066 rollback to remove ledger plan input from existing tenants")
	}
	if !schemaColumnExists(t, ctx, pool, "tenant_import_legacy", "import_sessions", "ledger_verification") {
		t.Fatal("066 rollback must retain ledger verification from 065")
	}
	execSQL(t, ctx, pool, `SELECT create_tenant_schema('tenant_import_after_plan_rollback')`)
	if !schemaColumnExists(t, ctx, pool, "tenant_import_after_plan_rollback", "import_sessions", "ledger_verification") {
		t.Fatal("expected 066 rollback to retain the 065 bootstrap")
	}
	if schemaColumnExists(t, ctx, pool, "tenant_import_after_plan_rollback", "import_sessions", "ledger_plan_input") {
		t.Fatal("expected 066 rollback to remove planning metadata from future tenant bootstrap")
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown 065 failed: %v", err)
	}
	if schemaColumnExists(t, ctx, pool, "tenant_import_legacy", "import_sessions", "ledger_verification") {
		t.Fatal("expected 065 rollback to remove ledger verification from existing tenants")
	}
	execSQL(t, ctx, pool, `SELECT create_tenant_schema('tenant_import_after_rollback')`)
	if schemaColumnExists(t, ctx, pool, "tenant_import_after_rollback", "import_sessions", "ledger_verification") {
		t.Fatal("expected 065 rollback to restore the migration-064 tenant bootstrap")
	}
}

func TestBrowserOnboardingBatchMigrationsUpgradeRetryRollbackAndFreshSchema(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensure migrations table: %v", err)
	}
	execSQL(t, ctx, pool, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE public.tenants (
			id UUID PRIMARY KEY,
			schema_name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE public.smartaccounts_browser_pairings (
			id UUID PRIMARY KEY
		);
	`)

	dir := t.TempDir()
	for _, name := range []string{
		"081_smartaccounts_browser_onboarding_batches.up.sql",
		"081_smartaccounts_browser_onboarding_batches.down.sql",
		"082_smartaccounts_browser_batch_workflows.up.sql",
		"082_smartaccounts_browser_batch_workflows.down.sql",
		"083_smartaccounts_browser_batch_capture_runs.up.sql",
		"083_smartaccounts_browser_batch_capture_runs.down.sql",
	} {
		copyRepositoryMigration(t, dir, name)
	}

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("upgrade 081→083: %v", err)
	}
	for _, tableName := range []string{
		"smartaccounts_browser_onboarding_catalog_receipts",
		"smartaccounts_browser_onboarding_batches",
		"smartaccounts_browser_onboarding_batch_outcomes",
		"smartaccounts_browser_batch_workflows",
		"smartaccounts_browser_batch_source_workflows",
	} {
		if !tableExists(t, ctx, pool, tableName) {
			t.Fatalf("expected %s after 081→083 upgrade", tableName)
		}
	}
	if !schemaColumnExists(t, ctx, pool, "public", "smartaccounts_browser_batch_source_workflows", "capture_run_id") {
		t.Fatal("083 upgrade must add durable non-secret capture_run_id")
	}

	// Exercise the real cross-migration foreign keys using only safe control
	// metadata: opaque selectors, tenant IDs, and digests—not browser/session
	// material or financial source data.
	execSQL(t, ctx, pool, `
		INSERT INTO public.tenants (id, schema_name)
		VALUES ('11111111-1111-4111-8111-111111111111', 'tenant_batch_migration');
		INSERT INTO public.smartaccounts_browser_onboarding_catalog_receipts (
			id, workflow_id, owner_id, token_sha256, nonce_sha256,
			schema_version, intent_version, source_id_version, digest_algorithm,
			status, catalog_sha256, catalog_count, companies, observed_at,
			expires_at, receipt_expires_at, accepted_at
		) VALUES (
			'22222222-2222-4222-8222-222222222222',
			'33333333-3333-4333-8333-333333333333',
			'owner-1', repeat('a', 64), repeat('b', 64),
			'smartaccounts-browser-source-catalog-v1',
			'smartaccounts-browser-source-catalog-intent-v1', 'sa-browser-v1', 'sha256',
			'ACCEPTED', repeat('c', 64), 1,
			'[{"source_company_id":"sa-browser-v1-1","display_name":"Company One"}]'::jsonb,
			now(), now() + interval '2 minutes', now() + interval '10 minutes', now()
		);
		INSERT INTO public.smartaccounts_browser_onboarding_batches (
			id, owner_id, catalog_receipt_id, relay_observed_at, mode,
			selected_sources, observed_source_ids, observed_sources_sha256,
			manifest_sha256, status
		) VALUES (
			'44444444-4444-4444-8444-444444444444', 'owner-1',
			'22222222-2222-4222-8222-222222222222', now(), 'all',
			'[{"source_company_id":"sa-browser-v1-1","source_company_name":"Company One"}]'::jsonb,
			'["sa-browser-v1-1"]'::jsonb, repeat('c', 64), repeat('d', 64), 'PENDING'
		);
		INSERT INTO public.smartaccounts_browser_batch_workflows (
			batch_id, owner_id, schema_version, history_from,
			preparatory_manifest_sha256, preparatory_consented_at
		) VALUES (
			'44444444-4444-4444-8444-444444444444', 'owner-1',
			'smartaccounts-browser-batch-workflow-v1', '2017-01-01', repeat('e', 64), now()
		);
		INSERT INTO public.smartaccounts_browser_batch_source_workflows (
			batch_id, source_company_id, tenant_id, ordinal, phase
		) VALUES (
			'44444444-4444-4444-8444-444444444444', 'sa-browser-v1-1',
			'11111111-1111-4111-8111-111111111111', 0, 'PAIRED'
		);
	`)

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("081→083 retry must be a no-op: %v", err)
	}
	var sourceWorkflows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.smartaccounts_browser_batch_source_workflows`).Scan(&sourceWorkflows); err != nil {
		t.Fatalf("count source workflows after retry: %v", err)
	}
	if sourceWorkflows != 1 {
		t.Fatalf("expected retry to preserve one source workflow, got %d", sourceWorkflows)
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("rollback 083: %v", err)
	}
	if schemaColumnExists(t, ctx, pool, "public", "smartaccounts_browser_batch_source_workflows", "capture_run_id") {
		t.Fatal("083 rollback must remove only capture_run_id")
	}
	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("rollback 082: %v", err)
	}
	if tableExists(t, ctx, pool, "smartaccounts_browser_batch_workflows") || tableExists(t, ctx, pool, "smartaccounts_browser_batch_source_workflows") {
		t.Fatal("082 rollback must remove only its workflow tables")
	}
	if !tableExists(t, ctx, pool, "smartaccounts_browser_onboarding_batches") {
		t.Fatal("082 rollback must retain immutable 081 batches")
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("rollback 081: %v", err)
	}
	if tableExists(t, ctx, pool, "smartaccounts_browser_onboarding_catalog_receipts") || tableExists(t, ctx, pool, "smartaccounts_browser_onboarding_batches") {
		t.Fatal("081 rollback must remove catalog and batch controls")
	}

	// Reapply to the rolled-back database as a fresh schema check. Both
	// migrations use idempotent table/index DDL and must not depend on stale
	// metadata from a previous selected/all run.
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("fresh 081→083 reapply: %v", err)
	}
	if !tableExists(t, ctx, pool, "smartaccounts_browser_batch_source_workflows") {
		t.Fatal("fresh 081→083 reapply must create the source workflow table")
	}
	if !schemaColumnExists(t, ctx, pool, "public", "smartaccounts_browser_batch_source_workflows", "capture_run_id") {
		t.Fatal("fresh 081→083 reapply must add capture_run_id")
	}
}

func TestBrowserGeneralLedgerContractV2MigrationAcceptsV2WithoutInvalidatingV1Evidence(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensure migrations table: %v", err)
	}
	execSQL(t, ctx, pool, `CREATE TABLE public.tenants (id UUID PRIMARY KEY, schema_name TEXT NOT NULL UNIQUE);`)
	execSQL(t, ctx, pool, `INSERT INTO public.tenants (id, schema_name) VALUES ('11111111-1111-4111-8111-111111111111', 'tenant_browser_gl');`)

	dir := t.TempDir()
	for _, name := range []string{
		"074_smartaccounts_browser_capture_authorizations.up.sql",
		"074_smartaccounts_browser_capture_authorizations.down.sql",
		"078_smartaccounts_browser_discovery_receipts.up.sql",
		"078_smartaccounts_browser_discovery_receipts.down.sql",
		"084_smartaccounts_browser_general_ledger_contract_v2.up.sql",
		"084_smartaccounts_browser_general_ledger_contract_v2.down.sql",
	} {
		copyRepositoryMigration(t, dir, name)
	}
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("upgrade 074/078/084: %v", err)
	}

	insertCapture := func(runID, version string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO public.smartaccounts_browser_capture_authorizations
				(run_id, tenant_id, source_company_id, manifest_version, scope, token_sha256, created_by, expires_at)
			VALUES ($1, '11111111-1111-4111-8111-111111111111', 'sa-browser-v1-1', $2, '{"mode":"partial_browser_capture"}'::jsonb, repeat('a', 64), 'owner-1', now() + interval '10 minutes')
		`, runID, version)
		return err
	}
	resourceIDs := `(SELECT jsonb_agg('resource-' || value) FROM generate_series(1, 31) AS value)`
	insertDiscovery := func(discoveryID, version string) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO public.smartaccounts_browser_discovery_authorizations
				(discovery_id, tenant_id, source_company_id, manifest_version, contract_version, resource_ids, metadata_only_consent_confirmed, header_probe_consent_confirmed, consented_at, created_by, expires_at)
			VALUES ($1, '11111111-1111-4111-8111-111111111111', 'sa-browser-v1-1', $2, 'smartaccounts-brave-discovery-contract-v1', `+resourceIDs+`, true, false, now(), 'owner-1', now() + interval '10 minutes')
		`, discoveryID, version)
		return err
	}
	for index, version := range []string{"smartaccounts-brave-ui-v1", "smartaccounts-brave-ui-v2"} {
		if err := insertCapture(fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1), version); err != nil {
			t.Fatalf("capture authorization %s must remain readable/insertable: %v", version, err)
		}
		if err := insertDiscovery(fmt.Sprintf("00000000-0000-4000-9000-%012d", index+1), version); err != nil {
			t.Fatalf("discovery authorization %s must remain readable/insertable: %v", version, err)
		}
	}
	if err := insertCapture("00000000-0000-4000-8000-999999999999", "smartaccounts-brave-ui-v3"); err == nil {
		t.Fatal("084 must still reject an unknown browser manifest version")
	}
	if err := insertDiscovery("00000000-0000-4000-9000-999999999999", "smartaccounts-brave-ui-v3"); err == nil {
		t.Fatal("084 must still reject an unknown discovery manifest version")
	}
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("074/078/084 retry must be a no-op: %v", err)
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("084 forward-only rollback marker: %v", err)
	}
	if err := insertCapture("00000000-0000-4000-8000-888888888888", "smartaccounts-brave-ui-v2"); err != nil {
		t.Fatalf("084 rollback marker must not invalidate v2 audit evidence: %v", err)
	}
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("084 fresh reapply after marker rollback: %v", err)
	}
}

func TestSmartAccountsReconciliationMigrationAddsActiveSourceIdentityForExistingAndFreshExecutorSchemas(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensure migrations table: %v", err)
	}
	execSQL(t, ctx, pool, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE public.tenants (id UUID PRIMARY KEY, schema_name TEXT NOT NULL UNIQUE);
		CREATE TABLE public.smartaccounts_browser_onboarding_batches (id UUID PRIMARY KEY);
		INSERT INTO public.tenants (id, schema_name) VALUES ('11111111-1111-4111-8111-111111111111', 'tenant_reconciliation_legacy');
		CREATE SCHEMA tenant_reconciliation_legacy;
		CREATE TABLE tenant_reconciliation_legacy.journal_entries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, source_type TEXT, source_id UUID,
			status TEXT NOT NULL DEFAULT 'DRAFT'
		);
		CREATE TABLE tenant_reconciliation_legacy.smartaccounts_financial_postings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, provider TEXT NOT NULL,
			source_company_id TEXT NOT NULL, resource TEXT NOT NULL, external_id TEXT NOT NULL, revision TEXT NOT NULL,
			status TEXT NOT NULL, journal_entry_id UUID, package_id TEXT NOT NULL, preview_id UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(), applied_at TIMESTAMPTZ NULL,
			UNIQUE (tenant_id, provider, source_company_id, resource, external_id)
		);
	`)
	dir := t.TempDir()
	copyRepositoryMigration(t, dir, "087_smartaccounts_reconciliation_receipts.up.sql")
	copyRepositoryMigration(t, dir, "087_smartaccounts_reconciliation_receipts.down.sql")
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("upgrade 087: %v", err)
	}
	if !schemaColumnExists(t, ctx, pool, "public", "smartaccounts_gl_apply_receipt_identities", "reservation_id") {
		t.Fatal("087 receipt identity must retain the reservation source ID used to prove the exact target journal")
	}
	if !schemaColumnExists(t, ctx, pool, "public", "smartaccounts_gl_tolerance_policies", "algorithm_version") {
		t.Fatal("087 immutable tolerance policy must retain its server-owned algorithm version")
	}
	for _, column := range []string{"applied_by", "reserved_by"} {
		if !schemaColumnExists(t, ctx, pool, "tenant_reconciliation_legacy", "smartaccounts_financial_postings", column) {
			t.Fatalf("087 must add %s to deployed executor state", column)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_reconciliation_legacy.journal_entries (tenant_id, source_type, source_id, status)
		VALUES ('11111111-1111-4111-8111-111111111111', 'SMARTACCOUNTS_GL', '22222222-2222-4222-8222-222222222222', 'DRAFT')`); err != nil {
		t.Fatalf("insert active source identity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_reconciliation_legacy.journal_entries (tenant_id, source_type, source_id, status)
		VALUES ('11111111-1111-4111-8111-111111111111', 'SMARTACCOUNTS_GL', '22222222-2222-4222-8222-222222222222', 'POSTED')`); err == nil {
		t.Fatal("087 must reject a second active SmartAccounts GL source identity")
	}
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("087 retry must be idempotent: %v", err)
	}

	// The current bootstrap helper creates executor state before the 087 helper
	// runs. Exercise that fresh sequence without source data or financial rows.
	execSQL(t, ctx, pool, `
		CREATE SCHEMA tenant_reconciliation_fresh;
		CREATE TABLE tenant_reconciliation_fresh.journal_entries (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, source_type TEXT, source_id UUID,
			status TEXT NOT NULL DEFAULT 'DRAFT'
		);
		SELECT add_smartaccounts_executor_tables('tenant_reconciliation_fresh');
		SELECT add_smartaccounts_executor_reconciliation_columns('tenant_reconciliation_fresh');
	`)
	for _, column := range []string{"applied_by", "reserved_by"} {
		if !schemaColumnExists(t, ctx, pool, "tenant_reconciliation_fresh", "smartaccounts_financial_postings", column) {
			t.Fatalf("fresh executor bootstrap must include %s", column)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_reconciliation_fresh.journal_entries (tenant_id, source_type, source_id, status)
		VALUES ('11111111-1111-4111-8111-111111111111', 'SMARTACCOUNTS_GL', '33333333-3333-4333-8333-333333333333', 'DRAFT')`); err != nil {
		t.Fatalf("insert fresh source identity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_reconciliation_fresh.journal_entries (tenant_id, source_type, source_id, status)
		VALUES ('11111111-1111-4111-8111-111111111111', 'SMARTACCOUNTS_GL', '33333333-3333-4333-8333-333333333333', 'POSTED')`); err == nil {
		t.Fatal("fresh executor bootstrap must reject duplicate active SmartAccounts GL source identities")
	}
}

func createImportSessionBootstrapStubs(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, functionName := range []string{
		"create_accounting_tables",
		"add_journal_entry_post_reason",
		"add_vat_columns_to_journal_lines",
		"add_payment_reversal_columns",
		"add_reconciliation_tables_to_schema",
		"add_recurring_tables_to_schema",
		"add_quotes_and_orders_tables",
		"add_fixed_assets_tables",
		"add_fixed_asset_disposal_journal_links",
		"create_inventory_tables",
		"add_inventory_movement_tracking_metadata",
		"add_inventory_lot_reservations",
		"add_payroll_tables",
		"add_leave_management_tables",
		"create_email_tables_only",
		"add_kmd_tables_to_schema",
		"fix_email_log_schema",
		"add_reminder_rules_to_schema",
		"sync_email_template_type_constraint",
		"add_interest_tables",
		"add_document_tables",
		"add_document_review_workflow",
		"add_bank_transaction_review_columns",
		"add_close_pack_document_entity",
		"add_order_stock_reservations",
		"add_journal_entry_evidence_requirement",
		"add_journal_entry_templates",
		"add_journal_entry_template_recurrence",
		"add_bank_match_rules",
		"add_invoice_vat_treatment",
		"add_expense_tables",
		"add_commercial_document_entities",
		"add_leave_record_document_entity",
		"add_tax_declaration_document_entities",
		"add_document_lifecycle_workflow",
		"add_document_legal_hold_workflow",
		"add_document_lifecycle_integrity",
		"add_cost_center_tables",
		"add_migration_execution_run_tables",
		"add_financial_report_indexes",
	} {
		execSQL(t, ctx, pool, fmt.Sprintf(`
			CREATE OR REPLACE FUNCTION %s(schema_name TEXT) RETURNS VOID AS $$
			BEGIN
			END;
			$$ LANGUAGE plpgsql;
		`, functionName))
	}
}

func TestEmailTemplateTypeMigrationAllowsQuoteAndOrderTemplates(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			schema_name TEXT NOT NULL UNIQUE,
			is_active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE SCHEMA tenant_email_types;
		INSERT INTO tenants (schema_name, is_active) VALUES ('tenant_email_types', true);

		CREATE TABLE tenant_email_types.email_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			template_type VARCHAR(50) NOT NULL
				CONSTRAINT email_templates_template_type_check
				CHECK (template_type IN (
					'INVOICE_SEND',
					'INVOICE_REMINDER',
					'PAYMENT_RECEIPT',
					'OVERDUE_REMINDER',
					'WELCOME',
					'CUSTOM',
					'PAYMENT_DUE_SOON',
					'PAYMENT_DUE_TODAY'
				)),
			subject TEXT NOT NULL,
			body_html TEXT NOT NULL,
			body_text TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE (tenant_id, template_type)
		);
	`)

	dir := t.TempDir()
	copyRepositoryMigration(t, dir, "053_quote_order_email_template_types.up.sql")
	copyRepositoryMigration(t, dir, "053_quote_order_email_template_types.down.sql")

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		INSERT INTO tenant_email_types.email_templates (tenant_id, template_type, subject, body_html, body_text)
		VALUES
			((SELECT id FROM tenants WHERE schema_name = 'tenant_email_types'), 'QUOTE_SEND', 'Quote', '<p>Quote</p>', 'Quote'),
			((SELECT id FROM tenants WHERE schema_name = 'tenant_email_types'), 'ORDER_CONFIRM', 'Order', '<p>Order</p>', 'Order');
	`)

	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_email_types.email_templates (tenant_id, template_type, subject, body_html)
		VALUES ((SELECT id FROM tenants WHERE schema_name = 'tenant_email_types'), 'UNKNOWN_TEMPLATE', 'Unknown', '<p>Unknown</p>')
	`); err == nil {
		t.Fatalf("expected unknown template type to be rejected")
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown failed: %v", err)
	}

	var remaining int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM tenant_email_types.email_templates
		WHERE template_type IN ('QUOTE_SEND', 'ORDER_CONFIRM')
	`).Scan(&remaining); err != nil {
		t.Fatalf("count quote/order templates after rollback: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected quote/order templates to be removed on rollback, got %d", remaining)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_email_types.email_templates (tenant_id, template_type, subject, body_html)
		VALUES ((SELECT id FROM tenants WHERE schema_name = 'tenant_email_types'), 'QUOTE_SEND', 'Quote', '<p>Quote</p>')
	`); err == nil {
		t.Fatalf("expected quote template type to be rejected after rollback")
	}
}

func TestEmailTemplateTypeMigrationAllowsDocumentRetentionReminderTemplate(t *testing.T) {
	pool := setupMigrationTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE TABLE tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			schema_name TEXT NOT NULL UNIQUE,
			is_active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE SCHEMA tenant_retention_email_types;
		INSERT INTO tenants (schema_name, is_active) VALUES ('tenant_retention_email_types', true);

		CREATE TABLE tenant_retention_email_types.email_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			template_type VARCHAR(50) NOT NULL
				CONSTRAINT email_templates_template_type_check
				CHECK (template_type IN (
					'INVOICE_SEND',
					'INVOICE_REMINDER',
					'PAYMENT_RECEIPT',
					'OVERDUE_REMINDER',
					'WELCOME',
					'CUSTOM',
					'PAYMENT_DUE_SOON',
					'PAYMENT_DUE_TODAY',
					'QUOTE_SEND',
					'ORDER_CONFIRM'
				)),
			subject TEXT NOT NULL,
			body_html TEXT NOT NULL,
			body_text TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE (tenant_id, template_type)
		);
	`)

	dir := t.TempDir()
	copyRepositoryMigration(t, dir, "054_document_retention_reminder_template.up.sql")
	copyRepositoryMigration(t, dir, "054_document_retention_reminder_template.down.sql")

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp failed: %v", err)
	}

	execSQL(t, ctx, pool, `
		INSERT INTO tenant_retention_email_types.email_templates (tenant_id, template_type, subject, body_html, body_text)
		VALUES ((SELECT id FROM tenants WHERE schema_name = 'tenant_retention_email_types'), 'DOCUMENT_RETENTION_REMINDER', 'Retention', '<p>Retention</p>', 'Retention');
	`)

	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_retention_email_types.email_templates (tenant_id, template_type, subject, body_html)
		VALUES ((SELECT id FROM tenants WHERE schema_name = 'tenant_retention_email_types'), 'UNKNOWN_TEMPLATE', 'Unknown', '<p>Unknown</p>')
	`); err == nil {
		t.Fatalf("expected unknown template type to be rejected")
	}

	if err := migrateDown(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateDown failed: %v", err)
	}

	var remaining int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM tenant_retention_email_types.email_templates
		WHERE template_type = 'DOCUMENT_RETENTION_REMINDER'
	`).Scan(&remaining); err != nil {
		t.Fatalf("count document retention reminder templates after rollback: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected document retention reminder templates to be removed on rollback, got %d", remaining)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO tenant_retention_email_types.email_templates (tenant_id, template_type, subject, body_html)
		VALUES ((SELECT id FROM tenants WHERE schema_name = 'tenant_retention_email_types'), 'DOCUMENT_RETENTION_REMINDER', 'Retention', '<p>Retention</p>')
	`); err == nil {
		t.Fatalf("expected document retention reminder template type to be rejected after rollback")
	}
}

func writeMigration(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write migration %s: %v", name, err)
	}
}

func copyRepositoryMigration(t *testing.T, dir, name string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatalf("failed to read repository migration %s: %v", name, err)
	}
	writeMigration(t, dir, name, string(content))
}

func execSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("failed to execute test sql: %v", err)
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

func schemaTableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_name = $2
	)`, schemaName, tableName).Scan(&exists); err != nil {
		t.Fatalf("failed to check schema table existence: %v", err)
	}
	return exists
}

func schemaColumnExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schemaName, tableName, columnName string) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
	)`, schemaName, tableName, columnName).Scan(&exists); err != nil {
		t.Fatalf("failed to check schema column existence: %v", err)
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
		baseURL = connStringFromPool(testutil.SetupTestDB(t))
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
