-- Browser-only commercial-detail control state.  This is intentionally a
-- review/archive-only capability ledger: no source rows, names, amounts,
-- contracts, cookies, or raw bearer tokens are retained in OA.
CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_commercial_detail_authorizations (
    run_id UUID NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    batch_id UUID NOT NULL,
    workflow_id UUID NOT NULL,
    source_company_id TEXT NOT NULL,
    manifest_version TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    schema_id TEXT NOT NULL,
    source_schema TEXT NOT NULL,
    review_audit_id UUID NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL,
    contract_sha256 CHAR(64) NOT NULL,
    route_sha256 CHAR(64) NOT NULL,
    consent_sha256 CHAR(64) NOT NULL,
    from_inclusive DATE NOT NULL,
    to_inclusive DATE NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    token_sha256 CHAR(64) NOT NULL,
    status TEXT NOT NULL,
    ndjson_sha256 CHAR(64),
    record_count INTEGER NOT NULL DEFAULT 0,
    review_required INTEGER NOT NULL DEFAULT 0,
    package_id TEXT,
    package_sha256 CHAR(64),
    bridge_started_at TIMESTAMPTZ,
    created_by TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, tenant_id),
    UNIQUE (tenant_id, workflow_id, resource_id),
    UNIQUE (tenant_id, batch_id, source_company_id, resource_id),
    CONSTRAINT smartaccounts_browser_commercial_source_format CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_browser_commercial_manifest_check CHECK (manifest_version = 'smartaccounts-browser-commercial-detail-v1'),
    CONSTRAINT smartaccounts_browser_commercial_resource_check CHECK (
        (resource_id = 'client_invoices' AND sequence = 1 AND schema_id = 'client_invoices_detail_v1' AND source_schema = 'smartaccounts-browser-commercial-detail-v1/client_invoices_detail_v1') OR
        (resource_id = 'bank_payments' AND sequence = 2 AND schema_id = 'bank_payments_detail_v1' AND source_schema = 'smartaccounts-browser-commercial-detail-v1/bank_payments_detail_v1')
    ),
    CONSTRAINT smartaccounts_browser_commercial_hash_check CHECK (
        contract_sha256 ~ '^[0-9a-f]{64}$' AND route_sha256 ~ '^[0-9a-f]{64}$' AND
        consent_sha256 ~ '^[0-9a-f]{64}$' AND token_sha256 ~ '^[0-9a-f]{64}$' AND
        (ndjson_sha256 IS NULL OR ndjson_sha256 ~ '^[0-9a-f]{64}$') AND
        (package_sha256 IS NULL OR package_sha256 ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT smartaccounts_browser_commercial_status_check CHECK (status IN ('list_selector_required', 'open', 'finalized_archived_evidence')),
    CONSTRAINT smartaccounts_browser_commercial_counts_check CHECK (record_count >= 0 AND review_required >= 0 AND review_required <= record_count),
    CONSTRAINT smartaccounts_browser_commercial_scope_check CHECK (from_inclusive <= to_inclusive AND cutoff_at > '2000-01-01'),
    CONSTRAINT smartaccounts_browser_commercial_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_commercial_tenant_expiry
    ON public.smartaccounts_browser_commercial_detail_authorizations (tenant_id, expires_at);
