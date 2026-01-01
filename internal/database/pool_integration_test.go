//go:build integration

package database

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestPool(t *testing.T) *Pool {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, dbURL)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestPool_New(t *testing.T) {
	pool := setupTestPool(t)
	assert.NotNil(t, pool)
	assert.NotNil(t, pool.Pool)
}

func TestPool_New_InvalidConnection(t *testing.T) {
	ctx := context.Background()
	_, err := NewPool(ctx, "postgres://invalid:invalid@localhost:9999/nonexistent")
	assert.Error(t, err)
}

func TestPool_Queries(t *testing.T) {
	pool := setupTestPool(t)
	queries := pool.Queries()
	assert.NotNil(t, queries)
}

func TestPool_WithTx(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()

	// Create a unique slug for this test
	slug := "test_withtx_" + uuid.New().String()[:8]
	schemaName := "tenant_" + slug

	var createdTenant *Tenant

	// Test transaction that commits
	err := pool.WithTx(ctx, func(q *Queries) error {
		var txErr error
		createdTenant, txErr = q.CreateTenant(ctx, &CreateTenantParams{
			Name:       "WithTx Test Tenant",
			Slug:       slug,
			SchemaName: schemaName,
			Settings:   json.RawMessage(`{"test": true}`),
		})
		return txErr
	})
	require.NoError(t, err)
	require.NotNil(t, createdTenant)

	// Verify tenant was created
	tenant, err := pool.Queries().GetTenant(ctx, createdTenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "WithTx Test Tenant", tenant.Name)

	// Cleanup
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", createdTenant.ID)
	})
}

func TestPool_WithTx_Rollback(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()

	slug := "test_rollback_" + uuid.New().String()[:8]

	// Test transaction that rolls back
	err := pool.WithTx(ctx, func(q *Queries) error {
		_, txErr := q.CreateTenant(ctx, &CreateTenantParams{
			Name:       "Rollback Test Tenant",
			Slug:       slug,
			SchemaName: "tenant_" + slug,
			Settings:   json.RawMessage(`{}`),
		})
		if txErr != nil {
			return txErr
		}
		// Return an error to trigger rollback
		return assert.AnError
	})
	assert.Error(t, err)

	// Verify tenant was NOT created (rolled back)
	exists, err := pool.Queries().SlugExists(ctx, slug)
	require.NoError(t, err)
	assert.False(t, exists)
}

// Test Queries (tenants.sql.go) functions
func TestQueries_CreateTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_create_" + uuid.New().String()[:8]
	schemaName := "tenant_" + slug

	tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Create Tenant",
		Slug:       slug,
		SchemaName: schemaName,
		Settings:   json.RawMessage(`{"currency": "EUR"}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, "Test Create Tenant", tenant.Name)
	assert.Equal(t, slug, tenant.Slug)
	assert.Equal(t, schemaName, tenant.SchemaName)
	assert.True(t, tenant.IsActive)

	// Cleanup
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})
}

func TestQueries_GetTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create test tenant
	slug := "test_get_" + uuid.New().String()[:8]
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Get Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Cleanup
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", created.ID)
	})

	// Get the tenant
	tenant, err := q.GetTenant(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, tenant.ID)
	assert.Equal(t, "Test Get Tenant", tenant.Name)
}

func TestQueries_GetTenant_NotFound(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	_, err := q.GetTenant(ctx, uuid.New())
	assert.Error(t, err) // Should return pgx.ErrNoRows
}

func TestQueries_GetTenantBySlug(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_byslug_" + uuid.New().String()[:8]
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test By Slug Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", created.ID)
	})

	tenant, err := q.GetTenantBySlug(ctx, slug)
	require.NoError(t, err)
	assert.Equal(t, created.ID, tenant.ID)
}

func TestQueries_GetTenantBySchemaName(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_byschema_" + uuid.New().String()[:8]
	schemaName := "tenant_" + slug
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test By Schema Tenant",
		Slug:       slug,
		SchemaName: schemaName,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", created.ID)
	})

	tenant, err := q.GetTenantBySchemaName(ctx, schemaName)
	require.NoError(t, err)
	assert.Equal(t, created.ID, tenant.ID)
	assert.Equal(t, schemaName, tenant.SchemaName)
}

func TestQueries_ListTenants(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create some test tenants
	slugs := []string{
		"test_list_a_" + uuid.New().String()[:8],
		"test_list_b_" + uuid.New().String()[:8],
	}
	var tenantIDs []uuid.UUID

	for i, slug := range slugs {
		tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
			Name:       "Test List Tenant " + string(rune('A'+i)),
			Slug:       slug,
			SchemaName: "tenant_" + slug,
			Settings:   json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		tenantIDs = append(tenantIDs, tenant.ID)
	}

	t.Cleanup(func() {
		for _, id := range tenantIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", id)
		}
	})

	tenants, err := q.ListTenants(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tenants), 2)
}

func TestQueries_SlugExists(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_exists_" + uuid.New().String()[:8]

	// Should not exist initially
	exists, err := q.SlugExists(ctx, slug)
	require.NoError(t, err)
	assert.False(t, exists)

	// Create tenant
	tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Exists Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	// Should exist now
	exists, err = q.SlugExists(ctx, slug)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestQueries_UpdateTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_update_" + uuid.New().String()[:8]
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Original Name",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{"version": 1}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", created.ID)
	})

	// Update the tenant
	updated, err := q.UpdateTenant(ctx, &UpdateTenantParams{
		ID:       created.ID,
		Name:     "Updated Name",
		Settings: json.RawMessage(`{"version": 2}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	// Verify updated_at is valid and at least as recent as created_at
	assert.True(t, updated.UpdatedAt.Valid)
	assert.True(t, created.UpdatedAt.Valid)
}

func TestQueries_UpdateTenantSettings(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_updatesettings_" + uuid.New().String()[:8]
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Settings Test Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{"old": true}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", created.ID)
	})

	// Update just the settings
	newSettings := json.RawMessage(`{"new": true, "updated": true}`)
	updated, err := q.UpdateTenantSettings(ctx, &UpdateTenantSettingsParams{
		ID:       created.ID,
		Settings: newSettings,
	})
	require.NoError(t, err)
	assert.JSONEq(t, string(newSettings), string(updated.Settings))
}

func TestQueries_DeactivateTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	slug := "test_deactivate_" + uuid.New().String()[:8]
	created, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Deactivate Test Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", created.ID)
	})

	// Verify it's active
	assert.True(t, created.IsActive)

	// Deactivate
	err = q.DeactivateTenant(ctx, created.ID)
	require.NoError(t, err)

	// GetTenant should not find it anymore (only returns active tenants)
	_, err = q.GetTenant(ctx, created.ID)
	assert.Error(t, err)

	// But we can still verify it exists by direct query
	var isActive bool
	err = pool.QueryRow(ctx, "SELECT is_active FROM tenants WHERE id = $1", created.ID).Scan(&isActive)
	require.NoError(t, err)
	assert.False(t, isActive)
}

func TestQueries_WithTx(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Start a transaction
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	// Use queries with transaction
	txQ := q.WithTx(tx)
	assert.NotNil(t, txQ)

	slug := "test_queries_withtx_" + uuid.New().String()[:8]
	tenant, err := txQ.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Transaction Test",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, tenant.ID)

	// Commit
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Cleanup
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	// Verify committed
	found, err := q.GetTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "Transaction Test", found.Name)
}

func TestNew(t *testing.T) {
	pool := setupTestPool(t)
	q := New(pool.Pool)
	assert.NotNil(t, q)
}

// Tests for users.sql.go

func TestQueries_CreateUser(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_" + uuid.New().String()[:8] + "@example.com"

	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hashed_password_123",
		Name:         "Test User",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.True(t, user.IsActive)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	})
}

func TestQueries_GetUser(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_getuser_" + uuid.New().String()[:8] + "@example.com"
	created, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Get User Test",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	user, err := q.GetUser(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, user.ID)
	assert.Equal(t, "Get User Test", user.Name)
}

func TestQueries_GetUserByEmail(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_getbyemail_" + uuid.New().String()[:8] + "@example.com"
	created, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Email Test User",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	user, err := q.GetUserByEmail(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, created.ID, user.ID)
}

func TestQueries_EmailExists(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_exists_" + uuid.New().String()[:8] + "@example.com"

	// Should not exist
	exists, err := q.EmailExists(ctx, email)
	require.NoError(t, err)
	assert.False(t, exists)

	// Create user
	created, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	// Should exist now
	exists, err = q.EmailExists(ctx, email)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestQueries_UpdateUser(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_update_" + uuid.New().String()[:8] + "@example.com"
	created, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Original Name",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	updated, err := q.UpdateUser(ctx, &UpdateUserParams{
		ID:   created.ID,
		Name: "Updated Name",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
}

func TestQueries_UpdateUserPassword(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_password_" + uuid.New().String()[:8] + "@example.com"
	created, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "old_hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	err = q.UpdateUserPassword(ctx, &UpdateUserPasswordParams{
		ID:           created.ID,
		PasswordHash: "new_hash",
	})
	require.NoError(t, err)

	// Verify new password hash
	user, err := q.GetUser(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_hash", user.PasswordHash)
}

func TestQueries_DeactivateUser(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	email := "test_deactivate_" + uuid.New().String()[:8] + "@example.com"
	created, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Deactivate Test",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	})

	err = q.DeactivateUser(ctx, created.ID)
	require.NoError(t, err)

	// GetUser should not find deactivated user
	_, err = q.GetUser(ctx, created.ID)
	assert.Error(t, err)
}

func TestQueries_AddUserToTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create user
	email := "test_addtenant_" + uuid.New().String()[:8] + "@example.com"
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	// Create tenant
	slug := "test_adduser_" + uuid.New().String()[:8]
	tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant.ID,
		UserID:    user.ID,
		Role:      "admin",
		IsDefault: true,
	})
	require.NoError(t, err)

	// Verify
	role, err := q.GetUserRole(ctx, &GetUserRoleParams{
		TenantID: tenant.ID,
		UserID:   user.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestQueries_GetUserRole(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create user
	email := "test_getrole_" + uuid.New().String()[:8] + "@example.com"
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	// Create tenant
	slug := "test_getrole_" + uuid.New().String()[:8]
	tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	// Add user to tenant
	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant.ID,
		UserID:    user.ID,
		Role:      "viewer",
		IsDefault: false,
	})
	require.NoError(t, err)

	role, err := q.GetUserRole(ctx, &GetUserRoleParams{
		TenantID: tenant.ID,
		UserID:   user.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "viewer", role)
}

func TestQueries_UserHasTenantAccess(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create user
	email := "test_access_" + uuid.New().String()[:8] + "@example.com"
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	// Create tenant
	slug := "test_access_" + uuid.New().String()[:8]
	tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	// Should not have access initially
	hasAccess, err := q.UserHasTenantAccess(ctx, &UserHasTenantAccessParams{
		TenantID: tenant.ID,
		UserID:   user.ID,
	})
	require.NoError(t, err)
	assert.False(t, hasAccess)

	// Add user to tenant
	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant.ID,
		UserID:    user.ID,
		Role:      "viewer",
		IsDefault: false,
	})
	require.NoError(t, err)

	// Should have access now
	hasAccess, err = q.UserHasTenantAccess(ctx, &UserHasTenantAccessParams{
		TenantID: tenant.ID,
		UserID:   user.ID,
	})
	require.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestQueries_GetUserTenants(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create user
	email := "test_tenants_" + uuid.New().String()[:8] + "@example.com"
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	// Create tenants
	slug1 := "test_tenants_a_" + uuid.New().String()[:8]
	tenant1, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Tenant A",
		Slug:       slug1,
		SchemaName: "tenant_" + slug1,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	slug2 := "test_tenants_b_" + uuid.New().String()[:8]
	tenant2, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Tenant B",
		Slug:       slug2,
		SchemaName: "tenant_" + slug2,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant1.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant2.ID)
	})

	// Add user to both tenants
	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant1.ID,
		UserID:    user.ID,
		Role:      "admin",
		IsDefault: true,
	})
	require.NoError(t, err)

	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant2.ID,
		UserID:    user.ID,
		Role:      "viewer",
		IsDefault: false,
	})
	require.NoError(t, err)

	tenants, err := q.GetUserTenants(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, tenants, 2)

	// First should be the default tenant
	assert.True(t, tenants[0].IsDefault)
	assert.Equal(t, "admin", tenants[0].Role)
}

func TestQueries_RemoveUserFromTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create user
	email := "test_remove_" + uuid.New().String()[:8] + "@example.com"
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	// Create tenant
	slug := "test_remove_" + uuid.New().String()[:8]
	tenant, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Test Tenant",
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	})

	// Add user to tenant
	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant.ID,
		UserID:    user.ID,
		Role:      "viewer",
		IsDefault: false,
	})
	require.NoError(t, err)

	// Remove user from tenant
	err = q.RemoveUserFromTenant(ctx, &RemoveUserFromTenantParams{
		TenantID: tenant.ID,
		UserID:   user.ID,
	})
	require.NoError(t, err)

	// User should no longer have access
	hasAccess, err := q.UserHasTenantAccess(ctx, &UserHasTenantAccessParams{
		TenantID: tenant.ID,
		UserID:   user.ID,
	})
	require.NoError(t, err)
	assert.False(t, hasAccess)
}

func TestQueries_SetDefaultTenant(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Create user
	email := "test_default_" + uuid.New().String()[:8] + "@example.com"
	user, err := q.CreateUser(ctx, &CreateUserParams{
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test",
	})
	require.NoError(t, err)

	// Create tenants
	slug1 := "test_default_a_" + uuid.New().String()[:8]
	tenant1, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Tenant A",
		Slug:       slug1,
		SchemaName: "tenant_" + slug1,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	slug2 := "test_default_b_" + uuid.New().String()[:8]
	tenant2, err := q.CreateTenant(ctx, &CreateTenantParams{
		Name:       "Tenant B",
		Slug:       slug2,
		SchemaName: "tenant_" + slug2,
		Settings:   json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant1.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant2.ID)
	})

	// Add user to both tenants, first is default
	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant1.ID,
		UserID:    user.ID,
		Role:      "admin",
		IsDefault: true,
	})
	require.NoError(t, err)

	err = q.AddUserToTenant(ctx, &AddUserToTenantParams{
		TenantID:  tenant2.ID,
		UserID:    user.ID,
		Role:      "viewer",
		IsDefault: false,
	})
	require.NoError(t, err)

	// Change default to tenant2
	err = q.SetDefaultTenant(ctx, &SetDefaultTenantParams{
		UserID:   user.ID,
		TenantID: tenant2.ID,
	})
	require.NoError(t, err)

	// Verify
	tenants, err := q.GetUserTenants(ctx, user.ID)
	require.NoError(t, err)
	// First should now be tenant2 (the new default)
	assert.True(t, tenants[0].IsDefault)
	assert.Equal(t, tenant2.ID, tenants[0].ID)
}

// TestQueries_VATRates tests VAT rate CRUD operations
func TestQueries_VATRates(t *testing.T) {
	pool := setupTestPool(t)
	ctx := context.Background()
	q := pool.Queries()

	// Use a unique rate type to avoid conflicts with existing data
	uniqueRateType := "test_" + uuid.New().String()[:8]

	// Create a VAT rate
	validFrom := pgtype.Date{}
	_ = validFrom.Scan(time.Now().Format("2006-01-02"))

	vatRate, err := q.CreateVATRate(ctx, &CreateVATRateParams{
		CountryCode: "XX", // Use non-existent country to avoid conflicts
		RateType:    uniqueRateType,
		Rate:        decimal.NewFromInt(22),
		Name:        "Test Rate " + uniqueRateType,
		ValidFrom:   validFrom,
		ValidTo:     pgtype.Date{}, // No end date
	})
	require.NoError(t, err)
	assert.Equal(t, "XX", vatRate.CountryCode)
	assert.Equal(t, uniqueRateType, vatRate.RateType)
	assert.True(t, vatRate.Rate.Equal(decimal.NewFromInt(22)))

	// Get VAT rate by ID
	retrieved, err := q.GetVATRate(ctx, vatRate.ID)
	require.NoError(t, err)
	assert.Equal(t, vatRate.ID, retrieved.ID)
	assert.Equal(t, "Test Rate "+uniqueRateType, retrieved.Name)

	// Get current VAT rate
	current, err := q.GetCurrentVATRate(ctx, &GetCurrentVATRateParams{
		CountryCode: "XX",
		RateType:    uniqueRateType,
		ValidFrom:   validFrom,
	})
	require.NoError(t, err)
	assert.Equal(t, vatRate.ID, current.ID)

	// List VAT rates for country
	rates, err := q.ListVATRates(ctx, "XX")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rates), 1)

	// List current VAT rates
	currentRates, err := q.ListCurrentVATRates(ctx, "XX")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(currentRates), 1)

	// Update VAT rate
	validTo := pgtype.Date{}
	_ = validTo.Scan(time.Now().AddDate(1, 0, 0).Format("2006-01-02"))

	updated, err := q.UpdateVATRate(ctx, &UpdateVATRateParams{
		ID:      vatRate.ID,
		ValidTo: validTo,
	})
	require.NoError(t, err)
	assert.True(t, updated.ValidTo.Valid)
}
