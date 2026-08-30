-- Migration 065: Add safe ledger verification metadata to existing
-- receipt-only import sessions. Migration 064 is already deployed and must
-- remain its original receipt schema.

CREATE OR REPLACE FUNCTION add_import_session_ledger_verification(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Be defensive for partially provisioned or manually created tenant
    -- schemas. Normal tenant bootstrap creates import_sessions before this
    -- helper is called.
    IF to_regclass(format('%I.import_sessions', schema_name)) IS NULL THEN
        RETURN;
    END IF;

    EXECUTE format(
        'ALTER TABLE %I.import_sessions ADD COLUMN IF NOT EXISTS ledger_verification JSONB NOT NULL DEFAULT ''{}''::JSONB',
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
        PERFORM add_import_session_ledger_verification(tenant_schema);
    END LOOP;
END $$;

-- Keep future tenant schemas equivalent to upgraded existing schemas.
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
    PERFORM add_import_session_ledger_verification(schema_name);
END;
$$ LANGUAGE plpgsql;
