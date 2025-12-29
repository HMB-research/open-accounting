-- Rollback: Remove email system tables

-- Remove tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.email_log CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.email_templates CASCADE', tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Drop the helper function
DROP FUNCTION IF EXISTS add_email_tables_to_schema(TEXT);
