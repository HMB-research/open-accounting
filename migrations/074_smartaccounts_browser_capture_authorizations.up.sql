-- Browser-capture relay tokens are only stored as SHA-256 digests. The
-- immutable tenant/source/run scope prevents a browser-origin request from
-- crossing a selected SmartAccounts company or target tenant boundary.
CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_capture_authorizations (
    run_id UUID NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    manifest_version TEXT NOT NULL,
    scope JSONB NOT NULL,
    token_sha256 CHAR(64) NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, tenant_id),
    CONSTRAINT smartaccounts_browser_capture_source_format CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_browser_capture_manifest_version CHECK (manifest_version = 'smartaccounts-brave-ui-v1'),
    CONSTRAINT smartaccounts_browser_capture_token_hash_format CHECK (token_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT smartaccounts_browser_capture_expiry CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_capture_authorizations_tenant_expiry
    ON public.smartaccounts_browser_capture_authorizations (tenant_id, expires_at);
