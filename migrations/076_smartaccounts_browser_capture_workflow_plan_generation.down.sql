ALTER TABLE public.smartaccounts_browser_capture_workflows
    DROP CONSTRAINT IF EXISTS smartaccounts_browser_capture_workflow_unique_policy;

ALTER TABLE public.smartaccounts_browser_capture_workflows
    ADD CONSTRAINT smartaccounts_browser_capture_workflow_unique_policy
    UNIQUE (tenant_id, source_company_id, from_inclusive);
