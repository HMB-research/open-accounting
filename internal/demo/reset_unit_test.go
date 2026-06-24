package demo

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResetServiceUnitPaths(t *testing.T) {
	ctx := context.Background()
	user := ResetUser{
		Number: 1,
		Email:  "demo@example.com",
		Slug:   "demo",
		Schema: "tenant_demo",
	}

	repository := &unitResetRepository{}
	var receivedNums []int
	service := NewResetServiceWithRepository(repository, func(userNums []int) string {
		receivedNums = append(receivedNums, userNums...)
		return "seed sql"
	})

	require.NoError(t, service.Reset(ctx, []ResetUser{user}, []int{user.Number}))
	assert.Equal(t, []int{user.Number}, receivedNums)
	assert.Equal(t, 1, repository.calls)
	assert.Equal(t, []ResetUser{user}, repository.users)
	assert.Equal(t, "seed sql", repository.seedSQL)

	expectedErr := errors.New("repository failed")
	service = NewResetServiceWithRepository(&unitResetRepository{err: expectedErr}, func(userNums []int) string {
		return "seed sql"
	})
	require.ErrorIs(t, service.Reset(ctx, []ResetUser{user}, []int{user.Number}), expectedErr)

	service = NewResetServiceWithRepository(nil, func(userNums []int) string {
		return "seed sql"
	})
	err := service.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo reset service is not configured")

	var nilService *ResetService
	err = nilService.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo reset service is not configured")

	service = NewResetServiceWithRepository(&unitResetRepository{}, nil)
	err = service.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo seed script provider is not configured")

	service = NewResetServiceWithRepository(&unitResetRepository{}, func(userNums []int) string {
		return ""
	})
	err = service.Reset(ctx, []ResetUser{user}, []int{user.Number})
	require.ErrorContains(t, err, "demo seed script is empty")
}

func TestDemoConstructorsRejectNilPools(t *testing.T) {
	ctx := context.Background()

	service, err := NewResetService(ctx, nil, func(userNums []int) string { return "seed sql" })
	require.ErrorContains(t, err, "database pool is not configured")
	assert.Nil(t, service)

	repository, err := NewResetRepositoryFromPool(ctx, nil)
	require.ErrorContains(t, err, "database pool is not configured")
	assert.Nil(t, repository)

	reader, err := NewStatusReader(nil)
	require.ErrorContains(t, err, "database pool is not configured")
	assert.Nil(t, reader)
}

func TestResetRepositoryUnitConfiguration(t *testing.T) {
	pool := &pgxpool.Pool{}
	db := newDryRunGormDB(t)

	repository := NewResetRepository(pool, db)
	require.NotNil(t, repository)
	assert.Same(t, pool, repository.pool)
	assert.Same(t, db, repository.db)
	assert.Equal(t, ResetAdvisoryLockKey, repository.advisoryLockKey)

	tests := []struct {
		name       string
		repository *GORMResetRepository
	}{
		{name: "nil receiver"},
		{name: "missing pool", repository: NewResetRepository(nil, db)},
		{name: "missing db", repository: NewResetRepository(pool, nil)},
		{name: "missing both", repository: NewResetRepository(nil, nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.repository.ResetDemoData(context.Background(), []ResetUser{{Number: 1}}, "seed sql")
			require.ErrorContains(t, err, "demo reset repository is not configured")
		})
	}
}

func TestResetRepositoryDryRunCleanupQueries(t *testing.T) {
	ctx := context.Background()
	repository := NewResetRepository(nil, newDryRunGormDB(t))
	user := ResetUser{
		Email: "demo@example.com",
		Slug:  "demo-company",
	}

	require.NoError(t, repository.cleanPublicDemoPluginFixtures(ctx))
	require.NoError(t, repository.cleanPublicDemoRows(ctx, user))
}

func TestResetRepositoryDryRunCleanupErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("delete failed")

	t.Run("plugin fixtures", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteError(t, db, expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoPluginFixtures(ctx)

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean demo plugin fixture tenant links")
	})

	t.Run("demo rows", func(t *testing.T) {
		db := newDryRunGormDB(t)
		registerDryRunDeleteError(t, db, expectedErr)
		repository := NewResetRepository(nil, db)

		err := repository.cleanPublicDemoRows(ctx, ResetUser{
			Email: "demo@example.com",
			Slug:  "demo-company",
		})

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "clean tenant users for demo-company")
	})
}

func TestResetRepositoryDropTenantSchemaRejectsInvalidSchema(t *testing.T) {
	repository := NewResetRepository(nil, nil)

	err := repository.dropTenantSchema(context.Background(), nil, "tenant-demo")

	require.ErrorContains(t, err, "quote tenant schema")
}

func registerDryRunDeleteError(t *testing.T, db *gorm.DB, err error) {
	t.Helper()

	callbackName := fmt.Sprintf("demo_reset_unit:delete_error_%d", atomic.AddUint64(&dryRunCallbackID, 1))
	registerErr := db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		tx.AddError(err)
	})
	require.NoError(t, registerErr)
}

type unitResetRepository struct {
	calls   int
	users   []ResetUser
	seedSQL string
	err     error
}

func (r *unitResetRepository) ResetDemoData(_ context.Context, users []ResetUser, seedSQL string) error {
	r.calls++
	r.users = append([]ResetUser(nil), users...)
	r.seedSQL = seedSQL
	return r.err
}
