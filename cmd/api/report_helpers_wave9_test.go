package main

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func TestWave9ExtendedReportExportsSmoke(t *testing.T) {
	aging := &analytics.AgingReport{
		ReportType: "receivables",
		AsOfDate:   mustParseDateForReportTest("2026-06-25"),
		Buckets: []analytics.AgingBucket{{
			Label:  "current",
			Amount: decimal.NewFromInt(125),
			Count:  2,
		}},
		ByContact: []analytics.ContactAging{{
			ContactID:   "contact-1",
			ContactName: "Alpha OU",
			Current:     decimal.NewFromInt(125),
			Total:       decimal.NewFromInt(125),
		}},
		Total: decimal.NewFromInt(125),
	}
	csvBytes, err := agingReportCSV(aging)
	require.NoError(t, err)
	assert.Contains(t, string(csvBytes), "receivables")
	xlsxBytes, err := agingReportXLSX(aging)
	require.NoError(t, err)
	requireXLSXContains(t, xlsxBytes, "Alpha OU", "<v>125</v>")
	pdfBytes, err := agingReportPDF(aging)
	require.NoError(t, err)
	requirePDF(t, pdfBytes)

	summary := &reports.BalanceConfirmationSummary{
		Type:     reports.BalanceTypeReceivable,
		AsOfDate: "2026-06-25",
		Contacts: []reports.ContactBalance{{
			ContactID:    "contact-1",
			ContactName:  "Alpha OU",
			InvoiceCount: 1,
			Balance:      decimal.NewFromInt(125),
		}},
		TotalBalance: decimal.NewFromInt(125),
	}
	csvBytes, err = balanceConfirmationSummaryCSV(summary)
	require.NoError(t, err)
	assert.Contains(t, string(csvBytes), "Alpha OU")
	xlsxBytes, err = balanceConfirmationSummaryXLSX(summary)
	require.NoError(t, err)
	requireXLSXContains(t, xlsxBytes, "Alpha OU", "<v>125</v>")
	pdfBytes, err = balanceConfirmationSummaryPDF(summary)
	require.NoError(t, err)
	requirePDF(t, pdfBytes)

	statement := &reports.ContactStatement{
		ContactID:   "contact-1",
		ContactName: "Alpha OU",
		Type:        reports.BalanceTypeReceivable,
		StartDate:   "2026-06-01",
		EndDate:     "2026-06-25",
		Entries: []reports.ContactStatementEntry{{
			Date:            "2026-06-10",
			DocumentType:    "invoice",
			DocumentID:      "invoice-1",
			DocumentNumber:  "INV-1",
			Description:     "Invoice INV-1",
			Currency:        "EUR",
			DocumentAmount:  decimal.NewFromInt(125),
			StatementAmount: decimal.NewFromInt(125),
			IncreaseAmount:  decimal.NewFromInt(125),
			Balance:         decimal.NewFromInt(125),
		}},
		ClosingBalance: decimal.NewFromInt(125),
	}
	csvBytes, err = contactStatementCSV(statement)
	require.NoError(t, err)
	assert.Contains(t, string(csvBytes), "Invoice INV-1")
	xlsxBytes, err = contactStatementXLSX(statement)
	require.NoError(t, err)
	requireXLSXContains(t, xlsxBytes, "Invoice INV-1")
	pdfBytes, err = contactStatementPDF(statement)
	require.NoError(t, err)
	requirePDF(t, pdfBytes)

	margin := &reports.SalesMarginReport{
		StartDate: "2026-06-01",
		EndDate:   "2026-06-25",
		Lines: []reports.SalesMarginLine{{
			InvoiceID:     "invoice-1",
			InvoiceNumber: "INV-1",
			InvoiceDate:   "2026-06-10",
			ContactID:     "contact-1",
			ContactName:   "Alpha OU",
			ProductID:     "product-1",
			ProductName:   "Widget",
			Description:   "Widget",
			Quantity:      decimal.NewFromInt(2),
			Revenue:       decimal.NewFromInt(200),
			UnitCost:      decimal.NewFromInt(40),
			Cost:          decimal.NewFromInt(80),
			Margin:        decimal.NewFromInt(120),
			MarginPercent: decimal.NewFromInt(60),
		}},
		TotalRevenue:  decimal.NewFromInt(200),
		TotalCost:     decimal.NewFromInt(80),
		TotalMargin:   decimal.NewFromInt(120),
		MarginPercent: decimal.NewFromInt(60),
		LineCount:     1,
	}
	csvBytes, err = salesMarginCSV(margin)
	require.NoError(t, err)
	assert.Contains(t, string(csvBytes), "Widget")
	xlsxBytes, err = salesMarginXLSX(margin)
	require.NoError(t, err)
	requireXLSXContains(t, xlsxBytes, "Widget")
	pdfBytes, err = salesMarginPDF(margin)
	require.NoError(t, err)
	requirePDF(t, pdfBytes)

	costCenter := &accounting.CostCenterReport{
		PeriodStart: mustParseDateForReportTest("2026-06-01"),
		PeriodEnd:   mustParseDateForReportTest("2026-06-25"),
		CostCenters: []accounting.CostCenterSummary{{
			CostCenter: accounting.CostCenter{
				ID:   "cc-1",
				Code: "ADMIN",
				Name: "Administration",
			},
			TotalExpenses: decimal.NewFromInt(80),
			BudgetAmount:  decimal.NewFromInt(100),
			BudgetUsed:    decimal.NewFromInt(80),
		}},
		TotalExpenses: decimal.NewFromInt(80),
		TotalBudget:   decimal.NewFromInt(100),
	}
	csvBytes, err = costCenterReportCSV(costCenter)
	require.NoError(t, err)
	assert.Contains(t, string(csvBytes), "Administration")
	xlsxBytes, err = costCenterReportXLSX(costCenter)
	require.NoError(t, err)
	requireXLSXContains(t, xlsxBytes, "Administration")
	pdfBytes, err = costCenterReportPDF(costCenter)
	require.NoError(t, err)
	requirePDF(t, pdfBytes)
}
