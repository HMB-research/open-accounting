-- Migration 064: Tenant-scoped, receipt-only external import sessions.
--
-- This table persists no canonical package payloads or credentials. It records
-- a validated package fingerprint and the safe validation receipt only.

CREATE TABLE IF NOT EXISTS public.import_source_bindings (
    provider VARCHAR(64) NOT NULL,
    source_company_id VARCHAR(255) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, source_company_id)
);

CREATE INDEX IF NOT EXISTS idx_import_source_bindings_tenant
ON public.import_source_bindings(tenant_id, created_at DESC);

CREATE OR REPLACE FUNCTION add_import_session_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.import_sessions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            provider VARCHAR(64) NOT NULL,
            source_company_id VARCHAR(255) NOT NULL,
            schema_version VARCHAR(32) NOT NULL,
            package_sha256 VARCHAR(64) NOT NULL,
            status VARCHAR(32) NOT NULL,
            record_count INTEGER NOT NULL DEFAULT 0 CHECK (record_count >= 0),
            entity_counts JSONB NOT NULL DEFAULT ''{}''::JSONB,
            validation JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_by TEXT NOT NULL DEFAULT '''',
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT import_sessions_package_sha256_check CHECK (package_sha256 ~ ''^[0-9a-f]{64}$''),
            CONSTRAINT import_sessions_package_identity_unique UNIQUE (tenant_id, provider, source_company_id, package_sha256)
        )
    ', schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.import_sessions(tenant_id, created_at DESC)',
        'idx_' || replace(schema_name, '-', '_') || '_import_sessions_tenant_created',
        schema_name
    );
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_import_session_tables(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_journal_entry_post_reason(schema_name);
    PERFORM add_vat_columns_to_journal_lines(schema_name);
    PERFORM add_payment_reversal_columns(schema_name);
    PERFORM add_reconciliation_tables_to_schema(schema_name);
    PERFORM add_recurring_tables_to_schema(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM add_fixed_asset_disposal_journal_links(schema_name);
    PERFORM create_inventory_tables(schema_name);
    PERFORM add_inventory_movement_tracking_metadata(schema_name);
    PERFORM add_inventory_lot_reservations(schema_name);
    PERFORM add_payroll_tables(schema_name);
    PERFORM add_leave_management_tables(schema_name);
    PERFORM create_email_tables_only(schema_name);
    PERFORM add_kmd_tables_to_schema(schema_name);
    PERFORM fix_email_log_schema(schema_name);
    PERFORM add_reminder_rules_to_schema(schema_name);
    PERFORM sync_email_template_type_constraint(schema_name);
    PERFORM add_interest_tables(schema_name);
    PERFORM add_document_tables(schema_name);
    PERFORM add_document_review_workflow(schema_name);
    PERFORM add_bank_transaction_review_columns(schema_name);
    PERFORM add_close_pack_document_entity(schema_name);
    PERFORM add_order_stock_reservations(schema_name);
    PERFORM add_journal_entry_evidence_requirement(schema_name);
    PERFORM add_journal_entry_templates(schema_name);
    PERFORM add_journal_entry_template_recurrence(schema_name);
    PERFORM add_bank_match_rules(schema_name);
    PERFORM add_invoice_vat_treatment(schema_name);
    PERFORM add_expense_tables(schema_name);
    PERFORM add_commercial_document_entities(schema_name);
    PERFORM add_leave_record_document_entity(schema_name);
    PERFORM add_tax_declaration_document_entities(schema_name);
    PERFORM add_document_lifecycle_workflow(schema_name);
    PERFORM add_document_legal_hold_workflow(schema_name);
    PERFORM add_document_lifecycle_integrity(schema_name);
    PERFORM add_cost_center_tables(schema_name);
    PERFORM add_migration_execution_run_tables(schema_name);
    PERFORM add_financial_report_indexes(schema_name);
    PERFORM add_import_session_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
