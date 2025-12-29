-- Rollback: Remove user roles enhancements

DROP TABLE IF EXISTS user_invitations CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;

ALTER TABLE tenant_users
DROP COLUMN IF EXISTS invited_by,
DROP COLUMN IF EXISTS invited_at;

ALTER TABLE tenant_users
DROP CONSTRAINT IF EXISTS tenant_users_role_check;
