-- Migration 067: tenant-scoped control metadata for the future SmartAccounts
-- bridge. This stores an opaque secret-manager reference only; it must never
-- receive a raw SmartAccounts API key or source accounting data.
-- `source_company_id` is a bridge-verified stable provider identifier, not a
-- display name. The composite key allows selected source-to-target bindings
-- and deliberately provides no all-source or all-tenant default.

CREATE TABLE IF NOT EXISTS public.smartaccounts_sync_controls (
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id VARCHAR(255) NOT NULL,
    source_company_name TEXT NOT NULL,
    secret_reference TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    dry_run_requested_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, source_company_id),
    CONSTRAINT smartaccounts_sync_controls_reference_check CHECK (secret_reference ~ '^(secret-ref|vault|op|sops)://[^[:space:]]+$')
);
