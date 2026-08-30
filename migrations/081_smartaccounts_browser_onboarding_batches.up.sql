-- Migration 081: server-issued relay catalog receipts and immutable,
-- owner-confirmed browser-company onboarding batches.
-- These records contain only opaque source selectors, target/pairing control
-- metadata and digests. They never carry source rows, browser/session data,
-- credentials, package content, or financial write instructions.

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_catalog_receipts (
    id UUID PRIMARY KEY,
    workflow_id UUID NOT NULL,
    owner_id TEXT NOT NULL,
    token_sha256 CHAR(64) NOT NULL,
    nonce_sha256 CHAR(64) NOT NULL,
    schema_version TEXT NOT NULL,
    intent_version TEXT NOT NULL,
    source_id_version TEXT NOT NULL,
    digest_algorithm TEXT NOT NULL,
    status TEXT NOT NULL,
    catalog_sha256 CHAR(64) NULL,
    catalog_count INTEGER NULL,
    companies JSONB NULL,
    observed_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    receipt_expires_at TIMESTAMPTZ NULL,
    accepted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_browser_onboarding_catalog_schema_check CHECK (
        schema_version = 'smartaccounts-browser-source-catalog-v1' AND
        intent_version = 'smartaccounts-browser-source-catalog-intent-v1' AND
        source_id_version = 'sa-browser-v1' AND digest_algorithm = 'sha256'
    ),
    CONSTRAINT smartaccounts_browser_onboarding_catalog_capability_digest_check CHECK (
        token_sha256 ~ '^[0-9a-f]{64}$' AND nonce_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_browser_onboarding_catalog_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT smartaccounts_browser_onboarding_catalog_state_check CHECK (
        (status = 'ISSUED' AND catalog_sha256 IS NULL AND catalog_count IS NULL AND companies IS NULL AND observed_at IS NULL AND receipt_expires_at IS NULL AND accepted_at IS NULL) OR
        (status = 'ACCEPTED' AND catalog_sha256 ~ '^[0-9a-f]{64}$' AND catalog_count BETWEEN 1 AND 250 AND jsonb_typeof(companies) = 'array' AND jsonb_array_length(companies) = catalog_count AND observed_at IS NOT NULL AND receipt_expires_at > observed_at AND accepted_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_smartaccounts_browser_onboarding_catalog_workflow
    ON public.smartaccounts_browser_onboarding_catalog_receipts (owner_id, workflow_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_smartaccounts_browser_onboarding_catalog_owner_id
    ON public.smartaccounts_browser_onboarding_catalog_receipts (owner_id, id);

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_batches (
    id UUID PRIMARY KEY,
    owner_id TEXT NOT NULL,
    -- An opaque fresh receipt ID is a server-issued relay capability. Its
    -- source catalog is resolved server-side; callers never submit it here.
    catalog_receipt_id UUID NOT NULL,
    relay_observed_at TIMESTAMPTZ NOT NULL,
    mode TEXT NOT NULL,
    selected_sources JSONB NOT NULL,
    observed_source_ids JSONB NOT NULL,
    observed_sources_sha256 CHAR(64) NOT NULL,
    manifest_sha256 CHAR(64) NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_browser_onboarding_batch_mode_check CHECK (mode IN ('selected', 'all')),
    CONSTRAINT smartaccounts_browser_onboarding_batch_selected_sources_check CHECK (
        jsonb_typeof(selected_sources) = 'array' AND jsonb_array_length(selected_sources) BETWEEN 1 AND 250
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_observed_sources_check CHECK (
        jsonb_typeof(observed_source_ids) = 'array' AND jsonb_array_length(observed_source_ids) BETWEEN 1 AND 250
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_observed_digest_check CHECK (
        observed_sources_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_manifest_digest_check CHECK (
        manifest_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_status_check CHECK (
        status IN ('PENDING', 'REVIEW_REQUIRED', 'READY', 'COMPLETE')
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_catalog_owner_fk FOREIGN KEY (owner_id, catalog_receipt_id)
        REFERENCES public.smartaccounts_browser_onboarding_catalog_receipts (owner_id, id) ON DELETE RESTRICT
);

-- A relay receipt can produce one immutable selected/all manifest. Exact
-- retries reuse it; a changed selection/name/digest conflicts. A fresh later
-- receipt may safely form a new batch against the same observed source IDs.
CREATE UNIQUE INDEX IF NOT EXISTS uq_smartaccounts_browser_onboarding_batch_owner_receipt
    ON public.smartaccounts_browser_onboarding_batches (owner_id, catalog_receipt_id);

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_onboarding_batch_outcomes (
    batch_id UUID NOT NULL REFERENCES public.smartaccounts_browser_onboarding_batches(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    source_company_name TEXT NOT NULL,
    tenant_id UUID NULL REFERENCES public.tenants(id) ON DELETE RESTRICT,
    tenant_name TEXT NOT NULL DEFAULT '',
    pairing_id UUID NULL REFERENCES public.smartaccounts_browser_pairings(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    tenant_created BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_reused BOOLEAN NOT NULL DEFAULT FALSE,
    reason_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (batch_id, source_company_id),
    CONSTRAINT smartaccounts_browser_onboarding_batch_outcome_source_check CHECK (
        source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_outcome_name_check CHECK (
        length(btrim(source_company_name)) BETWEEN 1 AND 120
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_outcome_status_check CHECK (
        status IN ('TARGET_READY', 'PAIRING_ISSUED', 'PAIRED', 'REVIEW_REQUIRED', 'FAILED')
    ),
    CONSTRAINT smartaccounts_browser_onboarding_batch_outcome_target_check CHECK (
        (tenant_id IS NULL AND tenant_name = '') OR
        (tenant_id IS NOT NULL AND length(btrim(tenant_name)) BETWEEN 1 AND 120)
    )
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_onboarding_batch_outcomes_source
    ON public.smartaccounts_browser_onboarding_batch_outcomes (source_company_id);
