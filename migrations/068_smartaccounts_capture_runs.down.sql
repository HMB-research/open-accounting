ALTER TABLE public.smartaccounts_sync_controls
    DROP CONSTRAINT IF EXISTS smartaccounts_sync_controls_capture_run_id_check;

ALTER TABLE public.smartaccounts_sync_controls
    DROP COLUMN IF EXISTS capture_run_id;
