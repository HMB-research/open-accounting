package payments

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryTenantTableRequiresConfiguredDatabase(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		repo *GORMRepository
	}{
		{name: "nil receiver"},
		{name: "nil database", repo: NewGORMRepository(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := tt.repo.tenantTable(ctx, "tenant_test", "payments")

			require.ErrorIs(t, err, errRepositoryDatabaseNotConfigured)
			assert.EqualError(t, err, "payments repository database is not configured")
			assert.Nil(t, db)
		})
	}
}

func TestGORMRepositoryMethodsRequireConfiguredDatabase(t *testing.T) {
	ctx := context.Background()
	reversedAt := time.Date(2026, 4, 5, 12, 30, 0, 0, time.UTC)
	payment := &Payment{
		ID:            "payment-1",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-00001",
		PaymentType:   PaymentTypeReceived,
		Amount:        decimal.RequireFromString("125.50"),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.RequireFromString("125.50"),
		PaymentDate:   reversedAt,
		CreatedAt:     reversedAt,
		CreatedBy:     "user-1",
	}
	allocation := &PaymentAllocation{
		ID:        "allocation-1",
		TenantID:  "tenant-1",
		PaymentID: "payment-1",
		InvoiceID: "invoice-1",
		Amount:    decimal.RequireFromString("25.50"),
		CreatedAt: reversedAt,
	}

	methods := []struct {
		name string
		call func(*GORMRepository) error
	}{
		{
			name: "Create",
			call: func(repo *GORMRepository) error {
				return repo.Create(ctx, "tenant_test", payment)
			},
		},
		{
			name: "CreateReversal",
			call: func(repo *GORMRepository) error {
				return repo.CreateReversal(ctx, "tenant_test", "original-payment-1", payment, []PaymentAllocation{*allocation}, reversedAt, "user-1", "Duplicate import")
			},
		},
		{
			name: "GetByID",
			call: func(repo *GORMRepository) error {
				_, err := repo.GetByID(ctx, "tenant_test", "tenant-1", "payment-1")
				return err
			},
		},
		{
			name: "List",
			call: func(repo *GORMRepository) error {
				_, err := repo.List(ctx, "tenant_test", "tenant-1", &PaymentFilter{PaymentType: PaymentTypeReceived})
				return err
			},
		},
		{
			name: "CreateAllocation",
			call: func(repo *GORMRepository) error {
				return repo.CreateAllocation(ctx, "tenant_test", allocation)
			},
		},
		{
			name: "GetAllocations",
			call: func(repo *GORMRepository) error {
				_, err := repo.GetAllocations(ctx, "tenant_test", "tenant-1", "payment-1")
				return err
			},
		},
		{
			name: "GetNextPaymentNumber",
			call: func(repo *GORMRepository) error {
				_, err := repo.GetNextPaymentNumber(ctx, "tenant_test", "tenant-1", PaymentTypeReceived)
				return err
			},
		},
		{
			name: "GetUnallocatedPayments",
			call: func(repo *GORMRepository) error {
				_, err := repo.GetUnallocatedPayments(ctx, "tenant_test", "tenant-1", PaymentTypeReceived)
				return err
			},
		},
	}

	repos := []struct {
		name string
		repo *GORMRepository
	}{
		{name: "nil receiver"},
		{name: "nil database", repo: NewGORMRepository(nil)},
	}

	for _, repoCase := range repos {
		t.Run(repoCase.name, func(t *testing.T) {
			for _, method := range methods {
				t.Run(method.name, func(t *testing.T) {
					err := method.call(repoCase.repo)

					require.ErrorIs(t, err, errRepositoryDatabaseNotConfigured)
					assert.EqualError(t, err, "payments repository database is not configured")
				})
			}
		})
	}
}

func TestPaymentModelMappingPreservesFields(t *testing.T) {
	contactID := "contact-1"
	journalEntryID := "journal-entry-1"
	reversalOfPaymentID := "payment-original"
	reversedByPaymentID := "payment-reversal"
	reversedAt := time.Date(2026, 4, 6, 9, 45, 0, 0, time.UTC)
	createdAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	paymentDate := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	reversedBy := "user-2"
	model := &models.Payment{
		ID:                  "payment-1",
		TenantID:            "tenant-1",
		PaymentNumber:       "OUT-00017",
		PaymentType:         models.PaymentTypeMade,
		ContactID:           &contactID,
		PaymentDate:         paymentDate,
		Amount:              models.Decimal{Decimal: decimal.RequireFromString("250.75")},
		Currency:            "USD",
		ExchangeRate:        models.Decimal{Decimal: decimal.RequireFromString("0.9234")},
		BaseAmount:          models.Decimal{Decimal: decimal.RequireFromString("231.64")},
		PaymentMethod:       "WIRE",
		BankAccount:         "EE123",
		Reference:           "REF-17",
		Notes:               "Imported settlement",
		JournalEntryID:      &journalEntryID,
		ReversalOfPaymentID: &reversalOfPaymentID,
		ReversedByPaymentID: &reversedByPaymentID,
		ReversedAt:          &reversedAt,
		ReversedBy:          &reversedBy,
		ReversalReason:      "Duplicate bank import",
		CreatedAt:           createdAt,
		CreatedBy:           "user-1",
	}

	payment := modelToPayment(model)

	require.NotNil(t, payment)
	assert.Equal(t, model.ID, payment.ID)
	assert.Equal(t, model.TenantID, payment.TenantID)
	assert.Equal(t, model.PaymentNumber, payment.PaymentNumber)
	assert.Equal(t, PaymentTypeMade, payment.PaymentType)
	assert.Equal(t, contactID, *payment.ContactID)
	assert.Equal(t, model.PaymentDate, payment.PaymentDate)
	assert.True(t, payment.Amount.Equal(model.Amount.Decimal))
	assert.Equal(t, model.Currency, payment.Currency)
	assert.True(t, payment.ExchangeRate.Equal(model.ExchangeRate.Decimal))
	assert.True(t, payment.BaseAmount.Equal(model.BaseAmount.Decimal))
	assert.Equal(t, model.PaymentMethod, payment.PaymentMethod)
	assert.Equal(t, model.BankAccount, payment.BankAccount)
	assert.Equal(t, model.Reference, payment.Reference)
	assert.Equal(t, model.Notes, payment.Notes)
	assert.Equal(t, journalEntryID, *payment.JournalEntryID)
	assert.Equal(t, reversalOfPaymentID, *payment.ReversalOfPaymentID)
	assert.Equal(t, reversedByPaymentID, *payment.ReversedByPaymentID)
	assert.Equal(t, reversedAt, *payment.ReversedAt)
	assert.Equal(t, reversedBy, *payment.ReversedBy)
	assert.Equal(t, model.ReversalReason, payment.ReversalReason)
	assert.Equal(t, model.CreatedAt, payment.CreatedAt)
	assert.Equal(t, model.CreatedBy, payment.CreatedBy)

	roundTripModel := paymentToModel(payment)

	require.NotNil(t, roundTripModel)
	assert.Equal(t, model.ID, roundTripModel.ID)
	assert.Equal(t, model.TenantID, roundTripModel.TenantID)
	assert.Equal(t, model.PaymentNumber, roundTripModel.PaymentNumber)
	assert.Equal(t, models.PaymentTypeMade, roundTripModel.PaymentType)
	assert.Equal(t, contactID, *roundTripModel.ContactID)
	assert.Equal(t, model.PaymentDate, roundTripModel.PaymentDate)
	assert.True(t, roundTripModel.Amount.Decimal.Equal(model.Amount.Decimal))
	assert.Equal(t, model.Currency, roundTripModel.Currency)
	assert.True(t, roundTripModel.ExchangeRate.Decimal.Equal(model.ExchangeRate.Decimal))
	assert.True(t, roundTripModel.BaseAmount.Decimal.Equal(model.BaseAmount.Decimal))
	assert.Equal(t, model.PaymentMethod, roundTripModel.PaymentMethod)
	assert.Equal(t, model.BankAccount, roundTripModel.BankAccount)
	assert.Equal(t, model.Reference, roundTripModel.Reference)
	assert.Equal(t, model.Notes, roundTripModel.Notes)
	assert.Equal(t, journalEntryID, *roundTripModel.JournalEntryID)
	assert.Equal(t, reversalOfPaymentID, *roundTripModel.ReversalOfPaymentID)
	assert.Equal(t, reversedByPaymentID, *roundTripModel.ReversedByPaymentID)
	assert.Equal(t, reversedAt, *roundTripModel.ReversedAt)
	assert.Equal(t, reversedBy, *roundTripModel.ReversedBy)
	assert.Equal(t, model.ReversalReason, roundTripModel.ReversalReason)
	assert.Equal(t, model.CreatedAt, roundTripModel.CreatedAt)
	assert.Equal(t, model.CreatedBy, roundTripModel.CreatedBy)
}

func TestPaymentAllocationModelMappingPreservesFields(t *testing.T) {
	createdAt := time.Date(2026, 4, 7, 8, 15, 0, 0, time.UTC)
	model := &models.PaymentAllocation{
		ID:        "allocation-1",
		TenantID:  "tenant-1",
		PaymentID: "payment-1",
		InvoiceID: "invoice-1",
		Amount:    models.Decimal{Decimal: decimal.RequireFromString("99.95")},
		CreatedAt: createdAt,
	}

	allocation := modelToAllocation(model)

	require.NotNil(t, allocation)
	assert.Equal(t, model.ID, allocation.ID)
	assert.Equal(t, model.TenantID, allocation.TenantID)
	assert.Equal(t, model.PaymentID, allocation.PaymentID)
	assert.Equal(t, model.InvoiceID, allocation.InvoiceID)
	assert.True(t, allocation.Amount.Equal(model.Amount.Decimal))
	assert.Equal(t, model.CreatedAt, allocation.CreatedAt)

	roundTripModel := allocationToModel(allocation)

	require.NotNil(t, roundTripModel)
	assert.Equal(t, model.ID, roundTripModel.ID)
	assert.Equal(t, model.TenantID, roundTripModel.TenantID)
	assert.Equal(t, model.PaymentID, roundTripModel.PaymentID)
	assert.Equal(t, model.InvoiceID, roundTripModel.InvoiceID)
	assert.True(t, roundTripModel.Amount.Decimal.Equal(model.Amount.Decimal))
	assert.Equal(t, model.CreatedAt, roundTripModel.CreatedAt)
}

func TestPaymentNumberHelpersWithoutDatabase(t *testing.T) {
	assert.Equal(t, "PMT", PaymentNumberPrefix(PaymentTypeReceived))
	assert.Equal(t, "OUT", PaymentNumberPrefix(PaymentTypeMade))
	assert.Equal(t, "PMT", PaymentNumberPrefix(PaymentType("UNKNOWN")))
	assert.Equal(t, "PMT-00003", FormatPaymentNumber(PaymentTypeReceived, 3))
	assert.Equal(t, "OUT-00012", FormatPaymentNumber(PaymentTypeMade, 12))

	sequenceTests := []struct {
		name           string
		paymentNumbers []string
		paymentType    PaymentType
		want           int
	}{
		{
			name:           "starts at one when empty",
			paymentNumbers: nil,
			paymentType:    PaymentTypeReceived,
			want:           1,
		},
		{
			name: "increments highest matching received sequence",
			paymentNumbers: []string{
				"OUT-00099",
				"PMT-00002",
				"PMT-00001",
				"PMT-",
				" PMT-00004-adjusted ",
			},
			paymentType: PaymentTypeReceived,
			want:        5,
		},
		{
			name: "increments highest matching outgoing sequence",
			paymentNumbers: []string{
				"PMT-00099",
				"OUT-00007",
				"OUT-00008-reversal",
			},
			paymentType: PaymentTypeMade,
			want:        9,
		},
	}

	for _, tt := range sequenceTests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NextPaymentNumberSequence(tt.paymentNumbers, tt.paymentType))
		})
	}

	parseTests := []struct {
		name          string
		paymentNumber string
		prefix        string
		want          int
		wantOK        bool
	}{
		{name: "trims spaces", paymentNumber: " PMT-00008 ", prefix: "PMT", want: 8, wantOK: true},
		{name: "stops before suffix", paymentNumber: "OUT-00009-reversal", prefix: "OUT", want: 9, wantOK: true},
		{name: "rejects wrong prefix", paymentNumber: "OUT-00009", prefix: "PMT", wantOK: false},
		{name: "rejects empty sequence", paymentNumber: "PMT-", prefix: "PMT", wantOK: false},
		{name: "rejects missing digits before suffix", paymentNumber: "PMT-adjusted", prefix: "PMT", wantOK: false},
		{name: "rejects overflow", paymentNumber: "PMT-999999999999999999999999999999999999", prefix: "PMT", wantOK: false},
	}

	for _, tt := range parseTests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := paymentNumberSequence(tt.paymentNumber, tt.prefix)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
