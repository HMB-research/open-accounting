-- Migration 055: Track inventory reservations by lot, serial, and expiry metadata

CREATE OR REPLACE FUNCTION add_inventory_lot_reservations(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.inventory_lot_reservations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            product_id UUID NOT NULL REFERENCES %I.products(id) ON DELETE CASCADE,
            warehouse_id UUID NOT NULL REFERENCES %I.warehouses(id) ON DELETE CASCADE,
            lot_number VARCHAR(100) NOT NULL DEFAULT '''',
            serial_number VARCHAR(100) NOT NULL DEFAULT '''',
            expiry_date VARCHAR(20) NOT NULL DEFAULT '''',
            quantity NUMERIC(18,6) NOT NULL DEFAULT 0,
            reason TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID,
            CHECK (quantity >= 0)
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format(
        'CREATE UNIQUE INDEX IF NOT EXISTS %I ON %I.inventory_lot_reservations(tenant_id, product_id, warehouse_id, lot_number, serial_number, expiry_date)',
        'idx_' || replace(schema_name, '-', '_') || '_lot_reservation_key',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.inventory_lot_reservations(tenant_id, product_id, warehouse_id) WHERE quantity > 0',
        'idx_' || replace(schema_name, '-', '_') || '_lot_reservation_active',
        schema_name
    );
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_inventory_lot_reservations(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_vat_columns_to_journal_lines(schema_name);
    PERFORM add_payment_reversal_columns(schema_name);
    PERFORM add_reconciliation_tables_to_schema(schema_name);
    PERFORM add_recurring_tables_to_schema(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM add_fixed_asset_disposal_journal_links(schema_name);
    PERFORM create_inventory_tables(schema_name);
    PERFORM add_inventory_movement_tracking_metadata(schema_name);
    PERFORM add_inventory_lot_reservations(schema_name);
    PERFORM add_payroll_tables(schema_name);
    PERFORM add_leave_management_tables(schema_name);
    PERFORM create_email_tables_only(schema_name);
    PERFORM add_kmd_tables_to_schema(schema_name);
    PERFORM fix_email_log_schema(schema_name);
    PERFORM add_reminder_rules_to_schema(schema_name);
    PERFORM sync_email_template_type_constraint(schema_name);
    PERFORM add_interest_tables(schema_name);
    PERFORM add_document_tables(schema_name);
    PERFORM add_document_review_workflow(schema_name);
    PERFORM add_bank_transaction_review_columns(schema_name);
    PERFORM add_close_pack_document_entity(schema_name);
    PERFORM add_order_stock_reservations(schema_name);
    PERFORM add_journal_entry_evidence_requirement(schema_name);
    PERFORM add_journal_entry_templates(schema_name);
    PERFORM add_journal_entry_template_recurrence(schema_name);
    PERFORM add_bank_match_rules(schema_name);
    PERFORM add_invoice_vat_treatment(schema_name);
    PERFORM add_expense_tables(schema_name);
    PERFORM add_commercial_document_entities(schema_name);
    PERFORM add_leave_record_document_entity(schema_name);
    PERFORM add_cost_center_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
