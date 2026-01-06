-- Inventory/Warehouse Management Module Rollback
-- Drops inventory-specific tables and columns from tenant schemas

-- Drop inventory tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT id, schema_name FROM public.tenants WHERE schema_name IS NOT NULL
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.inventory_movements CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.stock_levels CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.warehouses CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.sku_sequence CASCADE', tenant_record.schema_name);

        -- Remove added columns from products table
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS category_id', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS min_stock_level', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS current_stock', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS reorder_point', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS barcode', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS supplier_id', tenant_record.schema_name);
        EXECUTE format('ALTER TABLE %I.products DROP COLUMN IF EXISTS lead_time_days', tenant_record.schema_name);

        EXECUTE format('DROP TABLE IF EXISTS %I.product_categories CASCADE', tenant_record.schema_name);
    END LOOP;
END $$;

-- Drop the helper function
DROP FUNCTION IF EXISTS create_inventory_tables(text);
