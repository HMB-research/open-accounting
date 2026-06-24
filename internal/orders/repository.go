package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository defines the contract for order data access
type Repository interface {
	Create(ctx context.Context, schemaName string, order *Order) error
	GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*Order, error)
	List(ctx context.Context, schemaName, tenantID string, filter *OrderFilter) ([]Order, error)
	Update(ctx context.Context, schemaName string, order *Order) error
	UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status OrderStatus) error
	Delete(ctx context.Context, schemaName, tenantID, orderID string) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error)
	SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error
	ListStockReservations(ctx context.Context, schemaName, tenantID, orderID string) ([]OrderStockReservation, error)
	GetStockReservation(ctx context.Context, schemaName, tenantID, orderID, productID, warehouseID string) (*OrderStockReservation, error)
	UpsertStockReservation(ctx context.Context, schemaName string, reservation *OrderStockReservation) error
	ReleaseStockReservation(ctx context.Context, schemaName, tenantID, orderID, productID, warehouseID string, quantity decimal.Decimal, reason, releasedBy string) (*OrderStockReservation, error)
}

// ErrOrderNotFound is returned when an order is not found
var ErrOrderNotFound = fmt.Errorf("order not found")

// ErrOrderStockReservationNotFound is returned when an order stock reservation is not found.
var ErrOrderStockReservationNotFound = fmt.Errorf("order stock reservation not found")

var errOrdersRepositoryDatabaseNotConfigured = errors.New("orders repository database is not configured")

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create orders GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errOrdersRepositoryDatabaseNotConfigured
	}
	return r.db.WithContext(ctx), nil
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return database.TenantTable(db, schemaName, tableName)
}

// Create inserts a new order with its lines
func (r *GORMRepository) Create(ctx context.Context, schemaName string, order *Order) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		ordersTable, err := database.TenantTable(tx, schemaName, "orders")
		if err != nil {
			return fmt.Errorf("qualify orders table: %w", err)
		}
		if err := ordersTable.Create(orderToModel(order)).Error; err != nil {
			return fmt.Errorf("insert order: %w", err)
		}

		if len(order.Lines) == 0 {
			return nil
		}

		linesTable, err := database.TenantTable(tx, schemaName, "order_lines")
		if err != nil {
			return fmt.Errorf("qualify order lines table: %w", err)
		}
		lineModels := make([]models.OrderLine, len(order.Lines))
		for i := range order.Lines {
			order.Lines[i].OrderID = order.ID
			lineModels[i] = *orderLineToModel(&order.Lines[i])
		}
		if err := linesTable.Create(&lineModels).Error; err != nil {
			return fmt.Errorf("insert order line: %w", err)
		}
		return nil
	})
}

// GetByID retrieves an order by ID with its lines
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*Order, error) {
	db, err := r.tenantTable(ctx, schemaName, "orders")
	if err != nil {
		return nil, fmt.Errorf("qualify orders table: %w", err)
	}

	var orderModel models.Order
	err = db.Where("id = ? AND tenant_id = ?", orderID, tenantID).First(&orderModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}

	order := orderFromModel(&orderModel)
	lines, err := r.listOrderLines(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return nil, err
	}
	order.Lines = lines
	return order, nil
}

// List retrieves orders with optional filtering
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter *OrderFilter) ([]Order, error) {
	db, err := r.tenantTable(ctx, schemaName, "orders")
	if err != nil {
		return nil, fmt.Errorf("qualify orders table: %w", err)
	}

	query := db.Where("tenant_id = ?", tenantID)
	if filter != nil {
		if filter.Status != "" {
			query = query.Where("status = ?", string(filter.Status))
		}
		if filter.ContactID != "" {
			query = query.Where("contact_id = ?", filter.ContactID)
		}
		if filter.FromDate != nil {
			query = query.Where("order_date >= ?", filter.FromDate)
		}
		if filter.ToDate != nil {
			query = query.Where("order_date <= ?", filter.ToDate)
		}
		if strings.TrimSpace(filter.Search) != "" {
			query = query.Where("order_number ILIKE ?", "%"+strings.TrimSpace(filter.Search)+"%")
		}
	}

	var orderModels []models.Order
	if err := query.
		Order("order_date DESC").
		Order("order_number DESC").
		Find(&orderModels).Error; err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	orders := make([]Order, len(orderModels))
	for i := range orderModels {
		orders[i] = *orderFromModel(&orderModels[i])
	}
	return orders, nil
}

// Update updates an order and its lines
func (r *GORMRepository) Update(ctx context.Context, schemaName string, order *Order) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		ordersTable, err := database.TenantTable(tx, schemaName, "orders")
		if err != nil {
			return fmt.Errorf("qualify orders table: %w", err)
		}
		result := ordersTable.Where("id = ? AND tenant_id = ? AND status IN ?", order.ID, order.TenantID, []string{string(OrderStatusPending), string(OrderStatusConfirmed)}).
			Updates(map[string]interface{}{
				"contact_id":        order.ContactID,
				"order_date":        order.OrderDate,
				"expected_delivery": order.ExpectedDelivery,
				"currency":          order.Currency,
				"exchange_rate":     order.ExchangeRate.String(),
				"subtotal":          order.Subtotal.String(),
				"vat_amount":        order.VATAmount.String(),
				"total":             order.Total.String(),
				"notes":             order.Notes,
				"updated_at":        time.Now(),
			})
		if result.Error != nil {
			return fmt.Errorf("update order: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrOrderNotFound
		}

		linesTable, err := database.TenantTable(tx, schemaName, "order_lines")
		if err != nil {
			return fmt.Errorf("qualify order lines table: %w", err)
		}
		if err := linesTable.Where("order_id = ?", order.ID).Delete(&models.OrderLine{}).Error; err != nil {
			return fmt.Errorf("delete order lines: %w", err)
		}
		if len(order.Lines) == 0 {
			return nil
		}

		lineModels := make([]models.OrderLine, len(order.Lines))
		for i := range order.Lines {
			order.Lines[i].OrderID = order.ID
			lineModels[i] = *orderLineToModel(&order.Lines[i])
		}
		if err := linesTable.Create(&lineModels).Error; err != nil {
			return fmt.Errorf("insert order line: %w", err)
		}
		return nil
	})
}

// UpdateStatus updates the status of an order
func (r *GORMRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status OrderStatus) error {
	db, err := r.tenantTable(ctx, schemaName, "orders")
	if err != nil {
		return fmt.Errorf("qualify orders table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", orderID, tenantID).
		Updates(map[string]interface{}{
			"status":     string(status),
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// Delete removes an order (only pending)
func (r *GORMRepository) Delete(ctx context.Context, schemaName, tenantID, orderID string) error {
	db, err := r.tenantTable(ctx, schemaName, "orders")
	if err != nil {
		return fmt.Errorf("qualify orders table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ? AND status = ?", orderID, tenantID, string(OrderStatusPending)).
		Delete(&models.Order{})
	if result.Error != nil {
		return fmt.Errorf("delete order: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// GenerateNumber generates a new order number
func (r *GORMRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	db, err := r.tenantTable(ctx, schemaName, "orders")
	if err != nil {
		return "", fmt.Errorf("qualify orders table: %w", err)
	}

	var seq int
	if err := db.
		Select(`
			COALESCE(MAX(
				CASE
					WHEN order_number ~ ? THEN CAST(SUBSTRING(order_number FROM ?) AS INTEGER)
					ELSE 0
				END
			), 0) + 1
		`, "ORD-[0-9]+$", "ORD-([0-9]+)$").
		Where("tenant_id = ?", tenantID).
		Scan(&seq).Error; err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}
	return fmt.Sprintf("ORD-%05d", seq), nil
}

// SetConvertedToInvoice marks an order as converted to an invoice
func (r *GORMRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error {
	db, err := r.tenantTable(ctx, schemaName, "orders")
	if err != nil {
		return fmt.Errorf("qualify orders table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", orderID, tenantID).
		Updates(map[string]interface{}{
			"converted_to_invoice_id": invoiceID,
			"updated_at":              time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("set converted to invoice: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// ListStockReservations retrieves current stock reservations for an order.
func (r *GORMRepository) ListStockReservations(ctx context.Context, schemaName, tenantID, orderID string) ([]OrderStockReservation, error) {
	db, err := r.tenantTable(ctx, schemaName, "order_stock_reservations")
	if err != nil {
		return nil, fmt.Errorf("qualify order stock reservations table: %w", err)
	}

	var reservationModels []models.OrderStockReservation
	if err := db.
		Where("tenant_id = ? AND order_id = ?", tenantID, orderID).
		Order("created_at ASC").
		Order("product_id ASC").
		Order("warehouse_id ASC").
		Find(&reservationModels).Error; err != nil {
		return nil, fmt.Errorf("list order stock reservations: %w", err)
	}
	return stockReservationsFromModels(reservationModels), nil
}

// GetStockReservation retrieves one order stock reservation.
func (r *GORMRepository) GetStockReservation(ctx context.Context, schemaName, tenantID, orderID, productID, warehouseID string) (*OrderStockReservation, error) {
	db, err := r.tenantTable(ctx, schemaName, "order_stock_reservations")
	if err != nil {
		return nil, fmt.Errorf("qualify order stock reservations table: %w", err)
	}

	var reservationModel models.OrderStockReservation
	err = db.
		Where("tenant_id = ? AND order_id = ? AND product_id = ? AND warehouse_id = ?", tenantID, orderID, productID, warehouseID).
		First(&reservationModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOrderStockReservationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order stock reservation: %w", err)
	}
	return stockReservationFromModel(&reservationModel), nil
}

// UpsertStockReservation increases or recreates an order stock reservation.
func (r *GORMRepository) UpsertStockReservation(ctx context.Context, schemaName string, reservation *OrderStockReservation) error {
	db, err := r.tenantTable(ctx, schemaName, "order_stock_reservations")
	if err != nil {
		return fmt.Errorf("qualify order stock reservations table: %w", err)
	}

	reservationModel := stockReservationToModel(reservation)
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "order_id"},
			{Name: "product_id"},
			{Name: "warehouse_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":    gorm.Expr("order_stock_reservations.quantity + EXCLUDED.quantity"),
			"status":      OrderStockReservationStatusReserved,
			"reason":      gorm.Expr("COALESCE(EXCLUDED.reason, order_stock_reservations.reason)"),
			"updated_at":  time.Now(),
			"released_at": nil,
			"released_by": nil,
		}),
	}).Create(reservationModel).Error; err != nil {
		return fmt.Errorf("upsert order stock reservation: %w", err)
	}
	return nil
}

// ReleaseStockReservation decreases an order stock reservation.
func (r *GORMRepository) ReleaseStockReservation(
	ctx context.Context,
	schemaName string,
	tenantID string,
	orderID string,
	productID string,
	warehouseID string,
	quantity decimal.Decimal,
	reason string,
	releasedBy string,
) (*OrderStockReservation, error) {
	db, err := r.tenantTable(ctx, schemaName, "order_stock_reservations")
	if err != nil {
		return nil, fmt.Errorf("qualify order stock reservations table: %w", err)
	}

	releasedByValue := nilIfEmpty(releasedBy)
	result := db.Model(&models.OrderStockReservation{}).
		Where("tenant_id = ? AND order_id = ? AND product_id = ? AND warehouse_id = ? AND quantity >= ?", tenantID, orderID, productID, warehouseID, quantity).
		Updates(map[string]interface{}{
			"quantity":    gorm.Expr("quantity - ?", quantity),
			"status":      gorm.Expr("CASE WHEN quantity - ? <= 0 THEN ? ELSE ? END", quantity, OrderStockReservationStatusReleased, OrderStockReservationStatusReserved),
			"reason":      gorm.Expr("COALESCE(?, reason)", nilIfEmpty(reason)),
			"updated_at":  time.Now(),
			"released_at": gorm.Expr("CASE WHEN quantity - ? <= 0 THEN ? ELSE released_at END", quantity, time.Now()),
			"released_by": gorm.Expr("CASE WHEN quantity - ? <= 0 THEN ? ELSE released_by END", quantity, releasedByValue),
		})
	if result.Error != nil {
		return nil, fmt.Errorf("release order stock reservation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrOrderStockReservationNotFound
	}
	return r.GetStockReservation(ctx, schemaName, tenantID, orderID, productID, warehouseID)
}

func (r *GORMRepository) listOrderLines(ctx context.Context, schemaName, tenantID, orderID string) ([]OrderLine, error) {
	db, err := r.tenantTable(ctx, schemaName, "order_lines")
	if err != nil {
		return nil, fmt.Errorf("qualify order lines table: %w", err)
	}

	var lineModels []models.OrderLine
	if err := db.
		Where("order_id = ? AND tenant_id = ?", orderID, tenantID).
		Order("line_number ASC").
		Find(&lineModels).Error; err != nil {
		return nil, fmt.Errorf("get order lines: %w", err)
	}

	lines := make([]OrderLine, len(lineModels))
	for i := range lineModels {
		lines[i] = *orderLineFromModel(&lineModels[i])
	}
	return lines, nil
}

func orderToModel(order *Order) *models.Order {
	return &models.Order{
		ID:                   order.ID,
		TenantID:             order.TenantID,
		OrderNumber:          order.OrderNumber,
		ContactID:            order.ContactID,
		OrderDate:            order.OrderDate,
		ExpectedDelivery:     order.ExpectedDelivery,
		Status:               string(order.Status),
		Currency:             order.Currency,
		ExchangeRate:         models.Decimal{Decimal: order.ExchangeRate},
		Subtotal:             models.Decimal{Decimal: order.Subtotal},
		VATAmount:            models.Decimal{Decimal: order.VATAmount},
		Total:                models.Decimal{Decimal: order.Total},
		Notes:                order.Notes,
		QuoteID:              order.QuoteID,
		ConvertedToInvoiceID: order.ConvertedToInvoiceID,
		CreatedAt:            order.CreatedAt,
		CreatedBy:            order.CreatedBy,
		UpdatedAt:            order.UpdatedAt,
	}
}

func orderFromModel(order *models.Order) *Order {
	return &Order{
		ID:                   order.ID,
		TenantID:             order.TenantID,
		OrderNumber:          order.OrderNumber,
		ContactID:            order.ContactID,
		OrderDate:            order.OrderDate,
		ExpectedDelivery:     order.ExpectedDelivery,
		Status:               OrderStatus(order.Status),
		Currency:             order.Currency,
		ExchangeRate:         order.ExchangeRate.Decimal,
		Subtotal:             order.Subtotal.Decimal,
		VATAmount:            order.VATAmount.Decimal,
		Total:                order.Total.Decimal,
		Notes:                order.Notes,
		QuoteID:              order.QuoteID,
		ConvertedToInvoiceID: order.ConvertedToInvoiceID,
		CreatedAt:            order.CreatedAt,
		CreatedBy:            order.CreatedBy,
		UpdatedAt:            order.UpdatedAt,
	}
}

func orderLineToModel(line *OrderLine) *models.OrderLine {
	return &models.OrderLine{
		ID:              line.ID,
		TenantID:        line.TenantID,
		OrderID:         line.OrderID,
		LineNumber:      line.LineNumber,
		Description:     line.Description,
		Quantity:        models.Decimal{Decimal: line.Quantity},
		Unit:            line.Unit,
		UnitPrice:       models.Decimal{Decimal: line.UnitPrice},
		DiscountPercent: models.Decimal{Decimal: line.DiscountPercent},
		VATRate:         models.Decimal{Decimal: line.VATRate},
		LineSubtotal:    models.Decimal{Decimal: line.LineSubtotal},
		LineVAT:         models.Decimal{Decimal: line.LineVAT},
		LineTotal:       models.Decimal{Decimal: line.LineTotal},
		ProductID:       line.ProductID,
	}
}

func orderLineFromModel(line *models.OrderLine) *OrderLine {
	return &OrderLine{
		ID:              line.ID,
		TenantID:        line.TenantID,
		OrderID:         line.OrderID,
		LineNumber:      line.LineNumber,
		Description:     line.Description,
		Quantity:        line.Quantity.Decimal,
		Unit:            line.Unit,
		UnitPrice:       line.UnitPrice.Decimal,
		DiscountPercent: line.DiscountPercent.Decimal,
		VATRate:         line.VATRate.Decimal,
		LineSubtotal:    line.LineSubtotal.Decimal,
		LineVAT:         line.LineVAT.Decimal,
		LineTotal:       line.LineTotal.Decimal,
		ProductID:       line.ProductID,
	}
}

func stockReservationToModel(reservation *OrderStockReservation) *models.OrderStockReservation {
	return &models.OrderStockReservation{
		ID:          reservation.ID,
		TenantID:    reservation.TenantID,
		OrderID:     reservation.OrderID,
		ProductID:   reservation.ProductID,
		WarehouseID: reservation.WarehouseID,
		Quantity:    models.Decimal{Decimal: reservation.Quantity},
		Status:      reservation.Status,
		Reason:      nilIfEmpty(reservation.Reason),
		CreatedAt:   reservation.CreatedAt,
		CreatedBy:   nilIfEmpty(reservation.CreatedBy),
		UpdatedAt:   reservation.UpdatedAt,
		ReleasedAt:  reservation.ReleasedAt,
		ReleasedBy:  nilIfEmpty(reservation.ReleasedBy),
	}
}

func stockReservationFromModel(reservation *models.OrderStockReservation) *OrderStockReservation {
	return &OrderStockReservation{
		ID:          reservation.ID,
		TenantID:    reservation.TenantID,
		OrderID:     reservation.OrderID,
		ProductID:   reservation.ProductID,
		WarehouseID: reservation.WarehouseID,
		Quantity:    reservation.Quantity.Decimal,
		Status:      reservation.Status,
		Reason:      valueOrEmpty(reservation.Reason),
		CreatedAt:   reservation.CreatedAt,
		CreatedBy:   valueOrEmpty(reservation.CreatedBy),
		UpdatedAt:   reservation.UpdatedAt,
		ReleasedAt:  reservation.ReleasedAt,
		ReleasedBy:  valueOrEmpty(reservation.ReleasedBy),
	}
}

func stockReservationsFromModels(reservationModels []models.OrderStockReservation) []OrderStockReservation {
	reservations := make([]OrderStockReservation, len(reservationModels))
	for i := range reservationModels {
		reservations[i] = *stockReservationFromModel(&reservationModels[i])
	}
	return reservations
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
