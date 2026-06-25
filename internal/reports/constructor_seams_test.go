package reports

import (
	"context"
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

func TestNewGORMRepositoryUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubNewGormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	repo := NewGORMRepository(pool)

	require.True(t, called)
	require.NotNil(t, repo)
	require.Same(t, expectedDB, repo.db)
}
