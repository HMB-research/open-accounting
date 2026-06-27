package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
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
	reversalErr   error
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

func (m *mockPaymentsRepository) CreateReversal(ctx context.Context, schemaName string, originalPaymentID string, reversal *payments.Payment, allocations []payments.PaymentAllocation, reversedAt time.Time, reversedBy string, reason string) error {
	if m.reversalErr != nil {
		return m.reversalErr
	}
	original, ok := m.payments[originalPaymentID]
	if !ok || original.TenantID != reversal.TenantID {
		return payments.ErrPaymentNotFound
	}
	if original.ReversedByPaymentID != nil {
		return payments.ErrPaymentAlreadyReversed
	}
	m.payments[reversal.ID] = reversal
	m.allocations[reversal.ID] = append([]payments.PaymentAllocation(nil), allocations...)
	original.ReversedByPaymentID = &reversal.ID
	original.ReversedAt = &reversedAt
	original.ReversedBy = &reversedBy
	original.ReversalReason = reason
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
	recordPaymentErr   error
	recordPaymentCalls []struct {
		invoiceID string
		amount    decimal.Decimal
	}
}

func (m *mockInvoiceServiceForPayments) RecordPayment(ctx context.Context, tenantID, schemaName, invoiceID string, amount decimal.Decimal) error {
	m.recordPaymentCalls = append(m.recordPaymentCalls, struct {
		invoiceID string
		amount    decimal.Decimal
	}{invoiceID, amount})
	return m.recordPaymentErr
}

func (m *mockInvoiceServiceForPayments) ResolveInvoiceIDByNumber(ctx context.Context, tenantID, schemaName, invoiceNumber string) (string, error) {
	return "", invoicing.ErrInvoiceNotFound
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

func TestPaymentReceiptEvidenceRequirement(t *testing.T) {
	h, repo, tenantRepo := setupPaymentTestHandlers()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
		Name:       "Test Tenant",
	}
	repo.payments["pay-1"] = &payments.Payment{
		ID:          "pay-1",
		TenantID:    "tenant-1",
		PaymentType: payments.PaymentTypeReceived,
		PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Amount:      decimal.NewFromInt(100),
		Currency:    "EUR",
	}

	docRepo := newMockDocumentRepository()
	h.documentsService = documents.NewService(docRepo, nil)

	err := h.requireApprovedPaymentReceiptEvidence(context.Background(), "tenant_test", "tenant-1", "pay-1", true)
	require.Error(t, err)
	assert.ErrorIs(t, err, errApprovedPaymentReceiptEvidenceRequired)

	docRepo.docs["doc-1"] = &documents.Document{
		ID:           "doc-1",
		TenantID:     "tenant-1",
		EntityType:   documents.EntityTypePayment,
		EntityID:     "pay-1",
		DocumentType: documents.DocumentTypeReceipt,
		ReviewStatus: documents.ReviewStatusApproved,
	}
	err = h.requireApprovedPaymentReceiptEvidence(context.Background(), "tenant_test", "tenant-1", "pay-1", true)
	require.NoError(t, err)

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/payments/pay-1/email-receipt", bytes.NewBufferString(`{
		"recipient_email":"billing@example.com",
		"require_approved_evidence":true
	}`)), map[string]string{"tenantID": "tenant-1", "paymentID": "pay-1"})
	docRepo.docs = map[string]*documents.Document{}
	rr := httptest.NewRecorder()
	h.EmailPaymentReceipt(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "approved payment receipt evidence is required")
	var conflict struct {
		Error                 string                                `json:"error"`
		EvidencePolicyResults []documents.EvidencePolicyResult      `json:"evidence_policy_results"`
		RemediationActions    []documents.DocumentRemediationAction `json:"remediation_actions"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&conflict))
	assert.Contains(t, conflict.Error, "approved payment receipt evidence is required")
	require.Len(t, conflict.EvidencePolicyResults, 1)
	assert.Equal(t, documents.EntityTypePayment, conflict.EvidencePolicyResults[0].EntityType)
	assert.Equal(t, "pay-1", conflict.EvidencePolicyResults[0].EntityID)
	assert.False(t, conflict.EvidencePolicyResults[0].Compliant)
	require.Len(t, conflict.RemediationActions, 1)
	assert.Equal(t, "document_evidence_missing", conflict.RemediationActions[0].Code)
	assert.Equal(t, "oa documents upload --entity-type payment --entity-id pay-1 --document-type receipt --file <file>", conflict.RemediationActions[0].CLICommand)
}

func TestEmailPaymentReceiptSendsEmail(t *testing.T) {
	h, repo, tenantRepo := setupPaymentTestHandlers()
	emailRepo, mailer := configureEmailHandlerService(h, "tenant-1")
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	repo.payments["pay-email"] = &payments.Payment{
		ID:            "pay-email",
		TenantID:      "tenant-1",
		PaymentNumber: "PMT-EMAIL-001",
		PaymentType:   payments.PaymentTypeReceived,
		PaymentDate:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Amount:        decimal.NewFromInt(100),
		Currency:      "EUR",
		Reference:     "REF-100",
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payments/pay-email/email-receipt", email.SendPaymentReceiptRequest{
		RecipientEmail: "payer@example.com",
		RecipientName:  "Payer",
		Subject:        "Custom receipt subject",
		Message:        "Thanks for your payment.",
	}, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "paymentID": "pay-email"})
	rr := httptest.NewRecorder()

	h.EmailPaymentReceipt(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var result email.EmailSentResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.True(t, result.Success)
	assert.Equal(t, 1, mailer.sentCount)
	require.Len(t, emailRepo.logs, 1)
	assert.Equal(t, string(email.TemplatePaymentReceipt), emailRepo.logs[0].EmailType)
	assert.Equal(t, "pay-email", emailRepo.logs[0].RelatedID)
	assert.Equal(t, "Custom receipt subject", emailRepo.logs[0].Subject)
	assert.Equal(t, email.StatusSent, emailRepo.logs[0].Status)
}

func TestEmailPaymentReceiptValidationAndErrorBranches(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		rawBody    string
		setup      func(*Handlers, *mockPaymentsRepository, *mockTenantRepository)
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid JSON",
			rawBody:    "{",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid request body",
		},
		{
			name:       "missing recipient",
			body:       email.SendPaymentReceiptRequest{},
			wantStatus: http.StatusBadRequest,
			wantError:  "recipient email is required",
		},
		{
			name: "payment not found",
			body: email.SendPaymentReceiptRequest{RecipientEmail: "payer@example.com"},
			setup: func(_ *Handlers, _ *mockPaymentsRepository, tenantRepo *mockTenantRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Payment not found",
		},
		{
			name: "required evidence without document service",
			body: email.SendPaymentReceiptRequest{
				RecipientEmail:          "payer@example.com",
				RequireApprovedEvidence: true,
			},
			setup: func(_ *Handlers, repo *mockPaymentsRepository, tenantRepo *mockTenantRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				repo.payments["pay-email"] = &payments.Payment{
					ID:          "pay-email",
					TenantID:    "tenant-1",
					PaymentType: payments.PaymentTypeReceived,
					PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:      decimal.NewFromInt(100),
					Currency:    "EUR",
				}
			},
			wantStatus: http.StatusConflict,
			wantError:  "approved payment receipt evidence is required",
		},
		{
			name: "evidence evaluation failure",
			body: email.SendPaymentReceiptRequest{
				RecipientEmail:          "payer@example.com",
				RequireApprovedEvidence: true,
			},
			setup: func(h *Handlers, repo *mockPaymentsRepository, tenantRepo *mockTenantRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				repo.payments["pay-email"] = &payments.Payment{
					ID:          "pay-email",
					TenantID:    "tenant-1",
					PaymentType: payments.PaymentTypeReceived,
					PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:      decimal.NewFromInt(100),
					Currency:    "EUR",
				}
				docRepo := newMockDocumentRepository()
				docRepo.listDocumentsErr = errors.New("document repository unavailable")
				h.documentsService = documents.NewService(docRepo, nil)
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to verify payment receipt evidence",
		},
		{
			name: "tenant lookup failure",
			body: email.SendPaymentReceiptRequest{RecipientEmail: "payer@example.com"},
			setup: func(_ *Handlers, repo *mockPaymentsRepository, _ *mockTenantRepository) {
				repo.payments["pay-email"] = &payments.Payment{
					ID:          "pay-email",
					TenantID:    "tenant-1",
					PaymentType: payments.PaymentTypeReceived,
					PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:      decimal.NewFromInt(100),
					Currency:    "EUR",
				}
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to get tenant",
		},
		{
			name: "template lookup failure",
			body: email.SendPaymentReceiptRequest{RecipientEmail: "payer@example.com"},
			setup: func(h *Handlers, repo *mockPaymentsRepository, tenantRepo *mockTenantRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				repo.payments["pay-email"] = &payments.Payment{
					ID:          "pay-email",
					TenantID:    "tenant-1",
					PaymentType: payments.PaymentTypeReceived,
					PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:      decimal.NewFromInt(100),
					Currency:    "EUR",
				}
				emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
				emailRepo.getTemplateErr = errors.New("template repository unavailable")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to get email template",
		},
		{
			name: "template render failure",
			body: email.SendPaymentReceiptRequest{RecipientEmail: "payer@example.com"},
			setup: func(h *Handlers, repo *mockPaymentsRepository, tenantRepo *mockTenantRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				repo.payments["pay-email"] = &payments.Payment{
					ID:          "pay-email",
					TenantID:    "tenant-1",
					PaymentType: payments.PaymentTypeReceived,
					PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:      decimal.NewFromInt(100),
					Currency:    "EUR",
				}
				emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
				emailRepo.templates[emailTemplateKey("tenant-1", email.TemplatePaymentReceipt)] = email.EmailTemplate{
					TenantID:     "tenant-1",
					TemplateType: email.TemplatePaymentReceipt,
					Subject:      "{{",
					BodyHTML:     "<p>Receipt</p>",
					IsActive:     true,
				}
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to render email template",
		},
		{
			name: "send failure",
			body: email.SendPaymentReceiptRequest{RecipientEmail: "payer@example.com"},
			setup: func(h *Handlers, repo *mockPaymentsRepository, tenantRepo *mockTenantRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				repo.payments["pay-email"] = &payments.Payment{
					ID:          "pay-email",
					TenantID:    "tenant-1",
					PaymentType: payments.PaymentTypeReceived,
					PaymentDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:      decimal.NewFromInt(100),
					Currency:    "EUR",
				}
				emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
				emailRepo.settings["tenant-1"] = []byte(`{}`)
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "SMTP is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupPaymentTestHandlers()
			if tt.setup != nil {
				tt.setup(h, repo, tenantRepo)
			}

			var req *http.Request
			if tt.rawBody != "" {
				req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/payments/pay-email/email-receipt", bytes.NewBufferString(tt.rawBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payments/pay-email/email-receipt", tt.body, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
			}
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "paymentID": "pay-email"})
			rr := httptest.NewRecorder()

			h.EmailPaymentReceipt(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
			var body map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
			assert.Contains(t, body["error"], tt.wantError)
		})
	}
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
		name        string
		body        map[string]interface{}
		setupTenant func(*mockTenantRepository)
		wantStatus  int
		wantErr     string
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
			name: "payment blocked by period lock",
			body: map[string]interface{}{
				"payment_type": "RECEIVED",
				"amount":       "100.00",
				"payment_date": "2026-01-15T00:00:00Z",
				"currency":     "EUR",
			},
			setupTenant: func(repo *mockTenantRepository) {
				lockDate := "2026-01-31"
				repo.tenants["tenant-1"].Settings.PeriodLockDate = &lockDate
			},
			wantStatus: http.StatusConflict,
			wantErr:    "period locked through 2026-01-31",
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
			if tt.setupTenant != nil {
				tt.setupTenant(tenantRepo)
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

func TestReversePayment(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]interface{}
		setupRepo   func(*mockPaymentsRepository)
		setupTenant func(*mockTenantRepository)
		wantStatus  int
		wantErr     string
	}{
		{
			name: "valid reversal",
			body: map[string]interface{}{
				"payment_date": "2026-03-20T00:00:00Z",
				"reason":       "Duplicate bank import",
			},
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:            "payment-1",
					TenantID:      "tenant-1",
					PaymentNumber: "PMT-00001",
					PaymentType:   payments.PaymentTypeReceived,
					PaymentDate:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
					Amount:        decimal.RequireFromString("100.00"),
					Currency:      "EUR",
					ExchangeRate:  decimal.NewFromInt(1),
					BaseAmount:    decimal.RequireFromString("100.00"),
				}
				repo.allocations["payment-1"] = []payments.PaymentAllocation{{
					ID:        "alloc-1",
					TenantID:  "tenant-1",
					PaymentID: "payment-1",
					InvoiceID: "invoice-1",
					Amount:    decimal.RequireFromString("40.00"),
				}}
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing reason rejected",
			body: map[string]interface{}{
				"payment_date": "2026-03-20T00:00:00Z",
			},
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:           "payment-1",
					TenantID:     "tenant-1",
					PaymentType:  payments.PaymentTypeReceived,
					Amount:       decimal.NewFromInt(10),
					Currency:     "EUR",
					ExchangeRate: decimal.NewFromInt(1),
					BaseAmount:   decimal.NewFromInt(10),
				}
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "reversal reason is required",
		},
		{
			name: "already reversed rejected",
			body: map[string]interface{}{
				"payment_date": "2026-03-20T00:00:00Z",
				"reason":       "Duplicate",
			},
			setupRepo: func(repo *mockPaymentsRepository) {
				reversedByPaymentID := "payment-2"
				repo.payments["payment-1"] = &payments.Payment{
					ID:                  "payment-1",
					TenantID:            "tenant-1",
					PaymentType:         payments.PaymentTypeReceived,
					Amount:              decimal.NewFromInt(10),
					Currency:            "EUR",
					ExchangeRate:        decimal.NewFromInt(1),
					BaseAmount:          decimal.NewFromInt(10),
					ReversedByPaymentID: &reversedByPaymentID,
				}
			},
			wantStatus: http.StatusConflict,
			wantErr:    "already reversed",
		},
		{
			name: "period lock rejected",
			body: map[string]interface{}{
				"payment_date": "2026-01-15T00:00:00Z",
				"reason":       "Duplicate",
			},
			setupRepo: func(repo *mockPaymentsRepository) {
				repo.payments["payment-1"] = &payments.Payment{
					ID:           "payment-1",
					TenantID:     "tenant-1",
					PaymentType:  payments.PaymentTypeReceived,
					Amount:       decimal.NewFromInt(10),
					Currency:     "EUR",
					ExchangeRate: decimal.NewFromInt(1),
					BaseAmount:   decimal.NewFromInt(10),
				}
			},
			setupTenant: func(repo *mockTenantRepository) {
				lockDate := "2026-01-31"
				repo.tenants["tenant-1"].Settings.PeriodLockDate = &lockDate
			},
			wantStatus: http.StatusConflict,
			wantErr:    "period locked through 2026-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupPaymentTestHandlers()
			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}
			if tt.setupTenant != nil {
				tt.setupTenant(tenantRepo)
			}
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/payments/payment-1/reverse", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "paymentID": "payment-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ReversePayment(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
				return
			}

			var result payments.PaymentReversalResult
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
			require.NotNil(t, result.OriginalPayment)
			require.NotNil(t, result.ReversalPayment)
			assert.Equal(t, payments.PaymentTypeMade, result.ReversalPayment.PaymentType)
			assert.Equal(t, result.ReversalPayment.ID, *result.OriginalPayment.ReversedByPaymentID)
			assert.Equal(t, "payment-1", *result.ReversalPayment.ReversalOfPaymentID)
			assert.Equal(t, "Duplicate bank import", result.ReversalPayment.ReversalReason)
			require.Len(t, result.ReversalPayment.Allocations, 1)
			assert.Equal(t, "invoice-1", result.ReversalPayment.Allocations[0].InvoiceID)
		})
	}
}

func TestImportPayments(t *testing.T) {
	h, repo, tenantRepo := setupPaymentTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	body := map[string]interface{}{
		"file_name":   "payments.csv",
		"csv_content": "payment_number,payment_type,payment_date,amount,invoice_id,allocation_amount\nPAY-001,RECEIVED,2026-03-15,100.00,88888888-8888-4888-8888-888888888888,60.00\n",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/payments/import", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ImportPayments(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result payments.ImportPaymentsResult
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.PaymentsCreated)
	assert.Equal(t, 0, result.RowsSkipped)

	require.Len(t, repo.payments, 1)
	for _, payment := range repo.payments {
		assert.Equal(t, "PAY-001", payment.PaymentNumber)
		assert.Equal(t, "user-1", payment.CreatedBy)
		assert.True(t, payment.Amount.Equal(decimal.RequireFromString("100.00")))
		require.Len(t, repo.allocations[payment.ID], 1)
		assert.Equal(t, "88888888-8888-4888-8888-888888888888", repo.allocations[payment.ID][0].InvoiceID)
	}
}

func TestExportSEPAPayments(t *testing.T) {
	h, _, tenantRepo := setupPaymentTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payments/sepa-export", payments.SEPAExportRequest{
		MessageID:        "MSG-20260331",
		PaymentInfoID:    "PMTINF-20260331",
		CreationDateTime: "2026-03-31T09:30:00Z",
		DebtorName:       "Example OU",
		DebtorIBAN:       "EE382200221020145685",
		DebtorBIC:        "HABAEE2X",
		ExecutionDate:    "2026-04-01",
		Lines: []payments.SEPACreditTransferLine{{
			EndToEndID:   "INV-1001",
			CreditorName: "Supplier AS",
			CreditorIBAN: "EE471000001020145685",
			Amount:       decimal.RequireFromString("125.50"),
			Remittance:   "Invoice INV-1001",
		}},
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.ExportSEPAPayments(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/xml", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "sepa-payments-2026-04-01.xml")
	assert.Contains(t, rr.Body.String(), `<MsgId>MSG-20260331</MsgId>`)
	assert.Contains(t, rr.Body.String(), `<InstdAmt Ccy="EUR">125.50</InstdAmt>`)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payments/sepa-export", payments.SEPAExportRequest{
		DebtorName:    "Example OU",
		DebtorIBAN:    "EE001",
		ExecutionDate: "2026-04-01",
		Lines: []payments.SEPACreditTransferLine{{
			CreditorName: "Supplier AS",
			CreditorIBAN: "EE471000001020145685",
			Amount:       decimal.RequireFromString("125.50"),
		}},
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.ExportSEPAPayments(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "debtor_iban")
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
