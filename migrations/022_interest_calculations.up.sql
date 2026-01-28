-- Migration 022: Add late payment interest calculations
-- This migration adds support for tracking and calculating interest on overdue invoices

-- Function to add interest tracking to a tenant schema
CREATE OR REPLACE FUNCTION add_interest_tables(schema_name TEXT) RETURNS void AS $$
BEGIN
    -- Create invoice_interest table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.invoice_interest (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            invoice_id UUID NOT NULL REFERENCES %I.invoices(id) ON DELETE CASCADE,
            calculated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            days_overdue INTEGER NOT NULL,
            principal_amount DECIMAL(15,2) NOT NULL,
            interest_rate DECIMAL(8,6) NOT NULL,
            interest_amount DECIMAL(15,2) NOT NULL,
            total_with_interest DECIMAL(15,2) NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )', schema_name, schema_name);

    -- Create indexes
    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_invoice_interest_invoice_id
        ON %I.invoice_interest(invoice_id)
    ', schema_name);

    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_invoice_interest_calculated_at
        ON %I.invoice_interest(calculated_at DESC)
    ', schema_name);
END;
$$ LANGUAGE plpgsql;

-- Apply to all existing tenant schemas
DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE is_active = true
    LOOP
        PERFORM add_interest_tables(tenant_schema);
    END LOOP;
END $$;

-- Update create_tenant_schema to include interest tables for new tenants
-- Preserves existing function from migration 019 and adds interest tables
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Create the schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    -- Create core accounting tables (from migration 001/011)
    PERFORM create_accounting_tables(schema_name);

    -- Create payroll tables (from migration 007)
    PERFORM add_payroll_tables(schema_name);

    -- Create email tables (from migration 004/019)
    PERFORM create_email_tables_only(schema_name);

    -- Create KMD tables (from migration 019)
    PERFORM add_kmd_tables_to_schema(schema_name);

    -- Fix email_log schema (from migration 019)
    PERFORM fix_email_log_schema(schema_name);

    -- Create reminder rules tables (from migration 021)
    PERFORM add_reminder_rules_to_schema(schema_name);

    -- Add interest tracking tables (this migration)
    PERFORM add_interest_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
