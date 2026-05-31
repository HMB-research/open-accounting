-- Migration 050: make recurring invoice schema provisioning migration-owned.
--
-- Runtime recurring services use the GORM repository and should not issue DDL.
-- This normalizes the recurring table shape to the current ORM models and wires
-- it back into create_tenant_schema for newly created tenants.

CREATE OR REPLACE FUNCTION add_recurring_tables_to_schema(schema_name TEXT)
RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.recurring_invoices (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            contact_id UUID NOT NULL REFERENCES %I.contacts(id),
            invoice_type VARCHAR(20) NOT NULL DEFAULT ''SALES'',
            currency VARCHAR(3) NOT NULL DEFAULT ''EUR'',
            frequency VARCHAR(20) NOT NULL CHECK (frequency IN (''WEEKLY'', ''BIWEEKLY'', ''MONTHLY'', ''QUARTERLY'', ''YEARLY'')),
            start_date DATE NOT NULL,
            end_date DATE,
            next_generation_date DATE NOT NULL,
            payment_terms_days INTEGER NOT NULL DEFAULT 14,
            reference TEXT,
            notes TEXT,
            is_active BOOLEAN NOT NULL DEFAULT true,
            last_generated_at TIMESTAMPTZ,
            generated_count INTEGER NOT NULL DEFAULT 0,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            created_by UUID NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            send_email_on_generation BOOLEAN DEFAULT false,
            email_template_type VARCHAR(50) DEFAULT ''INVOICE_SEND'',
            recipient_email_override TEXT,
            attach_pdf_to_email BOOLEAN DEFAULT true,
            email_subject_override TEXT,
            email_message TEXT
        )
    ', schema_name, schema_name);

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.recurring_invoice_lines (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            recurring_invoice_id UUID NOT NULL REFERENCES %I.recurring_invoices(id) ON DELETE CASCADE,
            line_number INTEGER NOT NULL,
            description TEXT NOT NULL,
            quantity NUMERIC(18,6) NOT NULL DEFAULT 1,
            unit VARCHAR(20),
            unit_price NUMERIC(28,8) NOT NULL,
            discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
            vat_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
            account_id UUID REFERENCES %I.accounts(id),
            product_id UUID
        )
    ', schema_name, schema_name, schema_name);

    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS invoice_type VARCHAR(20) NOT NULL DEFAULT ''SALES''', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS currency VARCHAR(3) NOT NULL DEFAULT ''EUR''', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS reference TEXT', schema_name);

    EXECUTE format('
        DO $normalize$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''invoices_generated''
            ) AND NOT EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''generated_count''
            ) THEN
                ALTER TABLE %I.recurring_invoices RENAME COLUMN invoices_generated TO generated_count;
            ELSIF EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''invoices_generated''
            ) AND EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''generated_count''
            ) THEN
                UPDATE %I.recurring_invoices
                SET generated_count = CASE
                    WHEN generated_count IS NULL OR generated_count = 0 THEN COALESCE(invoices_generated, 0)
                    ELSE generated_count
                END
                WHERE invoices_generated IS NOT NULL;
                ALTER TABLE %I.recurring_invoices DROP COLUMN invoices_generated;
            END IF;
        END $normalize$
    ', schema_name, schema_name, schema_name, schema_name, schema_name, schema_name, schema_name);

    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS generated_count INTEGER NOT NULL DEFAULT 0', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ALTER COLUMN generated_count SET DEFAULT 0', schema_name);
    EXECUTE format('UPDATE %I.recurring_invoices SET generated_count = 0 WHERE generated_count IS NULL', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ALTER COLUMN generated_count SET NOT NULL', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS send_email_on_generation BOOLEAN DEFAULT false', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS email_template_type VARCHAR(50) DEFAULT ''INVOICE_SEND''', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS recipient_email_override TEXT', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS attach_pdf_to_email BOOLEAN DEFAULT true', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS email_subject_override TEXT', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoices ADD COLUMN IF NOT EXISTS email_message TEXT', schema_name);

    EXECUTE format('ALTER TABLE %I.recurring_invoice_lines DROP COLUMN IF EXISTS tenant_id', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoice_lines ADD COLUMN IF NOT EXISTS unit VARCHAR(20)', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoice_lines ADD COLUMN IF NOT EXISTS discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0', schema_name);
    EXECUTE format('ALTER TABLE %I.recurring_invoice_lines ADD COLUMN IF NOT EXISTS product_id UUID', schema_name);

    EXECUTE format('
        ALTER TABLE %I.invoices
        ADD COLUMN IF NOT EXISTS recurring_invoice_id UUID REFERENCES %I.recurring_invoices(id),
        ADD COLUMN IF NOT EXISTS last_email_sent_at TIMESTAMPTZ,
        ADD COLUMN IF NOT EXISTS last_email_status VARCHAR(20),
        ADD COLUMN IF NOT EXISTS last_email_log_id UUID
    ', schema_name, schema_name);

    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.recurring_invoices(tenant_id)',
        'idx_' || replace(schema_name, '-', '_') || '_recurring_tenant',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.recurring_invoices(next_generation_date) WHERE is_active = true',
        'idx_' || replace(schema_name, '-', '_') || '_recurring_next_date',
        schema_name
    );
    EXECUTE format(
        'CREATE INDEX IF NOT EXISTS %I ON %I.recurring_invoice_lines(recurring_invoice_id)',
        'idx_' || replace(schema_name, '-', '_') || '_recurring_lines',
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
        PERFORM add_recurring_tables_to_schema(tenant_schema);
    END LOOP;
END $$;

CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    PERFORM create_accounting_tables(schema_name);
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
