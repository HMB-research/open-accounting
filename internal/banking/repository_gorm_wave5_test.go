package banking

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave5ListPaymentMatchCandidatesScansRows(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	paymentDate := time.Date(2026, time.June, 25, 15, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunScanRows(bankingDryRunRowSet{
		columns: []string{"id", "payment_number", "payment_date", "amount", "contact_name", "reference"},
		values: [][]driver.Value{{
			"payment-1",
			"RCV-00007",
			paymentDate,
			"125.50",
			"Acme OU",
			"INV-2026-001",
		}},
	})))

	candidates, err := repo.ListPaymentMatchCandidates(ctx, "tenant_banking", tenantID, payments.PaymentTypeReceived, decimal.NewFromInt(120), 0)

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "payment-1", candidates[0].ID)
	assert.Equal(t, "RCV-00007", candidates[0].PaymentNumber)
	assert.Equal(t, paymentDate, candidates[0].PaymentDate)
	assert.True(t, candidates[0].Amount.Equal(decimal.RequireFromString("125.50")))
	assert.Equal(t, "Acme OU", candidates[0].ContactName)
	assert.Equal(t, "INV-2026-001", candidates[0].Reference)
}

func TestGORMRepositoryWave5CreatePaymentFromTransactionGuards(t *testing.T) {
	ctx := context.Background()
	repo := NewGORMRepository(newBankingDryRunDB(t))
	transaction := &BankTransaction{
		ID:       "transaction-1",
		TenantID: "tenant-1",
		Status:   StatusMatched,
	}

	paymentID, err := repo.CreatePaymentFromTransaction(ctx, "tenant_banking", "tenant-1", "user-1", nil)
	assert.Empty(t, paymentID)
	assert.ErrorIs(t, err, ErrTransactionNotFound)

	paymentID, err = repo.CreatePaymentFromTransaction(ctx, "tenant_banking", "tenant-2", "user-1", transaction)
	assert.Empty(t, paymentID)
	assert.ErrorIs(t, err, ErrTransactionNotFound)

	paymentID, err = repo.CreatePaymentFromTransaction(ctx, "tenant_banking", "tenant-1", "user-1", transaction)
	assert.Empty(t, paymentID)
	assert.ErrorIs(t, err, ErrTransactionAlreadyMatched)
}

func TestGORMRepositoryWave5IncrementLatestImportMatchedCountBranches(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	accountID := "22222222-2222-4222-8222-222222222222"

	t.Run("ignores non-positive increments", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t))

		require.NoError(t, repo.IncrementLatestImportMatchedCount(ctx, "tenant_banking", tenantID, accountID, 0))
	})

	t.Run("ignores missing latest import", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(gorm.ErrRecordNotFound)))

		require.NoError(t, repo.IncrementLatestImportMatchedCount(ctx, "tenant_banking", tenantID, accountID, 1))
	})

	t.Run("wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("import update failed")
		repo := NewGORMRepository(newBankingDryRunDB(t,
			withBankingDryRunFixtures(bankingDryRunFixtures{
				importRecord: &models.BankStatementImport{
					ID:            "import-1",
					TenantID:      tenantID,
					BankAccountID: accountID,
					CreatedAt:     time.Now(),
				},
			}),
			withBankingDryRunUpdateError(expectedErr),
		))

		err := repo.IncrementLatestImportMatchedCount(ctx, "tenant_banking", tenantID, accountID, 2)

		require.ErrorContains(t, err, "increment matched import count")
		assert.ErrorIs(t, err, expectedErr)
	})
}
