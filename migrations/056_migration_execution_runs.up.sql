-- Migration 056: Persist tenant-scoped migration execution run snapshots

CREATE OR REPLACE FUNCTION add_migration_execution_run_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.migration_execution_runs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            created_by TEXT NOT NULL DEFAULT '''',
            status VARCHAR(32) NOT NULL,
            confirmed BOOLEAN NOT NULL DEFAULT FALSE,
            resumed BOOLEAN NOT NULL DEFAULT FALSE,
            step_count INTEGER NOT NULL DEFAULT 0,
            succeeded_step_count INTEGER NOT NULL DEFAULT 0,
            failed_step_count INTEGER NOT NULL DEFAULT 0,
            skipped_step_count INTEGER NOT NULL DEFAULT 0,
            planned_step_count INTEGER NOT NULL DEFAULT 0,
            resumed_step_count INTEGER NOT NULL DEFAULT 0,
            file_names TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
            run_payload JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.migration_execution_runs(tenant_id, created_at DESC)',
        'idx_' || replace(schema_name, '-', '_') || '_migration_runs_tenant_created',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.migration_execution_runs(tenant_id, status, created_at DESC)',
        'idx_' || replace(schema_name, '-', '_') || '_migration_runs_tenant_status',
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
        PERFORM add_migration_execution_run_tables(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
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
    PERFORM add_cost_center_tables(schema_name);
    PERFORM add_migration_execution_run_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
