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

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// Error definitions for quotes mock repository
var errQuoteNotFound = errors.New("quote not found")

// mockQuotesRepository implements quotes.Repository for testing
type mockQuotesRepository struct {
	quotes      map[string]*quotes.Quote
	quoteNumber int

	createErr       error
	getErr          error
	listErr         error
	updateErr       error
	deleteErr       error
	updateStatusErr error
}

func newMockQuotesRepository() *mockQuotesRepository {
	return &mockQuotesRepository{
		quotes:      make(map[string]*quotes.Quote),
		quoteNumber: 1,
	}
}

func (m *mockQuotesRepository) Create(ctx context.Context, schemaName string, quote *quotes.Quote) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.quotes[quote.ID] = quote
	return nil
}

func (m *mockQuotesRepository) GetByID(ctx context.Context, schemaName, tenantID, quoteID string) (*quotes.Quote, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if q, ok := m.quotes[quoteID]; ok && q.TenantID == tenantID {
		return q, nil
	}
	return nil, errQuoteNotFound
}

func (m *mockQuotesRepository) List(ctx context.Context, schemaName, tenantID string, filter *quotes.QuoteFilter) ([]quotes.Quote, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []quotes.Quote
	for _, q := range m.quotes {
		if q.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.Status != "" && q.Status != filter.Status {
				continue
			}
			if filter.ContactID != "" && q.ContactID != filter.ContactID {
				continue
			}
		}
		result = append(result, *q)
	}
	return result, nil
}

func (m *mockQuotesRepository) Update(ctx context.Context, schemaName string, quote *quotes.Quote) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.quotes[quote.ID] = quote
	return nil
}

func (m *mockQuotesRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, quoteID string, status quotes.QuoteStatus) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	if q, ok := m.quotes[quoteID]; ok && q.TenantID == tenantID {
		q.Status = status
		return nil
	}
	return errQuoteNotFound
}

func (m *mockQuotesRepository) Delete(ctx context.Context, schemaName, tenantID, quoteID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.quotes[quoteID]; !ok {
		return errQuoteNotFound
	}
	delete(m.quotes, quoteID)
	return nil
}

func (m *mockQuotesRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	num := m.quoteNumber
	m.quoteNumber++
	return "QT-" + string(rune('0'+num)), nil
}

func (m *mockQuotesRepository) SetConvertedToOrder(ctx context.Context, schemaName, tenantID, quoteID, orderID string) error {
	if q, ok := m.quotes[quoteID]; ok && q.TenantID == tenantID {
		q.ConvertedToOrderID = &orderID
		q.Status = quotes.QuoteStatusConverted
		return nil
	}
	return errQuoteNotFound
}

func (m *mockQuotesRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error {
	if q, ok := m.quotes[quoteID]; ok && q.TenantID == tenantID {
		q.ConvertedToInvoiceID = &invoiceID
		q.Status = quotes.QuoteStatusConverted
		return nil
	}
	return errQuoteNotFound
}

func setupQuotesTestHandlers() (*Handlers, *mockQuotesRepository, *mockTenantRepository) {
	quotesRepo := newMockQuotesRepository()
	quotesSvc := quotes.NewServiceWithRepository(quotesRepo)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		quotesService: quotesSvc,
		tenantService: tenantSvc,
	}
	return h, quotesRepo, tenantRepo
}

func setupQuotesImportTestHandlers() (*Handlers, *mockQuotesRepository, *mockTenantRepository, *mockContactsRepository) {
	h, quotesRepo, tenantRepo := setupQuotesTestHandlers()
	contactsRepo := newMockContactsRepository()
	h.contactsService = contacts.NewServiceWithRepository(contactsRepo)
	return h, quotesRepo, tenantRepo, contactsRepo
}

func TestListQuotes(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	quoteDate := time.Now()
	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   quoteDate,
		Status:      quotes.QuoteStatusDraft,
		Total:       decimal.NewFromInt(1000),
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(1000)},
		},
	}
	repo.quotes["quote-2"] = &quotes.Quote{
		ID:          "quote-2",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-002",
		ContactID:   "contact-2",
		QuoteDate:   quoteDate,
		Status:      quotes.QuoteStatusSent,
		Total:       decimal.NewFromInt(2000),
		Lines: []quotes.QuoteLine{
			{ID: "line-2", Description: "Test Item 2", Quantity: decimal.NewFromInt(2), UnitPrice: decimal.NewFromInt(1000)},
		},
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all quotes",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "filter by status",
			query:      "?status=DRAFT",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/quotes"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListQuotes(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []quotes.Quote
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestCreateQuote(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid quote",
			body: map[string]interface{}{
				"contact_id": "contact-1",
				"quote_date": time.Now().Format(time.RFC3339),
				"lines": []map[string]interface{}{
					{
						"description": "Test Item",
						"quantity":    "1",
						"unit_price":  "100.00",
						"vat_rate":    "20",
					},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, tenantRepo := setupQuotesTestHandlers()

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

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/quotes", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreateQuote(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusCreated {
				var result quotes.Quote
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, quotes.QuoteStatusDraft, result.Status)
			}
		})
	}
}

func TestImportQuotes(t *testing.T) {
	h, repo, tenantRepo, contactsRepo := setupQuotesImportTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	contact := contactsRepo.addTestContact("contact-1", "tenant-1", "Acme", contacts.ContactTypeCustomer, true)
	contact.Code = "CUST-1"

	csvContent := `quote_number,contact_code,quote_date,valid_until,status,line_description,quantity,unit_price,vat_rate
QT-100,CUST-1,2026-03-15,2026-04-15,sent,Consulting,2,100,22
`
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/import", map[string]interface{}{
		"file_name":   "quotes.csv",
		"csv_content": csvContent,
	}, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.ImportQuotes(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var result quotes.ImportQuotesResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "quotes.csv", result.FileName)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.QuotesCreated)
	assert.Equal(t, 1, result.LinesImported)
	assert.Zero(t, result.RowsSkipped)

	require.Len(t, repo.quotes, 1)
	for _, quote := range repo.quotes {
		assert.Equal(t, "QT-100", quote.QuoteNumber)
		assert.Equal(t, "contact-1", quote.ContactID)
		assert.Equal(t, quotes.QuoteStatusSent, quote.Status)
		assert.Equal(t, "user-1", quote.CreatedBy)
	}

	missingReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/import", map[string]interface{}{
		"file_name": "quotes.csv",
	}, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	missingReq = withURLParams(missingReq, map[string]string{"tenantID": "tenant-1"})
	missingResp := httptest.NewRecorder()
	h.ImportQuotes(missingResp, missingReq)

	assert.Equal(t, http.StatusBadRequest, missingResp.Code)
	assert.Contains(t, missingResp.Body.String(), "csv_content is required")
}

func TestGetQuote(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusDraft,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	tests := []struct {
		name       string
		quoteID    string
		wantStatus int
	}{
		{
			name:       "existing quote",
			quoteID:    "quote-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent quote",
			quoteID:    "quote-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/quotes/"+tt.quoteID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": tt.quoteID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetQuote(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestGetQuotePDF(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()
	h.pdfService = pdf.NewService()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		Contact:     &contacts.Contact{Name: "Acme OU", Email: "billing@example.com"},
		QuoteDate:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Status:      quotes.QuoteStatusDraft,
		Currency:    "EUR",
		Subtotal:    decimal.NewFromInt(100),
		VATAmount:   decimal.NewFromInt(22),
		Total:       decimal.NewFromInt(122),
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Consulting", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100), VATRate: decimal.NewFromInt(22), LineTotal: decimal.NewFromInt(122)},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/quotes/quote-1/pdf", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetQuotePDF(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), `quote-QT-001.pdf`)
	assert.True(t, bytes.HasPrefix(rr.Body.Bytes(), []byte("%PDF")))
}

func TestEmailQuoteMarksDraftSent(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()
	h.pdfService = pdf.NewService()
	emailRepo := &emailHandlerRepository{
		settings:  make(map[string][]byte),
		templates: make(map[string]email.EmailTemplate),
		logs:      []email.EmailLog{},
	}
	emailRepo.settings["tenant-1"] = []byte(`{
		"smtp_host":"smtp.example.com",
		"smtp_port":587,
		"smtp_username":"user@example.com",
		"smtp_password":"secret",
		"smtp_from_email":"billing@example.com",
		"smtp_from_name":"Billing",
		"smtp_use_tls":true
	}`)
	mailer := &emailHandlerMailer{}
	h.emailService = email.NewServiceWithRepository(emailRepo, mailer)
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	validUntil := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		Contact:     &contacts.Contact{Name: "Acme OU", Email: "billing@example.com"},
		QuoteDate:   time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		ValidUntil:  &validUntil,
		Status:      quotes.QuoteStatusDraft,
		Currency:    "EUR",
		Subtotal:    decimal.NewFromInt(100),
		VATAmount:   decimal.NewFromInt(22),
		Total:       decimal.NewFromInt(122),
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Consulting", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100), VATRate: decimal.NewFromInt(22), LineTotal: decimal.NewFromInt(122)},
		},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", email.SendQuoteRequest{
		RecipientEmail: "billing@example.com",
		RecipientName:  "Acme",
		Subject:        "Quote QT-001",
		Message:        "Please review.",
		AttachPDF:      true,
	}, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})

	rr := httptest.NewRecorder()
	h.EmailQuote(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var result email.EmailSentResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	assert.True(t, result.Success)
	assert.Equal(t, 1, mailer.sentCount)
	assert.Equal(t, quotes.QuoteStatusSent, repo.quotes["quote-1"].Status)
	require.NotEmpty(t, emailRepo.logs)
	assert.Equal(t, string(email.TemplateQuoteSend), emailRepo.logs[0].EmailType)
	assert.Equal(t, "quote-1", emailRepo.logs[0].RelatedID)
	assert.Equal(t, email.StatusSent, emailRepo.logs[0].Status)
}

func TestEmailQuoteRequiresApprovedEvidence(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()
	store, err := documents.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	documentRepo := newMockDocumentRepository()
	h.documentsService = documents.NewService(documentRepo, store)

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}
	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusDraft,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", map[string]any{
		"recipient_email":           "customer@example.com",
		"require_approved_evidence": true,
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})

	rr := httptest.NewRecorder()
	h.EmailQuote(rr, req)

	assertQuoteEvidenceConflict(t, rr, "quote-1")
	assert.Equal(t, quotes.QuoteStatusDraft, repo.quotes["quote-1"].Status)
}

func TestUpdateQuote(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusDraft,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	body := map[string]interface{}{
		"contact_id": "contact-1",
		"quote_date": time.Now().Format(time.RFC3339),
		"notes":      "Updated notes",
		"lines": []map[string]interface{}{
			{
				"description": "Updated Item",
				"quantity":    "2",
				"unit_price":  "150.00",
				"vat_rate":    "20",
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/quotes/quote-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.UpdateQuote(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result quotes.Quote
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Updated notes", result.Notes)
}

func TestDeleteQuote(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		Status:      quotes.QuoteStatusDraft,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	tests := []struct {
		name       string
		quoteID    string
		wantStatus int
	}{
		{
			name:       "delete existing quote",
			quoteID:    "quote-1",
			wantStatus: http.StatusNoContent, // Handler returns 204 on success
		},
		{
			name:       "delete non-existent quote",
			quoteID:    "quote-999",
			wantStatus: http.StatusBadRequest, // Handler returns 400 on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/quotes/"+tt.quoteID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": tt.quoteID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.DeleteQuote(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestSendQuote(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusDraft,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.SendQuote(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSendQuoteRequiresApprovedEvidence(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()
	store, err := documents.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	documentRepo := newMockDocumentRepository()
	h.documentsService = documents.NewService(documentRepo, store)

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}
	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusDraft,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", map[string]any{
		"require_approved_evidence": true,
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})

	rr := httptest.NewRecorder()
	h.SendQuote(rr, req)

	assertQuoteEvidenceConflict(t, rr, "quote-1")

	documentRepo.docs["doc-quote"] = &documents.Document{
		ID:           "doc-quote",
		TenantID:     "tenant-1",
		EntityType:   documents.EntityTypeQuote,
		EntityID:     "quote-1",
		DocumentType: documents.DocumentTypeContract,
		FileName:     "signed-offer.pdf",
		ReviewStatus: documents.ReviewStatusApproved,
		UploadedBy:   "user-1",
		CreatedAt:    time.Now(),
	}

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/send", map[string]any{
		"require_approved_evidence": true,
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	rr = httptest.NewRecorder()
	h.SendQuote(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func assertQuoteEvidenceConflict(t *testing.T, rr *httptest.ResponseRecorder, quoteID string) {
	t.Helper()

	assert.Equal(t, http.StatusConflict, rr.Code, "response body: %s", rr.Body.String())

	var conflict struct {
		Error                 string                                `json:"error"`
		EvidencePolicyResults []documents.EvidencePolicyResult      `json:"evidence_policy_results"`
		RemediationActions    []documents.DocumentRemediationAction `json:"remediation_actions"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&conflict))
	assert.Contains(t, conflict.Error, "approved quote evidence is required")
	require.Len(t, conflict.EvidencePolicyResults, 1)
	assert.Equal(t, documents.EntityTypeQuote, conflict.EvidencePolicyResults[0].EntityType)
	assert.Equal(t, quoteID, conflict.EvidencePolicyResults[0].EntityID)
	assert.False(t, conflict.EvidencePolicyResults[0].Compliant)
	require.Len(t, conflict.RemediationActions, 1)
	assert.Equal(t, "document_evidence_missing", conflict.RemediationActions[0].Code)
	assert.Equal(t, "oa documents upload --entity-type quote --entity-id "+quoteID+" --document-type contract --file <file>", conflict.RemediationActions[0].CLICommand)
}

func TestAcceptQuote(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusSent,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/accept", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.AcceptQuote(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRejectQuote(t *testing.T) {
	h, repo, tenantRepo := setupQuotesTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.quotes["quote-1"] = &quotes.Quote{
		ID:          "quote-1",
		TenantID:    "tenant-1",
		QuoteNumber: "QT-001",
		ContactID:   "contact-1",
		QuoteDate:   time.Now(),
		Status:      quotes.QuoteStatusSent,
		Lines: []quotes.QuoteLine{
			{ID: "line-1", Description: "Test Item", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/reject", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.RejectQuote(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestConvertQuoteToInvoice(t *testing.T) {
	h, quotesRepo, tenantRepo := setupQuotesTestHandlers()
	invoiceRepo := newMockInvoicingRepository()
	h.invoicingService = invoicing.NewServiceWithRepository(invoiceRepo, nil)

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	quoteDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	quotesRepo.quotes["quote-1"] = &quotes.Quote{
		ID:           "quote-1",
		TenantID:     "tenant-1",
		QuoteNumber:  "QT-001",
		ContactID:    "contact-1",
		QuoteDate:    quoteDate,
		Status:       quotes.QuoteStatusAccepted,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Notes:        "Quote notes",
		Lines: []quotes.QuoteLine{
			{
				ID:              "line-1",
				TenantID:        "tenant-1",
				QuoteID:         "quote-1",
				LineNumber:      1,
				Description:     "Consulting",
				Quantity:        decimal.NewFromInt(2),
				Unit:            "hour",
				UnitPrice:       decimal.NewFromInt(100),
				DiscountPercent: decimal.NewFromInt(10),
				VATRate:         decimal.NewFromInt(22),
			},
		},
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/tenants/tenant-1/quotes/quote-1/convert-to-invoice",
		bytes.NewBufferString(`{"issue_date":"2026-03-10T00:00:00Z","due_date":"2026-03-24T00:00:00Z","notes":"Invoice notes"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ConvertQuoteToInvoice(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var result quotes.QuoteInvoiceConversionResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	require.NotNil(t, result.Quote)
	require.NotNil(t, result.Invoice)
	assert.Equal(t, quotes.QuoteStatusConverted, result.Quote.Status)
	require.NotNil(t, result.Quote.ConvertedToInvoiceID)
	assert.Equal(t, result.Invoice.ID, *result.Quote.ConvertedToInvoiceID)
	assert.Equal(t, result.Invoice.ID, *quotesRepo.quotes["quote-1"].ConvertedToInvoiceID)
	assert.Equal(t, quotes.QuoteStatusConverted, quotesRepo.quotes["quote-1"].Status)

	assert.Equal(t, invoicing.InvoiceTypeSales, result.Invoice.InvoiceType)
	assert.Equal(t, invoicing.StatusDraft, result.Invoice.Status)
	assert.Equal(t, "contact-1", result.Invoice.ContactID)
	assert.Equal(t, "QT-001", result.Invoice.Reference)
	assert.Equal(t, "Invoice notes", result.Invoice.Notes)
	assert.Equal(t, "2026-03-10", result.Invoice.IssueDate.Format("2006-01-02"))
	assert.Equal(t, "2026-03-24", result.Invoice.DueDate.Format("2006-01-02"))
	require.Len(t, result.Invoice.Lines, 1)
	assert.Equal(t, "Consulting", result.Invoice.Lines[0].Description)
	assert.True(t, result.Invoice.Lines[0].Quantity.Equal(decimal.NewFromInt(2)))
	assert.True(t, result.Invoice.Lines[0].UnitPrice.Equal(decimal.NewFromInt(100)))
	assert.True(t, result.Invoice.Lines[0].DiscountPercent.Equal(decimal.NewFromInt(10)))
	assert.True(t, result.Invoice.Lines[0].VATRate.Equal(decimal.NewFromInt(22)))
}
