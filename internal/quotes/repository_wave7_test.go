package quotes

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepositoryWave7PanicsOnUnreachablePool(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewRepository(pool)
	})
}

func TestNewRepositoryWave7NilPoolReturnsGuardedRepository(t *testing.T) {
	repo := NewRepository(nil)
	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	db, err := repo.dbWithContext(context.Background())
	assert.Nil(t, db)
	assert.ErrorIs(t, err, errQuotesRepositoryDatabaseNotConfigured)
}
