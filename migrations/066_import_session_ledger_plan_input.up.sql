-- Migration 066: Persist minimal, normalized ledger metadata for deterministic
-- receipt-only import planning. This is deliberately separate from 064/065:
-- those migrations may already be applied on deployed databases.

CREATE OR REPLACE FUNCTION add_import_session_ledger_plan_input(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    IF to_regclass(format('%I.import_sessions', schema_name)) IS NULL THEN
        RETURN;
    END IF;

    EXECUTE format(
        'ALTER TABLE %I.import_sessions ADD COLUMN IF NOT EXISTS ledger_plan_input JSONB NOT NULL DEFAULT ''[]''::JSONB',
        schema_name
    );
END;
$$ LANGUAGE plpgsql;

-- Redefine the original table helper forward-only so tenants provisioned after
-- this migration receive the same receipt schema as upgraded tenants. The
-- retained value is normalized journal metadata only; source payloads and
-- credentials remain outside Open Accounting.
CREATE OR REPLACE FUNCTION add_import_session_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.import_sessions (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            provider VARCHAR(64) NOT NULL,
            source_company_id VARCHAR(255) NOT NULL,
            schema_version VARCHAR(32) NOT NULL,
            package_sha256 VARCHAR(64) NOT NULL,
            status VARCHAR(32) NOT NULL,
            record_count INTEGER NOT NULL DEFAULT 0 CHECK (record_count >= 0),
            entity_counts JSONB NOT NULL DEFAULT ''{}''::JSONB,
            validation JSONB NOT NULL DEFAULT ''{}''::JSONB,
            created_by TEXT NOT NULL DEFAULT '''',
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT import_sessions_package_sha256_check CHECK (package_sha256 ~ ''^[0-9a-f]{64}$''),
            CONSTRAINT import_sessions_package_identity_unique UNIQUE (tenant_id, provider, source_company_id, package_sha256)
        )
    ', schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.import_sessions(tenant_id, created_at DESC)',
        'idx_' || replace(schema_name, '-', '_') || '_import_sessions_tenant_created',
        schema_name
    );

    PERFORM add_import_session_ledger_plan_input(schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_import_session_ledger_plan_input(tenant_schema);
    END LOOP;
END $$;
