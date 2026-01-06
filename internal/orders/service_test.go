package orders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct {
	orders        map[string]*Order
	nextNumber    int
	CreateErr     error
	GetByIDErr    error
	ListErr       error
	UpdateErr     error
	UpdateStatErr error
	DeleteErr     error
	GenNumberErr  error
	SetConvErr    error
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		orders:     make(map[string]*Order),
		nextNumber: 1,
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, order *Order) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.orders[order.ID] = order
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*Order, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	order, ok := m.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *OrderFilter) ([]Order, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	var result []Order
	for _, order := range m.orders {
		if order.TenantID == tenantID {
			result = append(result, *order)
		}
	}
	return result, nil
}

func (m *MockRepository) Update(ctx context.Context, schemaName string, order *Order) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	m.orders[order.ID] = order
	return nil
}

func (m *MockRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status OrderStatus) error {
	if m.UpdateStatErr != nil {
		return m.UpdateStatErr
	}
	if order, ok := m.orders[orderID]; ok {
		order.Status = status
	}
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, schemaName, tenantID, orderID string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	delete(m.orders, orderID)
	return nil
}

func (m *MockRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	if m.GenNumberErr != nil {
		return "", m.GenNumberErr
	}
	num := m.nextNumber
	m.nextNumber++
	return "ORD-2024-" + string(rune('0'+num)), nil
}

func (m *MockRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error {
	if m.SetConvErr != nil {
		return m.SetConvErr
	}
	if order, ok := m.orders[orderID]; ok {
		order.ConvertedToInvoiceID = &invoiceID
		order.Status = OrderStatusDelivered
	}
	return nil
}

// AddOrder adds an order to the mock repository for testing
func (m *MockRepository) AddOrder(order *Order) {
	m.orders[order.ID] = order
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	require.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
}

func TestService_Create_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	req := &CreateOrderRequest{
		ContactID: "contact-1",
		OrderDate: time.Now(),
		Currency:  "EUR",
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(2),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
		UserID: "user-1",
	}

	order, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	require.NotNil(t, order)

	assert.NotEmpty(t, order.ID)
	assert.Equal(t, "tenant-1", order.TenantID)
	assert.Equal(t, "contact-1", order.ContactID)
	assert.Equal(t, OrderStatusPending, order.Status)
	assert.NotEmpty(t, order.OrderNumber)
	assert.Len(t, order.Lines, 1)
}

func TestService_Create_DefaultValues(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	req := &CreateOrderRequest{
		ContactID: "contact-1",
		// No currency, no exchange rate, no order date
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
		UserID: "user-1",
	}

	order, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)

	assert.Equal(t, "EUR", order.Currency)
	assert.True(t, order.ExchangeRate.Equal(decimal.NewFromInt(1)))
	assert.False(t, order.OrderDate.IsZero())
}

func TestService_Create_ValidationError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	req := &CreateOrderRequest{
		ContactID: "", // Missing contact
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	order, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestService_Create_GenerateNumberError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GenNumberErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	req := &CreateOrderRequest{
		ContactID: "contact-1",
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	order, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "generate order number")
}

func TestService_Create_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.CreateErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	req := &CreateOrderRequest{
		ContactID: "contact-1",
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product A",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(50.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	order, err := svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "create order")
}

func TestService_GetByID_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingOrder := &Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusPending,
	}
	repo.AddOrder(existingOrder)

	order, err := svc.GetByID(ctx, "tenant-1", "test_schema", "order-1")
	require.NoError(t, err)
	assert.Equal(t, "order-1", order.ID)
}

func TestService_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	order, err := svc.GetByID(ctx, "tenant-1", "test_schema", "nonexistent")
	require.Error(t, err)
	assert.Nil(t, order)
}

func TestService_GetByID_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetByIDErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	order, err := svc.GetByID(ctx, "tenant-1", "test_schema", "order-1")
	require.Error(t, err)
	assert.Nil(t, order)
	assert.Contains(t, err.Error(), "get order")
}

func TestService_List_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{ID: "order-1", TenantID: "tenant-1"})
	repo.AddOrder(&Order{ID: "order-2", TenantID: "tenant-1"})
	repo.AddOrder(&Order{ID: "order-3", TenantID: "tenant-2"}) // Different tenant

	orders, err := svc.List(ctx, "tenant-1", "test_schema", nil)
	require.NoError(t, err)
	assert.Len(t, orders, 2)
}

func TestService_List_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.ListErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	orders, err := svc.List(ctx, "tenant-1", "test_schema", nil)
	require.Error(t, err)
	assert.Nil(t, orders)
	assert.Contains(t, err.Error(), "list orders")
}

func TestService_Update_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingOrder := &Order{
		ID:        "order-1",
		TenantID:  "tenant-1",
		ContactID: "contact-1",
		Status:    OrderStatusPending,
		Lines: []OrderLine{
			{
				ID:          "line-1",
				Description: "Old Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
			},
		},
	}
	repo.AddOrder(existingOrder)

	req := &UpdateOrderRequest{
		ContactID: "contact-2",
		OrderDate: time.Now(),
		Currency:  "USD",
		Lines: []CreateOrderLineRequest{
			{
				Description: "New Product",
				Quantity:    decimal.NewFromInt(3),
				UnitPrice:   decimal.NewFromFloat(150.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	order, err := svc.Update(ctx, "tenant-1", "test_schema", "order-1", req)
	require.NoError(t, err)
	assert.Equal(t, "contact-2", order.ContactID)
	assert.Equal(t, "USD", order.Currency)
	assert.Len(t, order.Lines, 1)
	assert.Equal(t, "New Product", order.Lines[0].Description)
}

func TestService_Update_OnlyPendingOrConfirmed(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		status  OrderStatus
		wantErr bool
	}{
		{"pending - can update", OrderStatusPending, false},
		{"confirmed - can update", OrderStatusConfirmed, false},
		{"processing - cannot update", OrderStatusProcessing, true},
		{"shipped - cannot update", OrderStatusShipped, true},
		{"delivered - cannot update", OrderStatusDelivered, true},
		{"canceled - cannot update", OrderStatusCanceled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			existingOrder := &Order{
				ID:        "order-1",
				TenantID:  "tenant-1",
				ContactID: "contact-1",
				Status:    tt.status,
			}
			repo.AddOrder(existingOrder)

			req := &UpdateOrderRequest{
				ContactID: "contact-2",
				OrderDate: time.Now(),
				Lines: []CreateOrderLineRequest{
					{
						Description: "Product",
						Quantity:    decimal.NewFromInt(1),
						UnitPrice:   decimal.NewFromFloat(100.00),
						VATRate:     decimal.NewFromInt(22),
					},
				},
			}

			_, err := svc.Update(ctx, "tenant-1", "test_schema", "order-1", req)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "only pending or confirmed")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_Update_DefaultValues(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingOrder := &Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusPending,
	}
	repo.AddOrder(existingOrder)

	req := &UpdateOrderRequest{
		ContactID: "contact-1",
		OrderDate: time.Now(),
		Currency:  "", // Should default to EUR
		// ExchangeRate is zero - should default to 1
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	order, err := svc.Update(ctx, "tenant-1", "test_schema", "order-1", req)
	require.NoError(t, err)
	assert.Equal(t, "EUR", order.Currency)
	assert.True(t, order.ExchangeRate.Equal(decimal.NewFromInt(1)))
}

func TestService_Confirm_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusPending,
	})

	err := svc.Confirm(ctx, "tenant-1", "test_schema", "order-1")
	require.NoError(t, err)
	assert.Equal(t, OrderStatusConfirmed, repo.orders["order-1"].Status)
}

func TestService_Confirm_NotPending(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusConfirmed, // Already confirmed
	})

	err := svc.Confirm(ctx, "tenant-1", "test_schema", "order-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in pending status")
}

func TestService_Process_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusConfirmed,
	})

	err := svc.Process(ctx, "tenant-1", "test_schema", "order-1")
	require.NoError(t, err)
	assert.Equal(t, OrderStatusProcessing, repo.orders["order-1"].Status)
}

func TestService_Process_NotConfirmed(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusPending, // Not confirmed yet
	})

	err := svc.Process(ctx, "tenant-1", "test_schema", "order-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be confirmed")
}

func TestService_Ship_Success(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		status OrderStatus
	}{
		{"from processing", OrderStatusProcessing},
		{"from confirmed", OrderStatusConfirmed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			repo.AddOrder(&Order{
				ID:       "order-1",
				TenantID: "tenant-1",
				Status:   tt.status,
			})

			err := svc.Ship(ctx, "tenant-1", "test_schema", "order-1")
			require.NoError(t, err)
			assert.Equal(t, OrderStatusShipped, repo.orders["order-1"].Status)
		})
	}
}

func TestService_Ship_InvalidStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusPending, // Cannot ship from pending
	})

	err := svc.Ship(ctx, "tenant-1", "test_schema", "order-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be shipped")
}

func TestService_Deliver_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusShipped,
	})

	err := svc.Deliver(ctx, "tenant-1", "test_schema", "order-1")
	require.NoError(t, err)
	assert.Equal(t, OrderStatusDelivered, repo.orders["order-1"].Status)
}

func TestService_Deliver_NotShipped(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusProcessing, // Not shipped
	})

	err := svc.Deliver(ctx, "tenant-1", "test_schema", "order-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be shipped")
}

func TestService_Cancel_Success(t *testing.T) {
	ctx := context.Background()

	statuses := []OrderStatus{
		OrderStatusPending,
		OrderStatusConfirmed,
		OrderStatusProcessing,
		OrderStatusShipped,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			repo.AddOrder(&Order{
				ID:       "order-1",
				TenantID: "tenant-1",
				Status:   status,
			})

			err := svc.Cancel(ctx, "tenant-1", "test_schema", "order-1")
			require.NoError(t, err)
			assert.Equal(t, OrderStatusCanceled, repo.orders["order-1"].Status)
		})
	}
}

func TestService_Cancel_AlreadyDeliveredOrCanceled(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		status OrderStatus
	}{
		{"already delivered", OrderStatusDelivered},
		{"already canceled", OrderStatusCanceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			svc := NewServiceWithRepository(repo)

			repo.AddOrder(&Order{
				ID:       "order-1",
				TenantID: "tenant-1",
				Status:   tt.status,
			})

			err := svc.Cancel(ctx, "tenant-1", "test_schema", "order-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be canceled")
		})
	}
}

func TestService_Delete_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusPending,
	})

	err := svc.Delete(ctx, "tenant-1", "test_schema", "order-1")
	require.NoError(t, err)
	_, exists := repo.orders["order-1"]
	assert.False(t, exists)
}

func TestService_Delete_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.DeleteErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.Delete(ctx, "tenant-1", "test_schema", "order-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete order")
}

func TestService_ConvertToInvoice_Success(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	repo.AddOrder(&Order{
		ID:       "order-1",
		TenantID: "tenant-1",
		Status:   OrderStatusDelivered,
	})

	err := svc.ConvertToInvoice(ctx, "tenant-1", "test_schema", "order-1", "invoice-1")
	require.NoError(t, err)
	assert.NotNil(t, repo.orders["order-1"].ConvertedToInvoiceID)
	assert.Equal(t, "invoice-1", *repo.orders["order-1"].ConvertedToInvoiceID)
}

func TestService_ConvertToInvoice_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.SetConvErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	err := svc.ConvertToInvoice(ctx, "tenant-1", "test_schema", "order-1", "invoice-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to invoice")
}

func TestService_StatusTransitions_Errors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		method func(*Service, context.Context, string, string, string) error
		status OrderStatus
	}{
		{"confirm not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Confirm(ctx, t, sc, id)
		}, OrderStatusPending},
		{"process not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Process(ctx, t, sc, id)
		}, OrderStatusConfirmed},
		{"ship not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Ship(ctx, t, sc, id)
		}, OrderStatusProcessing},
		{"deliver not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Deliver(ctx, t, sc, id)
		}, OrderStatusShipped},
		{"cancel not found", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Cancel(ctx, t, sc, id)
		}, OrderStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			repo.GetByIDErr = errors.New("not found")
			svc := NewServiceWithRepository(repo)

			err := tt.method(svc, ctx, "tenant-1", "test_schema", "order-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "get order")
		})
	}
}

func TestService_Update_GetByIDError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetByIDErr = errors.New("not found")
	svc := NewServiceWithRepository(repo)

	req := &UpdateOrderRequest{
		ContactID: "contact-1",
		OrderDate: time.Now(),
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	_, err := svc.Update(ctx, "tenant-1", "test_schema", "order-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get order")
}

func TestService_Update_ValidationError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)

	existingOrder := &Order{
		ID:        "order-1",
		TenantID:  "tenant-1",
		ContactID: "contact-1",
		Status:    OrderStatusPending,
	}
	repo.AddOrder(existingOrder)

	req := &UpdateOrderRequest{
		ContactID: "", // Missing contact - validation error
		OrderDate: time.Now(),
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	_, err := svc.Update(ctx, "tenant-1", "test_schema", "order-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestService_Update_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.UpdateErr = errors.New("database error")
	svc := NewServiceWithRepository(repo)

	existingOrder := &Order{
		ID:        "order-1",
		TenantID:  "tenant-1",
		ContactID: "contact-1",
		Status:    OrderStatusPending,
	}
	repo.AddOrder(existingOrder)

	req := &UpdateOrderRequest{
		ContactID: "contact-2",
		OrderDate: time.Now(),
		Lines: []CreateOrderLineRequest{
			{
				Description: "Product",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(100.00),
				VATRate:     decimal.NewFromInt(22),
			},
		},
	}

	_, err := svc.Update(ctx, "tenant-1", "test_schema", "order-1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update order")
}

func TestService_StatusTransitions_UpdateErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		method func(*Service, context.Context, string, string, string) error
		status OrderStatus
	}{
		{"confirm update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Confirm(ctx, t, sc, id)
		}, OrderStatusPending},
		{"process update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Process(ctx, t, sc, id)
		}, OrderStatusConfirmed},
		{"ship update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Ship(ctx, t, sc, id)
		}, OrderStatusProcessing},
		{"deliver update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Deliver(ctx, t, sc, id)
		}, OrderStatusShipped},
		{"cancel update error", func(s *Service, ctx context.Context, t, sc, id string) error {
			return s.Cancel(ctx, t, sc, id)
		}, OrderStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			repo.UpdateStatErr = errors.New("database error")
			repo.AddOrder(&Order{
				ID:       "order-1",
				TenantID: "tenant-1",
				Status:   tt.status,
			})
			svc := NewServiceWithRepository(repo)

			err := tt.method(svc, ctx, "tenant-1", "test_schema", "order-1")
			require.Error(t, err)
		})
	}
}

// TestNewService tests the NewService constructor with a nil pool
func TestNewService(t *testing.T) {
	// NewService should create a service with nil pool (won't panic until used)
	svc := NewService(nil)
	require.NotNil(t, svc)
	assert.Nil(t, svc.db)
	assert.NotNil(t, svc.repo)
}
