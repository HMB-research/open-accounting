package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// mockOrdersRepository implements orders.Repository for testing
type mockOrdersRepository struct {
	orders      map[string]*orders.Order
	orderNumber int
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	statusErr   error
	deleteErr   error
}

func newMockOrdersRepository() *mockOrdersRepository {
	return &mockOrdersRepository{
		orders:      make(map[string]*orders.Order),
		orderNumber: 1,
	}
}

func (m *mockOrdersRepository) Create(ctx context.Context, schemaName string, order *orders.Order) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrdersRepository) GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*orders.Order, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if o, ok := m.orders[orderID]; ok && o.TenantID == tenantID {
		return o, nil
	}
	return nil, orders.ErrOrderNotFound
}

func (m *mockOrdersRepository) List(ctx context.Context, schemaName, tenantID string, filter *orders.OrderFilter) ([]orders.Order, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []orders.Order
	for _, o := range m.orders {
		if o.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.Status != "" && o.Status != filter.Status {
				continue
			}
			if filter.ContactID != "" && o.ContactID != filter.ContactID {
				continue
			}
		}
		result = append(result, *o)
	}
	return result, nil
}

func (m *mockOrdersRepository) Update(ctx context.Context, schemaName string, order *orders.Order) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrdersRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status orders.OrderStatus) error {
	if m.statusErr != nil {
		return m.statusErr
	}
	if o, ok := m.orders[orderID]; ok && o.TenantID == tenantID {
		o.Status = status
		return nil
	}
	return orders.ErrOrderNotFound
}

func (m *mockOrdersRepository) Delete(ctx context.Context, schemaName, tenantID, orderID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.orders[orderID]; !ok {
		return orders.ErrOrderNotFound
	}
	delete(m.orders, orderID)
	return nil
}

func (m *mockOrdersRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	num := m.orderNumber
	m.orderNumber++
	return "ORD-" + string(rune('0'+num)), nil
}

func (m *mockOrdersRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error {
	if o, ok := m.orders[orderID]; ok && o.TenantID == tenantID {
		o.ConvertedToInvoiceID = &invoiceID
		return nil
	}
	return orders.ErrOrderNotFound
}

func setupOrdersTestHandlers() (*Handlers, *mockOrdersRepository, *mockTenantRepository) {
	ordersRepo := newMockOrdersRepository()
	ordersSvc := orders.NewServiceWithRepository(ordersRepo)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		ordersService: ordersSvc,
		tenantService: tenantSvc,
	}
	return h, ordersRepo, tenantRepo
}

func TestListOrders(t *testing.T) {
	h, repo, tenantRepo := setupOrdersTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.orders["order-1"] = &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-001",
		ContactID:   "contact-1",
		Status:      orders.OrderStatusPending,
		Total:       decimal.NewFromInt(1000),
		Lines:       []orders.OrderLine{},
	}
	repo.orders["order-2"] = &orders.Order{
		ID:          "order-2",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-002",
		ContactID:   "contact-2",
		Status:      orders.OrderStatusConfirmed,
		Total:       decimal.NewFromInt(2000),
		Lines:       []orders.OrderLine{},
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all orders",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "filter by status PENDING",
			query:      "?status=PENDING",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "filter by contact",
			query:      "?contact_id=contact-1",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListOrders(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []orders.Order
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid order",
			body: map[string]interface{}{
				"contact_id": "contact-1",
				"order_date": "2026-01-15T00:00:00Z",
				"currency":   "EUR",
				"lines": []map[string]interface{}{
					{
						"description": "Product A",
						"quantity":    "10",
						"unit_price":  "50.00",
						"vat_rate":    "20",
					},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing contact_id",
			body: map[string]interface{}{
				"order_date": "2026-01-15T00:00:00Z",
				"lines": []map[string]interface{}{
					{
						"description": "Product A",
						"quantity":    "10",
						"unit_price":  "50.00",
						"vat_rate":    "20",
					},
				},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "Contact",
		},
		{
			name: "missing lines",
			body: map[string]interface{}{
				"contact_id": "contact-1",
				"order_date": "2026-01-15T00:00:00Z",
				"lines":      []map[string]interface{}{},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "line",
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, tenantRepo := setupOrdersTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("{invalid")
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/orders", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreateOrder(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}

			if tt.wantStatus == http.StatusCreated {
				var result orders.Order
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.NotEmpty(t, result.OrderNumber)
				assert.Equal(t, orders.OrderStatusPending, result.Status)
			}
		})
	}
}

func TestGetOrder(t *testing.T) {
	h, repo, tenantRepo := setupOrdersTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.orders["order-1"] = &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-001",
		ContactID:   "contact-1",
		Status:      orders.OrderStatusPending,
		Lines:       []orders.OrderLine{},
	}

	tests := []struct {
		name       string
		orderID    string
		wantStatus int
	}{
		{
			name:       "existing order",
			orderID:    "order-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent order",
			orderID:    "order-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/"+tt.orderID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": tt.orderID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetOrder(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result orders.Order
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, tt.orderID, result.ID)
			}
		})
	}
}

func TestUpdateOrder(t *testing.T) {
	h, repo, tenantRepo := setupOrdersTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.orders["order-1"] = &orders.Order{
		ID:          "order-1",
		TenantID:    "tenant-1",
		OrderNumber: "ORD-001",
		ContactID:   "contact-1",
		Status:      orders.OrderStatusPending,
		OrderDate:   time.Now(),
		Lines: []orders.OrderLine{
			{ID: "line-1", Description: "Original", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	body := map[string]interface{}{
		"contact_id": "contact-1",
		"order_date": "2026-01-20T00:00:00Z",
		"lines": []map[string]interface{}{
			{
				"description": "Updated Product",
				"quantity":    "20",
				"unit_price":  "100.00",
				"vat_rate":    "20",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/orders/order-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.UpdateOrder(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteOrder(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mockOrdersRepository)
		orderID    string
		wantStatus int
	}{
		{
			name: "delete pending order",
			setupRepo: func(repo *mockOrdersRepository) {
				repo.orders["order-1"] = &orders.Order{
					ID:       "order-1",
					TenantID: "tenant-1",
					Status:   orders.OrderStatusPending,
					Lines:    []orders.OrderLine{},
				}
			},
			orderID:    "order-1",
			wantStatus: http.StatusNoContent,
		},
		{
			name: "non-existent order",
			setupRepo: func(repo *mockOrdersRepository) {
				// no setup
			},
			orderID:    "order-999",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupOrdersTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			req := httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/orders/"+tt.orderID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": tt.orderID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.DeleteOrder(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestOrderStatusTransitions(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  orders.OrderStatus
		handler        string
		expectedStatus orders.OrderStatus
		wantStatus     int
	}{
		{
			name:           "confirm pending order",
			initialStatus:  orders.OrderStatusPending,
			handler:        "confirm",
			expectedStatus: orders.OrderStatusConfirmed,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "process confirmed order",
			initialStatus:  orders.OrderStatusConfirmed,
			handler:        "process",
			expectedStatus: orders.OrderStatusProcessing,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "ship processing order",
			initialStatus:  orders.OrderStatusProcessing,
			handler:        "ship",
			expectedStatus: orders.OrderStatusShipped,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "deliver shipped order",
			initialStatus:  orders.OrderStatusShipped,
			handler:        "deliver",
			expectedStatus: orders.OrderStatusDelivered,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "cancel pending order",
			initialStatus:  orders.OrderStatusPending,
			handler:        "cancel",
			expectedStatus: orders.OrderStatusCanceled,
			wantStatus:     http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupOrdersTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			repo.orders["order-1"] = &orders.Order{
				ID:       "order-1",
				TenantID: "tenant-1",
				Status:   tt.initialStatus,
				Lines:    []orders.OrderLine{},
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/orders/order-1/"+tt.handler, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()

			switch tt.handler {
			case "confirm":
				h.ConfirmOrder(rr, req)
			case "process":
				h.ProcessOrder(rr, req)
			case "ship":
				h.ShipOrder(rr, req)
			case "deliver":
				h.DeliverOrder(rr, req)
			case "cancel":
				h.CancelOrder(rr, req)
			}

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				updatedOrder := repo.orders["order-1"]
				assert.Equal(t, tt.expectedStatus, updatedOrder.Status)
			}
		})
	}
}
