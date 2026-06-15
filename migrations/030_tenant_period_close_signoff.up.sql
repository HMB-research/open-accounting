ALTER TABLE tenant_period_closes
    ADD COLUMN IF NOT EXISTS reviewer_sign_off BOOLEAN NOT NULL DEFAULT false;
