package orders

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type orderDryRunConnPool struct{}

func (orderDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run orders tests should not prepare statements")
}

func (orderDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run orders tests should not execute statements")
}

func (orderDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run orders tests should not query rows")
}

func (orderDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (orderDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &orderDryRunTx{}, nil
}

type orderDryRunTx struct {
	orderDryRunConnPool
}

func (*orderDryRunTx) Commit() error {
	return nil
}

func (*orderDryRunTx) Rollback() error {
	return nil
}

type orderDryRunDBOption func(t *testing.T, db *gorm.DB)

type orderDryRunFixtures struct {
	order             *models.Order
	orders            []models.Order
	orderLines        []models.OrderLine
	sequence          *int
	stockReservation  *models.OrderStockReservation
	stockReservations []models.OrderStockReservation
}

type orderDryRunSQLCapture struct {
	statements []string
}

func newOrderDryRunDB(t *testing.T, opts ...orderDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: orderDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withOrderDryRunFixtures(fixtures orderDryRunFixtures) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(orderDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.Order:
				if fixtures.order != nil {
					*dest = *fixtures.order
					tx.RowsAffected = 1
				}
			case *[]models.Order:
				if fixtures.orders != nil {
					*dest = append([]models.Order(nil), fixtures.orders...)
					tx.RowsAffected = int64(len(fixtures.orders))
				}
			case *[]models.OrderLine:
				if fixtures.orderLines != nil {
					*dest = append([]models.OrderLine(nil), fixtures.orderLines...)
					tx.RowsAffected = int64(len(fixtures.orderLines))
				}
			case *int:
				if fixtures.sequence != nil {
					*dest = *fixtures.sequence
					tx.RowsAffected = 1
				}
			case *models.OrderStockReservation:
				if fixtures.stockReservation != nil {
					*dest = *fixtures.stockReservation
					tx.RowsAffected = 1
				}
			case *[]models.OrderStockReservation:
				if fixtures.stockReservations != nil {
					*dest = append([]models.OrderStockReservation(nil), fixtures.stockReservations...)
					tx.RowsAffected = int64(len(fixtures.stockReservations))
				}
			}
		})
		require.NoError(t, err)
	}
}

func withOrderDryRunSQLCapture(capture *orderDryRunSQLCapture) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(orderDryRunCallbackName(t, "capture_create"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Query().After("gorm:query").Register(orderDryRunCallbackName(t, "capture_query"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Update().After("gorm:update").Register(orderDryRunCallbackName(t, "capture_update"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Delete().After("gorm:delete").Register(orderDryRunCallbackName(t, "capture_delete"), capture.capture)
		require.NoError(t, err)
	}
}

func withOrderDryRunQueryError(expectedErr error) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(orderDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withOrderDryRunCreateErrorOnCall(call int, expectedErr error) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var calls int
		err := db.Callback().Create().Before("gorm:create").Register(orderDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			calls++
			if calls == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withOrderDryRunUpdateRows(rows ...int64) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(orderDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func withOrderDryRunUpdateError(expectedErr error) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(orderDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withOrderDryRunDeleteRows(rows ...int64) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(orderDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func orderDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "orders_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func (c *orderDryRunSQLCapture) capture(tx *gorm.DB) {
	if sql := strings.TrimSpace(tx.Statement.SQL.String()); sql != "" {
		c.statements = append(c.statements, sql)
	}
}

func (c *orderDryRunSQLCapture) assertContains(t *testing.T, expected string) {
	t.Helper()
	for _, statement := range c.statements {
		if strings.Contains(statement, expected) {
			return
		}
	}
	t.Fatalf("expected dry-run SQL to contain %q in %#v", expected, c.statements)
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)
	orderModel := orderToModel(order)
	lineModel := orderLineToModel(&order.Lines[0])
	reservation := orderDryRunStockReservation(tenantID, order.ID, now)
	reservationModel := stockReservationToModel(reservation)
	capture := &orderDryRunSQLCapture{}
	repo := NewGORMRepository(newOrderDryRunDB(t,
		withOrderDryRunFixtures(orderDryRunFixtures{
			order:             orderModel,
			orders:            []models.Order{*orderModel},
			orderLines:        []models.OrderLine{*lineModel},
			stockReservation:  reservationModel,
			stockReservations: []models.OrderStockReservation{*reservationModel},
		}),
		withOrderDryRunUpdateRows(1, 1, 1, 1),
		withOrderDryRunDeleteRows(1),
		withOrderDryRunSQLCapture(capture),
	))

	require.NoError(t, repo.Create(ctx, schemaName, order))
	assert.Equal(t, order.ID, order.Lines[0].OrderID)

	gotOrder, err := repo.GetByID(ctx, schemaName, tenantID, order.ID)
	require.NoError(t, err)
	require.Len(t, gotOrder.Lines, 1)
	assert.Equal(t, order.ID, gotOrder.ID)
	assert.Equal(t, order.Lines[0].ID, gotOrder.Lines[0].ID)

	fromDate := now.AddDate(0, 0, -7)
	toDate := now.AddDate(0, 0, 7)
	listedOrders, err := repo.List(ctx, schemaName, tenantID, &OrderFilter{
		Status:    OrderStatusConfirmed,
		ContactID: order.ContactID,
		FromDate:  &fromDate,
		ToDate:    &toDate,
		Search:    " ORD-00042 ",
	})
	require.NoError(t, err)
	require.Len(t, listedOrders, 1)
	assert.Equal(t, order.OrderNumber, listedOrders[0].OrderNumber)

	order.Notes = "updated dry-run order"
	require.NoError(t, repo.Update(ctx, schemaName, order))
	require.NoError(t, repo.UpdateStatus(ctx, schemaName, tenantID, order.ID, OrderStatusShipped))
	require.NoError(t, repo.Delete(ctx, schemaName, tenantID, order.ID))

	require.NoError(t, repo.SetConvertedToInvoice(ctx, schemaName, tenantID, order.ID, "invoice-1"))

	reservations, err := repo.ListStockReservations(ctx, schemaName, tenantID, order.ID)
	require.NoError(t, err)
	require.Len(t, reservations, 1)
	assert.True(t, reservation.Quantity.Equal(reservations[0].Quantity))

	gotReservation, err := repo.GetStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID)
	require.NoError(t, err)
	assert.Equal(t, reservation.ID, gotReservation.ID)

	require.NoError(t, repo.UpsertStockReservation(ctx, schemaName, reservation))

	releasedReservation, err := repo.ReleaseStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID, decimal.NewFromInt(1), "release", "user-2")
	require.NoError(t, err)
	assert.Equal(t, reservation.ID, releasedReservation.ID)

	capture.assertContains(t, `"tenant_orders"."orders"`)
	capture.assertContains(t, `"tenant_orders"."order_lines"`)
	capture.assertContains(t, `"tenant_orders"."order_stock_reservations"`)
	capture.assertContains(t, `order_number ILIKE`)
	capture.assertContains(t, `ORDER BY order_date DESC`)
	capture.assertContains(t, `line_number ASC`)
}

func TestGORMRepositoryDryRunInvalidSchema(t *testing.T) {
	ctx := context.Background()
	invalidSchema := "tenant-orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)
	reservation := orderDryRunStockReservation(tenantID, order.ID, now)
	repo := NewGORMRepository(newOrderDryRunDB(t))

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, invalidSchema, order)
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				got, err := repo.GetByID(ctx, invalidSchema, tenantID, order.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				got, err := repo.List(ctx, invalidSchema, tenantID, &OrderFilter{Status: OrderStatusPending})
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T) error {
				return repo.Update(ctx, invalidSchema, order)
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				return repo.UpdateStatus(ctx, invalidSchema, tenantID, order.ID, OrderStatusConfirmed)
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				return repo.Delete(ctx, invalidSchema, tenantID, order.ID)
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T) error {
				got, err := repo.GenerateNumber(ctx, invalidSchema, tenantID)
				assert.Empty(t, got)
				return err
			},
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T) error {
				return repo.SetConvertedToInvoice(ctx, invalidSchema, tenantID, order.ID, "invoice-1")
			},
		},
		{
			name: "ListStockReservations",
			run: func(t *testing.T) error {
				got, err := repo.ListStockReservations(ctx, invalidSchema, tenantID, order.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "GetStockReservation",
			run: func(t *testing.T) error {
				got, err := repo.GetStockReservation(ctx, invalidSchema, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpsertStockReservation",
			run: func(t *testing.T) error {
				return repo.UpsertStockReservation(ctx, invalidSchema, reservation)
			},
		},
		{
			name: "ReleaseStockReservation",
			run: func(t *testing.T) error {
				got, err := repo.ReleaseStockReservation(ctx, invalidSchema, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID, decimal.NewFromInt(1), "release", "user-1")
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "listOrderLines",
			run: func(t *testing.T) error {
				got, err := repo.listOrderLines(ctx, invalidSchema, tenantID, order.ID)
				assert.Nil(t, got)
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SQL identifier")
		})
	}
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)

	t.Run("GenerateNumber wraps dry-run scan errors", func(t *testing.T) {
		repo := NewGORMRepository(newOrderDryRunDB(t))

		got, err := repo.GenerateNumber(ctx, schemaName, tenantID)

		assert.Empty(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "generate order number")
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})

	t.Run("listOrderLines wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("line query failed")
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunQueryError(expectedErr)))

		got, err := repo.listOrderLines(ctx, schemaName, tenantID, order.ID)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get order lines")
	})

	t.Run("Update wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("update failed")
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateError(expectedErr)))

		err := repo.Update(ctx, schemaName, order)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update order")
	})

	t.Run("Update returns not found when no rows change", func(t *testing.T) {
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateRows(0)))

		err := repo.Update(ctx, schemaName, order)

		require.ErrorIs(t, err, ErrOrderNotFound)
	})

	t.Run("Create wraps order line insert errors", func(t *testing.T) {
		expectedErr := errors.New("line insert failed")
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunCreateErrorOnCall(2, expectedErr)))

		err := repo.Create(ctx, schemaName, order)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert order line")
	})
}

func orderDryRunOrder(tenantID string, now time.Time) *Order {
	expectedDelivery := now.AddDate(0, 0, 14)
	productID := "product-1"
	quoteID := "quote-1"
	return &Order{
		ID:               "order-1",
		TenantID:         tenantID,
		OrderNumber:      "ORD-00042",
		ContactID:        "contact-1",
		OrderDate:        now,
		ExpectedDelivery: &expectedDelivery,
		Status:           OrderStatusConfirmed,
		Currency:         "EUR",
		ExchangeRate:     decimal.NewFromInt(1),
		Subtotal:         decimal.RequireFromString("100.00"),
		VATAmount:        decimal.RequireFromString("22.00"),
		Total:            decimal.RequireFromString("122.00"),
		Notes:            "dry-run order",
		QuoteID:          &quoteID,
		CreatedAt:        now,
		CreatedBy:        "user-1",
		UpdatedAt:        now,
		Lines: []OrderLine{
			{
				ID:              "order-line-1",
				TenantID:        tenantID,
				OrderID:         "order-1",
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
				ProductID:       &productID,
			},
		},
	}
}

func orderDryRunStockReservation(tenantID, orderID string, now time.Time) *OrderStockReservation {
	return &OrderStockReservation{
		ID:          "reservation-1",
		TenantID:    tenantID,
		OrderID:     orderID,
		ProductID:   "product-1",
		WarehouseID: "warehouse-1",
		Quantity:    decimal.RequireFromString("2.000"),
		Status:      OrderStockReservationStatusReserved,
		Reason:      "dry-run reservation",
		CreatedAt:   now,
		CreatedBy:   "user-1",
		UpdatedAt:   now,
	}
}
