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

	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// mockPaymentsRepository implements payments.Repository for testing
type mockPaymentsRepository struct {
	payments      map[string]*payments.Payment
	allocations   map[string][]payments.PaymentAllocation
	paymentNumber int
	createErr     error
	getErr        error
	listErr       error
	allocErr      error
	unallocErr    error
}

func newMockPaymentsRepository() *mockPaymentsRepository {
	return &mockPaymentsRepository{
		payments:      make(map[string]*payments.Payment),
		allocations:   make(map[string][]payments.PaymentAllocation),
		paymentNumber: 1,
	}
}

func (m *mockPaymentsRepository) Create(ctx context.Context, schemaName string, payment *payments.Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.payments[payment.ID] = payment
	return nil
}

func (m *mockPaymentsRepository) GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*payments.Payment, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if p, ok := m.payments[paymentID]; ok && p.TenantID == tenantID {
		return p, nil
	}
	return nil, payments.ErrPaymentNotFound
}

func (m *mockPaymentsRepository) List(ctx context.Context, schemaName, tenantID string, filter *payments.PaymentFilter) ([]payments.Payment, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []payments.Payment
	for _, p := range m.payments {
		if p.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.PaymentType != "" && p.PaymentType != filter.PaymentType {
				continue
			}
			if filter.ContactID != "" && (p.ContactID == nil || *p.ContactID != filter.ContactID) {
				continue
			}
		}
		result = append(result, *p)
	}
	return result, nil
}

func (m *mockPaymentsRepository) CreateAllocation(ctx context.Context, schemaName string, allocation *payments.PaymentAllocation) error {
	if m.allocErr != nil {
		return m.allocErr
	}
	m.allocations[allocation.PaymentID] = append(m.allocations[allocation.PaymentID], *allocation)
	return nil
}

func (m *mockPaymentsRepository) GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]payments.PaymentAllocation, error) {
	return m.allocations[paymentID], nil
}

func (m *mockPaymentsRepository) GetNextPaymentNumber(ctx context.Context, schemaName, tenantID string, paymentType payments.PaymentType) (int, error) {
	num := m.paymentNumber
	m.paymentNumber++
	return num, nil
}

func (m *mockPaymentsRepository) GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType payments.PaymentType) ([]payments.Payment, error) {
	if m.unallocErr != nil {
		return nil, m.unallocErr
	}
	var result []payments.Payment
	for _, p := range m.payments {
		if p.TenantID != tenantID || p.PaymentType != paymentType {
			continue
		}
		// Check if fully allocated
		allocs := m.allocations[p.ID]
		totalAllocated := decimal.Zero
		for _, a := range allocs {
			totalAllocated = totalAllocated.Add(a.Amount)
		}
		if p.Amount.GreaterThan(totalAllocated) {
			result = append(result, *p)
		}
	}
	return result, nil
}

// mockInvoiceServiceForPayments implements payments.InvoiceService
type mockInvoiceServiceForPayments struct {
	recordPaymentErr error
}

func (m *mockInvoiceServiceForPayments) RecordPayment(ctx context.Context, tenantID, schemaName, invoiceID string, amount decimal.Decimal) error {
	return m.recordPaymentErr
}

func setupPaymentTestHandlers() (*Handlers, *mockPaymentsRepository, *mockTenantRepository) {
	paymentsRepo := newMockPaymentsRepository()
	invoiceSvc := &mockInvoiceServiceForPayments{}
	paymentsSvc := payments.NewServiceWithRepository(paymentsRepo, invoiceSvc)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		paymentsService: paymentsSvc,
		tenantService:   tenantSvc,
	}
	return h, paymentsRepo, tenantRepo
}

func TestListPayments(t *testing.T) {
	h, repo, tenantRepo := setupPaymentTestHandlers()

	// Setup tenant
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	contactID := "contact-1"
	// Add some payments
	repo.payments["payment-1"] = &payments.Payment{
		ID:            "payment-1",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-00001",
		PaymentType:   payments.PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.NewFromInt(100),
		PaymentDate:   time.Now(),
	}
	repo.payments["payment-2"] = &payments.Payment{
		ID:            "payment-2",
		TenantID:      "tenant-1",
		PaymentNumber: "OUT-00001",
		PaymentType:   payments.PaymentTypeMade,
		Amount:        decimal.NewFromInt(50),
		PaymentDate:   time.Now(),
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all payments",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "filter by type RECEIVED",
			query:      "?type=RECEIVED",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "filter by type MADE",
			query:      "?type=MADE",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "filter by contact_id",
			query:      "?contact_id=contact-1",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/payments"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListPayments(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []payments.Payment
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestCreatePayment(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid received payment",
			body: map[string]interface{}{
				"payment_type": "RECEIVED",
				"amount":       "100.00",
				"payment_date": "2026-01-15T00:00:00Z",
				"currency":     "EUR",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid made payment",
			body: map[string]interface{}{
				"payment_type": "MADE",
				"amount":       "50.00",
				"payment_date": "2026-01-15T00:00:00Z",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "payment with allocations",
			body: map[string]interface{}{
				"payment_type": "RECEIVED",
				"amount":       "100.00",
				"payment_date": "2026-01-15T00:00:00Z",
				"allocations": []map[string]interface{}{
					{"invoice_id": "inv-1", "amount": "50.00"},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "zero amount rejected",
			body: map[string]interface{}{
				"payment_type": "RECEIVED",
				"amount":       "0",
				"payment_date": "2026-01-15T00:00:00Z",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "positive",
		},
		{
			name: "negative amount rejected",
			body: map[string]interface{}{
				"payment_type": "RECEIVED",
				"amount":       "-50.00",
				"payment_date": "2026-01-15T00:00:00Z",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "positive",
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
			h, _, tenantRepo := setupPaymentTestHandlers()

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

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/payments", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreatePayment(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}

			if tt.wantStatus == http.StatusCreated {
				var result payments.Payment
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.NotEmpty(t, result.PaymentNumber)
			}
		})
	}
}

func TestGetPayment(t *testing.T) {
	h, repo, tenantRepo := setupPaymentTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.payments["payment-1"] = &payments.Payment{
		ID:            "payment-1",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-00001",
		PaymentType:   payments.PaymentTypeReceived,
		Amount:        decimal.NewFromInt(100),
		PaymentDate:   time.Now(),
	}

	tests := []struct {
		name       string
		paymentID  string
		wantStatus int
	}{
		{
			name:       "existing payment",
			paymentID:  "payment-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent payment",
			paymentID:  "payment-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/payments/"+tt.paymentID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "paymentID": tt.paymentID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetPayment(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result payments.Payment
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, tt.paymentID, result.ID)
			}
		})
	}
}

func TestAllocatePayment(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mockPaymentsRepository)
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid allocation",
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:          "payment-1",
					TenantID:    "tenant-1",
					Amount:      decimal.NewFromInt(100),
					PaymentType: payments.PaymentTypeReceived,
				}
			},
			body: map[string]interface{}{
				"invoice_id": "inv-1",
				"amount":     "50.00",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing invoice_id",
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:       "payment-1",
					TenantID: "tenant-1",
					Amount:   decimal.NewFromInt(100),
				}
			},
			body: map[string]interface{}{
				"amount": "50.00",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invoice ID",
		},
		{
			name: "zero amount",
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:       "payment-1",
					TenantID: "tenant-1",
					Amount:   decimal.NewFromInt(100),
				}
			},
			body: map[string]interface{}{
				"invoice_id": "inv-1",
				"amount":     "0",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "positive",
		},
		{
			name: "amount exceeds unallocated",
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:          "payment-1",
					TenantID:    "tenant-1",
					Amount:      decimal.NewFromInt(100),
					PaymentType: payments.PaymentTypeReceived,
				}
				repo.allocations["payment-1"] = []payments.PaymentAllocation{
					{Amount: decimal.NewFromInt(80)},
				}
			},
			body: map[string]interface{}{
				"invoice_id": "inv-1",
				"amount":     "50.00",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "exceeds",
		},
		{
			name: "invalid JSON",
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:       "payment-1",
					TenantID: "tenant-1",
					Amount:   decimal.NewFromInt(100),
				}
			},
			body:       nil,
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupPaymentTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("{invalid")
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/payments/payment-1/allocate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "paymentID": "payment-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.AllocatePayment(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}
		})
	}
}

func TestGetUnallocatedPayments(t *testing.T) {
	h, repo, tenantRepo := setupPaymentTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	// Add payments with different allocation states
	repo.payments["payment-1"] = &payments.Payment{
		ID:          "payment-1",
		TenantID:    "tenant-1",
		PaymentType: payments.PaymentTypeReceived,
		Amount:      decimal.NewFromInt(100),
	}
	repo.payments["payment-2"] = &payments.Payment{
		ID:          "payment-2",
		TenantID:    "tenant-1",
		PaymentType: payments.PaymentTypeReceived,
		Amount:      decimal.NewFromInt(50),
	}
	// Fully allocate payment-2
	repo.allocations["payment-2"] = []payments.PaymentAllocation{
		{PaymentID: "payment-2", Amount: decimal.NewFromInt(50)},
	}
	repo.payments["payment-3"] = &payments.Payment{
		ID:          "payment-3",
		TenantID:    "tenant-1",
		PaymentType: payments.PaymentTypeMade,
		Amount:      decimal.NewFromInt(75),
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "unallocated received payments",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  1, // Only payment-1 (payment-2 is fully allocated)
		},
		{
			name:       "unallocated made payments",
			query:      "?type=MADE",
			wantStatus: http.StatusOK,
			wantCount:  1, // payment-3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/payments/unallocated"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetUnallocatedPayments(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []payments.Payment
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}
