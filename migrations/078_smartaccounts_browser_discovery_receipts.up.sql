-- Migration 078: action-time browser discovery authorization and only the
-- bridge-safe aggregate receipt. The redacted per-resource contract remains
-- private bridge authority and is intentionally not duplicated in OA.

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_discovery_authorizations (
    discovery_id UUID NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    manifest_version TEXT NOT NULL,
    contract_version TEXT NOT NULL,
    resource_ids JSONB NOT NULL,
    metadata_only_consent_confirmed BOOLEAN NOT NULL,
    header_probe_consent_confirmed BOOLEAN NOT NULL,
    consented_at TIMESTAMPTZ NOT NULL,
    created_by TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    receipt_status TEXT NULL,
    contract_sha256 CHAR(64) NULL,
    resource_count INTEGER NULL,
    capture_ready_count INTEGER NULL,
    filter_contract_required_count INTEGER NULL,
    page_only_contract_required_count INTEGER NULL,
    private_endpoint_required_count INTEGER NULL,
    binding_blocked_count INTEGER NULL,
    receipt_recorded_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, discovery_id),
    CONSTRAINT smartaccounts_browser_discovery_source_check CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_browser_discovery_manifest_check CHECK (manifest_version = 'smartaccounts-brave-ui-v1'),
    CONSTRAINT smartaccounts_browser_discovery_contract_check CHECK (contract_version = 'smartaccounts-brave-discovery-contract-v1'),
    -- The server-owned v1 full observed manifest has exactly 31 UI surfaces.
    -- This is metadata-only; resource IDs remain fixed in OA code and never
    -- caller-selectable, so this exact count cannot expand capture.
    CONSTRAINT smartaccounts_browser_discovery_resource_ids_check CHECK (jsonb_typeof(resource_ids) = 'array' AND jsonb_array_length(resource_ids) = 31),
    CONSTRAINT smartaccounts_browser_discovery_consent_check CHECK (metadata_only_consent_confirmed),
    CONSTRAINT smartaccounts_browser_discovery_expiry_check CHECK (expires_at > consented_at),
    CONSTRAINT smartaccounts_browser_discovery_receipt_check CHECK (
        (receipt_status IS NULL AND contract_sha256 IS NULL AND resource_count IS NULL AND capture_ready_count IS NULL AND filter_contract_required_count IS NULL AND page_only_contract_required_count IS NULL AND private_endpoint_required_count IS NULL AND binding_blocked_count IS NULL AND receipt_recorded_at IS NULL) OR
        (receipt_status IN ('completed', 'awaiting_browser', 'company_binding_blocked', 'expired', 'discovery_failed') AND contract_sha256 ~ '^[0-9a-f]{64}$' AND resource_count >= 0 AND capture_ready_count >= 0 AND filter_contract_required_count >= 0 AND page_only_contract_required_count >= 0 AND private_endpoint_required_count >= 0 AND binding_blocked_count >= 0 AND resource_count = capture_ready_count + filter_contract_required_count + page_only_contract_required_count + private_endpoint_required_count + binding_blocked_count AND receipt_recorded_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_discovery_authorizations_expiry
    ON public.smartaccounts_browser_discovery_authorizations (tenant_id, expires_at DESC);
