-- Rollback: Remove reminder rules tables

CREATE OR REPLACE FUNCTION remove_reminder_rules_from_schema(schema_name TEXT)
RETURNS void AS $$
BEGIN
    EXECUTE format('DROP TABLE IF EXISTS %I.payment_reminders CASCADE', schema_name);
    EXECUTE format('DROP TABLE IF EXISTS %I.reminder_rules CASCADE', schema_name);
    EXECUTE format('DELETE FROM %I.email_templates WHERE template_type IN (''PAYMENT_DUE_SOON'', ''PAYMENT_DUE_TODAY'')', schema_name);
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    tenant_record RECORD;
BEGIN
    FOR tenant_record IN
        SELECT schema_name FROM tenants WHERE schema_name IS NOT NULL
    LOOP
        PERFORM remove_reminder_rules_from_schema(tenant_record.schema_name);
    END LOOP;
END;
$$;

DROP FUNCTION IF EXISTS add_reminder_rules_to_schema(TEXT);
DROP FUNCTION IF EXISTS remove_reminder_rules_from_schema(TEXT);
