-- Migration 014: Quotes and Orders tables
-- This migration adds support for quotes/offers and orders

-- =============================================================================
-- FUNCTION: Add quotes and orders tables to tenant schema
-- =============================================================================

CREATE OR REPLACE FUNCTION add_quotes_and_orders_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- =========================================================================
    -- QUOTES (Pakkumised)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.quotes (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            quote_number VARCHAR(50) NOT NULL,
            contact_id UUID NOT NULL REFERENCES %I.contacts(id),
            quote_date DATE NOT NULL,
            valid_until DATE,
            status VARCHAR(20) NOT NULL DEFAULT ''DRAFT'' CHECK (status IN (''DRAFT'', ''SENT'', ''ACCEPTED'', ''REJECTED'', ''EXPIRED'', ''CONVERTED'')),
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            subtotal NUMERIC(28,8) NOT NULL DEFAULT 0,
            vat_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            total NUMERIC(28,8) NOT NULL DEFAULT 0,
            notes TEXT,
            converted_to_order_id UUID,
            converted_to_invoice_id UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, quote_number)
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_quotes_tenant ON %I.quotes(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_quotes_contact ON %I.quotes(contact_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_quotes_status ON %I.quotes(status)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_quotes_date ON %I.quotes(quote_date)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- QUOTE LINES
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.quote_lines (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            quote_id UUID NOT NULL REFERENCES %I.quotes(id) ON DELETE CASCADE,
            line_number INTEGER NOT NULL,
            description TEXT NOT NULL,
            quantity NUMERIC(18,6) NOT NULL DEFAULT 1,
            unit VARCHAR(20),
            unit_price NUMERIC(28,8) NOT NULL,
            discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
            vat_rate NUMERIC(5,2) NOT NULL,
            line_subtotal NUMERIC(28,8) NOT NULL,
            line_vat NUMERIC(28,8) NOT NULL,
            line_total NUMERIC(28,8) NOT NULL,
            product_id UUID,
            UNIQUE (quote_id, line_number)
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_quotel_quote ON %I.quote_lines(quote_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- ORDERS (Tellimused)
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.orders (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            order_number VARCHAR(50) NOT NULL,
            contact_id UUID NOT NULL REFERENCES %I.contacts(id),
            order_date DATE NOT NULL,
            expected_delivery DATE,
            status VARCHAR(20) NOT NULL DEFAULT ''PENDING'' CHECK (status IN (''PENDING'', ''CONFIRMED'', ''PROCESSING'', ''SHIPPED'', ''DELIVERED'', ''CANCELLED'')),
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            subtotal NUMERIC(28,8) NOT NULL DEFAULT 0,
            vat_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            total NUMERIC(28,8) NOT NULL DEFAULT 0,
            notes TEXT,
            quote_id UUID,
            converted_to_invoice_id UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, order_number)
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_orders_tenant ON %I.orders(tenant_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_orders_contact ON %I.orders(contact_id)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_orders_status ON %I.orders(status)',
        replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_orders_date ON %I.orders(order_date)',
        replace(schema_name, '-', '_'), schema_name);

    -- =========================================================================
    -- ORDER LINES
    -- =========================================================================

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.order_lines (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            order_id UUID NOT NULL REFERENCES %I.orders(id) ON DELETE CASCADE,
            line_number INTEGER NOT NULL,
            description TEXT NOT NULL,
            quantity NUMERIC(18,6) NOT NULL DEFAULT 1,
            unit VARCHAR(20),
            unit_price NUMERIC(28,8) NOT NULL,
            discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
            vat_rate NUMERIC(5,2) NOT NULL,
            line_subtotal NUMERIC(28,8) NOT NULL,
            line_vat NUMERIC(28,8) NOT NULL,
            line_total NUMERIC(28,8) NOT NULL,
            product_id UUID,
            UNIQUE (order_id, line_number)
        )
    ', schema_name, schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_orderl_order ON %I.order_lines(order_id)',
        replace(schema_name, '-', '_'), schema_name);

    -- Add foreign key references for converted documents
    BEGIN
        EXECUTE format('ALTER TABLE %I.quotes ADD CONSTRAINT fk_quotes_order
            FOREIGN KEY (converted_to_order_id) REFERENCES %I.orders(id)', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN
        -- Constraint already exists
    END;

    BEGIN
        EXECUTE format('ALTER TABLE %I.orders ADD CONSTRAINT fk_orders_quote
            FOREIGN KEY (quote_id) REFERENCES %I.quotes(id)', schema_name, schema_name);
    EXCEPTION WHEN duplicate_object THEN
        -- Constraint already exists
    END;

END;
$$ LANGUAGE plpgsql;

-- Apply to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_quotes_and_orders_tables(tenant_record.schema_name);
    END LOOP;
END $$;

-- Update create_accounting_tables to include quotes and orders for new tenants
-- This wraps the existing function to also call add_quotes_and_orders_tables
CREATE OR REPLACE FUNCTION create_accounting_tables_with_quotes(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Call original function (if it exists, this is a no-op for tables that already exist)
    PERFORM create_accounting_tables(schema_name);
    -- Add quotes and orders tables
    PERFORM add_quotes_and_orders_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
