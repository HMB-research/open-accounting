-- Migration 068: safe bridge capture-run pointer for one explicit
-- SmartAccounts source-to-target tenant binding. This stores no source rows,
-- credential, bridge cursor, query, evidence path, or financial data.

ALTER TABLE public.smartaccounts_sync_controls
    ADD COLUMN IF NOT EXISTS capture_run_id TEXT NULL;

ALTER TABLE public.smartaccounts_sync_controls
    DROP CONSTRAINT IF EXISTS smartaccounts_sync_controls_capture_run_id_check;

ALTER TABLE public.smartaccounts_sync_controls
    ADD CONSTRAINT smartaccounts_sync_controls_capture_run_id_check
    CHECK (capture_run_id IS NULL OR capture_run_id ~ '^[A-Za-z0-9._-]{1,128}$');
