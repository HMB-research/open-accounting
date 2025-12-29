-- Migration: Add email system tables
-- This migration adds tables for email templates and logging

-- Create function to add email tables to a tenant schema
CREATE OR REPLACE FUNCTION add_email_tables_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Email templates table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.email_templates (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            template_type VARCHAR(50) NOT NULL CHECK (template_type IN (
                ''INVOICE_SEND'',
                ''INVOICE_REMINDER'',
                ''PAYMENT_RECEIPT'',
                ''OVERDUE_REMINDER'',
                ''WELCOME'',
                ''CUSTOM''
            )),
            name VARCHAR(100) NOT NULL,
            subject TEXT NOT NULL,
            body_html TEXT NOT NULL,
            body_text TEXT,
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE (tenant_id, template_type)
        )', schema_name);

    -- Email log table
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.email_log (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            template_id UUID REFERENCES %I.email_templates(id),
            email_type VARCHAR(50) NOT NULL,
            recipient_email VARCHAR(255) NOT NULL,
            recipient_name VARCHAR(255),
            subject TEXT NOT NULL,
            body_html TEXT,
            status VARCHAR(20) DEFAULT ''PENDING'' CHECK (status IN (''PENDING'', ''SENT'', ''FAILED'', ''BOUNCED'')),
            related_entity_type VARCHAR(50),
            related_entity_id UUID,
            sent_at TIMESTAMPTZ,
            error_message TEXT,
            retry_count INTEGER DEFAULT 0,
            created_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_email_tpl_tenant ON %I.email_templates(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_email_log_tenant ON %I.email_log(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_email_log_status ON %I.email_log(status) WHERE status = ''PENDING''',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_email_log_entity ON %I.email_log(related_entity_type, related_entity_id)',
        replace(schema_name, 'tenant_', ''), schema_name);

    -- Insert default email templates
    EXECUTE format('
        INSERT INTO %I.email_templates (tenant_id, template_type, name, subject, body_html, body_text)
        VALUES
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''INVOICE_SEND'',
             ''Invoice'',
             ''Invoice {{invoice_number}} from {{company_name}}'',
             ''<p>Dear {{contact_name}},</p><p>Please find attached invoice {{invoice_number}} for {{total_amount}}.</p><p>Payment is due by {{due_date}}.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nPlease find attached invoice {{invoice_number}} for {{total_amount}}.\n\nPayment is due by {{due_date}}.\n\nBest regards,\n{{company_name}}''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''PAYMENT_RECEIPT'',
             ''Payment Receipt'',
             ''Payment Receipt from {{company_name}}'',
             ''<p>Dear {{contact_name}},</p><p>Thank you for your payment of {{amount}} received on {{payment_date}}.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nThank you for your payment of {{amount}} received on {{payment_date}}.\n\nBest regards,\n{{company_name}}''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''OVERDUE_REMINDER'',
             ''Overdue Invoice Reminder'',
             ''Reminder: Invoice {{invoice_number}} is overdue'',
             ''<p>Dear {{contact_name}},</p><p>This is a reminder that invoice {{invoice_number}} for {{total_amount}} was due on {{due_date}}.</p><p>Please arrange payment at your earliest convenience.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nThis is a reminder that invoice {{invoice_number}} for {{total_amount}} was due on {{due_date}}.\n\nPlease arrange payment at your earliest convenience.\n\nBest regards,\n{{company_name}}'')
        ON CONFLICT (tenant_id, template_type) DO NOTHING
    ', schema_name, schema_name, schema_name, schema_name);
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
        PERFORM add_email_tables_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;
