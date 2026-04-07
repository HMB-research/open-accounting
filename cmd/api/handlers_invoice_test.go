package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// Mock Invoicing Repository
// =============================================================================

type mockInvoicingRepository struct {
	invoices      map[string]*invoicing.Invoice
	invoiceNumber int

	// Error injection
	createErr        error
	getErr           error
	listErr          error
	updateStatusErr  error
	updatePaymentErr error
	generateNumErr   error
}

func newMockInvoicingRepository() *mockInvoicingRepository {
	return &mockInvoicingRepository{
		invoices:      make(map[string]*invoicing.Invoice),
		invoiceNumber: 1,
	}
}

func (m *mockInvoicingRepository) Create(ctx context.Context, schemaName string, invoice *invoicing.Invoice) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m *mockInvoicingRepository) GetByID(ctx context.Context, schemaName, tenantID, invoiceID string) (*invoicing.Invoice, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	inv, ok := m.invoices[invoiceID]
	if !ok {
		return nil, invoicing.ErrInvoiceNotFound
	}
	if inv.TenantID != tenantID {
		return nil, invoicing.ErrInvoiceNotFound
	}
	return inv, nil
}

func (m *mockInvoicingRepository) List(ctx context.Context, schemaName, tenantID string, filter *invoicing.InvoiceFilter) ([]invoicing.Invoice, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []invoicing.Invoice
	for _, inv := range m.invoices {
		if inv.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.InvoiceType != "" && inv.InvoiceType != filter.InvoiceType {
				continue
			}
			if filter.Status != "" && inv.Status != filter.Status {
				continue
			}
			if filter.ContactID != "" && inv.ContactID != filter.ContactID {
				continue
			}
		}
		result = append(result, *inv)
	}
	return result, nil
}

func (m *mockInvoicingRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, invoiceID string, status invoicing.InvoiceStatus) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	inv, ok := m.invoices[invoiceID]
	if !ok {
		return invoicing.ErrInvoiceNotFound
	}
	if inv.TenantID != tenantID {
		return invoicing.ErrInvoiceNotFound
	}
	inv.Status = status
	return nil
}

func (m *mockInvoicingRepository) UpdatePayment(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status invoicing.InvoiceStatus) error {
	if m.updatePaymentErr != nil {
		return m.updatePaymentErr
	}
	inv, ok := m.invoices[invoiceID]
	if !ok {
		return invoicing.ErrInvoiceNotFound
	}
	inv.AmountPaid = amountPaid
	inv.Status = status
	return nil
}

func (m *mockInvoicingRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string, invoiceType invoicing.InvoiceType) (string, error) {
	if m.generateNumErr != nil {
		return "", m.generateNumErr
	}
	prefix := "INV"
	if invoiceType == invoicing.InvoiceTypePurchase {
		prefix = "BILL"
	}
	num := m.invoiceNumber
	m.invoiceNumber++
	return prefix + "-2026-" + padNumber(num, 4), nil
}

func (m *mockInvoicingRepository) UpdateOverdueStatus(ctx context.Context, schemaName, tenantID string) (int, error) {
	count := 0
	today := time.Now()
	for _, inv := range m.invoices {
		if inv.TenantID == tenantID && inv.Status == invoicing.StatusSent && inv.DueDate.Before(today) {
			inv.Status = invoicing.StatusOverdue
			count++
		}
	}
	return count, nil
}

// Helper to pad numbers
func padNumber(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s += "0"
	}
	ns := s + string(rune('0'+n%10))
	if n >= 10 {
		ns = s[:len(s)-1] + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return ns[len(ns)-width:]
}

// Helper to add a test invoice
func (m *mockInvoicingRepository) addTestInvoice(id, tenantID, contactID string, invType invoicing.InvoiceType, status invoicing.InvoiceStatus) *invoicing.Invoice {
	inv := &invoicing.Invoice{
		ID:            id,
		TenantID:      tenantID,
		InvoiceNumber: "INV-001",
		InvoiceType:   invType,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 14),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		Status:        status,
		Subtotal:      decimal.NewFromInt(100),
		VATAmount:     decimal.NewFromInt(20),
		Total:         decimal.NewFromInt(120),
		AmountPaid:    decimal.Zero,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	m.invoices[id] = inv
	return inv
}

// =============================================================================
// Test Setup Helpers
// =============================================================================

func setupInvoiceTestHandlers() (*Handlers, *mockTenantRepository, *mockInvoicingRepository) {
	tenantRepo := newMockTenantRepository()
	invoiceRepo := newMockInvoicingRepository()

	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)
	invoiceSvc := invoicing.NewServiceWithRepository(invoiceRepo, nil)
	tokenSvc := auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour)

	h := &Handlers{
		tenantService:    tenantSvc,
		invoicingService: invoiceSvc,
		tokenService:     tokenSvc,
	}

	return h, tenantRepo, invoiceRepo
}

func setupInvoiceImportTestHandlers() (*Handlers, *mockTenantRepository, *mockInvoicingRepository, *mockContactsRepository) {
	h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
	contactsRepo := newMockContactsRepository()
	h.contactsService = contacts.NewServiceWithRepository(contactsRepo)
	return h, tenantRepo, invoiceRepo, contactsRepo
}

// =============================================================================
// ListInvoices Handler Tests
// =============================================================================

func TestListInvoices(t *testing.T) {
	tests := []struct {
		name          string
		tenantID      string
		queryParams   map[string]string
		claims        *auth.Claims
		setupMock     func(*mockTenantRepository, *mockInvoicingRepository)
		wantStatus    int
		checkResponse func(*testing.T, []map[string]interface{})
	}{
		{
			name:     "list all invoices",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
				ir.addTestInvoice("inv-2", "tenant-1", "contact-2", invoicing.InvoiceTypePurchase, invoicing.StatusSent)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 2)
			},
		},
		{
			name:     "filter by type - sales only",
			tenantID: "tenant-1",
			queryParams: map[string]string{
				"type": "SALES",
			},
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
				ir.addTestInvoice("inv-2", "tenant-1", "contact-2", invoicing.InvoiceTypePurchase, invoicing.StatusSent)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 1)
			},
		},
		{
			name:     "filter by status - draft only",
			tenantID: "tenant-1",
			queryParams: map[string]string{
				"status": "DRAFT",
			},
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
				ir.addTestInvoice("inv-2", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 1)
			},
		},
		{
			name:     "empty list",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Empty(t, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, invoiceRepo)
			}

			path := "/tenants/" + tt.tenantID + "/invoices"
			if len(tt.queryParams) > 0 {
				path += "?"
				for k, v := range tt.queryParams {
					path += k + "=" + v + "&"
				}
			}

			req := makeAuthenticatedRequest(http.MethodGet, path, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ListInvoices(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.checkResponse != nil {
				var resp []map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

// =============================================================================
// CreateInvoice Handler Tests
// =============================================================================

func TestCreateInvoice(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository, *mockInvoicingRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:     "create sales invoice",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"invoice_type": "SALES",
				"contact_id":   "contact-1",
				"issue_date":   "2026-01-15T00:00:00Z",
				"due_date":     "2026-01-29T00:00:00Z",
				"currency":     "EUR",
				"lines": []map[string]interface{}{
					{
						"description": "Service Fee",
						"quantity":    "1",
						"unit_price":  "100.00",
						"vat_rate":    "20",
					},
				},
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["id"])
				assert.Equal(t, "SALES", resp["invoice_type"])
				assert.Equal(t, "DRAFT", resp["status"])
			},
		},
		{
			name:     "create purchase invoice (bill)",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"invoice_type": "PURCHASE",
				"contact_id":   "supplier-1",
				"issue_date":   "2026-01-15T00:00:00Z",
				"due_date":     "2026-02-15T00:00:00Z",
				"currency":     "EUR",
				"lines": []map[string]interface{}{
					{
						"description": "Supplies",
						"quantity":    "10",
						"unit_price":  "50.00",
						"vat_rate":    "20",
					},
				},
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "PURCHASE", resp["invoice_type"])
			},
		},
		{
			name:     "missing contact_id",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"invoice_type": "SALES",
				"issue_date":   "2026-01-15T00:00:00Z",
				"due_date":     "2026-01-29T00:00:00Z",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "Contact",
		},
		{
			name:     "create invoice blocked by period lock",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"invoice_type": "SALES",
				"contact_id":   "contact-1",
				"issue_date":   "2026-01-15T00:00:00Z",
				"due_date":     "2026-01-29T00:00:00Z",
				"currency":     "EUR",
				"lines": []map[string]interface{}{
					{
						"description": "Service Fee",
						"quantity":    "1",
						"unit_price":  "100.00",
						"vat_rate":    "20",
					},
				},
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				lockedTenant := tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				lockDate := "2026-01-31"
				lockedTenant.Settings.PeriodLockDate = &lockDate
			},
			wantStatus:     http.StatusConflict,
			wantErrContain: "period locked through 2026-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, invoiceRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/invoices", tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.CreateInvoice(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestCreateInvoiceInvalidJSON(t *testing.T) {
	h, tenantRepo, _ := setupInvoiceTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.CreateInvoice(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportInvoices(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository, *mockInvoicingRepository, *mockContactsRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, invoicing.ImportInvoicesResult, *mockInvoicingRepository)
	}{
		{
			name:     "imports grouped invoice rows",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"file_name": "invoices.csv",
				"csv_content": "invoice_number,invoice_type,contact_code,issue_date,due_date,status,line_description,quantity,unit_price,vat_rate,amount_paid\n" +
					"INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,PAID,Implementation work,1,100.00,22,183.00\n" +
					"INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,PAID,Support retainer,1,50.00,22,183.00\n",
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				contact := cr.addTestContact("contact-1", "tenant-1", "Acme Corp", contacts.ContactTypeCustomer, true)
				contact.Code = "CUST-001"
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp invoicing.ImportInvoicesResult, invoiceRepo *mockInvoicingRepository) {
				assert.Equal(t, "invoices.csv", resp.FileName)
				assert.Equal(t, 2, resp.RowsProcessed)
				assert.Equal(t, 1, resp.InvoicesCreated)
				assert.Equal(t, 2, resp.LinesImported)
				assert.Zero(t, resp.RowsSkipped)
				assert.Empty(t, resp.Errors)
				require.Len(t, invoiceRepo.invoices, 1)
				for _, invoice := range invoiceRepo.invoices {
					assert.Equal(t, "INV-EXT-001", invoice.InvoiceNumber)
					assert.Equal(t, invoicing.StatusPaid, invoice.Status)
				}
			},
		},
		{
			name:     "skips locked invoice rows and returns summary",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"csv_content": "invoice_number,invoice_type,contact_name,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
					"INV-LOCK-001,SALES,Locked Customer,2026-01-10,2026-01-24,Implementation work,1,100.00,22\n",
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository, cr *mockContactsRepository) {
				lockDate := "2026-01-31"
				lockedTenant := tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				lockedTenant.Settings.PeriodLockDate = &lockDate
				cr.addTestContact("contact-1", "tenant-1", "Locked Customer", contacts.ContactTypeCustomer, true)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp invoicing.ImportInvoicesResult, invoiceRepo *mockInvoicingRepository) {
				assert.Equal(t, "invoices_import.csv", resp.FileName)
				assert.Equal(t, 1, resp.RowsProcessed)
				assert.Zero(t, resp.InvoicesCreated)
				assert.Zero(t, resp.LinesImported)
				assert.Equal(t, 1, resp.RowsSkipped)
				require.Len(t, resp.Errors, 1)
				assert.Contains(t, resp.Errors[0].Message, "period locked through 2026-01-31")
				assert.Empty(t, invoiceRepo.invoices)
			},
		},
		{
			name:     "rejects missing csv content",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{"file_name": "invoices.csv"},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "csv_content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo, contactsRepo := setupInvoiceImportTestHandlers()
			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, invoiceRepo, contactsRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/invoices/import", tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ImportInvoices(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
				return
			}

			if tt.checkResponse != nil {
				var resp invoicing.ImportInvoicesResult
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp, invoiceRepo)
			}
		})
	}
}

// =============================================================================
// GetInvoice Handler Tests
// =============================================================================

func TestGetInvoice(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		invoiceID      string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository, *mockInvoicingRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "get existing invoice",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "inv-1", resp["id"])
			},
		},
		{
			name:      "invoice not found",
			tenantID:  "tenant-1",
			invoiceID: "nonexistent",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusNotFound,
			wantErrContain: "not found",
		},
		{
			name:      "invoice from different tenant",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-2", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
			},
			wantStatus:     http.StatusNotFound,
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, invoiceRepo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID+"/invoices/"+tt.invoiceID, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "invoiceID": tt.invoiceID})
			w := httptest.NewRecorder()

			h.GetInvoice(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

// =============================================================================
// SendInvoice Handler Tests
// =============================================================================

func TestSendInvoice(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		invoiceID      string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository, *mockInvoicingRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:      "send draft invoice",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "cannot send already sent invoice",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not in draft",
		},
		{
			name:      "invoice not found",
			tenantID:  "tenant-1",
			invoiceID: "nonexistent",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, invoiceRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/invoices/"+tt.invoiceID+"/send", nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "invoiceID": tt.invoiceID})
			w := httptest.NewRecorder()

			h.SendInvoice(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}
		})
	}
}

// =============================================================================
// VoidInvoice Handler Tests
// =============================================================================

func TestVoidInvoice(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		invoiceID      string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository, *mockInvoicingRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:      "void draft invoice",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "void sent invoice",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "cannot void paid invoice",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				inv := ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusPaid)
				inv.AmountPaid = inv.Total
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "payments",
		},
		{
			name:      "invoice not found",
			tenantID:  "tenant-1",
			invoiceID: "nonexistent",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not found",
		},
		{
			name:      "void invoice blocked by period lock",
			tenantID:  "tenant-1",
			invoiceID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				lockedTenant := tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				lockDate := "2026-01-31"
				lockedTenant.Settings.PeriodLockDate = &lockDate
				invoice := ir.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
				invoice.IssueDate = time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
			},
			wantStatus:     http.StatusConflict,
			wantErrContain: "period locked through 2026-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, invoiceRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/invoices/"+tt.invoiceID+"/void", nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "invoiceID": tt.invoiceID})
			w := httptest.NewRecorder()

			h.VoidInvoice(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}
		})
	}
}
