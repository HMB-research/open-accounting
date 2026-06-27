package main

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func TestReportResponseFormatAdditionalBranches(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/reports?format=%20XLSX%20", nil)
	format, err := reportResponseFormat(req)
	require.NoError(t, err)
	assert.Equal(t, "xlsx", format)

	req = httptest.NewRequest(http.MethodGet, "/reports?format=xml", nil)
	format, err = reportResponseFormat(req)
	require.Error(t, err)
	assert.Empty(t, format)
	assert.Contains(t, err.Error(), "format must be json, csv, xlsx, or pdf")
}

func TestReportResponseWritersHeaders(t *testing.T) {
	tests := []struct {
		name        string
		respond     func(http.ResponseWriter, string, []byte)
		contentType string
		fileName    string
	}{
		{name: "csv", respond: respondReportCSV, contentType: "text/csv; charset=utf-8", fileName: "report.csv"},
		{name: "xlsx", respond: respondReportXLSX, contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", fileName: "report.xlsx"},
		{name: "pdf", respond: respondReportPDF, contentType: "application/pdf", fileName: "report.pdf"},
		{name: "xml", respond: respondReportXML, contentType: "application/xml", fileName: "report.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			tt.respond(w, tt.fileName, []byte("payload"))

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.contentType, w.Header().Get("Content-Type"))
			assert.Contains(t, w.Header().Get("Content-Disposition"), tt.fileName)
			assert.Equal(t, "payload", w.Body.String())
		})
	}
}

func TestCoreReportCSVHelpersWithRows(t *testing.T) {
	asset := accounting.AccountBalance{
		AccountID:    "acc-1",
		AccountCode:  "1000",
		AccountName:  "Cash",
		AccountType:  accounting.AccountTypeAsset,
		DebitBalance: decimal.NewFromInt(125),
		NetBalance:   decimal.NewFromInt(125),
	}
	liability := accounting.AccountBalance{
		AccountID:     "acc-2",
		AccountCode:   "2000",
		AccountName:   "Payables",
		AccountType:   accounting.AccountTypeLiability,
		CreditBalance: decimal.NewFromInt(25),
		NetBalance:    decimal.NewFromInt(-25),
	}

	trialCSV, err := trialBalanceCSV(&accounting.TrialBalance{
		Accounts:     []accounting.AccountBalance{asset, liability},
		TotalDebits:  decimal.NewFromInt(125),
		TotalCredits: decimal.NewFromInt(25),
	})
	require.NoError(t, err)
	assert.Contains(t, string(trialCSV), "account_code,account_name,account_type")
	assert.Contains(t, string(trialCSV), "1000,Cash,ASSET")
	assert.Contains(t, string(trialCSV), "TOTAL,Total")

	balanceCSV, err := balanceSheetCSV(&accounting.BalanceSheet{
		Assets:           []accounting.AccountBalance{asset},
		Liabilities:      []accounting.AccountBalance{liability},
		TotalAssets:      decimal.NewFromInt(125),
		TotalLiabilities: decimal.NewFromInt(25),
		TotalEquity:      decimal.NewFromInt(100),
		RetainedEarnings: decimal.NewFromInt(100),
	})
	require.NoError(t, err)
	assert.Contains(t, string(balanceCSV), "assets,1000,Cash,ASSET")
	assert.Contains(t, string(balanceCSV), "total_equity")

	incomeCSV, err := incomeStatementCSV(&accounting.IncomeStatement{
		Revenue:       []accounting.AccountBalance{{AccountCode: "3000", AccountName: "Sales", AccountType: accounting.AccountTypeRevenue, CreditBalance: decimal.NewFromInt(200), NetBalance: decimal.NewFromInt(200)}},
		Expenses:      []accounting.AccountBalance{{AccountCode: "5000", AccountName: "Rent", AccountType: accounting.AccountTypeExpense, DebitBalance: decimal.NewFromInt(60), NetBalance: decimal.NewFromInt(60)}},
		TotalRevenue:  decimal.NewFromInt(200),
		TotalExpenses: decimal.NewFromInt(60),
		NetIncome:     decimal.NewFromInt(140),
	})
	require.NoError(t, err)
	assert.Contains(t, string(incomeCSV), "revenue,3000,Sales,REVENUE")
	assert.Contains(t, string(incomeCSV), "net_income")

	cashFlowCSV, err := cashFlowStatementCSV(&reports.CashFlowStatement{
		StartDate:           "2026-01-01",
		EndDate:             "2026-12-31",
		OperatingActivities: []reports.CashFlowItem{{Code: "op", Description: "Operating receipts", DescriptionET: "Laekumised", Amount: decimal.NewFromInt(50), IsSubtotal: true}},
		InvestingActivities: []reports.CashFlowItem{{Code: "inv", Description: "Equipment", Amount: decimal.NewFromInt(-10)}},
		FinancingActivities: []reports.CashFlowItem{{Code: "fin", Description: "Loan", Amount: decimal.NewFromInt(20)}},
		OpeningCash:         decimal.NewFromInt(100),
		NetCashChange:       decimal.NewFromInt(60),
		ClosingCash:         decimal.NewFromInt(160),
	})
	require.NoError(t, err)
	assert.Contains(t, string(cashFlowCSV), "operating,op,Operating receipts,Laekumised,50,true")
	assert.Contains(t, string(cashFlowCSV), "summary,closing_cash")

	accountCSV, err := accountBalanceCSV("acc-1", "2026-06-30", "125")
	require.NoError(t, err)
	assert.Contains(t, string(accountCSV), "account_id,as_of_date,balance")
	assert.Contains(t, string(accountCSV), "acc-1,2026-06-30,125")
}

func TestReportCSVSectionWritersPropagateErrors(t *testing.T) {
	writer := csv.NewWriter(erroringWriter{})
	err := writeAccountSectionCSV(writer, "assets", []accounting.AccountBalance{{AccountCode: "1000"}})
	writer.Flush()
	require.NoError(t, err)
	require.Error(t, writer.Error())

	writer = csv.NewWriter(erroringWriter{})
	err = writeCashFlowSectionCSV(writer, "operating", []reports.CashFlowItem{{Code: "op"}})
	writer.Flush()
	require.NoError(t, err)
	require.Error(t, writer.Error())
}

type erroringWriter struct{}

func (erroringWriter) Write([]byte) (int, error) {
	return 0, assert.AnError
}

func TestReportPDFAndXLSXAdditionalBranches(t *testing.T) {
	_, err := reportCSVBytesToPDF("Broken", "", []byte{})
	require.ErrorContains(t, err, "report contains no rows")

	_, err = reportCSVBytesToXLSX("Broken", []byte("a,\"unterminated"))
	require.Error(t, err)

	pdf, err := reportRowsPDF("Title", "Subtitle", [][]string{
		{"amount", "description"},
		{"123.45", strings.Repeat("Long value ", 8)},
		{"not numeric", "short"},
	})
	require.NoError(t, err)
	requirePDF(t, pdf)

	xlsx, err := reportRowsXLSX(`Very/Long*Invalid?Sheet:Name[With]ExtraCharacters`, [][]string{
		{"amount", "is_subtotal", "description"},
		{"123.45", "true", "A&B"},
		{"not numeric", "false", ""},
	})
	require.NoError(t, err)
	requireXLSXContains(t, xlsx, `<v>123.45</v>`, `<v>1</v>`, `A&amp;B`)
}

func TestReportFormattingHelperBoundaries(t *testing.T) {
	assert.Equal(t, "Report", safeSheetName(" "))
	assert.Equal(t, "InvalidSheetName", safeSheetName(`Invalid/Sheet*Name?`))
	assert.Len(t, safeSheetName(strings.Repeat("a", 40)), 31)

	assert.Equal(t, "", reportExportDate(time.Time{}))
	assert.Equal(t, "2026-06-25", reportExportDate(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)))

	assert.Equal(t, "", reportPDFTitleCase(" "))
	assert.Equal(t, "Sales", reportPDFTitleCase("SALES"))
	assert.Equal(t, "123abc", reportPDFTitleCase("123ABC"))

	assert.Equal(t, "", truncateReportPDFText("abcdef", 0))
	assert.Equal(t, "ab", truncateReportPDFText("abcdef", 2))
	assert.Equal(t, "abcdef", truncateReportPDFText("abcdef", 6))
	assert.Equal(t, "abc...", truncateReportPDFText("abcdefghi", 6))

	assert.Equal(t, float64(5), reportPDFFontSize(15, false))
	assert.Equal(t, float64(6), reportPDFFontSize(10, false))
	assert.Equal(t, float64(7), reportPDFFontSize(4, true))
	assert.Equal(t, float64(7.5), reportPDFFontSize(4, false))

	assert.Equal(t, 12, reportPDFTextLimit(13, true))
	assert.Equal(t, 18, reportPDFTextLimit(8, true))
	assert.Equal(t, 14, reportPDFTextLimit(15, false))
	assert.Equal(t, 18, reportPDFTextLimit(10, false))
	assert.Equal(t, 32, reportPDFTextLimit(4, false))

	assert.Equal(t, align.Right, reportPDFAlign("amount", false))
	assert.Equal(t, align.Left, reportPDFAlign("amount", true))
	assert.Equal(t, align.Left, reportPDFAlign("description", false))

	assert.True(t, isNumericReportColumn(" Total_Amount "))
	assert.False(t, isNumericReportColumn("description"))
	assert.True(t, isBoolReportColumn(" is_subtotal "))
	assert.False(t, isBoolReportColumn("description"))
}
