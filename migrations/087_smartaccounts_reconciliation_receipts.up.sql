-- Migration 087: immutable, digest-only SmartAccounts reconciliation control
-- evidence. These tables deliberately contain no source rows, proof payloads,
-- monetary amounts, account names, browser data, credentials, or notes.

CREATE TABLE IF NOT EXISTS public.smartaccounts_gl_apply_receipts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    package_id TEXT NOT NULL,
    preview_id UUID NOT NULL,
    preview_sha256 CHAR(64) NOT NULL,
    mapping_snapshot_sha256 CHAR(64) NOT NULL,
    applied_identity_sha256 CHAR(64) NOT NULL,
    tolerance_policy_sha256 CHAR(64) NOT NULL,
    mapping_count INTEGER NOT NULL,
    applied_identity_count INTEGER NOT NULL,
    first_applied_by TEXT NOT NULL,
    first_applied_at TIMESTAMPTZ NOT NULL,
    exact_replay_by TEXT NULL,
    exact_replay_at TIMESTAMPTZ NULL,
    CONSTRAINT smartaccounts_gl_apply_receipt_source_check CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_gl_apply_receipt_digest_check CHECK (
        preview_sha256 ~ '^[0-9a-f]{64}$' AND mapping_snapshot_sha256 ~ '^[0-9a-f]{64}$' AND
        applied_identity_sha256 ~ '^[0-9a-f]{64}$' AND tolerance_policy_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_gl_apply_receipt_counts_check CHECK (mapping_count > 0 AND applied_identity_count > 0),
    CONSTRAINT smartaccounts_gl_apply_receipt_replay_check CHECK (
        (exact_replay_by IS NULL AND exact_replay_at IS NULL) OR
        (exact_replay_by IS NOT NULL AND exact_replay_at IS NOT NULL AND exact_replay_at >= first_applied_at)
    ),
    UNIQUE (tenant_id, source_company_id, package_id, preview_sha256)
);

CREATE TABLE IF NOT EXISTS public.smartaccounts_gl_apply_receipt_mappings (
    receipt_id UUID NOT NULL REFERENCES public.smartaccounts_gl_apply_receipts(id) ON DELETE CASCADE,
    source_account_external_id TEXT NOT NULL,
    target_account_id UUID NOT NULL,
    PRIMARY KEY (receipt_id, source_account_external_id)
);

CREATE TABLE IF NOT EXISTS public.smartaccounts_gl_apply_receipt_identities (
    receipt_id UUID NOT NULL REFERENCES public.smartaccounts_gl_apply_receipts(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    revision CHAR(64) NOT NULL,
    reservation_id UUID NOT NULL,
    journal_id UUID NOT NULL,
    PRIMARY KEY (receipt_id, external_id),
    CONSTRAINT smartaccounts_gl_apply_receipt_identity_revision_check CHECK (revision ~ '^[0-9a-f]{64}$')
);

-- Policy handles are created only by an accountant-facing control path. The
-- digest itself is opaque here; source/package/scope binding makes a copied
-- handle unusable for a different staged source or capture scope.
CREATE TABLE IF NOT EXISTS public.smartaccounts_gl_tolerance_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	algorithm_version TEXT NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    package_id TEXT NOT NULL,
    scope_sha256 CHAR(64) NOT NULL,
	preview_sha256 CHAR(64) NOT NULL,
    tolerance_policy_sha256 CHAR(64) NOT NULL,
    approved_by TEXT NOT NULL,
    approved_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT smartaccounts_gl_tolerance_policy_source_check CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_gl_tolerance_policy_digest_check CHECK (
        scope_sha256 ~ '^[0-9a-f]{64}$' AND preview_sha256 ~ '^[0-9a-f]{64}$' AND tolerance_policy_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_gl_tolerance_policy_algorithm_check CHECK (algorithm_version = 'smartaccounts-exact-match-v1'),
    UNIQUE (tenant_id, source_company_id, package_id, scope_sha256, preview_sha256, algorithm_version, tolerance_policy_sha256)
);

-- The initial executor migration predates receipt recovery. Preserve its
-- deployed history and extend the tenant-local posting identity with only the
-- original actor ID required to recover a failed public receipt safely.
CREATE OR REPLACE FUNCTION add_smartaccounts_executor_reconciliation_columns(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('ALTER TABLE %I.smartaccounts_financial_postings ADD COLUMN IF NOT EXISTS applied_by TEXT NULL', schema_name);
	EXECUTE format('ALTER TABLE %I.smartaccounts_financial_postings ADD COLUMN IF NOT EXISTS reserved_by TEXT NULL', schema_name);
	-- A SmartAccounts reservation UUID is also the accounting journal source ID.
	-- This unique active identity prevents two crash-recovery attempts from
	-- independently creating an otherwise identical target journal.
	EXECUTE format('CREATE UNIQUE INDEX IF NOT EXISTS %I ON %I.journal_entries (tenant_id, source_type, source_id) WHERE source_type = ''SMARTACCOUNTS_GL'' AND source_id IS NOT NULL AND status <> ''VOIDED''', 'smartaccounts_gl_active_source_identity_unique', schema_name);
END; $$ LANGUAGE plpgsql;

DO $$ DECLARE tenant_schema TEXT; BEGIN
  FOR tenant_schema IN SELECT schema_name FROM public.tenants WHERE schema_name IS NOT NULL LOOP
    PERFORM add_smartaccounts_executor_reconciliation_columns(tenant_schema);
  END LOOP;
END $$;

-- Future tenant schemas still call the versioned executor bootstrap helper;
-- redefine only that helper to make the added identity column available on a
-- fresh tenant without rewriting migration 070.
CREATE OR REPLACE FUNCTION add_smartaccounts_executor_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I.smartaccounts_executor_previews (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
        package_id VARCHAR(255) NOT NULL, source_company_id VARCHAR(255) NOT NULL,
        preview_sha256 VARCHAR(64) NOT NULL, status VARCHAR(32) NOT NULL,
        plan JSONB NOT NULL, reconciliation JSONB NOT NULL, issues JSONB NOT NULL,
        created_by UUID NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), applied_at TIMESTAMPTZ NULL,
        UNIQUE (tenant_id, package_id, preview_sha256))', schema_name);
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I.smartaccounts_source_account_mappings (
        tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE, provider VARCHAR(64) NOT NULL,
        source_company_id VARCHAR(255) NOT NULL, source_account_external_id VARCHAR(255) NOT NULL,
        target_account_id UUID NOT NULL, decision JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        PRIMARY KEY (tenant_id, provider, source_company_id, source_account_external_id))', schema_name);
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I.smartaccounts_financial_postings (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
        provider VARCHAR(64) NOT NULL, source_company_id VARCHAR(255) NOT NULL, resource VARCHAR(128) NOT NULL,
        external_id VARCHAR(255) NOT NULL, revision VARCHAR(64) NOT NULL, status VARCHAR(32) NOT NULL,
        journal_entry_id UUID NULL, package_id VARCHAR(255) NOT NULL, preview_id UUID NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(), reserved_by TEXT NULL, applied_at TIMESTAMPTZ NULL, applied_by TEXT NULL,
        UNIQUE (tenant_id, provider, source_company_id, resource, external_id))', schema_name);
    EXECUTE format('ALTER TABLE %I.smartaccounts_financial_postings ADD COLUMN IF NOT EXISTS applied_by TEXT NULL', schema_name);
    EXECUTE format('ALTER TABLE %I.smartaccounts_financial_postings ADD COLUMN IF NOT EXISTS reserved_by TEXT NULL', schema_name);
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I.smartaccounts_correction_reviews (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
        provider VARCHAR(64) NOT NULL, source_company_id VARCHAR(255) NOT NULL, resource VARCHAR(128) NOT NULL,
        external_id VARCHAR(255) NOT NULL, revision VARCHAR(64) NOT NULL, operation VARCHAR(32) NOT NULL,
        reason VARCHAR(255) NOT NULL, package_id VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now())', schema_name);
END; $$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS public.smartaccounts_reconciliation_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES public.smartaccounts_browser_onboarding_batches(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    package_id TEXT NOT NULL DEFAULT '',
    manifest_sha256 CHAR(64) NOT NULL DEFAULT '',
    records_sha256 CHAR(64) NOT NULL DEFAULT '',
    scope_sha256 CHAR(64) NOT NULL DEFAULT '',
    source_as_of_date DATE NULL,
    cutoff_at TIMESTAMPTZ NULL,
    gl_preview_id UUID NULL,
    gl_preview_sha256 CHAR(64) NOT NULL DEFAULT '',
    gl_state TEXT NOT NULL,
    gl_mapping_snapshot_sha256 CHAR(64) NOT NULL DEFAULT '',
    gl_applied_identity_sha256 CHAR(64) NOT NULL DEFAULT '',
    reference_preview_id UUID NULL,
    reference_preview_sha256 CHAR(64) NULL,
    reference_state TEXT NOT NULL,
    proof_id TEXT NOT NULL DEFAULT '',
    proof_sha256 CHAR(64) NOT NULL DEFAULT '',
    claim_sha256 CHAR(64) NOT NULL DEFAULT '',
    coverage_sha256 CHAR(64) NOT NULL DEFAULT '',
    claim_kind TEXT NOT NULL DEFAULT '',
    expected_coverage_state TEXT NOT NULL DEFAULT '',
    tolerance_sha256 CHAR(64) NOT NULL DEFAULT '',
    variance_within_policy BOOLEAN NOT NULL DEFAULT FALSE,
    gl_revision_unresolved INTEGER NOT NULL DEFAULT 0,
    gl_tombstone_unresolved INTEGER NOT NULL DEFAULT 0,
    reference_revision_unresolved INTEGER NOT NULL DEFAULT 0,
    reference_tombstone_unresolved INTEGER NOT NULL DEFAULT 0,
    blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_sha256 CHAR(64) NOT NULL,
    evidence_submitted_by TEXT NOT NULL,
    gl_first_applied_by TEXT NOT NULL DEFAULT '',
    gl_exact_replay_by TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    accountant_approved_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_reconciliation_evaluation_source_check CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_reconciliation_evaluation_status_check CHECK (status IN ('EVIDENCE_PENDING', 'READY_FOR_ACCOUNTANT', 'PASS', 'PARTIAL_FAILURE')),
    CONSTRAINT smartaccounts_reconciliation_evaluation_state_check CHECK (
        gl_state IN ('EVIDENCE_PENDING', 'APPLIED', 'APPLIED_REPLAY_VERIFIED') AND
        reference_state IN ('NOT_APPLICABLE', 'EVIDENCE_PENDING', 'APPLIED')
    ),
    CONSTRAINT smartaccounts_reconciliation_evaluation_coverage_check CHECK (
        claim_kind IN ('', 'full', 'partial') AND expected_coverage_state IN ('', 'full', 'partial', 'unknown')
    ),
    CONSTRAINT smartaccounts_reconciliation_evaluation_counts_check CHECK (
        gl_revision_unresolved >= 0 AND gl_tombstone_unresolved >= 0 AND
        reference_revision_unresolved >= 0 AND reference_tombstone_unresolved >= 0
    ),
    CONSTRAINT smartaccounts_reconciliation_evaluation_blockers_check CHECK (jsonb_typeof(blockers) = 'array'),
    CONSTRAINT smartaccounts_reconciliation_evaluation_evidence_check CHECK (evidence_sha256 ~ '^[0-9a-f]{64}$'),
    UNIQUE (batch_id, source_company_id, tenant_id, evidence_sha256)
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_reconciliation_evaluations_rollup
    ON public.smartaccounts_reconciliation_evaluations (batch_id, source_company_id, tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS public.smartaccounts_reconciliation_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evaluation_id UUID NOT NULL REFERENCES public.smartaccounts_reconciliation_evaluations(id) ON DELETE CASCADE,
    evidence_sha256 CHAR(64) NOT NULL,
    tolerance_sha256 CHAR(64) NOT NULL,
    approved_by TEXT NOT NULL,
    approved_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT smartaccounts_reconciliation_approval_digest_check CHECK (
        evidence_sha256 ~ '^[0-9a-f]{64}$' AND tolerance_sha256 ~ '^[0-9a-f]{64}$'
    ),
    UNIQUE (evaluation_id, evidence_sha256, tolerance_sha256)
);

-- New tenant schemas are created by the current function body; reconciliation
-- receipts intentionally live in public so selected/all roll-ups can bind
-- tenant-isolated results without copying financial details across schemas.

-- Keep fresh tenant schemas equivalent to upgraded tenants. This is the
-- latest versioned bootstrap function; call the reconciliation helper after
-- executor tables exist so it adds the crash-resume columns and active source
-- identity constraint as well.
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);
    PERFORM create_accounting_tables(schema_name); PERFORM add_journal_entry_post_reason(schema_name); PERFORM add_vat_columns_to_journal_lines(schema_name); PERFORM add_payment_reversal_columns(schema_name); PERFORM add_reconciliation_tables_to_schema(schema_name); PERFORM add_recurring_tables_to_schema(schema_name); PERFORM add_quotes_and_orders_tables(schema_name); PERFORM add_fixed_assets_tables(schema_name); PERFORM add_fixed_asset_disposal_journal_links(schema_name); PERFORM create_inventory_tables(schema_name); PERFORM add_inventory_movement_tracking_metadata(schema_name); PERFORM add_inventory_lot_reservations(schema_name); PERFORM add_payroll_tables(schema_name); PERFORM add_leave_management_tables(schema_name); PERFORM create_email_tables_only(schema_name); PERFORM add_kmd_tables_to_schema(schema_name); PERFORM fix_email_log_schema(schema_name); PERFORM add_reminder_rules(schema_name); PERFORM sync_email_template_type_constraint(schema_name); PERFORM add_interest_tables(schema_name); PERFORM add_document_tables(schema_name); PERFORM add_document_review_workflow(schema_name); PERFORM add_bank_transaction_review_columns(schema_name); PERFORM add_close_pack_document_entity(schema_name); PERFORM add_order_stock_reservations(schema_name); PERFORM add_journal_entry_evidence_requirement(schema_name); PERFORM add_journal_entry_templates(schema_name); PERFORM add_journal_entry_template_recurrence(schema_name); PERFORM add_bank_match_rules(schema_name); PERFORM add_invoice_vat_treatment(schema_name); PERFORM add_expense_tables(schema_name); PERFORM add_commercial_document_entities(schema_name); PERFORM add_leave_record_document_entity(schema_name); PERFORM add_tax_declaration_document_entities(schema_name); PERFORM add_document_lifecycle_workflow(schema_name); PERFORM add_document_legal_hold_workflow(schema_name); PERFORM add_cost_center_tables(schema_name); PERFORM add_migration_execution_run_tables(schema_name); PERFORM add_financial_report_indexes(schema_name); PERFORM add_import_session_tables(schema_name); PERFORM add_import_session_ledger_verification(schema_name); PERFORM add_import_session_ledger_plan_input(schema_name); PERFORM add_external_import_delivery_tables(schema_name); PERFORM add_smartaccounts_executor_tables(schema_name); PERFORM add_smartaccounts_reference_master_tables(schema_name); PERFORM add_smartaccounts_executor_reconciliation_columns(schema_name);
END; $$ LANGUAGE plpgsql;
