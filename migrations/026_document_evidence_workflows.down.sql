-- Migration 026 down: remove extended document evidence metadata and entity coverage

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        EXECUTE format('DELETE FROM %I.documents WHERE entity_type IN (''bank_transaction'', ''asset'')', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_review_status_check', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_entity_type_check', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS reviewed_at', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS reviewed_by', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS review_status', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS retention_until', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS notes', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS document_type', tenant_schema);
        EXECUTE format('ALTER TABLE %I.documents ADD CONSTRAINT documents_entity_type_check CHECK (entity_type IN (''invoice'', ''journal_entry'', ''payment''))', tenant_schema);
    END LOOP;
END $$;

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

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_documents_entity ON %I.documents(tenant_id, entity_type, entity_id, created_at DESC)', replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_documents_storage_key ON %I.documents(storage_key)', replace(schema_name, '-', '_'), schema_name);
END;
$$ LANGUAGE plpgsql;
