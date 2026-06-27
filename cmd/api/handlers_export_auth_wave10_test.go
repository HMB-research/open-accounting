package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payroll"
	internalpdf "github.com/HMB-research/open-accounting/internal/pdf"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

var errWave10 = errors.New("wave10 forced error")

func wave10AccountBalance(accountType accounting.AccountType) accounting.AccountBalance {
	return accounting.AccountBalance{
		AccountID:     "account-" + strings.ToLower(string(accountType)),
		AccountCode:   "1000",
		AccountName:   "Cash",
		AccountType:   accountType,
		DebitBalance:  decimal.NewFromInt(100),
		CreditBalance: decimal.Zero,
		NetBalance:    decimal.NewFromInt(100),
	}
}

func wave10TrialBalance() *accounting.TrialBalance {
	return &accounting.TrialBalance{
		TenantID:     "tenant-1",
		AsOfDate:     time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Accounts:     []accounting.AccountBalance{wave10AccountBalance(accounting.AccountTypeAsset)},
		TotalDebits:  decimal.NewFromInt(100),
		TotalCredits: decimal.NewFromInt(100),
		IsBalanced:   true,
	}
}

func wave10BalanceSheet() *accounting.BalanceSheet {
	return &accounting.BalanceSheet{
		TenantID:         "tenant-1",
		AsOfDate:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Assets:           []accounting.AccountBalance{wave10AccountBalance(accounting.AccountTypeAsset)},
		Liabilities:      []accounting.AccountBalance{wave10AccountBalance(accounting.AccountTypeLiability)},
		Equity:           []accounting.AccountBalance{wave10AccountBalance(accounting.AccountTypeEquity)},
		TotalAssets:      decimal.NewFromInt(100),
		TotalLiabilities: decimal.NewFromInt(40),
		RetainedEarnings: decimal.NewFromInt(10),
		TotalEquity:      decimal.NewFromInt(50),
	}
}

func wave10IncomeStatement() *accounting.IncomeStatement {
	return &accounting.IncomeStatement{
		TenantID:      "tenant-1",
		StartDate:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Revenue:       []accounting.AccountBalance{wave10AccountBalance(accounting.AccountTypeRevenue)},
		Expenses:      []accounting.AccountBalance{wave10AccountBalance(accounting.AccountTypeExpense)},
		TotalRevenue:  decimal.NewFromInt(100),
		TotalExpenses: decimal.NewFromInt(60),
		NetIncome:     decimal.NewFromInt(40),
	}
}

func wave10CashFlowStatement() *reports.CashFlowStatement {
	return &reports.CashFlowStatement{
		TenantID:       "tenant-1",
		StartDate:      "2026-01-01",
		EndDate:        "2026-01-31",
		Method:         reports.CashFlowMethodDirect,
		OpeningCash:    decimal.NewFromInt(100),
		NetCashChange:  decimal.NewFromInt(10),
		ClosingCash:    decimal.NewFromInt(110),
		TotalOperating: decimal.NewFromInt(10),
		OperatingActivities: []reports.CashFlowItem{{
			Code:        reports.CFOperReceipts,
			Description: "Cash receipts",
			Amount:      decimal.NewFromInt(10),
		}},
	}
}

func wave10FailCSVWriteAt(t *testing.T, failAt int) {
	t.Helper()
	original := reportCSVWriteRecord
	calls := 0
	reportCSVWriteRecord = func(writer *csv.Writer, row []string) error {
		calls++
		if calls == failAt {
			return errWave10
		}
		return original(writer, row)
	}
	t.Cleanup(func() {
		reportCSVWriteRecord = original
	})
}

func wave10FailCSVFlush(t *testing.T) {
	t.Helper()
	original := reportCSVFlush
	reportCSVFlush = func(*csv.Writer) error {
		return errWave10
	}
	t.Cleanup(func() {
		reportCSVFlush = original
	})
}

func TestWave10TokenGenerationSeams(t *testing.T) {
	t.Run("login maps access token generation errors", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User", "secret", true)
		original := generateAccessToken
		generateAccessToken = func(*auth.TokenService, string, string, string, string) (string, error) {
			return "", errWave10
		}
		t.Cleanup(func() {
			generateAccessToken = original
		})

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/login", map[string]string{
			"email":    "user@example.com",
			"password": "secret",
		}, nil)
		rr := httptest.NewRecorder()
		h.Login(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate token")
	})

	t.Run("login maps refresh token generation errors", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User", "secret", true)
		original := generateRefreshToken
		generateRefreshToken = func(*auth.TokenService, string) (string, error) {
			return "", errWave10
		}
		t.Cleanup(func() {
			generateRefreshToken = original
		})

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/login", map[string]string{
			"email":    "user@example.com",
			"password": "secret",
		}, nil)
		rr := httptest.NewRecorder()
		h.Login(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate refresh token")
	})

	t.Run("refresh maps access and refresh token generation errors", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User", "secret", true)
		refreshToken := createMockRefreshSession(t, h, "user-1")

		originalAccess := generateAccessToken
		generateAccessToken = func(*auth.TokenService, string, string, string, string) (string, error) {
			return "", errWave10
		}
		req := makeAuthenticatedRequest(http.MethodPost, "/auth/refresh", map[string]string{"refresh_token": refreshToken}, nil)
		rr := httptest.NewRecorder()
		h.RefreshToken(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate token")
		generateAccessToken = originalAccess

		h, repo = setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User", "secret", true)
		refreshToken = createMockRefreshSession(t, h, "user-1")
		originalRefresh := generateRefreshToken
		generateRefreshToken = func(*auth.TokenService, string) (string, error) {
			return "", errWave10
		}
		t.Cleanup(func() {
			generateAccessToken = originalAccess
			generateRefreshToken = originalRefresh
		})

		req = makeAuthenticatedRequest(http.MethodPost, "/auth/refresh", map[string]string{"refresh_token": refreshToken}, nil)
		rr = httptest.NewRecorder()
		h.RefreshToken(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate refresh token")
	})

	t.Run("helper maps refresh claim validation errors", func(t *testing.T) {
		h, _ := setupAuthTestHandlers()
		original := validateRefreshTokenClaims
		validateRefreshTokenClaims = func(*auth.TokenService, string) (*auth.RefreshClaims, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			validateRefreshTokenClaims = original
		})

		token, claims, err := h.generateRefreshTokenWithClaims("user-1")
		require.ErrorIs(t, err, errWave10)
		assert.Empty(t, token)
		assert.Nil(t, claims)
	})
}

func TestWave10ReportCSVWriteAndFlushSeams(t *testing.T) {
	writeCases := []struct {
		name   string
		failAt int
		call   func() ([]byte, error)
	}{
		{"trial balance header", 1, func() ([]byte, error) { return trialBalanceCSV(wave10TrialBalance()) }},
		{"trial balance account row", 2, func() ([]byte, error) { return trialBalanceCSV(wave10TrialBalance()) }},
		{"trial balance total row", 2, func() ([]byte, error) {
			report := wave10TrialBalance()
			report.Accounts = nil
			return trialBalanceCSV(report)
		}},
		{"balance sheet header", 1, func() ([]byte, error) { return balanceSheetCSV(wave10BalanceSheet()) }},
		{"balance sheet assets", 2, func() ([]byte, error) { return balanceSheetCSV(wave10BalanceSheet()) }},
		{"balance sheet liabilities", 2, func() ([]byte, error) {
			report := wave10BalanceSheet()
			report.Assets = nil
			return balanceSheetCSV(report)
		}},
		{"balance sheet equity", 2, func() ([]byte, error) {
			report := wave10BalanceSheet()
			report.Assets = nil
			report.Liabilities = nil
			return balanceSheetCSV(report)
		}},
		{"balance sheet totals", 2, func() ([]byte, error) {
			report := wave10BalanceSheet()
			report.Assets = nil
			report.Liabilities = nil
			report.Equity = nil
			return balanceSheetCSV(report)
		}},
		{"income statement header", 1, func() ([]byte, error) { return incomeStatementCSV(wave10IncomeStatement()) }},
		{"income statement revenue", 2, func() ([]byte, error) { return incomeStatementCSV(wave10IncomeStatement()) }},
		{"income statement expenses", 2, func() ([]byte, error) {
			report := wave10IncomeStatement()
			report.Revenue = nil
			return incomeStatementCSV(report)
		}},
		{"income statement totals", 2, func() ([]byte, error) {
			report := wave10IncomeStatement()
			report.Revenue = nil
			report.Expenses = nil
			return incomeStatementCSV(report)
		}},
		{"cash flow header", 1, func() ([]byte, error) { return cashFlowStatementCSV(wave10CashFlowStatement()) }},
		{"cash flow operating", 2, func() ([]byte, error) { return cashFlowStatementCSV(wave10CashFlowStatement()) }},
		{"cash flow investing", 2, func() ([]byte, error) {
			report := wave10CashFlowStatement()
			report.OperatingActivities = nil
			report.InvestingActivities = []reports.CashFlowItem{{Code: "I", Amount: decimal.NewFromInt(1)}}
			return cashFlowStatementCSV(report)
		}},
		{"cash flow financing", 2, func() ([]byte, error) {
			report := wave10CashFlowStatement()
			report.OperatingActivities = nil
			report.FinancingActivities = []reports.CashFlowItem{{Code: "F", Amount: decimal.NewFromInt(1)}}
			return cashFlowStatementCSV(report)
		}},
		{"cash flow summary", 2, func() ([]byte, error) {
			report := wave10CashFlowStatement()
			report.OperatingActivities = nil
			return cashFlowStatementCSV(report)
		}},
		{"account balance header", 1, func() ([]byte, error) { return accountBalanceCSV("account-1", "2026-01-31", "100") }},
		{"account balance value", 2, func() ([]byte, error) { return accountBalanceCSV("account-1", "2026-01-31", "100") }},
		{"rows to csv row", 1, func() ([]byte, error) { return rowsToCSV([][]string{{"a"}}) }},
	}

	for _, tt := range writeCases {
		t.Run(tt.name, func(t *testing.T) {
			wave10FailCSVWriteAt(t, tt.failAt)
			_, err := tt.call()
			require.ErrorIs(t, err, errWave10)
		})
	}

	flushCases := []struct {
		name string
		call func() ([]byte, error)
	}{
		{"trial balance", func() ([]byte, error) { return trialBalanceCSV(wave10TrialBalance()) }},
		{"balance sheet", func() ([]byte, error) { return balanceSheetCSV(wave10BalanceSheet()) }},
		{"income statement", func() ([]byte, error) { return incomeStatementCSV(wave10IncomeStatement()) }},
		{"cash flow", func() ([]byte, error) { return cashFlowStatementCSV(wave10CashFlowStatement()) }},
		{"account balance", func() ([]byte, error) { return accountBalanceCSV("account-1", "2026-01-31", "100") }},
		{"rows to csv", func() ([]byte, error) { return rowsToCSV([][]string{{"a"}}) }},
	}

	for _, tt := range flushCases {
		t.Run(tt.name+" flush", func(t *testing.T) {
			wave10FailCSVFlush(t)
			_, err := tt.call()
			require.ErrorIs(t, err, errWave10)
		})
	}
}

func TestWave10ReportXLSXAndPDFHelperSeams(t *testing.T) {
	t.Run("xlsx source csv errors", func(t *testing.T) {
		cases := []struct {
			name  string
			setup func(*testing.T)
			call  func() ([]byte, error)
		}{
			{"trial", func(t *testing.T) {
				original := exportTrialBalanceCSV
				exportTrialBalanceCSV = func(*accounting.TrialBalance) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportTrialBalanceCSV = original })
			}, func() ([]byte, error) { return trialBalanceXLSX(wave10TrialBalance()) }},
			{"balance", func(t *testing.T) {
				original := exportBalanceSheetCSV
				exportBalanceSheetCSV = func(*accounting.BalanceSheet) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportBalanceSheetCSV = original })
			}, func() ([]byte, error) { return balanceSheetXLSX(wave10BalanceSheet()) }},
			{"income", func(t *testing.T) {
				original := exportIncomeStatementCSV
				exportIncomeStatementCSV = func(*accounting.IncomeStatement) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportIncomeStatementCSV = original })
			}, func() ([]byte, error) { return incomeStatementXLSX(wave10IncomeStatement()) }},
			{"cash flow", func(t *testing.T) {
				original := exportCashFlowStatementCSV
				exportCashFlowStatementCSV = func(*reports.CashFlowStatement) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportCashFlowStatementCSV = original })
			}, func() ([]byte, error) { return cashFlowStatementXLSX(wave10CashFlowStatement()) }},
			{"account", func(t *testing.T) {
				original := exportAccountBalanceCSV
				exportAccountBalanceCSV = func(string, string, string) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportAccountBalanceCSV = original })
			}, func() ([]byte, error) { return accountBalanceXLSX("account-1", "2026-01-31", "100") }},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				tt.setup(t)
				_, err := tt.call()
				require.ErrorIs(t, err, errWave10)
			})
		}
	})

	t.Run("xlsx zip writer errors", func(t *testing.T) {
		_, err := reportCSVBytesToXLSX("Bad CSV", []byte("\"unterminated"))
		require.Error(t, err)

		originalWrite := reportXLSXWritePart
		reportXLSXWritePart = func(*zip.Writer, string, string) error { return errWave10 }
		_, err = reportRowsXLSX("Report", [][]string{{"header"}})
		require.ErrorIs(t, err, errWave10)
		reportXLSXWritePart = originalWrite

		originalClose := reportXLSXClose
		reportXLSXClose = func(*zip.Writer) error { return errWave10 }
		t.Cleanup(func() {
			reportXLSXWritePart = originalWrite
			reportXLSXClose = originalClose
		})
		_, err = reportRowsXLSX("Report", [][]string{{"header"}})
		require.ErrorIs(t, err, errWave10)

		originalCreate := reportXLSXCreatePart
		reportXLSXCreatePart = func(*zip.Writer, string) (io.Writer, error) {
			return nil, errWave10
		}
		err = writeXLSXPart(zip.NewWriter(&bytes.Buffer{}), "broken.xml", "content")
		require.ErrorIs(t, err, errWave10)
		reportXLSXCreatePart = originalCreate

		originalWriteContent := reportXLSXWritePartContent
		reportXLSXWritePartContent = func(io.Writer, string) (int, error) {
			return 0, errWave10
		}
		t.Cleanup(func() {
			reportXLSXCreatePart = originalCreate
			reportXLSXWritePartContent = originalWriteContent
		})
		err = writeXLSXPart(zip.NewWriter(&bytes.Buffer{}), "broken.xml", "content")
		require.ErrorIs(t, err, errWave10)
	})

	t.Run("pdf source csv and generate errors", func(t *testing.T) {
		cases := []struct {
			name  string
			setup func(*testing.T)
			call  func() ([]byte, error)
		}{
			{"trial", func(t *testing.T) {
				original := exportTrialBalanceCSV
				exportTrialBalanceCSV = func(*accounting.TrialBalance) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportTrialBalanceCSV = original })
			}, func() ([]byte, error) { return trialBalancePDF(wave10TrialBalance(), time.Now()) }},
			{"balance", func(t *testing.T) {
				original := exportBalanceSheetCSV
				exportBalanceSheetCSV = func(*accounting.BalanceSheet) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportBalanceSheetCSV = original })
			}, func() ([]byte, error) { return balanceSheetPDF(wave10BalanceSheet(), time.Now()) }},
			{"income", func(t *testing.T) {
				original := exportIncomeStatementCSV
				exportIncomeStatementCSV = func(*accounting.IncomeStatement) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportIncomeStatementCSV = original })
			}, func() ([]byte, error) { return incomeStatementPDF(wave10IncomeStatement(), time.Now(), time.Now()) }},
			{"cash flow", func(t *testing.T) {
				original := exportCashFlowStatementCSV
				exportCashFlowStatementCSV = func(*reports.CashFlowStatement) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportCashFlowStatementCSV = original })
			}, func() ([]byte, error) { return cashFlowStatementPDF(wave10CashFlowStatement()) }},
			{"account", func(t *testing.T) {
				original := exportAccountBalanceCSV
				exportAccountBalanceCSV = func(string, string, string) ([]byte, error) { return nil, errWave10 }
				t.Cleanup(func() { exportAccountBalanceCSV = original })
			}, func() ([]byte, error) { return accountBalancePDF("account-1", "2026-01-31", "100") }},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				tt.setup(t)
				_, err := tt.call()
				require.ErrorIs(t, err, errWave10)
			})
		}

		_, err := reportCSVBytesToPDF("Bad CSV", "", []byte("\"unterminated"))
		require.Error(t, err)

		originalGenerate := reportPDFGenerate
		reportPDFGenerate = func(core.Maroto) (core.Document, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			reportPDFGenerate = originalGenerate
		})
		_, err = reportRowsPDF("Report", "", [][]string{{"header"}})
		require.ErrorIs(t, err, errWave10)
	})
}

func wave10AccountingHandlers() (*Handlers, *mockAccountingRepository) {
	h, tenantRepo, repo := setupAccountingTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	repo.accountBalance = decimal.NewFromInt(100)
	repo.trialBalances = []accounting.AccountBalance{
		wave10AccountBalance(accounting.AccountTypeAsset),
		wave10AccountBalance(accounting.AccountTypeEquity),
	}
	repo.periodBalances = []accounting.AccountBalance{
		wave10AccountBalance(accounting.AccountTypeRevenue),
		wave10AccountBalance(accounting.AccountTypeExpense),
	}
	return h, repo
}

func TestWave10CoreReportHandlerExportErrors(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		params map[string]string
		setup  func(*testing.T)
		call   func(*Handlers, http.ResponseWriter, *http.Request)
		want   string
	}{
		{"trial csv", "/tenants/tenant-1/reports/trial-balance?as_of_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportTrialBalanceCSV
			exportTrialBalanceCSV = func(*accounting.TrialBalance) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportTrialBalanceCSV = original })
		}, (*Handlers).GetTrialBalance, "Failed to export trial balance CSV"},
		{"trial xlsx", "/tenants/tenant-1/reports/trial-balance?as_of_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportTrialBalanceXLSX
			exportTrialBalanceXLSX = func(*accounting.TrialBalance) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportTrialBalanceXLSX = original })
		}, (*Handlers).GetTrialBalance, "Failed to export trial balance XLSX"},
		{"trial pdf", "/tenants/tenant-1/reports/trial-balance?as_of_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportTrialBalancePDF
			exportTrialBalancePDF = func(*accounting.TrialBalance, time.Time) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportTrialBalancePDF = original })
		}, (*Handlers).GetTrialBalance, "Failed to export trial balance PDF"},
		{"account csv", "/tenants/tenant-1/reports/account-balance/account-1?as_of_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1", "accountID": "account-1"}, func(t *testing.T) {
			original := exportAccountBalanceCSV
			exportAccountBalanceCSV = func(string, string, string) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAccountBalanceCSV = original })
		}, (*Handlers).GetAccountBalance, "Failed to export account balance CSV"},
		{"account xlsx", "/tenants/tenant-1/reports/account-balance/account-1?as_of_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1", "accountID": "account-1"}, func(t *testing.T) {
			original := exportAccountBalanceXLSX
			exportAccountBalanceXLSX = func(string, string, string) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAccountBalanceXLSX = original })
		}, (*Handlers).GetAccountBalance, "Failed to export account balance XLSX"},
		{"account pdf", "/tenants/tenant-1/reports/account-balance/account-1?as_of_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1", "accountID": "account-1"}, func(t *testing.T) {
			original := exportAccountBalancePDF
			exportAccountBalancePDF = func(string, string, string) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAccountBalancePDF = original })
		}, (*Handlers).GetAccountBalance, "Failed to export account balance PDF"},
		{"balance csv", "/tenants/tenant-1/reports/balance-sheet?as_of=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportBalanceSheetCSV
			exportBalanceSheetCSV = func(*accounting.BalanceSheet) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceSheetCSV = original })
		}, (*Handlers).GetBalanceSheet, "Failed to export balance sheet CSV"},
		{"balance xlsx", "/tenants/tenant-1/reports/balance-sheet?as_of=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportBalanceSheetXLSX
			exportBalanceSheetXLSX = func(*accounting.BalanceSheet) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceSheetXLSX = original })
		}, (*Handlers).GetBalanceSheet, "Failed to export balance sheet XLSX"},
		{"balance pdf", "/tenants/tenant-1/reports/balance-sheet?as_of=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportBalanceSheetPDF
			exportBalanceSheetPDF = func(*accounting.BalanceSheet, time.Time) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceSheetPDF = original })
		}, (*Handlers).GetBalanceSheet, "Failed to export balance sheet PDF"},
		{"income csv", "/tenants/tenant-1/reports/income-statement?start=2026-01-01&end=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportIncomeStatementCSV
			exportIncomeStatementCSV = func(*accounting.IncomeStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportIncomeStatementCSV = original })
		}, (*Handlers).GetIncomeStatement, "Failed to export income statement CSV"},
		{"income xlsx", "/tenants/tenant-1/reports/income-statement?start=2026-01-01&end=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportIncomeStatementXLSX
			exportIncomeStatementXLSX = func(*accounting.IncomeStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportIncomeStatementXLSX = original })
		}, (*Handlers).GetIncomeStatement, "Failed to export income statement XLSX"},
		{"income pdf", "/tenants/tenant-1/reports/income-statement?start=2026-01-01&end=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportIncomeStatementPDF
			exportIncomeStatementPDF = func(*accounting.IncomeStatement, time.Time, time.Time) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportIncomeStatementPDF = original })
		}, (*Handlers).GetIncomeStatement, "Failed to export income statement PDF"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := wave10AccountingHandlers()
			tt.setup(t)
			req := withURLParams(httptest.NewRequest(http.MethodGet, tt.path, nil), tt.params)
			rr := httptest.NewRecorder()
			tt.call(h, rr, req)

			require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), tt.want)
		})
	}
}

func wave10MiscReportHandlers() (*Handlers, *reports.MockRepository, *accounting.MockCostCenterRepository) {
	h, tenantRepo, reportsRepo, _, costCenterRepo, _ := setupMiscHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	reportsRepo.CashBalance = decimal.NewFromInt(100)
	reportsRepo.Contact = reports.ContactInfo{ID: "contact-1", Name: "Customer"}
	costCenterRepo.CostCenters["cost-center-1"] = &accounting.CostCenter{
		ID:       "cost-center-1",
		TenantID: "tenant-1",
		Code:     "OPS",
		Name:     "Operations",
		IsActive: true,
	}
	return h, reportsRepo, costCenterRepo
}

func TestWave10ExtendedReportHandlerExportErrors(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		params map[string]string
		setup  func(*testing.T)
		call   func(*Handlers, http.ResponseWriter, *http.Request)
		want   string
	}{
		{"cash csv", "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportCashFlowStatementCSV
			exportCashFlowStatementCSV = func(*reports.CashFlowStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportCashFlowStatementCSV = original })
		}, (*Handlers).GetCashFlowStatement, "Failed to export cash flow CSV"},
		{"cash xlsx", "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportCashFlowStatementXLSX
			exportCashFlowStatementXLSX = func(*reports.CashFlowStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportCashFlowStatementXLSX = original })
		}, (*Handlers).GetCashFlowStatement, "Failed to export cash flow XLSX"},
		{"cash pdf", "/tenants/tenant-1/reports/cash-flow?start_date=2026-01-01&end_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportCashFlowStatementPDF
			exportCashFlowStatementPDF = func(*reports.CashFlowStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportCashFlowStatementPDF = original })
		}, (*Handlers).GetCashFlowStatement, "Failed to export cash flow PDF"},
		{"balance confirmations csv", "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE&as_of_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportBalanceConfirmationSummaryCSV
			exportBalanceConfirmationSummaryCSV = func(*reports.BalanceConfirmationSummary) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceConfirmationSummaryCSV = original })
		}, (*Handlers).GetBalanceConfirmationSummary, "Failed to export balance confirmations CSV"},
		{"balance confirmations xlsx", "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE&as_of_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportBalanceConfirmationSummaryXLSX
			exportBalanceConfirmationSummaryXLSX = func(*reports.BalanceConfirmationSummary) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceConfirmationSummaryXLSX = original })
		}, (*Handlers).GetBalanceConfirmationSummary, "Failed to export balance confirmations XLSX"},
		{"balance confirmations pdf", "/tenants/tenant-1/reports/balance-confirmations?type=RECEIVABLE&as_of_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportBalanceConfirmationSummaryPDF
			exportBalanceConfirmationSummaryPDF = func(*reports.BalanceConfirmationSummary) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceConfirmationSummaryPDF = original })
		}, (*Handlers).GetBalanceConfirmationSummary, "Failed to export balance confirmations PDF"},
		{"balance confirmation csv", "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"}, func(t *testing.T) {
			original := exportBalanceConfirmationCSV
			exportBalanceConfirmationCSV = func(*reports.BalanceConfirmation) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceConfirmationCSV = original })
		}, (*Handlers).GetBalanceConfirmation, "Failed to export balance confirmation CSV"},
		{"balance confirmation xlsx", "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"}, func(t *testing.T) {
			original := exportBalanceConfirmationXLSX
			exportBalanceConfirmationXLSX = func(*reports.BalanceConfirmation) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceConfirmationXLSX = original })
		}, (*Handlers).GetBalanceConfirmation, "Failed to export balance confirmation XLSX"},
		{"balance confirmation pdf", "/tenants/tenant-1/reports/balance-confirmations/contact-1?type=RECEIVABLE&as_of_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"}, func(t *testing.T) {
			original := exportBalanceConfirmationPDF
			exportBalanceConfirmationPDF = func(*reports.BalanceConfirmation) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportBalanceConfirmationPDF = original })
		}, (*Handlers).GetBalanceConfirmation, "Failed to export balance confirmation PDF"},
		{"contact statement csv", "/tenants/tenant-1/reports/contact-statements/contact-1?type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"}, func(t *testing.T) {
			original := exportContactStatementCSV
			exportContactStatementCSV = func(*reports.ContactStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportContactStatementCSV = original })
		}, (*Handlers).GetContactStatement, "Failed to export contact statement CSV"},
		{"contact statement xlsx", "/tenants/tenant-1/reports/contact-statements/contact-1?type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"}, func(t *testing.T) {
			original := exportContactStatementXLSX
			exportContactStatementXLSX = func(*reports.ContactStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportContactStatementXLSX = original })
		}, (*Handlers).GetContactStatement, "Failed to export contact statement XLSX"},
		{"contact statement pdf", "/tenants/tenant-1/reports/contact-statements/contact-1?type=RECEIVABLE&start_date=2026-01-01&end_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"}, func(t *testing.T) {
			original := exportContactStatementPDF
			exportContactStatementPDF = func(*reports.ContactStatement) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportContactStatementPDF = original })
		}, (*Handlers).GetContactStatement, "Failed to export contact statement PDF"},
		{"sales margin csv", "/tenants/tenant-1/reports/sales-margin?start_date=2026-01-01&end_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportSalesMarginCSV
			exportSalesMarginCSV = func(*reports.SalesMarginReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportSalesMarginCSV = original })
		}, (*Handlers).GetSalesMarginReport, "Failed to export sales margin CSV"},
		{"sales margin xlsx", "/tenants/tenant-1/reports/sales-margin?start_date=2026-01-01&end_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportSalesMarginXLSX
			exportSalesMarginXLSX = func(*reports.SalesMarginReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportSalesMarginXLSX = original })
		}, (*Handlers).GetSalesMarginReport, "Failed to export sales margin XLSX"},
		{"sales margin pdf", "/tenants/tenant-1/reports/sales-margin?start_date=2026-01-01&end_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportSalesMarginPDF
			exportSalesMarginPDF = func(*reports.SalesMarginReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportSalesMarginPDF = original })
		}, (*Handlers).GetSalesMarginReport, "Failed to export sales margin PDF"},
		{"cost center csv", "/tenants/tenant-1/cost-centers/report?start_date=2026-01-01&end_date=2026-01-31&format=csv", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportCostCenterReportCSV
			exportCostCenterReportCSV = func(*accounting.CostCenterReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportCostCenterReportCSV = original })
		}, (*Handlers).GetCostCenterReport, "Failed to export cost center report CSV"},
		{"cost center xlsx", "/tenants/tenant-1/cost-centers/report?start_date=2026-01-01&end_date=2026-01-31&format=xlsx", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportCostCenterReportXLSX
			exportCostCenterReportXLSX = func(*accounting.CostCenterReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportCostCenterReportXLSX = original })
		}, (*Handlers).GetCostCenterReport, "Failed to export cost center report XLSX"},
		{"cost center pdf", "/tenants/tenant-1/cost-centers/report?start_date=2026-01-01&end_date=2026-01-31&format=pdf", map[string]string{"tenantID": "tenant-1"}, func(t *testing.T) {
			original := exportCostCenterReportPDF
			exportCostCenterReportPDF = func(*accounting.CostCenterReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportCostCenterReportPDF = original })
		}, (*Handlers).GetCostCenterReport, "Failed to export cost center report PDF"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := wave10MiscReportHandlers()
			tt.setup(t)
			req := withURLParams(httptest.NewRequest(http.MethodGet, tt.path, nil), tt.params)
			rr := httptest.NewRecorder()
			tt.call(h, rr, req)

			require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), tt.want)
		})
	}
}

func TestWave10AgingReportHandlerExportErrors(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		call  func(*Handlers, http.ResponseWriter, *http.Request)
		setup func(*testing.T)
		want  string
	}{
		{"receivables csv", "/tenants/tenant-1/reports/aging/receivables?format=csv", (*Handlers).GetReceivablesAging, func(t *testing.T) {
			original := exportAgingReportCSV
			exportAgingReportCSV = func(*analytics.AgingReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAgingReportCSV = original })
		}, "Failed to export aging CSV"},
		{"receivables xlsx", "/tenants/tenant-1/reports/aging/receivables?format=xlsx", (*Handlers).GetReceivablesAging, func(t *testing.T) {
			original := exportAgingReportXLSX
			exportAgingReportXLSX = func(*analytics.AgingReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAgingReportXLSX = original })
		}, "Failed to export aging XLSX"},
		{"receivables pdf", "/tenants/tenant-1/reports/aging/receivables?format=pdf", (*Handlers).GetReceivablesAging, func(t *testing.T) {
			original := exportAgingReportPDF
			exportAgingReportPDF = func(*analytics.AgingReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAgingReportPDF = original })
		}, "Failed to export aging PDF"},
		{"payables csv", "/tenants/tenant-1/reports/aging/payables?format=csv", (*Handlers).GetPayablesAging, func(t *testing.T) {
			original := exportAgingReportCSV
			exportAgingReportCSV = func(*analytics.AgingReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAgingReportCSV = original })
		}, "Failed to export aging CSV"},
		{"payables xlsx", "/tenants/tenant-1/reports/aging/payables?format=xlsx", (*Handlers).GetPayablesAging, func(t *testing.T) {
			original := exportAgingReportXLSX
			exportAgingReportXLSX = func(*analytics.AgingReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAgingReportXLSX = original })
		}, "Failed to export aging XLSX"},
		{"payables pdf", "/tenants/tenant-1/reports/aging/payables?format=pdf", (*Handlers).GetPayablesAging, func(t *testing.T) {
			original := exportAgingReportPDF
			exportAgingReportPDF = func(*analytics.AgingReport) ([]byte, error) { return nil, errWave10 }
			t.Cleanup(func() { exportAgingReportPDF = original })
		}, "Failed to export aging PDF"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h, _, tenantRepo := setupAnalyticsTestHandlers()
			tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
			tt.setup(t)
			req := withURLParams(httptest.NewRequest(http.MethodGet, tt.path, nil), map[string]string{"tenantID": "tenant-1"})
			rr := httptest.NewRecorder()
			tt.call(h, rr, req)

			require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
			assert.Contains(t, rr.Body.String(), tt.want)
		})
	}
}

func TestWave10BusinessPDFGenerationSeams(t *testing.T) {
	t.Run("invoice pdf", func(t *testing.T) {
		h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		invoiceRepo.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
		h.pdfService = internalpdf.NewService()
		original := generateInvoicePDF
		generateInvoicePDF = func(*internalpdf.Service, *invoicing.Invoice, *tenant.Tenant, internalpdf.PDFSettings) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generateInvoicePDF = original
		})

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/invoices/inv-1/pdf", nil), map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-1"})
		rr := httptest.NewRecorder()
		h.GetInvoicePDF(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})

	t.Run("invoice email attachment pdf", func(t *testing.T) {
		h, tenantRepo, invoiceRepo := setupInvoiceTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		invoiceRepo.addTestInvoice("inv-1", "tenant-1", "contact-1", invoicing.InvoiceTypeSales, invoicing.StatusSent)
		emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
		emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateInvoiceSend)] = email.EmailTemplate{
			TenantID:     "tenant-1",
			TemplateType: email.TemplateInvoiceSend,
			Subject:      "Invoice",
			BodyHTML:     "<p>Invoice</p>",
			IsActive:     true,
		}
		h.pdfService = internalpdf.NewService()
		original := generateInvoicePDF
		generateInvoicePDF = func(*internalpdf.Service, *invoicing.Invoice, *tenant.Tenant, internalpdf.PDFSettings) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generateInvoicePDF = original
		})

		req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/invoices/inv-1/email", email.SendInvoiceRequest{
			RecipientEmail: "customer@example.com",
			RecipientName:  "Customer",
			AttachPDF:      true,
		}, createTestClaims("user-1", "test@example.com", "tenant-1", "owner"))
		req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "invoiceID": "inv-1"})
		rr := httptest.NewRecorder()
		h.EmailInvoice(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})

	t.Run("quote pdf", func(t *testing.T) {
		h, quoteRepo, tenantRepo := setupQuotesTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		quoteRepo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
		h.pdfService = internalpdf.NewService()
		original := generateQuotePDF
		generateQuotePDF = func(*internalpdf.Service, *quotes.Quote, *tenant.Tenant, internalpdf.PDFSettings) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generateQuotePDF = original
		})

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/quotes/quote-1/pdf", nil), map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
		rr := httptest.NewRecorder()
		h.GetQuotePDF(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})

	t.Run("quote email attachment pdf", func(t *testing.T) {
		h, quoteRepo, tenantRepo := setupQuotesTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		quoteRepo.quotes["quote-1"] = wave5Quote(quotes.QuoteStatusSent)
		emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
		emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateQuoteSend)] = email.EmailTemplate{
			TenantID:     "tenant-1",
			TemplateType: email.TemplateQuoteSend,
			Subject:      "Quote",
			BodyHTML:     "<p>Quote</p>",
			IsActive:     true,
		}
		h.pdfService = internalpdf.NewService()
		original := generateQuotePDF
		generateQuotePDF = func(*internalpdf.Service, *quotes.Quote, *tenant.Tenant, internalpdf.PDFSettings) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generateQuotePDF = original
		})

		req := wave5Request(http.MethodPost, "/tenants/tenant-1/quotes/quote-1/email", email.SendQuoteRequest{
			RecipientEmail: "customer@example.com",
			RecipientName:  "Customer",
			AttachPDF:      true,
		}, map[string]string{"tenantID": "tenant-1", "quoteID": "quote-1"})
		rr := httptest.NewRecorder()
		h.EmailQuote(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})

	t.Run("order pdf", func(t *testing.T) {
		h, orderRepo, tenantRepo := setupOrdersTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		orderRepo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
		h.pdfService = internalpdf.NewService()
		original := generateOrderPDF
		generateOrderPDF = func(*internalpdf.Service, *orders.Order, *tenant.Tenant, internalpdf.PDFSettings) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generateOrderPDF = original
		})

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/orders/order-1/pdf", nil), map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
		rr := httptest.NewRecorder()
		h.GetOrderPDF(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})

	t.Run("order email attachment pdf", func(t *testing.T) {
		h, orderRepo, tenantRepo := setupOrdersTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		orderRepo.orders["order-1"] = wave5Order(orders.OrderStatusConfirmed)
		emailRepo, _ := configureEmailHandlerService(h, "tenant-1")
		emailRepo.templates[emailTemplateKey("tenant-1", email.TemplateOrderConfirm)] = email.EmailTemplate{
			TenantID:     "tenant-1",
			TemplateType: email.TemplateOrderConfirm,
			Subject:      "Order",
			BodyHTML:     "<p>Order</p>",
			IsActive:     true,
		}
		h.pdfService = internalpdf.NewService()
		original := generateOrderPDF
		generateOrderPDF = func(*internalpdf.Service, *orders.Order, *tenant.Tenant, internalpdf.PDFSettings) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generateOrderPDF = original
		})

		req := wave5Request(http.MethodPost, "/tenants/tenant-1/orders/order-1/email", email.SendOrderRequest{
			RecipientEmail: "customer@example.com",
			RecipientName:  "Customer",
			AttachPDF:      true,
		}, map[string]string{"tenantID": "tenant-1", "orderID": "order-1"})
		rr := httptest.NewRecorder()
		h.EmailOrder(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})

	t.Run("payslip pdf", func(t *testing.T) {
		h, payrollRepo, _ := setupPayrollImportHandlerTest(t)
		tenantRepo := newMockTenantRepository()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		h.tenantService = tenant.NewServiceWithRepository(tenantRepo)
		payrollRepo.payrollRuns["run-1"] = &payroll.PayrollRun{
			ID:          "run-1",
			TenantID:    "tenant-1",
			PeriodYear:  2026,
			PeriodMonth: 5,
		}
		payrollRepo.payslips = []payroll.Payslip{{
			ID:           "payslip-1",
			TenantID:     "tenant-1",
			PayrollRunID: "run-1",
			EmployeeID:   "employee-1",
		}}
		h.pdfService = internalpdf.NewService()
		original := generatePayslipPDF
		generatePayslipPDF = func(*internalpdf.Service, *payroll.Payslip, *payroll.PayrollRun, *tenant.Tenant) ([]byte, error) {
			return nil, errWave10
		}
		t.Cleanup(func() {
			generatePayslipPDF = original
		})

		req := payrollHandlerRequest(http.MethodGet, "/tenants/tenant-1/payroll-runs/run-1/payslips/payslip-1/pdf", nil, map[string]string{
			"tenantID":  "tenant-1",
			"runID":     "run-1",
			"payslipID": "payslip-1",
		})
		rr := httptest.NewRecorder()
		h.GetPayslipPDF(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Failed to generate PDF")
	})
}
