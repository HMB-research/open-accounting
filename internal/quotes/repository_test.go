package quotes

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
	schemaName := "tenant_test"
	tenantID := "tenant-1"
	quoteID := "quote-1"
	quote := &Quote{ID: quoteID, TenantID: tenantID}

	repositories := []struct {
		name string
		repo *GORMRepository
	}{
		{name: "nil receiver", repo: nil},
		{name: "nil gorm database", repo: NewGORMRepository(nil)},
		{name: "nil pool constructor", repo: NewRepository(nil)},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, repo *GORMRepository) error
	}{
		{
			name: "Create",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Create(ctx, schemaName, quote)
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.GetByID(ctx, schemaName, tenantID, quoteID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T, repo *GORMRepository) error {
				fromDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
				toDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
				got, err := repo.List(ctx, schemaName, tenantID, &QuoteFilter{
					Status:    QuoteStatusSent,
					ContactID: "contact-1",
					FromDate:  &fromDate,
					ToDate:    &toDate,
					Search:    " Q-00001 ",
				})
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Update(ctx, schemaName, quote)
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateStatus(ctx, schemaName, tenantID, quoteID, QuoteStatusAccepted)
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Delete(ctx, schemaName, tenantID, quoteID)
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.GenerateNumber(ctx, schemaName, tenantID)
				assert.Empty(t, got)
				return err
			},
		},
		{
			name: "SetConvertedToOrder",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.SetConvertedToOrder(ctx, schemaName, tenantID, quoteID, "order-1")
			},
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quoteID, "invoice-1")
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.tenantTable(ctx, schemaName, "quotes")
				assert.Nil(t, got)
				return err
			},
		},
	}

	for _, repository := range repositories {
		repository := repository
		t.Run(repository.name, func(t *testing.T) {
			for _, tt := range tests {
				tt := tt
				t.Run(tt.name, func(t *testing.T) {
					err := tt.run(t, repository.repo)
					require.Error(t, err)
					assert.Contains(t, err.Error(), "quotes repository database is not configured")
				})
			}
		})
	}
}

func TestQuoteModelMappingRoundTrip(t *testing.T) {
	quoteDate := time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC)
	validUntil := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.February, 2, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 3, 11, 45, 0, 0, time.UTC)
	orderID := "order-1"
	invoiceID := "invoice-1"
	quote := &Quote{
		ID:                   "quote-1",
		TenantID:             "tenant-1",
		QuoteNumber:          "Q-00042",
		ContactID:            "contact-1",
		QuoteDate:            quoteDate,
		ValidUntil:           &validUntil,
		Status:               QuoteStatusConverted,
		Currency:             "USD",
		ExchangeRate:         decimal.RequireFromString("1.2345000000"),
		Subtotal:             decimal.RequireFromString("100.25"),
		VATAmount:            decimal.RequireFromString("22.06"),
		Total:                decimal.RequireFromString("122.31"),
		Notes:                "priority customer",
		ConvertedToOrderID:   &orderID,
		ConvertedToInvoiceID: &invoiceID,
		CreatedAt:            createdAt,
		CreatedBy:            "user-1",
		UpdatedAt:            updatedAt,
	}

	model := quoteToModel(quote)

	assert.Equal(t, string(quote.Status), model.Status)
	assert.True(t, model.ExchangeRate.Decimal.Equal(quote.ExchangeRate))
	assert.True(t, model.Subtotal.Decimal.Equal(quote.Subtotal))
	assert.True(t, model.VATAmount.Decimal.Equal(quote.VATAmount))
	assert.True(t, model.Total.Decimal.Equal(quote.Total))
	assert.Equal(t, quote, quoteFromModel(model))
}

func TestQuoteModelMappingPreservesNilOptionalStrings(t *testing.T) {
	quoteDate := time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.February, 2, 10, 30, 0, 0, time.UTC)
	quote := &Quote{
		ID:           "quote-1",
		TenantID:     "tenant-1",
		QuoteNumber:  "Q-00042",
		ContactID:    "contact-1",
		QuoteDate:    quoteDate,
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.RequireFromString("100.00"),
		VATAmount:    decimal.RequireFromString("22.00"),
		Total:        decimal.RequireFromString("122.00"),
		CreatedAt:    createdAt,
		CreatedBy:    "user-1",
		UpdatedAt:    createdAt,
	}

	model := quoteToModel(quote)

	assert.Nil(t, model.ValidUntil)
	assert.Nil(t, model.ConvertedToOrderID)
	assert.Nil(t, model.ConvertedToInvoiceID)
	assert.Equal(t, quote, quoteFromModel(model))
}

func TestQuoteLineModelMappingRoundTrip(t *testing.T) {
	productID := "product-1"
	line := &QuoteLine{
		ID:              "line-1",
		TenantID:        "tenant-1",
		QuoteID:         "quote-1",
		LineNumber:      2,
		Description:     "Configured hardware",
		Quantity:        decimal.RequireFromString("3.500"),
		Unit:            "pcs",
		UnitPrice:       decimal.RequireFromString("19.9900"),
		DiscountPercent: decimal.RequireFromString("5.50"),
		VATRate:         decimal.RequireFromString("22.00"),
		LineSubtotal:    decimal.RequireFromString("66.12"),
		LineVAT:         decimal.RequireFromString("14.55"),
		LineTotal:       decimal.RequireFromString("80.67"),
		ProductID:       &productID,
	}

	model := quoteLineToModel(line)

	assert.True(t, model.Quantity.Decimal.Equal(line.Quantity))
	assert.True(t, model.UnitPrice.Decimal.Equal(line.UnitPrice))
	assert.True(t, model.DiscountPercent.Decimal.Equal(line.DiscountPercent))
	assert.True(t, model.VATRate.Decimal.Equal(line.VATRate))
	assert.True(t, model.LineSubtotal.Decimal.Equal(line.LineSubtotal))
	assert.True(t, model.LineVAT.Decimal.Equal(line.LineVAT))
	assert.True(t, model.LineTotal.Decimal.Equal(line.LineTotal))
	assert.Equal(t, line, quoteLineFromModel(model))
}

func TestQuoteLineModelMappingPreservesNilOptionalStrings(t *testing.T) {
	line := &QuoteLine{
		ID:              "line-1",
		TenantID:        "tenant-1",
		QuoteID:         "quote-1",
		LineNumber:      1,
		Description:     "Consulting",
		Quantity:        decimal.NewFromInt(1),
		Unit:            "hour",
		UnitPrice:       decimal.RequireFromString("100.00"),
		DiscountPercent: decimal.Zero,
		VATRate:         decimal.RequireFromString("22.00"),
		LineSubtotal:    decimal.RequireFromString("100.00"),
		LineVAT:         decimal.RequireFromString("22.00"),
		LineTotal:       decimal.RequireFromString("122.00"),
	}

	model := quoteLineToModel(line)

	assert.Nil(t, model.ProductID)
	assert.Equal(t, line, quoteLineFromModel(model))
}

func TestQuoteFromModelMapsStatusAndDecimals(t *testing.T) {
	validUntil := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC)
	model := &models.Quote{
		ID:           "quote-1",
		TenantID:     "tenant-1",
		QuoteNumber:  "Q-00042",
		ContactID:    "contact-1",
		QuoteDate:    time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC),
		ValidUntil:   &validUntil,
		Status:       string(QuoteStatusSent),
		Currency:     "EUR",
		ExchangeRate: models.Decimal{Decimal: decimal.RequireFromString("1.0000000000")},
		Subtotal:     models.Decimal{Decimal: decimal.RequireFromString("100.00")},
		VATAmount:    models.Decimal{Decimal: decimal.RequireFromString("22.00")},
		Total:        models.Decimal{Decimal: decimal.RequireFromString("122.00")},
		Notes:        "send soon",
		CreatedAt:    time.Date(2026, time.February, 2, 10, 30, 0, 0, time.UTC),
		CreatedBy:    "user-1",
		UpdatedAt:    time.Date(2026, time.February, 3, 11, 45, 0, 0, time.UTC),
	}

	quote := quoteFromModel(model)

	assert.Equal(t, QuoteStatusSent, quote.Status)
	assert.True(t, quote.ExchangeRate.Equal(model.ExchangeRate.Decimal))
	assert.True(t, quote.Subtotal.Equal(model.Subtotal.Decimal))
	assert.True(t, quote.VATAmount.Equal(model.VATAmount.Decimal))
	assert.True(t, quote.Total.Equal(model.Total.Decimal))
}
