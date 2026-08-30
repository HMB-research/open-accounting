-- Migration 073: compatibility alias for tenant bootstraps emitted by the
-- already-applied 069/070 migrations. Those versions call
-- add_reminder_rules(schema_name), while the canonical helper introduced by
-- migration 021 is named add_reminder_rules_to_schema(schema_name).
--
-- Keep the migration history immutable and make new tenant creation safe on
-- both upgraded and fresh databases.
CREATE OR REPLACE FUNCTION add_reminder_rules(schema_name TEXT)
RETURNS VOID AS $$
BEGIN
    PERFORM add_reminder_rules_to_schema(schema_name);
END;
$$ LANGUAGE plpgsql;
