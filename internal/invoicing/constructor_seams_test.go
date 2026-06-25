package invoicing

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

func TestConstructorsUseInjectedGormDB(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, pool *pgxpool.Pool, expectedDB *gorm.DB)
	}{
		{
			name: "service",
			run: func(t *testing.T, pool *pgxpool.Pool, expectedDB *gorm.DB) {
				service := NewService(pool, nil)
				require.NotNil(t, service)
				repo, ok := service.repo.(*GORMRepository)
				require.True(t, ok)
				require.Same(t, expectedDB, repo.db)
			},
		},
		{
			name: "interest repository",
			run: func(t *testing.T, pool *pgxpool.Pool, expectedDB *gorm.DB) {
				repo := NewInterestRepository(pool)
				require.NotNil(t, repo)
				require.Same(t, expectedDB, repo.db)
			},
		},
		{
			name: "reminder repository",
			run: func(t *testing.T, pool *pgxpool.Pool, expectedDB *gorm.DB) {
				repo := NewReminderRepository(pool)
				require.NotNil(t, repo)
				require.Same(t, expectedDB, repo.db)
			},
		},
		{
			name: "reminder rule repository",
			run: func(t *testing.T, pool *pgxpool.Pool, expectedDB *gorm.DB) {
				repo := NewReminderRuleRepository(pool)
				require.NotNil(t, repo)
				require.Same(t, expectedDB, repo.db)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedDB := &gorm.DB{}
			pool := new(pgxpool.Pool)
			var called bool
			stubNewGormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
				require.NotNil(t, ctx)
				require.Same(t, pool, got)
				called = true
				return expectedDB, nil
			})

			tt.run(t, pool, expectedDB)
			require.True(t, called)
		})
	}
}

func TestNewInterestRepositoryPanicsOnGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create interest GORM repository: pool unavailable", func() {
		_ = NewInterestRepository(pool)
	})
}
