DROP TABLE IF EXISTS public.smartaccounts_browser_onboarding_bindings;

ALTER TABLE public.smartaccounts_browser_pairings
    DROP CONSTRAINT IF EXISTS smartaccounts_browser_pairings_expected_source_check;

ALTER TABLE public.smartaccounts_browser_pairings
    DROP COLUMN IF EXISTS expected_source_company_id;
