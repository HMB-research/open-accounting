-- Migration: Remove KMD declarations tables and email_log related_id column

-- Remove KMD tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.kmd_rows CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.kmd_declarations CASCADE', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.email_log DROP COLUMN IF EXISTS related_id', tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Restore create_tenant_schema to previous version (without KMD tables)
-- Note: This restores to the version from migration 007 which didn't include email tables call
-- The add_email_tables_to_schema function was added in migration 004 but not called in create_tenant_schema
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Create the schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    -- Create core accounting tables (existing)
    PERFORM create_accounting_tables(schema_name);

    -- Create payroll tables
    PERFORM add_payroll_tables(schema_name);

    -- Create email tables (added in migration 004)
    PERFORM add_email_tables_to_schema(schema_name);
END;
$$ LANGUAGE plpgsql;

-- Drop helper functions
DROP FUNCTION IF EXISTS add_kmd_tables_to_schema(TEXT);
DROP FUNCTION IF EXISTS fix_email_log_schema(TEXT);
