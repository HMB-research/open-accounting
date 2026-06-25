package banking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave6LookupErrorBranches(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"

	t.Run("maps not found errors", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(gorm.ErrRecordNotFound)))

		account, err := repo.GetBankAccount(ctx, "tenant_banking", tenantID, "account-1")
		assert.Nil(t, account)
		assert.ErrorIs(t, err, ErrBankAccountNotFound)

		rule, err := repo.GetBankMatchRule(ctx, "tenant_banking", tenantID, "rule-1")
		assert.Nil(t, rule)
		assert.ErrorIs(t, err, ErrBankMatchRuleNotFound)

		transaction, err := repo.GetTransaction(ctx, "tenant_banking", tenantID, "transaction-1")
		assert.Nil(t, transaction)
		assert.ErrorIs(t, err, ErrTransactionNotFound)

		reconciliation, err := repo.GetReconciliation(ctx, "tenant_banking", tenantID, "reconciliation-1")
		assert.Nil(t, reconciliation)
		assert.ErrorIs(t, err, ErrReconciliationNotFound)
	})

	t.Run("wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("query failed")
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		_, err := repo.GetBankAccount(ctx, "tenant_banking", tenantID, "account-1")
		require.ErrorContains(t, err, "get bank account")
		assert.ErrorIs(t, err, expectedErr)

		_, err = repo.GetBankMatchRule(ctx, "tenant_banking", tenantID, "rule-1")
		require.ErrorContains(t, err, "get bank match rule")
		assert.ErrorIs(t, err, expectedErr)

		_, err = repo.GetTransaction(ctx, "tenant_banking", tenantID, "transaction-1")
		require.ErrorContains(t, err, "get transaction")
		assert.ErrorIs(t, err, expectedErr)

		_, err = repo.GetReconciliation(ctx, "tenant_banking", tenantID, "reconciliation-1")
		require.ErrorContains(t, err, "get reconciliation")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryWave6MutationErrorBranches(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	account := &BankAccount{ID: "account-1", TenantID: tenantID, Name: "Main", AccountNumber: "EE123", Currency: "EUR", IsActive: true}
	rule := &BankMatchRule{ID: "rule-1", TenantID: tenantID, Name: "Utilities", MatchField: BankMatchFieldDescription, Pattern: "UTIL", MinConfidence: 0.75, IsActive: true, UpdatedAt: now}
	transaction := &BankTransaction{ID: "transaction-1", TenantID: tenantID, Amount: decimal.NewFromInt(25), Currency: "EUR", Status: StatusUnmatched, TransactionDate: now}
	reconciliation := &BankReconciliation{ID: "reconciliation-1", TenantID: tenantID, BankAccountID: account.ID, StatementDate: now, Status: ReconciliationInProgress}
	importRecord := &BankStatementImport{ID: "import-1", TenantID: tenantID, BankAccountID: account.ID, FileName: "statement.csv", CreatedAt: now}

	t.Run("wraps create and update errors", func(t *testing.T) {
		expectedErr := errors.New("write failed")

		err := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunCreateError(expectedErr))).CreateBankAccount(ctx, "tenant_banking", account)
		require.ErrorContains(t, err, "insert bank account")
		assert.ErrorIs(t, err, expectedErr)

		err = NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunUpdateError(expectedErr))).UpdateBankAccount(ctx, "tenant_banking", account)
		require.ErrorContains(t, err, "update bank account")
		assert.ErrorIs(t, err, expectedErr)

		err = NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunUpdateError(expectedErr))).UnsetDefaultAccounts(ctx, "tenant_banking", tenantID)
		require.ErrorContains(t, err, "unset default")
		assert.ErrorIs(t, err, expectedErr)

		err = NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunCreateError(expectedErr))).CreateBankMatchRule(ctx, "tenant_banking", rule)
		require.ErrorContains(t, err, "insert bank match rule")
		assert.ErrorIs(t, err, expectedErr)

		err = NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunCreateError(expectedErr))).CreateTransaction(ctx, "tenant_banking", transaction)
		require.ErrorContains(t, err, "insert transaction")
		assert.ErrorIs(t, err, expectedErr)

		err = NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunCreateError(expectedErr))).CreateReconciliation(ctx, "tenant_banking", reconciliation)
		require.ErrorContains(t, err, "create reconciliation")
		assert.ErrorIs(t, err, expectedErr)

		err = NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunCreateError(expectedErr))).CreateImportRecord(ctx, "tenant_banking", importRecord)
		require.ErrorContains(t, err, "create import record")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("maps zero affected rows", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunUpdateRows(0), withBankingDryRunDeleteRows(0)))

		assert.ErrorIs(t, repo.DeleteBankAccount(ctx, "tenant_banking", tenantID, account.ID), ErrBankAccountNotFound)
		assert.ErrorIs(t, repo.UpdateBankMatchRule(ctx, "tenant_banking", rule), ErrBankMatchRuleNotFound)
		assert.ErrorIs(t, repo.DeleteBankMatchRule(ctx, "tenant_banking", tenantID, rule.ID), ErrBankMatchRuleNotFound)
		assert.ErrorIs(t, repo.MatchTransaction(ctx, "tenant_banking", tenantID, transaction.ID, "payment-1"), ErrTransactionAlreadyMatched)
		assert.ErrorIs(t, repo.UnmatchTransaction(ctx, "tenant_banking", tenantID, transaction.ID), ErrTransactionNotMatched)
		assert.ErrorIs(t, repo.AddTransactionToReconciliation(ctx, "tenant_banking", tenantID, transaction.ID, reconciliation.ID), ErrTransactionNotFound)

		_, err := repo.UpdateTransactionReview(ctx, "tenant_banking", tenantID, transaction.ID, TransactionReviewUpdate{ReviewedBy: "user-1", ReviewedAt: now})
		assert.ErrorIs(t, err, ErrTransactionNotFound)
	})
}

func TestGORMRepositoryWave6ListAndCountErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	expectedErr := errors.New("read failed")
	repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

	_, err := repo.ListBankAccounts(ctx, "tenant_banking", tenantID, &BankAccountFilter{Currency: "EUR"})
	require.ErrorContains(t, err, "list bank accounts")
	assert.ErrorIs(t, err, expectedErr)

	_, err = repo.CountTransactionsForAccount(ctx, "tenant_banking", "account-1")
	require.ErrorContains(t, err, "count transactions")
	assert.ErrorIs(t, err, expectedErr)

	_, err = repo.ListBankMatchRules(ctx, "tenant_banking", tenantID, &BankMatchRuleFilter{ActiveOnly: true, BankAccountID: "account-1"})
	require.ErrorContains(t, err, "list bank match rules")
	assert.ErrorIs(t, err, expectedErr)

	_, err = repo.GetImportHistory(ctx, "tenant_banking", tenantID, "account-1")
	require.ErrorContains(t, err, "get import history")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave6TransactionDuplicateBranches(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	accountID := "22222222-2222-4222-8222-222222222222"
	txDate := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunFixtures(bankingDryRunFixtures{
		counts: []int64{0},
		transactions: []models.BankTransaction{{
			ID:              "transaction-1",
			TenantID:        tenantID,
			BankAccountID:   accountID,
			TransactionDate: txDate,
			Amount:          models.NewDecimal(decimal.RequireFromString("12.34")),
		}},
	})))

	duplicate, err := repo.IsTransactionDuplicate(ctx, "tenant_banking", tenantID, accountID, txDate, decimal.RequireFromString("12.34"), "external-1")

	require.NoError(t, err)
	assert.True(t, duplicate)
}

func TestGORMRepositoryWave6NilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	transaction := &BankTransaction{
		ID:              "transaction-1",
		TenantID:        "tenant-1",
		Status:          StatusUnmatched,
		Amount:          decimal.NewFromInt(15),
		TransactionDate: time.Now(),
	}
	repo := NewGORMRepository(nil)

	candidates, err := repo.ListPaymentMatchCandidates(ctx, "tenant_banking", "tenant-1", "", decimal.NewFromInt(15), 10)
	assert.Nil(t, candidates)
	require.ErrorContains(t, err, "banking repository database is not configured")

	paymentID, err := repo.CreatePaymentFromTransaction(ctx, "tenant_banking", "tenant-1", "user-1", transaction)
	assert.Empty(t, paymentID)
	require.ErrorContains(t, err, "banking repository database is not configured")
}

func withBankingDryRunCreateError(expectedErr error) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(bankingDryRunCallbackName(t, "create_error_wave6"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}
