package reports

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewGORMRepositoryWave10UsesPoolFactory(t *testing.T) {
	originalFactory := newGormDBFromPool
	t.Cleanup(func() {
		newGormDBFromPool = originalFactory
	})

	expectedDB := newReportsDryRunDB(t)
	newGormDBFromPool = func(ctx context.Context, pool *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.NotNil(t, pool)
		return expectedDB, nil
	}

	pool := new(pgxpool.Pool)

	repo := NewGORMRepository(pool)

	require.NotNil(t, repo)
	assert.Same(t, expectedDB, repo.db)
}
