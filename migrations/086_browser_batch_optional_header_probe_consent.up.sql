-- Migration 086: retain the optional, owner-selected bounded header-probe
-- consent with the immutable 082 preparation. Existing workflows predate the
-- optional path and therefore safely default to no Range/header probing.

ALTER TABLE public.smartaccounts_browser_batch_workflows
    ADD COLUMN IF NOT EXISTS header_probe_consent_confirmed BOOLEAN NOT NULL DEFAULT FALSE;
