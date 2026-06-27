package accounting

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func stubNewGormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func stubNewGormDBFromPoolError(t *testing.T, err error) *pgxpool.Pool {
	t.Helper()
	pool := new(pgxpool.Pool)
	stubNewGormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, err
	})
	return pool
}

func TestNewRepositoryUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubNewGormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	repo := NewRepository(pool)

	require.True(t, called)
	require.NotNil(t, repo)
	require.Same(t, expectedDB, repo.db)
}

func TestNewRepositoryPanicsOnGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create accounting GORM repository: pool unavailable", func() {
		_ = NewRepository(pool)
	})
}

func TestNewCostCenterRepositoryUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubNewGormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	repo := NewCostCenterRepository(pool)

	require.True(t, called)
	require.NotNil(t, repo)
	require.Same(t, expectedDB, repo.db)
}
