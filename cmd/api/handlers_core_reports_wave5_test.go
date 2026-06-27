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

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestWave5AuthHelperBranches(t *testing.T) {
	assert.Equal(t, 1, loginRetryAfterSeconds(0))
	assert.Equal(t, 1, loginRetryAfterSeconds(500*time.Millisecond))
	assert.Equal(t, 3, loginRetryAfterSeconds(3*time.Second))

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("User-Agent", "wave5-agent")

	h := &Handlers{}
	h.recordSecurityAuditEvent(req, nil)
	h.recordSecurityAuditEvent(req, &auth.SecurityAuditEvent{Action: auth.SecurityAuditActionLogin})

	auditSvc := &mockSecurityAuditService{}
	h.securityAuditService = auditSvc
	event := &auth.SecurityAuditEvent{Action: auth.SecurityAuditActionPasswordChanged}
	h.recordSecurityAuditEvent(req, event)
	require.Len(t, auditSvc.events, 1)
	assert.Equal(t, "192.0.2.10:1234", auditSvc.events[0].RequestIP)
	assert.Equal(t, "wave5-agent", auditSvc.events[0].UserAgent)

	h.securityAuditService = &mockSecurityAuditService{err: errors.New("audit store down")}
	h.recordSecurityAuditEvent(req, &auth.SecurityAuditEvent{
		Action:    auth.SecurityAuditActionLogout,
		RequestIP: "198.51.100.5",
		UserAgent: "already-set",
	})
}

func TestWave5UpdateTenantBranches(t *testing.T) {
	ownerClaims := createTestClaims("user-1", "owner@example.com", "tenant-1", tenant.RoleOwner)

	t.Run("rejects invalid json", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestTenant("tenant-1", "Old Name", "tenant-one")
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsActive: true}}

		req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1", nil, ownerClaims)
		req = httptest.NewRequest(http.MethodPut, "/tenants/tenant-1", strings.NewReader("{")).WithContext(req.Context())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.UpdateTenant(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("maps service error branches", func(t *testing.T) {
		tests := []struct {
			name       string
			setup      func(*mockTenantRepository)
			body       string
			wantStatus int
			wantBody   string
		}{
			{
				name: "tenant not found",
				setup: func(repo *mockTenantRepository) {
					repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsActive: true}}
				},
				body:       `{"name":"New Name"}`,
				wantStatus: http.StatusNotFound,
				wantBody:   "Tenant not found",
			},
			{
				name: "period lock setting cannot be changed directly",
				setup: func(repo *mockTenantRepository) {
					repo.addTestTenant("tenant-1", "Old Name", "tenant-one")
					repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsActive: true}}
				},
				body:       `{"settings":{"period_lock_date":"2026-01-31"}}`,
				wantStatus: http.StatusBadRequest,
				wantBody:   "period lock must be managed",
			},
			{
				name: "invalid inventory setting",
				setup: func(repo *mockTenantRepository) {
					repo.addTestTenant("tenant-1", "Old Name", "tenant-one")
					repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsActive: true}}
				},
				body:       `{"settings":{"inventory_valuation_method":"NOT_A_METHOD"}}`,
				wantStatus: http.StatusBadRequest,
				wantBody:   "invalid inventory",
			},
			{
				name: "repository update error",
				setup: func(repo *mockTenantRepository) {
					repo.addTestTenant("tenant-1", "Old Name", "tenant-one")
					repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsActive: true}}
					repo.updateTenantErr = errors.New("update failed")
				},
				body:       `{"name":"New Name"}`,
				wantStatus: http.StatusInternalServerError,
				wantBody:   "update failed",
			},
			{
				name: "audit write failure",
				setup: func(repo *mockTenantRepository) {
					repo.addTestTenant("tenant-1", "Old Name", "tenant-one")
					repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsActive: true}}
					repo.createAuditEventErr = errors.New("audit failed")
				},
				body:       `{"name":"New Name"}`,
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to record tenant audit event",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, repo := setupAuthTestHandlers()
				tt.setup(repo)
				req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1", json.RawMessage(tt.body), ownerClaims)
				req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
				rr := httptest.NewRecorder()

				h.UpdateTenant(rr, req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})
}

func TestWave5AccountAndJournalHandlerBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	t.Run("account import and create reject malformed json", func(t *testing.T) {
		h, _, _ := setupAccountingTestHandlers()

		req := badJSONRequest(http.MethodPost, "/tenants/tenant-1/accounts", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.CreateAccount(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/accounts/import", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportAccounts(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("import accounts maps service error", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		accountingRepo.listAccountsErr = errors.New("list failed")

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/accounts/import", map[string]string{
			"csv_content": "code,name,account_type\n1000,Cash,ASSET\n",
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ImportAccounts(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "list existing accounts")
	})

	t.Run("import journal entries rejects missing auth malformed json and bad lock date", func(t *testing.T) {
		h, tenantRepo, _ := setupAccountingTestHandlers()
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import", strings.NewReader(`{"csv_content":"x"}`)), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.ImportJournalEntries(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code)

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportJournalEntries(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		badLock := "not-a-date"
		tenantRecord.Settings.PeriodLockDate = &badLock
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import", map[string]string{
			"csv_content": "entry_reference,entry_date,account_code,debit,credit\nBAD,2026-01-01,1000,1,0\n",
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportJournalEntries(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "tenant period_lock_date")
	})

	t.Run("delete account maps repository miss", func(t *testing.T) {
		h, _, _ := setupAccountingTestHandlers()
		req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/accounts/missing", nil, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "missing"})
		rr := httptest.NewRecorder()

		h.DeleteAccount(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Account not found")
	})

	t.Run("journal handlers map malformed json and service errors", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		entryDate := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
		accountingRepo.journalEntries["je-1"] = &accounting.JournalEntry{
			ID:          "je-1",
			TenantID:    "tenant-1",
			EntryDate:   entryDate,
			Description: "Draft",
			Status:      accounting.StatusDraft,
			CreatedBy:   "user-1",
			Lines: []accounting.JournalEntryLine{
				{AccountID: "cash", DebitAmount: decimal.NewFromInt(10), BaseDebit: decimal.NewFromInt(10), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1)},
				{AccountID: "sales", CreditAmount: decimal.NewFromInt(10), BaseCredit: decimal.NewFromInt(10), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1)},
			},
		}

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/journal-entries/missing", nil), map[string]string{"tenantID": "tenant-1", "entryID": "missing"})
		rr := httptest.NewRecorder()
		h.GetJournalEntry(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entries", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.CreateJournalEntry(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		accountingRepo.getJournalErr = errors.New("lookup failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-1/post", nil, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-1"})
		rr = httptest.NewRecorder()
		h.PostJournalEntry(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "lookup failed")
		accountingRepo.getJournalErr = nil

		accountingRepo.updateJournalErr = errors.New("post failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-1/post", nil, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-1"})
		rr = httptest.NewRecorder()
		h.PostJournalEntry(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "post failed")
		accountingRepo.updateJournalErr = nil

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-1/void", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-1"})
		rr = httptest.NewRecorder()
		h.VoidJournalEntry(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		accountingRepo.getJournalErr = errors.New("void lookup failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-1/void", map[string]string{"reason": "reverse"}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-1"})
		rr = httptest.NewRecorder()
		h.VoidJournalEntry(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "void lookup failed")
		accountingRepo.getJournalErr = nil

		accountingRepo.journalEntries["je-1"].Status = accounting.StatusPosted
		accountingRepo.voidJournalErr = errors.New("void failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-1/void", map[string]string{"reason": "reverse"}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-1"})
		rr = httptest.NewRecorder()
		h.VoidJournalEntry(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "void failed")
	})
}

func TestWave5JournalTemplateHandlerBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	t.Run("list and get templates map service errors", func(t *testing.T) {
		base, _, repo := setupAccountingTestHandlers()
		h := *base
		h.accountingService = accounting.NewServiceWithRepository(&templateErrorAccountingRepository{mockAccountingRepository: repo})

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/journal-entry-templates", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.ListJournalEntryTemplates(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "template list failed")

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/journal-entry-templates/tpl-missing", nil), map[string]string{"tenantID": "tenant-1", "templateID": "tpl-missing"})
		rr = httptest.NewRecorder()
		h.GetJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("create and generate reject malformed json and service errors", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		templateID := "22222222-2222-4222-8222-222222222222"
		accountingRepo.templates[templateID] = balancedTemplate(templateID, false, true)

		req := badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.CreateJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		accountingRepo.createJournalErr = errors.New("template create failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates", minimalTemplateBody(), claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.CreateJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "template create failed")
		accountingRepo.createJournalErr = nil

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/generate-due", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GenerateDueJournalEntryTemplates(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		tenantRepo.getTenantErr = errors.New("tenant read failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/generate-due", map[string]string{"as_of_date": "2026-03-31T00:00:00Z"}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GenerateDueJournalEntryTemplates(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		tenantRepo.getTenantErr = nil

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/"+templateID+"/generate", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": templateID})
		rr = httptest.NewRecorder()
		h.GenerateJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/missing/generate", map[string]string{}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": "missing"})
		rr = httptest.NewRecorder()
		h.GenerateJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestWave5CoreAccountingReportBranches(t *testing.T) {
	t.Run("service errors", func(t *testing.T) {
		tests := []struct {
			name       string
			setup      func(*mockAccountingRepository)
			target     string
			params     map[string]string
			call       func(*Handlers, http.ResponseWriter, *http.Request)
			wantStatus int
			wantBody   string
		}{
			{
				name: "trial balance service error",
				setup: func(repo *mockAccountingRepository) {
					repo.getTrialBalanceErr = errors.New("trial balance down")
				},
				target:     "/tenants/tenant-1/reports/trial-balance?as_of_date=2026-02-28",
				params:     map[string]string{"tenantID": "tenant-1"},
				call:       (*Handlers).GetTrialBalance,
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to generate trial balance",
			},
			{
				name: "account balance service error",
				setup: func(repo *mockAccountingRepository) {
					repo.getBalanceErr = errors.New("balance down")
				},
				target:     "/tenants/tenant-1/reports/account-balance/cash?as_of_date=2026-02-28",
				params:     map[string]string{"tenantID": "tenant-1", "accountID": "cash"},
				call:       (*Handlers).GetAccountBalance,
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to get account balance",
			},
			{
				name: "balance sheet service error",
				setup: func(repo *mockAccountingRepository) {
					repo.getTrialBalanceErr = errors.New("balance sheet down")
				},
				target:     "/tenants/tenant-1/reports/balance-sheet?as_of=2026-02-28",
				params:     map[string]string{"tenantID": "tenant-1"},
				call:       (*Handlers).GetBalanceSheet,
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to generate balance sheet",
			},
			{
				name: "income statement service error",
				setup: func(repo *mockAccountingRepository) {
					repo.getPeriodBalancesErr = errors.New("income down")
				},
				target:     "/tenants/tenant-1/reports/income-statement?start=2026-01-01&end=2026-01-31",
				params:     map[string]string{"tenantID": "tenant-1"},
				call:       (*Handlers).GetIncomeStatement,
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to generate income statement",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, _, repo := setupAccountingTestHandlers()
				tt.setup(repo)
				req := withURLParams(httptest.NewRequest(http.MethodGet, tt.target, nil), tt.params)
				rr := httptest.NewRecorder()

				tt.call(h, rr, req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("invalid dates and missing income dates", func(t *testing.T) {
		h, _, _ := setupAccountingTestHandlers()

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/income-statement?start=bad&end=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.GetIncomeStatement(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid start date")

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/income-statement?start=2026-01-01&end=bad", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetIncomeStatement(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid end date")
	})
}

func TestWave5ExtendedReportAndReminderBranches(t *testing.T) {
	h, tenantRepo, reportsRepo, reminderRepo, _, _ := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")

	t.Run("cash flow validates missing end invalid end conflicts and service errors", func(t *testing.T) {
		tests := []struct {
			target     string
			wantStatus int
			want       string
			setup      func()
			cleanup    func()
		}{
			{target: "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01", wantStatus: http.StatusBadRequest, want: "start_date and end_date"},
			{target: "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=bad", wantStatus: http.StatusBadRequest, want: "Invalid end_date"},
			{target: "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31&operating_accounts=1000&investing_accounts=1000", wantStatus: http.StatusBadRequest, want: "cannot be assigned to both"},
			{
				target:     "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31",
				wantStatus: http.StatusInternalServerError,
				want:       "Failed to generate cash flow statement",
				setup: func() {
					reportsRepo.GetEntriesErr = errors.New("entries down")
				},
				cleanup: func() {
					reportsRepo.GetEntriesErr = nil
				},
			},
		}

		for _, tt := range tests {
			if tt.setup != nil {
				tt.setup()
			}
			req := withURLParams(httptest.NewRequest(http.MethodGet, tt.target, nil), map[string]string{"tenantID": "tenant-1"})
			rr := httptest.NewRecorder()
			h.GetCashFlowStatement(rr, req)
			require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), tt.want)
			if tt.cleanup != nil {
				tt.cleanup()
			}
		}
	})

	t.Run("cash flow mapping repository errors", func(t *testing.T) {
		reportsRepo.GetCashFlowMappingErr = errors.New("mapping read failed")
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/cash-flow/mapping", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.GetCashFlowMapping(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reportsRepo.GetCashFlowMappingErr = nil

		req = badJSONRequest(http.MethodPut, "/tenants/tenant-1/reports/cash-flow/mapping", nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.UpdateCashFlowMapping(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		reportsRepo.UpdateCashFlowMappingErr = errors.New("mapping write failed")
		req = makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/reports/cash-flow/mapping", map[string][]string{
			"operating_account_codes": {"1000"},
		}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.UpdateCashFlowMapping(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reportsRepo.UpdateCashFlowMappingErr = nil
	})

	t.Run("balance confirmation validation and repository errors", func(t *testing.T) {
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.GetBalanceConfirmationSummary(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmationSummary(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE&as_of_date=bad", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmationSummary(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		reportsRepo.GetContactBalancesErr = errors.New("balances down")
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE&as_of_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmationSummary(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reportsRepo.GetContactBalancesErr = nil

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations/contact-1", nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmation(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=OTHER&as_of_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmation(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=PAYABLE", nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmation(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		reportsRepo.GetContactInvoicesErr = errors.New("invoices down")
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
		rr = httptest.NewRecorder()
		h.GetBalanceConfirmation(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reportsRepo.GetContactInvoicesErr = nil
	})

	t.Run("contact and margin service errors", func(t *testing.T) {
		reportsRepo.GetContactErr = errors.New("contact down")
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/contact-statements/contact-1?type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
		rr := httptest.NewRecorder()
		h.GetContactStatement(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reportsRepo.GetContactErr = nil

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/contact-statements/?type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1", "contactID": ""})
		rr = httptest.NewRecorder()
		h.GetContactStatement(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "contactID path parameter")

		reportsRepo.GetSalesMarginLinesErr = errors.New("margin down")
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/sales-margin?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetSalesMarginReport(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reportsRepo.GetSalesMarginLinesErr = nil
	})

	t.Run("reminder error branches", func(t *testing.T) {
		reminderRepo.GetOverdueErr = errors.New("overdue down")
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/invoices/overdue", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.GetOverdueInvoices(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		reminderRepo.GetOverdueErr = nil

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/invoices/reminders/bulk", nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.SendBulkPaymentReminders(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		h.reminderService = invoicing.NewReminderServiceWithRepository(&wave5ReminderHistoryErrorRepository{MockReminderRepository: reminderRepo}, nil)
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/invoices/inv-1/reminders", nil), map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-1"})
		rr = httptest.NewRecorder()
		h.GetInvoiceReminderHistory(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestWave5CostCenterReportBranches(t *testing.T) {
	h, tenantRepo, _, _, costCenterRepo, _ := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	budget := decimal.NewFromInt(1000)
	costCenterID := "11111111-1111-4111-8111-111111111111"
	costCenterRepo.CostCenters[costCenterID] = &accounting.CostCenter{
		ID:           costCenterID,
		TenantID:     "tenant-1",
		Code:         "OPS",
		Name:         "Operations",
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: accounting.BudgetPeriodAnnual,
	}

	t.Run("cost center handler validation errors", func(t *testing.T) {
		req := badJSONRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/import", nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.ImportCostCenters(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/import", map[string]string{}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportCostCenters(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = badJSONRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations/import", nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportCostAllocations(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations/import", map[string]string{}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportCostAllocations(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/allocations?end_date=bad", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ListCostAllocations(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = badJSONRequest(http.MethodPut, "/tenants/tenant-1/cost-centers/"+costCenterID, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "costCenterID": costCenterID})
		rr = httptest.NewRecorder()
		h.UpdateCostCenter(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		req = makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/cost-centers/33333333-3333-4333-8333-333333333333", map[string]string{"code": "MISS", "name": "Missing"}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "costCenterID": "33333333-3333-4333-8333-333333333333"})
		rr = httptest.NewRecorder()
		h.UpdateCostCenter(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("cost center report validation and service errors", func(t *testing.T) {
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/report?end_date=bad", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.GetCostCenterReport(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)

		h.costCenterService = accounting.NewCostCenterServiceWithRepository(&costCenterErrorRepository{
			MockCostCenterRepository: accounting.NewMockCostCenterRepository(),
			listErr:                  errors.New("cost center report down"),
		})
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/report?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetCostCenterReport(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

type templateErrorAccountingRepository struct {
	*mockAccountingRepository
}

func (m *templateErrorAccountingRepository) ListJournalEntryTemplates(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.JournalEntryTemplate, error) {
	return nil, errors.New("template list failed")
}

func (m *templateErrorAccountingRepository) GetJournalEntryTemplateByID(ctx context.Context, schemaName, tenantID, templateID string) (*accounting.JournalEntryTemplate, error) {
	return nil, errors.New("template not found")
}

type wave5ReminderHistoryErrorRepository struct {
	*invoicing.MockReminderRepository
}

func (m *wave5ReminderHistoryErrorRepository) GetRemindersByInvoice(ctx context.Context, schemaName, tenantID, invoiceID string) ([]invoicing.PaymentReminder, error) {
	return nil, errors.New("history down")
}

func badJSONRequest(method, target string, claims *auth.Claims) *http.Request {
	req := makeAuthenticatedRequest(method, target, nil, claims)
	return httptest.NewRequest(method, target, strings.NewReader("{")).WithContext(req.Context())
}

func minimalTemplateBody() map[string]interface{} {
	return map[string]interface{}{
		"name": "Template",
		"lines": []map[string]interface{}{
			{"account_id": "cash", "debit_amount": "10.00"},
			{"account_id": "sales", "credit_amount": "10.00"},
		},
	}
}
