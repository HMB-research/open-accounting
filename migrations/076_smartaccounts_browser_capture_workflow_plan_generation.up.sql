-- A workflow's server-derived day bound is immutable. Reusing a historical
-- start on a later day must therefore create a new generation rather than
-- silently extending the old run. This also upgrades any database that
-- received the first 075 constraint before this correction.
ALTER TABLE public.smartaccounts_browser_capture_workflows
    DROP CONSTRAINT IF EXISTS smartaccounts_browser_capture_workflow_unique_policy;

ALTER TABLE public.smartaccounts_browser_capture_workflows
    ADD CONSTRAINT smartaccounts_browser_capture_workflow_unique_policy
    UNIQUE (tenant_id, source_company_id, from_inclusive, to_inclusive);
