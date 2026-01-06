-- Inventory/Warehouse Management Module Migration
-- Adds warehouses, stock levels, and updates inventory movements
-- Updates existing products table with inventory-specific columns

-- Create function to set up inventory tables in a tenant schema
CREATE OR REPLACE FUNCTION create_inventory_tables(schema_name text)
RETURNS void AS $$
BEGIN
    -- Product Categories Table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.product_categories (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            name VARCHAR(255) NOT NULL,
            description TEXT,
            parent_id UUID REFERENCES %I.product_categories(id) ON DELETE SET NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )', schema_name, schema_name);

    -- Create indexes for product_categories
    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_product_categories_tenant
        ON %I.product_categories(tenant_id)', schema_name);

    -- Add missing inventory columns to existing products table
    -- (Products table already exists from 001_initial_schema with 'code' column)
    BEGIN
        EXECUTE format('
            ALTER TABLE %I.products
            ADD COLUMN IF NOT EXISTS category_id UUID,
            ADD COLUMN IF NOT EXISTS min_stock_level DECIMAL(15,3) NOT NULL DEFAULT 0,
            ADD COLUMN IF NOT EXISTS current_stock DECIMAL(15,3) NOT NULL DEFAULT 0,
            ADD COLUMN IF NOT EXISTS reorder_point DECIMAL(15,3) NOT NULL DEFAULT 0,
            ADD COLUMN IF NOT EXISTS barcode VARCHAR(100),
            ADD COLUMN IF NOT EXISTS supplier_id UUID,
            ADD COLUMN IF NOT EXISTS lead_time_days INTEGER DEFAULT 0
        ', schema_name);
    EXCEPTION WHEN others THEN
        -- Ignore errors if columns already exist
        NULL;
    END;

    -- Warehouses Table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.warehouses (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            code VARCHAR(50) NOT NULL,
            name VARCHAR(255) NOT NULL,
            address TEXT,
            is_default BOOLEAN NOT NULL DEFAULT false,
            is_active BOOLEAN NOT NULL DEFAULT true,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )', schema_name);

    -- Create indexes for warehouses
    EXECUTE format('
        CREATE UNIQUE INDEX IF NOT EXISTS idx_warehouses_code
        ON %I.warehouses(tenant_id, code)', schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_warehouses_tenant
        ON %I.warehouses(tenant_id)', schema_name);

    -- Add warehouse_id to inventory_movements if it exists without it
    BEGIN
        EXECUTE format('
            ALTER TABLE %I.inventory_movements
            ADD COLUMN IF NOT EXISTS warehouse_id UUID,
            ADD COLUMN IF NOT EXISTS reference VARCHAR(255),
            ADD COLUMN IF NOT EXISTS to_warehouse_id UUID,
            ADD COLUMN IF NOT EXISTS movement_date TIMESTAMPTZ NOT NULL DEFAULT NOW()
        ', schema_name);
    EXCEPTION WHEN others THEN
        NULL;
    END;

    -- Stock Levels Table (tracks stock per product per warehouse)
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.stock_levels (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            product_id UUID NOT NULL REFERENCES %I.products(id) ON DELETE CASCADE,
            warehouse_id UUID REFERENCES %I.warehouses(id) ON DELETE CASCADE,
            quantity DECIMAL(15,3) NOT NULL DEFAULT 0,
            reserved_qty DECIMAL(15,3) NOT NULL DEFAULT 0,
            available_qty DECIMAL(15,3) NOT NULL DEFAULT 0,
            last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )', schema_name, schema_name, schema_name);

    -- Create unique index for stock_levels
    BEGIN
        EXECUTE format('
            CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_levels_product_warehouse
            ON %I.stock_levels(tenant_id, product_id, warehouse_id)', schema_name);
    EXCEPTION WHEN others THEN
        NULL;
    END;

    -- Create indexes for stock_levels
    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_stock_levels_product
        ON %I.stock_levels(product_id)', schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_stock_levels_warehouse
        ON %I.stock_levels(warehouse_id)', schema_name);

    -- SKU Sequence Table for auto-generating SKU codes
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.sku_sequence (
            tenant_id UUID PRIMARY KEY REFERENCES public.tenants(id) ON DELETE CASCADE,
            last_number INTEGER NOT NULL DEFAULT 0
        )', schema_name);

END;
$$ LANGUAGE plpgsql;

-- Apply inventory tables to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT id, schema_name FROM public.tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM create_inventory_tables(tenant_record.schema_name);
    END LOOP;
END $$;
