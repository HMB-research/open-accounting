package main

import (
	"archive/zip"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func TestWave6CoreReportExportWrappers(t *testing.T) {
	asOf := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	account := accounting.AccountBalance{
		AccountID:     "acc-1",
		AccountCode:   "1000",
		AccountName:   "Cash",
		AccountType:   accounting.AccountTypeAsset,
		DebitBalance:  decimal.NewFromInt(100),
		CreditBalance: decimal.Zero,
		NetBalance:    decimal.NewFromInt(100),
	}

	trial := &accounting.TrialBalance{
		Accounts:     []accounting.AccountBalance{account},
		TotalDebits:  decimal.NewFromInt(100),
		TotalCredits: decimal.Zero,
	}
	xlsx, err := trialBalanceXLSX(trial)
	require.NoError(t, err)
	requireXLSXContains(t, xlsx, "Cash", "<v>100</v>")
	pdf, err := trialBalancePDF(trial, asOf)
	require.NoError(t, err)
	requirePDF(t, pdf)

	balance := &accounting.BalanceSheet{
		Assets:      []accounting.AccountBalance{account},
		TotalAssets: decimal.NewFromInt(100),
		TotalEquity: decimal.NewFromInt(100),
	}
	xlsx, err = balanceSheetXLSX(balance)
	require.NoError(t, err)
	requireXLSXContains(t, xlsx, "total_assets")
	pdf, err = balanceSheetPDF(balance, asOf)
	require.NoError(t, err)
	requirePDF(t, pdf)

	income := &accounting.IncomeStatement{
		Revenue:      []accounting.AccountBalance{{AccountCode: "3000", AccountName: "Sales", AccountType: accounting.AccountTypeRevenue, CreditBalance: decimal.NewFromInt(250), NetBalance: decimal.NewFromInt(250)}},
		TotalRevenue: decimal.NewFromInt(250),
		NetIncome:    decimal.NewFromInt(250),
	}
	xlsx, err = incomeStatementXLSX(income)
	require.NoError(t, err)
	requireXLSXContains(t, xlsx, "Sales", "net_income")
	pdf, err = incomeStatementPDF(income, asOf, asOf.AddDate(0, 0, 5))
	require.NoError(t, err)
	requirePDF(t, pdf)

	cashFlow := &reports.CashFlowStatement{
		StartDate:           "2026-01-01",
		EndDate:             "2026-01-31",
		OperatingActivities: []reports.CashFlowItem{{Code: "cash_in", Description: "Receipts", Amount: decimal.NewFromInt(75)}},
		OpeningCash:         decimal.NewFromInt(25),
		NetCashChange:       decimal.NewFromInt(75),
		ClosingCash:         decimal.NewFromInt(100),
	}
	xlsx, err = cashFlowStatementXLSX(cashFlow)
	require.NoError(t, err)
	requireXLSXContains(t, xlsx, "cash_in", "closing_cash")
	pdf, err = cashFlowStatementPDF(cashFlow)
	require.NoError(t, err)
	requirePDF(t, pdf)

	xlsx, err = accountBalanceXLSX("acc-1", "2026-06-25", "100")
	require.NoError(t, err)
	requireXLSXContains(t, xlsx, "acc-1", "<v>100</v>")
	pdf, err = accountBalancePDF("acc-1", "2026-06-25", "100")
	require.NoError(t, err)
	requirePDF(t, pdf)
}

func TestWave6ReportRowsAndWorkbookBoundaries(t *testing.T) {
	pdf, err := reportRowsPDF("No Subtitle", " ", [][]string{
		{"amount", "description", "extra"},
		{"123.45", "short"},
	})
	require.NoError(t, err)
	requirePDF(t, pdf)

	_, err = reportCSVBytesToPDF("Broken CSV", "", []byte("amount,\"unterminated"))
	require.Error(t, err)

	xlsx, err := reportRowsXLSX("Empty Rows", nil)
	require.NoError(t, err)
	require.Greater(t, len(xlsx), 0)

	assert.Equal(t, "AA3", xlsxCellRef(3, 27))
	assert.Equal(t, `<c r="A1" t="inlineStr"><is><t></t></is></c>`, xlsxCellXML(1, 1, "description", ""))
	assert.Contains(t, xlsxWorkbookXML(`Budget & Forecast`), "Budget &amp; Forecast")

	writer := zip.NewWriter(erroringWriter{})
	require.NoError(t, writeXLSXPart(writer, "closed.xml", "payload"))
	require.Error(t, writer.Close())
}
