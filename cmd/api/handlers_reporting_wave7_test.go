package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestWave7PluginHandlersAdditionalBranches(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Tenant", "tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleAdmin,
		IsActive: true,
	}}
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)

	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "wave7-plugin",
		DisplayName:        "Wave7 Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/wave7-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(`{
			"name":"wave7-plugin",
			"display_name":"Wave7 Plugin",
			"version":"1.0.0",
			"permissions":["routes:register"],
			"backend":{
				"runtime":"http",
				"base_url":"http://127.0.0.1:1",
				"routes":[{"method":"GET","path":"/status","handler":"/routes/status"}]
			}
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "sync registry service error",
			handler:    h.SyncPluginRegistry,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugin-registries/"+uuid.NewString()+"/sync", nil), map[string]string{"id": uuid.NewString()}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "registry not found",
		},
		{
			name:       "search maps repository error",
			handler:    h.SearchPlugins,
			request:    httptest.NewRequest(http.MethodGet, "/admin/plugins/search?q=bank", nil),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "registry unavailable",
		},
		{
			name:       "enable rejects invalid JSON",
			handler:    h.EnablePlugin,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/enable", strings.NewReader("{")), map[string]string{"id": pluginID.String()}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "disable rejects invalid ID",
			handler:    h.DisablePlugin,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/bad/disable", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid plugin ID",
		},
		{
			name:       "runtime status maps missing plugin",
			handler:    h.GetPluginRuntimeStatus,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/admin/plugins/"+uuid.NewString()+"/runtime", nil), map[string]string{"id": uuid.NewString()}),
			wantStatus: http.StatusNotFound,
			wantBody:   "plugin not found",
		},
		{
			name:       "restart maps missing plugin",
			handler:    h.RestartPluginRuntime,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+uuid.NewString()+"/runtime/restart", nil), map[string]string{"id": uuid.NewString()}),
			wantStatus: http.StatusNotFound,
			wantBody:   "plugin not found",
		},
		{
			name:    "update tenant settings rejects invalid JSON",
			handler: h.UpdateTenantPluginSettings,
			request: withURLParams(
				makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", nil, &auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleAdmin}),
				map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()},
			),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:    "invoke rejects inaccessible tenant",
			handler: h.InvokeTenantPluginRoute,
			request: withURLParams(
				makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil, &auth.Claims{UserID: "missing-user", TenantID: tenantID.String(), Role: tenant.RoleViewer}),
				map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"},
			),
			wantStatus: http.StatusForbidden,
			wantBody:   "Access denied",
		},
	}

	repo.listRegistriesErr = errors.New("registry unavailable")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.name, "search") {
				repo.listRegistriesErr = nil
			} else {
				repo.listRegistriesErr = errors.New("registry unavailable")
			}
			w := httptest.NewRecorder()
			tt.handler(w, tt.request)
			require.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}

	enableReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", nil, &auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleAdmin})
	enableReq = withURLParams(enableReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	enableResp := httptest.NewRecorder()
	h.EnableTenantPlugin(enableResp, enableReq)
	require.Equal(t, http.StatusOK, enableResp.Code, enableResp.Body.String())
	require.JSONEq(t, `{}`, string(repo.tenantPlugins[pluginTenantKey(tenantID, pluginID)].Settings))

	repo.plugins[pluginID].Manifest = json.RawMessage(`{`)
	invokeReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil, &auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleAdmin})
	invokeReq = withURLParams(invokeReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"})
	invokeResp := httptest.NewRecorder()
	h.InvokeTenantPluginRoute(invokeResp, invokeReq)
	require.Equal(t, http.StatusBadGateway, invokeResp.Code, invokeResp.Body.String())
	assert.Contains(t, invokeResp.Body.String(), "Plugin runtime request failed")
}

func TestWave7MigrationValidationAndHelpers(t *testing.T) {
	h := &Handlers{}

	for _, tt := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		target  string
	}{
		{name: "validate", handler: h.ValidateMigrationBundle, target: "/tenants/tenant-1/migration/validate"},
		{name: "plan", handler: h.PlanMigrationExecution, target: "/tenants/tenant-1/migration/execution-plan"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := withURLParams(httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
			w := httptest.NewRecorder()
			tt.handler(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "Invalid request body")
		})
	}

	assert.Equal(t, time.Second, mustMigrationInterval(t, "/events"))
	assert.Equal(t, 10*time.Millisecond, mustMigrationInterval(t, "/events?interval_ms=10"))
	_, err := migrationExecutionRunStreamInterval(httptest.NewRequest(http.MethodGet, "/events?interval_ms=0", nil))
	require.Error(t, err)

	assert.Equal(t, 100, mustMigrationMaxEvents(t, "/events"))
	assert.Equal(t, 7, mustMigrationMaxEvents(t, "/events?max_events=7"))
	_, err = migrationExecutionRunStreamMaxEvents(httptest.NewRequest(http.MethodGet, "/events?max_events=1001", nil))
	require.Error(t, err)

	assert.True(t, migrationExecutionRunStreamTerminal(nil))
	assert.False(t, migrationExecutionRunStreamTerminal(&cutover.MigrationExecutionRun{}))
	assert.True(t, migrationExecutionRunStreamTerminal(&cutover.MigrationExecutionRun{Summary: cutover.MigrationExecutionRunSummary{Status: "BLOCKED"}}))

	req := &cutover.ExecuteMigrationRequest{ProviderPreset: cutover.MigrationProviderPresetGeneric}
	assert.Nil(t, savedMigrationExecutionRequest(nil))
	assert.Same(t, req, savedMigrationExecutionRequest(&cutover.MigrationExecutionRun{ExecutionRequest: req}))

	files := migrationFilesByExecutionStepKey([]cutover.BundleFile{{Kind: cutover.KindAccounts, FileName: " accounts.csv "}})
	_, ok := files[migrationExecutionStepFileKey(cutover.KindAccounts, "accounts.csv")]
	assert.True(t, ok)
	assert.Equal(t, "OB-2026", migrationOpeningBalanceReference("2026-01-01"))
	assert.Equal(t, "OB", migrationOpeningBalanceReference("26"))

	invoiceType, err := parseMigrationExecuteInvoiceType(" credit_note ")
	require.NoError(t, err)
	assert.Equal(t, invoicing.InvoiceTypeCreditNote, invoiceType)
	_, err = parseMigrationExecuteInvoiceType("bad")
	require.Error(t, err)
}

func TestWave7YearEndAndPeriodCloseErrorMappings(t *testing.T) {
	for _, tt := range []struct {
		err  error
		code int
	}{
		{err: errApprovedClosePackEvidenceRequired, code: http.StatusConflict},
		{err: errors.New("period end date is required"), code: http.StatusBadRequest},
		{err: errors.New("period must match the fiscal year end"), code: http.StatusBadRequest},
		{err: errors.New("user_id is required"), code: http.StatusBadRequest},
		{err: errors.New("reason is required"), code: http.StatusBadRequest},
		{err: errors.New("fiscal year must be closed first"), code: http.StatusConflict},
		{err: errors.New("carry-forward already exists"), code: http.StatusConflict},
		{err: errors.New("carry-forward does not exist"), code: http.StatusConflict},
		{err: errors.New("invalid current status"), code: http.StatusConflict},
		{err: errors.New("entry not in posted status"), code: http.StatusConflict},
		{err: errors.New("retained earnings account is required"), code: http.StatusConflict},
		{err: errors.New("no revenue or expense activity found"), code: http.StatusConflict},
	} {
		w := httptest.NewRecorder()
		respondYearEndCloseError(w, tt.err)
		assert.Equal(t, tt.code, w.Code, tt.err.Error())
	}

	for _, tt := range []struct {
		err  error
		code int
	}{
		{err: errApprovedClosePackEvidenceRequired, code: http.StatusConflict},
		{err: errors.New("tenant not found"), code: http.StatusNotFound},
		{err: errors.New("period end date is required"), code: http.StatusBadRequest},
		{err: errors.New("note is required"), code: http.StatusBadRequest},
		{err: errors.New("reviewer sign-off is required"), code: http.StatusBadRequest},
		{err: errors.New("invalid valuation method"), code: http.StatusBadRequest},
		{err: errors.New("period already closed through 2026-01-31"), code: http.StatusConflict},
		{err: errors.New("has not been closed yet"), code: http.StatusConflict},
		{err: errors.New("carry-forward has been posted"), code: http.StatusConflict},
	} {
		w := httptest.NewRecorder()
		respondPeriodCloseError(w, tt.err)
		assert.Equal(t, tt.code, w.Code, tt.err.Error())
	}
}

func TestWave7TaxPayrollRecurringReminderValidationBranches(t *testing.T) {
	h := &Handlers{}
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	validationCases := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "tax generate invalid json",
			handler:    h.HandleGenerateKMD,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/tax/kmd", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "tax import requires csv",
			handler:    h.HandleImportKMDHistory,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/tax/kmd/import-history", map[string]string{}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "csv_content is required",
		},
		{
			name:       "tax inf invalid threshold",
			handler:    h.HandleGenerateKMDINF,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/tax/kmd/2026/2/inf?threshold=0", nil), map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "threshold must be positive",
		},
		{
			name:       "tax oss invalid include b2b",
			handler:    h.HandleGenerateEUVATOSS,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/tax/eu-vat/oss?year=2026&quarter=1&include_b2b=maybe", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid include_b2b",
		},
		{
			name:       "payroll employees invalid json",
			handler:    h.ImportEmployees,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/employees/import", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "payroll history requires csv",
			handler:    h.ImportPayrollHistory,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payroll-runs/import-history", map[string]string{}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "csv_content is required",
		},
		{
			name:       "tsd history requires csv",
			handler:    h.ImportTSDHistory,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/tsd/import-history", map[string]string{}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "csv_content is required",
		},
		{
			name:       "leave balances requires csv",
			handler:    h.ImportLeaveBalances,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/leave-balances/import", map[string]string{}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "csv_content is required",
		},
		{
			name:       "recurring create invalid json",
			handler:    h.CreateRecurringInvoice,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "recurring import requires csv",
			handler:    h.ImportRecurringInvoices,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/import", map[string]string{}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "csv_content is required",
		},
		{
			name:       "reminder create invalid json",
			handler:    h.CreateReminderRule,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/reminder-rules", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "reminder update invalid json",
			handler:    h.UpdateReminderRule,
			request:    withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/reminder-rules/rule-1", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
	}

	for _, tt := range validationCases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.handler(w, tt.request)
			require.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}

	tenantHandlers, _ := setupTenantTestHandlers()
	w := httptest.NewRecorder()
	tenantHandlers.TriggerReminders(w, withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/missing/reminder-rules/trigger", nil), map[string]string{"tenantID": "missing"}))
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Tenant not found")
}

func TestWave7DocumentWebhookAndExpenseValidationBranches(t *testing.T) {
	h := &Handlers{}
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	cases := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "documents list requires entity",
			handler:    h.ListDocuments,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/documents", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "entity_type and entity_id are required",
		},
		{
			name:       "documents review summary invalid json",
			handler:    h.ListDocumentReviewSummaries,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents/review-summary", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "documents queue bad limit",
			handler:    h.GetDocumentReviewQueue,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/documents/review-queue?limit=-1", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be zero or greater",
		},
		{
			name:       "documents retention bad date",
			handler:    h.GetDocumentRetentionReview,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/documents/retention?as_of=bad", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid as_of date",
		},
		{
			name:       "documents purge invalid date",
			handler:    h.PurgeExpiredDocuments,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/purge", map[string]string{"as_of": "bad"}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid as_of date",
		},
		{
			name:       "documents update retention conflicts",
			handler:    h.UpdateDocumentRetention,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/retention", map[string]any{"retention_until": "2026-01-01", "clear_retention": true}, claims), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "retention_until cannot be set",
		},
		{
			name:       "documents lifecycle invalid json",
			handler:    h.UpdateDocumentLifecycle,
			request:    withURLParams(withClaims(httptest.NewRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/lifecycle", strings.NewReader("{")), claims), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "webhook create invalid json",
			handler:    h.CreateWebhookEndpoint,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "webhook deliveries bad limit",
			handler:    h.ListWebhookDeliveries,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/webhooks/hook-1/deliveries?limit=201", nil), map[string]string{"tenantID": "tenant-1", "webhookID": "hook-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Limit must be between 1 and 200",
		},
		{
			name:       "webhook test invalid json",
			handler:    h.TestWebhookEndpoint,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks/hook-1/test", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "webhookID": "hook-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "expenses list bad limit",
			handler:    h.ListExpenses,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/expenses?limit=bad", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be zero or greater",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.handler(w, tt.request)
			require.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}

	for _, tt := range []struct {
		err  error
		code int
	}{
		{err: errors.New("document not found"), code: http.StatusNotFound},
		{err: errors.New("unsupported document type"), code: http.StatusBadRequest},
		{err: errors.New("storage offline"), code: http.StatusInternalServerError},
	} {
		w := httptest.NewRecorder()
		respondDocumentError(w, tt.err)
		assert.Equal(t, tt.code, w.Code)
	}

	for _, tt := range []struct {
		err  error
		code int
	}{
		{err: expenses.ErrExpenseNotFound, code: http.StatusNotFound},
		{err: expenses.ErrApprovedReceiptRequired, code: http.StatusConflict},
		{err: expenses.ErrInvalidStatusTransition, code: http.StatusBadRequest},
	} {
		w := httptest.NewRecorder()
		respondExpenseError(w, tt.err)
		assert.Equal(t, tt.code, w.Code)
	}
}

func TestWave7WebhookEventTypesAndKMDEvidenceGuards(t *testing.T) {
	h := &Handlers{}
	w := httptest.NewRecorder()
	h.ListWebhookEventTypes(w, httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/webhooks/events", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), plugin.EventExpenseCreated)

	err := h.requireApprovedKMDSubmissionEvidence(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "tenant_1", "tenant-1", "decl-1")
	require.ErrorIs(t, err, errApprovedKMDSubmissionEvidenceRequired)
	err = h.requireApprovedKMDAcceptanceEvidence(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "tenant_1", "tenant-1", "decl-1")
	require.ErrorIs(t, err, errApprovedKMDAcceptanceEvidenceRequired)

	h.documentsService = documents.NewService(newMockDocumentRepository(), nil)
	err = h.requireApprovedKMDEvidence(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "tenant_1", "tenant-1", "decl-1", "submission", "submitted", errApprovedKMDSubmissionEvidenceRequired)
	require.ErrorIs(t, err, errApprovedKMDSubmissionEvidenceRequired)
}

func withClaims(req *http.Request, claims *auth.Claims) *http.Request {
	return req.WithContext(contextWithClaims(req.Context(), claims))
}

func mustMigrationInterval(t *testing.T, target string) time.Duration {
	t.Helper()
	value, err := migrationExecutionRunStreamInterval(httptest.NewRequest(http.MethodGet, target, nil))
	require.NoError(t, err)
	return value
}

func mustMigrationMaxEvents(t *testing.T, target string) int {
	t.Helper()
	value, err := migrationExecutionRunStreamMaxEvents(httptest.NewRequest(http.MethodGet, target, nil))
	require.NoError(t, err)
	return value
}
