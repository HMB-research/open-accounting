-- Migration 070: durable, tenant-scoped review and idempotency state for the
-- SmartAccounts GL-authoritative executor. This migration creates no journals.
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
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(), applied_at TIMESTAMPTZ NULL,
        UNIQUE (tenant_id, provider, source_company_id, resource, external_id))', schema_name);
    EXECUTE format('CREATE TABLE IF NOT EXISTS %I.smartaccounts_correction_reviews (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
        provider VARCHAR(64) NOT NULL, source_company_id VARCHAR(255) NOT NULL, resource VARCHAR(128) NOT NULL,
        external_id VARCHAR(255) NOT NULL, revision VARCHAR(64) NOT NULL, operation VARCHAR(32) NOT NULL,
        reason VARCHAR(255) NOT NULL, package_id VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now())', schema_name);
END; $$ LANGUAGE plpgsql;
DO $$ DECLARE tenant_schema TEXT; BEGIN
  FOR tenant_schema IN SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL LOOP
    PERFORM add_smartaccounts_executor_tables(tenant_schema);
  END LOOP;
END $$;

-- Fresh tenant schemas are built from the latest migration function, so keep
-- the executor state structurally available to tenants created after upgrade.
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
    PERFORM add_reminder_rules(schema_name);
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
    PERFORM add_cost_center_tables(schema_name);
    PERFORM add_migration_execution_run_tables(schema_name);
    PERFORM add_financial_report_indexes(schema_name);
    PERFORM add_import_session_tables(schema_name);
    PERFORM add_import_session_ledger_verification(schema_name);
    PERFORM add_import_session_ledger_plan_input(schema_name);
    PERFORM add_external_import_delivery_tables(schema_name);
    PERFORM add_smartaccounts_executor_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
