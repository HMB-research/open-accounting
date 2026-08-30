-- One resumable, explicitly-selected history policy per OA tenant/source/from
-- date. To/cutoff and the only v1 resource are server-derived and never come
-- from the browser relay or source page.
CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_capture_workflows (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    from_inclusive DATE NOT NULL,
    to_inclusive DATE NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    capture_run_id UUID,
    status TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_browser_capture_workflow_source_format CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_browser_capture_workflow_bounds CHECK (from_inclusive <= to_inclusive),
    CONSTRAINT smartaccounts_browser_capture_workflow_status CHECK (status IN ('READY_FOR_CONSENT', 'CAPTURE_ISSUED')),
    CONSTRAINT smartaccounts_browser_capture_workflow_unique_policy UNIQUE (tenant_id, source_company_id, from_inclusive, to_inclusive)
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_capture_workflows_tenant_updated
    ON public.smartaccounts_browser_capture_workflows (tenant_id, updated_at DESC);
