-- Migration 077: durable, non-financial selected-company browser onboarding.
-- The table reserves one opaque Brave selector to a single OA tenant. It
-- contains no credential, cookie, relay token, CSV, package, or accounting
-- state. Pairing claims still verify the selected source before configuration.

ALTER TABLE public.smartaccounts_browser_pairings
    ADD COLUMN IF NOT EXISTS expected_source_company_id TEXT NULL;

ALTER TABLE public.smartaccounts_browser_pairings
    DROP CONSTRAINT IF EXISTS smartaccounts_browser_pairings_expected_source_check;

ALTER TABLE public.smartaccounts_browser_pairings
    ADD CONSTRAINT smartaccounts_browser_pairings_expected_source_check CHECK (
        expected_source_company_id IS NULL OR
        expected_source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'
    );

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_bindings (
    source_company_id TEXT PRIMARY KEY,
    source_company_name TEXT NOT NULL,
    tenant_id UUID NULL REFERENCES public.tenants(id) ON DELETE RESTRICT,
    tenant_name TEXT NOT NULL DEFAULT '',
    pairing_id UUID NULL REFERENCES public.smartaccounts_browser_pairings(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_browser_onboarding_source_check CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_browser_onboarding_name_check CHECK (length(btrim(source_company_name)) BETWEEN 1 AND 120),
    CONSTRAINT smartaccounts_browser_onboarding_status_check CHECK (status IN ('TARGET_READY', 'PAIRING_ISSUED', 'PAIRED', 'REVIEW_REQUIRED', 'FAILED')),
    CONSTRAINT smartaccounts_browser_onboarding_target_check CHECK (
        (tenant_id IS NULL AND tenant_name = '') OR
        (tenant_id IS NOT NULL AND length(btrim(tenant_name)) BETWEEN 1 AND 120)
    )
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_onboarding_tenant
    ON public.smartaccounts_browser_onboarding_bindings (tenant_id);

-- The selected-company flow is deliberately one source per OA tenant. This
-- also makes independently retried/browser concurrent onboarding requests
-- fail closed instead of collapsing two source companies into one target.
CREATE UNIQUE INDEX IF NOT EXISTS uq_smartaccounts_browser_onboarding_tenant
    ON public.smartaccounts_browser_onboarding_bindings (tenant_id)
    WHERE tenant_id IS NOT NULL;
