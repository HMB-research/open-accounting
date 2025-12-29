-- Rollback payroll module
-- Migration: 007_payroll

-- Function to remove payroll tables from a tenant schema
CREATE OR REPLACE FUNCTION remove_payroll_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('DROP TABLE IF EXISTS %I.tsd_rows CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.tsd_declarations CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.payslips CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.payroll_runs CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.salary_components CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.employees CASCADE', schema_name);
END;
$$ LANGUAGE plpgsql;

-- Remove from all tenant schemas
DO $$
DECLARE
    schema_record RECORD;
BEGIN
    FOR schema_record IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM remove_payroll_tables(schema_record.schema_name);
    END LOOP;
END $$;

-- Drop the helper function
DROP FUNCTION IF EXISTS add_payroll_tables(TEXT);
DROP FUNCTION IF EXISTS remove_payroll_tables(TEXT);
