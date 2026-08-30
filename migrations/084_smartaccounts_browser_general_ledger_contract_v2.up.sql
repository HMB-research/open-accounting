-- Migration 084: permit the versioned, reviewed General Ledger browser
-- contract while retaining historical v1 browser authorizations as read-only
-- evidence. Migrations 074 and 078 are already deployed and must not be
-- rewritten. Runtime only issues v2 capabilities; retaining v1 here lets
-- prior receipts remain auditable without promoting their journal-summary
-- surface into a financial path.

ALTER TABLE public.smartaccounts_browser_capture_authorizations
    DROP CONSTRAINT IF EXISTS smartaccounts_browser_capture_manifest_version;

ALTER TABLE public.smartaccounts_browser_capture_authorizations
    ADD CONSTRAINT smartaccounts_browser_capture_manifest_version CHECK (
        manifest_version IN ('smartaccounts-brave-ui-v1', 'smartaccounts-brave-ui-v2')
    );

ALTER TABLE public.smartaccounts_browser_discovery_authorizations
    DROP CONSTRAINT IF EXISTS smartaccounts_browser_discovery_manifest_check;

ALTER TABLE public.smartaccounts_browser_discovery_authorizations
    ADD CONSTRAINT smartaccounts_browser_discovery_manifest_check CHECK (
        manifest_version IN ('smartaccounts-brave-ui-v1', 'smartaccounts-brave-ui-v2')
    );
