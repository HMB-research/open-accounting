-- Migration 038: Invoice line VAT treatment for reverse-charge VAT

CREATE OR REPLACE FUNCTION add_invoice_vat_treatment(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        ALTER TABLE %I.invoice_lines
        ADD COLUMN IF NOT EXISTS vat_treatment VARCHAR(30) NOT NULL DEFAULT ''STANDARD''
    ', schema_name);

    EXECUTE format('
        ALTER TABLE %I.invoice_lines
        DROP CONSTRAINT IF EXISTS invoice_lines_vat_treatment_check
    ', schema_name);

    EXECUTE format('
        ALTER TABLE %I.invoice_lines
        ADD CONSTRAINT invoice_lines_vat_treatment_check
        CHECK (vat_treatment IN (''STANDARD'', ''REVERSE_CHARGE''))
    ', schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_invoice_lines_vat_treatment ON %I.invoice_lines(vat_treatment)',
        replace(schema_name, '-', '_'),
        schema_name
    );
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_invoice_vat_treatment(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM create_inventory_tables(schema_name);
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
END;
$$ LANGUAGE plpgsql;
