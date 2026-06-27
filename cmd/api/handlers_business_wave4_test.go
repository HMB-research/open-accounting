package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func wave4Request(method, path string, body any, params map[string]string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	req = withURLParams(req, params)
	return req
}

func wave4RawRequest(method, path, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, params)
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))
	return req
}

func setupWave4AnalyticsHandlers(t *testing.T) (*Handlers, *mockAnalyticsRepository) {
	t.Helper()

	h, repo, tenantRepo := setupAnalyticsTestHandlers()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}
	return h, repo
}

func TestBusinessWave4AnalyticsErrorAndFormatBranches(t *testing.T) {
	t.Run("dashboard summary service error", func(t *testing.T) {
		h, repo := setupWave4AnalyticsHandlers(t)
		repo.revenueErr = errors.New("revenue query failed")

		rr := httptest.NewRecorder()
		h.GetDashboardSummary(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/analytics/dashboard", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get dashboard summary")
	})

	t.Run("revenue expense chart service error", func(t *testing.T) {
		h, repo := setupWave4AnalyticsHandlers(t)
		repo.monthlyErr = errors.New("monthly query failed")

		rr := httptest.NewRecorder()
		h.GetRevenueExpenseChart(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/analytics/revenue-expense?months=6", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get chart data")
	})

	t.Run("cash flow custom months and service error", func(t *testing.T) {
		h, repo := setupWave4AnalyticsHandlers(t)
		repo.cashFlowData = []analytics.MonthlyCashFlowData{
			{Label: "Mar 2026", Inflows: decimal.NewFromInt(800), Outflows: decimal.NewFromInt(450)},
		}

		rr := httptest.NewRecorder()
		h.GetCashFlowChart(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/analytics/cash-flow?months=3", nil, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var chart analytics.CashFlowChart
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&chart))
		assert.Equal(t, []string{"Mar 2026"}, chart.Labels)

		h, repo = setupWave4AnalyticsHandlers(t)
		repo.cashFlowErr = errors.New("cash flow query failed")
		rr = httptest.NewRecorder()
		h.GetCashFlowChart(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/analytics/cash-flow", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get chart data")
	})

	t.Run("receivables aging bad format and service error", func(t *testing.T) {
		h, _ := setupWave4AnalyticsHandlers(t)

		rr := httptest.NewRecorder()
		h.GetReceivablesAging(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/reports/aging/receivables?format=docx", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "format must be json")

		h, repo := setupWave4AnalyticsHandlers(t)
		repo.agingErr = errors.New("aging query failed")
		rr = httptest.NewRecorder()
		h.GetReceivablesAging(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/reports/aging/receivables", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get aging report")
	})

	t.Run("payables aging format branches and errors", func(t *testing.T) {
		h, _ := setupWave4AnalyticsHandlers(t)

		rr := httptest.NewRecorder()
		h.GetPayablesAging(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/reports/aging/payables?format=xml", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "format must be json")

		h, repo := setupWave4AnalyticsHandlers(t)
		repo.agingErr = errors.New("aging query failed")
		rr = httptest.NewRecorder()
		h.GetPayablesAging(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/reports/aging/payables", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get aging report")

		h, _ = setupWave4AnalyticsHandlers(t)
		rr = httptest.NewRecorder()
		h.GetPayablesAging(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/reports/aging/payables?format=csv", nil, map[string]string{"tenantID": "tenant-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/csv")
		assert.Contains(t, rr.Body.String(), "row_type,report_type,as_of_date")

		h, _ = setupWave4AnalyticsHandlers(t)
		rr = httptest.NewRecorder()
		h.GetPayablesAging(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/reports/aging/payables?format=xlsx", nil, map[string]string{"tenantID": "tenant-1"}))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		requireXLSXContains(t, rr.Body.Bytes(), "Customer A")
	})

	t.Run("recent activity service error", func(t *testing.T) {
		h, repo := setupWave4AnalyticsHandlers(t)
		repo.activityErr = errors.New("activity query failed")

		rr := httptest.NewRecorder()
		h.GetRecentActivity(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/analytics/activity?limit=5", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get recent activity")
	})
}

func TestBusinessWave4ContactBranches(t *testing.T) {
	t.Run("list contacts repository error", func(t *testing.T) {
		h, tenantRepo, contactsRepo := setupContactsTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		contactsRepo.listErr = errors.New("list contacts failed")

		rr := httptest.NewRecorder()
		h.ListContacts(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/contacts?search=acme&type=SUPPLIER", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list contacts")
	})

	t.Run("create contact service error", func(t *testing.T) {
		h, tenantRepo, contactsRepo := setupContactsTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		contactsRepo.createErr = errors.New("contact duplicate")

		rr := httptest.NewRecorder()
		h.CreateContact(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/contacts", contacts.CreateContactRequest{
			Name:        "Duplicate Customer",
			ContactType: contacts.ContactTypeCustomer,
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "contact duplicate")
	})

	t.Run("import contacts invalid json", func(t *testing.T) {
		h, tenantRepo, _ := setupContactsTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

		rr := httptest.NewRecorder()
		h.ImportContacts(rr, wave4RawRequest(http.MethodPost, "/tenants/tenant-1/contacts/import", "{", map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})
}

func TestBusinessWave4InvoiceBranches(t *testing.T) {
	t.Run("list invoices date filters and repository error", func(t *testing.T) {
		h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		invoiceRepo.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusDraft)

		rr := httptest.NewRecorder()
		h.ListInvoices(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/invoices?contact_id=contact-1&from_date=2026-01-01&to_date=2026-01-31", nil, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var listed []invoicing.Invoice
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&listed))
		assert.Len(t, listed, 1)

		h, tenantRepo, invoiceRepo = setupInvoiceTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		invoiceRepo.listErr = errors.New("list invoices failed")
		rr = httptest.NewRecorder()
		h.ListInvoices(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/invoices", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list invoices")
	})

	t.Run("create invoice default issue date and service error", func(t *testing.T) {
		body := map[string]any{
			"invoice_type": "SALES",
			"contact_id":   "contact-1",
			"due_date":     "2027-02-15T00:00:00Z",
			"currency":     "EUR",
			"lines": []map[string]any{{
				"description": "Service fee",
				"quantity":    "1",
				"unit_price":  "100.00",
				"vat_rate":    "22",
			}},
		}

		h, tenantRepo, _ := setupInvoiceTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		rr := httptest.NewRecorder()
		h.CreateInvoice(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/invoices", body, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var invoice invoicing.Invoice
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&invoice))
		assert.False(t, invoice.IssueDate.IsZero())

		h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		invoiceRepo.createErr = errors.New("create invoice failed")
		rr = httptest.NewRecorder()
		h.CreateInvoice(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/invoices", body, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "create invoice failed")
	})

	t.Run("invoice import invalid json and dependency errors", func(t *testing.T) {
		h, tenantRepo, _, _ := setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

		rr := httptest.NewRecorder()
		h.ImportInvoices(rr, wave4RawRequest(http.MethodPost, "/tenants/tenant-1/invoices/import", "{", map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")

		h, tenantRepo, _, contactsRepo := setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		contactsRepo.listErr = errors.New("contacts unavailable")
		rr = httptest.NewRecorder()
		h.ImportInvoices(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/invoices/import", invoicing.ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_name,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,Acme,2026-02-01,2026-02-15,Consulting,1,100,22\n",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load contacts")
	})

	t.Run("einvoice import handler errors and default filename", func(t *testing.T) {
		h, tenantRepo, _, _ := setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

		rr := httptest.NewRecorder()
		h.ImportEInvoice(rr, wave4RawRequest(http.MethodPost, "/tenants/tenant-1/invoices/import-einvoice", "{", map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")

		h, tenantRepo, _, contactsRepo := setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		contactsRepo.listErr = errors.New("contacts unavailable")
		rr = httptest.NewRecorder()
		h.ImportEInvoice(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/invoices/import-einvoice", invoicing.ImportEInvoiceRequest{
			XMLContent: handlerEInvoiceXML(),
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to load contacts")

		h, tenantRepo, _, contactsRepo = setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		contactsRepo.addTestContact("supplier-1", "tenant-1", "Supplier OÜ", contacts.ContactTypeSupplier, true).RegCode = "12345678"
		rr = httptest.NewRecorder()
		h.ImportEInvoice(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/invoices/import-einvoice", invoicing.ImportEInvoiceRequest{
			XMLContent: handlerEInvoiceXML(),
		}, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var result invoicing.ImportInvoicesResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
		assert.Equal(t, "einvoice_import.xml", result.FileName)

		h, tenantRepo, _, _ = setupInvoiceImportTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		rr = httptest.NewRecorder()
		h.ImportEInvoice(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/invoices/import-einvoice", invoicing.ImportEInvoiceRequest{
			XMLContent: "<E_Invoice>",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "XML")
	})
}

func TestBusinessWave4PaymentBranches(t *testing.T) {
	t.Run("list payments filters and repository error", func(t *testing.T) {
		h, repo, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		contactID := "contact-1"
		repo.payments["payment-1"] = &payments.Payment{
			ID:            "payment-1",
			TenantID:      "tenant-1",
			PaymentNumber: "PMT-00001",
			PaymentType:   payments.PaymentTypeReceived,
			ContactID:     &contactID,
			PaymentMethod: "BANK",
			Amount:        decimal.NewFromInt(100),
			PaymentDate:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		}

		rr := httptest.NewRecorder()
		h.ListPayments(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/payments?method=BANK&contact_id=contact-1&from_date=2026-02-01&to_date=2026-02-28", nil, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var listed []payments.Payment
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&listed))
		assert.Len(t, listed, 1)

		h, repo, tenantRepo = setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		repo.listErr = errors.New("list payments failed")
		rr = httptest.NewRecorder()
		h.ListPayments(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/payments", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list payments")
	})

	t.Run("create payment default date and repository error", func(t *testing.T) {
		body := payments.CreatePaymentRequest{
			PaymentType: payments.PaymentTypeReceived,
			Amount:      decimal.NewFromInt(100),
			Currency:    "EUR",
		}

		h, _, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		rr := httptest.NewRecorder()
		h.CreatePayment(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/payments", body, map[string]string{"tenantID": "tenant-1"}))

		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var payment payments.Payment
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&payment))
		assert.False(t, payment.PaymentDate.IsZero())

		h, repo, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		repo.createErr = errors.New("create payment failed")
		rr = httptest.NewRecorder()
		h.CreatePayment(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/payments", body, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "create payment failed")
	})

	t.Run("import payments invalid json and error branches", func(t *testing.T) {
		h, _, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

		rr := httptest.NewRecorder()
		h.ImportPayments(rr, wave4RawRequest(http.MethodPost, "/tenants/tenant-1/payments/import", "{", map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")

		h, _, tenantRepo = setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		rr = httptest.NewRecorder()
		h.ImportPayments(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/payments/import", payments.ImportPaymentsRequest{}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "csv_content is required")

		h, _, _ = setupPaymentTestHandlers()
		rr = httptest.NewRecorder()
		h.ImportPayments(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/payments/import", payments.ImportPaymentsRequest{
			CSVContent: "payment_number,payment_type,payment_date,amount\nPAY-001,RECEIVED,2026-03-15,100.00\n",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to validate period lock")

		h, repo, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		repo.listErr = errors.New("list existing payments failed")
		rr = httptest.NewRecorder()
		h.ImportPayments(rr, wave4Request(http.MethodPost, "/tenants/tenant-1/payments/import", payments.ImportPaymentsRequest{
			CSVContent: "payment_number,payment_type,payment_date,amount\nPAY-001,RECEIVED,2026-03-15,100.00\n",
		}, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "list existing payments")
	})

	t.Run("unallocated payments repository error", func(t *testing.T) {
		h, repo, tenantRepo := setupPaymentTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
		repo.unallocErr = errors.New("unallocated query failed")

		rr := httptest.NewRecorder()
		h.GetUnallocatedPayments(rr, wave4Request(http.MethodGet, "/tenants/tenant-1/payments/unallocated?type=MADE", nil, map[string]string{"tenantID": "tenant-1"}))

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to get unallocated payments")
	})
}
