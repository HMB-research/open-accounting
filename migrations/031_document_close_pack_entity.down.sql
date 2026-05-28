-- Migration 031 down: remove year-end close-pack evidence document entity support

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        EXECUTE format('DELETE FROM %I.documents WHERE entity_type = ''year_end_close''', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_entity_type_check', tenant_schema);
        EXECUTE format(
            'ALTER TABLE %I.documents ADD CONSTRAINT documents_entity_type_check CHECK (entity_type IN (''invoice'', ''journal_entry'', ''payment'', ''bank_transaction'', ''asset''))',
            tenant_schema
        );
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS add_close_pack_document_entity(TEXT);

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
