package webhooks

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestWebhooksWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := webhooksWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewService(pool)
	})
}

func TestWebhooksWave9DefaultClient(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil)

	require.NotNil(t, service.httpClient)
	require.Equal(t, defaultHTTPTimeout, service.httpClient.Timeout)
}

func webhooksWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
