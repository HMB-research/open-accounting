-- Rollback: Remove recurring invoices tables

-- Remove tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        EXECUTE format('ALTER TABLE IF EXISTS %I.invoices DROP COLUMN IF EXISTS recurring_invoice_id', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.recurring_invoice_lines CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.recurring_invoices CASCADE', tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Drop the helper function
DROP FUNCTION IF EXISTS add_recurring_tables_to_schema(TEXT);
