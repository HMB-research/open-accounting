-- Migration 025: Add tenant-scoped document attachments

CREATE OR REPLACE FUNCTION add_document_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.documents (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            entity_type VARCHAR(50) NOT NULL,
            entity_id UUID NOT NULL,
            file_name VARCHAR(255) NOT NULL,
            content_type VARCHAR(255) NOT NULL,
            file_size BIGINT NOT NULL,
            storage_key TEXT NOT NULL,
            uploaded_by UUID NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT documents_entity_type_check CHECK (entity_type IN (''invoice'', ''journal_entry'', ''payment''))
        )
    ', schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_documents_entity
        ON %I.documents(tenant_id, entity_type, entity_id, created_at DESC)
    ', replace(schema_name, '-', '_'), schema_name);

    EXECUTE format('
        CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_documents_storage_key
        ON %I.documents(storage_key)
    ', replace(schema_name, '-', '_'), schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_document_tables(tenant_schema);
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
END;
$$ LANGUAGE plpgsql;
