-- Migration 084 is forward-only. Existing v2 authorizations must remain
-- readable as non-promotable audit evidence, so a rollback does not narrow
-- the public constraints back to v1 and accidentally invalidate them.
SELECT 1;
