-- Rollback: Remove VAT tracking from journal entry lines

-- Create function to remove VAT fields from journal_entry_lines table
CREATE OR REPLACE FUNCTION remove_vat_columns_from_journal_lines(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Drop index first
    EXECUTE format('
        DROP INDEX IF EXISTS %I.idx_journal_entry_lines_vat_rate
    ', schema_name);

    -- Remove VAT columns from journal_entry_lines
    EXECUTE format('
        ALTER TABLE %I.journal_entry_lines
        DROP COLUMN IF EXISTS vat_rate,
        DROP COLUMN IF EXISTS is_vat_inclusive
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
        PERFORM remove_vat_columns_from_journal_lines(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Drop the helper functions
DROP FUNCTION IF EXISTS add_vat_columns_to_journal_lines(TEXT);
DROP FUNCTION IF EXISTS remove_vat_columns_from_journal_lines(TEXT);
