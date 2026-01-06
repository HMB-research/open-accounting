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

-- Drop helper functions
DROP FUNCTION IF EXISTS add_kmd_tables_to_schema(TEXT);
DROP FUNCTION IF EXISTS fix_email_log_schema(TEXT);
