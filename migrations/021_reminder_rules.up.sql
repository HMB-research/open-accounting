-- Migration: Add reminder rules and extend reminder tracking
-- This migration adds configurable reminder schedules per tenant

-- Create function to add reminder rules tables to a tenant schema
CREATE OR REPLACE FUNCTION add_reminder_rules_to_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    -- Drop and recreate check constraint with new types
    EXECUTE format('
        ALTER TABLE %I.email_templates DROP CONSTRAINT IF EXISTS email_templates_template_type_check
    ', schema_name);

    EXECUTE format('
        ALTER TABLE %I.email_templates ADD CONSTRAINT email_templates_template_type_check 
        CHECK (template_type IN (
            ''INVOICE_SEND'',
            ''INVOICE_REMINDER'',
            ''PAYMENT_RECEIPT'',
            ''OVERDUE_REMINDER'',
            ''WELCOME'',
            ''CUSTOM'',
            ''PAYMENT_DUE_SOON'',
            ''PAYMENT_DUE_TODAY''
        ))
    ', schema_name);

    -- Ensure email_templates has 'name' column (may be missing if created by create_email_tables_only)
    EXECUTE format('
        ALTER TABLE %I.email_templates ADD COLUMN IF NOT EXISTS name VARCHAR(100)
    ', schema_name);
    -- Reminder rules table - defines when to send reminders
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.reminder_rules (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(100) NOT NULL,
            trigger_type VARCHAR(20) NOT NULL CHECK (trigger_type IN (
                ''BEFORE_DUE'',
                ''ON_DUE'',
                ''AFTER_DUE''
            )),
            days_offset INTEGER NOT NULL DEFAULT 0,
            email_template_type VARCHAR(50) NOT NULL DEFAULT ''OVERDUE_REMINDER'',
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE (tenant_id, trigger_type, days_offset)
        )', schema_name);

    -- Payment reminders table - tracks sent reminders
    EXECUTE format('
        CREATE TABLE IF NOT EXISTS %I.payment_reminders (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            invoice_id UUID NOT NULL,
            invoice_number VARCHAR(50) NOT NULL,
            contact_id UUID NOT NULL,
            contact_name VARCHAR(255) NOT NULL,
            contact_email VARCHAR(255),
            rule_id UUID REFERENCES %I.reminder_rules(id),
            trigger_type VARCHAR(20) NOT NULL,
            days_offset INTEGER NOT NULL DEFAULT 0,
            reminder_number INTEGER NOT NULL DEFAULT 1,
            status VARCHAR(20) DEFAULT ''PENDING'' CHECK (status IN (
                ''PENDING'',
                ''SENT'',
                ''FAILED'',
                ''SKIPPED'',
                ''CANCELED''
            )),
            sent_at TIMESTAMPTZ,
            error_message TEXT,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        )', schema_name, schema_name);

    -- Create indexes
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_reminder_rules_tenant ON %I.reminder_rules(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_reminder_rules_active ON %I.reminder_rules(tenant_id, is_active) WHERE is_active = true',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_payment_reminders_tenant ON %I.payment_reminders(tenant_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_payment_reminders_invoice ON %I.payment_reminders(invoice_id)',
        replace(schema_name, 'tenant_', ''), schema_name);
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_payment_reminders_status ON %I.payment_reminders(status) WHERE status = ''PENDING''',
        replace(schema_name, 'tenant_', ''), schema_name);

    -- Insert default reminder rules
    EXECUTE format('
        INSERT INTO %I.reminder_rules (tenant_id, name, trigger_type, days_offset, email_template_type)
        VALUES
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''7 Days Before Due'', ''BEFORE_DUE'', 7, ''PAYMENT_DUE_SOON''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''On Due Date'', ''ON_DUE'', 0, ''PAYMENT_DUE_TODAY''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''7 Days Overdue'', ''AFTER_DUE'', 7, ''OVERDUE_REMINDER''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''14 Days Overdue'', ''AFTER_DUE'', 14, ''OVERDUE_REMINDER''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''30 Days Overdue'', ''AFTER_DUE'', 30, ''OVERDUE_REMINDER'')
        ON CONFLICT (tenant_id, trigger_type, days_offset) DO NOTHING
    ', schema_name, schema_name, schema_name, schema_name, schema_name, schema_name);

    -- Add new email template types
    EXECUTE format('
        INSERT INTO %I.email_templates (tenant_id, template_type, name, subject, body_html, body_text)
        VALUES
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''PAYMENT_DUE_SOON'',
             ''Payment Due Soon'',
             ''Reminder: Invoice {{invoice_number}} due in {{days_until_due}} days'',
             ''<p>Dear {{contact_name}},</p><p>This is a friendly reminder that invoice {{invoice_number}} for {{total_amount}} is due on {{due_date}} ({{days_until_due}} days from now).</p><p>Please arrange payment before the due date to avoid any late fees.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nThis is a friendly reminder that invoice {{invoice_number}} for {{total_amount}} is due on {{due_date}} ({{days_until_due}} days from now).\n\nPlease arrange payment before the due date to avoid any late fees.\n\nBest regards,\n{{company_name}}''),
            ((SELECT id FROM tenants WHERE schema_name = %L LIMIT 1),
             ''PAYMENT_DUE_TODAY'',
             ''Payment Due Today'',
             ''Invoice {{invoice_number}} is due today'',
             ''<p>Dear {{contact_name}},</p><p>This is a reminder that invoice {{invoice_number}} for {{total_amount}} is due today ({{due_date}}).</p><p>Please arrange payment as soon as possible.</p><p>Best regards,<br>{{company_name}}</p>'',
             ''Dear {{contact_name}},\n\nThis is a reminder that invoice {{invoice_number}} for {{total_amount}} is due today ({{due_date}}).\n\nPlease arrange payment as soon as possible.\n\nBest regards,\n{{company_name}}'')
        ON CONFLICT (tenant_id, template_type) DO NOTHING
    ', schema_name, schema_name, schema_name);
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
        PERFORM add_reminder_rules_to_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;
