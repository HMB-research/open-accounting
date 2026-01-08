-- Migration: Add VAT tracking to journal entry lines
-- This enables VAT calculations directly from journal entries for tax reporting

-- Create function to add VAT fields to journal_entry_lines table
CREATE OR REPLACE FUNCTION add_vat_columns_to_journal_lines(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Add VAT tracking fields to journal_entry_lines
    EXECUTE format('
        ALTER TABLE %I.journal_entry_lines
        ADD COLUMN IF NOT EXISTS vat_rate NUMERIC(5,2) DEFAULT 0,
        ADD COLUMN IF NOT EXISTS is_vat_inclusive BOOLEAN DEFAULT false
    ', schema_name);

    -- Add index for VAT queries
    EXECUTE format('
        CREATE INDEX IF NOT EXISTS idx_journal_entry_lines_vat_rate
        ON %I.journal_entry_lines(vat_rate)
        WHERE vat_rate > 0
    ', schema_name);
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
        PERFORM add_vat_columns_to_journal_lines(tenant_record.schema_name);
    END LOOP;
END;
$$;
