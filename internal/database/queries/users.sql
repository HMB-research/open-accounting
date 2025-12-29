-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 AND is_active = true;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 AND is_active = true;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeactivateUser :exec
UPDATE users
SET is_active = false, updated_at = NOW()
WHERE id = $1;

-- name: EmailExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);

-- name: GetUserTenants :many
SELECT t.*, tu.role, tu.is_default
FROM tenants t
JOIN tenant_users tu ON t.id = tu.tenant_id
WHERE tu.user_id = $1 AND t.is_active = true
ORDER BY tu.is_default DESC, t.name;

-- name: AddUserToTenant :exec
INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, user_id) DO UPDATE
SET role = EXCLUDED.role;

-- name: RemoveUserFromTenant :exec
DELETE FROM tenant_users
WHERE tenant_id = $1 AND user_id = $2;

-- name: GetUserRole :one
SELECT role FROM tenant_users
WHERE tenant_id = $1 AND user_id = $2;

-- name: SetDefaultTenant :exec
UPDATE tenant_users
SET is_default = (tenant_id = $2)
WHERE user_id = $1;

-- name: UserHasTenantAccess :one
SELECT EXISTS(
    SELECT 1 FROM tenant_users
    WHERE tenant_id = $1 AND user_id = $2
);
