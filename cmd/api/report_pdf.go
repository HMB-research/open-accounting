package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func trialBalancePDF(report *accounting.TrialBalance, asOfDate time.Time) ([]byte, error) {
	content, err := trialBalanceCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToPDF("Trial Balance", "As of "+asOfDate.Format("2006-01-02"), content)
}

func balanceSheetPDF(report *accounting.BalanceSheet, asOfDate time.Time) ([]byte, error) {
	content, err := balanceSheetCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToPDF("Balance Sheet", "As of "+asOfDate.Format("2006-01-02"), content)
}

func incomeStatementPDF(report *accounting.IncomeStatement, startDate, endDate time.Time) ([]byte, error) {
	content, err := incomeStatementCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToPDF("Income Statement", fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")), content)
}

func cashFlowStatementPDF(report *reports.CashFlowStatement) ([]byte, error) {
	content, err := cashFlowStatementCSV(report)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToPDF("Cash Flow Statement", fmt.Sprintf("%s to %s", report.StartDate, report.EndDate), content)
}

func accountBalancePDF(accountID, asOfDate, balance string) ([]byte, error) {
	content, err := accountBalanceCSV(accountID, asOfDate, balance)
	if err != nil {
		return nil, err
	}
	return reportCSVBytesToPDF("Account Balance", fmt.Sprintf("%s as of %s", accountID, asOfDate), content)
}

func agingReportPDF(report *analytics.AgingReport) ([]byte, error) {
	return reportRowsPDF(reportPDFTitleCase(report.ReportType)+" Aging", "As of "+reportExportDate(report.AsOfDate), agingReportRows(report))
}

func balanceConfirmationSummaryPDF(report *reports.BalanceConfirmationSummary) ([]byte, error) {
	return reportRowsPDF("Balance Confirmations", fmt.Sprintf("%s as of %s", report.Type, report.AsOfDate), balanceConfirmationSummaryRows(report))
}

func balanceConfirmationPDF(report *reports.BalanceConfirmation) ([]byte, error) {
	return reportRowsPDF("Balance Confirmation", fmt.Sprintf("%s %s as of %s", report.ContactName, report.Type, report.AsOfDate), balanceConfirmationRows(report))
}

func contactStatementPDF(report *reports.ContactStatement) ([]byte, error) {
	return reportRowsPDF("Contact Statement", fmt.Sprintf("%s %s from %s to %s", report.ContactName, report.Type, report.StartDate, report.EndDate), contactStatementRows(report))
}

func costCenterReportPDF(report *accounting.CostCenterReport) ([]byte, error) {
	return reportRowsPDF("Cost Center Report", fmt.Sprintf("%s to %s", reportExportDate(report.PeriodStart), reportExportDate(report.PeriodEnd)), costCenterReportRows(report))
}

func reportCSVBytesToPDF(title, subtitle string, content []byte) ([]byte, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return reportRowsPDF(title, subtitle, rows)
}

func reportRowsPDF(title, subtitle string, rows [][]string) ([]byte, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("report contains no rows")
	}

	maxCols := maxReportPDFColumns(rows)
	cfg := config.NewBuilder().
		WithPageNumber(props.PageNumber{
			Pattern: "Page {current} of {total}",
			Place:   props.RightBottom,
			Size:    8,
		}).
		WithOrientation(orientation.Horizontal).
		WithMaxGridSize(maxCols).
		WithLeftMargin(10).
		WithTopMargin(10).
		WithRightMargin(10).
		Build()

	m := maroto.New(cfg)
	m.AddRow(10,
		text.NewCol(maxCols, title, props.Text{
			Size:  16,
			Style: fontstyle.Bold,
			Align: align.Center,
		}),
	)
	if strings.TrimSpace(subtitle) != "" {
		m.AddRow(6,
			text.NewCol(maxCols, subtitle, props.Text{
				Size:  9,
				Align: align.Center,
			}),
		)
	}
	m.AddRow(3, col.New(maxCols).Add(line.New(props.Line{Thickness: 0.4})))
	m.AddRow(3)

	headers := normalizeReportPDFRow(rows[0], maxCols)
	m.AddRow(7, reportPDFColumns(headers, headers, true)...).WithStyle(&props.Cell{
		BackgroundColor: &props.Color{Red: 235, Green: 238, Blue: 242},
	})
	for i, rowData := range rows[1:] {
		rowStyle := &props.Cell{}
		if i%2 == 1 {
			rowStyle.BackgroundColor = &props.Color{Red: 248, Green: 249, Blue: 251}
		}
		m.AddRow(6, reportPDFColumns(normalizeReportPDFRow(rowData, maxCols), headers, false)...).WithStyle(rowStyle)
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate report PDF: %w", err)
	}
	return doc.GetBytes(), nil
}

func maxReportPDFColumns(rows [][]string) int {
	maxCols := 1
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	return maxCols
}

func normalizeReportPDFRow(row []string, maxCols int) []string {
	normalized := make([]string, maxCols)
	copy(normalized, row)
	return normalized
}

func reportPDFColumns(rowData, headers []string, isHeader bool) []core.Col {
	columns := make([]core.Col, 0, len(rowData))
	for index, value := range rowData {
		header := ""
		if index < len(headers) {
			header = headers[index]
		}
		style := props.Text{
			Size:  reportPDFFontSize(len(rowData), isHeader),
			Align: reportPDFAlign(header, isHeader),
		}
		if isHeader {
			style.Style = fontstyle.Bold
			value = reportPDFHeaderLabel(value)
		}
		columns = append(columns, text.NewCol(1, truncateReportPDFText(value, reportPDFTextLimit(len(rowData), isHeader)), style))
	}
	return columns
}

func reportPDFAlign(header string, isHeader bool) align.Type {
	if !isHeader && isNumericReportColumn(header) {
		return align.Right
	}
	return align.Left
}

func reportPDFFontSize(columnCount int, isHeader bool) float64 {
	if columnCount > 14 {
		return 5
	}
	if columnCount > 9 {
		return 6
	}
	if isHeader {
		return 7
	}
	return 7.5
}

func reportPDFTextLimit(columnCount int, isHeader bool) int {
	if isHeader {
		if columnCount > 12 {
			return 12
		}
		return 18
	}
	if columnCount > 14 {
		return 14
	}
	if columnCount > 9 {
		return 18
	}
	return 32
}

func reportPDFHeaderLabel(value string) string {
	return strings.ReplaceAll(value, "_", " ")
}

func reportPDFTitleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(strings.ToLower(value))
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] -= 'a' - 'A'
	}
	return string(runes)
}

func truncateReportPDFText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
