-- Rollback: Remove bank reconciliation tables

-- Remove columns and tables from all tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        -- Remove columns from bank_transactions
        EXECUTE format('
            ALTER TABLE IF EXISTS %I.bank_transactions
            DROP COLUMN IF EXISTS reconciliation_id,
            DROP COLUMN IF EXISTS match_status,
            DROP COLUMN IF EXISTS matched_payment_id,
            DROP COLUMN IF EXISTS matched_at,
            DROP COLUMN IF EXISTS import_id
        ', tenant_record.schema_name);

        -- Drop tables
        EXECUTE format('DROP TABLE IF EXISTS %I.bank_statement_imports CASCADE', tenant_record.schema_name);
        EXECUTE format('DROP TABLE IF EXISTS %I.bank_reconciliations CASCADE', tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Drop the helper function
DROP FUNCTION IF EXISTS add_reconciliation_tables_to_schema(TEXT);
