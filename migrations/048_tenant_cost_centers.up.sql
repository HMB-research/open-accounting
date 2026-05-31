-- Migration 048: Tenant-scoped cost center tables

CREATE OR REPLACE FUNCTION add_cost_center_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.cost_centers (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            code VARCHAR(20) NOT NULL,
            name VARCHAR(200) NOT NULL,
            description TEXT,
            parent_id UUID REFERENCES %I.cost_centers(id) ON DELETE SET NULL,
            is_active BOOLEAN NOT NULL DEFAULT true,
            budget_amount NUMERIC(15,2),
            budget_period VARCHAR(20) DEFAULT ''ANNUAL'',
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE(tenant_id, code)
        )
    ', schema_name, schema_name);

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.cost_allocations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            cost_center_id UUID NOT NULL REFERENCES %I.cost_centers(id) ON DELETE CASCADE,
            journal_entry_line_id UUID NOT NULL,
            amount NUMERIC(15,2) NOT NULL,
            allocation_percentage NUMERIC(5,2),
            allocation_date DATE NOT NULL,
            notes TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )
    ', schema_name, schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.cost_centers(tenant_id)',
        'idx_' || replace(schema_name, '-', '_') || '_cost_centers_tenant',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.cost_centers(parent_id)',
        'idx_' || replace(schema_name, '-', '_') || '_cost_centers_parent',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.cost_centers(tenant_id, is_active)',
        'idx_' || replace(schema_name, '-', '_') || '_cost_centers_active',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.cost_allocations(cost_center_id)',
        'idx_' || replace(schema_name, '-', '_') || '_cost_allocations_center',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.cost_allocations(allocation_date)',
        'idx_' || replace(schema_name, '-', '_') || '_cost_allocations_date',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.cost_allocations(journal_entry_line_id)',
        'idx_' || replace(schema_name, '-', '_') || '_cost_allocations_journal_line',
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
        PERFORM add_cost_center_tables(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
    PERFORM add_quotes_and_orders_tables(schema_name);
    PERFORM add_fixed_assets_tables(schema_name);
    PERFORM add_fixed_asset_disposal_journal_links(schema_name);
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
    PERFORM add_leave_record_document_entity(schema_name);
    PERFORM add_cost_center_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
