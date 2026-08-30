-- Restore the pre-073 helper surface when rolling back this compatibility
-- migration. Existing tenant schemas are unaffected.
DROP FUNCTION IF EXISTS add_reminder_rules(TEXT);
