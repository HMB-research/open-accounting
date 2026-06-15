-- Migration 058 rollback: Remove audited document lifecycle states

CREATE OR REPLACE FUNCTION remove_document_lifecycle_workflow(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('DROP INDEX IF EXISTS %I.%I', schema_name, 'idx_' || replace(schema_name, '-', '_') || '_documents_superseded_by');
    EXECUTE format('DROP INDEX IF EXISTS %I.%I', schema_name, 'idx_' || replace(schema_name, '-', '_') || '_documents_lifecycle_status');
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_lifecycle_status_check', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS lifecycle_actioned_at', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS lifecycle_actioned_by', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS superseded_by_document_id', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS lifecycle_note', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS lifecycle_status', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM remove_document_lifecycle_workflow(tenant_schema);
    END LOOP;
END $$;
