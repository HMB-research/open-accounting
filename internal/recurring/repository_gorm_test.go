package recurring

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	recurringInvoiceID := "recurring-invoice-1"
	invoiceID := "invoice-1"
	asOfDate := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	nextDate := time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)
	generatedAt := time.Date(2026, time.March, 15, 9, 30, 0, 0, time.UTC)
	sentAt := time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC)

	repositories := []struct {
		name string
		repo *GORMRepository
	}{
		{name: "nil receiver", repo: nil},
		{name: "nil database", repo: NewGORMRepository(nil)},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, repo *GORMRepository) error
	}{
		{
			name: "Create",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Create(ctx, schemaName, &RecurringInvoice{
					ID:       recurringInvoiceID,
					TenantID: tenantID,
				})
			},
		},
		{
			name: "CreateLine",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.CreateLine(ctx, schemaName, &RecurringInvoiceLine{
					ID:                 "line-1",
					RecurringInvoiceID: recurringInvoiceID,
				})
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T, repo *GORMRepository) error {
				invoice, err := repo.GetByID(ctx, schemaName, tenantID, recurringInvoiceID)
				assert.Nil(t, invoice)
				return err
			},
		},
		{
			name: "GetLines",
			run: func(t *testing.T, repo *GORMRepository) error {
				lines, err := repo.GetLines(ctx, schemaName, recurringInvoiceID)
				assert.Nil(t, lines)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T, repo *GORMRepository) error {
				invoices, err := repo.List(ctx, schemaName, tenantID, true)
				assert.Nil(t, invoices)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Update(ctx, schemaName, &RecurringInvoice{
					ID:       recurringInvoiceID,
					TenantID: tenantID,
				})
			},
		},
		{
			name: "DeleteLines",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.DeleteLines(ctx, schemaName, recurringInvoiceID)
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Delete(ctx, schemaName, tenantID, recurringInvoiceID)
			},
		},
		{
			name: "SetActive",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.SetActive(ctx, schemaName, tenantID, recurringInvoiceID, false)
			},
		},
		{
			name: "GetDueRecurringInvoiceIDs",
			run: func(t *testing.T, repo *GORMRepository) error {
				ids, err := repo.GetDueRecurringInvoiceIDs(ctx, schemaName, tenantID, asOfDate)
				assert.Nil(t, ids)
				return err
			},
		},
		{
			name: "UpdateAfterGeneration",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateAfterGeneration(ctx, schemaName, tenantID, recurringInvoiceID, nextDate, generatedAt)
			},
		},
		{
			name: "UpdateInvoiceEmailStatus",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateInvoiceEmailStatus(ctx, schemaName, invoiceID, &sentAt, "SENT", "email-log-1")
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T, repo *GORMRepository) error {
				table, err := repo.tenantTable(ctx, schemaName, "recurring_invoices")
				assert.Nil(t, table)
				return err
			},
		},
	}

	for _, repository := range repositories {
		t.Run(repository.name, func(t *testing.T) {
			if repository.name == "nil database" {
				require.NotNil(t, repository.repo)
				assert.Nil(t, repository.repo.db)
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := tt.run(t, repository.repo)
					require.ErrorContains(t, err, "recurring repository database is not configured")
				})
			}
		})
	}
}

func TestRecurringInvoiceModelMappingRoundTrip(t *testing.T) {
	endDate := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	lastGeneratedAt := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)
	startDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	nextGenerationDate := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.January, 1, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 1, 9, 45, 0, 0, time.UTC)

	tests := []struct {
		name    string
		invoice *RecurringInvoice
	}{
		{
			name: "active invoice with email configuration",
			invoice: &RecurringInvoice{
				ID:                     "recurring-invoice-1",
				TenantID:               "tenant-1",
				Name:                   "Monthly services",
				ContactID:              "contact-1",
				InvoiceType:            "SALES",
				Currency:               "EUR",
				Frequency:              FrequencyMonthly,
				StartDate:              startDate,
				EndDate:                &endDate,
				NextGenerationDate:     nextGenerationDate,
				PaymentTermsDays:       14,
				Reference:              "PO-123",
				Notes:                  "Bill on month end",
				IsActive:               true,
				LastGeneratedAt:        &lastGeneratedAt,
				GeneratedCount:         3,
				CreatedAt:              createdAt,
				CreatedBy:              "user-1",
				UpdatedAt:              updatedAt,
				SendEmailOnGeneration:  true,
				EmailTemplateType:      "INVOICE_SEND",
				RecipientEmailOverride: "billing@example.com",
				AttachPDFToEmail:       false,
				EmailSubjectOverride:   "Your recurring invoice",
				EmailMessage:           "Thanks for your business.",
			},
		},
		{
			name: "inactive invoice with disabled email fields",
			invoice: &RecurringInvoice{
				ID:                 "recurring-invoice-2",
				TenantID:           "tenant-1",
				Name:               "Paused yearly license",
				ContactID:          "contact-2",
				InvoiceType:        "SALES",
				Currency:           "USD",
				Frequency:          FrequencyYearly,
				StartDate:          startDate,
				NextGenerationDate: nextGenerationDate,
				PaymentTermsDays:   30,
				IsActive:           false,
				GeneratedCount:     0,
				CreatedAt:          createdAt,
				CreatedBy:          "user-2",
				UpdatedAt:          updatedAt,
				AttachPDFToEmail:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := recurringInvoiceToModel(tt.invoice)

			assert.Equal(t, models.Frequency(tt.invoice.Frequency), model.Frequency)
			assert.Equal(t, tt.invoice.IsActive, model.IsActive)
			assert.Equal(t, tt.invoice.LastGeneratedAt, model.LastGeneratedAt)
			assert.Equal(t, tt.invoice.GeneratedCount, model.GeneratedCount)
			assert.Equal(t, tt.invoice.SendEmailOnGeneration, model.SendEmailOnGeneration)
			assert.Equal(t, tt.invoice.EmailTemplateType, model.EmailTemplateType)
			assert.Equal(t, tt.invoice.RecipientEmailOverride, model.RecipientEmailOverride)
			assert.Equal(t, tt.invoice.AttachPDFToEmail, model.AttachPDFToEmail)
			assert.Equal(t, tt.invoice.EmailSubjectOverride, model.EmailSubjectOverride)
			assert.Equal(t, tt.invoice.EmailMessage, model.EmailMessage)

			roundTrip := modelToRecurringInvoice(model)
			assert.Equal(t, tt.invoice, roundTrip)
		})
	}
}

func TestRecurringInvoiceLineModelMappingRoundTrip(t *testing.T) {
	accountID := "account-1"
	productID := "product-1"

	tests := []struct {
		name string
		line *RecurringInvoiceLine
	}{
		{
			name: "line with accounting and product references",
			line: &RecurringInvoiceLine{
				ID:                 "line-1",
				RecurringInvoiceID: "recurring-invoice-1",
				LineNumber:         2,
				Description:        "Implementation",
				Quantity:           decimal.RequireFromString("3.500000"),
				Unit:               "hour",
				UnitPrice:          decimal.RequireFromString("95.12345678"),
				DiscountPercent:    decimal.RequireFromString("10.25"),
				VATRate:            decimal.RequireFromString("22.00"),
				AccountID:          &accountID,
				ProductID:          &productID,
			},
		},
		{
			name: "line without optional references",
			line: &RecurringInvoiceLine{
				ID:                 "line-2",
				RecurringInvoiceID: "recurring-invoice-2",
				LineNumber:         1,
				Description:        "Subscription",
				Quantity:           decimal.NewFromInt(1),
				UnitPrice:          decimal.RequireFromString("49.99"),
				DiscountPercent:    decimal.Zero,
				VATRate:            decimal.Zero,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := recurringInvoiceLineToModel(tt.line)

			assert.True(t, model.Quantity.Decimal.Equal(tt.line.Quantity))
			assert.True(t, model.UnitPrice.Decimal.Equal(tt.line.UnitPrice))
			assert.True(t, model.DiscountPercent.Decimal.Equal(tt.line.DiscountPercent))
			assert.True(t, model.VATRate.Decimal.Equal(tt.line.VATRate))
			assert.Equal(t, tt.line.AccountID, model.AccountID)
			assert.Equal(t, tt.line.ProductID, model.ProductID)

			roundTrip := modelToRecurringInvoiceLine(model)
			assert.Equal(t, tt.line, roundTrip)
		})
	}
}
