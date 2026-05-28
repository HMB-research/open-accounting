-- Rollback Migration 028: Remove explicit document approval decisions and review notes

DO $$
DECLARE
    tenant_schema TEXT;
    index_prefix TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        index_prefix := replace(tenant_schema, '-', '_');
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', tenant_schema, 'idx_' || index_prefix || '_documents_review_status');
        EXECUTE format('DROP INDEX IF EXISTS %I.%I', tenant_schema, 'idx_' || index_prefix || '_documents_retention');

        EXECUTE format('UPDATE %I.documents SET review_status = ''REVIEWED'' WHERE review_status IN (''APPROVED'', ''REJECTED'')', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_review_status_check', tenant_schema);
        EXECUTE format(
            'ALTER TABLE %I.documents ADD CONSTRAINT documents_review_status_check CHECK (review_status IN (''PENDING'', ''REVIEWED''))',
            tenant_schema
        );
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS review_note', tenant_schema);
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS add_document_review_workflow(TEXT);

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
