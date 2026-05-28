-- Migration 028: Add explicit document approval decisions and review notes

CREATE OR REPLACE FUNCTION add_document_review_workflow(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS review_note TEXT', schema_name);

    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_review_status_check', schema_name);
    EXECUTE format(
        'ALTER TABLE %I.documents ADD CONSTRAINT documents_review_status_check CHECK (review_status IN (''PENDING'', ''REVIEWED'', ''APPROVED'', ''REJECTED''))',
        schema_name
    );

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_documents_review_status ON %I.documents(review_status, reviewed_at DESC)', replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_documents_retention ON %I.documents(retention_until) WHERE retention_until IS NOT NULL', replace(schema_name, '-', '_'), schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_document_review_workflow(tenant_schema);
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
    PERFORM add_document_review_workflow(schema_name);
    PERFORM add_bank_transaction_review_columns(schema_name);
END;
$$ LANGUAGE plpgsql;
