package orders

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
	orderID := "order-1"
	productID := "product-1"
	warehouseID := "warehouse-1"
	order := &Order{ID: orderID, TenantID: tenantID}
	reservation := &OrderStockReservation{
		ID:          "reservation-1",
		TenantID:    tenantID,
		OrderID:     orderID,
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    decimal.NewFromInt(1),
	}

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
				return repo.Create(ctx, schemaName, order)
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.GetByID(ctx, schemaName, tenantID, orderID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.List(ctx, schemaName, tenantID, &OrderFilter{
					Status:    OrderStatusConfirmed,
					ContactID: "contact-1",
					Search:    " order ",
				})
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Update(ctx, schemaName, order)
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusShipped)
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.Delete(ctx, schemaName, tenantID, orderID)
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
			name: "SetConvertedToInvoice",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.SetConvertedToInvoice(ctx, schemaName, tenantID, orderID, "invoice-1")
			},
		},
		{
			name: "ListStockReservations",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.ListStockReservations(ctx, schemaName, tenantID, orderID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "GetStockReservation",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.GetStockReservation(ctx, schemaName, tenantID, orderID, productID, warehouseID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpsertStockReservation",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpsertStockReservation(ctx, schemaName, reservation)
			},
		},
		{
			name: "ReleaseStockReservation",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.ReleaseStockReservation(ctx, schemaName, tenantID, orderID, productID, warehouseID, decimal.NewFromInt(1), "release", "user-1")
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T, repo *GORMRepository) error {
				got, err := repo.tenantTable(ctx, schemaName, "orders")
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
					assert.Contains(t, err.Error(), "orders repository database is not configured")
				})
			}
		})
	}
}

func TestOrderModelMappingRoundTrip(t *testing.T) {
	orderDate := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	expectedDelivery := time.Date(2026, time.January, 16, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.January, 3, 11, 45, 0, 0, time.UTC)
	quoteID := "quote-1"
	invoiceID := "invoice-1"
	order := &Order{
		ID:                   "order-1",
		TenantID:             "tenant-1",
		OrderNumber:          "ORD-00042",
		ContactID:            "contact-1",
		OrderDate:            orderDate,
		ExpectedDelivery:     &expectedDelivery,
		Status:               OrderStatusDelivered,
		Currency:             "USD",
		ExchangeRate:         decimal.RequireFromString("1.2345000000"),
		Subtotal:             decimal.RequireFromString("100.25"),
		VATAmount:            decimal.RequireFromString("22.06"),
		Total:                decimal.RequireFromString("122.31"),
		Notes:                "priority customer",
		QuoteID:              &quoteID,
		ConvertedToInvoiceID: &invoiceID,
		CreatedAt:            createdAt,
		CreatedBy:            "user-1",
		UpdatedAt:            updatedAt,
	}

	model := orderToModel(order)

	assert.Equal(t, string(order.Status), model.Status)
	assert.True(t, model.ExchangeRate.Decimal.Equal(order.ExchangeRate))
	assert.True(t, model.Subtotal.Decimal.Equal(order.Subtotal))
	assert.True(t, model.VATAmount.Decimal.Equal(order.VATAmount))
	assert.True(t, model.Total.Decimal.Equal(order.Total))
	assert.Equal(t, order, orderFromModel(model))
}

func TestOrderLineModelMappingRoundTrip(t *testing.T) {
	productID := "product-1"
	line := &OrderLine{
		ID:              "line-1",
		TenantID:        "tenant-1",
		OrderID:         "order-1",
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

	model := orderLineToModel(line)

	assert.True(t, model.Quantity.Decimal.Equal(line.Quantity))
	assert.True(t, model.UnitPrice.Decimal.Equal(line.UnitPrice))
	assert.True(t, model.DiscountPercent.Decimal.Equal(line.DiscountPercent))
	assert.True(t, model.VATRate.Decimal.Equal(line.VATRate))
	assert.True(t, model.LineSubtotal.Decimal.Equal(line.LineSubtotal))
	assert.True(t, model.LineVAT.Decimal.Equal(line.LineVAT))
	assert.True(t, model.LineTotal.Decimal.Equal(line.LineTotal))
	assert.Equal(t, line, orderLineFromModel(model))
}

func TestStockReservationModelMappingHelpers(t *testing.T) {
	createdAt := time.Date(2026, time.February, 4, 9, 15, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 5, 10, 20, 0, 0, time.UTC)
	releasedAt := time.Date(2026, time.February, 6, 11, 25, 0, 0, time.UTC)
	reservation := &OrderStockReservation{
		ID:          "reservation-1",
		TenantID:    "tenant-1",
		OrderID:     "order-1",
		ProductID:   "product-1",
		WarehouseID: "warehouse-1",
		Quantity:    decimal.RequireFromString("4.250"),
		Status:      OrderStockReservationStatusReleased,
		Reason:      "manual release",
		CreatedAt:   createdAt,
		CreatedBy:   "user-1",
		UpdatedAt:   updatedAt,
		ReleasedAt:  &releasedAt,
		ReleasedBy:  "user-2",
	}

	model := stockReservationToModel(reservation)

	require.NotNil(t, model.Reason)
	require.NotNil(t, model.CreatedBy)
	require.NotNil(t, model.ReleasedBy)
	assert.Equal(t, reservation.Reason, *model.Reason)
	assert.Equal(t, reservation.CreatedBy, *model.CreatedBy)
	assert.Equal(t, reservation.ReleasedBy, *model.ReleasedBy)
	assert.True(t, model.Quantity.Decimal.Equal(reservation.Quantity))
	assert.Equal(t, reservation, stockReservationFromModel(model))

	emptyOptionalModel := stockReservationToModel(&OrderStockReservation{
		ID:          "reservation-2",
		TenantID:    "tenant-1",
		OrderID:     "order-1",
		ProductID:   "product-2",
		WarehouseID: "warehouse-1",
		Quantity:    decimal.RequireFromString("1.000"),
		Status:      OrderStockReservationStatusReserved,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	})
	assert.Nil(t, emptyOptionalModel.Reason)
	assert.Nil(t, emptyOptionalModel.CreatedBy)
	assert.Nil(t, emptyOptionalModel.ReleasedBy)

	reason := "reserved for order"
	createdBy := "user-3"
	releasedBy := "user-4"
	reservations := stockReservationsFromModels([]models.OrderStockReservation{
		{
			ID:          "reservation-3",
			TenantID:    "tenant-1",
			OrderID:     "order-2",
			ProductID:   "product-3",
			WarehouseID: "warehouse-1",
			Quantity:    models.Decimal{Decimal: decimal.RequireFromString("2.500")},
			Status:      OrderStockReservationStatusReserved,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
		{
			ID:          "reservation-4",
			TenantID:    "tenant-1",
			OrderID:     "order-2",
			ProductID:   "product-4",
			WarehouseID: "warehouse-2",
			Quantity:    models.Decimal{Decimal: decimal.RequireFromString("3.000")},
			Status:      OrderStockReservationStatusReleased,
			Reason:      &reason,
			CreatedAt:   createdAt,
			CreatedBy:   &createdBy,
			UpdatedAt:   updatedAt,
			ReleasedAt:  &releasedAt,
			ReleasedBy:  &releasedBy,
		},
	})

	require.Len(t, reservations, 2)
	assert.Empty(t, reservations[0].Reason)
	assert.Empty(t, reservations[0].CreatedBy)
	assert.Nil(t, reservations[0].ReleasedAt)
	assert.Empty(t, reservations[0].ReleasedBy)
	assert.Equal(t, reason, reservations[1].Reason)
	assert.Equal(t, createdBy, reservations[1].CreatedBy)
	assert.Equal(t, releasedBy, reservations[1].ReleasedBy)
	assert.True(t, reservations[1].Quantity.Equal(decimal.RequireFromString("3.000")))
}

func TestStringPointerMappingHelpers(t *testing.T) {
	assert.Nil(t, nilIfEmpty(""))

	value := nilIfEmpty("value")
	require.NotNil(t, value)
	assert.Equal(t, "value", *value)

	assert.Empty(t, valueOrEmpty(nil))
	assert.Equal(t, "value", valueOrEmpty(value))
}
