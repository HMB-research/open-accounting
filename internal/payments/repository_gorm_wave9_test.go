package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave9CreateReversalPaymentInsertError(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payments"
	tenantID := "tenant-1"
	originalPaymentID := "payment-original"
	reversedAt := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	reversal := paymentsDryRunPayment("payment-reversal", tenantID, reversedAt)
	reversal.ReversalOfPaymentID = &originalPaymentID
	allocation := PaymentAllocation{
		ID:        "allocation-reversal",
		TenantID:  tenantID,
		PaymentID: reversal.ID,
		InvoiceID: "invoice-1",
		Amount:    decimal.NewFromInt(40),
		CreatedAt: reversedAt,
	}
	expectedErr := errors.New("reversal payment insert failed")
	repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunCreateErrors(expectedErr)))

	err := repo.CreateReversal(ctx, schemaName, originalPaymentID, reversal, []PaymentAllocation{allocation}, reversedAt, "user-1", "Duplicate import")

	require.ErrorContains(t, err, "create reversal payment")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryCreateReversalRequiresConfiguredDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	err := repo.createReversal(context.Background(), "tenant_payments", "payment-original", nil, nil, time.Time{}, "user-1", "reason")
	assert.ErrorIs(t, err, errRepositoryDatabaseNotConfigured)
}
