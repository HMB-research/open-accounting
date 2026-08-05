package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/contactrefs"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentImportServiceEdges(t *testing.T) {
	t.Run("rejects missing content before repository access", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), nil)

		_, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", nil)

		require.ErrorContains(t, err, "csv_content is required")
	})

	t.Run("rejects files with only headers and blank rows", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), nil)

		_, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
			CSVContent: "payment_type,payment_date,amount\n,,\n",
		})

		require.ErrorContains(t, err, "no payments found in CSV")
	})

	t.Run("returns existing payment list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.listErr = errors.New("database unavailable")
		service := NewServiceWithRepository(repo, nil)

		_, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
			CSVContent: "payment_type,payment_date,amount\nRECEIVED,2026-03-01,10\n",
		})

		require.ErrorContains(t, err, "list existing payments: database unavailable")
	})

	t.Run("records payment create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createErr = errors.New("insert failed")
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
			CSVContent: "payment_number,payment_type,payment_date,amount,reference\nPAY-001,RECEIVED,2026-03-01,10,Receipt\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.PaymentsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "PAY-001", result.Errors[0].PaymentNumber)
		assert.Equal(t, "Receipt", result.Errors[0].Reference)
		assert.Contains(t, result.Errors[0].Message, "insert failed")
	})

	t.Run("records generated number errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.getNextNumErr = errors.New("sequence unavailable")
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
			CSVContent: "payment_type,payment_date,amount,reference\nRECEIVED,2026-03-01,10,Receipt\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.PaymentsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "generate payment number: sequence unavailable")
	})

	t.Run("records duplicate existing payment numbers as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.payments["existing"] = &Payment{
			ID:            "existing",
			TenantID:      "tenant-1",
			PaymentNumber: "PMT-001",
			PaymentType:   PaymentTypeReceived,
			PaymentDate:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Amount:        decimal.NewFromInt(10),
			Currency:      "EUR",
			ExchangeRate:  decimal.NewFromInt(1),
			BaseAmount:    decimal.NewFromInt(10),
		}
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
			CSVContent: "payment_number,payment_type,payment_date,amount,reference\nPMT-001,RECEIVED,2026-03-01,10,Receipt\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.PaymentsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, `duplicate payment_number "PMT-001"`)
	})

	t.Run("records allocation create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createAllocErr = errors.New("allocation insert failed")
		service := NewServiceWithRepository(repo, &MockInvoiceService{})
		invoiceID := "11111111-1111-4111-8111-111111111111"

		result, err := service.ImportPaymentsCSV(context.Background(), "tenant-1", "test_schema", &ImportPaymentsRequest{
			CSVContent: "payment_number,payment_type,payment_date,amount,invoice_id,allocation_amount\n" +
				"PAY-ALLOC,RECEIVED,2026-03-01,10," + invoiceID + ",5\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.PaymentsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "insert allocation: allocation insert failed")
	})
}

func TestPaymentImportRowsEdges(t *testing.T) {
	_, err := parsePaymentImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parsePaymentImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parsePaymentImportRows("payment_type,payment_date,amount\n\"unterminated")
	require.ErrorContains(t, err, "parse csv row 2")

	_, err = parsePaymentImportRows("payment_type,payment_date\nRECEIVED,2026-03-01\n")
	require.ErrorContains(t, err, "missing required columns: amount")

	rows, err := parsePaymentImportRows("\ufefftype;date;amount;customer_name;description;legacy.column\n" +
		"received;2026-03-01;12.50;Acme OU;Imported payment;ignored\n" +
		";;;;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "received", rows[0].values["payment_type"])
	assert.Equal(t, "2026-03-01", rows[0].values["payment_date"])
	assert.Equal(t, "12.50", rows[0].values["amount"])
	assert.Equal(t, "Acme OU", rows[0].values["contact_name"])
	assert.Equal(t, "Imported payment", rows[0].values["notes"])
	assert.Equal(t, "ignored", rows[0].values["legacy_column"])

	rows, err = parsePaymentImportRows("type\tdate\tamount\nMADE\t2026-03-02\t15\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "MADE", rows[0].values["payment_type"])
	assert.Equal(t, "15", rows[0].values["amount"])
}

func TestBuildPaymentFromImportRowDefaultsAndEdges(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)
	lockDate := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	baseValues := map[string]string{
		"payment_type":   " made ",
		"payment_date":   "2026-03-15",
		"amount":         "100",
		"currency":       " usd ",
		"exchange_rate":  "1.2345",
		"payment_method": " bank ",
		"bank_account":   " EE123 ",
		"reference":      " REF-1 ",
		"notes":          " imported ",
	}

	row := paymentImportRow{rowNumber: 2, values: clonePaymentImportValues(baseValues)}
	payment, allocation, err := service.buildPaymentFromImportRow(
		context.Background(),
		"tenant-1",
		"test_schema",
		row,
		"user-1",
		nil,
		contactrefs.ContactLookup{},
	)

	require.NoError(t, err)
	require.Nil(t, allocation)
	assert.Equal(t, "tenant-1", payment.TenantID)
	assert.Equal(t, PaymentTypeMade, payment.PaymentType)
	assert.Equal(t, "USD", payment.Currency)
	assert.True(t, payment.ExchangeRate.Equal(decimal.RequireFromString("1.2345")))
	assert.True(t, payment.BaseAmount.Equal(decimal.RequireFromString("123.45")))
	assert.Equal(t, "bank", payment.PaymentMethod)
	assert.Equal(t, "EE123", payment.BankAccount)
	assert.Equal(t, "REF-1", payment.Reference)
	assert.Equal(t, "imported", payment.Notes)
	assert.Equal(t, "user-1", payment.CreatedBy)

	tests := []struct {
		name        string
		mutate      func(map[string]string)
		lockDate    *time.Time
		wantMessage string
	}{
		{
			name:        "invalid payment type",
			mutate:      func(values map[string]string) { values["payment_type"] = "REFUND" },
			wantMessage: "invalid payment_type",
		},
		{
			name:        "invalid payment date",
			mutate:      func(values map[string]string) { values["payment_date"] = "03/15/2026" },
			wantMessage: "payment_date must be YYYY-MM-DD or RFC3339",
		},
		{
			name:        "locked period",
			mutate:      func(values map[string]string) { values["payment_date"] = "2026-01-31" },
			lockDate:    &lockDate,
			wantMessage: "period locked through 2026-01-31",
		},
		{
			name:        "missing amount",
			mutate:      func(values map[string]string) { values["amount"] = "" },
			wantMessage: "amount is required",
		},
		{
			name:        "invalid exchange rate",
			mutate:      func(values map[string]string) { values["exchange_rate"] = "0" },
			wantMessage: "exchange_rate must be positive",
		},
		{
			name:        "unknown contact reference",
			mutate:      func(values map[string]string) { values["contact_code"] = "CUST-404" },
			wantMessage: `contact_code "CUST-404" was not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := clonePaymentImportValues(baseValues)
			tt.mutate(values)

			_, _, err := service.buildPaymentFromImportRow(
				context.Background(),
				"tenant-1",
				"test_schema",
				paymentImportRow{rowNumber: 2, values: values},
				"user-1",
				tt.lockDate,
				contactrefs.ContactLookup{},
			)

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}
}

func TestBuildPaymentImportAllocationEdges(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil)
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	paymentAmount := decimal.RequireFromString("100")
	invoiceID := "11111111-1111-4111-8111-111111111111"

	allocation, err := service.buildPaymentImportAllocation(
		context.Background(),
		"tenant-1",
		"test_schema",
		paymentImportRow{rowNumber: 2, values: map[string]string{}},
		"payment-1",
		paymentAmount,
		now,
	)
	require.NoError(t, err)
	require.Nil(t, allocation)

	tests := []struct {
		name        string
		values      map[string]string
		wantMessage string
	}{
		{
			name: "requires invoice when allocation amount is provided",
			values: map[string]string{
				"allocation_amount": "10",
			},
			wantMessage: "invoice_id or invoice_number is required when allocation_amount is provided",
		},
		{
			name: "rejects invalid allocation amount",
			values: map[string]string{
				"invoice_id":        invoiceID,
				"allocation_amount": "ten",
			},
			wantMessage: "allocation_amount must be a decimal",
		},
		{
			name: "rejects allocation above payment amount",
			values: map[string]string{
				"invoice_id":        invoiceID,
				"allocation_amount": "100.01",
			},
			wantMessage: "allocation_amount exceeds payment amount",
		},
		{
			name: "requires invoicing service for invoice numbers",
			values: map[string]string{
				"invoice_number":    "INV-1",
				"allocation_amount": "10",
			},
			wantMessage: `invoice_number "INV-1" cannot be resolved without invoicing service`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.buildPaymentImportAllocation(
				context.Background(),
				"tenant-1",
				"test_schema",
				paymentImportRow{rowNumber: 2, values: tt.values},
				"payment-1",
				paymentAmount,
				now,
			)

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}

	allocation, err = service.buildPaymentImportAllocation(
		context.Background(),
		"tenant-1",
		"test_schema",
		paymentImportRow{rowNumber: 2, values: map[string]string{"invoice_id": invoiceID}},
		"payment-1",
		paymentAmount,
		now,
	)
	require.NoError(t, err)
	require.NotNil(t, allocation)
	assert.Equal(t, invoiceID, allocation.InvoiceID)
	assert.True(t, allocation.Amount.Equal(paymentAmount))
	assert.Equal(t, now, allocation.CreatedAt)
}

func TestPaymentImportContactLookupEdges(t *testing.T) {
	ctx := context.Background()
	rowWithReference := paymentImportRow{rowNumber: 2, values: map[string]string{"contact_code": "CUST-1"}}

	service := NewServiceWithRepository(NewMockRepository(), nil)
	lookup, err := service.paymentImportContactLookup(ctx, "test_schema", "tenant-1", []paymentImportRow{
		{rowNumber: 2, values: map[string]string{"contact_code": ""}},
	})
	require.NoError(t, err)
	contactID, err := lookup.ResolveID("", contactrefs.Reference{Field: "contact_code", Value: "CUST-1"})
	require.ErrorContains(t, err, `contact_code "CUST-1" was not found`)
	require.Nil(t, contactID)

	_, err = service.paymentImportContactLookup(ctx, "test_schema", "tenant-1", []paymentImportRow{rowWithReference})
	require.ErrorContains(t, err, "contact service is required to resolve payment contact references")

	service.contacts = &fakePaymentContactLister{err: errors.New("contacts offline")}
	_, err = service.paymentImportContactLookup(ctx, "test_schema", "tenant-1", []paymentImportRow{rowWithReference})
	require.ErrorContains(t, err, "list contacts for payment import: contacts offline")

	resolvedID := "22222222-2222-4222-8222-222222222222"
	service.contacts = &fakePaymentContactLister{contacts: []contacts.Contact{{ID: resolvedID, Code: "CUST-1"}}}
	lookup, err = service.paymentImportContactLookup(ctx, "test_schema", "tenant-1", []paymentImportRow{rowWithReference})
	require.NoError(t, err)
	contactID, err = lookup.ResolveID("", contactrefs.Reference{Field: "contact_code", Value: "cust-1"})
	require.NoError(t, err)
	require.NotNil(t, contactID)
	assert.Equal(t, resolvedID, *contactID)
}

func TestPaymentImportParserHelpers(t *testing.T) {
	paymentType, err := parsePaymentImportType(" received ")
	require.NoError(t, err)
	assert.Equal(t, PaymentTypeReceived, paymentType)

	_, err = parsePaymentImportType("refund")
	require.ErrorContains(t, err, "invalid payment_type")

	paymentDate, err := parsePaymentImportDate("2026-03-01T13:45:00Z")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 3, 1, 13, 45, 0, 0, time.UTC), paymentDate)

	_, err = parsePaymentImportDate("")
	require.ErrorContains(t, err, "payment_date is required")

	_, err = parsePaymentImportDate("2026/03/01")
	require.ErrorContains(t, err, "payment_date must be YYYY-MM-DD or RFC3339")

	amount, err := parsePaymentImportPositiveDecimal("amount", " 12.50 ")
	require.NoError(t, err)
	assert.True(t, amount.Equal(decimal.RequireFromString("12.50")))

	_, err = parsePaymentImportPositiveDecimal("amount", "")
	require.ErrorContains(t, err, "amount is required")

	_, err = parsePaymentImportPositiveDecimal("amount", "twelve")
	require.ErrorContains(t, err, "amount must be a decimal")

	_, err = parsePaymentImportPositiveDecimal("amount", "0")
	require.ErrorContains(t, err, "amount must be positive")

	fallback := decimal.RequireFromString("1")
	value, err := parsePaymentImportOptionalPositiveDecimal("exchange_rate", "", fallback)
	require.NoError(t, err)
	assert.True(t, value.Equal(fallback))

	_, err = parsePaymentImportOptionalPositiveDecimal("exchange_rate", "-1", fallback)
	require.ErrorContains(t, err, "exchange_rate must be positive")

	assert.Equal(t, '\t', detectPaymentImportDelimiter("type\tdate\tamount\nRECEIVED\t2026-03-01\t10"))
	assert.Equal(t, ';', detectPaymentImportDelimiter("type;date;amount\nRECEIVED;2026-03-01;10"))
	assert.Equal(t, ',', detectPaymentImportDelimiter("type,date,amount\nRECEIVED,2026-03-01,10"))
	assert.Equal(t, "payment_method", canonicalPaymentImportHeader("Payment Method"))
	assert.Equal(t, "legacy_column", canonicalPaymentImportHeader("Legacy.Column"))
}

func TestAssignImportedPaymentNumberEdges(t *testing.T) {
	t.Run("returns repository number errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.getNextNumErr = errors.New("sequence unavailable")
		service := NewServiceWithRepository(repo, nil)
		payment := &Payment{PaymentType: PaymentTypeReceived}

		err := service.assignImportedPaymentNumber(context.Background(), "test_schema", "tenant-1", payment, map[string]string{})

		require.ErrorContains(t, err, "generate payment number: sequence unavailable")
		assert.Empty(t, payment.PaymentNumber)
	})

	t.Run("exhausts duplicate generated numbers", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)
		usedNumbers := make(map[string]string)
		for seq := 1; seq <= 100; seq++ {
			usedNumbers[normalizedPaymentImportKey(FormatPaymentNumber(PaymentTypeReceived, seq))] = "existing"
		}
		payment := &Payment{PaymentType: PaymentTypeReceived}

		err := service.assignImportedPaymentNumber(context.Background(), "test_schema", "tenant-1", payment, usedNumbers)

		require.ErrorContains(t, err, "generate payment number: exhausted duplicate attempts")
		assert.Empty(t, payment.PaymentNumber)
	})
}

func clonePaymentImportValues(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
