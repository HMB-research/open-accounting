-- Migration 040: Tenant expense tracking

CREATE OR REPLACE FUNCTION add_expense_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.expenses (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            expense_number VARCHAR(30) NOT NULL,
            expense_date DATE NOT NULL,
            merchant VARCHAR(200) NOT NULL,
            description TEXT NOT NULL DEFAULT '''',
            employee_id UUID REFERENCES %I.employees(id),
            contact_id UUID REFERENCES %I.contacts(id),
            expense_account_id UUID NOT NULL REFERENCES %I.accounts(id),
            payment_account_id UUID NOT NULL REFERENCES %I.accounts(id),
            amount NUMERIC(28,8) NOT NULL,
            currency CHAR(3) NOT NULL DEFAULT ''EUR'',
            exchange_rate NUMERIC(18,10) NOT NULL DEFAULT 1,
            base_amount NUMERIC(28,8) NOT NULL,
            requires_receipt BOOLEAN NOT NULL DEFAULT TRUE,
            status VARCHAR(20) NOT NULL DEFAULT ''DRAFT'',
            journal_entry_id UUID REFERENCES %I.journal_entries(id),
            submitted_at TIMESTAMPTZ,
            submitted_by UUID,
            approved_at TIMESTAMPTZ,
            approved_by UUID,
            rejected_at TIMESTAMPTZ,
            rejected_by UUID,
            rejection_reason TEXT,
            posted_at TIMESTAMPTZ,
            posted_by UUID,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE (tenant_id, expense_number),
            CONSTRAINT expenses_amount_check CHECK (amount > 0),
            CONSTRAINT expenses_exchange_rate_check CHECK (exchange_rate > 0),
            CONSTRAINT expenses_base_amount_check CHECK (base_amount > 0),
            CONSTRAINT expenses_status_check CHECK (status IN (''DRAFT'', ''SUBMITTED'', ''APPROVED'', ''REJECTED'', ''POSTED''))
        )
    ', schema_name, schema_name, schema_name, schema_name, schema_name, schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_expenses_tenant_status ON %I.expenses(tenant_id, status, expense_date DESC)',
        replace(schema_name, '-', '_'),
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_expenses_journal_entry ON %I.expenses(journal_entry_id)',
        replace(schema_name, '-', '_'),
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_expenses_employee ON %I.expenses(employee_id)',
        replace(schema_name, '-', '_'),
        schema_name
    );
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_expense_tables(tenant_schema);
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
    PERFORM add_journal_entry_template_recurrence(schema_name);
    PERFORM add_bank_match_rules(schema_name);
    PERFORM add_invoice_vat_treatment(schema_name);
    PERFORM add_expense_tables(schema_name);
END;
$$ LANGUAGE plpgsql;

