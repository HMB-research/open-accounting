-- Migration: Fix recurring invoices schema
-- The original migration 003 was missing columns that the code expects.
-- This adds the missing invoice_type and reference columns, and renames invoices_generated to generated_count.

-- Create function to fix recurring_invoices schema
CREATE OR REPLACE FUNCTION fix_recurring_invoices_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Add invoice_type column if it doesn't exist
    EXECUTE format('
        ALTER TABLE %I.recurring_invoices
        ADD COLUMN IF NOT EXISTS invoice_type VARCHAR(20) NOT NULL DEFAULT ''SALES''
    ', schema_name);

    -- Add reference column if it doesn't exist
    EXECUTE format('
        ALTER TABLE %I.recurring_invoices
        ADD COLUMN IF NOT EXISTS reference TEXT
    ', schema_name);

    -- Rename invoices_generated to generated_count if the old column exists
    -- First check if old column exists and new doesn't
    EXECUTE format('
        DO $inner$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''invoices_generated''
            ) AND NOT EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''generated_count''
            ) THEN
                ALTER TABLE %I.recurring_invoices RENAME COLUMN invoices_generated TO generated_count;
            ELSIF NOT EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = %L AND table_name = ''recurring_invoices'' AND column_name = ''generated_count''
            ) THEN
                ALTER TABLE %I.recurring_invoices ADD COLUMN generated_count INTEGER NOT NULL DEFAULT 0;
            END IF;
        END
        $inner$
    ', schema_name, schema_name, schema_name, schema_name, schema_name);
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
        PERFORM fix_recurring_invoices_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Note: Keeping the function as it's called by demo seed SQL
-- DROP FUNCTION IF EXISTS fix_recurring_invoices_schema(TEXT);
