ALTER TABLE tenant_users
ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

UPDATE tenant_users
SET is_active = true
WHERE is_active IS NULL;
