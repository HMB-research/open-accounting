package banking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave8BaseGormErrorBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	expectedErr := errors.New("wave8 gorm failure")
	rule := &BankMatchRule{ID: "rule-1", TenantID: tenantID, Name: "Rule", MatchField: BankMatchFieldDescription, Pattern: "INV", UpdatedAt: now}
	update := TransactionReviewUpdate{ReviewedBy: "user-1", ReviewedAt: now}

	err := newBankingWave8ErrorRepository(t, expectedErr).DeleteBankAccount(ctx, schemaName, tenantID, "account-1")
	require.ErrorContains(t, err, "delete bank account")
	assert.ErrorIs(t, err, expectedErr)

	err = newBankingWave8ErrorRepository(t, expectedErr).UpdateBankMatchRule(ctx, schemaName, rule)
	require.ErrorContains(t, err, "update bank match rule")
	assert.ErrorIs(t, err, expectedErr)

	err = newBankingWave8ErrorRepository(t, expectedErr).DeleteBankMatchRule(ctx, schemaName, tenantID, rule.ID)
	require.ErrorContains(t, err, "delete bank match rule")
	assert.ErrorIs(t, err, expectedErr)

	err = newBankingWave8ErrorRepository(t, expectedErr).MatchTransaction(ctx, schemaName, tenantID, "transaction-1", "payment-1")
	require.ErrorContains(t, err, "match transaction")
	assert.ErrorIs(t, err, expectedErr)

	err = newBankingWave8ErrorRepository(t, expectedErr).UnmatchTransaction(ctx, schemaName, tenantID, "transaction-1")
	require.ErrorContains(t, err, "unmatch transaction")
	assert.ErrorIs(t, err, expectedErr)

	transaction, err := newBankingWave8ErrorRepository(t, expectedErr).UpdateTransactionReview(ctx, schemaName, tenantID, "transaction-1", update)
	assert.Nil(t, transaction)
	require.ErrorContains(t, err, "update transaction review")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave8CreatePaymentFromTransactionErrors(t *testing.T) {
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
		Currency:        "EUR",
		Status:          StatusUnmatched,
	}

	t.Run("payment insert error", func(t *testing.T) {
		expectedErr := errors.New("create payment failed")
		repo := NewGORMRepository(newBankingDryRunDB(t,
			withBankingDryRunFixtures(bankingDryRunFixtures{paymentNumbers: []string{"RCV-00001"}}),
			withBankingDryRunCreateError(expectedErr),
		))

		paymentID, err := repo.CreatePaymentFromTransaction(ctx, schemaName, tenantID, userID, transaction)

		assert.Empty(t, paymentID)
		require.ErrorContains(t, err, "create payment")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("link transaction update error", func(t *testing.T) {
		expectedErr := errors.New("link transaction failed")
		repo := NewGORMRepository(newBankingDryRunDB(t,
			withBankingDryRunFixtures(bankingDryRunFixtures{paymentNumbers: []string{"RCV-00001"}}),
			withBankingDryRunCreateCapture(nil),
			withBankingWave8UpdateErrorAfter(expectedErr),
		))

		paymentID, err := repo.CreatePaymentFromTransaction(ctx, schemaName, tenantID, userID, transaction)

		assert.Empty(t, paymentID)
		require.ErrorContains(t, err, "link transaction")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryWave8ReconciliationAndImportErrorBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "tenant-1"
	accountID := "account-1"
	reconciliationID := "reconciliation-1"
	transactionID := "transaction-1"
	expectedErr := errors.New("wave8 transaction failure")

	t.Run("complete reconciliation update transaction error", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t,
			withBankingDryRunUpdateRows(1, 1),
			withBankingWave8UpdateErrorOnCallAfter(2, expectedErr),
		))

		err := repo.CompleteReconciliation(ctx, schemaName, tenantID, reconciliationID)

		require.ErrorContains(t, err, "update transactions")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("add transaction to reconciliation update error", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingWave8UpdateErrorAfter(expectedErr)))

		err := repo.AddTransactionToReconciliation(ctx, schemaName, tenantID, transactionID, reconciliationID)

		require.ErrorContains(t, err, "add to reconciliation")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("latest import lookup error", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		err := repo.IncrementLatestImportMatchedCount(ctx, schemaName, tenantID, accountID, 1)

		require.ErrorContains(t, err, "find latest import record")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func newBankingWave8ErrorRepository(t *testing.T, expectedErr error) *GORMRepository {
	t.Helper()

	db := newBankingDryRunDB(t)
	db.AddError(expectedErr)
	return NewGORMRepository(db)
}

func withBankingWave8UpdateErrorAfter(expectedErr error) bankingDryRunDBOption {
	return withBankingWave8UpdateErrorOnCallAfter(1, expectedErr)
}

func withBankingWave8UpdateErrorOnCallAfter(call int, expectedErr error) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var calls int
		err := db.Callback().Update().After("gorm:update").Register(bankingDryRunCallbackName(t, "update_error_wave8"), func(tx *gorm.DB) {
			calls++
			if calls == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}
