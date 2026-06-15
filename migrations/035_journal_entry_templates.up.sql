-- Migration 035: Journal entry templates

CREATE OR REPLACE FUNCTION add_journal_entry_templates(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.journal_entry_templates (
            id UUID PRIMARY KEY,
            tenant_id UUID NOT NULL,
            name VARCHAR(120) NOT NULL,
            description TEXT NOT NULL DEFAULT '''',
            reference VARCHAR(100),
            requires_evidence BOOLEAN NOT NULL DEFAULT FALSE,
            is_active BOOLEAN NOT NULL DEFAULT TRUE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, name)
        )
    ', schema_name);

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.journal_entry_template_lines (
            id UUID PRIMARY KEY,
            template_id UUID NOT NULL REFERENCES %I.journal_entry_templates(id) ON DELETE CASCADE,
            line_number INTEGER NOT NULL,
            account_id UUID NOT NULL REFERENCES %I.accounts(id),
            description TEXT,
            debit_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            credit_amount NUMERIC(28,8) NOT NULL DEFAULT 0,
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            CHECK (debit_amount >= 0 AND credit_amount >= 0),
            CHECK (NOT (debit_amount > 0 AND credit_amount > 0))
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_jet_tenant_active
        ON %I.journal_entry_templates(tenant_id, is_active)
    ', replace(schema_name, '-', '_'), schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_jet_name
        ON %I.journal_entry_templates(tenant_id, name)
    ', replace(schema_name, '-', '_'), schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_%s_jet_lines_template
        ON %I.journal_entry_template_lines(template_id, line_number)
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
        PERFORM add_journal_entry_templates(tenant_schema);
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
    PERFORM add_journal_entry_evidence_requirement(schema_name);
    PERFORM add_journal_entry_templates(schema_name);
END;
$$ LANGUAGE plpgsql;
