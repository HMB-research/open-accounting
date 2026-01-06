-- Migration 015 Down: Remove fixed assets tables

-- Drop the wrapper function
DROP FUNCTION IF EXISTS create_accounting_tables_with_fixed_assets(TEXT);

-- Drop the add function
DROP FUNCTION IF EXISTS add_fixed_assets_tables(TEXT);

-- Drop tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.asset_maintenance CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.depreciation_entries CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.fixed_assets CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.asset_categories CASCADE', tenant_record.schema_name);
    END LOOP;
END $$;
