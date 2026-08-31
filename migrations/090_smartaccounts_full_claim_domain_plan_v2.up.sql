-- Preserve v1 receipts for audit while allowing the reviewed v2 plan to bind
-- fresh evidence. v2 adds explicit mandatory source paths for business domains
-- absent from or materially incomplete in the documented API.

ALTER TABLE public.smartaccounts_full_claim_domain_evidence
    DROP CONSTRAINT IF EXISTS smartaccounts_full_claim_domain_evidence_plan_check;

ALTER TABLE public.smartaccounts_full_claim_domain_evidence
    ADD CONSTRAINT smartaccounts_full_claim_domain_evidence_plan_check
    CHECK (plan_version IN (
        'smartaccounts-full-claim-domain-plan-v1',
        'smartaccounts-full-claim-domain-plan-v2'
    ));
