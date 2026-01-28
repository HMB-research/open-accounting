-- Migration 022: Add late payment interest calculations
-- This migration adds support for tracking and calculating interest on overdue invoices

-- Create invoice_interest table to track calculated interest per invoice
-- Note: This is created per-tenant schema via the create_tenant_schema function
-- For existing tenants, we add it directly

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

-- Update the create_tenant_schema function to include invoice_interest table
-- This ensures new tenants get the table automatically
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS void AS $$
BEGIN
    -- Create schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    -- Create all base tables (existing logic preserved)
    PERFORM create_accounting_tables(schema_name);
    PERFORM create_business_tables(schema_name);
    PERFORM create_email_tables(schema_name);
    PERFORM create_recurring_invoice_tables(schema_name);
    PERFORM create_banking_tables(schema_name);
    PERFORM create_payroll_tables(schema_name);
    PERFORM create_quotes_orders_tables(schema_name);
    PERFORM create_fixed_assets_tables(schema_name);
    PERFORM create_inventory_tables(schema_name);
    PERFORM create_leave_management_tables(schema_name);
    PERFORM create_cost_center_tables(schema_name);
    PERFORM create_reminder_rules_tables(schema_name);

    -- Add interest tracking tables
    PERFORM add_interest_tables(schema_name);
END;
$$ LANGUAGE plpgsql;
