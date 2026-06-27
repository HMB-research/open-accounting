package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/pdf"
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
	getErrAfterCalls int
	getCalls         int
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
	m.getCalls++
	if m.getErr != nil && (m.getErrAfterCalls == 0 || m.getCalls > m.getErrAfterCalls) {
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
			name:     "create reverse-charge purchase invoice",
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
						"description":   "EU service",
						"quantity":      "1",
						"unit_price":    "100.00",
						"vat_rate":      "22",
						"vat_treatment": "REVERSE_CHARGE",
					},
				},
			},
			setupMock: func(tr *mockTenantRepository, ir *mockInvoicingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "PURCHASE", resp["invoice_type"])
				assert.Equal(t, float64(0), resp["vat_amount"])
				assert.Equal(t, float64(100), resp["total"])
				lines, ok := resp["lines"].([]interface{})
				require.True(t, ok)
				require.Len(t, lines, 1)
				line, ok := lines[0].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "REVERSE_CHARGE", line["vat_treatment"])
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

func TestImportEInvoice(t *testing.T) {
	h, tenantRepo, invoiceRepo, contactsRepo := setupInvoiceImportTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	contact := contactsRepo.addTestContact("supplier-1", "tenant-1", "Supplier OÜ", contacts.ContactTypeSupplier, true)
	contact.RegCode = "12345678"
	contact.VATNumber = "EE12345678"

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/import-einvoice", map[string]interface{}{
		"file_name":   "supplier.xml",
		"xml_content": handlerEInvoiceXML(),
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ImportEInvoice(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp invoicing.ImportInvoicesResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "supplier.xml", resp.FileName)
	assert.Equal(t, 1, resp.RowsProcessed)
	assert.Equal(t, 1, resp.InvoicesCreated)
	assert.Equal(t, 1, resp.LinesImported)
	assert.Zero(t, resp.RowsSkipped)

	require.Len(t, invoiceRepo.invoices, 1)
	for _, invoice := range invoiceRepo.invoices {
		assert.Equal(t, "BILL-2026-001", invoice.InvoiceNumber)
		assert.Equal(t, invoicing.InvoiceTypePurchase, invoice.InvoiceType)
		assert.Equal(t, "supplier-1", invoice.ContactID)
	}
}

func TestImportEInvoiceRejectsMissingXML(t *testing.T) {
	h, tenantRepo, _, _ := setupInvoiceImportTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/import-einvoice", map[string]interface{}{
		"file_name": "supplier.xml",
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ImportEInvoice(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "xml_content is required")
}

func handlerEInvoiceXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<E_Invoice>
  <Header>
    <Date>2026-03-15</Date>
    <FileId>file-1</FileId>
    <Version>1.2</Version>
  </Header>
  <Invoice invoiceId="BILL-2026-001" regNumber="87654321" sellerRegnumber="12345678">
    <InvoiceParties>
      <SellerParty>
        <Name>Supplier OÜ</Name>
        <RegNumber>12345678</RegNumber>
        <VATRegNumber>EE12345678</VATRegNumber>
      </SellerParty>
      <BuyerParty>
        <Name>Buyer OÜ</Name>
        <RegNumber>87654321</RegNumber>
      </BuyerParty>
    </InvoiceParties>
    <InvoiceInformation>
      <Type type="DEB"></Type>
      <DocumentName>Invoice</DocumentName>
      <InvoiceNumber>BILL-2026-001</InvoiceNumber>
      <InvoiceContentText>Office supplies</InvoiceContentText>
      <PaymentReferenceNumber>RF18539007547034</PaymentReferenceNumber>
      <InvoiceDate>2026-03-15</InvoiceDate>
      <DueDate>2026-03-29</DueDate>
    </InvoiceInformation>
    <InvoiceSumGroup>
      <Currency>EUR</Currency>
    </InvoiceSumGroup>
    <InvoiceItem>
      <InvoiceItemGroup>
        <ItemEntry>
          <Description>Office chairs</Description>
          <ItemDetailInfo>
            <ItemUnit>pcs</ItemUnit>
            <ItemAmount>2</ItemAmount>
            <ItemPrice>100.00</ItemPrice>
          </ItemDetailInfo>
          <VAT><VATRate>22</VATRate></VAT>
        </ItemEntry>
      </InvoiceItemGroup>
    </InvoiceItem>
    <PaymentInfo>
      <Currency>EUR</Currency>
      <PayDueDate>2026-03-29</PayDueDate>
      <PaymentId>RF18539007547034</PaymentId>
    </PaymentInfo>
  </Invoice>
</E_Invoice>`
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

func TestGetInvoicePDF(t *testing.T) {
	h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
	h.pdfService = pdf.NewService()
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	tenantRecord.Settings.RegCode = "12345678"

	invoice := invoiceRepo.addTestInvoice("inv-pdf", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
	invoice.InvoiceNumber = "INV-PDF-001"
	invoice.Contact = &contacts.Contact{
		ID:          "contact-1",
		Name:        "Acme OU",
		Email:       "billing@example.com",
		CountryCode: "EE",
	}
	invoice.IssueDate = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	invoice.DueDate = time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	invoice.Subtotal = decimal.NewFromInt(100)
	invoice.VATAmount = decimal.NewFromInt(22)
	invoice.Total = decimal.NewFromInt(122)
	invoice.Lines = []invoicing.InvoiceLine{{
		ID:           "line-1",
		TenantID:     "tenant-1",
		InvoiceID:    "inv-pdf",
		LineNumber:   1,
		Description:  "Consulting",
		Quantity:     decimal.NewFromInt(1),
		Unit:         "hour",
		UnitPrice:    decimal.NewFromInt(100),
		VATRate:      decimal.NewFromInt(22),
		VATTreatment: invoicing.VATTreatmentStandard,
		LineSubtotal: decimal.NewFromInt(100),
		LineVAT:      decimal.NewFromInt(22),
		LineTotal:    decimal.NewFromInt(122),
	}}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/invoices/inv-pdf/pdf", nil, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-pdf"})
	rr := httptest.NewRecorder()

	h.GetInvoicePDF(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), `invoice-INV-PDF-001.pdf`)
	requirePDF(t, rr.Body.Bytes())
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

func TestSendPurchaseInvoiceRequiresApprovedEvidence(t *testing.T) {
	h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
	docRepo := newMockDocumentRepository()
	h.documentsService = documents.NewService(docRepo, nil)

	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	invoiceRepo.addTestInvoice("bill-1", "tenant-1", "supplier-1", invoicing.InvoiceTypePurchase, invoicing.StatusDraft)

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/bill-1/send", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "bill-1"})
	w := httptest.NewRecorder()

	h.SendInvoice(w, req)

	assertPurchaseInvoiceEvidenceConflict(t, w, "bill-1")
	assert.Equal(t, invoicing.StatusDraft, invoiceRepo.invoices["bill-1"].Status)

	docRepo.docs["doc-1"] = &documents.Document{
		ID:           "doc-1",
		TenantID:     "tenant-1",
		EntityType:   documents.EntityTypeInvoice,
		EntityID:     "bill-1",
		DocumentType: documents.DocumentTypeReceipt,
		ReviewStatus: documents.ReviewStatusApproved,
	}

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/bill-1/send", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "bill-1"})
	w = httptest.NewRecorder()

	h.SendInvoice(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	assert.Equal(t, invoicing.StatusSent, invoiceRepo.invoices["bill-1"].Status)
}

func TestEmailPurchaseInvoiceRequiresApprovedEvidence(t *testing.T) {
	h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
	h.documentsService = documents.NewService(newMockDocumentRepository(), nil)

	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	invoiceRepo.addTestInvoice("bill-1", "tenant-1", "supplier-1", invoicing.InvoiceTypePurchase, invoicing.StatusDraft)

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/bill-1/email", map[string]any{
		"recipient_email": "supplier@example.com",
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "bill-1"})
	w := httptest.NewRecorder()

	h.EmailInvoice(w, req)

	assertPurchaseInvoiceEvidenceConflict(t, w, "bill-1")
	assert.Equal(t, invoicing.StatusDraft, invoiceRepo.invoices["bill-1"].Status)
}

func TestEmailInvoiceSendsAttachmentAndMarksDraftSent(t *testing.T) {
	h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
	h.pdfService = pdf.NewService()
	emailRepo, mailer := configureEmailHandlerService(h, "tenant-1")

	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	tenantRecord.Settings.RegCode = "12345678"
	invoice := invoiceRepo.addTestInvoice("inv-email", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)
	invoice.InvoiceNumber = "INV-EMAIL-001"
	invoice.Contact = &contacts.Contact{
		ID:          "contact-1",
		Name:        "Acme OU",
		Email:       "billing@example.com",
		CountryCode: "EE",
	}
	invoice.IssueDate = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	invoice.DueDate = time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	invoice.Subtotal = decimal.NewFromInt(100)
	invoice.VATAmount = decimal.NewFromInt(22)
	invoice.Total = decimal.NewFromInt(122)
	invoice.Lines = []invoicing.InvoiceLine{{
		ID:           "line-1",
		TenantID:     "tenant-1",
		InvoiceID:    "inv-email",
		LineNumber:   1,
		Description:  "Consulting",
		Quantity:     decimal.NewFromInt(1),
		Unit:         "hour",
		UnitPrice:    decimal.NewFromInt(100),
		VATRate:      decimal.NewFromInt(22),
		VATTreatment: invoicing.VATTreatmentStandard,
		LineSubtotal: decimal.NewFromInt(100),
		LineVAT:      decimal.NewFromInt(22),
		LineTotal:    decimal.NewFromInt(122),
	}}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/inv-email/email", email.SendInvoiceRequest{
		RecipientEmail: "customer@example.com",
		RecipientName:  "Customer",
		Subject:        "Custom invoice subject",
		Message:        "Please review this invoice.",
		AttachPDF:      true,
	}, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-email"})
	rr := httptest.NewRecorder()

	h.EmailInvoice(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var result email.EmailSentResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.True(t, result.Success)
	assert.Equal(t, 1, mailer.sentCount)
	assert.Equal(t, invoicing.StatusSent, invoiceRepo.invoices["inv-email"].Status)
	require.Len(t, emailRepo.logs, 1)
	assert.Equal(t, string(email.TemplateInvoiceSend), emailRepo.logs[0].EmailType)
	assert.Equal(t, "inv-email", emailRepo.logs[0].RelatedID)
	assert.Equal(t, "Custom invoice subject", emailRepo.logs[0].Subject)
	assert.Equal(t, email.StatusSent, emailRepo.logs[0].Status)
}

func TestEmailInvoiceValidationAndErrorBranches(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		rawBody    string
		setup      func(*Handlers, *mockTenantRepository, *mockInvoicingRepository)
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
			body:       email.SendInvoiceRequest{},
			wantStatus: http.StatusBadRequest,
			wantError:  "recipient email is required",
		},
		{
			name: "invoice not found",
			body: email.SendInvoiceRequest{RecipientEmail: "customer@example.com"},
			setup: func(_ *Handlers, tenantRepo *mockTenantRepository, _ *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Invoice not found",
		},
		{
			name: "evidence guard invoice lookup failure",
			body: email.SendInvoiceRequest{RecipientEmail: "customer@example.com"},
			setup: func(_ *Handlers, tenantRepo *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
				invoiceRepo.getErr = invoicing.ErrInvoiceNotFound
				invoiceRepo.getErrAfterCalls = 1
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "get invoice",
		},
		{
			name: "purchase evidence required without document service",
			body: email.SendInvoiceRequest{RecipientEmail: "supplier@example.com"},
			setup: func(_ *Handlers, tenantRepo *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "supplier-1", invoicing.InvoiceTypePurchase, invoicing.StatusDraft)
			},
			wantStatus: http.StatusConflict,
			wantError:  "approved purchase-invoice evidence is required",
		},
		{
			name: "purchase evidence evaluation failure",
			body: email.SendInvoiceRequest{RecipientEmail: "supplier@example.com"},
			setup: func(h *Handlers, tenantRepo *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "supplier-1", invoicing.InvoiceTypePurchase, invoicing.StatusDraft)
				docRepo := newMockDocumentRepository()
				docRepo.listDocumentsErr = errors.New("document repository unavailable")
				h.documentsService = documents.NewService(docRepo, nil)
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "evaluate purchase invoice evidence",
		},
		{
			name: "tenant lookup failure",
			body: email.SendInvoiceRequest{RecipientEmail: "customer@example.com"},
			setup: func(_ *Handlers, _ *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to get tenant",
		},
		{
			name: "template lookup failure",
			body: email.SendInvoiceRequest{RecipientEmail: "customer@example.com"},
			setup: func(h *Handlers, tenantRepo *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
				emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
				emailRepo.getTemplateErr = errors.New("template repository unavailable")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to get email template",
		},
		{
			name: "template render failure",
			body: email.SendInvoiceRequest{RecipientEmail: "customer@example.com"},
			setup: func(h *Handlers, tenantRepo *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
				emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
				emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateInvoiceSend)] = email.EmailTemplate{
					TenantID:     "tenant-1",
					TemplateType: email.TemplateInvoiceSend,
					Subject:      "{{",
					BodyHTML:     "<p>Invoice</p>",
					IsActive:     true,
				}
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to render email template",
		},
		{
			name: "send failure",
			body: email.SendInvoiceRequest{RecipientEmail: "customer@example.com"},
			setup: func(h *Handlers, tenantRepo *mockTenantRepository, invoiceRepo *mockInvoicingRepository) {
				tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				invoiceRepo.addTestInvoice("inv-email", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
				emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
				emailRepo.settings["tenant-1"] = []byte(`{}`)
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "SMTP is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
			if tt.setup != nil {
				tt.setup(h, tenantRepo, invoiceRepo)
			}

			var req *http.Request
			if tt.rawBody != "" {
				req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/invoices/inv-email/email", strings.NewReader(tt.rawBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/inv-email/email", tt.body, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
			}
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-email"})
			rr := httptest.NewRecorder()

			h.EmailInvoice(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
			var body map[string]string
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
			assert.Contains(t, body["error"], tt.wantError)
		})
	}
}

func assertPurchaseInvoiceEvidenceConflict(t *testing.T, w *httptest.ResponseRecorder, invoiceID string) {
	t.Helper()

	assert.Equal(t, http.StatusConflict, w.Code, "response body: %s", w.Body.String())

	var conflict struct {
		Error                 string                                `json:"error"`
		EvidencePolicyResults []documents.EvidencePolicyResult      `json:"evidence_policy_results"`
		RemediationActions    []documents.DocumentRemediationAction `json:"remediation_actions"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&conflict))
	assert.Contains(t, conflict.Error, "approved purchase-invoice evidence is required")
	require.Len(t, conflict.EvidencePolicyResults, 1)
	assert.Equal(t, documents.EntityTypeInvoice, conflict.EvidencePolicyResults[0].EntityType)
	assert.Equal(t, invoiceID, conflict.EvidencePolicyResults[0].EntityID)
	assert.False(t, conflict.EvidencePolicyResults[0].Compliant)
	require.Len(t, conflict.RemediationActions, 1)
	assert.Equal(t, "document_evidence_missing", conflict.RemediationActions[0].Code)
	assert.Equal(t, "oa documents upload --entity-type invoice --entity-id "+invoiceID+" --document-type receipt --file <file>", conflict.RemediationActions[0].CLICommand)
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
