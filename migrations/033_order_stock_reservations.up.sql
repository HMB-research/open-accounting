-- Migration 033: Persist order-level stock reservations

CREATE OR REPLACE FUNCTION add_order_stock_reservations(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    IF to_regclass(format('%I.orders', schema_name)) IS NULL
        OR to_regclass(format('%I.products', schema_name)) IS NULL
        OR to_regclass(format('%I.warehouses', schema_name)) IS NULL THEN
        RETURN;
    END IF;

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.order_stock_reservations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            order_id UUID NOT NULL REFERENCES %I.orders(id) ON DELETE CASCADE,
            product_id UUID NOT NULL REFERENCES %I.products(id) ON DELETE CASCADE,
            warehouse_id UUID NOT NULL REFERENCES %I.warehouses(id) ON DELETE CASCADE,
            quantity DECIMAL(15,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
            status VARCHAR(20) NOT NULL DEFAULT ''RESERVED'' CHECK (status IN (''RESERVED'', ''RELEASED'')),
            reason TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            released_at TIMESTAMPTZ,
            released_by UUID,
            UNIQUE (tenant_id, order_id, product_id, warehouse_id)
        )
    ', schema_name, schema_name, schema_name, schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_order_stock_res_order
        ON %I.order_stock_reservations(tenant_id, order_id)
    ', replace(schema_name, '-', '_'), schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_order_stock_res_product
        ON %I.order_stock_reservations(tenant_id, product_id, warehouse_id)
    ', replace(schema_name, '-', '_'), schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_order_stock_res_status
        ON %I.order_stock_reservations(tenant_id, status)
    ', replace(schema_name, '-', '_'), schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_order_stock_reservations(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM create_inventory_tables(schema_name);
    PERFORM add_payroll_tables(schema_name);
    PERFORM add_leave_management_tables(schema_name);
    PERFORM create_email_tables_only(schema_name);
    PERFORM add_kmd_tables_to_schema(schema_name);
    PERFORM fix_email_log_schema(schema_name);
    PERFORM add_reminder_rules_to_schema(schema_name);
    PERFORM add_interest_tables(schema_name);
    PERFORM add_document_tables(schema_name);
    PERFORM add_document_review_workflow(schema_name);
    PERFORM add_bank_transaction_review_columns(schema_name);
    PERFORM add_close_pack_document_entity(schema_name);
    PERFORM add_order_stock_reservations(schema_name);
END;
$$ LANGUAGE plpgsql;
