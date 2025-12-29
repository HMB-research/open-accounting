-- Remove onboarding column from tenants
ALTER TABLE tenants DROP COLUMN IF EXISTS onboarding_completed;
