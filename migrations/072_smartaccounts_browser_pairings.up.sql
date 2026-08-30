-- Migration 072: short-lived one-time pairing metadata for the Brave
-- SmartAccounts browser relay. It stores only a SHA-256 pairing-token hash;
-- browser cookies, session tokens, API keys, source rows, and package data are
-- never accepted here.

CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_pairings (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    token_sha256 CHAR(64) NOT NULL,
    source_company_id TEXT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    claimed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT smartaccounts_browser_pairings_token_check CHECK (token_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT smartaccounts_browser_pairings_status_check CHECK (status IN ('ISSUED', 'CLAIMED')),
    CONSTRAINT smartaccounts_browser_pairings_claim_check CHECK (
        (status = 'ISSUED' AND claimed_at IS NULL AND source_company_id IS NULL) OR
        (status = 'CLAIMED' AND claimed_at IS NOT NULL AND source_company_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_smartaccounts_browser_pairings_tenant_expiry
    ON public.smartaccounts_browser_pairings (tenant_id, expires_at DESC);

ALTER TABLE public.smartaccounts_sync_controls
    DROP CONSTRAINT IF EXISTS smartaccounts_sync_controls_reference_check;

ALTER TABLE public.smartaccounts_sync_controls
    ADD CONSTRAINT smartaccounts_sync_controls_reference_check CHECK (
        secret_reference ~ '^(secret-ref|vault|op|sops)://[^[:space:]]+$' OR
        secret_reference ~ '^brave-session://[0-9a-f-]{36}$'
    );
