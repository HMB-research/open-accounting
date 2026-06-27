package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

const wave5BankAccountID = "44444444-4444-4444-4444-444444444444"

func setupWave5BankingHandlers(t *testing.T) (*Handlers, *mockBankingRepository, *mockTenantRepository) {
	t.Helper()

	h, repo, tenantRepo := setupBankingTestHandlers()
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
	return h, repo, tenantRepo
}

func wave5BankingRequest(method, path string, body any, params map[string]string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "owner@example.com", "tenant-1", tenant.RoleOwner))
	return withURLParams(req, params)
}

func wave5RawRequest(method, path, body string, claims *auth.Claims, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if claims != nil {
		req = req.WithContext(contextWithClaims(req.Context(), claims))
	}
	return withURLParams(req, params)
}

func TestWave5BankAccountHandlerBranches(t *testing.T) {
	t.Run("list bank accounts returns repository error", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)
		repo.listAccErr = errors.New("list unavailable")

		req := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts", nil, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ListBankAccounts(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list bank accounts")
	})

	t.Run("create bank account surfaces service error", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)
		repo.createAccErr = errors.New("bank create failed")

		req := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts", map[string]any{
			"name":           "Operating",
			"account_number": "EE471000001020145685",
			"currency":       "EUR",
		}, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.CreateBankAccount(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "bank create failed")
	})

	t.Run("import bank accounts defaults file name", func(t *testing.T) {
		h, _, _ := setupWave5BankingHandlers(t)

		req := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/import", map[string]any{
			"rows": []map[string]string{{
				"name":           "Defaulted import bank",
				"account_number": "EE111",
				"currency":       "EUR",
			}},
		}, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ImportBankAccounts(rr, req)

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var result banking.ImportBankAccountsResult
		require.NoError(t, decodeJSONResponse(rr.Body, &result))
		assert.Equal(t, "bank_accounts_import.csv", result.FileName)
		assert.Equal(t, 1, result.AccountsImported)
	})

	t.Run("import bank accounts rejects malformed json and service errors", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)

		badJSONReq := wave5RawRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/import", "{invalid", createTestClaims("user-1", "owner@example.com", "tenant-1", tenant.RoleOwner), map[string]string{"tenantID": "tenant-1"})
		badJSONRR := httptest.NewRecorder()
		h.ImportBankAccounts(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		repo.listAccErr = errors.New("preload accounts failed")
		serviceErrReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/import", map[string]any{
			"rows": []map[string]string{{"name": "Operating", "account_number": "EE222"}},
		}, map[string]string{"tenantID": "tenant-1"})
		serviceErrRR := httptest.NewRecorder()
		h.ImportBankAccounts(serviceErrRR, serviceErrReq)
		assert.Equal(t, http.StatusBadRequest, serviceErrRR.Code)
		assert.Contains(t, serviceErrRR.Body.String(), "preload accounts failed")
	})

	t.Run("update bank account rejects malformed json and missing account", func(t *testing.T) {
		h, _, _ := setupWave5BankingHandlers(t)

		badJSONReq := wave5RawRequest(http.MethodPut, "/tenants/tenant-1/bank-accounts/missing", "{invalid", createTestClaims("user-1", "owner@example.com", "tenant-1", tenant.RoleOwner), map[string]string{"tenantID": "tenant-1", "accountID": "missing"})
		badJSONRR := httptest.NewRecorder()
		h.UpdateBankAccount(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		missingReq := wave5BankingRequest(http.MethodPut, "/tenants/tenant-1/bank-accounts/missing", map[string]any{"name": "Updated"}, map[string]string{"tenantID": "tenant-1", "accountID": "missing"})
		missingRR := httptest.NewRecorder()
		h.UpdateBankAccount(missingRR, missingReq)
		assert.Equal(t, http.StatusBadRequest, missingRR.Code)
		assert.Contains(t, missingRR.Body.String(), banking.ErrBankAccountNotFound.Error())
	})
}

func TestWave5BankMatchRuleHandlerBranches(t *testing.T) {
	t.Run("list bank match rules returns repository error", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)
		repo.listRuleErr = errors.New("rules unavailable")

		req := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-match-rules?bank_account_id=%20"+wave5BankAccountID+"&active_only=true&include_global=true", nil, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ListBankMatchRules(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list bank match rules")
	})

	t.Run("create bank match rule rejects malformed json and validation errors", func(t *testing.T) {
		h, _, _ := setupWave5BankingHandlers(t)

		badJSONReq := wave5RawRequest(http.MethodPost, "/tenants/tenant-1/bank-match-rules", "{invalid", createTestClaims("user-1", "owner@example.com", "tenant-1", tenant.RoleOwner), map[string]string{"tenantID": "tenant-1"})
		badJSONRR := httptest.NewRecorder()
		h.CreateBankMatchRule(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		invalidReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-match-rules", map[string]any{
			"name":        "Missing pattern",
			"match_field": "DESCRIPTION",
		}, map[string]string{"tenantID": "tenant-1"})
		invalidRR := httptest.NewRecorder()
		h.CreateBankMatchRule(invalidRR, invalidReq)
		assert.Equal(t, http.StatusBadRequest, invalidRR.Code)
		assert.Contains(t, invalidRR.Body.String(), "pattern is required")
	})

	t.Run("get update and delete bank match rule error branches", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)
		repo.matchRules["rule-1"] = &banking.BankMatchRule{
			ID:              "rule-1",
			TenantID:        "tenant-1",
			Name:            "Existing rule",
			MatchField:      banking.BankMatchFieldDescription,
			Pattern:         "stripe",
			MinConfidence:   0.8,
			MaxDateDiffDays: 5,
			IsActive:        true,
		}

		getReq := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-match-rules/missing", nil, map[string]string{"tenantID": "tenant-1", "ruleID": "missing"})
		getRR := httptest.NewRecorder()
		h.GetBankMatchRule(getRR, getReq)
		assert.Equal(t, http.StatusNotFound, getRR.Code)

		badJSONReq := wave5RawRequest(http.MethodPut, "/tenants/tenant-1/bank-match-rules/rule-1", "{invalid", createTestClaims("user-1", "owner@example.com", "tenant-1", tenant.RoleOwner), map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"})
		badJSONRR := httptest.NewRecorder()
		h.UpdateBankMatchRule(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		invalidUpdateReq := wave5BankingRequest(http.MethodPut, "/tenants/tenant-1/bank-match-rules/rule-1", map[string]any{"min_confidence": 1.5}, map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"})
		invalidUpdateRR := httptest.NewRecorder()
		h.UpdateBankMatchRule(invalidUpdateRR, invalidUpdateReq)
		assert.Equal(t, http.StatusBadRequest, invalidUpdateRR.Code)
		assert.Contains(t, invalidUpdateRR.Body.String(), "min confidence")

		deleteReq := wave5BankingRequest(http.MethodDelete, "/tenants/tenant-1/bank-match-rules/missing", nil, map[string]string{"tenantID": "tenant-1", "ruleID": "missing"})
		deleteRR := httptest.NewRecorder()
		h.DeleteBankMatchRule(deleteRR, deleteReq)
		assert.Equal(t, http.StatusNotFound, deleteRR.Code)
	})
}

func TestWave5BankTransactionAndReconciliationBranches(t *testing.T) {
	t.Run("list bank transactions parses date filters and reports repository errors", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)
		repo.transactions["tx-1"] = &banking.BankTransaction{
			ID:              "tx-1",
			TenantID:        "tenant-1",
			BankAccountID:   "acc-1",
			TransactionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			Status:          banking.StatusUnmatched,
			Amount:          decimal.NewFromInt(10),
		}

		filterReq := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/transactions?status=UNMATCHED&from_date=2026-01-01&to_date=2026-01-31", nil, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		filterRR := httptest.NewRecorder()
		h.ListBankTransactions(filterRR, filterReq)
		require.Equal(t, http.StatusOK, filterRR.Code, filterRR.Body.String())

		invalidDateReq := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/transactions?from_date=bad&to_date=bad", nil, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		invalidDateRR := httptest.NewRecorder()
		h.ListBankTransactions(invalidDateRR, invalidDateReq)
		assert.Equal(t, http.StatusOK, invalidDateRR.Code)

		repo.listTxErr = errors.New("transactions unavailable")
		errorReq := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/transactions", nil, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		errorRR := httptest.NewRecorder()
		h.ListBankTransactions(errorRR, errorReq)
		assert.Equal(t, http.StatusInternalServerError, errorRR.Code)
		assert.Contains(t, errorRR.Body.String(), "Failed to list transactions")
	})

	t.Run("create payment from transaction handles lookup and service errors", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)
		repo.transactions["tx-matched"] = &banking.BankTransaction{
			ID:              "tx-matched",
			TenantID:        "tenant-1",
			BankAccountID:   "acc-1",
			TransactionDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			Status:          banking.StatusMatched,
			Amount:          decimal.NewFromInt(50),
		}

		missingReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/missing/create-payment", nil, map[string]string{"tenantID": "tenant-1", "transactionID": "missing"})
		missingRR := httptest.NewRecorder()
		h.CreatePaymentFromTransaction(missingRR, missingReq)
		assert.Equal(t, http.StatusBadRequest, missingRR.Code)
		assert.Contains(t, missingRR.Body.String(), banking.ErrTransactionNotFound.Error())

		matchedReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-matched/create-payment", nil, map[string]string{"tenantID": "tenant-1", "transactionID": "tx-matched"})
		matchedRR := httptest.NewRecorder()
		h.CreatePaymentFromTransaction(matchedRR, matchedReq)
		assert.Equal(t, http.StatusBadRequest, matchedRR.Code)
		assert.Contains(t, matchedRR.Body.String(), "already matched")
	})

	t.Run("reconciliation handlers report list create and evidence errors", func(t *testing.T) {
		h, repo, _ := setupWave5BankingHandlers(t)

		repo.listRecErr = errors.New("reconciliations unavailable")
		listReq := wave5BankingRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/reconciliations", nil, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		listRR := httptest.NewRecorder()
		h.ListReconciliations(listRR, listReq)
		assert.Equal(t, http.StatusInternalServerError, listRR.Code)
		assert.Contains(t, listRR.Body.String(), "Failed to list reconciliations")
		repo.listRecErr = nil

		createReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/acc-1/reconciliation", map[string]any{
			"statement_date":  "not-a-date",
			"opening_balance": "0",
			"closing_balance": "1",
		}, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
		createRR := httptest.NewRecorder()
		h.CreateReconciliation(createRR, createReq)
		assert.Equal(t, http.StatusBadRequest, createRR.Code)
		assert.Contains(t, createRR.Body.String(), "invalid statement date")

		repo.listTxErr = errors.New("transaction scan failed")
		completeErrorReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/reconciliations/rec-err/complete", nil, map[string]string{"tenantID": "tenant-1", "reconciliationID": "rec-err"})
		completeErrorRR := httptest.NewRecorder()
		h.CompleteReconciliation(completeErrorRR, completeErrorReq)
		assert.Equal(t, http.StatusInternalServerError, completeErrorRR.Code)
		assert.Contains(t, completeErrorRR.Body.String(), "load reconciliation transactions")
		repo.listTxErr = nil

		reconciliationID := "rec-evidence"
		repo.transactions["tx-evidence"] = &banking.BankTransaction{
			ID:               "tx-evidence",
			TenantID:         "tenant-1",
			BankAccountID:    "acc-1",
			Status:           banking.StatusMatched,
			FollowUpStatus:   banking.FollowUpEvidenceRequired,
			ReconciliationID: &reconciliationID,
		}
		completeReq := wave5BankingRequest(http.MethodPost, "/tenants/tenant-1/reconciliations/rec-evidence/complete", nil, map[string]string{"tenantID": "tenant-1", "reconciliationID": reconciliationID})
		completeRR := httptest.NewRecorder()
		h.CompleteReconciliation(completeRR, completeReq)
		assert.Equal(t, http.StatusConflict, completeRR.Code)
		assert.Contains(t, completeRR.Body.String(), "approved reconciliation evidence is required")
	})
}

type wave5TenantRepository struct {
	*mockTenantRepository

	listTenantUsersErr      error
	listInvitationsErr      error
	revokeInvitationErr     error
	removeTenantUserErr     error
	updateTenantUserErr     error
	updateTenantUserRoleErr error
}

func newWave5TenantRepository() *wave5TenantRepository {
	return &wave5TenantRepository{mockTenantRepository: newMockTenantRepository()}
}

func (m *wave5TenantRepository) ListTenantUsers(ctx context.Context, tenantID string) ([]tenant.TenantUser, error) {
	if m.listTenantUsersErr != nil {
		return nil, m.listTenantUsersErr
	}
	return m.mockTenantRepository.ListTenantUsers(ctx, tenantID)
}

func (m *wave5TenantRepository) SetTenantUserActive(ctx context.Context, tenantID, userID string, active bool) error {
	if m.updateTenantUserErr != nil {
		return m.updateTenantUserErr
	}
	return m.mockTenantRepository.SetTenantUserActive(ctx, tenantID, userID, active)
}

func (m *wave5TenantRepository) UpdateTenantUserRole(ctx context.Context, tenantID, userID, role string) error {
	if m.updateTenantUserRoleErr != nil {
		return m.updateTenantUserRoleErr
	}
	return m.mockTenantRepository.UpdateTenantUserRole(ctx, tenantID, userID, role)
}

func (m *wave5TenantRepository) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	if m.removeTenantUserErr != nil {
		return m.removeTenantUserErr
	}
	return m.mockTenantRepository.RemoveTenantUser(ctx, tenantID, userID)
}

func (m *wave5TenantRepository) ListInvitations(ctx context.Context, tenantID string) ([]tenant.UserInvitation, error) {
	if m.listInvitationsErr != nil {
		return nil, m.listInvitationsErr
	}
	return m.mockTenantRepository.ListInvitations(ctx, tenantID)
}

func (m *wave5TenantRepository) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	if m.revokeInvitationErr != nil {
		return m.revokeInvitationErr
	}
	return m.mockTenantRepository.RevokeInvitation(ctx, tenantID, invitationID)
}

func setupWave5TenantHandlers() (*Handlers, *wave5TenantRepository) {
	repo := newWave5TenantRepository()
	h := &Handlers{
		tenantService:         tenant.NewServiceWithRepository(repo),
		tokenService:          auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour),
		refreshSessionService: newMockRefreshSessionService(),
		securityAuditService:  &mockSecurityAuditService{},
	}
	return h, repo
}

func wave5AdminClaims() *auth.Claims {
	return &auth.Claims{UserID: "admin-1", Email: "admin@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}
}

func wave5SeedTenantUsers(repo *wave5TenantRepository) {
	now := time.Now()
	repo.addTestUser("admin-1", "admin@example.com", "Admin User", "password123", true)
	repo.addTestUser("user-2", "target@example.com", "Target User", "password123", true)
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleOwner, IsActive: true, CreatedAt: now},
		{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleViewer, IsActive: true, CreatedAt: now},
	}
}

func TestWave5TenantUserAdminListBranches(t *testing.T) {
	t.Run("list tenant users reports service error", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		repo.listTenantUsersErr = errors.New("users unavailable")

		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ListTenantUsers(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list users")
	})

	t.Run("auth session list covers missing target service and list errors", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)
		h.refreshSessionService = nil

		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/sessions", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		rr := httptest.NewRecorder()
		h.ListTenantUserAuthSessions(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Refresh session service unavailable")

		h.refreshSessionService = newMockRefreshSessionService()
		h.refreshSessionService.(*mockRefreshSessionService).listErr = errors.New("session list failed")
		rr = httptest.NewRecorder()
		h.ListTenantUserAuthSessions(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list refresh sessions")
	})

	t.Run("security audit list covers invalid limit missing service and service error", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)

		invalidLimitReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/security-events?limit=0", nil, wave5AdminClaims())
		invalidLimitReq = withURLParams(invalidLimitReq, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		invalidLimitRR := httptest.NewRecorder()
		h.ListTenantUserSecurityAuditEvents(invalidLimitRR, invalidLimitReq)
		assert.Equal(t, http.StatusBadRequest, invalidLimitRR.Code)
		assert.Contains(t, invalidLimitRR.Body.String(), "Limit must be between 1 and 200")

		h.securityAuditService = nil
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/security-events", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		missingSvcRR := httptest.NewRecorder()
		h.ListTenantUserSecurityAuditEvents(missingSvcRR, req)
		assert.Equal(t, http.StatusInternalServerError, missingSvcRR.Code)
		assert.Contains(t, missingSvcRR.Body.String(), "Security audit service unavailable")

		h.securityAuditService = &mockSecurityAuditService{err: errors.New("audit list failed")}
		serviceErrRR := httptest.NewRecorder()
		h.ListTenantUserSecurityAuditEvents(serviceErrRR, req)
		assert.Equal(t, http.StatusInternalServerError, serviceErrRR.Code)
		assert.Contains(t, serviceErrRR.Body.String(), "Failed to list security audit events")
	})

	t.Run("authorize tenant user admin rejects blank user id", func(t *testing.T) {
		h, _ := setupWave5TenantHandlers()

		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/%20/sessions", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": " "})
		rr := httptest.NewRecorder()

		h.ListTenantUserAuthSessions(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "User id is required")
	})
}

func TestWave5TenantUserStatusAndSessionBranches(t *testing.T) {
	t.Run("update tenant user status rejects malformed and missing body fields", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)

		badJSONReq := wave5RawRequest(http.MethodPut, "/tenants/tenant-1/users/user-2/status", "{invalid", wave5AdminClaims(), map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		badJSONRR := httptest.NewRecorder()
		h.UpdateTenantUserStatus(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		missingFieldReq := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/users/user-2/status", map[string]any{}, wave5AdminClaims())
		missingFieldReq = withURLParams(missingFieldReq, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		missingFieldRR := httptest.NewRecorder()
		h.UpdateTenantUserStatus(missingFieldRR, missingFieldReq)
		assert.Equal(t, http.StatusBadRequest, missingFieldRR.Code)
		assert.Contains(t, missingFieldRR.Body.String(), "is_active is required")
	})

	t.Run("update tenant user status handles missing target refresh and audit failures", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)
		statusReq := func() *http.Request {
			req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/users/user-2/status", map[string]bool{"is_active": false}, wave5AdminClaims())
			return withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		}

		missingReq := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/users/missing/status", map[string]bool{"is_active": false}, wave5AdminClaims())
		missingReq = withURLParams(missingReq, map[string]string{"tenantID": "tenant-1", "userID": "missing"})
		missingRR := httptest.NewRecorder()
		h.UpdateTenantUserStatus(missingRR, missingReq)
		assert.Equal(t, http.StatusNotFound, missingRR.Code)
		assert.Contains(t, missingRR.Body.String(), "User not found in tenant")

		h.refreshSessionService = nil
		noRefreshRR := httptest.NewRecorder()
		h.UpdateTenantUserStatus(noRefreshRR, statusReq())
		assert.Equal(t, http.StatusInternalServerError, noRefreshRR.Code)
		assert.Contains(t, noRefreshRR.Body.String(), "Refresh session service unavailable")

		h.refreshSessionService = &failingRefreshSessionService{mockRefreshSessionService: newMockRefreshSessionService(), revokeAllErr: errors.New("revoke all failed")}
		revokeErrRR := httptest.NewRecorder()
		h.UpdateTenantUserStatus(revokeErrRR, statusReq())
		assert.Equal(t, http.StatusInternalServerError, revokeErrRR.Code)
		assert.Contains(t, revokeErrRR.Body.String(), "Failed to revoke refresh sessions")

		h.refreshSessionService = newMockRefreshSessionService()
		repo.createAuditEventErr = errors.New("audit write failed")
		auditErrRR := httptest.NewRecorder()
		h.UpdateTenantUserStatus(auditErrRR, statusReq())
		assert.Equal(t, http.StatusInternalServerError, auditErrRR.Code)
		assert.Contains(t, auditErrRR.Body.String(), "Failed to record tenant audit event")
	})

	t.Run("revoke one tenant user session covers missing service id not found and generic errors", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)

		h.refreshSessionService = nil
		req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/sessions/session-1", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2", "sessionID": "session-1"})
		rr := httptest.NewRecorder()
		h.RevokeTenantUserAuthSession(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Refresh session service unavailable")

		h.refreshSessionService = newMockRefreshSessionService()
		blankReq := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/sessions/%20", nil, wave5AdminClaims())
		blankReq = withURLParams(blankReq, map[string]string{"tenantID": "tenant-1", "userID": "user-2", "sessionID": " "})
		blankRR := httptest.NewRecorder()
		h.RevokeTenantUserAuthSession(blankRR, blankReq)
		assert.Equal(t, http.StatusBadRequest, blankRR.Code)
		assert.Contains(t, blankRR.Body.String(), "Session id is required")

		notFoundRR := httptest.NewRecorder()
		h.RevokeTenantUserAuthSession(notFoundRR, req)
		assert.Equal(t, http.StatusNotFound, notFoundRR.Code)
		assert.Contains(t, notFoundRR.Body.String(), "Refresh session not found")

		h.refreshSessionService = &failingRefreshSessionService{mockRefreshSessionService: newMockRefreshSessionService(), revokeByIDErr: errors.New("store failed")}
		genericErrRR := httptest.NewRecorder()
		h.RevokeTenantUserAuthSession(genericErrRR, req)
		assert.Equal(t, http.StatusInternalServerError, genericErrRR.Code)
		assert.Contains(t, genericErrRR.Body.String(), "Failed to revoke refresh session")
	})

	t.Run("revoke all tenant user sessions covers service and audit failures", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)

		req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/sessions", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})

		h.refreshSessionService = nil
		noServiceRR := httptest.NewRecorder()
		h.RevokeTenantUserAuthSessions(noServiceRR, req)
		assert.Equal(t, http.StatusInternalServerError, noServiceRR.Code)

		h.refreshSessionService = &failingRefreshSessionService{mockRefreshSessionService: newMockRefreshSessionService(), revokeAllErr: errors.New("store failed")}
		revokeErrRR := httptest.NewRecorder()
		h.RevokeTenantUserAuthSessions(revokeErrRR, req)
		assert.Equal(t, http.StatusInternalServerError, revokeErrRR.Code)
		assert.Contains(t, revokeErrRR.Body.String(), "Failed to revoke refresh sessions")

		h.refreshSessionService = newMockRefreshSessionService()
		repo.createAuditEventErr = errors.New("audit write failed")
		auditErrRR := httptest.NewRecorder()
		h.RevokeTenantUserAuthSessions(auditErrRR, req)
		assert.Equal(t, http.StatusInternalServerError, auditErrRR.Code)
		assert.Contains(t, auditErrRR.Body.String(), "Failed to record tenant audit event")
	})
}

func TestWave5TenantInvitationAndAuditBranches(t *testing.T) {
	t.Run("list tenant audit events reports repository error", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		repo.listAuditEventsErr = errors.New("audit read failed")

		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/audit-events", nil, wave5AdminClaims())
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
		rr := httptest.NewRecorder()

		h.ListTenantAuditEvents(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to list audit events")
	})

	t.Run("remove and update tenant users cover service and audit failures", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		wave5SeedTenantUsers(repo)
		repo.removeTenantUserErr = errors.New("remove failed")

		removeReq := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2", nil, wave5AdminClaims())
		removeReq = withURLParams(removeReq, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		removeRR := httptest.NewRecorder()
		h.RemoveTenantUser(removeRR, removeReq)
		assert.Equal(t, http.StatusBadRequest, removeRR.Code)
		assert.Contains(t, removeRR.Body.String(), "remove failed")
		repo.removeTenantUserErr = nil

		repo.updateTenantUserRoleErr = errors.New("role update failed")
		roleReq := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/users/user-2/role", map[string]string{"role": tenant.RoleAccountant}, wave5AdminClaims())
		roleReq = withURLParams(roleReq, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		roleRR := httptest.NewRecorder()
		h.UpdateTenantUserRole(roleRR, roleReq)
		assert.Equal(t, http.StatusBadRequest, roleRR.Code)
		assert.Contains(t, roleRR.Body.String(), "role update failed")
		repo.updateTenantUserRoleErr = nil

		badJSONReq := wave5RawRequest(http.MethodPut, "/tenants/tenant-1/users/user-2/role", "{invalid", wave5AdminClaims(), map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
		badJSONRR := httptest.NewRecorder()
		h.UpdateTenantUserRole(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		repo.createAuditEventErr = errors.New("audit write failed")
		auditErrRR := httptest.NewRecorder()
		h.RemoveTenantUser(auditErrRR, removeReq)
		assert.Equal(t, http.StatusInternalServerError, auditErrRR.Code)
		assert.Contains(t, auditErrRR.Body.String(), "Failed to record tenant audit event")
	})

	t.Run("invitation handlers cover malformed service list revoke and audit errors", func(t *testing.T) {
		h, repo := setupWave5TenantHandlers()
		repo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

		badJSONReq := wave5RawRequest(http.MethodPost, "/tenants/tenant-1/invitations", "{invalid", wave5AdminClaims(), map[string]string{"tenantID": "tenant-1"})
		badJSONRR := httptest.NewRecorder()
		h.CreateInvitation(badJSONRR, badJSONReq)
		assert.Equal(t, http.StatusBadRequest, badJSONRR.Code)
		assert.Contains(t, badJSONRR.Body.String(), "Invalid request body")

		invalidRoleReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invitations", map[string]string{"email": "new@example.com", "role": tenant.RoleOwner}, wave5AdminClaims())
		invalidRoleReq = withURLParams(invalidRoleReq, map[string]string{"tenantID": "tenant-1"})
		invalidRoleRR := httptest.NewRecorder()
		h.CreateInvitation(invalidRoleRR, invalidRoleReq)
		assert.Equal(t, http.StatusBadRequest, invalidRoleRR.Code)
		assert.Contains(t, invalidRoleRR.Body.String(), "invalid role")

		repo.listInvitationsErr = errors.New("invitation list failed")
		listReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/invitations", nil, wave5AdminClaims())
		listReq = withURLParams(listReq, map[string]string{"tenantID": "tenant-1"})
		listRR := httptest.NewRecorder()
		h.ListInvitations(listRR, listReq)
		assert.Equal(t, http.StatusInternalServerError, listRR.Code)
		assert.Contains(t, listRR.Body.String(), "Failed to list invitations")
		repo.listInvitationsErr = nil

		repo.revokeInvitationErr = errors.New("revoke failed")
		revokeReq := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/invitations/inv-1", nil, wave5AdminClaims())
		revokeReq = withURLParams(revokeReq, map[string]string{"tenantID": "tenant-1", "invitationID": "inv-1"})
		revokeRR := httptest.NewRecorder()
		h.RevokeInvitation(revokeRR, revokeReq)
		assert.Equal(t, http.StatusBadRequest, revokeRR.Code)
		assert.Contains(t, revokeRR.Body.String(), "revoke failed")
		repo.revokeInvitationErr = nil

		repo.createAuditEventErr = errors.New("audit write failed")
		auditErrRR := httptest.NewRecorder()
		h.RevokeInvitation(auditErrRR, revokeReq)
		assert.Equal(t, http.StatusInternalServerError, auditErrRR.Code)
		assert.Contains(t, auditErrRR.Body.String(), "Failed to record tenant audit event")
	})
}

func decodeJSONResponse(body *bytes.Buffer, dest any) error {
	return decodeJSON(httptest.NewRequest(http.MethodPost, "/", body), dest)
}
