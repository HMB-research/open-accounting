package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestAccountingHandlers_ListCreateAndGet(t *testing.T) {
	h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	accountingRepo.accounts["acc-1"] = &accounting.Account{
		ID:          "acc-1",
		TenantID:    "tenant-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: accounting.AccountTypeAsset,
		IsActive:    true,
	}
	accountingRepo.accounts["acc-2"] = &accounting.Account{
		ID:          "acc-2",
		TenantID:    "tenant-1",
		Code:        "9999",
		Name:        "Inactive",
		AccountType: accounting.AccountTypeExpense,
		IsActive:    false,
	}

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/accounts?active_only=true", nil), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.ListAccounts(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var accounts []accounting.Account
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &accounts))
	require.Len(t, accounts, 1)
	assert.Equal(t, "acc-1", accounts[0].ID)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/accounts", map[string]interface{}{
		"code":         "2000",
		"name":         "Receivables",
		"account_type": accounting.AccountTypeAsset,
	}, &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateAccount(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	var created accounting.Account
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Equal(t, "2000", created.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/accounts", map[string]interface{}{
		"name":         "Missing Code",
		"account_type": accounting.AccountTypeAsset,
	}, &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateAccount(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/accounts/acc-1", nil), map[string]string{
		"tenantID":  "tenant-1",
		"accountID": "acc-1",
	})
	rr = httptest.NewRecorder()
	h.GetAccount(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/accounts/missing", nil), map[string]string{
		"tenantID":  "tenant-1",
		"accountID": "missing",
	})
	rr = httptest.NewRecorder()
	h.GetAccount(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestJournalEntryHandlers_CreatePostVoidAndGet(t *testing.T) {
	h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	accountingRepo.accounts["cash"] = &accounting.Account{
		ID:          "cash",
		TenantID:    "tenant-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: accounting.AccountTypeAsset,
		IsActive:    true,
	}
	accountingRepo.accounts["sales"] = &accounting.Account{
		ID:          "sales",
		TenantID:    "tenant-1",
		Code:        "3000",
		Name:        "Sales",
		AccountType: accounting.AccountTypeRevenue,
		IsActive:    true,
	}

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries", map[string]interface{}{
		"description": "Bad entry",
		"lines": []map[string]interface{}{
			{"account_id": "cash", "debit_amount": "100"},
		},
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.CreateJournalEntry(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries", map[string]interface{}{
		"entry_date":  "2026-02-10T00:00:00Z",
		"description": "Sales journal",
		"reference":   "REF-1",
		"source_type": "MANUAL",
		"lines": []map[string]interface{}{
			{"account_id": "cash", "debit_amount": "100.00"},
			{"account_id": "sales", "credit_amount": "100.00"},
		},
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.CreateJournalEntry(rr, req)
	assert.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var created accounting.JournalEntry
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Equal(t, accounting.StatusDraft, created.Status)
	require.NotEmpty(t, created.ID)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/journal-entries/"+created.ID, nil), map[string]string{
		"tenantID": "tenant-1",
		"entryID":  created.ID,
	})
	rr = httptest.NewRecorder()
	h.GetJournalEntry(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/"+created.ID+"/post", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": created.ID})
	rr = httptest.NewRecorder()
	h.PostJournalEntry(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/"+created.ID+"/void", map[string]string{
		"reason": "reverse entry",
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": created.ID})
	rr = httptest.NewRecorder()
	h.VoidJournalEntry(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/"+created.ID+"/void", map[string]string{}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "entryID": created.ID})
	rr = httptest.NewRecorder()
	h.VoidJournalEntry(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestReportHandlers(t *testing.T) {
	h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	accountingRepo.accountBalance = decimal.NewFromInt(275)
	accountingRepo.trialBalances = []accounting.AccountBalance{
		{
			AccountID:     "cash",
			AccountCode:   "1000",
			AccountName:   "Cash",
			AccountType:   accounting.AccountTypeAsset,
			DebitBalance:  decimal.NewFromInt(1000),
			CreditBalance: decimal.Zero,
			NetBalance:    decimal.NewFromInt(1000),
		},
		{
			AccountID:     "equity",
			AccountCode:   "3000",
			AccountName:   "Equity",
			AccountType:   accounting.AccountTypeEquity,
			DebitBalance:  decimal.Zero,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(-1000),
		},
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:   "sales",
			AccountCode: "4000",
			AccountName: "Sales",
			AccountType: accounting.AccountTypeRevenue,
			NetBalance:  decimal.NewFromInt(-500),
		},
		{
			AccountID:   "expense",
			AccountCode: "5000",
			AccountName: "Expense",
			AccountType: accounting.AccountTypeExpense,
			NetBalance:  decimal.NewFromInt(200),
		},
	}

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/trial-balance?as_of_date=2026-02-28", nil), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.GetTrialBalance(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/trial-balance?as_of_date=bad-date", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetTrialBalance(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/account-balance/cash?as_of_date=2026-02-28", nil), map[string]string{
		"tenantID":  "tenant-1",
		"accountID": "cash",
	})
	rr = httptest.NewRecorder()
	h.GetAccountBalance(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/account-balance/cash?as_of_date=bad-date", nil), map[string]string{
		"tenantID":  "tenant-1",
		"accountID": "cash",
	})
	rr = httptest.NewRecorder()
	h.GetAccountBalance(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-sheet?as_of=2026-02-28", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetBalanceSheet(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/balance-sheet?as_of=not-a-date", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetBalanceSheet(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/income-statement?start=2026-01-01&end=2026-01-31", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetIncomeStatement(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/income-statement", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetIncomeStatement(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reports/income-statement?start=2026-01-31&end=2026-01-01", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetIncomeStatement(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
