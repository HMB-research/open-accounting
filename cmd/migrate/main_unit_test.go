package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMigrationFileDiscoveryUnit(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"002_second.up.sql",
		"001_first.up.sql",
		"003_third.down.sql",
		"ignored.txt",
	} {
		writeUnitMigration(t, dir, name, "-- noop")
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
		t.Fatalf("unexpected up version: %s", version)
	}
	if version := extractVersion(filepath.Join(dir, "003_third.down.sql")); version != "003_third" {
		t.Fatalf("unexpected down version: %s", version)
	}
}

func TestImportSessionLedgerVerificationMigrationStaysForwardOnlyUnit(t *testing.T) {
	readMigration := func(t *testing.T, name string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(contents)
	}

	initial := readMigration(t, "064_import_sessions.up.sql")
	if strings.Contains(initial, "ledger_verification") {
		t.Fatal("deployed migration 064 must retain its original receipt schema")
	}

	upgrade := readMigration(t, "065_import_session_ledger_verification.up.sql")
	for _, expected := range []string{
		"CREATE OR REPLACE FUNCTION add_import_session_ledger_verification",
		"ADD COLUMN IF NOT EXISTS ledger_verification JSONB NOT NULL DEFAULT ''{}''::JSONB",
		"PERFORM add_import_session_ledger_verification(tenant_schema)",
		"PERFORM add_import_session_ledger_verification(schema_name)",
	} {
		if !strings.Contains(upgrade, expected) {
			t.Errorf("065 upgrade must contain %q", expected)
		}
	}

	rollback := readMigration(t, "065_import_session_ledger_verification.down.sql")
	for _, expected := range []string{
		"DROP COLUMN IF EXISTS ledger_verification",
		"DROP FUNCTION IF EXISTS add_import_session_ledger_verification(TEXT)",
		"PERFORM add_import_session_tables(schema_name)",
	} {
		if !strings.Contains(rollback, expected) {
			t.Errorf("065 rollback must contain %q", expected)
		}
	}
	if strings.Contains(rollback, "PERFORM add_import_session_ledger_verification(schema_name)") {
		t.Fatal("065 rollback must restore the migration-064 tenant bootstrap")
	}

	planUpgrade := readMigration(t, "066_import_session_ledger_plan_input.up.sql")
	for _, expected := range []string{
		"CREATE OR REPLACE FUNCTION add_import_session_ledger_plan_input",
		"ADD COLUMN IF NOT EXISTS ledger_plan_input JSONB NOT NULL DEFAULT ''[]''::JSONB",
		"PERFORM add_import_session_ledger_plan_input(tenant_schema)",
		"PERFORM add_import_session_ledger_plan_input(schema_name)",
	} {
		if !strings.Contains(planUpgrade, expected) {
			t.Errorf("066 upgrade must contain %q", expected)
		}
	}
	if strings.Contains(initial, "ledger_plan_input") || strings.Contains(upgrade, "ledger_plan_input") {
		t.Fatal("deployed migrations 064 and 065 must not be rewritten with 066 planning metadata")
	}

	planRollback := readMigration(t, "066_import_session_ledger_plan_input.down.sql")
	for _, expected := range []string{
		"DROP COLUMN IF EXISTS ledger_plan_input",
		"DROP FUNCTION IF EXISTS add_import_session_ledger_plan_input(TEXT)",
		"CREATE OR REPLACE FUNCTION add_import_session_tables",
	} {
		if !strings.Contains(planRollback, expected) {
			t.Errorf("066 rollback must contain %q", expected)
		}
	}
}

func TestSmartAccountsSyncControlMigrationUsesExplicitTenantSourceBindingsUnit(t *testing.T) {
	readMigration := func(t *testing.T, name string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(contents)
	}

	upgrade := readMigration(t, "067_smartaccounts_sync_controls.up.sql")
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_sync_controls",
		"PRIMARY KEY (tenant_id, source_company_id)",
		"secret_reference TEXT NOT NULL",
		"secret-ref|vault|op|sops",
	} {
		if !strings.Contains(upgrade, expected) {
			t.Errorf("067 upgrade must contain %q", expected)
		}
	}
	if strings.Contains(strings.ToLower(upgrade), "hold my beer") {
		t.Fatal("067 must not persist a hard-coded source-company default")
	}

	rollback := readMigration(t, "067_smartaccounts_sync_controls.down.sql")
	if !strings.Contains(rollback, "DROP TABLE IF EXISTS public.smartaccounts_sync_controls") {
		t.Fatal("067 rollback must drop only the SmartAccounts control table")
	}
}

func TestExternalImportArchiveDeliveryMigrationIsBoundedAndRestoresBootstrapUnit(t *testing.T) {
	readMigration := func(t *testing.T, name string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(contents)
	}

	upgrade := readMigration(t, "069_external_import_archive_delivery.up.sql")
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.import_delivery_nonces",
		"CREATE OR REPLACE FUNCTION add_external_import_delivery_tables",
		"external_import_record_chunks",
		"external_import_artifact_chunks",
		"octet_length(data) <= 1048576",
		"PERFORM add_external_import_delivery_tables(schema_name)",
	} {
		if !strings.Contains(upgrade, expected) {
			t.Errorf("069 upgrade must contain %q", expected)
		}
	}
	if strings.Contains(upgrade, "064_import_sessions") {
		t.Fatal("069 must not rewrite deployed import-session migration history")
	}
	if !strings.Contains(upgrade, "CONSTRAINT external_import_artifact_chunks_sequence_lt_chunk_count_check") {
		t.Fatal("069 must give its cross-column artifact sequence check a non-default constraint name")
	}
	if strings.Contains(upgrade, "CONSTRAINT external_import_artifact_chunks_sequence_check CHECK (sequence < chunk_count)") {
		t.Fatal("069 must not reuse PostgreSQL's default name for the column-level sequence check")
	}

	rollback := readMigration(t, "069_external_import_archive_delivery.down.sql")
	for _, expected := range []string{
		"CREATE OR REPLACE FUNCTION create_tenant_schema",
		"DROP TABLE IF EXISTS public.import_delivery_nonces",
		"DROP FUNCTION IF EXISTS add_external_import_delivery_tables(TEXT)",
	} {
		if !strings.Contains(rollback, expected) {
			t.Errorf("069 rollback must contain %q", expected)
		}
	}
	if strings.Contains(rollback, "PERFORM add_external_import_delivery_tables(schema_name)") {
		t.Fatal("069 rollback must not leave future tenant bootstrap calling a removed helper")
	}
}

func TestReminderRulesBootstrapCompatibilityMigrationUnit(t *testing.T) {
	readMigration := func(t *testing.T, name string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(contents)
	}

	upgrade := readMigration(t, "073_reminder_rules_bootstrap_compatibility.up.sql")
	for _, expected := range []string{
		"CREATE OR REPLACE FUNCTION add_reminder_rules(schema_name TEXT)",
		"PERFORM add_reminder_rules_to_schema(schema_name)",
	} {
		if !strings.Contains(upgrade, expected) {
			t.Errorf("073 upgrade must contain %q", expected)
		}
	}
	if strings.Contains(upgrade, "CREATE OR REPLACE FUNCTION create_tenant_schema") {
		t.Fatal("073 must preserve the existing tenant bootstrap and supply only a compatibility helper")
	}

	rollback := readMigration(t, "073_reminder_rules_bootstrap_compatibility.down.sql")
	if !strings.Contains(rollback, "DROP FUNCTION IF EXISTS add_reminder_rules(TEXT)") {
		t.Fatal("073 rollback must remove the compatibility helper")
	}
}

func TestBrowserDiscoveryReceiptMigrationRequiresTheFullServerOwnedManifestUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "078_smartaccounts_browser_discovery_receipts.up.sql"))
	if err != nil {
		t.Fatalf("read 078 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_discovery_authorizations",
		"PRIMARY KEY (tenant_id, discovery_id)",
		"source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'",
		"jsonb_array_length(resource_ids) = 31",
		"manifest_version = 'smartaccounts-brave-ui-v1'",
		"contract_version = 'smartaccounts-brave-discovery-contract-v1'",
		"contract_sha256 CHAR(64)",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("078 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"cookie", "credential", "header_name", "control_ids", "source_rows", "export_body"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("078 public schema must not persist %q", forbidden)
		}
	}

	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "078_smartaccounts_browser_discovery_receipts.down.sql"))
	if err != nil {
		t.Fatalf("read 078 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "DROP TABLE IF EXISTS public.smartaccounts_browser_discovery_authorizations") {
		t.Fatal("078 rollback must remove only the browser discovery receipt table")
	}
}

func TestBrowserGeneralLedgerContractV2MigrationPreservesHistoricalEvidenceUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "084_smartaccounts_browser_general_ledger_contract_v2.up.sql"))
	if err != nil {
		t.Fatalf("read 084 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"DROP CONSTRAINT IF EXISTS smartaccounts_browser_capture_manifest_version",
		"DROP CONSTRAINT IF EXISTS smartaccounts_browser_discovery_manifest_check",
		"smartaccounts-brave-ui-v1",
		"smartaccounts-brave-ui-v2",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("084 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"api_key", "api_secret", "cookie", "credential", "source_rows", "journal_entries_csv_v1"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("084 public schema must not persist %q", forbidden)
		}
	}

	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "084_smartaccounts_browser_general_ledger_contract_v2.down.sql"))
	if err != nil {
		t.Fatalf("read 084 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "SELECT 1") || strings.Contains(string(rollback), "DROP CONSTRAINT") {
		t.Fatal("084 rollback must remain a forward-only no-op so v2 audit evidence remains readable")
	}
}

func TestBrowserMasterDetailAuthorizationMigrationUsesImmutableGenerationsUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "085_smartaccounts_browser_master_detail_authorizations.up.sql"))
	if err != nil {
		t.Fatalf("read 085 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_master_detail_authorizations",
		"batch_id UUID NOT NULL",
		"UNIQUE (tenant_id, batch_id, resource_id)",
		"manifest_version = 'smartaccounts-browser-master-detail-v1'",
		"clients_detail_v1",
		"vendors_detail_v1",
		"articles_detail_v1",
		"expires_at > created_at",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("085 upgrade must contain %q", expected)
		}
	}
	if strings.Contains(contents, "UNIQUE (tenant_id, source_company_id, resource_id, snapshot_date)") {
		t.Fatal("085 must permit an explicitly owner-confirmed same-day refresh as a new batch generation")
	}
	for _, forbidden := range []string{"capture_token", "cookie", "credential", "source_rows", "ndjson"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("085 public schema must not persist %q", forbidden)
		}
	}
	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "085_smartaccounts_browser_master_detail_authorizations.down.sql"))
	if err != nil {
		t.Fatalf("read 085 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "SELECT 1") {
		t.Fatal("085 rollback must remain a forward-only no-op for deployed audit metadata")
	}
}

func TestBrowserBatchOptionalHeaderProbeConsentMigrationDefaultsSafelyUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "086_browser_batch_optional_header_probe_consent.up.sql"))
	if err != nil {
		t.Fatalf("read 086 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"ALTER TABLE public.smartaccounts_browser_batch_workflows",
		"ADD COLUMN IF NOT EXISTS header_probe_consent_confirmed BOOLEAN NOT NULL DEFAULT FALSE",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("086 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"header_value", "cookie", "credential", "capture_token"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("086 public schema must not persist %q", forbidden)
		}
	}
	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "086_browser_batch_optional_header_probe_consent.down.sql"))
	if err != nil {
		t.Fatalf("read 086 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "SELECT 1") {
		t.Fatal("086 rollback must remain a forward-only no-op for persisted consent audit metadata")
	}
}

func TestSmartAccountsReconciliationMigrationHasCrashResumeIdentityGuardUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "087_smartaccounts_reconciliation_receipts.up.sql"))
	if err != nil {
		t.Fatalf("read 087 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"smartaccounts_gl_apply_receipts",
		"reservation_id UUID NOT NULL",
		"algorithm_version TEXT NOT NULL",
		"smartaccounts-exact-match-v1",
		"smartaccounts_gl_tolerance_policies",
		"preview_sha256 CHAR(64) NOT NULL",
		"ADD COLUMN IF NOT EXISTS reserved_by TEXT NULL",
		"smartaccounts_gl_active_source_identity_unique",
		"WHERE source_type = ''SMARTACCOUNTS_GL'' AND source_id IS NOT NULL AND status <> ''VOIDED''",
		"PERFORM add_smartaccounts_executor_reconciliation_columns(schema_name)",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("087 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"proof_payload", "source_rows", "monetary_amount", "capture_token", "cookie_value"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("087 public schema must not persist %q", forbidden)
		}
	}
	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "087_smartaccounts_reconciliation_receipts.down.sql"))
	if err != nil {
		t.Fatalf("read 087 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "SELECT 1") {
		t.Fatal("087 rollback must remain forward-only for reconciliation audit evidence")
	}
}

func TestBrowserCommercialDetailMigrationStoresOnlyRelayControlMetadataUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "088_smartaccounts_browser_commercial_detail_authorizations.up.sql"))
	if err != nil {
		t.Fatalf("read 088 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_commercial_detail_authorizations",
		"UNIQUE (tenant_id, batch_id, source_company_id, resource_id)",
		"manifest_version = 'smartaccounts-browser-commercial-detail-v1'",
		"client_invoices_detail_v1",
		"bank_payments_detail_v1",
		"list_selector_required",
		"token_sha256 CHAR(64) NOT NULL",
		"bridge_started_at TIMESTAMPTZ",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("088 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"capture_token", "contract jsonb", "source_rows", "cookie_value", "credential_value", "monetary_amount"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("088 public schema must not persist %q", forbidden)
		}
	}
	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "088_smartaccounts_browser_commercial_detail_authorizations.down.sql"))
	if err != nil {
		t.Fatalf("read 088 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "DROP TABLE IF EXISTS public.smartaccounts_browser_commercial_detail_authorizations") {
		t.Fatal("088 rollback must target only the new commercial relay table")
	}
}

func TestFullClaimDomainEvidenceMigrationIsBoundedAndForwardOnlyUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "089_smartaccounts_full_claim_domain_evidence.up.sql"))
	if err != nil {
		t.Fatalf("read 089 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"CREATE TABLE IF NOT EXISTS public.smartaccounts_full_claim_domain_evidence",
		"batch_id UUID NOT NULL",
		"tenant_id UUID NOT NULL",
		"source_company_id TEXT NOT NULL",
		"package_id TEXT NOT NULL",
		"scope_sha256 CHAR(64) NOT NULL",
		"reconciliation_evidence_sha256 CHAR(64) NOT NULL",
		"plan_version = 'smartaccounts-full-claim-domain-plan-v1'",
		"live_source_validated AND schema_validated AND completeness_validated",
		"reconciliation_validated AND tombstones_resolved AND accountant_attested",
		"UNIQUE (batch_id, tenant_id, source_company_id, package_id, scope_sha256, reconciliation_evidence_sha256, plan_version, domain_id)",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("089 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"source_rows", "monetary_amount", "cookie", "credential", "request_body", "response_body", "free_form_note"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("089 public schema must not persist %q", forbidden)
		}
	}

	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "089_smartaccounts_full_claim_domain_evidence.down.sql"))
	if err != nil {
		t.Fatalf("read 089 rollback: %v", err)
	}
	if !strings.Contains(string(rollback), "SELECT 1") {
		t.Fatal("089 rollback must remain forward-only so historical full-claim evidence remains readable")
	}
}

func TestReferenceMasterStateMigrationIsTenantScopedAndBootstrappedUnit(t *testing.T) {
	upgrade, err := os.ReadFile(filepath.Join("..", "..", "migrations", "079_smartaccounts_reference_master_state.up.sql"))
	if err != nil {
		t.Fatalf("read 079 upgrade: %v", err)
	}
	contents := string(upgrade)
	for _, expected := range []string{
		"CREATE OR REPLACE FUNCTION add_smartaccounts_reference_master_tables",
		"smartaccounts_reference_previews",
		"smartaccounts_reference_identities",
		"UNIQUE (tenant_id, package_id, preview_sha256)",
		"UNIQUE (tenant_id, provider, source_company_id, entity_type, external_id)",
		"PERFORM add_smartaccounts_reference_master_tables(tenant_schema)",
		"PERFORM add_smartaccounts_reference_master_tables(schema_name)",
	} {
		if !strings.Contains(contents, expected) {
			t.Errorf("079 upgrade must contain %q", expected)
		}
	}
	for _, forbidden := range []string{"journal_entries", "invoices", "payments"} {
		if strings.Contains(strings.ToLower(contents), forbidden) {
			t.Errorf("079 must not create financial posting state for %q", forbidden)
		}
	}

	rollback, err := os.ReadFile(filepath.Join("..", "..", "migrations", "079_smartaccounts_reference_master_state.down.sql"))
	if err != nil {
		t.Fatalf("read 079 rollback: %v", err)
	}
	for _, expected := range []string{
		"DROP TABLE IF EXISTS %I.smartaccounts_reference_identities",
		"DROP TABLE IF EXISTS %I.smartaccounts_reference_previews",
		"DROP FUNCTION IF EXISTS add_smartaccounts_reference_master_tables(TEXT)",
	} {
		if !strings.Contains(string(rollback), expected) {
			t.Errorf("079 rollback must contain %q", expected)
		}
	}
}

func TestEnsureAndGetAppliedMigrationsUnit(t *testing.T) {
	ctx := context.Background()
	pool := newFakeMigrationPool(map[string]bool{
		"001_initial": true,
		"002_next":    true,
	})

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		t.Fatalf("ensureMigrationsTable failed: %v", err)
	}
	if len(pool.execs) != 1 || !strings.Contains(pool.execs[0], "CREATE TABLE IF NOT EXISTS schema_migrations") {
		t.Fatalf("expected migrations table DDL, got %v", pool.execs)
	}

	applied, err := getAppliedMigrations(ctx, pool)
	if err != nil {
		t.Fatalf("getAppliedMigrations failed: %v", err)
	}
	if !applied["001_initial"] || !applied["002_next"] {
		t.Fatalf("expected applied migrations to be returned, got %v", applied)
	}

	pool.rowsErr = errors.New("row iteration failed")
	if _, err := getAppliedMigrations(ctx, pool); err == nil || !strings.Contains(err.Error(), "row iteration failed") {
		t.Fatalf("expected rows.Err to be returned, got %v", err)
	}
}

func TestGetAppliedMigrationsErrorBranchesUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("query error", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		pool.queryErr = errors.New("query unavailable")

		if _, err := getAppliedMigrations(ctx, pool); err == nil || !strings.Contains(err.Error(), "query unavailable") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("scan error", func(t *testing.T) {
		pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
		pool.scanErr = errors.New("scan failed")

		if _, err := getAppliedMigrations(ctx, pool); err == nil || !strings.Contains(err.Error(), "scan failed") {
			t.Fatalf("expected scan error, got %v", err)
		}
	})
}

func TestGetMigrationFilesMalformedPatternUnit(t *testing.T) {
	if _, err := getMigrationFiles("[", ".up"); err == nil {
		t.Fatal("expected malformed glob pattern to fail")
	}
}

func TestMigrateUpHonorsStepsSkipsAndNoopsUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_initial.up.sql", "SELECT 1;")
	writeUnitMigration(t, dir, "002_second.up.sql", "SELECT 2;")
	writeUnitMigration(t, dir, "003_third.up.sql", "SELECT 3;")

	pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
	if err := migrateUp(ctx, pool, dir, 1); err != nil {
		t.Fatalf("migrateUp step-limited failed: %v", err)
	}
	if !pool.applied["002_second"] {
		t.Fatalf("expected second migration to be applied, got %v", pool.applied)
	}
	if pool.applied["003_third"] {
		t.Fatalf("expected third migration to remain pending with steps=1")
	}
	if got := len(pool.txs); got != 1 {
		t.Fatalf("expected 1 transaction, got %d", got)
	}
	if !pool.txs[0].committed || pool.txs[0].rolledBack {
		t.Fatalf("expected first transaction to commit cleanly: %+v", pool.txs[0])
	}

	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp remaining migrations failed: %v", err)
	}
	if !pool.applied["003_third"] {
		t.Fatalf("expected third migration to be applied, got %v", pool.applied)
	}

	txCount := len(pool.txs)
	if err := migrateUp(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateUp no-op failed: %v", err)
	}
	if len(pool.txs) != txCount {
		t.Fatalf("expected no transaction for already-applied migrations")
	}
}

func TestMigrateUpRollsBackFailedMigrationUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_bad.up.sql", "SELECT fail_up;")

	pool := newFakeMigrationPool(nil)
	pool.txErrContains = "fail_up"
	pool.txErr = errors.New("boom")

	err := migrateUp(ctx, pool, dir, 0)
	if err == nil || !strings.Contains(err.Error(), "execute migration 001_bad") {
		t.Fatalf("expected execute migration error, got %v", err)
	}
	if pool.applied["001_bad"] {
		t.Fatalf("failed migration should not be recorded as applied")
	}
	if got := len(pool.txs); got != 1 {
		t.Fatalf("expected 1 transaction, got %d", got)
	}
	if !pool.txs[0].rolledBack || pool.txs[0].committed {
		t.Fatalf("expected failed migration to roll back: %+v", pool.txs[0])
	}
}

func TestMigrateUpErrorBranchesUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("applied query error", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		pool.queryErr = errors.New("query unavailable")

		err := migrateUp(ctx, pool, t.TempDir(), 0)
		if err == nil || !strings.Contains(err.Error(), "get applied migrations") {
			t.Fatalf("expected applied migration query error, got %v", err)
		}
	})

	t.Run("glob error", func(t *testing.T) {
		err := migrateUp(ctx, newFakeMigrationPool(nil), "[", 0)
		if err == nil || !strings.Contains(err.Error(), "get migration files") {
			t.Fatalf("expected migration file glob error, got %v", err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "001_bad.up.sql"), 0o755); err != nil {
			t.Fatalf("failed to create directory migration: %v", err)
		}

		err := migrateUp(ctx, newFakeMigrationPool(nil), dir, 0)
		if err == nil || !strings.Contains(err.Error(), "read migration file") {
			t.Fatalf("expected migration read error, got %v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.up.sql", "SELECT 1;")
		pool := newFakeMigrationPool(nil)
		pool.beginErr = errors.New("begin failed")

		err := migrateUp(ctx, pool, dir, 0)
		if err == nil || !strings.Contains(err.Error(), "begin transaction") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("record error", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.up.sql", "SELECT 1;")
		pool := newFakeMigrationPool(nil)
		pool.txErrContains = "INSERT INTO schema_migrations"
		pool.txErr = errors.New("insert failed")

		err := migrateUp(ctx, pool, dir, 0)
		if err == nil || !strings.Contains(err.Error(), "record migration 001_initial") {
			t.Fatalf("expected record migration error, got %v", err)
		}
		if got := len(pool.txs); got != 1 || !pool.txs[0].rolledBack {
			t.Fatalf("expected record failure rollback, txs=%+v", pool.txs)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.up.sql", "SELECT 1;")
		pool := newFakeMigrationPool(nil)
		pool.commitErr = errors.New("commit failed")

		err := migrateUp(ctx, pool, dir, 0)
		if err == nil || !strings.Contains(err.Error(), "commit migration 001_initial") {
			t.Fatalf("expected commit migration error, got %v", err)
		}
	})
}

func TestMigrateDownDefaultsSkipsAndNoopsUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_initial.down.sql", "SELECT down_one;")
	writeUnitMigration(t, dir, "002_second.down.sql", "SELECT down_two;")
	writeUnitMigration(t, dir, "003_unapplied.down.sql", "SELECT ignored;")

	pool := newFakeMigrationPool(map[string]bool{
		"001_initial": true,
		"002_second":  true,
	})
	if err := migrateDown(ctx, pool, dir, 0); err != nil {
		t.Fatalf("migrateDown default step failed: %v", err)
	}
	if pool.applied["002_second"] {
		t.Fatalf("expected newest applied migration to be rolled back first")
	}
	if !pool.applied["001_initial"] {
		t.Fatalf("expected default rollback to leave older migration applied")
	}

	if err := migrateDown(ctx, pool, dir, 5); err != nil {
		t.Fatalf("migrateDown remaining migrations failed: %v", err)
	}
	if pool.applied["001_initial"] {
		t.Fatalf("expected remaining applied migration to be rolled back")
	}

	txCount := len(pool.txs)
	if err := migrateDown(ctx, pool, dir, 5); err != nil {
		t.Fatalf("migrateDown no-op failed: %v", err)
	}
	if len(pool.txs) != txCount {
		t.Fatalf("expected no transaction for unapplied down migrations")
	}
}

func TestMigrateDownErrorBranchesUnit(t *testing.T) {
	ctx := context.Background()

	t.Run("applied query error", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		pool.queryErr = errors.New("query unavailable")

		err := migrateDown(ctx, pool, t.TempDir(), 1)
		if err == nil || !strings.Contains(err.Error(), "get applied migrations") {
			t.Fatalf("expected applied migration query error, got %v", err)
		}
	})

	t.Run("glob error", func(t *testing.T) {
		err := migrateDown(ctx, newFakeMigrationPool(nil), "[", 1)
		if err == nil || !strings.Contains(err.Error(), "get migration files") {
			t.Fatalf("expected migration file glob error, got %v", err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "001_bad.down.sql"), 0o755); err != nil {
			t.Fatalf("failed to create directory migration: %v", err)
		}

		err := migrateDown(ctx, newFakeMigrationPool(map[string]bool{"001_bad": true}), dir, 1)
		if err == nil || !strings.Contains(err.Error(), "read migration file") {
			t.Fatalf("expected rollback read error, got %v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.down.sql", "SELECT 1;")
		pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
		pool.beginErr = errors.New("begin failed")

		err := migrateDown(ctx, pool, dir, 1)
		if err == nil || !strings.Contains(err.Error(), "begin transaction") {
			t.Fatalf("expected begin error, got %v", err)
		}
	})

	t.Run("record delete error", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.down.sql", "SELECT 1;")
		pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
		pool.txErrContains = "DELETE FROM schema_migrations"
		pool.txErr = errors.New("delete failed")

		err := migrateDown(ctx, pool, dir, 1)
		if err == nil || !strings.Contains(err.Error(), "remove migration record 001_initial") {
			t.Fatalf("expected remove migration record error, got %v", err)
		}
		if got := len(pool.txs); got != 1 || !pool.txs[0].rolledBack {
			t.Fatalf("expected delete failure rollback, txs=%+v", pool.txs)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.down.sql", "SELECT 1;")
		pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
		pool.commitErr = errors.New("commit failed")

		err := migrateDown(ctx, pool, dir, 1)
		if err == nil || !strings.Contains(err.Error(), "commit rollback 001_initial") {
			t.Fatalf("expected commit rollback error, got %v", err)
		}
	})
}

func TestMigrateDownRollsBackFailedRollbackUnit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeUnitMigration(t, dir, "001_bad.down.sql", "SELECT fail_down;")

	pool := newFakeMigrationPool(map[string]bool{"001_bad": true})
	pool.txErrContains = "fail_down"
	pool.txErr = errors.New("boom")

	err := migrateDown(ctx, pool, dir, 1)
	if err == nil || !strings.Contains(err.Error(), "execute rollback 001_bad") {
		t.Fatalf("expected execute rollback error, got %v", err)
	}
	if !pool.applied["001_bad"] {
		t.Fatalf("failed rollback should leave migration recorded as applied")
	}
	if got := len(pool.txs); got != 1 {
		t.Fatalf("expected 1 transaction, got %d", got)
	}
	if !pool.txs[0].rolledBack || pool.txs[0].committed {
		t.Fatalf("expected failed rollback to roll back: %+v", pool.txs[0])
	}
}

func TestMainRequiresDatabaseURLUnit(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMainUnitHelperProcess", "--")
	cmd.Env = []string{"GO_WANT_MIGRATE_UNIT_HELPER=1"}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected helper to fail without db url, output: %s", string(out))
	}
	if !strings.Contains(string(out), "missing database URL") {
		t.Fatalf("expected missing database error, got: %s", string(out))
	}
}

func TestMainRunsWithInjectedMigrationPoolUnit(t *testing.T) {
	dir := t.TempDir()
	pool := newFakeMigrationPool(nil)
	oldArgs := os.Args
	oldNewMigrationPool := newMigrationPool
	oldFatalMigrationError := fatalMigrationError
	defer func() {
		os.Args = oldArgs
		newMigrationPool = oldNewMigrationPool
		fatalMigrationError = oldFatalMigrationError
	}()

	os.Args = []string{"migrate", "-db", "postgres://unit", "-path", dir, "-direction", "up"}
	newMigrationPool = func(ctx context.Context, databaseURL string) (migrationPool, error) {
		if databaseURL != "postgres://unit" {
			t.Fatalf("databaseURL = %q, want postgres://unit", databaseURL)
		}
		return pool, nil
	}

	main()

	if !pool.closed {
		t.Fatal("expected migration pool to be closed")
	}
	if len(pool.execs) == 0 || !strings.Contains(pool.execs[0], "CREATE TABLE IF NOT EXISTS schema_migrations") {
		t.Fatalf("expected migrations table DDL, got %v", pool.execs)
	}
}

func TestMainReportsMigrationErrorUnit(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	oldArgs := os.Args
	oldFatalMigrationError := fatalMigrationError
	defer func() {
		os.Args = oldArgs
		fatalMigrationError = oldFatalMigrationError
	}()

	os.Args = []string{"migrate"}
	var gotErr error
	fatalMigrationError = func(err error) {
		gotErr = err
		panic("fatal migration error")
	}

	defer func() {
		recovered := recover()
		if recovered != "fatal migration error" {
			t.Fatalf("recover() = %v, want fatal migration error", recovered)
		}
		if gotErr == nil || !strings.Contains(gotErr.Error(), "missing database URL") {
			t.Fatalf("fatal error = %v, want missing database URL", gotErr)
		}
	}()

	main()
}

func TestDefaultFatalMigrationErrorLogsAndExitsUnit(t *testing.T) {
	oldExitMigrationProcess := exitMigrationProcess
	defer func() {
		exitMigrationProcess = oldExitMigrationProcess
	}()

	var exitCode int
	exitMigrationProcess = func(code int) {
		exitCode = code
		panic("exit called")
	}

	defer func() {
		recovered := recover()
		if recovered != "exit called" {
			t.Fatalf("recover() = %v, want exit called", recovered)
		}
		if exitCode != 1 {
			t.Fatalf("exit code = %d, want 1", exitCode)
		}
	}()

	defaultFatalMigrationError(errors.New("boom"))
}

func TestOpenMigrationPoolReturnsPoolUnit(t *testing.T) {
	pool, err := openMigrationPool(context.Background(), "postgres://unit")
	if err != nil {
		t.Fatalf("openMigrationPool() error = %v", err)
	}
	if pool == nil {
		t.Fatal("openMigrationPool() returned nil pool")
	}
	pool.Close()
}

func TestRunMigrationCLIErrorBranchesUnit(t *testing.T) {
	oldNewMigrationPool := newMigrationPool
	defer func() {
		newMigrationPool = oldNewMigrationPool
	}()

	t.Run("flag parse error", func(t *testing.T) {
		err := runMigrationCLI(context.Background(), []string{"-steps", "not-a-number"}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "invalid value") {
			t.Fatalf("expected flag parse error, got %v", err)
		}
	})

	t.Run("connect error", func(t *testing.T) {
		newMigrationPool = func(context.Context, string) (migrationPool, error) {
			return nil, errors.New("connect failed")
		}
		err := runMigrationCLI(context.Background(), []string{"-db", "postgres://unit"}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "failed to connect to database") {
			t.Fatalf("expected connect error, got %v", err)
		}
	})

	t.Run("ping error closes pool", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		pool.pingErr = errors.New("ping failed")
		newMigrationPool = func(context.Context, string) (migrationPool, error) {
			return pool, nil
		}
		err := runMigrationCLI(context.Background(), []string{"-db", "postgres://unit"}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "failed to ping database") {
			t.Fatalf("expected ping error, got %v", err)
		}
		if !pool.closed {
			t.Fatal("expected pool to be closed after ping error")
		}
	})

	t.Run("environment database url and down direction", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitMigration(t, dir, "001_initial.down.sql", "SELECT down;")
		pool := newFakeMigrationPool(map[string]bool{"001_initial": true})
		newMigrationPool = func(ctx context.Context, databaseURL string) (migrationPool, error) {
			if databaseURL != "postgres://env" {
				t.Fatalf("databaseURL = %q, want postgres://env", databaseURL)
			}
			return pool, nil
		}
		err := runMigrationCLI(context.Background(), []string{"-path", dir, "-direction", "down"}, func(key string) string {
			if key == "DATABASE_URL" {
				return "postgres://env"
			}
			return ""
		})
		if err != nil {
			t.Fatalf("runMigrationCLI down failed: %v", err)
		}
		if pool.applied["001_initial"] {
			t.Fatal("expected down migration to remove applied version")
		}
	})

	t.Run("invalid direction", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		newMigrationPool = func(context.Context, string) (migrationPool, error) {
			return pool, nil
		}
		err := runMigrationCLI(context.Background(), []string{"-db", "postgres://unit", "-direction", "sideways"}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), `invalid direction "sideways"`) {
			t.Fatalf("expected invalid direction error, got %v", err)
		}
	})

	t.Run("ensure migrations table error", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		pool.execErr = errors.New("ddl failed")
		newMigrationPool = func(context.Context, string) (migrationPool, error) {
			return pool, nil
		}
		err := runMigrationCLI(context.Background(), []string{"-db", "postgres://unit"}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "failed to create migrations table") {
			t.Fatalf("expected migrations table error, got %v", err)
		}
	})

	t.Run("migrate up error", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		newMigrationPool = func(context.Context, string) (migrationPool, error) {
			return pool, nil
		}
		err := runMigrationCLI(context.Background(), []string{"-db", "postgres://unit", "-path", "["}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "migration up failed") {
			t.Fatalf("expected migration up error, got %v", err)
		}
	})

	t.Run("migrate down error", func(t *testing.T) {
		pool := newFakeMigrationPool(nil)
		newMigrationPool = func(context.Context, string) (migrationPool, error) {
			return pool, nil
		}
		err := runMigrationCLI(context.Background(), []string{"-db", "postgres://unit", "-direction", "down", "-path", "["}, func(string) string { return "" })
		if err == nil || !strings.Contains(err.Error(), "migration down failed") {
			t.Fatalf("expected migration down error, got %v", err)
		}
	})
}

func TestMainUnitHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MIGRATE_UNIT_HELPER") != "1" {
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

func writeUnitMigration(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write migration %s: %v", name, err)
	}
}

type fakeMigrationPool struct {
	applied       map[string]bool
	execs         []string
	txs           []*fakeMigrationTx
	execErr       error
	queryErr      error
	rowsErr       error
	scanErr       error
	beginErr      error
	txErrContains string
	txErr         error
	commitErr     error
	pingErr       error
	closed        bool
}

func newFakeMigrationPool(applied map[string]bool) *fakeMigrationPool {
	copied := make(map[string]bool)
	for version, ok := range applied {
		copied[version] = ok
	}
	return &fakeMigrationPool{applied: copied}
}

func (p *fakeMigrationPool) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	p.execs = append(p.execs, sql)
	if p.execErr != nil {
		return pgconn.CommandTag{}, p.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (p *fakeMigrationPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if p.queryErr != nil {
		return nil, p.queryErr
	}

	versions := make([]string, 0, len(p.applied))
	for version, applied := range p.applied {
		if applied {
			versions = append(versions, version)
		}
	}
	sort.Strings(versions)

	return &fakeMigrationRows{versions: versions, err: p.rowsErr, scanErr: p.scanErr}, nil
}

func (p *fakeMigrationPool) Begin(context.Context) (pgx.Tx, error) {
	if p.beginErr != nil {
		return nil, p.beginErr
	}

	tx := &fakeMigrationTx{pool: p}
	p.txs = append(p.txs, tx)
	return tx, nil
}

func (p *fakeMigrationPool) Ping(context.Context) error {
	return p.pingErr
}

func (p *fakeMigrationPool) Close() {
	p.closed = true
}

type fakeMigrationRows struct {
	versions []string
	index    int
	closed   bool
	err      error
	scanErr  error
}

func (r *fakeMigrationRows) Close() {
	r.closed = true
}

func (r *fakeMigrationRows) Err() error {
	return r.err
}

func (r *fakeMigrationRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeMigrationRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeMigrationRows) Next() bool {
	if r.index >= len(r.versions) {
		r.closed = true
		return false
	}
	r.index++
	return true
}

func (r *fakeMigrationRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}

	version, ok := dest[0].(*string)
	if !ok {
		return errors.New("expected string scan destination")
	}
	*version = r.versions[r.index-1]
	return nil
}

func (r *fakeMigrationRows) Values() ([]any, error) {
	return []any{r.versions[r.index-1]}, nil
}

func (r *fakeMigrationRows) RawValues() [][]byte {
	return [][]byte{[]byte(r.versions[r.index-1])}
}

func (r *fakeMigrationRows) Conn() *pgx.Conn {
	return nil
}

type fakeMigrationTx struct {
	pool       *fakeMigrationPool
	execs      []string
	committed  bool
	rolledBack bool
}

func (tx *fakeMigrationTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested transactions are not implemented")
}

func (tx *fakeMigrationTx) Commit(context.Context) error {
	tx.committed = true
	return tx.pool.commitErr
}

func (tx *fakeMigrationTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakeMigrationTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("copy is not implemented")
}

func (tx *fakeMigrationTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeMigrationTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeMigrationTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("prepare is not implemented")
}

func (tx *fakeMigrationTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.execs = append(tx.execs, sql)
	if tx.pool.txErrContains != "" && strings.Contains(sql, tx.pool.txErrContains) {
		return pgconn.CommandTag{}, tx.pool.txErr
	}

	switch strings.TrimSpace(sql) {
	case "INSERT INTO schema_migrations (version) VALUES ($1)":
		tx.pool.applied[arguments[0].(string)] = true
	case "DELETE FROM schema_migrations WHERE version = $1":
		tx.pool.applied[arguments[0].(string)] = false
	}

	return pgconn.CommandTag{}, nil
}

func (tx *fakeMigrationTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("query is not implemented")
}

func (tx *fakeMigrationTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeMigrationRow{}
}

func (tx *fakeMigrationTx) Conn() *pgx.Conn {
	return nil
}

type fakeMigrationRow struct{}

func (fakeMigrationRow) Scan(...any) error {
	return errors.New("query row is not implemented")
}
