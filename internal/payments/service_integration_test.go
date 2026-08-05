//go:build integration

package payments

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestService_AtomicPaymentWorkflows(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payments-atomic-"+uuid.NewString()+"@example.com")
	contactID := createAtomicPaymentContact(t, pool, tenant.ID, tenant.SchemaName)
	invoiceService := invoicing.NewService(pool, nil)
	paymentService := NewService(pool, invoiceService)
	ctx := context.Background()

	t.Run("create rolls back when invoice update fails", func(t *testing.T) {
		invoice := createAtomicPaymentInvoice(t, invoiceService, tenant.ID, tenant.SchemaName, contactID, userID)
		require.NoError(t, invoiceService.Void(ctx, tenant.ID, tenant.SchemaName, invoice.ID))

		_, err := paymentService.Create(ctx, tenant.ID, tenant.SchemaName, &CreatePaymentRequest{
			PaymentType: PaymentTypeReceived,
			Amount:      decimal.NewFromInt(40),
			Reference:   "atomic-create-" + uuid.NewString(),
			UserID:      userID,
			Allocations: []AllocationRequest{{InvoiceID: invoice.ID, Amount: decimal.NewFromInt(40)}},
		})
		require.ErrorContains(t, err, "cannot record payment on voided invoice")

		assertPaymentCount(t, pool, tenant.SchemaName, tenant.ID, "atomic-create", 0)
		assertInvoicePayment(t, pool, tenant.SchemaName, tenant.ID, invoice.ID, decimal.Zero, string(invoicing.StatusVoided))
	})

	t.Run("allocation rolls back when invoice update fails", func(t *testing.T) {
		invoice := createAtomicPaymentInvoice(t, invoiceService, tenant.ID, tenant.SchemaName, contactID, userID)
		require.NoError(t, invoiceService.Void(ctx, tenant.ID, tenant.SchemaName, invoice.ID))
		payment, err := paymentService.Create(ctx, tenant.ID, tenant.SchemaName, &CreatePaymentRequest{
			PaymentType: PaymentTypeReceived,
			Amount:      decimal.NewFromInt(50),
			Reference:   "atomic-allocation-" + uuid.NewString(),
			UserID:      userID,
		})
		require.NoError(t, err)

		err = paymentService.AllocateToInvoice(ctx, tenant.ID, tenant.SchemaName, payment.ID, invoice.ID, decimal.NewFromInt(50))
		require.ErrorContains(t, err, "cannot record payment on voided invoice")

		assertAllocationCount(t, pool, tenant.SchemaName, tenant.ID, payment.ID, 0)
	})

	t.Run("reversal rolls back when invoice update fails", func(t *testing.T) {
		invoice := createAtomicPaymentInvoice(t, invoiceService, tenant.ID, tenant.SchemaName, contactID, userID)
		payment, err := paymentService.Create(ctx, tenant.ID, tenant.SchemaName, &CreatePaymentRequest{
			PaymentType: PaymentTypeReceived,
			Amount:      decimal.NewFromInt(60),
			Reference:   "atomic-reversal-" + uuid.NewString(),
			UserID:      userID,
			Allocations: []AllocationRequest{{InvoiceID: invoice.ID, Amount: decimal.NewFromInt(60)}},
		})
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `UPDATE `+tenant.SchemaName+`.invoices SET status = 'VOIDED' WHERE id = $1 AND tenant_id = $2`, invoice.ID, tenant.ID)
		require.NoError(t, err)

		_, err = paymentService.Reverse(ctx, tenant.ID, tenant.SchemaName, payment.ID, &ReversePaymentRequest{
			Reason: "atomicity test",
			UserID: userID,
		})
		require.ErrorContains(t, err, "cannot record payment on voided invoice")

		assertReversalCount(t, pool, tenant.SchemaName, tenant.ID, payment.ID, 0)
		assertOriginalPaymentUnreversed(t, pool, tenant.SchemaName, tenant.ID, payment.ID)
	})

	t.Run("concurrent allocations serialize on the payment row", func(t *testing.T) {
		firstInvoice := createAtomicPaymentInvoice(t, invoiceService, tenant.ID, tenant.SchemaName, contactID, userID)
		secondInvoice := createAtomicPaymentInvoice(t, invoiceService, tenant.ID, tenant.SchemaName, contactID, userID)
		payment, err := paymentService.Create(ctx, tenant.ID, tenant.SchemaName, &CreatePaymentRequest{
			PaymentType: PaymentTypeReceived,
			Amount:      decimal.NewFromInt(100),
			Reference:   "atomic-concurrent-" + uuid.NewString(),
			UserID:      userID,
		})
		require.NoError(t, err)

		errs := make(chan error, 2)
		go func() {
			errs <- paymentService.AllocateToInvoice(ctx, tenant.ID, tenant.SchemaName, payment.ID, firstInvoice.ID, decimal.NewFromInt(60))
		}()
		go func() {
			errs <- paymentService.AllocateToInvoice(ctx, tenant.ID, tenant.SchemaName, payment.ID, secondInvoice.ID, decimal.NewFromInt(60))
		}()

		firstErr, secondErr := <-errs, <-errs
		require.NotEqual(t, firstErr == nil, secondErr == nil, "exactly one concurrent allocation should succeed: %v, %v", firstErr, secondErr)
		failedErr := firstErr
		if failedErr == nil {
			failedErr = secondErr
		}
		require.ErrorContains(t, failedErr, "amount exceeds unallocated balance")

		updated, err := paymentService.GetByID(ctx, tenant.ID, tenant.SchemaName, payment.ID)
		require.NoError(t, err)
		require.True(t, updated.TotalAllocated().Equal(decimal.NewFromInt(60)))
	})
}

func createAtomicPaymentContact(t *testing.T, pool *pgxpool.Pool, tenantID, schemaName string) string {
	t.Helper()
	contactID := uuid.NewString()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO `+schemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', $3, NOW(), NOW())
	`, contactID, tenantID, "Atomic payment contact "+contactID)
	require.NoError(t, err)
	return contactID
}

func createAtomicPaymentInvoice(t *testing.T, service *invoicing.Service, tenantID, schemaName, contactID, userID string) *invoicing.Invoice {
	t.Helper()
	invoice, err := service.Create(context.Background(), tenantID, schemaName, &invoicing.CreateInvoiceRequest{
		InvoiceType: invoicing.InvoiceTypeSales,
		ContactID:   contactID,
		IssueDate:   time.Now().UTC(),
		DueDate:     time.Now().UTC().AddDate(0, 0, 30),
		UserID:      userID,
		Lines: []invoicing.CreateInvoiceLineRequest{{
			Description: "Atomic payment test",
			Quantity:    decimal.NewFromInt(1),
			UnitPrice:   decimal.NewFromInt(100),
			VATRate:     decimal.NewFromInt(22),
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Send(context.Background(), tenantID, schemaName, invoice.ID))
	return invoice
}

func assertPaymentCount(t *testing.T, pool *pgxpool.Pool, schemaName, tenantID, referencePrefix string, expected int) {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+schemaName+`.payments WHERE tenant_id = $1 AND reference LIKE $2`, tenantID, referencePrefix+"%").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, expected, count)
}

func assertInvoicePayment(t *testing.T, pool *pgxpool.Pool, schemaName, tenantID, invoiceID string, expectedAmount decimal.Decimal, expectedStatus string) {
	t.Helper()
	var amount string
	var status string
	err := pool.QueryRow(context.Background(), `SELECT amount_paid, status FROM `+schemaName+`.invoices WHERE id = $1 AND tenant_id = $2`, invoiceID, tenantID).Scan(&amount, &status)
	require.NoError(t, err)
	require.True(t, expectedAmount.Equal(decimal.RequireFromString(amount)), "amount_paid = %s", amount)
	require.Equal(t, expectedStatus, status)
}

func assertAllocationCount(t *testing.T, pool *pgxpool.Pool, schemaName, tenantID, paymentID string, expected int) {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+schemaName+`.payment_allocations WHERE tenant_id = $1 AND payment_id = $2`, tenantID, paymentID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, expected, count)
}

func assertReversalCount(t *testing.T, pool *pgxpool.Pool, schemaName, tenantID, originalPaymentID string, expected int) {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+schemaName+`.payments WHERE tenant_id = $1 AND reversal_of_payment_id = $2`, tenantID, originalPaymentID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, expected, count)
}

func assertOriginalPaymentUnreversed(t *testing.T, pool *pgxpool.Pool, schemaName, tenantID, paymentID string) {
	t.Helper()
	var reversedBy *string
	err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT reversed_by_payment_id FROM %s.payments WHERE id = $1 AND tenant_id = $2`, schemaName), paymentID, tenantID).Scan(&reversedBy)
	require.NoError(t, err)
	require.Nil(t, reversedBy)
}
