-- Migration: Add recurring invoices tables
-- This migration adds tables for recurring invoice functionality

-- Create function to add recurring invoice tables to a tenant schema
CREATE OR REPLACE FUNCTION add_recurring_tables_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Recurring invoices template table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.recurring_invoices (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            contact_id UUID NOT NULL REFERENCES %I.contacts(id),
            frequency VARCHAR(20) NOT NULL CHECK (frequency IN (''WEEKLY'', ''BIWEEKLY'', ''MONTHLY'', ''QUARTERLY'', ''YEARLY'')),
            start_date DATE NOT NULL,
            end_date DATE,
            next_generation_date DATE NOT NULL,
            payment_terms_days INTEGER DEFAULT 14,
            currency VARCHAR(3) DEFAULT ''EUR'',
            notes TEXT,
            is_active BOOLEAN DEFAULT true,
            last_generated_at TIMESTAMPTZ,
            invoices_generated INTEGER DEFAULT 0,
            created_by UUID,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Recurring invoice line items
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.recurring_invoice_lines (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            recurring_invoice_id UUID NOT NULL REFERENCES %I.recurring_invoices(id) ON DELETE CASCADE,
            line_number INTEGER NOT NULL,
            description TEXT NOT NULL,
            quantity NUMERIC(18,6) DEFAULT 1,
            unit_price NUMERIC(28,8) NOT NULL,
            vat_rate NUMERIC(5,2) NOT NULL,
            account_id UUID REFERENCES %I.accounts(id),
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name, schema_name);

    -- Add recurring_invoice_id to invoices table
    EXECUTE format('
        ALTER TABLE %I.invoices
        ADD COLUMN IF NOT EXISTS recurring_invoice_id UUID REFERENCES %I.recurring_invoices(id)
    ', schema_name, schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_recurring_tenant ON %I.recurring_invoices(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_recurring_next_date ON %I.recurring_invoices(next_generation_date) WHERE is_active = true',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_recurring_lines ON %I.recurring_invoice_lines(recurring_invoice_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
END;
$$ LANGUAGE plpgsql;

-- Apply to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_recurring_tables_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Update create_tenant_schema to include recurring tables for new tenants
-- This is done by modifying the original function or creating a hook
