package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	fromDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, schemaName, &Invoice{
					TenantID: tenantID,
					Lines: []InvoiceLine{
						{TenantID: tenantID, Description: "Consulting"},
					},
				})
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				invoice, err := repo.GetByID(ctx, schemaName, tenantID, "invoice-1")
				assert.Nil(t, invoice)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				invoices, err := repo.List(ctx, schemaName, tenantID, &InvoiceFilter{
					InvoiceType: InvoiceTypeSales,
					Status:      StatusSent,
					ContactID:   "contact-1",
					FromDate:    &fromDate,
					ToDate:      &toDate,
					Search:      "INV-001",
				})
				assert.Nil(t, invoices)
				return err
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				return repo.UpdateStatus(ctx, schemaName, tenantID, "invoice-1", StatusSent)
			},
		},
		{
			name: "UpdatePayment",
			run: func(t *testing.T) error {
				return repo.UpdatePayment(ctx, schemaName, tenantID, "invoice-1", decimal.RequireFromString("42.50"), StatusPartiallyPaid)
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T) error {
				number, err := repo.GenerateNumber(ctx, schemaName, tenantID, InvoiceTypePurchase)
				assert.Empty(t, number)
				return err
			},
		},
		{
			name: "UpdateOverdueStatus",
			run: func(t *testing.T) error {
				count, err := repo.UpdateOverdueStatus(ctx, schemaName, tenantID)
				assert.Zero(t, count)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "invoices")
				assert.Nil(t, table)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invoicing repository database is not configured")
		})
	}
}

func TestInvoiceModelMappingRoundTrip(t *testing.T) {
	journalEntryID := "journal-entry-id"
	eInvoiceID := "einvoice-id"
	eInvoiceSentAt := time.Date(2026, time.February, 6, 11, 20, 0, 0, time.UTC)
	issueDate := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.February, 1, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 7, 9, 45, 0, 0, time.UTC)
	invoice := &Invoice{
		ID:             "invoice-id",
		TenantID:       "tenant-id",
		InvoiceNumber:  "INV-00042",
		InvoiceType:    InvoiceTypeSales,
		ContactID:      "contact-id",
		IssueDate:      issueDate,
		DueDate:        dueDate,
		Currency:       "EUR",
		ExchangeRate:   decimal.RequireFromString("1.0000000000"),
		Subtotal:       decimal.RequireFromString("100.00"),
		VATAmount:      decimal.RequireFromString("22.00"),
		Total:          decimal.RequireFromString("122.00"),
		BaseSubtotal:   decimal.RequireFromString("100.00"),
		BaseVATAmount:  decimal.RequireFromString("22.00"),
		BaseTotal:      decimal.RequireFromString("122.00"),
		AmountPaid:     decimal.RequireFromString("40.00"),
		Status:         StatusPartiallyPaid,
		Reference:      "PO-123",
		Notes:          "Payment in two parts",
		JournalEntryID: &journalEntryID,
		EInvoiceSentAt: &eInvoiceSentAt,
		EInvoiceID:     &eInvoiceID,
		CreatedAt:      createdAt,
		CreatedBy:      "user-id",
		UpdatedAt:      updatedAt,
	}

	model := invoiceToModel(invoice)
	assert.Equal(t, string(invoice.InvoiceType), string(model.InvoiceType))
	assert.True(t, model.Total.Decimal.Equal(invoice.Total))
	assert.Equal(t, invoice.JournalEntryID, model.JournalEntryID)

	roundTrip := modelToInvoice(model)
	assert.Equal(t, invoice, roundTrip)
}

func TestInvoiceLineModelMappingRoundTrip(t *testing.T) {
	accountID := "account-id"
	productID := "product-id"
	line := &InvoiceLine{
		ID:              "line-id",
		TenantID:        "tenant-id",
		InvoiceID:       "invoice-id",
		LineNumber:      2,
		Description:     "Implementation",
		Quantity:        decimal.RequireFromString("3.50"),
		Unit:            "hour",
		UnitPrice:       decimal.RequireFromString("95.00"),
		DiscountPercent: decimal.RequireFromString("10.00"),
		VATRate:         decimal.RequireFromString("22.00"),
		VATTreatment:    VATTreatmentReverseCharge,
		LineSubtotal:    decimal.RequireFromString("299.25"),
		LineVAT:         decimal.Zero,
		LineTotal:       decimal.RequireFromString("299.25"),
		AccountID:       &accountID,
		ProductID:       &productID,
	}

	model := invoiceLineToModel(line)
	assert.Equal(t, string(VATTreatmentReverseCharge), model.VATTreatment)
	assert.True(t, model.Quantity.Decimal.Equal(line.Quantity))

	roundTrip := modelToInvoiceLine(model)
	assert.Equal(t, line, roundTrip)
}

func TestInvoiceLineModelMappingDefaultsVATTreatment(t *testing.T) {
	line := &InvoiceLine{
		ID:          "line-id",
		TenantID:    "tenant-id",
		InvoiceID:   "invoice-id",
		LineNumber:  1,
		Description: "Support",
		Quantity:    decimal.NewFromInt(1),
		UnitPrice:   decimal.RequireFromString("50.00"),
		LineTotal:   decimal.RequireFromString("50.00"),
	}

	model := invoiceLineToModel(line)
	assert.Equal(t, string(VATTreatmentStandard), model.VATTreatment)

	roundTrip := modelToInvoiceLine(model)
	assert.Equal(t, VATTreatmentStandard, roundTrip.VATTreatment)
}
