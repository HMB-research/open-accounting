package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

type wave9PluginRepository struct {
	*pluginHandlerRepository
	getTenantPluginsErr      error
	returnEmptyTenantPlugins bool
}

func (r *wave9PluginRepository) GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]plugin.TenantPlugin, error) {
	if r.getTenantPluginsErr != nil {
		return nil, r.getTenantPluginsErr
	}
	if r.returnEmptyTenantPlugins {
		return []plugin.TenantPlugin{}, nil
	}
	return r.pluginHandlerRepository.GetTenantPluginsWithAll(ctx, tenantID)
}

func TestWave9PluginTenantValidationAndReloadErrors(t *testing.T) {
	t.Run("admin uninstall maps valid missing plugin id", func(t *testing.T) {
		h, _ := setupPluginTestHandlers(t)
		req := withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugins/"+uuid.NewString(), nil), map[string]string{"id": uuid.NewString()})
		rec := httptest.NewRecorder()

		h.UninstallPlugin(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "plugin not found")
	})

	t.Run("tenant plugin validation rejects unauthenticated and malformed route ids", func(t *testing.T) {
		h, repo, tenantID, pluginID, userID := wave9PluginHandlers(t)
		claims := createTestClaims(userID, "user@example.com", tenantID.String(), tenant.RoleAdmin)

		for _, tt := range []struct {
			name       string
			handler    func(http.ResponseWriter, *http.Request)
			request    *http.Request
			wantStatus int
			wantBody   string
		}{
			{
				name:       "settings requires auth",
				handler:    h.GetTenantPluginSettings,
				request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", nil), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()}),
				wantStatus: http.StatusUnauthorized,
				wantBody:   "Not authenticated",
			},
			{
				name:       "settings rejects bad tenant id",
				handler:    h.GetTenantPluginSettings,
				request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/bad/plugins/"+pluginID.String()+"/settings", nil, claims), map[string]string{"tenantID": "bad", "pluginID": pluginID.String()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid tenant ID",
			},
			{
				name:       "update settings requires auth",
				handler:    h.UpdateTenantPluginSettings,
				request:    withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", strings.NewReader(`{}`)), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()}),
				wantStatus: http.StatusUnauthorized,
				wantBody:   "Not authenticated",
			},
			{
				name:       "update settings rejects bad tenant id",
				handler:    h.UpdateTenantPluginSettings,
				request:    withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/bad/plugins/"+pluginID.String()+"/settings", map[string]string{"account": "2000"}, claims), map[string]string{"tenantID": "bad", "pluginID": pluginID.String()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid tenant ID",
			},
			{
				name:       "disable rejects bad tenant id",
				handler:    h.DisableTenantPlugin,
				request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/bad/plugins/"+pluginID.String()+"/disable", nil, claims), map[string]string{"tenantID": "bad", "pluginID": pluginID.String()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid tenant ID",
			},
			{
				name:       "invoke requires auth",
				handler:    h.InvokeTenantPluginRoute,
				request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"}),
				wantStatus: http.StatusUnauthorized,
				wantBody:   "Not authenticated",
			},
			{
				name:       "invoke rejects bad tenant id",
				handler:    h.InvokeTenantPluginRoute,
				request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/bad/plugins/"+pluginID.String()+"/runtime/status", nil, claims), map[string]string{"tenantID": "bad", "pluginID": pluginID.String(), "*": "status"}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid tenant ID",
			},
			{
				name:       "invoke rejects bad plugin id",
				handler:    h.InvokeTenantPluginRoute,
				request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/bad/runtime/status", nil, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": "bad", "*": "status"}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid plugin ID",
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				tt.handler(rec, tt.request)
				require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), tt.wantBody)
			})
		}

		repo.getTenantPluginsErr = errors.New("tenant plugin store down")
		enableReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", map[string]json.RawMessage{
			"settings": json.RawMessage(`{"account":"1000"}`),
		}, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
		enableResp := httptest.NewRecorder()
		h.EnableTenantPlugin(enableResp, enableReq)
		require.Equal(t, http.StatusInternalServerError, enableResp.Code, enableResp.Body.String())
		assert.Contains(t, enableResp.Body.String(), "Failed to load tenant plugin")

		h, repo, tenantID, pluginID, userID = wave9PluginHandlers(t)
		claims = createTestClaims(userID, "user@example.com", tenantID.String(), tenant.RoleAdmin)
		repo.returnEmptyTenantPlugins = true
		enableReq = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", map[string]json.RawMessage{
			"settings": json.RawMessage(`{"account":"1000"}`),
		}, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
		enableResp = httptest.NewRecorder()
		h.EnableTenantPlugin(enableResp, enableReq)
		require.Equal(t, http.StatusInternalServerError, enableResp.Code, enableResp.Body.String())
		assert.Contains(t, enableResp.Body.String(), "Failed to load tenant plugin")
	})
}

func TestWave9MigrationExecutorDispatchReferenceErrors(t *testing.T) {
	contactsRepo := newMockContactsRepository()
	contactsRepo.listErr = errors.New("contacts unavailable")
	h := &Handlers{contactsService: contacts.NewServiceWithRepository(contactsRepo)}
	executor := &handlerMigrationStepExecutor{h: h}

	for _, kind := range []cutover.FileKind{
		cutover.KindInvoices,
		cutover.KindQuotes,
		cutover.KindRecurringInvoices,
		cutover.KindOrders,
	} {
		t.Run(string(kind)+" contacts", func(t *testing.T) {
			result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
				cutover.MigrationExecutionStep{Kind: kind},
				cutover.BundleFile{Kind: kind, FileName: string(kind) + ".csv", CSVContent: "header\n"},
				&cutover.ExecuteMigrationRequest{},
			)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "load contacts")
		})
	}

	contactsRepo.listErr = nil
	inventoryRepo := newMockInventoryRepository()
	quotesRepo := newMockQuotesRepository()
	quotesRepo.listErr = errors.New("quotes unavailable")
	h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
	h.quotesService = quotes.NewServiceWithRepository(quotesRepo)

	result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
		cutover.MigrationExecutionStep{Kind: cutover.KindOrders},
		cutover.BundleFile{Kind: cutover.KindOrders, FileName: "orders.csv", CSVContent: "header\n"},
		&cutover.ExecuteMigrationRequest{},
	)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "load quotes")
}

func TestWave9YearEndHelperRemainingBoundaries(t *testing.T) {
	h, _, _, tenantRecord := setupWave6YearEndReady(t)

	err := (&Handlers{}).requireApprovedYearEndClosePackEvidence(context.Background(), tenantRecord, "2025-12-31")
	require.ErrorIs(t, err, errApprovedClosePackEvidenceRequired)
	assert.Contains(t, err.Error(), "entity_id")

	assert.Equal(t, "file", safeArchiveFileName(" ._- "))

	attachYearEndInventoryFixture(h, decimal.Zero)
	err = h.requireYearEndInventoryCostingReady(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, tenantRecord.Settings.FiscalYearStart, "2025-12-31", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inventory costing review")

	status := &accounting.YearEndCloseStatus{IsFiscalYearEnd: true, CarryForwardReady: true}
	require.NoError(t, h.attachYearEndInventoryCostingReview(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, "", status))
	require.NotNil(t, status.InventoryCostingReview)
	assert.False(t, status.CarryForwardReady)
}

func TestWave9TaxEvidenceAndValidationBranches(t *testing.T) {
	h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")

	body := invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=2026&quarter=bad",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid quarter", body["error"])

	taxRepo.getDecl = &tax.KMDDeclaration{ID: "decl-wave9", TenantID: tenantRecord.ID, Year: 2026, Month: 2}
	docRepo := &wave6DocumentRepository{mockDocumentRepository: newMockDocumentRepository()}
	docRepo.listDocumentsErr = errors.New("evidence store failed")
	h.documentsService = documents.NewService(docRepo, nil)

	body = invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleMarkKMDSubmitted, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/2026/2/submit",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "Failed to verify KMD submission evidence", body["error"])
}

func TestWave9PeriodCloseAndReminderNotFoundBranches(t *testing.T) {
	h, tenantRepo := setupTenantTestHandlers()
	tenantID := "tenant-1"
	tenantRepo.addTestTenant(tenantID, "Tenant", "tenant")
	tenantRepo.tenantUsers[tenantID] = []tenant.TenantUser{{
		TenantID: tenantID,
		UserID:   "viewer-1",
		Role:     tenant.RoleViewer,
		IsActive: true,
	}}

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", strings.NewReader(`{"period_end_date":"2025-12-31","reason":"fix"}`)), map[string]string{"tenantID": tenantID})
	rec := httptest.NewRecorder()
	h.ReopenPeriod(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

	req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", map[string]string{
		"period_end_date": "2025-12-31",
		"reason":          "fix",
	}, &auth.Claims{UserID: "viewer-1", TenantID: tenantID, Role: tenant.RoleViewer}), map[string]string{"tenantID": tenantID})
	rec = httptest.NewRecorder()
	h.ReopenPeriod(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Insufficient permissions")

	reminderHandlers := &Handlers{automatedReminderService: invoicing.NewAutomatedReminderServiceWithRepository(newMockReminderRuleRepository(), nil)}
	for _, tt := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		request *http.Request
	}{
		{
			name:    "get",
			handler: reminderHandlers.GetReminderRule,
			request: withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reminder-rules/missing", nil), map[string]string{"tenantID": "tenant-1", "ruleID": "missing"}),
		},
		{
			name:    "update",
			handler: reminderHandlers.UpdateReminderRule,
			request: withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/reminder-rules/missing", strings.NewReader(`{"name":"Updated"}`)), map[string]string{"tenantID": "tenant-1", "ruleID": "missing"}),
		},
		{
			name:    "delete",
			handler: reminderHandlers.DeleteReminderRule,
			request: withURLParams(httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/reminder-rules/missing", nil), map[string]string{"tenantID": "tenant-1", "ruleID": "missing"}),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler(rec, tt.request)
			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "Rule not found")
		})
	}
}

func TestWave9DocumentDownloadAndWebhookServiceErrors(t *testing.T) {
	reviewHandlers, reviewRepo := setupDocumentHandlers(t)
	reviewRepo.docs["doc-review"] = &documents.Document{
		ID:           "doc-review",
		TenantID:     "tenant-1",
		EntityType:   documents.EntityTypeBankTxn,
		EntityID:     "txn-1",
		DocumentType: documents.DocumentTypeReconciliation,
		ReviewStatus: documents.ReviewStatusPending,
	}
	reviewReq := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/documents/review-queue?status=PENDING", nil), map[string]string{"tenantID": "tenant-1"})
	reviewResp := httptest.NewRecorder()
	reviewHandlers.GetDocumentReviewQueue(reviewResp, reviewReq)
	require.Equal(t, http.StatusOK, reviewResp.Code, reviewResp.Body.String())

	uploadHandlers, uploadRepo := setupDocumentHandlers(t)
	uploadRepo.entityExists = false
	var uploadBody bytes.Buffer
	uploadWriter := multipart.NewWriter(&uploadBody)
	require.NoError(t, uploadWriter.WriteField("entity_type", documents.EntityTypeBankTxn))
	require.NoError(t, uploadWriter.WriteField("entity_id", "missing-txn"))
	require.NoError(t, uploadWriter.WriteField("document_type", documents.DocumentTypeReconciliation))
	uploadPart, err := uploadWriter.CreateFormFile("file", "receipt.txt")
	require.NoError(t, err)
	_, err = uploadPart.Write([]byte("receipt"))
	require.NoError(t, err)
	require.NoError(t, uploadWriter.Close())
	uploadReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents", nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleAdmin)), map[string]string{"tenantID": "tenant-1"})
	uploadReq.Body = ioNopCloser{Reader: bytes.NewReader(uploadBody.Bytes())}
	uploadReq.ContentLength = int64(uploadBody.Len())
	uploadReq.Header.Set("Content-Type", uploadWriter.FormDataContentType())
	uploadResp := httptest.NewRecorder()
	uploadHandlers.UploadDocument(uploadResp, uploadReq)
	require.Equal(t, http.StatusNotFound, uploadResp.Code, uploadResp.Body.String())

	docRepo := newMockDocumentRepository()
	docRepo.docs["doc-1"] = &documents.Document{
		ID:          "doc-1",
		TenantID:    "tenant-1",
		FileName:    "receipt.txt",
		ContentType: "text/plain",
		FileSize:    10,
		StorageKey:  "receipt.txt",
	}
	h := &Handlers{documentsService: documents.NewService(docRepo, &wave6DocumentStore{
		reader: &wave6ReadCloser{reader: strings.NewReader(""), readErr: errors.New("copy failed")},
	})}
	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/documents/doc-1/download", nil), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"})
	rec := httptest.NewRecorder()

	h.DownloadDocument(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "copy failed")

	webhookHandlers := &Handlers{webhookService: webhooks.NewServiceWithRepository(newMemoryWebhookRepository(), nil)}
	createReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/webhooks", webhooks.CreateEndpointRequest{
		Name:   "Bad Hook",
		URL:    "not-a-url",
		Events: []string{"invoice.created"},
	}, nil), map[string]string{"tenantID": "tenant-1"})
	createResp := httptest.NewRecorder()
	webhookHandlers.CreateWebhookEndpoint(createResp, createReq)
	require.Equal(t, http.StatusBadRequest, createResp.Code, createResp.Body.String())

	testReq := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks/missing/test", nil), map[string]string{"tenantID": "tenant-1", "webhookID": "missing"})
	testResp := httptest.NewRecorder()
	webhookHandlers.TestWebhookEndpoint(testResp, testReq)
	require.Equal(t, http.StatusBadRequest, testResp.Code, testResp.Body.String())
	assert.Contains(t, testResp.Body.String(), "webhook endpoint not found")
}

func TestWave9ExpenseAndPayrollImportServiceErrors(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleAdmin)
	expenseAccountID := "11111111-1111-4111-8111-111111111111"
	paymentAccountID := "22222222-2222-4222-8222-222222222222"

	expenseRepo := newErroringExpenseRepository()
	expenseHandlers, _, evidence := setupExpenseHandlers()
	expenseHandlers.expensesService = expenses.NewServiceWithRepository(expenseRepo, &expenseHandlerAccounting{
		accounts: map[string]*accounting.Account{
			expenseAccountID: {ID: expenseAccountID, Code: "5500", AccountType: accounting.AccountTypeExpense},
			paymentAccountID: {ID: paymentAccountID, Code: "1000", AccountType: accounting.AccountTypeAsset},
		},
	}, evidence)
	importReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/import", expenses.ImportExpensesRequest{
		CSVContent: "expense_number\n\"unterminated",
	}, claims), map[string]string{"tenantID": "tenant-1"})
	importResp := httptest.NewRecorder()
	expenseHandlers.ImportExpenses(importResp, importReq)
	require.Equal(t, http.StatusBadRequest, importResp.Code, importResp.Body.String())
	assert.Contains(t, importResp.Body.String(), "parse")

	submitted := &expenses.Expense{
		ID:               "expense-submitted",
		TenantID:         "tenant-1",
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office",
		ExpenseAccountID: expenseAccountID,
		PaymentAccountID: paymentAccountID,
		Amount:           decimal.NewFromInt(10),
		Status:           expenses.StatusSubmitted,
	}
	require.NoError(t, expenseRepo.expenseHandlerRepository.Create(context.Background(), "tenant_test", submitted))
	expenseRepo.updateErr = errors.New("reject failed")
	rejectReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/expense-submitted/reject", expenses.RejectExpenseRequest{Reason: "missing project"}, claims), map[string]string{"tenantID": "tenant-1", "expenseID": "expense-submitted"})
	rejectResp := httptest.NewRecorder()
	expenseHandlers.RejectExpense(rejectResp, rejectReq)
	require.Equal(t, http.StatusBadRequest, rejectResp.Code, rejectResp.Body.String())
	assert.Contains(t, rejectResp.Body.String(), "reject failed")

	lockedHandlers, lockedRepo, _ := setupExpenseHandlers()
	tenantRepo := newMockTenantRepository()
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant", "tenant")
	tenantRecord.Settings.PeriodLockDate = stringPtr("2026-06-01")
	lockedHandlers.tenantService = tenant.NewServiceWithRepository(tenantRepo)
	require.NoError(t, lockedRepo.Create(context.Background(), "tenant_test", &expenses.Expense{
		ID:               "expense-approved",
		TenantID:         "tenant-1",
		ExpenseDate:      time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office",
		ExpenseAccountID: "expense-account",
		PaymentAccountID: "cash-account",
		Amount:           decimal.NewFromInt(10),
		Status:           expenses.StatusApproved,
	}))
	postReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/expenses/expense-approved/post", nil, claims), map[string]string{"tenantID": "tenant-1", "expenseID": "expense-approved"})
	postResp := httptest.NewRecorder()
	lockedHandlers.PostExpense(postResp, postReq)
	require.Equal(t, http.StatusConflict, postResp.Code, postResp.Body.String())
	assert.Contains(t, postResp.Body.String(), "locked")

	payrollHandlers, _, _ := setupPayrollImportHandlerTest(t)
	for _, tt := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		body    any
	}{
		{
			name:    "employees",
			handler: payrollHandlers.ImportEmployees,
			body:    payroll.ImportEmployeesRequest{CSVContent: "not,a,valid\n"},
		},
		{
			name:    "payroll history",
			handler: payrollHandlers.ImportPayrollHistory,
			body:    payroll.ImportPayrollHistoryRequest{CSVContent: "not,a,valid\n"},
		},
		{
			name:    "tsd history",
			handler: payrollHandlers.ImportTSDHistory,
			body:    payroll.ImportTSDHistoryRequest{CSVContent: "not,a,valid\n"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/import", tt.body, claims), map[string]string{"tenantID": "tenant-1"})
			rec := httptest.NewRecorder()
			tt.handler(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

type ioNopCloser struct {
	*bytes.Reader
}

func (c ioNopCloser) Close() error {
	return nil
}

func wave9PluginHandlers(t *testing.T) (*Handlers, *wave9PluginRepository, uuid.UUID, uuid.UUID, string) {
	t.Helper()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	repo := &wave9PluginRepository{pluginHandlerRepository: newPluginHandlerRepository()}
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "wave9-plugin",
		DisplayName:        "Wave9 Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/wave9-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(`{
			"name":"wave9-plugin",
			"display_name":"Wave9 Plugin",
			"version":"1.0.0",
			"permissions":["routes:register"]
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Wave9 Tenant", "wave9-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleAdmin,
		IsActive: true,
	}}

	return &Handlers{
		pluginService: plugin.NewServiceWithRepository(repo, nil, t.TempDir()),
		tenantService: tenant.NewServiceWithRepository(tenantRepo),
	}, repo, tenantID, pluginID, userID
}
