package payments

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentImportWave8InvoiceUpdateFailureSkipsPayment(t *testing.T) {
	repo := NewMockRepository()
	invoices := &MockInvoiceService{recordPaymentErr: errors.New("invoice offline")}
	service := NewServiceWithRepository(repo, invoices)
	invoiceID := "11111111-1111-4111-8111-111111111111"

	result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
		CSVContent: "payment_number,payment_type,payment_date,amount,invoice_id,allocation_amount\n" +
			"PAY-ALLOC,RECEIVED,2026-03-01,10," + invoiceID + ",5\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.PaymentsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "invoice offline")
	require.Len(t, invoices.recordPaymentCalls, 1)
	assert.Equal(t, invoiceID, invoices.recordPaymentCalls[0].invoiceID)
	require.Len(t, repo.payments, 1)
}

func TestPaymentImportWave8AllocationRequiresInvoicingService(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)
	invoiceID := "11111111-1111-4111-8111-111111111111"

	result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
		CSVContent: "payment_number,payment_type,payment_date,amount,invoice_id,allocation_amount\n" +
			"PAY-ALLOC,RECEIVED,2026-03-01,10," + invoiceID + ",5\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.PaymentsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	assert.Contains(t, result.Errors[0].Message, "invoicing service is required for payment allocations")
}

func TestPaymentImportWave8ServiceAndParserBranches(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)

	_, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
		CSVContent: `"unterminated`,
	})
	require.ErrorContains(t, err, "parse csv header")

	rows, err := parsePaymentImportRows("payment_type,payment_date,amount,\n,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	repo := NewMockRepository()
	repo.payments["blank"] = &Payment{ID: "blank", TenantID: "tenant-1", PaymentNumber: " "}
	service = NewServiceWithRepository(repo, nil)

	result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
		CSVContent: "payment_type,payment_date,amount\nRECEIVED,2026-03-01,10\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.PaymentsCreated)
	assert.Empty(t, result.Errors)

	service = NewServiceWithRepository(NewMockRepository(), nil)
	service.contacts = &fakePaymentContactLister{err: errors.New("contacts offline")}
	_, err = service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
		CSVContent: "payment_type,payment_date,amount,contact_code\nRECEIVED,2026-03-01,10,CUST-1\n",
	})
	require.ErrorContains(t, err, "list contacts for payment import: contacts offline")
}
