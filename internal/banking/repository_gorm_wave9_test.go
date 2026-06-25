package banking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave9CreatePaymentFromTransactionRemainingErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "tenant-1"
	userID := "user-1"
	transaction := &BankTransaction{
		ID:              "transaction-1",
		TenantID:        tenantID,
		BankAccountID:   "account-1",
		TransactionDate: time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC),
		Amount:          decimal.NewFromInt(100),
		Status:          StatusUnmatched,
	}

	t.Run("payment number query error", func(t *testing.T) {
		expectedErr := errors.New("payment number lookup failed")
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		paymentID, err := repo.CreatePaymentFromTransaction(ctx, schemaName, tenantID, userID, transaction)

		assert.Empty(t, paymentID)
		require.ErrorContains(t, err, "generate payment number")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("link transaction no rows", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t,
			withBankingDryRunFixtures(bankingDryRunFixtures{paymentNumbers: []string{"RCV-00001"}}),
			withBankingDryRunCreateCapture(nil),
			withBankingDryRunUpdateRows(0),
		))

		paymentID, err := repo.CreatePaymentFromTransaction(ctx, schemaName, tenantID, userID, transaction)

		assert.Empty(t, paymentID)
		require.ErrorIs(t, err, ErrTransactionAlreadyMatched)
	})
}

func TestGORMRepositoryWave9DuplicateAndImportEdges(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	accountID := "account-1"

	t.Run("duplicate check invalid schema without external id reaches date amount branch", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t))

		duplicate, err := repo.IsTransactionDuplicate(ctx, "tenant-banking", tenantID, accountID, time.Now(), decimal.NewFromInt(10), "")

		assert.False(t, duplicate)
		require.ErrorContains(t, err, "invalid SQL identifier")
	})

	t.Run("duplicate date amount query error", func(t *testing.T) {
		expectedErr := errors.New("candidate lookup failed")
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		duplicate, err := repo.IsTransactionDuplicate(ctx, "tenant_banking", tenantID, accountID, time.Now(), decimal.NewFromInt(10), "")

		assert.False(t, duplicate)
		require.ErrorContains(t, err, "check duplicate")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("list reconciliations query error", func(t *testing.T) {
		expectedErr := errors.New("reconciliation list failed")
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		reconciliations, err := repo.ListReconciliations(ctx, "tenant_banking", tenantID, accountID)

		assert.Nil(t, reconciliations)
		require.ErrorContains(t, err, "list reconciliations")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("increment latest import ignores non-positive counts before database access", func(t *testing.T) {
		var repo *GORMRepository

		require.NoError(t, repo.IncrementLatestImportMatchedCount(ctx, "tenant-banking", tenantID, accountID, 0))
	})
}
