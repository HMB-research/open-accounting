-- Migration: Remove email configuration from recurring invoices

-- Create function to remove email fields from recurring_invoices table
CREATE OR REPLACE FUNCTION remove_recurring_email_fields_from_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Remove email configuration fields from recurring_invoices
    EXECUTE format('
        ALTER TABLE %I.recurring_invoices
        DROP COLUMN IF EXISTS send_email_on_generation,
        DROP COLUMN IF EXISTS email_template_type,
        DROP COLUMN IF EXISTS recipient_email_override,
        DROP COLUMN IF EXISTS attach_pdf_to_email,
        DROP COLUMN IF EXISTS email_subject_override,
        DROP COLUMN IF EXISTS email_message
    ', schema_name);

    -- Remove email tracking fields from invoices
    EXECUTE format('
        ALTER TABLE %I.invoices
        DROP COLUMN IF EXISTS last_email_sent_at,
        DROP COLUMN IF EXISTS last_email_status,
        DROP COLUMN IF EXISTS last_email_log_id
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
        PERFORM remove_recurring_email_fields_from_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

-- Drop the function
DROP FUNCTION IF EXISTS add_recurring_email_fields_to_schema(TEXT);
