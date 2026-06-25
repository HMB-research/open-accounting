package payments

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPaymentsWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := paymentsWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewService(pool, nil)
	})
}

func TestPaymentsWave9ImportParserHeaderError(t *testing.T) {
	_, err := parsePaymentImportRows(`"unterminated`)

	require.ErrorContains(t, err, "parse csv header")
}

func TestPaymentsWave9SEPAValidationEdges(t *testing.T) {
	t.Run("invalid creditor BIC", func(t *testing.T) {
		_, _, err := sepaTransactionFromLine(0, SEPACreditTransferLine{
			CreditorName: "Supplier",
			CreditorIBAN: "EE382200221020145685",
			CreditorBIC:  "BAD",
			Amount:       decimal.NewFromInt(10),
		})

		require.ErrorContains(t, err, "creditor_bic")
	})

	t.Run("IBAN checksum failure", func(t *testing.T) {
		_, err := normalizeIBAN("EE382200221020145686")

		require.ErrorContains(t, err, "checksum")
	})
}

func paymentsWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
