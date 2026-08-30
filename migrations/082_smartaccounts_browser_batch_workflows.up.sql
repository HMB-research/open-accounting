-- Migration 082: resumable, serial, non-financial browser workflow state.
-- This layer extends immutable 081 selected/all batches without storing relay
-- capabilities, browser/session state, source rows, headers, CSV bodies, or
-- any instruction to apply accounting entries.

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_batch_workflows (
    batch_id UUID PRIMARY KEY REFERENCES public.smartaccounts_browser_onboarding_batches(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    history_from DATE NOT NULL,
    preparatory_manifest_sha256 CHAR(64) NOT NULL,
    preparatory_consented_at TIMESTAMPTZ NOT NULL,
    transfer_manifest_sha256 CHAR(64) NULL,
    transfer_scope JSONB NULL,
    transfer_confirmed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_browser_batch_workflow_schema_check CHECK (
        schema_version = 'smartaccounts-browser-batch-workflow-v1'
    ),
    CONSTRAINT smartaccounts_browser_batch_workflow_preparatory_digest_check CHECK (
        preparatory_manifest_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_browser_batch_workflow_transfer_state_check CHECK (
        (transfer_manifest_sha256 IS NULL AND transfer_scope IS NULL AND transfer_confirmed_at IS NULL) OR
        (transfer_manifest_sha256 ~ '^[0-9a-f]{64}$' AND jsonb_typeof(transfer_scope) = 'object' AND transfer_confirmed_at IS NOT NULL)
    ),
    -- Ownership is verified by every service/repository query. The immutable
    -- 081 batch primary key is the database relation; 081 intentionally does
    -- not expose a redundant composite owner key.
    CONSTRAINT smartaccounts_browser_batch_workflow_batch_fk FOREIGN KEY (batch_id)
        REFERENCES public.smartaccounts_browser_onboarding_batches (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_batch_source_workflows (
    batch_id UUID NOT NULL REFERENCES public.smartaccounts_browser_batch_workflows(batch_id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE RESTRICT,
    ordinal INTEGER NOT NULL,
    phase TEXT NOT NULL,
    phase_generation BIGINT NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    lease_id UUID NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    discovery_id UUID NULL,
    discovery_contract_sha256 CHAR(64) NULL,
    discovery_receipt_sha256 CHAR(64) NULL,
    schema_id TEXT NULL,
    schema_approval_sha256 CHAR(64) NULL,
    package_id TEXT NULL,
    package_sha256 CHAR(64) NULL,
    preview_id UUID NULL,
    preview_sha256 CHAR(64) NULL,
    reason_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (batch_id, source_company_id),
    CONSTRAINT smartaccounts_browser_batch_source_source_check CHECK (
        source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'
    ),
    CONSTRAINT smartaccounts_browser_batch_source_ordinal_check CHECK (ordinal BETWEEN 0 AND 249),
    CONSTRAINT smartaccounts_browser_batch_source_generation_check CHECK (phase_generation >= 0 AND attempt_count >= 0),
    CONSTRAINT smartaccounts_browser_batch_source_phase_check CHECK (phase IN (
        'PAIRED', 'DISCOVERY_REQUIRED', 'DISCOVERY_RUNNING', 'DISCOVERY_COMPLETE',
        'SCHEMA_REVIEW_REQUIRED', 'SCHEMA_APPROVED', 'TRANSFER_CONFIRMATION_REQUIRED',
        'CAPTURE_RUNNING', 'STAGED', 'PREVIEW_READY', 'REVIEW_REQUIRED', 'FAILED_RETRYABLE', 'BLOCKED'
    )),
    CONSTRAINT smartaccounts_browser_batch_source_lease_check CHECK (
        (lease_id IS NULL AND lease_expires_at IS NULL) OR
        (lease_id IS NOT NULL AND lease_expires_at IS NOT NULL)
    ),
    CONSTRAINT smartaccounts_browser_batch_source_discovery_digest_check CHECK (
        (discovery_contract_sha256 IS NULL AND discovery_receipt_sha256 IS NULL) OR
        (discovery_contract_sha256 ~ '^[0-9a-f]{64}$' AND discovery_receipt_sha256 ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT smartaccounts_browser_batch_source_schema_digest_check CHECK (
        (schema_id IS NULL AND schema_approval_sha256 IS NULL) OR
        (length(btrim(schema_id)) BETWEEN 1 AND 120 AND schema_approval_sha256 ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT smartaccounts_browser_batch_source_package_digest_check CHECK (
        (package_id IS NULL AND package_sha256 IS NULL) OR
        (length(btrim(package_id)) BETWEEN 1 AND 160 AND package_sha256 ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT smartaccounts_browser_batch_source_preview_digest_check CHECK (
        (preview_id IS NULL AND preview_sha256 IS NULL) OR
        (preview_id IS NOT NULL AND preview_sha256 ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT smartaccounts_browser_batch_source_phase_proof_check CHECK (
        (phase IN ('DISCOVERY_RUNNING', 'CAPTURE_RUNNING')) = (lease_id IS NOT NULL) AND
        (phase NOT IN ('DISCOVERY_COMPLETE', 'SCHEMA_REVIEW_REQUIRED', 'SCHEMA_APPROVED', 'TRANSFER_CONFIRMATION_REQUIRED', 'CAPTURE_RUNNING', 'STAGED', 'PREVIEW_READY') OR discovery_id IS NOT NULL) AND
        (phase NOT IN ('SCHEMA_APPROVED', 'TRANSFER_CONFIRMATION_REQUIRED', 'CAPTURE_RUNNING', 'STAGED', 'PREVIEW_READY') OR schema_id IS NOT NULL) AND
        (phase NOT IN ('STAGED', 'PREVIEW_READY') OR package_id IS NOT NULL) AND
        (phase <> 'PREVIEW_READY' OR preview_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_smartaccounts_browser_batch_source_workflow_ordinal
    ON public.smartaccounts_browser_batch_source_workflows (batch_id, ordinal);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_batch_source_workflow_phase
    ON public.smartaccounts_browser_batch_source_workflows (batch_id, phase, ordinal);
