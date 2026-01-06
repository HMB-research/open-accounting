-- Rollback leave management tables
-- Migration: 017_leave_management

-- Function to remove leave management tables from a tenant schema
CREATE OR REPLACE FUNCTION remove_leave_management_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('DROP TABLE IF EXISTS %I.leave_records CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.leave_balances CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.absence_types CASCADE', schema_name);
END;
$$ LANGUAGE plpgsql;

-- Remove from all tenant schemas
DO $$
DECLARE
    schema_rec RECORD;
BEGIN
    FOR schema_rec IN
        SELECT schema_name
        FROM information_schema.schemata
        WHERE schema_name LIKE 'tenant_%'
           OR schema_name = 'schema_demo1'
           OR schema_name = 'schema_demo2'
           OR schema_name = 'schema_demo3'
           OR schema_name = 'schema_demo4'
    LOOP
        PERFORM remove_leave_management_tables(schema_rec.schema_name);
    END LOOP;
END;
$$;

-- Drop the helper function
DROP FUNCTION IF EXISTS add_leave_management_tables(TEXT);
DROP FUNCTION IF EXISTS remove_leave_management_tables(TEXT);
