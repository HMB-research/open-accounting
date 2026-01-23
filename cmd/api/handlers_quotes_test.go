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
