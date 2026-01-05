-- Migration 015: Fixed Assets tables
-- This migration adds support for fixed asset management and depreciation tracking

-- =============================================================================
-- FUNCTION: Add fixed assets tables to tenant schema
-- =============================================================================

CREATE OR REPLACE FUNCTION add_fixed_assets_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- =========================================================================
    -- ASSET CATEGORIES (Põhivara kategooriad)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.asset_categories (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            description TEXT,
            depreciation_method VARCHAR(20) NOT NULL DEFAULT ''STRAIGHT_LINE'' CHECK (depreciation_method IN (''STRAIGHT_LINE'', ''DECLINING_BALANCE'', ''UNITS_OF_PRODUCTION'')),
            default_useful_life_months INTEGER NOT NULL DEFAULT 60,
            default_residual_value_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
            asset_account_id UUID,
            depreciation_expense_account_id UUID,
            accumulated_depreciation_account_id UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, name)
        )
    ', schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_asset_cat_tenant ON %I.asset_categories(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- FIXED ASSETS (Põhivarad)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.fixed_assets (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            asset_number VARCHAR(50) NOT NULL,
            name VARCHAR(200) NOT NULL,
            description TEXT,
            category_id UUID REFERENCES %I.asset_categories(id),
            status VARCHAR(20) NOT NULL DEFAULT ''ACTIVE'' CHECK (status IN (''DRAFT'', ''ACTIVE'', ''DISPOSED'', ''SOLD'')),

            -- Purchase Information
            purchase_date DATE NOT NULL,
            purchase_cost NUMERIC(28,8) NOT NULL,
            supplier_id UUID REFERENCES %I.contacts(id),
            invoice_id UUID,
            serial_number VARCHAR(100),
            location VARCHAR(200),

            -- Depreciation Settings
            depreciation_method VARCHAR(20) NOT NULL DEFAULT ''STRAIGHT_LINE'' CHECK (depreciation_method IN (''STRAIGHT_LINE'', ''DECLINING_BALANCE'', ''UNITS_OF_PRODUCTION'')),
            useful_life_months INTEGER NOT NULL DEFAULT 60,
            residual_value NUMERIC(28,8) NOT NULL DEFAULT 0,
            depreciation_start_date DATE,

            -- Calculated Values (updated by depreciation runs)
            accumulated_depreciation NUMERIC(28,8) NOT NULL DEFAULT 0,
            book_value NUMERIC(28,8) NOT NULL,
            last_depreciation_date DATE,

            -- Disposal Information
            disposal_date DATE,
            disposal_method VARCHAR(20) CHECK (disposal_method IN (''SOLD'', ''SCRAPPED'', ''DONATED'', ''LOST'')),
            disposal_proceeds NUMERIC(28,8) DEFAULT 0,
            disposal_notes TEXT,

            -- Account Links
            asset_account_id UUID,
            depreciation_expense_account_id UUID,
            accumulated_depreciation_account_id UUID,

            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, asset_number)
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_fassets_tenant ON %I.fixed_assets(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_fassets_category ON %I.fixed_assets(category_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_fassets_status ON %I.fixed_assets(status)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_fassets_purchase_date ON %I.fixed_assets(purchase_date)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- DEPRECIATION ENTRIES (Amortisatsiooni kanded)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.depreciation_entries (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            asset_id UUID NOT NULL REFERENCES %I.fixed_assets(id) ON DELETE CASCADE,
            depreciation_date DATE NOT NULL,
            period_start DATE NOT NULL,
            period_end DATE NOT NULL,
            depreciation_amount NUMERIC(28,8) NOT NULL,
            accumulated_total NUMERIC(28,8) NOT NULL,
            book_value_after NUMERIC(28,8) NOT NULL,
            journal_entry_id UUID,
            notes TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            UNIQUE (asset_id, period_start, period_end)
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_deprec_asset ON %I.depreciation_entries(asset_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_deprec_date ON %I.depreciation_entries(depreciation_date)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- ASSET MAINTENANCE LOG (Hoolduse logi)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.asset_maintenance (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            asset_id UUID NOT NULL REFERENCES %I.fixed_assets(id) ON DELETE CASCADE,
            maintenance_date DATE NOT NULL,
            maintenance_type VARCHAR(50) NOT NULL,
            description TEXT NOT NULL,
            cost NUMERIC(28,8) DEFAULT 0,
            performed_by VARCHAR(200),
            next_maintenance_date DATE,
            invoice_id UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_asset_maint_asset ON %I.asset_maintenance(asset_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_asset_maint_date ON %I.asset_maintenance(maintenance_date)',
        replace(schema_name, '-', '_'), schema_name);

END;
$$ LANGUAGE plpgsql;

-- Apply to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_fixed_assets_tables(tenant_record.schema_name);
    END LOOP;
END $$;

-- Update create_accounting_tables to include fixed assets for new tenants
CREATE OR REPLACE FUNCTION create_accounting_tables_with_fixed_assets(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Call existing functions
    PERFORM create_accounting_tables(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
