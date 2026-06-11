-- Rollback migration 053: remove quote and order delivery email template types.

CREATE OR REPLACE FUNCTION revert_email_template_type_constraint(schema_name TEXT)
RETURNS void AS $$
BEGIN
    IF to_regclass(format('%I.email_templates', schema_name)) IS NULL THEN
        RETURN;
    END IF;

    EXECUTE format('
        DELETE FROM %I.email_templates
        WHERE template_type IN (''QUOTE_SEND'', ''ORDER_CONFIRM'')
    ', schema_name);

    EXECUTE format('
        ALTER TABLE %I.email_templates
        DROP CONSTRAINT IF EXISTS email_templates_template_type_check
    ', schema_name);

    EXECUTE format('
        ALTER TABLE %I.email_templates
        ADD CONSTRAINT email_templates_template_type_check
        CHECK (template_type IN (
            ''INVOICE_SEND'',
            ''INVOICE_REMINDER'',
            ''PAYMENT_RECEIPT'',
            ''OVERDUE_REMINDER'',
            ''WELCOME'',
            ''CUSTOM'',
            ''PAYMENT_DUE_SOON'',
            ''PAYMENT_DUE_TODAY''
        )) NOT VALID
    ', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM revert_email_template_type_constraint(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_email_tables_only(schema_name TEXT)
RETURNS void AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.email_templates (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            template_type VARCHAR(50) NOT NULL,
            subject TEXT NOT NULL,
            body_html TEXT NOT NULL,
            body_text TEXT,
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE (tenant_id, template_type)
        )', schema_name);

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.email_log (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            email_type VARCHAR(50) NOT NULL,
            recipient_email VARCHAR(255) NOT NULL,
            recipient_name VARCHAR(255),
            subject TEXT NOT NULL,
            status VARCHAR(20) DEFAULT ''PENDING'',
            sent_at TIMESTAMPTZ,
            error_message TEXT,
            related_id UUID,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name);

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_email_log_tenant ON %I.email_log(tenant_id)',
        schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_email_log_status ON %I.email_log(status)',
        schema_name);
END;
$$ LANGUAGE plpgsql;

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

DROP FUNCTION IF EXISTS sync_email_template_type_constraint(TEXT);
DROP FUNCTION IF EXISTS revert_email_template_type_constraint(TEXT);
