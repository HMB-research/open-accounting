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
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestUpdateAccountCoreMoreBranches(t *testing.T) {
	t.Run("rejects invalid json before service validation", func(t *testing.T) {
		h, _, _ := setupAccountingTestHandlers()
		req := withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/accounts/acc-1", strings.NewReader("{")), map[string]string{
			"tenantID":  "tenant-1",
			"accountID": "acc-1",
		})
		rr := httptest.NewRecorder()

		h.UpdateAccount(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		h, _, _ := setupAccountingTestHandlers()
		req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/accounts/acc-1", map[string]string{"code": "1001"}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		rr := httptest.NewRecorder()

		h.UpdateAccount(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Code, name, and account_type are required")
	})

	t.Run("updates editable account", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		parentID := "11111111-1111-4111-8111-111111111111"
		accountingRepo.accounts["acc-1"] = &accounting.Account{
			ID:          "acc-1",
			TenantID:    "tenant-1",
			Code:        "1000",
			Name:        "Cash",
			AccountType: accounting.AccountTypeAsset,
			IsActive:    true,
		}

		req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/accounts/acc-1", map[string]interface{}{
			"code":         "1005",
			"name":         "Operating Cash",
			"account_type": accounting.AccountTypeAsset,
			"parent_id":    parentID,
			"description":  "Daily cash account",
		}, nil)
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		rr := httptest.NewRecorder()

		h.UpdateAccount(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		updated := accountingRepo.accounts["acc-1"]
		require.NotNil(t, updated.ParentID)
		assert.Equal(t, parentID, *updated.ParentID)
		assert.Equal(t, "1005", updated.Code)
		assert.Equal(t, "Operating Cash", updated.Name)
		assert.Equal(t, "Daily cash account", updated.Description)
	})

	t.Run("maps service validation and repository errors", func(t *testing.T) {
		tests := []struct {
			name       string
			account    *accounting.Account
			req        map[string]interface{}
			setupRepo  func(*mockAccountingRepository)
			wantStatus int
			wantBody   string
		}{
			{
				name: "invalid account type",
				account: &accounting.Account{
					ID:          "acc-1",
					TenantID:    "tenant-1",
					Code:        "1000",
					Name:        "Cash",
					AccountType: accounting.AccountTypeAsset,
				},
				req: map[string]interface{}{
					"code":         "1000",
					"name":         "Cash",
					"account_type": "NOPE",
				},
				wantStatus: http.StatusBadRequest,
				wantBody:   "invalid account_type",
			},
			{
				name: "system account immutable",
				account: &accounting.Account{
					ID:          "acc-1",
					TenantID:    "tenant-1",
					Code:        "3000",
					Name:        "Retained Earnings",
					AccountType: accounting.AccountTypeEquity,
					IsSystem:    true,
				},
				req: map[string]interface{}{
					"code":         "3000",
					"name":         "Retained Earnings",
					"account_type": accounting.AccountTypeEquity,
				},
				wantStatus: http.StatusBadRequest,
				wantBody:   "system accounts cannot be modified",
			},
			{
				name: "update repository error becomes not found",
				account: &accounting.Account{
					ID:          "acc-1",
					TenantID:    "tenant-1",
					Code:        "1000",
					Name:        "Cash",
					AccountType: accounting.AccountTypeAsset,
				},
				req: map[string]interface{}{
					"code":         "1000",
					"name":         "Cash",
					"account_type": accounting.AccountTypeAsset,
				},
				setupRepo: func(repo *mockAccountingRepository) {
					repo.updateAccountErr = errors.New("write failed")
				},
				wantStatus: http.StatusNotFound,
				wantBody:   "Account not found",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
				tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
				accountingRepo.accounts[tt.account.ID] = tt.account
				if tt.setupRepo != nil {
					tt.setupRepo(accountingRepo)
				}

				req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/accounts/acc-1", tt.req, nil)
				req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
				rr := httptest.NewRecorder()

				h.UpdateAccount(rr, req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})
}

func TestApplyJournalEntryTemplateCoreMoreBranches(t *testing.T) {
	t.Run("rejects invalid json", func(t *testing.T) {
		h, tenantRepo, _ := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/tpl/apply", nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner))
		req.Body = http.NoBody
		req = httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/tpl/apply", strings.NewReader("{")).WithContext(req.Context())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": "tpl"})
		rr := httptest.NewRecorder()

		h.ApplyJournalEntryTemplate(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid request body")
	})

	t.Run("creates draft entry from active template", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		templateID := "22222222-2222-4222-8222-222222222222"
		accountingRepo.templates[templateID] = balancedTemplate(templateID, false, true)

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/"+templateID+"/apply", map[string]interface{}{
			"entry_date":  time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
			"description": "February accrual",
			"reference":   "ACCR-2026-02",
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": templateID})
		rr := httptest.NewRecorder()

		h.ApplyJournalEntryTemplate(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		require.Len(t, accountingRepo.journalEntries, 1)
		for _, entry := range accountingRepo.journalEntries {
			assert.Equal(t, accounting.SourceTypeJournalTemplate, entry.SourceType)
			require.NotNil(t, entry.SourceID)
			assert.Equal(t, templateID, *entry.SourceID)
			assert.Equal(t, "February accrual", entry.Description)
			assert.Equal(t, "ACCR-2026-02", entry.Reference)
			assert.Equal(t, accounting.StatusDraft, entry.Status)
		}
	})

	t.Run("maps service errors to conflict not found and bad request", func(t *testing.T) {
		tests := []struct {
			name       string
			templateID string
			template   *accounting.JournalEntryTemplate
			body       map[string]interface{}
			wrapRepo   func(*mockAccountingRepository) accounting.RepositoryInterface
			wantStatus int
			wantBody   string
		}{
			{
				name:       "evidence template cannot auto-post",
				templateID: "33333333-3333-4333-8333-333333333333",
				template:   balancedTemplate("33333333-3333-4333-8333-333333333333", true, true),
				body:       map[string]interface{}{"post": true},
				wantStatus: http.StatusConflict,
				wantBody:   "cannot auto-post",
			},
			{
				name:       "missing template",
				templateID: "44444444-4444-4444-8444-444444444444",
				body:       map[string]interface{}{},
				wrapRepo: func(repo *mockAccountingRepository) accounting.RepositoryInterface {
					return &templateNotFoundAccountingRepository{mockAccountingRepository: repo}
				},
				wantStatus: http.StatusNotFound,
				wantBody:   "not found",
			},
			{
				name:       "inactive template",
				templateID: "55555555-5555-4555-8555-555555555555",
				template:   balancedTemplate("55555555-5555-4555-8555-555555555555", false, false),
				body:       map[string]interface{}{},
				wantStatus: http.StatusBadRequest,
				wantBody:   "inactive",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
				tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
				if tt.template != nil {
					accountingRepo.templates[tt.templateID] = tt.template
				}
				if tt.wrapRepo != nil {
					h.accountingService = accounting.NewServiceWithRepository(tt.wrapRepo(accountingRepo))
				}

				req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/"+tt.templateID+"/apply", tt.body, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner))
				req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": tt.templateID})
				rr := httptest.NewRecorder()

				h.ApplyJournalEntryTemplate(rr, req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("rejects locked periods before applying template", func(t *testing.T) {
		h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
		lockedDate := "2026-03-31"
		tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		tenantRecord.Settings.PeriodLockDate = &lockedDate
		templateID := "66666666-6666-4666-8666-666666666666"
		accountingRepo.templates[templateID] = balancedTemplate(templateID, false, true)

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entry-templates/"+templateID+"/apply", map[string]interface{}{
			"entry_date": time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		}, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "templateID": templateID})
		rr := httptest.NewRecorder()

		h.ApplyJournalEntryTemplate(rr, req)

		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "period locked through 2026-03-31")
		assert.Empty(t, accountingRepo.journalEntries)
	})
}

func TestAuthSessionCoreMoreBranches(t *testing.T) {
	claims := createTestClaims("user-1", "user@example.com", "", "")

	t.Run("requires authentication", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		handlers := []struct {
			name   string
			invoke func(http.ResponseWriter, *http.Request)
			req    *http.Request
		}{
			{name: "list", invoke: h.ListAuthSessions, req: httptest.NewRequest(http.MethodGet, "/auth/sessions", nil)},
			{name: "revoke one", invoke: h.RevokeAuthSession, req: withURLParams(httptest.NewRequest(http.MethodDelete, "/auth/sessions/session-1", nil), map[string]string{"sessionID": "session-1"})},
			{name: "revoke all", invoke: h.RevokeAllAuthSessions, req: httptest.NewRequest(http.MethodDelete, "/auth/sessions", nil)},
		}

		for _, tt := range handlers {
			t.Run(tt.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				tt.invoke(rr, tt.req)
				require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
			})
		}
	})

	t.Run("reports unavailable refresh session service", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		h.refreshSessionService = nil
		handlers := []struct {
			name   string
			invoke func(http.ResponseWriter, *http.Request)
			req    *http.Request
		}{
			{name: "list", invoke: h.ListAuthSessions, req: makeAuthenticatedRequest(http.MethodGet, "/auth/sessions", nil, claims)},
			{name: "revoke one", invoke: h.RevokeAuthSession, req: withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/auth/sessions/session-1", nil, claims), map[string]string{"sessionID": "session-1"})},
			{name: "revoke all", invoke: h.RevokeAllAuthSessions, req: makeAuthenticatedRequest(http.MethodDelete, "/auth/sessions", nil, claims)},
		}

		for _, tt := range handlers {
			t.Run(tt.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				tt.invoke(rr, tt.req)
				require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), "Refresh session service unavailable")
			})
		}
	})

	t.Run("maps refresh service errors", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		refreshSvc := &failingRefreshSessionService{
			mockRefreshSessionService: newMockRefreshSessionService(),
			revokeByIDErr:             errors.New("database down"),
			revokeAllErr:              errors.New("database down"),
		}
		refreshSvc.listErr = errors.New("database down")
		h.refreshSessionService = refreshSvc

		listReq := makeAuthenticatedRequest(http.MethodGet, "/auth/sessions", nil, claims)
		rr := httptest.NewRecorder()
		h.ListAuthSessions(rr, listReq)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to list refresh sessions")

		missingIDReq := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/auth/sessions/%20", nil, claims), map[string]string{"sessionID": " "})
		rr = httptest.NewRecorder()
		h.RevokeAuthSession(rr, missingIDReq)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Session id is required")

		revokeReq := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/auth/sessions/session-1", nil, claims), map[string]string{"sessionID": "session-1"})
		rr = httptest.NewRecorder()
		h.RevokeAuthSession(rr, revokeReq)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to revoke refresh session")

		revokeAllReq := makeAuthenticatedRequest(http.MethodDelete, "/auth/sessions", nil, claims)
		rr = httptest.NewRecorder()
		h.RevokeAllAuthSessions(rr, revokeAllReq)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to revoke refresh sessions")
	})
}

func TestAuthResetLogoutAuditCoreMoreBranches(t *testing.T) {
	t.Run("logout validation and service errors", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", strings.NewReader("{"))
		rr := httptest.NewRecorder()
		h.Logout(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		req = makeAuthenticatedRequest(http.MethodPost, "/auth/logout", map[string]string{"refresh_token": "not-a-token"}, nil)
		rr = httptest.NewRecorder()
		h.Logout(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())

		refreshToken, _, err := h.generateRefreshTokenWithClaims("user-1")
		require.NoError(t, err)
		h.refreshSessionService = nil
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/logout", map[string]string{"refresh_token": refreshToken}, nil)
		rr = httptest.NewRecorder()
		h.Logout(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Refresh session service unavailable")

		h, _ = setupAuthTestHandlers()
		refreshToken, _, err = h.generateRefreshTokenWithClaims("user-1")
		require.NoError(t, err)
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/logout", map[string]string{"refresh_token": refreshToken}, nil)
		rr = httptest.NewRecorder()
		h.Logout(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())

		h, _ = setupAuthTestHandlers()
		refreshSvc := h.refreshSessionService.(*mockRefreshSessionService)
		refreshSvc.revokeErr = errors.New("revoke failed")
		refreshToken, _, err = h.generateRefreshTokenWithClaims("user-1")
		require.NoError(t, err)
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/logout", map[string]string{"refresh_token": refreshToken}, nil)
		rr = httptest.NewRecorder()
		h.Logout(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to revoke refresh session")
	})

	t.Run("password reset request branches", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/request", strings.NewReader("{"))
		rr := httptest.NewRecorder()
		h.RequestPasswordReset(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/request", map[string]string{"email": " "}, nil)
		rr = httptest.NewRecorder()
		h.RequestPasswordReset(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Email is required")

		h.passwordResetService = nil
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/request", map[string]string{"email": "user@example.com"}, nil)
		rr = httptest.NewRecorder()
		h.RequestPasswordReset(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())

		h, _ = setupAuthTestHandlers()
		h.passwordResetService.(*mockPasswordResetService).requestErr = errors.New("reset request failed")
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/request", map[string]string{"email": "user@example.com"}, nil)
		rr = httptest.NewRecorder()
		h.RequestPasswordReset(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to request password reset")

		h, _ = setupAuthTestHandlers()
		expiresAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
		h.passwordResetExposeToken = true
		h.passwordResetService.(*mockPasswordResetService).requestResult = passwordResetRequestResult(&expiresAt)
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/request", map[string]string{"email": "user@example.com"}, nil)
		rr = httptest.NewRecorder()
		h.RequestPasswordReset(rr, req)
		require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "reset-token")
	})

	t.Run("password reset confirmation branches", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		req := httptest.NewRequest(http.MethodPost, "/auth/password-reset/confirm", strings.NewReader("{"))
		rr := httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "", "new_password": ""}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Token and new password are required")

		h.passwordResetService = nil
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "token", "new_password": "newpassword123"}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Password reset service unavailable")

		h, _ = setupAuthTestHandlers()
		h.refreshSessionService = nil
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "token", "new_password": "newpassword123"}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Refresh session service unavailable")

		h, _ = setupAuthTestHandlers()
		h.passwordResetService.(*mockPasswordResetService).resetErr = auth.ErrPasswordResetTokenInvalid
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "bad", "new_password": "newpassword123"}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())

		h, _ = setupAuthTestHandlers()
		h.passwordResetService.(*mockPasswordResetService).resetErr = errors.New("weak password")
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "token", "new_password": "short"}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

		h, _ = setupAuthTestHandlers()
		h.passwordResetService.(*mockPasswordResetService).resetTokens["token"] = "user-1"
		h.refreshSessionService = &failingRefreshSessionService{
			mockRefreshSessionService: newMockRefreshSessionService(),
			revokeAllErr:              errors.New("revoke failed"),
		}
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "token", "new_password": "newpassword123"}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to revoke refresh sessions")

		h, _ = setupAuthTestHandlers()
		h.passwordResetService.(*mockPasswordResetService).resetTokens["token"] = "user-1"
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/password-reset/confirm", map[string]string{"token": "token", "new_password": "newpassword123"}, nil)
		rr = httptest.NewRecorder()
		h.ResetPassword(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("security audit list validation and errors", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		req := httptest.NewRequest(http.MethodGet, "/auth/security-events", nil)
		rr := httptest.NewRecorder()
		h.ListSecurityAuditEvents(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())

		claims := createTestClaims("user-1", "user@example.com", "", "")
		h.securityAuditService = nil
		req = makeAuthenticatedRequest(http.MethodGet, "/auth/security-events", nil, claims)
		rr = httptest.NewRecorder()
		h.ListSecurityAuditEvents(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Security audit service unavailable")

		h, _ = setupAuthTestHandlers()
		req = makeAuthenticatedRequest(http.MethodGet, "/auth/security-events?limit=0", nil, claims)
		rr = httptest.NewRecorder()
		h.ListSecurityAuditEvents(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Invalid limit")

		h, _ = setupAuthTestHandlers()
		h.securityAuditService.(*mockSecurityAuditService).err = errors.New("audit failed")
		req = makeAuthenticatedRequest(http.MethodGet, "/auth/security-events?limit=10", nil, claims)
		rr = httptest.NewRecorder()
		h.ListSecurityAuditEvents(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to list security audit events")
	})
}

func TestConsolidatedReportCoreMoreBranches(t *testing.T) {
	t.Run("requires authentication", func(t *testing.T) {
		h, _, _ := setupAccountingTestHandlers()
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/consolidated", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.GetConsolidatedReport(rr, req)

		require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	})

	t.Run("rejects tenant and date parser edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			tenantID string
			query    string
			wantBody string
		}{
			{
				name:     "blank selected tenants",
				tenantID: "",
				query:    "tenant_ids=,%20",
				wantBody: "at least one tenant_id is required",
			},
			{
				name:     "too many tenants",
				tenantID: "tenant-1",
				query:    "tenant_ids=t01,t02,t03,t04,t05,t06,t07,t08,t09,t10,t11,t12,t13,t14,t15,t16,t17,t18,t19,t20,t21",
				wantBody: "at most 20 tenants",
			},
			{
				name:     "invalid start date",
				tenantID: "tenant-1",
				query:    "start=bad",
				wantBody: "invalid start date format",
			},
			{
				name:     "invalid end date",
				tenantID: "tenant-1",
				query:    "end=bad",
				wantBody: "invalid end date format",
			},
			{
				name:     "end before start",
				tenantID: "tenant-1",
				query:    "start=2026-02-01&end=2026-01-31",
				wantBody: "end date must be on or after start date",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, _, _ := setupAccountingTestHandlers()
				req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID+"/reports/consolidated?"+tt.query, nil, createTestClaims("user-1", "user@example.com", "", ""))
				req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
				rr := httptest.NewRecorder()

				h.GetConsolidatedReport(rr, req)

				require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("allows api token scoped to anchor tenant", func(t *testing.T) {
		h, tenantRepo, _ := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/consolidated?as_of_date=2026-12-31", nil, &auth.Claims{
			UserID:    "api-token",
			TenantID:  "tenant-1",
			TokenKind: auth.TokenKindAPIToken,
		})
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.GetConsolidatedReport(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"tenant_count":1`)
	})

	t.Run("maps tenant access resolution errors", func(t *testing.T) {
		h, tenantRepo, _ := setupAccountingTestHandlers()
		tenantRepo.getTenantErr = errors.New("tenant lookup failed")
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/consolidated", nil, &auth.Claims{
			UserID:    "api-token",
			TenantID:  "tenant-1",
			TokenKind: auth.TokenKindAPIToken,
		})
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.GetConsolidatedReport(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to resolve tenant access")
	})

	t.Run("maps downstream accounting report errors", func(t *testing.T) {
		tests := []struct {
			name         string
			repo         accounting.RepositoryInterface
			wantBody     string
			wantHTTPCode int
		}{
			{
				name: "trial balance failure",
				repo: &consolidatedAccountingRepository{
					mockAccountingRepository: newMockAccountingRepository(),
					trialErrOnCall:           1,
				},
				wantBody:     "Failed to generate consolidated trial balance",
				wantHTTPCode: http.StatusInternalServerError,
			},
			{
				name: "balance sheet failure",
				repo: &consolidatedAccountingRepository{
					mockAccountingRepository: newMockAccountingRepository(),
					trialErrOnCall:           2,
				},
				wantBody:     "Failed to generate consolidated balance sheet",
				wantHTTPCode: http.StatusInternalServerError,
			},
			{
				name: "income statement failure",
				repo: &consolidatedAccountingRepository{
					mockAccountingRepository: &mockAccountingRepository{periodBalances: nil, getPeriodBalancesErr: errors.New("period failed"), accounts: map[string]*accounting.Account{}, journalEntries: map[string]*accounting.JournalEntry{}, templates: map[string]*accounting.JournalEntryTemplate{}},
				},
				wantBody:     "Failed to generate consolidated income statement",
				wantHTTPCode: http.StatusInternalServerError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tenantRepo := newMockTenantRepository()
				tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
				tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner}}
				h := &Handlers{
					tenantService:     tenant.NewServiceWithRepository(tenantRepo),
					accountingService: accounting.NewServiceWithRepository(tt.repo),
				}

				req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/consolidated", nil, createTestClaims("user-1", "user@example.com", "", ""))
				req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
				rr := httptest.NewRecorder()

				h.GetConsolidatedReport(rr, req)

				require.Equal(t, tt.wantHTTPCode, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})
}

func TestReportValidationAndErrorCoreMoreBranches(t *testing.T) {
	t.Run("cash flow mapping validation and service errors", func(t *testing.T) {
		h, _, reportsRepo, _, _, _ := setupMiscHandlers()
		reportsRepo.GetCashFlowMappingErr = errors.New("mapping read failed")
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/cash-flow/mapping", nil), map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()
		h.GetCashFlowMapping(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to get cash flow mapping")

		h, _, _, _, _, _ = setupMiscHandlers()
		req = withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/reports/cash-flow/mapping", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.UpdateCashFlowMapping(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Invalid request body")

		req = withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/reports/cash-flow/mapping", map[string][]string{
			"operating_account_codes": {"1200"},
			"investing_account_codes": {"1200"},
		}, nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.UpdateCashFlowMapping(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "cannot be assigned to both")

		h, _, reportsRepo, _, _, _ = setupMiscHandlers()
		reportsRepo.UpdateCashFlowMappingErr = errors.New("mapping update failed")
		req = withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/reports/cash-flow/mapping", map[string][]string{
			"operating_account_codes": {"1200"},
		}, nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.UpdateCashFlowMapping(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to update cash flow mapping")

		h, _, _, _, _, _ = setupMiscHandlers()
		req = withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/reports/cash-flow/mapping", map[string][]string{
			"operating_account_codes": {" 1200 ", "1100"},
		}, nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.UpdateCashFlowMapping(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "1100")
	})

	t.Run("balance confirmation summary validation and service errors", func(t *testing.T) {
		tests := []struct {
			name       string
			path       string
			setupRepo  func(*reports.MockRepository)
			wantStatus int
			wantBody   string
		}{
			{name: "missing type", path: "/tenants/tenant-1/reports/balance-confirmations?as_of_date=2026-01-31", wantStatus: http.StatusBadRequest, wantBody: "type parameter is required"},
			{name: "missing as of date", path: "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE", wantStatus: http.StatusBadRequest, wantBody: "as_of_date parameter is required"},
			{name: "invalid date", path: "/tenants/tenant-1/reports/balance-confirmations?type=PAYABLE&as_of_date=not-a-date", wantStatus: http.StatusBadRequest, wantBody: "Invalid as_of_date format"},
			{
				name: "repo error",
				path: "/tenants/tenant-1/reports/balance-confirmations?type=PAYABLE&as_of_date=2026-01-31",
				setupRepo: func(repo *reports.MockRepository) {
					repo.GetContactBalancesErr = errors.New("summary failed")
				},
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to get balance confirmation summary",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, _, reportsRepo, _, _, _ := setupMiscHandlers()
				if tt.setupRepo != nil {
					tt.setupRepo(reportsRepo)
				}
				req := withURLParams(httptest.NewRequest(http.MethodGet, tt.path, nil), map[string]string{"tenantID": "tenant-1"})
				rr := httptest.NewRecorder()

				h.GetBalanceConfirmationSummary(rr, req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("specific balance confirmation validation and service errors", func(t *testing.T) {
		tests := []struct {
			name       string
			path       string
			setupRepo  func(*reports.MockRepository)
			wantStatus int
			wantBody   string
		}{
			{name: "missing type", path: "/tenants/tenant-1/reports/balance-confirmations/contact-1?as_of_date=2026-01-31", wantStatus: http.StatusBadRequest, wantBody: "type parameter is required"},
			{name: "invalid type", path: "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=BAD&as_of_date=2026-01-31", wantStatus: http.StatusBadRequest, wantBody: "type must be RECEIVABLE or PAYABLE"},
			{name: "missing as of date", path: "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE", wantStatus: http.StatusBadRequest, wantBody: "as_of_date parameter is required"},
			{
				name: "repo error",
				path: "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=2026-01-31",
				setupRepo: func(repo *reports.MockRepository) {
					repo.GetContactErr = errors.New("contact failed")
				},
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to get balance confirmation",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h, _, reportsRepo, _, _, _ := setupMiscHandlers()
				reportsRepo.Contact = reports.ContactInfo{ID: "contact-1", Name: "Contact One"}
				if tt.setupRepo != nil {
					tt.setupRepo(reportsRepo)
				}
				req := withURLParams(httptest.NewRequest(http.MethodGet, tt.path, nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
				rr := httptest.NewRecorder()

				h.GetBalanceConfirmation(rr, req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("contact statement and sales margin service errors", func(t *testing.T) {
		h, _, reportsRepo, _, _, _ := setupMiscHandlers()
		reportsRepo.Contact = reports.ContactInfo{ID: "contact-1", Name: "Contact One"}
		reportsRepo.GetContactStatementOpeningErr = errors.New("opening failed")
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/contact-statements/contact-1?type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
		rr := httptest.NewRecorder()
		h.GetContactStatement(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to get contact statement")

		h, _, reportsRepo, _, _, _ = setupMiscHandlers()
		reportsRepo.GetSalesMarginLinesErr = errors.New("margin failed")
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/sales-margin?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
		rr = httptest.NewRecorder()
		h.GetSalesMarginReport(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to get sales margin report")
	})

	t.Run("direct query helpers cover validation edges", func(t *testing.T) {
		contactCases := []struct {
			name      string
			contactID string
			query     string
			want      string
		}{
			{name: "missing type", contactID: "contact-1", query: "start_date=2026-01-01&end_date=2026-01-31", want: "type parameter is required"},
			{name: "invalid type", contactID: "contact-1", query: "type=OTHER&start_date=2026-01-01&end_date=2026-01-31", want: "type must be RECEIVABLE or PAYABLE"},
			{name: "missing start", contactID: "contact-1", query: "type=RECEIVABLE&end_date=2026-01-31", want: "start_date parameter is required"},
			{name: "bad start", contactID: "contact-1", query: "type=RECEIVABLE&start_date=bad&end_date=2026-01-31", want: "invalid start_date format"},
			{name: "missing end", contactID: "contact-1", query: "type=RECEIVABLE&start_date=2026-01-01", want: "end_date parameter is required"},
			{name: "bad end", contactID: "contact-1", query: "type=RECEIVABLE&start_date=2026-01-01&end_date=bad", want: "invalid end_date format"},
			{name: "missing contact", contactID: " ", query: "type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31", want: "contactID path parameter is required"},
		}
		for _, tt := range contactCases {
			t.Run("contact statement "+tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
				_, err := contactStatementRequestFromQuery(tt.contactID, req)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.want)
			})
		}

		marginCases := []struct {
			name  string
			query string
			want  string
		}{
			{name: "missing start", query: "end_date=2026-01-31", want: "start_date parameter is required"},
			{name: "bad start", query: "start_date=bad&end_date=2026-01-31", want: "invalid start_date format"},
			{name: "missing end", query: "start_date=2026-01-01", want: "end_date parameter is required"},
			{name: "bad end", query: "start_date=2026-01-01&end_date=bad", want: "invalid end_date format"},
		}
		for _, tt := range marginCases {
			t.Run("sales margin "+tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)
				_, err := salesMarginRequestFromQuery(req)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.want)
			})
		}
	})
}

func TestCostCenterCoreMoreBranches(t *testing.T) {
	t.Run("maps repository error classifications", func(t *testing.T) {
		costCenterID := "77777777-7777-4777-8777-777777777777"
		validAllocationBody := map[string]interface{}{
			"cost_center_id":        costCenterID,
			"journal_entry_line_id": "88888888-8888-4888-8888-888888888888",
			"amount":                "10.00",
			"allocation_date":       "2026-01-20T00:00:00Z",
		}

		tests := []struct {
			name       string
			repo       *costCenterErrorRepository
			request    *http.Request
			invoke     func(*Handlers, http.ResponseWriter, *http.Request)
			wantStatus int
			wantBody   string
		}{
			{
				name:       "list internal error",
				repo:       &costCenterErrorRepository{listErr: errors.New("list failed")},
				request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers", nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.ListCostCenters(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "list failed",
			},
			{
				name:       "get internal error",
				repo:       &costCenterErrorRepository{getErr: errors.New("lookup failed")},
				request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/"+costCenterID, nil), map[string]string{"tenantID": "tenant-1", "costCenterID": costCenterID}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.GetCostCenter(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "lookup failed",
			},
			{
				name:       "create internal error",
				repo:       &costCenterErrorRepository{createErr: errors.New("create failed")},
				request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers", map[string]string{"code": "OPS", "name": "Operations"}, nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CreateCostCenter(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "create failed",
			},
			{
				name:       "update internal error",
				repo:       seededCostCenterErrorRepository(costCenterID, errors.New("update failed")),
				request:    withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/cost-centers/"+costCenterID, map[string]string{"code": "OPS", "name": "Operations"}, nil), map[string]string{"tenantID": "tenant-1", "costCenterID": costCenterID}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.UpdateCostCenter(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "update failed",
			},
			{
				name:       "delete conflict",
				repo:       &costCenterErrorRepository{deleteErr: errors.New("cannot delete cost center with allocations")},
				request:    withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/cost-centers/"+costCenterID, nil, nil), map[string]string{"tenantID": "tenant-1", "costCenterID": costCenterID}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.DeleteCostCenter(w, r) },
				wantStatus: http.StatusConflict,
				wantBody:   "cannot delete",
			},
			{
				name:       "delete internal error",
				repo:       &costCenterErrorRepository{deleteErr: errors.New("delete failed")},
				request:    withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/cost-centers/"+costCenterID, nil, nil), map[string]string{"tenantID": "tenant-1", "costCenterID": costCenterID}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.DeleteCostCenter(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "delete failed",
			},
			{
				name:       "create allocation internal error",
				repo:       seededCostCenterAllocationErrorRepository(costCenterID, errors.New("allocation failed")),
				request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations", validAllocationBody, nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.CreateCostAllocation(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "allocation failed",
			},
			{
				name:       "report internal error",
				repo:       &costCenterErrorRepository{listErr: errors.New("report failed")},
				request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/report?start_date=2026-01-01&end_date=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     func(h *Handlers, w http.ResponseWriter, r *http.Request) { h.GetCostCenterReport(w, r) },
				wantStatus: http.StatusInternalServerError,
				wantBody:   "report failed",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h := &Handlers{costCenterService: accounting.NewCostCenterServiceWithRepository(tt.repo)}
				rr := httptest.NewRecorder()

				tt.invoke(h, rr, tt.request)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("covers request parsing branches", func(t *testing.T) {
		h, _, _, _, _, _ := setupMiscHandlers()

		cases := []struct {
			name       string
			req        *http.Request
			invoke     func(http.ResponseWriter, *http.Request)
			wantStatus int
			wantBody   string
		}{
			{
				name:       "create invalid json",
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/cost-centers", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.CreateCostCenter,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "import cost centers invalid json",
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/import", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.ImportCostCenters,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "import cost centers missing content",
				req:        withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/import", map[string]string{"file_name": "cc.csv"}, nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.ImportCostCenters,
				wantStatus: http.StatusBadRequest,
				wantBody:   "csv_content is required",
			},
			{
				name:       "import allocations invalid json",
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations/import", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.ImportCostAllocations,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "import allocations missing content",
				req:        withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations/import", map[string]string{"file_name": "alloc.csv"}, nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.ImportCostAllocations,
				wantStatus: http.StatusBadRequest,
				wantBody:   "csv_content is required",
			},
			{
				name:       "list allocations bad start date",
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/allocations?start_date=bad", nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.ListCostAllocations,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid start_date format",
			},
			{
				name:       "list allocations bad end date",
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/allocations?end_date=bad", nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.ListCostAllocations,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid end_date format",
			},
			{
				name:       "create allocation invalid json",
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/cost-centers/allocations", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.CreateCostAllocation,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "update invalid json",
				req:        withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/cost-centers/cc-1", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "costCenterID": "cc-1"}),
				invoke:     h.UpdateCostCenter,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "report bad end date",
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/cost-centers/report?end_date=bad", nil), map[string]string{"tenantID": "tenant-1"}),
				invoke:     h.GetCostCenterReport,
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid end_date format",
			},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				rr := httptest.NewRecorder()

				tt.invoke(rr, tt.req)

				require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
				assert.Contains(t, rr.Body.String(), tt.wantBody)
			})
		}
	})
}

func balancedTemplate(id string, requiresEvidence, active bool) *accounting.JournalEntryTemplate {
	return &accounting.JournalEntryTemplate{
		ID:               id,
		TenantID:         "tenant-1",
		Name:             "Monthly accrual",
		Description:      "Default accrual",
		Reference:        "ACCR",
		RequiresEvidence: requiresEvidence,
		IsActive:         active,
		Lines: []accounting.JournalEntryTemplateLine{
			{
				ID:           "line-1",
				TemplateID:   id,
				LineNumber:   1,
				AccountID:    "expense",
				Description:  "Expense",
				DebitAmount:  decimal.NewFromInt(100),
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			},
			{
				ID:           "line-2",
				TemplateID:   id,
				LineNumber:   2,
				AccountID:    "accrual",
				Description:  "Accrued liability",
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromInt(100),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			},
		},
	}
}

type failingRefreshSessionService struct {
	*mockRefreshSessionService
	revokeByIDErr error
	revokeAllErr  error
}

func (m *failingRefreshSessionService) RevokeRefreshSessionByID(ctx context.Context, userID, tokenID string) error {
	if m.revokeByIDErr != nil {
		return m.revokeByIDErr
	}
	return m.mockRefreshSessionService.RevokeRefreshSessionByID(ctx, userID, tokenID)
}

func (m *failingRefreshSessionService) RevokeAllRefreshSessions(ctx context.Context, userID string) error {
	if m.revokeAllErr != nil {
		return m.revokeAllErr
	}
	return m.mockRefreshSessionService.RevokeAllRefreshSessions(ctx, userID)
}

type consolidatedAccountingRepository struct {
	*mockAccountingRepository
	trialCalls     int
	trialErrOnCall int
}

func (m *consolidatedAccountingRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]accounting.AccountBalance, error) {
	m.trialCalls++
	if m.trialCalls == m.trialErrOnCall {
		return nil, errors.New("trial failed")
	}
	return m.mockAccountingRepository.GetTrialBalance(ctx, schemaName, tenantID, asOfDate)
}

type templateNotFoundAccountingRepository struct {
	*mockAccountingRepository
}

func (m *templateNotFoundAccountingRepository) GetJournalEntryTemplateByID(ctx context.Context, schemaName, tenantID, templateID string) (*accounting.JournalEntryTemplate, error) {
	return nil, errors.New("journal entry template not found")
}

type costCenterErrorRepository struct {
	*accounting.MockCostCenterRepository
	getErr              error
	listErr             error
	createErr           error
	updateErr           error
	deleteErr           error
	expensesErr         error
	createAllocationErr error
	listAllocationsErr  error
}

func newCostCenterErrorRepository() *costCenterErrorRepository {
	return &costCenterErrorRepository{MockCostCenterRepository: accounting.NewMockCostCenterRepository()}
}

func seededCostCenterErrorRepository(costCenterID string, updateErr error) *costCenterErrorRepository {
	repo := newCostCenterErrorRepository()
	repo.MockCostCenterRepository.CostCenters[costCenterID] = &accounting.CostCenter{
		ID:       costCenterID,
		TenantID: "tenant-1",
		Code:     "OPS",
		Name:     "Operations",
		IsActive: true,
	}
	repo.updateErr = updateErr
	return repo
}

func seededCostCenterAllocationErrorRepository(costCenterID string, createAllocationErr error) *costCenterErrorRepository {
	repo := seededCostCenterErrorRepository(costCenterID, nil)
	repo.createAllocationErr = createAllocationErr
	return repo
}

func (m *costCenterErrorRepository) ensureRepo() {
	if m.MockCostCenterRepository == nil {
		m.MockCostCenterRepository = accounting.NewMockCostCenterRepository()
	}
}

func (m *costCenterErrorRepository) GetByID(ctx context.Context, schemaName, tenantID, costCenterID string) (*accounting.CostCenter, error) {
	m.ensureRepo()
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.MockCostCenterRepository.GetByID(ctx, schemaName, tenantID, costCenterID)
}

func (m *costCenterErrorRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.CostCenter, error) {
	m.ensureRepo()
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.MockCostCenterRepository.List(ctx, schemaName, tenantID, activeOnly)
}

func (m *costCenterErrorRepository) Create(ctx context.Context, schemaName string, cc *accounting.CostCenter) error {
	m.ensureRepo()
	if m.createErr != nil {
		return m.createErr
	}
	return m.MockCostCenterRepository.Create(ctx, schemaName, cc)
}

func (m *costCenterErrorRepository) Update(ctx context.Context, schemaName string, cc *accounting.CostCenter) error {
	m.ensureRepo()
	if m.updateErr != nil {
		return m.updateErr
	}
	return m.MockCostCenterRepository.Update(ctx, schemaName, cc)
}

func (m *costCenterErrorRepository) Delete(ctx context.Context, schemaName, tenantID, costCenterID string) error {
	m.ensureRepo()
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return m.MockCostCenterRepository.Delete(ctx, schemaName, tenantID, costCenterID)
}

func (m *costCenterErrorRepository) GetExpensesByPeriod(ctx context.Context, schemaName, tenantID, costCenterID string, start, end time.Time) (decimal.Decimal, error) {
	m.ensureRepo()
	if m.expensesErr != nil {
		return decimal.Zero, m.expensesErr
	}
	return m.MockCostCenterRepository.GetExpensesByPeriod(ctx, schemaName, tenantID, costCenterID, start, end)
}

func (m *costCenterErrorRepository) CreateAllocation(ctx context.Context, schemaName string, allocation *accounting.CostAllocation) error {
	m.ensureRepo()
	if m.createAllocationErr != nil {
		return m.createAllocationErr
	}
	return m.MockCostCenterRepository.CreateAllocation(ctx, schemaName, allocation)
}

func (m *costCenterErrorRepository) ListAllocations(ctx context.Context, schemaName, tenantID string, filters accounting.CostAllocationFilters) ([]accounting.CostAllocation, error) {
	m.ensureRepo()
	if m.listAllocationsErr != nil {
		return nil, m.listAllocationsErr
	}
	return m.MockCostCenterRepository.ListAllocations(ctx, schemaName, tenantID, filters)
}
