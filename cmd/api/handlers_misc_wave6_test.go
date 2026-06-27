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

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

func TestWave6TenantPluginHandlerBranches(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Wave 6 Tenant", "wave-6-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{
		{TenantID: tenantID.String(), UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true},
		{TenantID: tenantID.String(), UserID: "viewer-1", Role: tenant.RoleViewer, IsActive: true},
	}
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "wave6-plugin",
		DisplayName:        "Wave 6 Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/wave6-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(`{
			"name":"wave6-plugin",
			"display_name":"Wave 6 Plugin",
			"version":"1.0.0",
			"permissions":["routes:register"],
			"backend":{
				"runtime":"http",
				"base_url":"http://127.0.0.1:1",
				"routes":[{"method":"GET","path":"/status","handler":"/status"}]
			}
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}
	repo.tenantPlugins[pluginTenantKey(tenantID, pluginID)] = &plugin.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  json.RawMessage(`{"account":"1000"}`),
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	adminClaims := &auth.Claims{UserID: "admin-1", TenantID: tenantID.String(), Role: tenant.RoleAdmin}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins", nil, adminClaims)
	req = withURLParams(req, map[string]string{"tenantID": tenantID.String()})
	rec := httptest.NewRecorder()
	h.ListTenantPlugins(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tenantPlugins []plugin.TenantPlugin
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tenantPlugins))
	require.Len(t, tenantPlugins, 1)
	assert.True(t, tenantPlugins[0].IsEnabled)

	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins", nil, &auth.Claims{UserID: "missing-user"})
	req = withURLParams(req, map[string]string{"tenantID": tenantID.String()})
	rec = httptest.NewRecorder()
	h.ListTenantPlugins(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+uuid.NewString()+"/enable", nil, adminClaims)
	req = withURLParams(req, map[string]string{"tenantID": tenantID.String(), "pluginID": uuid.NewString()})
	rec = httptest.NewRecorder()
	h.EnableTenantPlugin(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/disable", nil), map[string]string{
		"tenantID": tenantID.String(),
		"pluginID": pluginID.String(),
	})
	rec = httptest.NewRecorder()
	h.DisableTenantPlugin(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

	req = makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", map[string]string{"account": "2000"}, &auth.Claims{UserID: "viewer-1", TenantID: tenantID.String(), Role: tenant.RoleViewer})
	req = withURLParams(req, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	rec = httptest.NewRecorder()
	h.UpdateTenantPluginSettings(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	req = httptest.NewRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", strings.NewReader("{"))
	req = req.WithContext(contextWithClaims(req.Context(), adminClaims))
	req = withURLParams(req, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	rec = httptest.NewRecorder()
	h.UpdateTenantPluginSettings(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/bad/plugins/"+pluginID.String()+"/runtime/status", nil, adminClaims)
	req = withURLParams(req, map[string]string{"tenantID": "bad", "pluginID": pluginID.String(), "*": "status"})
	rec = httptest.NewRecorder()
	h.InvokeTenantPluginRoute(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestWave6TaxHandlerAdditionalErrorBranches(t *testing.T) {
	h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
	tenantRecord.Settings.RegCode = "12345678"

	body := invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleImportKMDHistory, taxHandlerRequestWithBody(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/import-history",
		strings.NewReader("{"),
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid request body", body["error"])

	taxRepo.saveErr = errors.New("kmd save failed")
	importResult := invokeTaxHandlerJSON[tax.ImportKMDHistoryResult](t, http.StatusOK, h.HandleImportKMDHistory, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/import-history",
		tax.ImportKMDHistoryRequest{CSVContent: "year,month,row_code,tax_base,tax_amount\n2026,1,1,100,22\n"},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, importResult.Errors, 1)
	require.Contains(t, importResult.Errors[0].Message, "kmd save failed")
	taxRepo.saveErr = nil

	body = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateKMDINF, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/bad/2/inf",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "bad", "month": "2"},
	))
	require.Equal(t, "Invalid year", body["error"])

	body = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateKMDINF, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/bad/inf",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "bad"},
	))
	require.Equal(t, "Invalid month", body["error"])

	body = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=2019&quarter=1",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid year", body["error"])

	taxRepo.getDecl = nil
	body = invokeTaxHandlerJSON[map[string]string](t, http.StatusNotFound, h.HandleMarkKMDSubmitted, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/2026/2/submit",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "Declaration not found", body["error"])

	taxRepo.getDecl = &tax.KMDDeclaration{ID: "decl-wave6", TenantID: tenantRecord.ID, Year: 2026, Month: 2}
	docRepo := &wave6DocumentRepository{mockDocumentRepository: newMockDocumentRepository()}
	docRepo.listDocumentsErr = errors.New("evidence store failed")
	h.documentsService = documents.NewService(docRepo, nil)
	body = invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleMarkKMDAccepted, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/2026/2/accept",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "Failed to verify KMD acceptance evidence", body["error"])
}

type wave6DocumentRepository struct {
	*mockDocumentRepository
	listReviewQueueErr   error
	listRetentionErr     error
	listReviewSummaryErr error
	updateRetentionErr   error
}

func (r *wave6DocumentRepository) ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter documents.ReviewQueueFilter) ([]documents.Document, error) {
	if r.listReviewQueueErr != nil {
		return nil, r.listReviewQueueErr
	}
	return r.mockDocumentRepository.ListReviewQueueDocuments(ctx, schemaName, tenantID, filter)
}

func (r *wave6DocumentRepository) ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]documents.Document, error) {
	if r.listRetentionErr != nil {
		return nil, r.listRetentionErr
	}
	return r.mockDocumentRepository.ListRetentionReviewDocuments(ctx, schemaName, tenantID, cutoff, includeMissing)
}

func (r *wave6DocumentRepository) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]documents.ReviewSummary, error) {
	if r.listReviewSummaryErr != nil {
		return nil, r.listReviewSummaryErr
	}
	return r.mockDocumentRepository.ListReviewSummaries(ctx, schemaName, tenantID, entityType, entityIDs)
}

func (r *wave6DocumentRepository) UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error {
	if r.updateRetentionErr != nil {
		return r.updateRetentionErr
	}
	return r.mockDocumentRepository.UpdateDocumentRetention(ctx, schemaName, tenantID, documentID, retentionUntil)
}

func TestWave6DocumentHandlerServiceErrorBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")
	docRepo := &wave6DocumentRepository{mockDocumentRepository: newMockDocumentRepository()}
	h, _ := setupDocumentHandlers(t)
	h.documentsService = documents.NewService(docRepo, nil)

	docRepo.listReviewSummaryErr = errors.New("summaries failed")
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/review-summary", map[string]any{
		"entity_type": documents.EntityTypeExpense,
		"entity_ids":  []string{"expense-1"},
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec := httptest.NewRecorder()
	h.ListDocumentReviewSummaries(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	docRepo.listReviewSummaryErr = nil

	docRepo.listReviewQueueErr = errors.New("queue failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/review-queue?review_status=ALL", nil, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.GetDocumentReviewQueue(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	docRepo.listReviewQueueErr = nil

	docRepo.listDocumentsErr = errors.New("evidence failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/evidence-policy", documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeExpense,
		EntityIDs:  []string{"expense-1"},
		Rules:      []documents.EvidencePolicyRule{{DocumentTypes: []string{documents.DocumentTypeReceipt}, MinCount: 1}},
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.EvaluateDocumentEvidencePolicy(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	docRepo.listDocumentsErr = nil

	docRepo.listRetentionErr = errors.New("retention failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/retention?as_of=2026-03-15", nil, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.GetDocumentRetentionReview(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	docRepo.listRetentionErr = nil

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/purge", map[string]any{
		"as_of": "2026/03/15",
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.PurgeExpiredDocuments(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/purge", map[string]any{
		"limit": -1,
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.PurgeExpiredDocuments(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	docRepo.docs["doc-1"] = &documents.Document{ID: "doc-1", TenantID: "tenant-1", EntityType: documents.EntityTypeExpense, EntityID: "expense-1", DocumentType: documents.DocumentTypeReceipt}
	docRepo.updateRetentionErr = errors.New("retention update failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/retention", map[string]any{
		"retention_until": "2027-03-15",
	}, claims), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"})
	rec = httptest.NewRecorder()
	h.UpdateDocumentRetention(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPatch, "/tenants/tenant-1/documents/missing/legal-hold", documents.DocumentLegalHoldRequest{
		LegalHold: true,
		Note:      "litigation",
	}, claims), map[string]string{"tenantID": "tenant-1", "documentID": "missing"})
	rec = httptest.NewRecorder()
	h.UpdateDocumentLegalHold(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/missing/mark-reviewed", nil, claims), map[string]string{"tenantID": "tenant-1", "documentID": "missing"})
	rec = httptest.NewRecorder()
	h.MarkDocumentReviewed(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())

	rec = httptest.NewRecorder()
	respondDocumentError(rec, errors.New("document not found"))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestWave6RecurringHandlerAdditionalBranches(t *testing.T) {
	h, tenantRepo, recurringRepo, _ := setupRecurringTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	recurringRepo.listErr = errors.New("list failed")
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/recurring-invoices", nil, claims), map[string]string{"tenantID": "tenant-1"})
	rec := httptest.NewRecorder()
	h.ListRecurringInvoices(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	recurringRepo.listErr = nil

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices", recurring.CreateRecurringInvoiceRequest{}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.CreateRecurringInvoice(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/import", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.ImportRecurringInvoices(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	hImport, tenantRepoImport, _, contactsRepo := setupRecurringImportTestHandlers()
	tenantRepoImport.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
	contactsRepo.listErr = errors.New("contacts failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/import", recurring.ImportRecurringInvoicesRequest{
		CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price\nTemplate,CUST-1,MONTHLY,2026-01-01,Line,1,10\n",
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	hImport.ImportRecurringInvoices(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/from-invoice/invoice-1", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "invoiceID": "invoice-1"})
	rec = httptest.NewRecorder()
	h.CreateRecurringInvoiceFromInvoice(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/recurring-invoices/rec-1", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
	rec = httptest.NewRecorder()
	h.UpdateRecurringInvoice(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/missing/resume", nil, claims), map[string]string{"tenantID": "tenant-1", "recurringID": "missing"})
	rec = httptest.NewRecorder()
	h.ResumeRecurringInvoice(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/missing/generate", nil, claims), map[string]string{"tenantID": "tenant-1", "recurringID": "missing"})
	rec = httptest.NewRecorder()
	h.GenerateRecurringInvoice(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

type wave6WebhookRepository struct {
	*memoryWebhookRepository
	listErr           error
	updateErr         error
	listDeliveriesErr error
}

func (r *wave6WebhookRepository) ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]webhooks.Endpoint, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.memoryWebhookRepository.ListEndpoints(ctx, tenantID, activeOnly)
}

func (r *wave6WebhookRepository) UpdateEndpoint(ctx context.Context, endpoint *webhooks.Endpoint) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.memoryWebhookRepository.UpdateEndpoint(ctx, endpoint)
}

func (r *wave6WebhookRepository) ListDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]webhooks.Delivery, error) {
	if r.listDeliveriesErr != nil {
		return nil, r.listDeliveriesErr
	}
	return r.memoryWebhookRepository.ListDeliveries(ctx, tenantID, endpointID, limit)
}

func TestWave6WebhookHandlerErrorAndEmitBranches(t *testing.T) {
	repo := &wave6WebhookRepository{memoryWebhookRepository: newMemoryWebhookRepository()}
	h := &Handlers{webhookService: webhooks.NewServiceWithRepository(repo, nil)}

	repo.listErr = errors.New("list failed")
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/webhooks", nil, nil), map[string]string{"tenantID": "tenant-1"})
	rec := httptest.NewRecorder()
	h.ListWebhookEndpoints(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	repo.listErr = nil

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.CreateWebhookEndpoint(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	active := true
	endpoint, err := h.webhookService.CreateEndpoint(context.Background(), "tenant-1", &webhooks.CreateEndpointRequest{
		Name:     "Wave 6",
		URL:      "http://127.0.0.1:1",
		Events:   []string{plugin.EventExpenseCreated},
		IsActive: &active,
	})
	require.NoError(t, err)

	repo.updateErr = errors.New("update failed")
	name := "Renamed"
	req = withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/webhooks/"+endpoint.ID, webhooks.UpdateEndpointRequest{Name: &name}, nil), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID})
	rec = httptest.NewRecorder()
	h.UpdateWebhookEndpoint(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	repo.updateErr = nil

	req = withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/webhooks/missing", nil, nil), map[string]string{"tenantID": "tenant-1", "webhookID": "missing"})
	rec = httptest.NewRecorder()
	h.DeleteWebhookEndpoint(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/webhooks/"+endpoint.ID+"/deliveries?limit=0", nil, nil), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID})
	rec = httptest.NewRecorder()
	h.ListWebhookDeliveries(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	repo.listDeliveriesErr = errors.New("deliveries failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/webhooks/"+endpoint.ID+"/deliveries", nil, nil), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID})
	rec = httptest.NewRecorder()
	h.ListWebhookDeliveries(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks/"+endpoint.ID+"/test", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID})
	rec = httptest.NewRecorder()
	h.TestWebhookEndpoint(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var nilHandlers *Handlers
	nilHandlers.emitWebhookEvent(plugin.EventExpenseCreated, "tenant-1", map[string]string{"id": "expense-1"})
	h.emitWebhookEvent(plugin.EventExpenseCreated, "tenant-1", func() {})
	h.emitWebhookEvent(plugin.EventExpenseCreated, "tenant-1", map[string]string{"id": "expense-1"})
}

func TestWave6PeriodCloseAdditionalBranches(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant", "tenant")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner}}
	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", Role: tenant.RoleOwner}

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/period-close", strings.NewReader("{"))
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rec := httptest.NewRecorder()
	h.ClosePeriod(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	repo.getTenantErr = tenant.ErrTenantNotFound
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-close", map[string]any{
		"period_end_date": "2026-01-31",
		"note":            "Month close",
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.ClosePeriod(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	repo.getTenantErr = nil

	req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", strings.NewReader("{"))
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.ReopenPeriod(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = httptest.NewRecorder()
	respondPeriodCloseError(rec, errors.New("no closed period to reopen"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	rec = httptest.NewRecorder()
	respondPeriodCloseError(rec, errors.New("inventory costing review has blocking exception lines"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestWave6APITokenAdditionalBranches(t *testing.T) {
	h, repo := setupAPITokenHandlers()
	expiresAt := time.Now().Add(time.Hour)
	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}

	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", map[string]any{
		"name":       "expiring token",
		"expires_at": expiresAt,
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec := httptest.NewRecorder()
	h.CreateAPIToken(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, repo.tokens, 1)
	auditEvents := h.securityAuditService.(*mockSecurityAuditService).events
	require.NotEmpty(t, auditEvents)
	require.NotEmpty(t, auditEvents[0].Metadata["expires_at"])

	hTenant, tenantRepo, apiTokenRepo := setupTenantUserAPITokenHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	tenantRepo.addTestUser("user-2", "target@example.com", "Target", "password", true)
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true, CreatedAt: time.Now()},
		{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleViewer, IsActive: true, CreatedAt: time.Now()},
	}
	adminClaims := &auth.Claims{UserID: "admin-1", Email: "admin@example.com", TenantID: "tenant-1", Role: tenant.RoleAdmin}

	hTenant.apiTokenService = apitoken.NewServiceWithRepository(nil)
	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/api-tokens", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
	rec = httptest.NewRecorder()
	hTenant.ListTenantUserAPITokens(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())

	hTenant.apiTokenService = nil
	req = withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/api-tokens/token-1", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "user-2", "tokenID": "token-1"})
	rec = httptest.NewRecorder()
	hTenant.RevokeTenantUserAPIToken(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())

	hTenant.apiTokenService = apitoken.NewServiceWithRepository(apiTokenRepo)
	apiTokenRepo.tokens["token-1"] = &apitoken.APIToken{ID: "token-1", UserID: "user-2", TenantID: "tenant-1", Name: "Target token", TokenPrefix: "oa_target", CreatedAt: time.Now()}
	tenantRepo.createAuditEventErr = errors.New("audit write failed")
	req = withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/api-tokens/token-1", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "user-2", "tokenID": "token-1"})
	rec = httptest.NewRecorder()
	hTenant.RevokeTenantUserAPIToken(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestWave6ExpenseHandlerAdditionalBranches(t *testing.T) {
	h, repo, _ := setupExpenseHandlers()
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	repo.expenses["expense-1"] = &expenses.Expense{
		ID:               "expense-1",
		TenantID:         "tenant-1",
		Merchant:         "Office Store",
		Status:           expenses.StatusDraft,
		ExpenseDate:      time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(10),
	}
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses?limit=1", nil, claims), map[string]string{"tenantID": "tenant-1"})
	rec := httptest.NewRecorder()
	h.ListExpenses(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/expenses?limit=-1", nil, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.ListExpenses(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/expenses", strings.NewReader("{")).WithContext(contextWithClaims(context.Background(), claims)), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.CreateExpense(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	tenantRecord, err := h.tenantService.GetTenant(context.Background(), "tenant-1")
	require.NoError(t, err)
	tenantRecord.Settings.PeriodLockDate = stringPtr("2026-06-30")
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses", expenses.CreateExpenseRequest{
		ExpenseDate:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Merchant:         "Locked Merchant",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(15),
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.CreateExpense(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	tenantRecord.Settings.PeriodLockDate = stringPtr("not-a-date")
	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", expenses.ImportExpensesRequest{
		CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount,status\nEXP-1,2026-07-01,Office,expense-account,cash-account,10,DRAFT\n",
	}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.ImportExpenses(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	tenantRecord.Settings.PeriodLockDate = nil

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", expenses.ImportExpensesRequest{}, claims), map[string]string{"tenantID": "tenant-1"})
	rec = httptest.NewRecorder()
	h.ImportExpenses(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/expenses/expense-1/reject", strings.NewReader("{")).WithContext(contextWithClaims(context.Background(), claims)), map[string]string{"tenantID": "tenant-1", "expenseID": "expense-1"})
	rec = httptest.NewRecorder()
	h.RejectExpense(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/missing/post", nil, claims), map[string]string{"tenantID": "tenant-1", "expenseID": "missing"})
	rec = httptest.NewRecorder()
	h.PostExpense(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

type wave6PayrollImportRepository struct {
	*payrollImportHandlerRepository
	createEmployeeErr       error
	createPayrollRunErr     error
	createTSDDeclarationErr error
}

func (r *wave6PayrollImportRepository) CreateEmployee(ctx context.Context, schemaName string, emp *payroll.Employee) error {
	if r.createEmployeeErr != nil {
		return r.createEmployeeErr
	}
	return r.payrollImportHandlerRepository.CreateEmployee(ctx, schemaName, emp)
}

func (r *wave6PayrollImportRepository) CreatePayrollRun(ctx context.Context, schemaName string, run *payroll.PayrollRun) error {
	if r.createPayrollRunErr != nil {
		return r.createPayrollRunErr
	}
	return r.payrollImportHandlerRepository.CreatePayrollRun(ctx, schemaName, run)
}

func (r *wave6PayrollImportRepository) CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *payroll.TSDDeclaration) error {
	if r.createTSDDeclarationErr != nil {
		return r.createTSDDeclarationErr
	}
	return r.payrollImportHandlerRepository.CreateTSDDeclaration(ctx, schemaName, declaration)
}

func (r *wave6PayrollImportRepository) WithTransaction(ctx context.Context, fn func(txRepo payroll.Repository) error) error {
	return fn(r)
}

func TestWave6PayrollImportHandlerServiceErrorBranches(t *testing.T) {
	h, _, absenceRepo := setupPayrollImportHandlerTest(t)
	repo := &wave6PayrollImportRepository{
		payrollImportHandlerRepository: newPayrollImportHandlerRepository(),
		createEmployeeErr:              errors.New("employee store failed"),
	}
	h.payrollService = payroll.NewServiceWithRepository(repo, &payrollImportHandlerIDGenerator{prefix: "wave6"})

	employeeResult := invokePayrollImportJSON[payroll.ImportEmployeesResult](t, http.StatusOK, h.ImportEmployees, payrollImportRequest(
		http.MethodPost,
		"/tenants/tenant-1/employees/import",
		map[string]any{"csv_content": "employee_number,first_name,last_name,start_date\nEMP-1,Mari,Maasikas,2026-01-01\n"},
	))
	require.Len(t, employeeResult.Errors, 1)
	require.Contains(t, employeeResult.Errors[0].Message, "employee store failed")

	repo = &wave6PayrollImportRepository{
		payrollImportHandlerRepository: newPayrollImportHandlerRepository(),
		createPayrollRunErr:            errors.New("run store failed"),
	}
	repo.seedEmployee(payrollImportEmployee("emp-1", "EMP-1"))
	h.payrollService = payroll.NewServiceWithRepository(repo, &payrollImportHandlerIDGenerator{prefix: "wave6"})
	historyResult := invokePayrollImportJSON[payroll.ImportPayrollHistoryResult](t, http.StatusOK, h.ImportPayrollHistory, payrollImportRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs/import-history",
		map[string]any{"csv_content": "period_year,period_month,employee_number,gross_salary\n2026,1,EMP-1,1000\n"},
	))
	require.Len(t, historyResult.Errors, 1)
	require.Contains(t, historyResult.Errors[0].Message, "run store failed")

	repo = &wave6PayrollImportRepository{
		payrollImportHandlerRepository: newPayrollImportHandlerRepository(),
		createTSDDeclarationErr:        errors.New("tsd store failed"),
	}
	repo.seedEmployee(payrollImportEmployee("emp-1", "EMP-1"))
	h.payrollService = payroll.NewServiceWithRepository(repo, &payrollImportHandlerIDGenerator{prefix: "wave6"})
	tsdResult := invokePayrollImportJSON[payroll.ImportTSDHistoryResult](t, http.StatusOK, h.ImportTSDHistory, payrollImportRequest(
		http.MethodPost,
		"/tenants/tenant-1/tsd/import-history",
		map[string]any{"csv_content": "year,month,employee_number,gross_payment\n2026,1,EMP-1,1000\n"},
	))
	require.Len(t, tsdResult.Errors, 1)
	require.Contains(t, tsdResult.Errors[0].Message, "tsd store failed")

	absenceRepo.ListEmployeesErr = errors.New("leave employees failed")
	body := invokePayrollImportJSON[map[string]string](t, http.StatusBadRequest, h.ImportLeaveBalances, payrollImportRequest(
		http.MethodPost,
		"/tenants/tenant-1/leave-balances/import",
		map[string]any{"csv_content": "year,employee_number,absence_type_code\n2026,EMP-1,ANNUAL\n"},
	))
	require.Contains(t, body["error"], "leave employees failed")
}
