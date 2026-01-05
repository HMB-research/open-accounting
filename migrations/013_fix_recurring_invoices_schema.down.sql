-- Rollback: Fix recurring invoices schema
-- Note: This only removes the added columns, it does not revert the column rename
-- because the rename is dependent on which state the schema was in

-- Create function to rollback recurring_invoices schema changes
CREATE OR REPLACE FUNCTION rollback_recurring_invoices_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Remove invoice_type column
    EXECUTE format('
        ALTER TABLE %I.recurring_invoices
        DROP COLUMN IF EXISTS invoice_type
    ', schema_name);

    -- Remove reference column
    EXECUTE format('
        ALTER TABLE %I.recurring_invoices
        DROP COLUMN IF EXISTS reference
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
        PERFORM rollback_recurring_invoices_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Clean up
DROP FUNCTION IF EXISTS rollback_recurring_invoices_schema(TEXT);
