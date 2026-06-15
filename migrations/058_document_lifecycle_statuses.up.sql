-- Migration 058: Add audited document lifecycle states

CREATE OR REPLACE FUNCTION add_document_lifecycle_workflow(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS lifecycle_status VARCHAR(20) NOT NULL DEFAULT ''ACTIVE''', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS lifecycle_note TEXT', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS superseded_by_document_id UUID', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS lifecycle_actioned_by UUID', schema_name);
    EXECUTE format('ALTER TABLE %I.documents ADD COLUMN IF NOT EXISTS lifecycle_actioned_at TIMESTAMPTZ', schema_name);

    EXECUTE format('UPDATE %I.documents SET lifecycle_status = ''ACTIVE'' WHERE lifecycle_status IS NULL OR btrim(lifecycle_status) = ''''', schema_name);

    EXECUTE format('ALTER TABLE %I.documents DROP CONSTRAINT IF EXISTS documents_lifecycle_status_check', schema_name);
    EXECUTE format(
        'ALTER TABLE %I.documents ADD CONSTRAINT documents_lifecycle_status_check CHECK (lifecycle_status IN (''ACTIVE'', ''SUPERSEDED'', ''ARCHIVED'', ''DISPOSED''))',
        schema_name
    );

    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_documents_lifecycle_status ON %I.documents(lifecycle_status, lifecycle_actioned_at DESC)', replace(schema_name, '-', '_'), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_documents_superseded_by ON %I.documents(superseded_by_document_id) WHERE superseded_by_document_id IS NOT NULL', replace(schema_name, '-', '_'), schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_document_lifecycle_workflow(tenant_schema);
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
    PERFORM add_tax_declaration_document_entities(schema_name);
    PERFORM add_document_lifecycle_workflow(schema_name);
    PERFORM add_cost_center_tables(schema_name);
    PERFORM add_migration_execution_run_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
