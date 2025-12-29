-- Add onboarding tracking to tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS onboarding_completed BOOLEAN NOT NULL DEFAULT false;

-- Add comment for documentation
COMMENT ON COLUMN tenants.onboarding_completed IS 'Whether the tenant has completed the initial onboarding wizard';
