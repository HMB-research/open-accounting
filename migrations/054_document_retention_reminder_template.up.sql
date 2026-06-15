-- Migration 054: allow document retention reminder email templates.

CREATE OR REPLACE FUNCTION sync_email_template_type_constraint(schema_name TEXT)
RETURNS void AS $$
BEGIN
    IF to_regclass(format('%I.email_templates', schema_name)) IS NULL THEN
        RETURN;
    END IF;

    EXECUTE format('
        ALTER TABLE %I.email_templates
        DROP CONSTRAINT IF EXISTS email_templates_template_type_check
    ', schema_name);

    EXECUTE format('
        ALTER TABLE %I.email_templates
        ADD CONSTRAINT email_templates_template_type_check
        CHECK (template_type IN (
            ''INVOICE_SEND'',
            ''INVOICE_REMINDER'',
            ''PAYMENT_RECEIPT'',
            ''OVERDUE_REMINDER'',
            ''WELCOME'',
            ''CUSTOM'',
            ''PAYMENT_DUE_SOON'',
            ''PAYMENT_DUE_TODAY'',
            ''QUOTE_SEND'',
            ''ORDER_CONFIRM'',
            ''DOCUMENT_RETENTION_REMINDER''
        )) NOT VALID
    ', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_schema TEXT;
BEGIN
    FOR tenant_schema IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM sync_email_template_type_constraint(tenant_schema);
    END LOOP;
END $$;
