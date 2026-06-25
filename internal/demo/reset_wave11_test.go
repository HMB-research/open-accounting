package demo

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWave11NewResetRepositoryWithPoolConfiguresAcquire(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.MinConns = 0
	config.MaxConns = 1
	config.HealthCheckPeriod = time.Hour
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	defer pool.Close()

	db := &gorm.DB{}

	repository := NewResetRepository(pool, db)

	require.NotNil(t, repository)
	require.Same(t, pool, repository.pool)
	require.Same(t, db, repository.db)
	require.NotNil(t, repository.acquireResetConn)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	conn, err := repository.acquireResetConn(ctx)
	require.Nil(t, conn)
	require.Error(t, err)
}
