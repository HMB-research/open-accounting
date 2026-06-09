-- Roll back structured payment reversal metadata.

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_reconciliation_tables_to_schema(schema_name);
    PERFORM add_recurring_tables_to_schema(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM add_fixed_asset_disposal_journal_links(schema_name);
    PERFORM create_inventory_tables(schema_name);
    PERFORM add_inventory_movement_tracking_metadata(schema_name);
    PERFORM add_payroll_tables(schema_name);
    PERFORM add_leave_management_tables(schema_name);
    PERFORM create_email_tables_only(schema_name);
    PERFORM add_kmd_tables_to_schema(schema_name);
    PERFORM fix_email_log_schema(schema_name);
    PERFORM add_reminder_rules_to_schema(schema_name);
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
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', tenant_schema, 'idx_' || replace(tenant_schema, '-', '_') || '_pay_reversal_of');
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', tenant_schema, 'idx_' || replace(tenant_schema, '-', '_') || '_pay_reversed_by_payment');
        EXECUTE format('ALTER TABLE %I.payments DROP COLUMN IF EXISTS reversal_reason', tenant_schema);
        EXECUTE format('ALTER TABLE %I.payments DROP COLUMN IF EXISTS reversed_by', tenant_schema);
        EXECUTE format('ALTER TABLE %I.payments DROP COLUMN IF EXISTS reversed_at', tenant_schema);
        EXECUTE format('ALTER TABLE %I.payments DROP COLUMN IF EXISTS reversed_by_payment_id', tenant_schema);
        EXECUTE format('ALTER TABLE %I.payments DROP COLUMN IF EXISTS reversal_of_payment_id', tenant_schema);
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS add_payment_reversal_columns(TEXT);
