package orders

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/inventory"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	Orders            map[string]*Order
	StockReservations map[string]*OrderStockReservation
	NextNumber        string
	GenerateErr       error
	CreateErr         error
	GetErr            error
	ListErr           error
	UpdateErr         error
	UpdateStatErr     error
	DeleteErr         error
	ConvertErr        error
	StockListErr      error
	StockGetErr       error
	StockUpsertErr    error
	StockReleaseErr   error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		Orders:            make(map[string]*Order),
		StockReservations: make(map[string]*OrderStockReservation),
		NextNumber:        "ORD-00001",
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, order *Order) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Orders[order.ID] = order
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*Order, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	order, ok := m.Orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *OrderFilter) ([]Order, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	var orders []Order
	for _, o := range m.Orders {
		if o.TenantID == tenantID {
			orders = append(orders, *o)
		}
	}
	return orders, nil
}

func (m *MockRepository) Update(ctx context.Context, schemaName string, order *Order) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.Orders[order.ID] = order
	return nil
}

func (m *MockRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status OrderStatus) error {
	if m.UpdateStatErr != nil {
		return m.UpdateStatErr
	}
	order, ok := m.Orders[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	order.Status = status
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, schemaName, tenantID, orderID string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	if _, ok := m.Orders[orderID]; !ok {
		return ErrOrderNotFound
	}
	delete(m.Orders, orderID)
	return nil
}

func (m *MockRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	if m.GenerateErr != nil {
		return "", m.GenerateErr
	}
	return m.NextNumber, nil
}

func (m *MockRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error {
	if m.ConvertErr != nil {
		return m.ConvertErr
	}
	order, ok := m.Orders[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	order.ConvertedToInvoiceID = &invoiceID
	return nil
}

func (m *MockRepository) ListStockReservations(ctx context.Context, schemaName, tenantID, orderID string) ([]OrderStockReservation, error) {
	if m.StockListErr != nil {
		return nil, m.StockListErr
	}
	var reservations []OrderStockReservation
	for _, reservation := range m.StockReservations {
		if reservation.TenantID == tenantID && reservation.OrderID == orderID {
			reservations = append(reservations, *reservation)
		}
	}
	return reservations, nil
}

func (m *MockRepository) GetStockReservation(ctx context.Context, schemaName, tenantID, orderID, productID, warehouseID string) (*OrderStockReservation, error) {
	if m.StockGetErr != nil {
		return nil, m.StockGetErr
	}
	key := orderStockReservationKey(orderID, productID, warehouseID)
	reservation, ok := m.StockReservations[key]
	if !ok || reservation.TenantID != tenantID {
		return nil, ErrOrderStockReservationNotFound
	}
	return reservation, nil
}

func (m *MockRepository) UpsertStockReservation(ctx context.Context, schemaName string, reservation *OrderStockReservation) error {
	if m.StockUpsertErr != nil {
		return m.StockUpsertErr
	}
	key := orderStockReservationKey(reservation.OrderID, reservation.ProductID, reservation.WarehouseID)
	existing, ok := m.StockReservations[key]
	if !ok {
		copy := *reservation
		copy.Status = OrderStockReservationStatusReserved
		m.StockReservations[key] = &copy
		return nil
	}
	existing.Quantity = existing.Quantity.Add(reservation.Quantity)
	existing.Status = OrderStockReservationStatusReserved
	existing.Reason = reservation.Reason
	return nil
}

func (m *MockRepository) ReleaseStockReservation(ctx context.Context, schemaName, tenantID, orderID, productID, warehouseID string, quantity decimal.Decimal, reason, releasedBy string) (*OrderStockReservation, error) {
	if m.StockReleaseErr != nil {
		return nil, m.StockReleaseErr
	}
	reservation, err := m.GetStockReservation(ctx, schemaName, tenantID, orderID, productID, warehouseID)
	if err != nil {
		return nil, err
	}
	if reservation.Quantity.LessThan(quantity) {
		return nil, ErrOrderStockReservationNotFound
	}
	reservation.Quantity = reservation.Quantity.Sub(quantity)
	if reservation.Quantity.IsZero() {
		reservation.Status = OrderStockReservationStatusReleased
		reservation.ReleasedBy = releasedBy
	} else {
		reservation.Status = OrderStockReservationStatusReserved
	}
	reservation.Reason = reason
	return reservation, nil
}

func orderStockReservationKey(orderID, productID, warehouseID string) string {
	return orderID + "|" + productID + "|" + warehouseID
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	assert.NotNil(t, svc)
	assert.Equal(t, repo, svc.repo)
}

func TestService_Create(t *testing.T) {
	t.Run("creates order successfully", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			Currency:  "EUR",
			UserID:    "user-1",
			Lines: []CreateOrderLineRequest{
				{
					Description: "Test product",
					Quantity:    decimal.NewFromInt(2),
					UnitPrice:   decimal.NewFromFloat(100.00),
					VATRate:     decimal.NewFromInt(20),
				},
			},
		}

		order, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.NotEmpty(t, order.ID)
		assert.Equal(t, "ORD-00001", order.OrderNumber)
		assert.Equal(t, "tenant-1", order.TenantID)
		assert.Equal(t, "contact-1", order.ContactID)
		assert.Equal(t, "EUR", order.Currency)
		assert.Equal(t, OrderStatusPending, order.Status)
		assert.Len(t, order.Lines, 1)
		assert.True(t, order.Subtotal.Equal(decimal.NewFromFloat(200.00)))
	})

	t.Run("defaults currency to EUR", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			Currency:  "", // empty
			UserID:    "user-1",
			Lines: []CreateOrderLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		order, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.Equal(t, "EUR", order.Currency)
	})

	t.Run("defaults exchange rate to 1", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateOrderLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		order, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.True(t, order.ExchangeRate.Equal(decimal.NewFromInt(1)))
	})

	t.Run("returns error on validation failure", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		req := &CreateOrderRequest{
			ContactID: "", // missing contact
			OrderDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateOrderLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("returns error when generate number fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GenerateErr = errors.New("generate error")
		svc := NewServiceWithRepository(repo)

		req := &CreateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateOrderLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "generate order number")
	})

	t.Run("returns error when repository create fails", func(t *testing.T) {
		repo := NewMockRepository()
		repo.CreateErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		req := &CreateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			UserID:    "user-1",
			Lines: []CreateOrderLineRequest{
				{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Create(context.Background(), "tenant-1", "test_schema", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create order")
	})
}

func TestService_ImportCSV(t *testing.T) {
	t.Run("imports grouped orders and preserves status", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_code,order_date,expected_delivery,status,currency,exchange_rate,notes,quote_id,line_description,quantity,unit,unit_price,discount_percent,vat_rate,product_code
ORD-LEGACY-1,CUST-1,2026-03-15,2026-03-22,confirmed,EUR,1,March order,33333333-3333-4333-8333-333333333333,Consulting,2,hour,100,10,22,SERV-001
ORD-LEGACY-1,CUST-1,2026-03-15,2026-03-22,confirmed,EUR,1,March order,33333333-3333-4333-8333-333333333333,Support,1,hour,50,0,22,
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Code:     "CUST-1",
			Name:     "Acme",
		}}, []inventory.Product{{
			ID:       "prod-1",
			TenantID: "tenant-1",
			Code:     "SERV-001",
		}}, &ImportOrdersRequest{
			CSVContent: csvContent,
			FileName:   "orders.csv",
			UserID:     "user-1",
		})

		require.NoError(t, err)
		assert.Equal(t, "orders.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.OrdersCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Zero(t, result.RowsSkipped)
		assert.Nil(t, result.Errors)

		require.Len(t, repo.Orders, 1)
		for _, order := range repo.Orders {
			assert.Equal(t, "ORD-LEGACY-1", order.OrderNumber)
			assert.Equal(t, "contact-1", order.ContactID)
			assert.Equal(t, OrderStatusConfirmed, order.Status)
			require.NotNil(t, order.QuoteID)
			assert.Equal(t, "33333333-3333-4333-8333-333333333333", *order.QuoteID)
			assert.True(t, order.Subtotal.Equal(decimal.RequireFromString("230.00")))
			assert.True(t, order.VATAmount.Equal(decimal.RequireFromString("50.60")))
			require.Len(t, order.Lines, 2)
			require.NotNil(t, order.Lines[0].ProductID)
			assert.Equal(t, "prod-1", *order.Lines[0].ProductID)
		}
	})

	t.Run("resolves contact by VAT number", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_vat_number,order_date,line_description,quantity,unit_price,vat_rate
ORD-VAT-1,EE123456789,2026-03-15,Consulting,1,100,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:        "contact-vat",
			TenantID:  "tenant-1",
			RegCode:   "12345678",
			VATNumber: "EE123456789",
			Name:      "VAT Customer",
		}}, nil, &ImportOrdersRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.OrdersCreated)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)
		require.Len(t, repo.Orders, 1)
		for _, order := range repo.Orders {
			assert.Equal(t, "contact-vat", order.ContactID)
		}
	})

	t.Run("resolves quote by quote number", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_code,order_date,quote_number,line_description,quantity,unit_price,vat_rate
ORD-QUOTE-1,CUST-1,2026-03-15,QT-LEGACY-1,Consulting,1,100,22
`

		result, err := svc.ImportCSVWithQuoteReferences(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Code:     "CUST-1",
			Name:     "Acme",
		}}, nil, []ImportQuoteReference{{
			ID:          "11111111-1111-4111-8111-111111111111",
			QuoteNumber: "QT-LEGACY-1",
		}}, &ImportOrdersRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.OrdersCreated)
		assert.Zero(t, result.RowsSkipped)
		require.Len(t, repo.Orders, 1)
		for _, order := range repo.Orders {
			require.NotNil(t, order.QuoteID)
			assert.Equal(t, "11111111-1111-4111-8111-111111111111", *order.QuoteID)
		}
	})

	t.Run("quote id takes precedence over quote number", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_code,order_date,quote_id,quote_number,line_description,quantity,unit_price,vat_rate
ORD-QUOTE-2,CUST-1,2026-03-15,22222222-2222-4222-8222-222222222222,QT-MISSING,Consulting,1,100,22
`

		result, err := svc.ImportCSVWithQuoteReferences(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Code:     "CUST-1",
			Name:     "Acme",
		}}, nil, nil, &ImportOrdersRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 1, result.OrdersCreated)
		require.Len(t, repo.Orders, 1)
		for _, order := range repo.Orders {
			require.NotNil(t, order.QuoteID)
			assert.Equal(t, "22222222-2222-4222-8222-222222222222", *order.QuoteID)
		}
	})

	t.Run("skips duplicate and invalid groups", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["existing"] = &Order{
			ID:          "existing",
			TenantID:    "tenant-1",
			OrderNumber: "ORD-EXISTING",
		}
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_id,order_date,line_description,quantity,unit_price,vat_rate
ORD-EXISTING,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,Duplicate,1,10,22
ORD-MISSING,cccccccc-cccc-4ccc-8ccc-cccccccccccc,2026-03-15,Unknown contact,1,10,22
ORD-BAD,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,Bad quantity,0,10,22
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			TenantID: "tenant-1",
			Name:     "Acme",
		}}, nil, &ImportOrdersRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 3, result.RowsProcessed)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 3, result.RowsSkipped)
		require.Len(t, result.Errors, 3)
		messages := make([]string, 0, len(result.Errors))
		for _, rowErr := range result.Errors {
			messages = append(messages, rowErr.Message)
		}
		joinedMessages := strings.Join(messages, "\n")
		assert.Contains(t, joinedMessages, "already exists")
		assert.Contains(t, joinedMessages, "contact_id")
		assert.Contains(t, joinedMessages, "quantity must be greater than zero")
	})

	t.Run("skips invalid uuid references", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_id,order_date,quote_id,line_description,quantity,unit_price,vat_rate,product_id
ORD-BAD-CONTACT,legacy-contact,2026-03-15,,Bad contact,1,10,22,
ORD-BAD-QUOTE,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,legacy-quote,Bad quote,1,10,22,
ORD-BAD-PRODUCT,bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb,2026-03-15,,Bad product,1,10,22,legacy-product
`

		result, err := svc.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			TenantID: "tenant-1",
			Name:     "Acme",
		}}, nil, &ImportOrdersRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Equal(t, 3, result.RowsProcessed)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 3, result.RowsSkipped)
		require.Len(t, result.Errors, 3)
		assert.Contains(t, result.Errors[0].Message, "contact_id must be a valid UUID")
		assert.Contains(t, result.Errors[1].Message, "quote_id must be a valid UUID")
		assert.Contains(t, result.Errors[2].Message, "product_id must be a valid UUID")
	})

	t.Run("skips unknown quote number", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		csvContent := `order_number,contact_code,order_date,quote_number,line_description,quantity,unit_price,vat_rate
ORD-UNKNOWN-QUOTE,CUST-1,2026-03-15,QT-MISSING,Consulting,1,100,22
`

		result, err := svc.ImportCSVWithQuoteReferences(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
			ID:       "contact-1",
			TenantID: "tenant-1",
			Code:     "CUST-1",
			Name:     "Acme",
		}}, nil, nil, &ImportOrdersRequest{CSVContent: csvContent})

		require.NoError(t, err)
		assert.Zero(t, result.OrdersCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, `quote_number "QT-MISSING" was not found`)
	})
}

func TestService_GetByID(t *testing.T) {
	t.Run("returns order when found", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", TenantID: "tenant-1", OrderNumber: "ORD-00001"}
		svc := NewServiceWithRepository(repo)

		order, err := svc.GetByID(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, "order-1", order.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetByID(context.Background(), "tenant-1", "test_schema", "not-found")

		require.Error(t, err)
	})
}

func TestService_List(t *testing.T) {
	t.Run("returns orders for tenant", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", TenantID: "tenant-1"}
		repo.Orders["order-2"] = &Order{ID: "order-2", TenantID: "tenant-1"}
		svc := NewServiceWithRepository(repo)

		orders, err := svc.List(context.Background(), "tenant-1", "test_schema", nil)

		require.NoError(t, err)
		assert.Len(t, orders, 2)
	})

	t.Run("returns error on repository failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		_, err := svc.List(context.Background(), "tenant-1", "test_schema", nil)

		require.Error(t, err)
	})
}

func TestService_Update(t *testing.T) {
	t.Run("updates pending order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{
			ID:       "order-1",
			TenantID: "tenant-1",
			Status:   OrderStatusPending,
		}
		svc := NewServiceWithRepository(repo)

		req := &UpdateOrderRequest{
			ContactID: "contact-2",
			OrderDate: time.Now(),
			Lines: []CreateOrderLineRequest{
				{Description: "Updated", Quantity: decimal.NewFromInt(3), UnitPrice: decimal.NewFromFloat(50)},
			},
		}

		order, err := svc.Update(context.Background(), "tenant-1", "test_schema", "order-1", req)

		require.NoError(t, err)
		assert.Equal(t, "contact-2", order.ContactID)
		assert.Len(t, order.Lines, 1)
	})

	t.Run("updates confirmed order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{
			ID:       "order-1",
			TenantID: "tenant-1",
			Status:   OrderStatusConfirmed,
		}
		svc := NewServiceWithRepository(repo)

		req := &UpdateOrderRequest{
			ContactID: "contact-2",
			OrderDate: time.Now(),
			Lines: []CreateOrderLineRequest{
				{Description: "Updated", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Update(context.Background(), "tenant-1", "test_schema", "order-1", req)

		require.NoError(t, err)
	})

	t.Run("returns error when updating shipped order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{
			ID:       "order-1",
			TenantID: "tenant-1",
			Status:   OrderStatusShipped,
		}
		svc := NewServiceWithRepository(repo)

		req := &UpdateOrderRequest{
			ContactID: "contact-2",
			OrderDate: time.Now(),
			Lines: []CreateOrderLineRequest{
				{Description: "Updated", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromFloat(10)},
			},
		}

		_, err := svc.Update(context.Background(), "tenant-1", "test_schema", "order-1", req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "only pending or confirmed orders can be updated")
	})
}

func TestService_Confirm(t *testing.T) {
	t.Run("confirms pending order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusPending}
		svc := NewServiceWithRepository(repo)

		err := svc.Confirm(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusConfirmed, repo.Orders["order-1"].Status)
	})

	t.Run("returns error when not pending", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusConfirmed}
		svc := NewServiceWithRepository(repo)

		err := svc.Confirm(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not in pending status")
	})
}

func TestService_Process(t *testing.T) {
	t.Run("processes confirmed order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusConfirmed}
		svc := NewServiceWithRepository(repo)

		err := svc.Process(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusProcessing, repo.Orders["order-1"].Status)
	})

	t.Run("returns error when not confirmed", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusPending}
		svc := NewServiceWithRepository(repo)

		err := svc.Process(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be confirmed before processing")
	})
}

func TestService_Ship(t *testing.T) {
	t.Run("ships processing order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusProcessing}
		svc := NewServiceWithRepository(repo)

		err := svc.Ship(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusShipped, repo.Orders["order-1"].Status)
	})

	t.Run("ships confirmed order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusConfirmed}
		svc := NewServiceWithRepository(repo)

		err := svc.Ship(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusShipped, repo.Orders["order-1"].Status)
	})

	t.Run("returns error when pending", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusPending}
		svc := NewServiceWithRepository(repo)

		err := svc.Ship(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be shipped")
	})
}

func TestService_Deliver(t *testing.T) {
	t.Run("delivers shipped order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusShipped}
		svc := NewServiceWithRepository(repo)

		err := svc.Deliver(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusDelivered, repo.Orders["order-1"].Status)
	})

	t.Run("returns error when not shipped", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusProcessing}
		svc := NewServiceWithRepository(repo)

		err := svc.Deliver(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be shipped before delivery")
	})
}

func TestService_Cancel(t *testing.T) {
	t.Run("cancels pending order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusPending}
		svc := NewServiceWithRepository(repo)

		err := svc.Cancel(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusCanceled, repo.Orders["order-1"].Status)
	})

	t.Run("cancels shipped order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusShipped}
		svc := NewServiceWithRepository(repo)

		err := svc.Cancel(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Equal(t, OrderStatusCanceled, repo.Orders["order-1"].Status)
	})

	t.Run("returns error when already delivered", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusDelivered}
		svc := NewServiceWithRepository(repo)

		err := svc.Cancel(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be canceled")
	})

	t.Run("returns error when already canceled", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1", Status: OrderStatusCanceled}
		svc := NewServiceWithRepository(repo)

		err := svc.Cancel(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be canceled")
	})
}

func TestService_Delete(t *testing.T) {
	t.Run("deletes order", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1"}
		svc := NewServiceWithRepository(repo)

		err := svc.Delete(context.Background(), "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		assert.Empty(t, repo.Orders)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.DeleteErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		err := svc.Delete(context.Background(), "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
	})
}

func TestService_ConvertToInvoice(t *testing.T) {
	t.Run("marks order as converted", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{ID: "order-1"}
		svc := NewServiceWithRepository(repo)

		err := svc.ConvertToInvoice(context.Background(), "tenant-1", "test_schema", "order-1", "invoice-1")

		require.NoError(t, err)
		assert.Equal(t, "invoice-1", *repo.Orders["order-1"].ConvertedToInvoiceID)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ConvertErr = errors.New("db error")
		svc := NewServiceWithRepository(repo)

		err := svc.ConvertToInvoice(context.Background(), "tenant-1", "test_schema", "order-1", "invoice-1")

		require.Error(t, err)
	})
}

func TestService_StockReservations(t *testing.T) {
	ctx := context.Background()

	t.Run("lists reservations", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockReservations[orderStockReservationKey("order-1", "product-1", "warehouse-1")] = &OrderStockReservation{
			ID:          "reservation-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   "product-1",
			WarehouseID: "warehouse-1",
			Quantity:    decimal.NewFromInt(3),
			Status:      OrderStockReservationStatusReserved,
		}
		svc := NewServiceWithRepository(repo)

		reservations, err := svc.ListStockReservations(ctx, "tenant-1", "test_schema", "order-1")

		require.NoError(t, err)
		require.Len(t, reservations, 1)
		assert.Equal(t, "reservation-1", reservations[0].ID)
	})

	t.Run("wraps list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockListErr = errors.New("list failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.ListStockReservations(ctx, "tenant-1", "test_schema", "order-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "list order stock reservations")
	})

	t.Run("gets reservation", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockReservations[orderStockReservationKey("order-1", "product-1", "warehouse-1")] = &OrderStockReservation{
			ID:          "reservation-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   "product-1",
			WarehouseID: "warehouse-1",
			Quantity:    decimal.NewFromInt(2),
			Status:      OrderStockReservationStatusReserved,
		}
		svc := NewServiceWithRepository(repo)

		reservation, err := svc.GetStockReservation(ctx, "tenant-1", "test_schema", "order-1", "product-1", "warehouse-1")

		require.NoError(t, err)
		assert.Equal(t, "reservation-1", reservation.ID)
		assert.True(t, reservation.Quantity.Equal(decimal.NewFromInt(2)))
	})

	t.Run("wraps get errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockGetErr = errors.New("get failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetStockReservation(ctx, "tenant-1", "test_schema", "order-1", "product-1", "warehouse-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get order stock reservation")
	})

	t.Run("upserts with defaults", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)
		reservation := &OrderStockReservation{
			OrderID:     "order-1",
			ProductID:   "product-1",
			WarehouseID: "warehouse-1",
			Quantity:    decimal.NewFromInt(4),
			Reason:      "sales order",
		}

		err := svc.UpsertStockReservation(ctx, "tenant-1", "test_schema", reservation)

		require.NoError(t, err)
		assert.NotEmpty(t, reservation.ID)
		assert.Equal(t, "tenant-1", reservation.TenantID)
		assert.Equal(t, OrderStockReservationStatusReserved, reservation.Status)
		stored := repo.StockReservations[orderStockReservationKey("order-1", "product-1", "warehouse-1")]
		require.NotNil(t, stored)
		assert.True(t, stored.Quantity.Equal(decimal.NewFromInt(4)))
	})

	t.Run("rejects nonpositive upsert quantity", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		err := svc.UpsertStockReservation(ctx, "tenant-1", "test_schema", &OrderStockReservation{
			OrderID:     "order-1",
			ProductID:   "product-1",
			WarehouseID: "warehouse-1",
			Quantity:    decimal.Zero,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reservation quantity must be positive")
	})

	t.Run("wraps upsert errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockUpsertErr = errors.New("upsert failed")
		svc := NewServiceWithRepository(repo)

		err := svc.UpsertStockReservation(ctx, "tenant-1", "test_schema", &OrderStockReservation{
			OrderID:     "order-1",
			ProductID:   "product-1",
			WarehouseID: "warehouse-1",
			Quantity:    decimal.NewFromInt(1),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "upsert order stock reservation")
	})

	t.Run("releases reservation", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockReservations[orderStockReservationKey("order-1", "product-1", "warehouse-1")] = &OrderStockReservation{
			ID:          "reservation-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   "product-1",
			WarehouseID: "warehouse-1",
			Quantity:    decimal.NewFromInt(5),
			Status:      OrderStockReservationStatusReserved,
		}
		svc := NewServiceWithRepository(repo)

		reservation, err := svc.ReleaseStockReservation(ctx, "tenant-1", "test_schema", "order-1", "product-1", "warehouse-1", decimal.NewFromInt(2), "picked", "user-1")

		require.NoError(t, err)
		assert.True(t, reservation.Quantity.Equal(decimal.NewFromInt(3)))
		assert.Equal(t, OrderStockReservationStatusReserved, reservation.Status)
		assert.Equal(t, "picked", reservation.Reason)
	})

	t.Run("rejects nonpositive release quantity", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		_, err := svc.ReleaseStockReservation(ctx, "tenant-1", "test_schema", "order-1", "product-1", "warehouse-1", decimal.Zero, "", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reservation quantity must be positive")
	})

	t.Run("wraps release errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.StockReleaseErr = errors.New("release failed")
		svc := NewServiceWithRepository(repo)

		_, err := svc.ReleaseStockReservation(ctx, "tenant-1", "test_schema", "order-1", "product-1", "warehouse-1", decimal.NewFromInt(1), "", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "release order stock reservation")
	})
}
