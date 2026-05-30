-- Migration 045: Add lot, serial, and expiry metadata to inventory movements

CREATE OR REPLACE FUNCTION add_inventory_movement_tracking_metadata(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    BEGIN
        EXECUTE format(
            'ALTER TABLE %I.inventory_movements
                ADD COLUMN IF NOT EXISTS lot_number VARCHAR(100),
                ADD COLUMN IF NOT EXISTS serial_number VARCHAR(100),
                ADD COLUMN IF NOT EXISTS expiry_date DATE',
            schema_name
        );

        EXECUTE format(
            'CREATE INDEX IF NOT EXISTS %I ON %I.inventory_movements(product_id, lot_number) WHERE lot_number IS NOT NULL',
            'idx_' || replace(schema_name, '-', '_') || '_inv_mov_lot',
            schema_name
        );
        EXECUTE format(
            'CREATE INDEX IF NOT EXISTS %I ON %I.inventory_movements(product_id, serial_number) WHERE serial_number IS NOT NULL',
            'idx_' || replace(schema_name, '-', '_') || '_inv_mov_serial',
            schema_name
        );
    EXCEPTION WHEN undefined_table THEN
        NULL;
    END;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_inventory_movement_tracking_metadata(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM create_inventory_tables(schema_name);
    PERFORM add_inventory_movement_tracking_metadata(schema_name);
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
    PERFORM add_journal_entry_evidence_requirement(schema_name);
    PERFORM add_journal_entry_templates(schema_name);
    PERFORM add_journal_entry_template_recurrence(schema_name);
    PERFORM add_bank_match_rules(schema_name);
    PERFORM add_invoice_vat_treatment(schema_name);
    PERFORM add_expense_tables(schema_name);
    PERFORM add_commercial_document_entities(schema_name);
END;
$$ LANGUAGE plpgsql;
