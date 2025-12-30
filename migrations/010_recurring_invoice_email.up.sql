-- Migration: Add email configuration to recurring invoices
-- This migration adds fields for automatic email sending when invoices are generated

-- Create function to add email fields to recurring_invoices table
CREATE OR REPLACE FUNCTION add_recurring_email_fields_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Add email configuration fields to recurring_invoices
    EXECUTE format('
        ALTER TABLE %I.recurring_invoices
        ADD COLUMN IF NOT EXISTS send_email_on_generation BOOLEAN DEFAULT false,
        ADD COLUMN IF NOT EXISTS email_template_type VARCHAR(50) DEFAULT ''INVOICE_SEND'',
        ADD COLUMN IF NOT EXISTS recipient_email_override VARCHAR(255),
        ADD COLUMN IF NOT EXISTS attach_pdf_to_email BOOLEAN DEFAULT true,
        ADD COLUMN IF NOT EXISTS email_subject_override TEXT,
        ADD COLUMN IF NOT EXISTS email_message TEXT
    ', schema_name);

    -- Add last_email_status to track email delivery for generated invoices
    EXECUTE format('
        ALTER TABLE %I.invoices
        ADD COLUMN IF NOT EXISTS last_email_sent_at TIMESTAMPTZ,
        ADD COLUMN IF NOT EXISTS last_email_status VARCHAR(20),
        ADD COLUMN IF NOT EXISTS last_email_log_id UUID
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
        PERFORM add_recurring_email_fields_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;
