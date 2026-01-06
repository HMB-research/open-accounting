-- Migration: Add KMD declarations tables and fix email_log schema
-- This migration creates the KMD (VAT return) tables and fixes the email_log related_id column

-- Create function to add KMD tables to a tenant schema
CREATE OR REPLACE FUNCTION add_kmd_tables_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- KMD declarations table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.kmd_declarations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            year INTEGER NOT NULL,
            month INTEGER NOT NULL,
            status VARCHAR(20) DEFAULT ''DRAFT'' CHECK (status IN (''DRAFT'', ''SUBMITTED'', ''ACCEPTED'', ''REJECTED'')),
            total_output_vat DECIMAL(15,2) DEFAULT 0,
            total_input_vat DECIMAL(15,2) DEFAULT 0,
            submitted_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE (tenant_id, year, month)
        )', schema_name);

    -- KMD rows table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.kmd_rows (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            declaration_id UUID NOT NULL REFERENCES %I.kmd_declarations(id) ON DELETE CASCADE,
            row_code VARCHAR(10) NOT NULL,
            amount DECIMAL(15,2) DEFAULT 0,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_kmd_declarations_tenant ON %I.kmd_declarations(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_kmd_rows_declaration ON %I.kmd_rows(declaration_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
END;
$$ LANGUAGE plpgsql;

-- Create function to fix email_log schema
CREATE OR REPLACE FUNCTION fix_email_log_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Add related_id column if it doesn't exist
    EXECUTE format('
        ALTER TABLE %I.email_log
        ADD COLUMN IF NOT EXISTS related_id UUID
    ', schema_name);

    -- Create index on related_id
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_email_log_related ON %I.email_log(related_id) WHERE related_id IS NOT NULL',
        replace(schema_name, 'tenant_', ''), schema_name);
END;
$$ LANGUAGE plpgsql;

-- Apply KMD tables to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM add_kmd_tables_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Apply email_log fix to all existing tenant schemas
DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM fix_email_log_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Update create_tenant_schema to include KMD tables for new tenants
CREATE OR REPLACE FUNCTION create_tenant_schema(schema_name TEXT) RETURNS VOID AS $$
BEGIN
    -- Create the schema
    EXECUTE format('CREATE SCHEMA IF NOT EXISTS %I', schema_name);

    -- Create core accounting tables (existing)
    PERFORM create_accounting_tables(schema_name);

    -- Create payroll tables
    PERFORM add_payroll_tables(schema_name);

    -- Create email tables (must be before fix_email_log_schema)
    PERFORM add_email_tables_to_schema(schema_name);

    -- Create KMD (VAT return) tables
    PERFORM add_kmd_tables_to_schema(schema_name);

    -- Fix email_log schema (add related_id column)
    PERFORM fix_email_log_schema(schema_name);
END;
$$ LANGUAGE plpgsql;
