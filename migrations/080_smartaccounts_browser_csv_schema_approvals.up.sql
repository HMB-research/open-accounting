-- Migration 080: owner-reviewed, aggregate-only CSV schema approval state.
-- Exact header evidence and source binding stay in the private bridge and the
-- existing discovery authorization respectively; OA never duplicates them.

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_csv_schema_approvals (
    tenant_id UUID NOT NULL,
    discovery_id UUID NOT NULL,
    resource_id TEXT NOT NULL,
    schema_id TEXT NOT NULL,
    review_version TEXT NOT NULL,
    confirmed BOOLEAN NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL,
    review_audit_id UUID NOT NULL,
    reviewed_by TEXT NOT NULL,
    status TEXT NOT NULL,
    approval_sha256 CHAR(64) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, discovery_id, resource_id),
    FOREIGN KEY (tenant_id, discovery_id)
        REFERENCES public.smartaccounts_browser_discovery_authorizations (tenant_id, discovery_id)
        ON DELETE CASCADE,
    CONSTRAINT smartaccounts_browser_csv_schema_resource_check CHECK (resource_id ~ '^[a-z0-9_]{1,80}$'),
    CONSTRAINT smartaccounts_browser_csv_schema_schema_check CHECK (schema_id ~ '^[a-z0-9_]{1,80}$'),
    CONSTRAINT smartaccounts_browser_csv_schema_review_check CHECK (
        review_version = 'smartaccounts-browser-csv-schema-review-v1' AND confirmed
    ),
    CONSTRAINT smartaccounts_browser_csv_schema_status_check CHECK (
        (status = 'PENDING_BRIDGE_REGISTRATION' AND approval_sha256 IS NULL) OR
        (status = 'REGISTERED' AND approval_sha256 ~ '^[0-9a-f]{64}$')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_smartaccounts_browser_csv_schema_review_audit
    ON public.smartaccounts_browser_csv_schema_approvals (tenant_id, review_audit_id);
