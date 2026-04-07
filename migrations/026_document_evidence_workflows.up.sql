-- Migration 026: Extend documents for reconciliation evidence and asset support

CREATE OR REPLACE FUNCTION add_document_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.documents (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            entity_type VARCHAR(50) NOT NULL,
            entity_id UUID NOT NULL,
            document_type VARCHAR(50) NOT NULL DEFAULT ''supporting_document'',
            file_name VARCHAR(255) NOT NULL,
            content_type VARCHAR(255) NOT NULL,
            file_size BIGINT NOT NULL,
            storage_key TEXT NOT NULL,
            notes TEXT,
            retention_until TIMESTAMPTZ,
            review_status VARCHAR(20) NOT NULL DEFAULT ''PENDING'',
            reviewed_by UUID,
            reviewed_at TIMESTAMPTZ,
            uploaded_by UUID NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT documents_entity_type_check CHECK (entity_type IN (''invoice'', ''journal_entry'', ''payment'', ''bank_transaction'', ''asset'')),
            CONSTRAINT documents_review_status_check CHECK (review_status IN (''PENDING'', ''REVIEWED''))
        )
    ', schema_name);

    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS document_type VARCHAR(50) NOT NULL DEFAULT ''supporting_document''', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS notes TEXT', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS retention_until TIMESTAMPTZ', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS review_status VARCHAR(20) NOT NULL DEFAULT ''PENDING''', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS reviewed_by UUID', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ', schema_name);

    EXECUTE format('UPDATE %I.documents SET document_type = ''supporting_document'' WHERE document_type IS NULL OR btrim(document_type) = ''''', schema_name);
    EXECUTE format('UPDATE %I.documents SET review_status = ''PENDING'' WHERE review_status IS NULL OR btrim(review_status) = ''''', schema_name);

    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_entity_type_check', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD CONSTRAINT documents_entity_type_check CHECK (entity_type IN (''invoice'', ''journal_entry'', ''payment'', ''bank_transaction'', ''asset''))', schema_name);

    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_review_status_check', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD CONSTRAINT documents_review_status_check CHECK (review_status IN (''PENDING'', ''REVIEWED''))', schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_documents_entity ON %I.documents(tenant_id, entity_type, entity_id, created_at DESC)', replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_documents_storage_key ON %I.documents(storage_key)', replace(schema_name, '-', '_'), schema_name);
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
