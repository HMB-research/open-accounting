package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides order operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new orders service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new orders service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// Create creates a new order
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreateOrderRequest) (*Order, error) {
	order := &Order{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		ContactID:        req.ContactID,
		OrderDate:        req.OrderDate,
		ExpectedDelivery: req.ExpectedDelivery,
		Currency:         req.Currency,
		ExchangeRate:     req.ExchangeRate,
		Status:           OrderStatusPending,
		Notes:            req.Notes,
		QuoteID:          req.QuoteID,
		CreatedAt:        time.Now(),
		CreatedBy:        req.UserID,
		UpdatedAt:        time.Now(),
	}

	if order.Currency == "" {
		order.Currency = "EUR"
	}
	if order.ExchangeRate.IsZero() {
		order.ExchangeRate = decimal.NewFromInt(1)
	}
	if order.OrderDate.IsZero() {
		order.OrderDate = time.Now()
	}

	// Convert request lines to order lines
	for i, reqLine := range req.Lines {
		line := OrderLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			LineNumber:      i + 1,
			Description:     reqLine.Description,
			Quantity:        reqLine.Quantity,
			Unit:            reqLine.Unit,
			UnitPrice:       reqLine.UnitPrice,
			DiscountPercent: reqLine.DiscountPercent,
			VATRate:         reqLine.VATRate,
			ProductID:       reqLine.ProductID,
		}
		line.Calculate()
		order.Lines = append(order.Lines, line)
	}

	// Calculate totals
	order.Calculate()

	// Validate
	if err := order.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate order number
	orderNumber, err := s.repo.GenerateNumber(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("generate order number: %w", err)
	}
	order.OrderNumber = orderNumber

	// Create order via repository
	if err := s.repo.Create(ctx, schemaName, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	return order, nil
}

// GetByID retrieves an order by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, orderID string) (*Order, error) {
	order, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	return order, nil
}

// List retrieves orders with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *OrderFilter) ([]Order, error) {
	orders, err := s.repo.List(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}

// Update updates an order (only pending/confirmed)
func (s *Service) Update(ctx context.Context, tenantID, schemaName, orderID string, req *UpdateOrderRequest) (*Order, error) {
	// Get existing order
	existing, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}

	if existing.Status != OrderStatusPending && existing.Status != OrderStatusConfirmed {
		return nil, fmt.Errorf("only pending or confirmed orders can be updated")
	}

	// Update fields
	existing.ContactID = req.ContactID
	existing.OrderDate = req.OrderDate
	existing.ExpectedDelivery = req.ExpectedDelivery
	existing.Currency = req.Currency
	existing.ExchangeRate = req.ExchangeRate
	existing.Notes = req.Notes
	existing.UpdatedAt = time.Now()

	if existing.Currency == "" {
		existing.Currency = "EUR"
	}
	if existing.ExchangeRate.IsZero() {
		existing.ExchangeRate = decimal.NewFromInt(1)
	}

	// Replace lines
	existing.Lines = nil
	for i, reqLine := range req.Lines {
		line := OrderLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			OrderID:         orderID,
			LineNumber:      i + 1,
			Description:     reqLine.Description,
			Quantity:        reqLine.Quantity,
			Unit:            reqLine.Unit,
			UnitPrice:       reqLine.UnitPrice,
			DiscountPercent: reqLine.DiscountPercent,
			VATRate:         reqLine.VATRate,
			ProductID:       reqLine.ProductID,
		}
		line.Calculate()
		existing.Lines = append(existing.Lines, line)
	}

	// Calculate totals
	existing.Calculate()

	// Validate
	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update via repository
	if err := s.repo.Update(ctx, schemaName, existing); err != nil {
		return nil, fmt.Errorf("update order: %w", err)
	}

	return existing, nil
}

// Confirm marks an order as confirmed
func (s *Service) Confirm(ctx context.Context, tenantID, schemaName, orderID string) error {
	order, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if order.Status != OrderStatusPending {
		return fmt.Errorf("order is not in pending status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusConfirmed); err != nil {
		return fmt.Errorf("confirm order: %w", err)
	}
	return nil
}

// Process marks an order as processing
func (s *Service) Process(ctx context.Context, tenantID, schemaName, orderID string) error {
	order, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if order.Status != OrderStatusConfirmed {
		return fmt.Errorf("order must be confirmed before processing")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusProcessing); err != nil {
		return fmt.Errorf("process order: %w", err)
	}
	return nil
}

// Ship marks an order as shipped
func (s *Service) Ship(ctx context.Context, tenantID, schemaName, orderID string) error {
	order, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if order.Status != OrderStatusProcessing && order.Status != OrderStatusConfirmed {
		return fmt.Errorf("order cannot be shipped in current status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusShipped); err != nil {
		return fmt.Errorf("ship order: %w", err)
	}
	return nil
}

// Deliver marks an order as delivered
func (s *Service) Deliver(ctx context.Context, tenantID, schemaName, orderID string) error {
	order, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if order.Status != OrderStatusShipped {
		return fmt.Errorf("order must be shipped before delivery")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusDelivered); err != nil {
		return fmt.Errorf("deliver order: %w", err)
	}
	return nil
}

// Cancel cancels an order
func (s *Service) Cancel(ctx context.Context, tenantID, schemaName, orderID string) error {
	order, err := s.repo.GetByID(ctx, schemaName, tenantID, orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if order.Status == OrderStatusDelivered || order.Status == OrderStatusCancelled {
		return fmt.Errorf("order cannot be cancelled in current status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusCancelled); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	return nil
}

// Delete deletes a pending order
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, orderID string) error {
	if err := s.repo.Delete(ctx, schemaName, tenantID, orderID); err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	return nil
}

// ConvertToInvoice marks an order as converted to an invoice
func (s *Service) ConvertToInvoice(ctx context.Context, tenantID, schemaName, orderID, invoiceID string) error {
	if err := s.repo.SetConvertedToInvoice(ctx, schemaName, tenantID, orderID, invoiceID); err != nil {
		return fmt.Errorf("convert to invoice: %w", err)
	}
	return nil
}
