-- Migration 037: Bank auto-match rules

CREATE OR REPLACE FUNCTION add_bank_match_rules(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.bank_match_rules (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            tenant_id UUID NOT NULL,
            bank_account_id UUID REFERENCES %I.bank_accounts(id) ON DELETE CASCADE,
            name VARCHAR(120) NOT NULL,
            priority INTEGER NOT NULL DEFAULT 100,
            match_field VARCHAR(30) NOT NULL,
            pattern TEXT NOT NULL,
            min_confidence DOUBLE PRECISION NOT NULL DEFAULT 0.7,
            max_date_diff_days INTEGER NOT NULL DEFAULT 7,
            require_exact_amount BOOLEAN NOT NULL DEFAULT false,
            is_active BOOLEAN NOT NULL DEFAULT true,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT bank_match_rules_field_check CHECK (match_field IN (''DESCRIPTION'', ''REFERENCE'', ''COUNTERPARTY_NAME'', ''COUNTERPARTY_ACCOUNT'')),
            CONSTRAINT bank_match_rules_confidence_check CHECK (min_confidence >= 0 AND min_confidence <= 1),
            CONSTRAINT bank_match_rules_date_diff_check CHECK (max_date_diff_days >= 0 AND max_date_diff_days <= 90)
        )
    ', schema_name, schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_bank_match_rules_tenant ON %I.bank_match_rules(tenant_id, is_active, priority)',
        replace(schema_name, '-', '_'),
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS idx_%s_bank_match_rules_account ON %I.bank_match_rules(bank_account_id)',
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
        PERFORM add_bank_match_rules(tenant_schema);
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
END;
$$ LANGUAGE plpgsql;
