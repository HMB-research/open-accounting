package banking

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave10CreatePaymentFromTransactionInvalidSchema(t *testing.T) {
	transaction := &BankTransaction{
		ID:              "transaction-1",
		TenantID:        "tenant-1",
		BankAccountID:   "account-1",
		TransactionDate: time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC),
		Amount:          decimal.NewFromInt(100),
		Status:          StatusUnmatched,
	}
	repo := NewGORMRepository(newBankingDryRunDB(t))

	paymentID, err := repo.CreatePaymentFromTransaction(context.Background(), "tenant-banking", "tenant-1", "user-1", transaction)

	assert.Empty(t, paymentID)
	require.ErrorContains(t, err, "invalid SQL identifier")
}
