-- Migration: Add bank reconciliation tables
-- This migration adds tables for bank reconciliation and import tracking

-- Create function to add reconciliation tables to a tenant schema
CREATE OR REPLACE FUNCTION add_reconciliation_tables_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Bank reconciliations table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.bank_reconciliations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            bank_account_id UUID NOT NULL REFERENCES %I.bank_accounts(id),
            statement_date DATE NOT NULL,
            opening_balance NUMERIC(28,8) NOT NULL,
            closing_balance NUMERIC(28,8) NOT NULL,
            reconciled_balance NUMERIC(28,8),
            status VARCHAR(20) DEFAULT ''IN_PROGRESS'' CHECK (status IN (''IN_PROGRESS'', ''COMPLETED'', ''CANCELLED'')),
            notes TEXT,
            completed_at TIMESTAMPTZ,
            completed_by UUID,
            created_by UUID,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Bank statement imports table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.bank_statement_imports (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            bank_account_id UUID NOT NULL REFERENCES %I.bank_accounts(id),
            file_name VARCHAR(255) NOT NULL,
            file_format VARCHAR(50) DEFAULT ''CSV'',
            transactions_imported INTEGER DEFAULT 0,
            transactions_matched INTEGER DEFAULT 0,
            transactions_skipped INTEGER DEFAULT 0,
            import_errors TEXT,
            status VARCHAR(20) DEFAULT ''COMPLETED'' CHECK (status IN (''PROCESSING'', ''COMPLETED'', ''FAILED'')),
            created_by UUID,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Add reconciliation columns to bank_transactions if not exists
    EXECUTE format('
        ALTER TABLE %I.bank_transactions
        ADD COLUMN IF NOT EXISTS reconciliation_id UUID REFERENCES %I.bank_reconciliations(id),
        ADD COLUMN IF NOT EXISTS match_status VARCHAR(20) DEFAULT ''UNMATCHED'' CHECK (match_status IN (''UNMATCHED'', ''MATCHED'', ''EXCLUDED'', ''MANUAL'')),
        ADD COLUMN IF NOT EXISTS matched_payment_id UUID REFERENCES %I.payments(id),
        ADD COLUMN IF NOT EXISTS matched_at TIMESTAMPTZ,
        ADD COLUMN IF NOT EXISTS import_id UUID REFERENCES %I.bank_statement_imports(id)
    ', schema_name, schema_name, schema_name, schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_recon_bank ON %I.bank_reconciliations(bank_account_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_recon_status ON %I.bank_reconciliations(status) WHERE status = ''IN_PROGRESS''',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_import_bank ON %I.bank_statement_imports(bank_account_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_bt_match ON %I.bank_transactions(match_status) WHERE match_status = ''UNMATCHED''',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_bt_recon ON %I.bank_transactions(reconciliation_id)',
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
        PERFORM add_reconciliation_tables_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;
