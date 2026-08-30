-- Migration 071: retain safe metadata for every explicit SmartAccounts
-- capture. A full-history capture can need a later, date-windowed capture for
-- documented interval-only services; retaining both run summaries prevents a
-- second run from obscuring the first staged package. No source rows,
-- credentials, cursor, query, or evidence path are stored here.

CREATE TABLE IF NOT EXISTS public.smartaccounts_sync_capture_run_history (
    tenant_id UUID NOT NULL,
    source_company_id VARCHAR(255) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    progress JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, source_company_id, run_id),
    CONSTRAINT smartaccounts_sync_capture_run_history_control_fk
        FOREIGN KEY (tenant_id, source_company_id)
        REFERENCES public.smartaccounts_sync_controls(tenant_id, source_company_id)
        ON DELETE CASCADE,
    CONSTRAINT smartaccounts_sync_capture_run_history_run_id_check
        CHECK (run_id ~ '^[A-Za-z0-9._-]{1,128}$')
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_sync_capture_run_history_updated
    ON public.smartaccounts_sync_capture_run_history(tenant_id, source_company_id, updated_at DESC);
