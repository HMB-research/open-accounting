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

// MockRepository implements Repository for testing
type MockRepository struct {
	Orders        map[string]*Order
	NextNumber    string
	GenerateErr   error
	CreateErr     error
	GetErr        error
	ListErr       error
	UpdateErr     error
	UpdateStatErr error
	DeleteErr     error
	ConvertErr    error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		Orders:     make(map[string]*Order),
		NextNumber: "ORD-00001",
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
