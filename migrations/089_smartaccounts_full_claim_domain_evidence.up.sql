-- Immutable, digest-only selected-domain proof ledger. One receipt can be
-- reused only for its exact batch/tenant/source/package/capture-scope tuple.

CREATE TABLE IF NOT EXISTS public.smartaccounts_full_claim_domain_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES public.smartaccounts_browser_onboarding_batches(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    source_company_id TEXT NOT NULL,
    package_id TEXT NOT NULL,
    scope_sha256 CHAR(64) NOT NULL,
    reconciliation_evidence_sha256 CHAR(64) NOT NULL,
    plan_version TEXT NOT NULL,
    domain_id TEXT NOT NULL,
    source TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    contract_version TEXT NOT NULL,
    live_source_validated BOOLEAN NOT NULL,
    schema_validated BOOLEAN NOT NULL,
    completeness_validated BOOLEAN NOT NULL,
    reconciliation_validated BOOLEAN NOT NULL,
    tombstones_resolved BOOLEAN NOT NULL,
    accountant_attested BOOLEAN NOT NULL,
    evidence_sha256 CHAR(64) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT smartaccounts_full_claim_domain_evidence_source_check CHECK (source_company_id ~ '^sa-browser-v1-[0-9]{1,20}$'),
    CONSTRAINT smartaccounts_full_claim_domain_evidence_scope_check CHECK (scope_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT smartaccounts_full_claim_domain_evidence_plan_check CHECK (plan_version = 'smartaccounts-full-claim-domain-plan-v1'),
    CONSTRAINT smartaccounts_full_claim_domain_evidence_route_check CHECK (
        length(domain_id) BETWEEN 1 AND 255 AND length(source) BETWEEN 1 AND 255 AND
        length(resource_id) BETWEEN 1 AND 255 AND length(contract_version) BETWEEN 1 AND 255
    ),
    CONSTRAINT smartaccounts_full_claim_domain_evidence_digest_check CHECK (
        reconciliation_evidence_sha256 ~ '^[0-9a-f]{64}$' AND evidence_sha256 ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT smartaccounts_full_claim_domain_evidence_all_gates_check CHECK (
        live_source_validated AND schema_validated AND completeness_validated AND
        reconciliation_validated AND tombstones_resolved AND accountant_attested
    ),
    UNIQUE (batch_id, tenant_id, source_company_id, package_id, scope_sha256, reconciliation_evidence_sha256, plan_version, domain_id)
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_full_claim_domain_evidence_binding
    ON public.smartaccounts_full_claim_domain_evidence
    (batch_id, tenant_id, source_company_id, package_id, scope_sha256, reconciliation_evidence_sha256, plan_version, domain_id);
