-- Migration 069: bounded, tenant-isolated archive delivery for private bridge
-- packages. This is deliberately separate from the browser-adjacent
-- import-session receipt receiver: it supports manifest-first resumable
-- server-to-server chunks but cannot create accounting transactions.

CREATE TABLE IF NOT EXISTS public.import_delivery_nonces (
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    nonce VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, nonce)
);

CREATE INDEX IF NOT EXISTS idx_import_delivery_nonces_expiry
ON public.import_delivery_nonces(expires_at);

CREATE OR REPLACE FUNCTION add_external_import_delivery_tables(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.external_import_deliveries (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
            package_id VARCHAR(255) NOT NULL,
            provider VARCHAR(64) NOT NULL,
            source_company_id VARCHAR(255) NOT NULL,
            manifest_sha256 VARCHAR(64) NOT NULL,
            package_sha256 VARCHAR(64) NOT NULL,
            records_sha256 VARCHAR(64) NOT NULL,
            record_count INTEGER NOT NULL CHECK (record_count >= 0),
            artifact_count INTEGER NOT NULL CHECK (artifact_count >= 0),
            status VARCHAR(64) NOT NULL,
            manifest JSONB NOT NULL,
            staged_session_id TEXT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            CONSTRAINT external_import_deliveries_identity_unique UNIQUE (tenant_id, package_id),
            CONSTRAINT external_import_deliveries_manifest_sha256_check CHECK (manifest_sha256 ~ ''^[0-9a-f]{64}$''),
            CONSTRAINT external_import_deliveries_package_sha256_check CHECK (package_sha256 ~ ''^[0-9a-f]{64}$''),
            CONSTRAINT external_import_deliveries_records_sha256_check CHECK (records_sha256 ~ ''^[0-9a-f]{64}$'')
        )
    ', schema_name);

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.external_import_record_chunks (
            delivery_id UUID NOT NULL REFERENCES %I.external_import_deliveries(id) ON DELETE CASCADE,
            sequence INTEGER NOT NULL CHECK (sequence >= 0),
            record_count INTEGER NOT NULL CHECK (record_count > 0),
            sha256 VARCHAR(64) NOT NULL CHECK (sha256 ~ ''^[0-9a-f]{64}$''),
            data BYTEA NOT NULL CHECK (octet_length(data) > 0 AND octet_length(data) <= 1048576),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            PRIMARY KEY (delivery_id, sequence)
        )
    ', schema_name, schema_name);

    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.external_import_artifact_chunks (
            delivery_id UUID NOT NULL REFERENCES %I.external_import_deliveries(id) ON DELETE CASCADE,
            artifact_id VARCHAR(255) NOT NULL,
            sequence INTEGER NOT NULL CHECK (sequence >= 0),
            chunk_count INTEGER NOT NULL CHECK (chunk_count > 0),
            sha256 VARCHAR(64) NOT NULL CHECK (sha256 ~ ''^[0-9a-f]{64}$''),
            data BYTEA NOT NULL CHECK (octet_length(data) > 0 AND octet_length(data) <= 1048576),
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            PRIMARY KEY (delivery_id, artifact_id, sequence),
            -- PostgreSQL assigns this exact default name to the preceding
            -- column-level CHECK (sequence >= 0). Give the cross-column
            -- condition its own name so tenant-schema creation can succeed.
            CONSTRAINT external_import_artifact_chunks_sequence_lt_chunk_count_check CHECK (sequence < chunk_count)
        )
    ', schema_name, schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL LOOP
        PERFORM add_external_import_delivery_tables(tenant_schema);
    END LOOP;
END $$;

-- Keep newly created tenants structurally equivalent to upgraded tenants.
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);
    PERFORM create_accounting_tables(schema_name);
    PERFORM add_journal_entry_post_reason(schema_name);
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
    PERFORM add_reminder_rules(schema_name);
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
    PERFORM add_document_legal_hold_workflow(schema_name);
    PERFORM add_cost_center_tables(schema_name);
    PERFORM add_migration_execution_run_tables(schema_name);
    PERFORM add_financial_report_indexes(schema_name);
    PERFORM add_import_session_tables(schema_name);
    PERFORM add_import_session_ledger_verification(schema_name);
    PERFORM add_import_session_ledger_plan_input(schema_name);
    PERFORM add_external_import_delivery_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
