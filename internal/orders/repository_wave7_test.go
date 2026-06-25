package orders

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepositoryWave7PanicsOnGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create orders GORM repository: pool unavailable", func() {
		_ = NewRepository(pool)
	})
}

func TestNewRepositoryWave7NilPoolReturnsGuardedRepository(t *testing.T) {
	repo := NewRepository(nil)
	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	db, err := repo.dbWithContext(context.Background())
	assert.Nil(t, db)
	assert.ErrorIs(t, err, errOrdersRepositoryDatabaseNotConfigured)
}
