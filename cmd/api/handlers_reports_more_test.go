package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestGetAnnualReportValidationBranches(t *testing.T) {
	h, _, _ := setupTenantAccountingHandlers()

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.GetAnnualReport(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "period end date is required")

	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-12-31&cash_flow_method=unsupported", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetAnnualReport(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "cash flow method")

	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/missing/reports/annual?period_end_date=2025-12-31", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "missing"})
	rr = httptest.NewRecorder()
	h.GetAnnualReport(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "Tenant not found")
}

func TestGetAnnualReportAccountingAndCashFlowErrors(t *testing.T) {
	h, tenantRepo, accountingRepo := setupTenantAccountingHandlers()
	reportsRepo := reports.NewMockRepository()
	h.reportsService = reports.NewServiceWithRepository(reportsRepo)

	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-11-30", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.GetAnnualReport(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "fiscal year end")

	accountingRepo.periodBalanceErr = errors.New("trial balance unavailable")
	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-12-31", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetAnnualReport(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	accountingRepo.periodBalanceErr = nil

	accountingRepo.periodBalances = annualReportBalancedPeriodBalances()
	reportsRepo.GetEntriesErr = errors.New("cash flow source down")
	req = makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-12-31&cash_flow_method=indirect", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.GetAnnualReport(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to generate annual report")
}

func TestGetAnnualReportDefaultsCashFlowMethod(t *testing.T) {
	h, tenantRepo, accountingRepo := setupTenantAccountingHandlers()
	reportsRepo := reports.NewMockRepository()
	h.reportsService = reports.NewServiceWithRepository(reportsRepo)

	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	accountingRepo.periodBalances = annualReportBalancedPeriodBalances()
	reportsRepo.JournalEntries = annualReportCashFlowEntries()

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-12-31", nil, &auth.Claims{UserID: "user-1"})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.GetAnnualReport(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var report reports.AnnualReport
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&report))
	require.NotNil(t, report.CashFlowStatement)
	assert.Equal(t, reports.CashFlowMethodDirect, report.CashFlowStatement.Method)
	assert.Equal(t, "2025", report.FiscalYearLabel)
}

func annualReportBalancedPeriodBalances() []accounting.AccountBalance {
	return []accounting.AccountBalance{
		{
			AccountID:    "asset-1",
			AccountCode:  "1000",
			AccountName:  "Bank",
			AccountType:  accounting.AccountTypeAsset,
			DebitBalance: decimal.NewFromInt(600),
			NetBalance:   decimal.NewFromInt(600),
		},
		{
			AccountID:     "equity-1",
			AccountCode:   "3200",
			AccountName:   "Retained Earnings",
			AccountType:   accounting.AccountTypeEquity,
			CreditBalance: decimal.NewFromInt(600),
			NetBalance:    decimal.NewFromInt(-600),
		},
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(-1000),
		},
		{
			AccountID:    "expense-1",
			AccountCode:  "5100",
			AccountName:  "Operating Expenses",
			AccountType:  accounting.AccountTypeExpense,
			DebitBalance: decimal.NewFromInt(400),
			NetBalance:   decimal.NewFromInt(400),
		},
	}
}

func annualReportCashFlowEntries() []reports.JournalEntryWithLines {
	return []reports.JournalEntryWithLines{{
		ID:        "je-1",
		EntryDate: mustParseDateForReportTest("2025-03-01"),
		Lines: []reports.JournalLine{
			{AccountCode: "1000", AccountName: "Bank", AccountType: "ASSET", Debit: decimal.NewFromInt(600)},
			{AccountCode: "4100", AccountName: "Sales Revenue", AccountType: "REVENUE", Credit: decimal.NewFromInt(600)},
		},
	}}
}

func mustParseDateForReportTest(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}
