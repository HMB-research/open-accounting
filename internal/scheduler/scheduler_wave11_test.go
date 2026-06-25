package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func stubWave11GormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestWave11NewSchedulerUsesInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	var called bool
	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		called = true
		return expectedDB, nil
	})

	scheduler := NewScheduler(pool, nil, nil, DefaultConfig())

	require.True(t, called)
	require.NotNil(t, scheduler)
	repo, ok := scheduler.repo.(*GORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, repo.db)
	require.NotNil(t, scheduler.cron)
}

func TestWave11NewSchedulerPanicsOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	require.PanicsWithError(t, "create scheduler GORM repository: pool unavailable", func() {
		_ = NewScheduler(new(pgxpool.Pool), nil, nil, DefaultConfig())
	})
}
