-- Migration 052: ensure journal line VAT columns are tenant-bootstrap owned.
--
-- Migration 020 added these columns to schemas that already existed, but new
-- tenants are created through create_tenant_schema. Keep the reusable helper as
-- the single schema-normalization path and call it from tenant bootstrap.

CREATE OR REPLACE FUNCTION add_vat_columns_to_journal_lines(schema_name TEXT)
RETURNS void AS $$
BEGIN
    EXECUTE format('
        ALTER TABLE %I.journal_entry_lines
        ADD COLUMN IF NOT EXISTS vat_rate NUMERIC(5,2) DEFAULT 0
    ', schema_name);
    EXECUTE format('UPDATE %I.journal_entry_lines SET vat_rate = 0 WHERE vat_rate IS NULL', schema_name);
    EXECUTE format('ALTER TABLE %I.journal_entry_lines ALTER COLUMN vat_rate SET DEFAULT 0', schema_name);
    EXECUTE format('ALTER TABLE %I.journal_entry_lines ALTER COLUMN vat_rate SET NOT NULL', schema_name);

    EXECUTE format('
        ALTER TABLE %I.journal_entry_lines
        ADD COLUMN IF NOT EXISTS is_vat_inclusive BOOLEAN DEFAULT false
    ', schema_name);
    EXECUTE format('UPDATE %I.journal_entry_lines SET is_vat_inclusive = false WHERE is_vat_inclusive IS NULL', schema_name);
    EXECUTE format('ALTER TABLE %I.journal_entry_lines ALTER COLUMN is_vat_inclusive SET DEFAULT false', schema_name);
    EXECUTE format('ALTER TABLE %I.journal_entry_lines ALTER COLUMN is_vat_inclusive SET NOT NULL', schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_journal_entry_lines_vat_rate
        ON %I.journal_entry_lines(vat_rate)
        WHERE vat_rate > 0
    ', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT t.schema_name
        FROM tenants t
        JOIN information_schema.tables tbl
            ON tbl.table_schema = t.schema_name
           AND tbl.table_name = 'journal_entry_lines'
        WHERE t.schema_name IS NOT NULL
    LOOP
        PERFORM add_vat_columns_to_journal_lines(tenant_schema);
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
