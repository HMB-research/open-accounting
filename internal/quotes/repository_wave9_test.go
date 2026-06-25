package quotes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave9UpdateLineInsertAndConversionErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)

	t.Run("Update wraps replacement line insert errors", func(t *testing.T) {
		expectedErr := errors.New("replacement quote line insert failed")
		repo := NewGORMRepository(newQuoteDryRunDB(t,
			withQuoteDryRunUpdateRows(1),
			withQuoteDryRunDeleteRows(1),
			withQuoteDryRunCreateErrorOnCall(1, expectedErr),
		))

		err := repo.Update(ctx, schemaName, quote)

		require.ErrorContains(t, err, "insert quote line")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("SetConvertedToInvoice wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("conversion update failed")
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateError(expectedErr)))

		err := repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quote.ID, "invoice-1")

		require.ErrorContains(t, err, "set converted to invoice")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("GetByID propagates quote line query errors", func(t *testing.T) {
		expectedErr := errors.New("quote line lookup failed")
		repo := NewGORMRepository(newQuoteDryRunDB(t,
			withQuoteDryRunFixtures(quoteDryRunFixtures{quote: quoteToModel(quote)}),
			withQuoteWave9DryRunQueryErrorOnCall(2, expectedErr),
		))

		found, err := repo.GetByID(ctx, schemaName, tenantID, quote.ID)

		assert.Nil(t, found)
		require.ErrorContains(t, err, "get quote lines")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("SetConvertedToInvoice returns not found when no rows change", func(t *testing.T) {
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateRows(0)))

		err := repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quote.ID, "invoice-1")

		require.ErrorIs(t, err, ErrQuoteNotFound)
	})
}

func withQuoteWave9DryRunQueryErrorOnCall(call int, expectedErr error) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var seen int
		err := db.Callback().Query().Before("gorm:query").Register(quoteDryRunCallbackName(t, "query_error_wave9"), func(tx *gorm.DB) {
			seen++
			if seen == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}
