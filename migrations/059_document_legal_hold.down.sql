-- Migration 059 rollback: Remove document legal hold controls

CREATE OR REPLACE FUNCTION remove_document_legal_hold_workflow(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('DROP INDEX IF EXISTS %I.%I', schema_name, 'idx_' || replace(schema_name, '-', '_') || '_documents_legal_hold');
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS legal_hold_actioned_at', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS legal_hold_actioned_by', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS legal_hold_note', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP COLUMN IF EXISTS legal_hold', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM remove_document_legal_hold_workflow(tenant_schema);
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS add_document_legal_hold_workflow(TEXT);
DROP FUNCTION IF EXISTS remove_document_legal_hold_workflow(TEXT);
