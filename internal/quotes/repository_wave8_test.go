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

func TestGORMRepositoryWave8NoLineAndSequenceBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)
	quote.Lines = nil
	repo := NewGORMRepository(newQuoteDryRunDB(t,
		withQuoteDryRunUpdateRows(1),
		withQuoteDryRunDeleteRows(1),
	))

	require.NoError(t, repo.Create(ctx, schemaName, quote))
	require.NoError(t, repo.Update(ctx, schemaName, quote))
}

func TestGORMRepositoryWave8DryRunErrorBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)
	expectedErr := errors.New("wave8 repository failure")

	t.Run("create wraps quote insert errors", func(t *testing.T) {
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.Create(ctx, schemaName, quote)

		require.ErrorContains(t, err, "insert quote")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("update wraps quote line delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newQuoteDryRunDB(t,
			withQuoteDryRunUpdateRows(1),
			withQuoteWave8DryRunDeleteError(expectedErr),
		))

		err := repo.Update(ctx, schemaName, quote)

		require.ErrorContains(t, err, "delete quote lines")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("base gorm errors are wrapped by query methods", func(t *testing.T) {
		repo := newQuoteWave8ErrorRepository(t, expectedErr)

		got, err := repo.GetByID(ctx, schemaName, tenantID, quote.ID)
		assert.Nil(t, got)
		require.ErrorContains(t, err, "get quote")
		assert.ErrorIs(t, err, expectedErr)

		list, err := repo.List(ctx, schemaName, tenantID, nil)
		assert.Nil(t, list)
		require.ErrorContains(t, err, "list quotes")
		assert.ErrorIs(t, err, expectedErr)

		number, err := repo.GenerateNumber(ctx, schemaName, tenantID)
		assert.Empty(t, number)
		require.ErrorContains(t, err, "generate quote number")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func newQuoteWave8ErrorRepository(t *testing.T, expectedErr error) *GORMRepository {
	t.Helper()

	db := newQuoteDryRunDB(t)
	db.AddError(expectedErr)
	return NewGORMRepository(db)
}

func withQuoteWave8DryRunDeleteError(expectedErr error) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(quoteDryRunCallbackName(t, "delete_error_wave8"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}
