package tax

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestTaxWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := taxWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewService(pool)
	})
}

func TestTaxWave9ImportParserEdges(t *testing.T) {
	t.Run("header parse error", func(t *testing.T) {
		_, err := parseKMDHistoryImportRows(`"unterminated`)
		require.ErrorContains(t, err, "parse csv header")
	})

	t.Run("status normalizes dashes and case", func(t *testing.T) {
		status, err := parseKMDHistoryStatus("SUBMITTED")
		require.NoError(t, err)
		require.Equal(t, KMDStatusSubmitted, status)
	})
}

func taxWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
