-- Rollback migration 022: Remove interest calculations tables

-- Function to remove interest tables from a tenant schema
CREATE OR REPLACE FUNCTION drop_interest_tables(schema_name TEXT) RETURNS void AS $$
BEGIN
    EXECUTE format('DROP TABLE IF EXISTS %I.invoice_interest CASCADE', schema_name);
END;
$$ LANGUAGE plpgsql;

-- Remove from all existing tenant schemas
DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM drop_interest_tables(tenant_schema);
    END LOOP;
END $$;

-- Drop the helper function
DROP FUNCTION IF EXISTS add_interest_tables(TEXT);
DROP FUNCTION IF EXISTS drop_interest_tables(TEXT);
