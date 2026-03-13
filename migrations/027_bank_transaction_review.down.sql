-- Migration 027 down: remove bank transaction review workflow fields

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        EXECUTE format('DROP INDEX IF EXISTS idx_%s_bt_follow_up_status', replace(tenant_schema, '-', '_'));
        EXECUTE format('ALTER TABLE %I.bank_transactions DROP CONSTRAINT IF EXISTS bank_transactions_follow_up_status_check', tenant_schema);
        EXECUTE format('ALTER TABLE %I.bank_transactions DROP COLUMN IF EXISTS reviewed_at', tenant_schema);
        EXECUTE format('ALTER TABLE %I.bank_transactions DROP COLUMN IF EXISTS reviewed_by', tenant_schema);
        EXECUTE format('ALTER TABLE %I.bank_transactions DROP COLUMN IF EXISTS review_note', tenant_schema);
        EXECUTE format('ALTER TABLE %I.bank_transactions DROP COLUMN IF EXISTS follow_up_status', tenant_schema);
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS add_bank_transaction_review_columns(TEXT);

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_payroll_tables(schema_name);
    PERFORM create_email_tables_only(schema_name);
    PERFORM add_kmd_tables_to_schema(schema_name);
    PERFORM fix_email_log_schema(schema_name);
    PERFORM add_reminder_rules_to_schema(schema_name);
    PERFORM add_interest_tables(schema_name);
    PERFORM add_document_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
