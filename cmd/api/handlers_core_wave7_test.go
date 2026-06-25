package main

import (
	"context"
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
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestWave7CoreAuthPermissionAndRefreshBranches(t *testing.T) {
	t.Run("permission helper rejects missing tenant context", func(t *testing.T) {
		h := &Handlers{}
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/current", nil, createTestClaims("user-1", "user@example.com", "", ""))
		rr := httptest.NewRecorder()

		h.RequireTenantPermission(func(perms tenant.RolePermissions) bool { return true })(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler should not be called")
		})).ServeHTTP(rr, req)

		require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Tenant context required")
	})

	t.Run("login rejects missing refresh session service after issuing tokens", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
		h.refreshSessionService = nil

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/login", map[string]string{
			"email":    " user@example.com ",
			"password": "password123",
		}, nil)
		rr := httptest.NewRecorder()

		h.Login(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Refresh session service unavailable")
	})

	t.Run("successful login resets configured failed-attempt limiter", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
		h.loginAttemptLimiter = auth.NewLoginAttemptLimiter(5, time.Minute, time.Minute)

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/login", map[string]string{
			"email":    "user@example.com",
			"password": "password123",
		}, nil)
		req.RemoteAddr = "203.0.113.10:54321"
		clientIP := auth.ClientIP(req)
		h.loginAttemptLimiter.RecordFailure("user@example.com", clientIP)
		require.Equal(t, 4, h.loginAttemptLimiter.Check("user@example.com", clientIP).Remaining)

		rr := httptest.NewRecorder()
		h.Login(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Equal(t, 5, h.loginAttemptLimiter.Check("user@example.com", clientIP).Remaining)
	})

	t.Run("refresh maps suspended tenant memberships", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{
			TenantID:  "tenant-1",
			UserID:    "user-1",
			Role:      tenant.RoleAdmin,
			IsActive:  false,
			CreatedAt: time.Now(),
		}}
		refreshToken := createMockRefreshSession(t, h, "user-1")

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/refresh", map[string]string{
			"refresh_token": refreshToken,
			"tenant_id":     "tenant-1",
		}, nil)
		rr := httptest.NewRecorder()

		h.RefreshToken(rr, req)

		require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Tenant access is suspended")
	})

	t.Run("password reset delivery failures still return accepted", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		expiresAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
		h.passwordResetService.(*mockPasswordResetService).requestResult = &auth.PasswordResetRequestResult{
			Email:     "user@example.com",
			UserID:    "user-1",
			Issued:    true,
			Token:     "reset-token",
			ExpiresAt: &expiresAt,
		}
		h.passwordResetMailer = &passwordResetDeliveryMailer{err: errors.New("smtp down")}
		h.passwordResetSMTPConfig = passwordResetSMTPConfig()
		h.passwordResetBaseURL = "https://app.example.com"

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/request", map[string]string{
			"email": "user@example.com",
		}, nil)
		rr := httptest.NewRecorder()

		h.RequestPasswordReset(rr, req)

		require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "accepted")
	})
}

func TestWave7CoreImportAndJournalBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	t.Run("opening balances rejects malformed json and service errors", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")

		req := badJSONRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import-opening-balances", claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.ImportOpeningBalances(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		accountingRepo.listAccountsErr = errors.New("ledger unavailable")
		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import-opening-balances", map[string]string{
			"entry_date":  "2026-03-31",
			"csv_content": "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n",
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportOpeningBalances(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "list accounts")
	})

	t.Run("journal import maps repository failures", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		accountingRepo.listAccountsErr = errors.New("accounts unavailable")

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import", map[string]string{
			"csv_content": "entry_reference,entry_date,account_code,debit,credit\nLEG-1,2026-03-31,1000,100.00,0\nLEG-1,2026-03-31,3000,0,100.00\n",
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ImportJournalEntries(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "list accounts")
	})

	t.Run("create post and void respect period locks", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		lockDate := "2026-02-28"
		tenantRecord.Settings.PeriodLockDate = &lockDate
		entryDate := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
		accountingRepo.journalEntries["je-locked"] = wave7JournalEntry("je-locked", entryDate, false)

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries", map[string]interface{}{
			"entry_date":  entryDate,
			"description": "Locked manual entry",
			"lines": []map[string]interface{}{
				{"account_id": "cash", "debit_amount": "10.00"},
				{"account_id": "sales", "credit_amount": "10.00"},
			},
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.CreateJournalEntry(rr, req)
		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-locked/post", nil, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-locked"})
		rr = httptest.NewRecorder()
		h.PostJournalEntry(rr, req)
		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-locked/void", map[string]string{"reason": "locked correction"}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": "je-locked"})
		rr = httptest.NewRecorder()
		h.VoidJournalEntry(rr, req)
		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	})

	t.Run("post journal entry maps evidence evaluation errors", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		entry := wave7JournalEntry("je-evidence-error", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), true)
		accountingRepo.journalEntries[entry.ID] = entry
		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("document store unavailable")
		h.documentsService = documents.NewService(docRepo, nil)

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/je-evidence-error/post", nil, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": entry.ID})
		rr := httptest.NewRecorder()

		h.PostJournalEntry(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	})

	t.Run("post journal entry without document service returns evidence conflict", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		entry := wave7JournalEntry("je-evidence-missing-service", time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), true)
		accountingRepo.journalEntries[entry.ID] = entry

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/"+entry.ID+"/post", nil, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": entry.ID})
		rr := httptest.NewRecorder()

		h.PostJournalEntry(rr, req)

		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "approved journal-entry evidence is required")
	})
}

func TestWave7CoreJournalTemplateGenerationBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

	t.Run("due generation maps repository list failures", func(t *testing.T) {
		base, tenantRepo, repo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		base.accountingService = accounting.NewServiceWithRepository(&wave7DueTemplateErrorRepository{mockAccountingRepository: repo})

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/generate-due", map[string]string{
			"as_of_date": "2026-03-31T00:00:00Z",
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		base.GenerateDueJournalEntryTemplates(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "list due recurring journal templates")
	})

	t.Run("single generation rejects explicit locked entry date before service call", func(t *testing.T) {
		h, tenantRepo, repo := setupAccountingTestHandlers()
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		lockDate := "2026-02-28"
		tenantRecord.Settings.PeriodLockDate = &lockDate
		templateDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
		repo.templates["tpl-locked"] = wave7RecurringTemplate("tpl-locked", false, templateDate)
		requested := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/tpl-locked/generate", map[string]interface{}{
			"entry_date": requested,
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": "tpl-locked"})
		rr := httptest.NewRecorder()

		h.GenerateJournalEntryTemplate(rr, req)

		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "period locked through 2026-02-28")
	})

	t.Run("single generation maps auto-post evidence conflicts and generic service errors", func(t *testing.T) {
		h, tenantRepo, repo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		nextDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
		repo.templates["tpl-evidence"] = wave7RecurringTemplate("tpl-evidence", true, nextDate)
		repo.templates["tpl-nonrecurring"] = balancedTemplate("tpl-nonrecurring", false, true)

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/tpl-evidence/generate", map[string]bool{
			"post": true,
		}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": "tpl-evidence"})
		rr := httptest.NewRecorder()
		h.GenerateJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), accounting.ErrTemplateEvidenceAutoPost.Error())

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/tpl-nonrecurring/generate", map[string]string{}, claims)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": "tpl-nonrecurring"})
		rr = httptest.NewRecorder()
		h.GenerateJournalEntryTemplate(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "not recurring")
	})
}

func TestWave7CoreReportFormatValidationBranches(t *testing.T) {
	tests := []struct {
		name   string
		target string
		params map[string]string
		call   func(*Handlers, http.ResponseWriter, *http.Request)
		setup  func(*Handlers)
	}{
		{
			name:   "trial balance invalid format",
			target: "/tenants/tenant-1/reports/trial-balance?format=xml",
			params: map[string]string{"tenantID": "tenant-1"},
			call:   (*Handlers).GetTrialBalance,
		},
		{
			name:   "balance sheet invalid format",
			target: "/tenants/tenant-1/reports/balance-sheet?format=xml",
			params: map[string]string{"tenantID": "tenant-1"},
			call:   (*Handlers).GetBalanceSheet,
		},
		{
			name:   "income statement invalid format",
			target: "/tenants/tenant-1/reports/income-statement?format=xml&start=2026-01-01&end=2026-01-31",
			params: map[string]string{"tenantID": "tenant-1"},
			call:   (*Handlers).GetIncomeStatement,
		},
		{
			name:   "balance confirmation summary invalid format",
			target: "/tenants/tenant-1/reports/balance-confirmations?format=xml&type=RECEIVABLE&as_of_date=2026-01-31",
			params: map[string]string{"tenantID": "tenant-1"},
			call:   (*Handlers).GetBalanceConfirmationSummary,
			setup: func(h *Handlers) {
				misc, _, _, _, _, _ := setupMiscHandlers()
				*h = *misc
			},
		},
		{
			name:   "balance confirmation invalid format",
			target: "/tenants/tenant-1/reports/balance-confirmations/contact-1?format=xml&type=RECEIVABLE&as_of_date=2026-01-31",
			params: map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"},
			call:   (*Handlers).GetBalanceConfirmation,
			setup: func(h *Handlers) {
				misc, _, _, _, _, _ := setupMiscHandlers()
				*h = *misc
			},
		},
		{
			name:   "contact statement invalid format",
			target: "/tenants/tenant-1/reports/contact-statements/contact-1?format=xml&type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31",
			params: map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"},
			call:   (*Handlers).GetContactStatement,
			setup: func(h *Handlers) {
				misc, _, _, _, _, _ := setupMiscHandlers()
				*h = *misc
			},
		},
		{
			name:   "sales margin invalid format",
			target: "/tenants/tenant-1/reports/sales-margin?format=xml&start_date=2026-01-01&end_date=2026-01-31",
			params: map[string]string{"tenantID": "tenant-1"},
			call:   (*Handlers).GetSalesMarginReport,
			setup: func(h *Handlers) {
				misc, _, _, _, _, _ := setupMiscHandlers()
				*h = *misc
			},
		},
		{
			name:   "cost center report invalid format",
			target: "/tenants/tenant-1/cost-centers/report?format=xml",
			params: map[string]string{"tenantID": "tenant-1"},
			call:   (*Handlers).GetCostCenterReport,
			setup: func(h *Handlers) {
				misc, _, _, _, _, _ := setupMiscHandlers()
				*h = *misc
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := setupAccountingTestHandlers()
			if tt.setup != nil {
				tt.setup(h)
			}
			req := withURLParams(httptest.NewRequest(http.MethodGet, tt.target, nil), tt.params)
			rr := httptest.NewRecorder()

			tt.call(h, rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), "format must be json, csv, xlsx, or pdf")
		})
	}
}

func TestWave7CoreCostCenterBudgetAndImportBranches(t *testing.T) {
	t.Run("budget report defaults missing end date", func(t *testing.T) {
		h, tenantRepo, _, _, costCenterRepo, _ := setupMiscHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		budget := decimal.NewFromInt(1200)
		costCenterRepo.CostCenters["cc-1"] = &accounting.CostCenter{
			ID:           "cc-1",
			TenantID:     "tenant-1",
			Code:         "OPS",
			Name:         "Operations",
			IsActive:     true,
			BudgetAmount: &budget,
			BudgetPeriod: accounting.BudgetPeriodAnnual,
		}

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/budget-vs-actual?start_date=2026-01-01", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.GetBudgetVsActualReport(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Operations")
	})

	t.Run("cost center imports map repository list failures", func(t *testing.T) {
		h := &Handlers{
			costCenterService: accounting.NewCostCenterServiceWithRepository(&costCenterErrorRepository{
				MockCostCenterRepository: accounting.NewMockCostCenterRepository(),
				listErr:                  errors.New("cost center list failed"),
			}),
		}

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/import", map[string]string{
			"csv_content": "code,name\nOPS,Operations\n",
		}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.ImportCostCenters(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "list existing cost centers")

		req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations/import", map[string]string{
			"csv_content": "cost_center_code,journal_entry_line_id,amount,allocation_date\nOPS,11111111-1111-4111-8111-111111111111,10.00,2026-01-15\n",
		}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.ImportCostAllocations(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "list cost centers")
	})

	t.Run("list allocations maps repository failures after date parsing", func(t *testing.T) {
		h := &Handlers{costCenterService: accounting.NewCostCenterServiceWithRepository(&costCenterErrorRepository{
			MockCostCenterRepository: accounting.NewMockCostCenterRepository(),
			listAllocationsErr:       errors.New("allocation list failed"),
		})}

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/allocations?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.ListCostAllocations(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	})
}

type wave7DueTemplateErrorRepository struct {
	*mockAccountingRepository
}

func (m *wave7DueTemplateErrorRepository) GetDueJournalEntryTemplateIDs(context.Context, string, string, time.Time) ([]string, error) {
	return nil, errors.New("due template list failed")
}

func wave7JournalEntry(id string, entryDate time.Time, requiresEvidence bool) *accounting.JournalEntry {
	return &accounting.JournalEntry{
		ID:               id,
		TenantID:         "tenant-1",
		EntryNumber:      strings.ToUpper(id),
		EntryDate:        entryDate,
		Description:      "Wave7 journal entry",
		Status:           accounting.StatusDraft,
		RequiresEvidence: requiresEvidence,
		CreatedBy:        "user-1",
		Lines: []accounting.JournalEntryLine{
			{AccountID: "cash", DebitAmount: decimal.NewFromInt(10), BaseDebit: decimal.NewFromInt(10), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1)},
			{AccountID: "sales", CreditAmount: decimal.NewFromInt(10), BaseCredit: decimal.NewFromInt(10), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1)},
		},
	}
}

func wave7RecurringTemplate(id string, requiresEvidence bool, nextDate time.Time) *accounting.JournalEntryTemplate {
	template := balancedTemplate(id, requiresEvidence, true)
	template.Frequency = accounting.JournalEntryTemplateFrequencyMonthly
	template.StartDate = &nextDate
	template.NextGenerationDate = &nextDate
	return template
}
