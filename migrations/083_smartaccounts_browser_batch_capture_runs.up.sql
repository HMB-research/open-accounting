-- Migration 083: retain the non-secret bridge run identity after an 082
-- lease expires.  082 is already deployed, so this is deliberately
-- forward-only.  The value is an opaque UUID, not a relay capability.

ALTER TABLE public.smartaccounts_browser_batch_source_workflows
    ADD COLUMN IF NOT EXISTS capture_run_id UUID NULL;
