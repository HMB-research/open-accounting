-- name: GetTenant :one
SELECT * FROM tenants
WHERE id = $1 AND is_active = true;

-- name: GetTenantBySlug :one
SELECT * FROM tenants
WHERE slug = $1 AND is_active = true;

-- name: GetTenantBySchemaName :one
SELECT * FROM tenants
WHERE schema_name = $1 AND is_active = true;

-- name: ListTenants :many
SELECT * FROM tenants
WHERE is_active = true
ORDER BY name;

-- name: CreateTenant :one
INSERT INTO tenants (name, slug, schema_name, settings)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateTenant :one
UPDATE tenants
SET name = $2, settings = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateTenantSettings :one
UPDATE tenants
SET settings = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeactivateTenant :exec
UPDATE tenants
SET is_active = false, updated_at = NOW()
WHERE id = $1;

-- name: SlugExists :one
SELECT EXISTS(SELECT 1 FROM tenants WHERE slug = $1);
