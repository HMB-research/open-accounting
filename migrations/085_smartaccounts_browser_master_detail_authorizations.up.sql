-- Browser master-detail relay authorization. The raw relay token is never
-- persisted; every immutable source/resource/current-snapshot binding has one
-- short-lived digest only. This table cannot create accounting postings.
CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_master_detail_authorizations (
    run_id UUID NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    batch_id UUID NOT NULL,
    source_company_id TEXT NOT NULL,
    snapshot_date DATE NOT NULL,
    manifest_version TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    schema_id TEXT NOT NULL,
    source_schema TEXT NOT NULL,
    contract JSONB NOT NULL,
    contract_sha256 CHAR(64) NOT NULL,
    approval_sha256 CHAR(64) NOT NULL,
    scope JSONB NOT NULL,
    token_sha256 CHAR(64) NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, tenant_id),
    UNIQUE (tenant_id, batch_id, resource_id),
    CONSTRAINT smartaccounts_browser_master_detail_source_format CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_browser_master_detail_manifest_check CHECK (manifest_version = 'smartaccounts-browser-master-detail-v1'),
    CONSTRAINT smartaccounts_browser_master_detail_resource_check CHECK (resource_id IN ('clients', 'vendors', 'articles')),
    CONSTRAINT smartaccounts_browser_master_detail_schema_check CHECK (
        (resource_id = 'clients' AND schema_id = 'clients_detail_v1' AND source_schema = 'smartaccounts-browser-master-detail-v1/clients_detail_v1') OR
        (resource_id = 'vendors' AND schema_id = 'vendors_detail_v1' AND source_schema = 'smartaccounts-browser-master-detail-v1/vendors_detail_v1') OR
        (resource_id = 'articles' AND schema_id = 'articles_detail_v1' AND source_schema = 'smartaccounts-browser-master-detail-v1/articles_detail_v1')
    ),
    CONSTRAINT smartaccounts_browser_master_detail_contract_hash_check CHECK (contract_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT smartaccounts_browser_master_detail_approval_hash_check CHECK (approval_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT smartaccounts_browser_master_detail_token_hash_check CHECK (token_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT smartaccounts_browser_master_detail_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_master_detail_tenant_expiry
    ON public.smartaccounts_browser_master_detail_authorizations (tenant_id, expires_at);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_master_detail_snapshot
    ON public.smartaccounts_browser_master_detail_authorizations (tenant_id, source_company_id, snapshot_date, created_at DESC);
