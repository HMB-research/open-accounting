-- Migration 027: Add bank transaction follow-up and review metadata

CREATE OR REPLACE FUNCTION add_bank_transaction_review_columns(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('ALTER TABLE %I.bank_transactions ADD COLUMN IF NOT EXISTS follow_up_status VARCHAR(30) NOT NULL DEFAULT ''NONE''', schema_name);
    EXECUTE format('ALTER TABLE %I.bank_transactions ADD COLUMN IF NOT EXISTS review_note TEXT', schema_name);
    EXECUTE format('ALTER TABLE %I.bank_transactions ADD COLUMN IF NOT EXISTS reviewed_by UUID', schema_name);
    EXECUTE format('ALTER TABLE %I.bank_transactions ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ', schema_name);

    EXECUTE format('UPDATE %I.bank_transactions SET follow_up_status = ''NONE'' WHERE follow_up_status IS NULL OR btrim(follow_up_status) = ''''', schema_name);

    EXECUTE format('ALTER TABLE %I.bank_transactions DROP CONSTRAINT IF EXISTS bank_transactions_follow_up_status_check', schema_name);
    EXECUTE format(
        'ALTER TABLE %I.bank_transactions ADD CONSTRAINT bank_transactions_follow_up_status_check CHECK (follow_up_status IN (''NONE'', ''EVIDENCE_REQUIRED'', ''READY_TO_MATCH''))',
        schema_name
    );

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_bt_follow_up_status ON %I.bank_transactions(follow_up_status)',
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
        PERFORM add_bank_transaction_review_columns(tenant_schema);
    END LOOP;
END $$;

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
    PERFORM add_bank_transaction_review_columns(schema_name);
END;
$$ LANGUAGE plpgsql;
