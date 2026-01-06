-- Migration 014 Down: Remove quotes and orders tables

-- Drop tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.order_lines CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.orders CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.quote_lines CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.quotes CASCADE', tenant_record.schema_name);
    END LOOP;
END $$;

-- Drop the function
DROP FUNCTION IF EXISTS add_quotes_and_orders_tables(TEXT);
