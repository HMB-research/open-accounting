-- Remove browser-paired control metadata before restoring the older reference
-- constraint. This migration stores no financial records or browser secrets.

DELETE FROM public.smartaccounts_sync_controls
WHERE secret_reference ~ '^brave-session://[0-9a-f-]{36}$';

ALTER TABLE public.smartaccounts_sync_controls
    DROP CONSTRAINT IF EXISTS smartaccounts_sync_controls_reference_check;

ALTER TABLE public.smartaccounts_sync_controls
    ADD CONSTRAINT smartaccounts_sync_controls_reference_check CHECK (
        secret_reference ~ '^(secret-ref|vault|op|sops)://[^[:space:]]+$'
    );

DROP INDEX IF EXISTS public.idx_smartaccounts_browser_pairings_tenant_expiry;
DROP TABLE IF EXISTS public.smartaccounts_browser_pairings;
