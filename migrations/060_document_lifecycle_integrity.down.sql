-- Migration 060 rollback: Remove document lifecycle integrity constraints

CREATE OR REPLACE FUNCTION remove_document_lifecycle_integrity(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_legal_hold_audit_check', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_lifecycle_actioned_at_check', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_superseded_link_check', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_legal_hold_actioned_by_user_fk', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_lifecycle_actioned_by_user_fk', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_reviewed_by_user_fk', schema_name);
    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_superseded_by_document_fk', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM remove_document_lifecycle_integrity(tenant_schema);
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS add_document_lifecycle_integrity(TEXT);
DROP FUNCTION IF EXISTS remove_document_lifecycle_integrity(TEXT);
