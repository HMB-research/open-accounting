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
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type mockRecurringRepository struct {
	invoices                 map[string]*recurring.RecurringInvoice
	lines                    map[string][]recurring.RecurringInvoiceLine
	dueRecurringInvoiceIDs   []string
	ensureSchemaErr          error
	createErr                error
	createLineErr            error
	getErr                   error
	getLinesErr              error
	listErr                  error
	updateErr                error
	deleteLinesErr           error
	deleteErr                error
	setActiveErr             error
	getDueRecurringIDsErr    error
	updateAfterGenerationErr error
}

func newMockRecurringRepository() *mockRecurringRepository {
	return &mockRecurringRepository{
		invoices: make(map[string]*recurring.RecurringInvoice),
		lines:    make(map[string][]recurring.RecurringInvoiceLine),
	}
}

func (m *mockRecurringRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	return m.ensureSchemaErr
}

func (m *mockRecurringRepository) Create(ctx context.Context, schemaName string, ri *recurring.RecurringInvoice) error {
	if m.createErr != nil {
		return m.createErr
	}
	copyInvoice := *ri
	m.invoices[ri.ID] = &copyInvoice
	return nil
}

func (m *mockRecurringRepository) CreateLine(ctx context.Context, schemaName string, line *recurring.RecurringInvoiceLine) error {
	if m.createLineErr != nil {
		return m.createLineErr
	}
	lineCopy := *line
	m.lines[line.RecurringInvoiceID] = append(m.lines[line.RecurringInvoiceID], lineCopy)
	return nil
}

func (m *mockRecurringRepository) GetByID(ctx context.Context, schemaName, tenantID, id string) (*recurring.RecurringInvoice, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	ri, ok := m.invoices[id]
	if !ok || ri.TenantID != tenantID {
		return nil, recurring.ErrRecurringInvoiceNotFound
	}
	copyInvoice := *ri
	return &copyInvoice, nil
}

func (m *mockRecurringRepository) GetLines(ctx context.Context, schemaName, recurringInvoiceID string) ([]recurring.RecurringInvoiceLine, error) {
	if m.getLinesErr != nil {
		return nil, m.getLinesErr
	}
	lines := m.lines[recurringInvoiceID]
	result := make([]recurring.RecurringInvoiceLine, len(lines))
	copy(result, lines)
	return result, nil
}

func (m *mockRecurringRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]recurring.RecurringInvoice, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]recurring.RecurringInvoice, 0, len(m.invoices))
	for _, ri := range m.invoices {
		if ri.TenantID != tenantID {
			continue
		}
		if activeOnly && !ri.IsActive {
			continue
		}
		result = append(result, *ri)
	}
	return result, nil
}

func (m *mockRecurringRepository) Update(ctx context.Context, schemaName string, ri *recurring.RecurringInvoice) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.invoices[ri.ID]; !ok {
		return recurring.ErrRecurringInvoiceNotFound
	}
	copyInvoice := *ri
	m.invoices[ri.ID] = &copyInvoice
	return nil
}

func (m *mockRecurringRepository) DeleteLines(ctx context.Context, schemaName, recurringInvoiceID string) error {
	if m.deleteLinesErr != nil {
		return m.deleteLinesErr
	}
	delete(m.lines, recurringInvoiceID)
	return nil
}

func (m *mockRecurringRepository) Delete(ctx context.Context, schemaName, tenantID, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	ri, ok := m.invoices[id]
	if !ok || ri.TenantID != tenantID {
		return recurring.ErrRecurringInvoiceNotFound
	}
	delete(m.invoices, id)
	delete(m.lines, id)
	return nil
}

func (m *mockRecurringRepository) SetActive(ctx context.Context, schemaName, tenantID, id string, active bool) error {
	if m.setActiveErr != nil {
		return m.setActiveErr
	}
	ri, ok := m.invoices[id]
	if !ok || ri.TenantID != tenantID {
		return recurring.ErrRecurringInvoiceNotFound
	}
	ri.IsActive = active
	return nil
}

func (m *mockRecurringRepository) GetDueRecurringInvoiceIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error) {
	if m.getDueRecurringIDsErr != nil {
		return nil, m.getDueRecurringIDsErr
	}
	if len(m.dueRecurringInvoiceIDs) > 0 {
		return append([]string(nil), m.dueRecurringInvoiceIDs...), nil
	}
	var ids []string
	for id, ri := range m.invoices {
		if ri.TenantID != tenantID || !ri.IsActive {
			continue
		}
		if !ri.NextGenerationDate.After(asOfDate) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (m *mockRecurringRepository) UpdateAfterGeneration(ctx context.Context, schemaName, tenantID, id string, nextDate time.Time, generatedAt time.Time) error {
	if m.updateAfterGenerationErr != nil {
		return m.updateAfterGenerationErr
	}
	ri, ok := m.invoices[id]
	if !ok || ri.TenantID != tenantID {
		return recurring.ErrRecurringInvoiceNotFound
	}
	ri.NextGenerationDate = nextDate
	ri.LastGeneratedAt = &generatedAt
	ri.GeneratedCount++
	return nil
}

func (m *mockRecurringRepository) UpdateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, sentAt *time.Time, status, logID string) error {
	return nil
}

type mockRecurringInvoicingService struct {
	getByIDInvoice *invoicing.Invoice
	getByIDErr     error
	createErr      error
	createRequests []*invoicing.CreateInvoiceRequest
}

func (m *mockRecurringInvoicingService) GetByID(ctx context.Context, tenantID, schemaName, id string) (*invoicing.Invoice, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if m.getByIDInvoice == nil {
		return nil, errors.New("invoice not found")
	}
	return m.getByIDInvoice, nil
}

func (m *mockRecurringInvoicingService) Create(ctx context.Context, tenantID, schemaName string, req *invoicing.CreateInvoiceRequest) (*invoicing.Invoice, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.createRequests = append(m.createRequests, req)
	invoice := &invoicing.Invoice{
		ID:            "generated-invoice-1",
		TenantID:      tenantID,
		InvoiceNumber: "INV-GEN-001",
		InvoiceType:   req.InvoiceType,
		ContactID:     req.ContactID,
		IssueDate:     req.IssueDate,
		DueDate:       req.DueDate,
		Currency:      req.Currency,
		ExchangeRate:  req.ExchangeRate,
		Lines:         make([]invoicing.InvoiceLine, 0, len(req.Lines)),
		Status:        invoicing.StatusDraft,
		CreatedBy:     req.UserID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	for i, line := range req.Lines {
		invoice.Lines = append(invoice.Lines, invoicing.InvoiceLine{
			ID:              "line-generated-" + string(rune('a'+i)),
			LineNumber:      i + 1,
			Description:     line.Description,
			Quantity:        line.Quantity,
			Unit:            line.Unit,
			UnitPrice:       line.UnitPrice,
			DiscountPercent: line.DiscountPercent,
			VATRate:         line.VATRate,
			AccountID:       line.AccountID,
			ProductID:       line.ProductID,
		})
	}
	invoice.Calculate()
	return invoice, nil
}

func setupRecurringTestHandlers() (*Handlers, *mockTenantRepository, *mockRecurringRepository, *mockRecurringInvoicingService) {
	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)
	recurringRepo := newMockRecurringRepository()
	invoicingSvc := &mockRecurringInvoicingService{}

	return &Handlers{
		tenantService:    tenantSvc,
		recurringService: recurring.NewServiceWithDependencies(recurringRepo, invoicingSvc, nil, nil, tenantSvc, nil),
	}, tenantRepo, recurringRepo, invoicingSvc
}

func seedRecurringInvoice(repo *mockRecurringRepository, tenantID, recurringID string) *recurring.RecurringInvoice {
	startDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nextDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	ri := &recurring.RecurringInvoice{
		ID:                 recurringID,
		TenantID:           tenantID,
		Name:               "Monthly Retainer",
		ContactID:          "contact-1",
		ContactName:        "Example Customer",
		InvoiceType:        "SALES",
		Currency:           "EUR",
		Frequency:          recurring.FrequencyMonthly,
		StartDate:          startDate,
		NextGenerationDate: nextDate,
		PaymentTermsDays:   14,
		Reference:          "RET-001",
		Notes:              "Recurring services",
		IsActive:           true,
		CreatedAt:          startDate,
		CreatedBy:          "user-1",
		UpdatedAt:          startDate,
		Lines: []recurring.RecurringInvoiceLine{{
			ID:                 "line-1",
			RecurringInvoiceID: recurringID,
			LineNumber:         1,
			Description:        "Accounting support",
			Quantity:           decimal.NewFromInt(1),
			Unit:               "month",
			UnitPrice:          decimal.NewFromInt(100),
			VATRate:            decimal.NewFromInt(22),
		}},
	}
	repo.invoices[recurringID] = ri
	repo.lines[recurringID] = append([]recurring.RecurringInvoiceLine(nil), ri.Lines...)
	return ri
}

func TestRecurringHandlers(t *testing.T) {
	h, tenantRepo, recurringRepo, invoicingSvc := setupRecurringTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}

	existing := seedRecurringInvoice(recurringRepo, "tenant-1", "rec-1")

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/recurring-invoices?active_only=true", nil), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.ListRecurringInvoices(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var listed []recurring.RecurringInvoice
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&listed))
	require.Len(t, listed, 1)
	assert.Equal(t, existing.ID, listed[0].ID)

	createReq := map[string]any{
		"name":               "Weekly bookkeeping",
		"contact_id":         "contact-2",
		"invoice_type":       "SALES",
		"currency":           "EUR",
		"frequency":          "WEEKLY",
		"start_date":         time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		"payment_terms_days": 7,
		"reference":          "WK-001",
		"notes":              "Weekly work",
		"lines": []map[string]any{{
			"description": "Weekly service",
			"quantity":    "1",
			"unit":        "week",
			"unit_price":  "250",
			"vat_rate":    "22",
		}},
	}
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices", createReq, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateRecurringInvoice(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var created recurring.RecurringInvoice
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	assert.Equal(t, "Weekly bookkeeping", created.Name)
	assert.Equal(t, claims.UserID, created.CreatedBy)
	require.NotEmpty(t, recurringRepo.invoices[created.ID])

	invoicingSvc.getByIDInvoice = &invoicing.Invoice{
		ID:          "invoice-1",
		TenantID:    "tenant-1",
		ContactID:   "contact-3",
		InvoiceType: invoicing.InvoiceTypeSales,
		IssueDate:   time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		DueDate:     time.Date(2026, 1, 24, 0, 0, 0, 0, time.UTC),
		Currency:    "EUR",
		Lines: []invoicing.InvoiceLine{{
			ID:          "invoice-line-1",
			LineNumber:  1,
			Description: "Retainer",
			Quantity:    decimal.NewFromInt(1),
			Unit:        "month",
			UnitPrice:   decimal.NewFromInt(500),
			VATRate:     decimal.NewFromInt(22),
		}},
	}
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/from-invoice/invoice-1", map[string]any{
		"name":               "From invoice template",
		"frequency":          "MONTHLY",
		"start_date":         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		"payment_terms_days": 14,
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "invoice-1"})
	rr = httptest.NewRecorder()
	h.CreateRecurringInvoiceFromInvoice(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/recurring-invoices/rec-1", nil), map[string]string{
		"tenantID":    "tenant-1",
		"recurringID": "rec-1",
	})
	rr = httptest.NewRecorder()
	h.GetRecurringInvoice(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var fetched recurring.RecurringInvoice
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&fetched))
	assert.Equal(t, "Monthly Retainer", fetched.Name)
	require.Len(t, fetched.Lines, 1)

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/recurring-invoices/rec-1", map[string]any{
		"name":               "Monthly Retainer Updated",
		"payment_terms_days": 21,
		"lines": []map[string]any{{
			"description": "Updated support",
			"quantity":    "2",
			"unit":        "month",
			"unit_price":  "120",
			"vat_rate":    "22",
		}},
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
	rr = httptest.NewRecorder()
	h.UpdateRecurringInvoice(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "Monthly Retainer Updated", recurringRepo.invoices["rec-1"].Name)
	require.Len(t, recurringRepo.lines["rec-1"], 1)
	assert.Equal(t, "Updated support", recurringRepo.lines["rec-1"][0].Description)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/rec-1/pause", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
	rr = httptest.NewRecorder()
	h.PauseRecurringInvoice(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, recurringRepo.invoices["rec-1"].IsActive)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/rec-1/resume", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
	rr = httptest.NewRecorder()
	h.ResumeRecurringInvoice(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.True(t, recurringRepo.invoices["rec-1"].IsActive)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/rec-1/generate", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
	rr = httptest.NewRecorder()
	h.GenerateRecurringInvoice(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var generated recurring.GenerationResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&generated))
	assert.Equal(t, "rec-1", generated.RecurringInvoiceID)
	assert.Equal(t, "generated-invoice-1", generated.GeneratedInvoiceID)
	require.Len(t, invoicingSvc.createRequests, 1)

	recurringRepo.dueRecurringInvoiceIDs = []string{"rec-1"}
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/generate-due", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GenerateDueRecurringInvoices(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var generatedDue []recurring.GenerationResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&generatedDue))
	require.Len(t, generatedDue, 1)
	assert.Equal(t, "rec-1", generatedDue[0].RecurringInvoiceID)

	req = makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/recurring-invoices/rec-1", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
	rr = httptest.NewRecorder()
	h.DeleteRecurringInvoice(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	_, exists := recurringRepo.invoices["rec-1"]
	assert.False(t, exists)
}

func TestRecurringHandlersErrorPaths(t *testing.T) {
	h, tenantRepo, recurringRepo, _ := setupRecurringTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.CreateRecurringInvoice(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/recurring-invoices/missing", nil), map[string]string{
		"tenantID":    "tenant-1",
		"recurringID": "missing",
	})
	rr = httptest.NewRecorder()
	h.GetRecurringInvoice(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())

	req = makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/recurring-invoices/missing", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "recurringID": "missing"})
	rr = httptest.NewRecorder()
	h.DeleteRecurringInvoice(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	recurringRepo.getDueRecurringIDsErr = errors.New("due list failed")
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/generate-due", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GenerateDueRecurringInvoices(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Failed to generate invoices")
}
