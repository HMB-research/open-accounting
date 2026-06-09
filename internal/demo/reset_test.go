//go:build integration

package demo

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestResetService_ResetUsesRepositoryBoundary(t *testing.T) {
	ctx := context.Background()
	user := ResetUser{
		Number: 1,
		Email:  "demo@example.com",
		Slug:   "demo",
		Schema: "tenant_demo",
	}
	repository := &fakeResetRepository{}
	var receivedNums []int
	service := NewResetServiceWithRepository(repository, func(userNums []int) string {
		receivedNums = append(receivedNums, userNums...)
		return "seed sql"
	})

	require.NoError(t, service.Reset(ctx, []ResetUser{user}, []int{user.Number}))
	require.Equal(t, []int{user.Number}, receivedNums)
	require.Equal(t, 1, repository.calls)
	require.Equal(t, []ResetUser{user}, repository.users)
	require.Equal(t, "seed sql", repository.seedSQL)
}

func TestResetService_ResetPropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("repository failed")
	service := NewResetServiceWithRepository(&fakeResetRepository{err: expectedErr}, func(userNums []int) string {
		return "seed sql"
	})

	err := service.Reset(context.Background(), []ResetUser{{Number: 1}}, []int{1})
	require.ErrorIs(t, err, expectedErr)
}

func TestResetService_ResetRejectsMissingRepository(t *testing.T) {
	service := NewResetServiceWithRepository(nil, func(userNums []int) string {
		return "seed sql"
	})

	err := service.Reset(context.Background(), []ResetUser{{Number: 1}}, []int{1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "demo reset service is not configured")
}

func TestResetService_ResetRejectsEmptySeedScript(t *testing.T) {
	service := NewResetServiceWithRepository(&fakeResetRepository{}, func(userNums []int) string {
		return ""
	})

	err := service.Reset(context.Background(), []ResetUser{{Number: 1}}, []int{1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "demo seed script is empty")
}

func TestResetService_ResetCleansAndSeedsDemoUser(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()

	user := ResetUser{
		Number: 9,
		Email:  "demo-reset-service@example.com",
		Slug:   "demo-reset-service",
		Schema: "tenant_demo_reset_service",
	}
	userID := "a9000000-0000-0000-0000-000000000001"
	tenantID := "b9000000-0000-0000-0000-000000000001"

	cleanup := func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS tenant_demo_reset_service CASCADE")
		_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id = $1", tenantID)
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1 OR slug = $2", tenantID, user.Slug)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1 OR email = $2", userID, user.Email)
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err := pool.Exec(ctx, "CREATE SCHEMA tenant_demo_reset_service")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, is_active)
		VALUES ($1, $2, 'old-hash', 'Old Demo User', true)
	`, userID, user.Email)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, schema_name, settings, is_active)
		VALUES ($1, 'Old Demo Tenant', $2, $3, '{}'::jsonb, true)
	`, tenantID, user.Slug, user.Schema)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
		VALUES ($1, $2, 'viewer', true)
	`, tenantID, userID)
	require.NoError(t, err)

	var receivedNums []int
	service, err := NewResetService(ctx, pool, func(userNums []int) string {
		receivedNums = append(receivedNums, userNums...)
		return `
			CREATE SCHEMA tenant_demo_reset_service;
			CREATE TABLE tenant_demo_reset_service.seed_marker (id INT PRIMARY KEY);
			INSERT INTO tenant_demo_reset_service.seed_marker (id) VALUES (1);
			INSERT INTO users (id, email, password_hash, name, is_active)
			VALUES ('a9000000-0000-0000-0000-000000000001'::uuid, 'demo-reset-service@example.com', 'new-hash', 'New Demo User', true);
			INSERT INTO tenants (id, name, slug, schema_name, settings, is_active)
			VALUES ('b9000000-0000-0000-0000-000000000001'::uuid, 'New Demo Tenant', 'demo-reset-service', 'tenant_demo_reset_service', '{}'::jsonb, true);
			INSERT INTO tenant_users (tenant_id, user_id, role, is_default)
			VALUES ('b9000000-0000-0000-0000-000000000001'::uuid, 'a9000000-0000-0000-0000-000000000001'::uuid, 'admin', true);
		`
	})
	require.NoError(t, err)

	require.NoError(t, service.Reset(ctx, []ResetUser{user}, []int{user.Number}))
	require.Equal(t, []int{user.Number}, receivedNums)

	var userName string
	err = pool.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", userID).Scan(&userName)
	require.NoError(t, err)
	require.Equal(t, "New Demo User", userName)

	var tenantName string
	err = pool.QueryRow(ctx, "SELECT name FROM tenants WHERE id = $1", tenantID).Scan(&tenantName)
	require.NoError(t, err)
	require.Equal(t, "New Demo Tenant", tenantName)

	var role string
	err = pool.QueryRow(ctx, "SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&role)
	require.NoError(t, err)
	require.Equal(t, "admin", role)

	var markerCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenant_demo_reset_service.seed_marker").Scan(&markerCount)
	require.NoError(t, err)
	require.Equal(t, 1, markerCount)
}

func TestResetService_ResetRejectsInvalidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()

	service, err := NewResetService(ctx, pool, func(userNums []int) string {
		return "SELECT 1"
	})
	require.NoError(t, err)

	err = service.Reset(ctx, []ResetUser{{
		Number: 1,
		Email:  "demo@example.com",
		Slug:   "demo",
		Schema: "tenant-demo-invalid",
	}}, []int{1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "quote tenant schema")
}

type fakeResetRepository struct {
	calls   int
	users   []ResetUser
	seedSQL string
	err     error
}

func (r *fakeResetRepository) ResetDemoData(_ context.Context, users []ResetUser, seedSQL string) error {
	r.calls++
	r.users = append([]ResetUser(nil), users...)
	r.seedSQL = seedSQL
	return r.err
}
