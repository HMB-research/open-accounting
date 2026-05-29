package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func agingReportCSV(report *analytics.AgingReport) ([]byte, error) {
	return rowsToCSV(agingReportRows(report))
}

func agingReportXLSX(report *analytics.AgingReport) ([]byte, error) {
	return reportRowsXLSX("Aging", agingReportRows(report))
}

func balanceConfirmationSummaryCSV(report *reports.BalanceConfirmationSummary) ([]byte, error) {
	return rowsToCSV(balanceConfirmationSummaryRows(report))
}

func balanceConfirmationSummaryXLSX(report *reports.BalanceConfirmationSummary) ([]byte, error) {
	return reportRowsXLSX("Balance Confirmations", balanceConfirmationSummaryRows(report))
}

func balanceConfirmationCSV(report *reports.BalanceConfirmation) ([]byte, error) {
	return rowsToCSV(balanceConfirmationRows(report))
}

func balanceConfirmationXLSX(report *reports.BalanceConfirmation) ([]byte, error) {
	return reportRowsXLSX("Balance Confirmation", balanceConfirmationRows(report))
}

func contactStatementCSV(report *reports.ContactStatement) ([]byte, error) {
	return rowsToCSV(contactStatementRows(report))
}

func contactStatementXLSX(report *reports.ContactStatement) ([]byte, error) {
	return reportRowsXLSX("Contact Statement", contactStatementRows(report))
}

func salesMarginCSV(report *reports.SalesMarginReport) ([]byte, error) {
	return rowsToCSV(salesMarginRows(report))
}

func salesMarginXLSX(report *reports.SalesMarginReport) ([]byte, error) {
	return reportRowsXLSX("Sales Margin", salesMarginRows(report))
}

func costCenterReportCSV(report *accounting.CostCenterReport) ([]byte, error) {
	return rowsToCSV(costCenterReportRows(report))
}

func costCenterReportXLSX(report *accounting.CostCenterReport) ([]byte, error) {
	return reportRowsXLSX("Cost Center Report", costCenterReportRows(report))
}

func rowsToCSV(rows [][]string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func agingReportRows(report *analytics.AgingReport) [][]string {
	asOfDate := reportExportDate(report.AsOfDate)
	rows := [][]string{{
		"row_type",
		"report_type",
		"as_of_date",
		"contact_id",
		"contact_name",
		"bucket_label",
		"amount",
		"count",
		"current",
		"days_1_30",
		"days_31_60",
		"days_61_90",
		"days_90_plus",
		"total",
	}}
	for _, bucket := range report.Buckets {
		rows = append(rows, []string{
			"bucket",
			report.ReportType,
			asOfDate,
			"",
			"",
			bucket.Label,
			bucket.Amount.String(),
			intString(bucket.Count),
			"",
			"",
			"",
			"",
			"",
			"",
		})
	}
	for _, contact := range report.ByContact {
		rows = append(rows, []string{
			"contact",
			report.ReportType,
			asOfDate,
			contact.ContactID,
			contact.ContactName,
			"",
			"",
			"",
			contact.Current.String(),
			contact.Days1to30.String(),
			contact.Days31to60.String(),
			contact.Days61to90.String(),
			contact.Days90Plus.String(),
			contact.Total.String(),
		})
	}
	rows = append(rows, []string{
		"total",
		report.ReportType,
		asOfDate,
		"",
		"",
		"",
		report.Total.String(),
		"",
		"",
		"",
		"",
		"",
		"",
		report.Total.String(),
	})
	return rows
}

func balanceConfirmationSummaryRows(report *reports.BalanceConfirmationSummary) [][]string {
	rows := [][]string{{
		"row_type",
		"type",
		"as_of_date",
		"total_balance",
		"contact_count",
		"invoice_count",
		"contact_id",
		"contact_name",
		"contact_code",
		"contact_email",
		"balance",
		"contact_invoice_count",
		"oldest_invoice",
	}}
	rows = append(rows, []string{
		"summary",
		string(report.Type),
		report.AsOfDate,
		report.TotalBalance.String(),
		intString(report.ContactCount),
		intString(report.InvoiceCount),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	})
	for _, contact := range report.Contacts {
		rows = append(rows, []string{
			"contact",
			string(report.Type),
			report.AsOfDate,
			"",
			"",
			"",
			contact.ContactID,
			contact.ContactName,
			contact.ContactCode,
			contact.ContactEmail,
			contact.Balance.String(),
			intString(contact.InvoiceCount),
			contact.OldestInvoice,
		})
	}
	return rows
}

func balanceConfirmationRows(report *reports.BalanceConfirmation) [][]string {
	rows := [][]string{{
		"row_type",
		"type",
		"as_of_date",
		"contact_id",
		"contact_name",
		"contact_code",
		"contact_email",
		"total_balance",
		"invoice_id",
		"invoice_number",
		"invoice_date",
		"due_date",
		"total_amount",
		"amount_paid",
		"outstanding_amount",
		"currency",
		"days_overdue",
	}}
	rows = append(rows, []string{
		"summary",
		string(report.Type),
		report.AsOfDate,
		report.ContactID,
		report.ContactName,
		report.ContactCode,
		report.ContactEmail,
		report.TotalBalance.String(),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	})
	for _, invoice := range report.Invoices {
		rows = append(rows, []string{
			"invoice",
			string(report.Type),
			report.AsOfDate,
			report.ContactID,
			report.ContactName,
			report.ContactCode,
			report.ContactEmail,
			"",
			invoice.InvoiceID,
			invoice.InvoiceNumber,
			invoice.InvoiceDate,
			invoice.DueDate,
			invoice.TotalAmount.String(),
			invoice.AmountPaid.String(),
			invoice.OutstandingAmount.String(),
			invoice.Currency,
			intString(invoice.DaysOverdue),
		})
	}
	return rows
}

func contactStatementRows(report *reports.ContactStatement) [][]string {
	rows := [][]string{{
		"row_type",
		"type",
		"start_date",
		"end_date",
		"contact_id",
		"contact_name",
		"contact_code",
		"contact_email",
		"opening_balance",
		"closing_balance",
		"total_invoiced",
		"total_paid",
		"date",
		"document_type",
		"document_id",
		"document_number",
		"due_date",
		"description",
		"reference",
		"currency",
		"document_amount",
		"statement_amount",
		"increase_amount",
		"decrease_amount",
		"balance",
	}}
	rows = append(rows, []string{
		"summary",
		string(report.Type),
		report.StartDate,
		report.EndDate,
		report.ContactID,
		report.ContactName,
		report.ContactCode,
		report.ContactEmail,
		report.OpeningBalance.String(),
		report.ClosingBalance.String(),
		report.TotalInvoiced.String(),
		report.TotalPaid.String(),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	})
	for _, entry := range report.Entries {
		rows = append(rows, []string{
			"entry",
			string(report.Type),
			report.StartDate,
			report.EndDate,
			report.ContactID,
			report.ContactName,
			report.ContactCode,
			report.ContactEmail,
			"",
			"",
			"",
			"",
			entry.Date,
			entry.DocumentType,
			entry.DocumentID,
			entry.DocumentNumber,
			entry.DueDate,
			entry.Description,
			entry.Reference,
			entry.Currency,
			entry.DocumentAmount.String(),
			entry.StatementAmount.String(),
			entry.IncreaseAmount.String(),
			entry.DecreaseAmount.String(),
			entry.Balance.String(),
		})
	}
	return rows
}

func salesMarginRows(report *reports.SalesMarginReport) [][]string {
	rows := [][]string{{
		"row_type",
		"start_date",
		"end_date",
		"total_revenue",
		"total_cost",
		"total_margin",
		"margin_percent",
		"line_count",
		"invoice_id",
		"invoice_number",
		"invoice_date",
		"contact_id",
		"contact_name",
		"product_id",
		"product_code",
		"product_name",
		"description",
		"quantity",
		"revenue",
		"unit_cost",
		"cost",
		"margin",
		"line_margin_percent",
	}}
	rows = append(rows, []string{
		"summary",
		report.StartDate,
		report.EndDate,
		report.TotalRevenue.String(),
		report.TotalCost.String(),
		report.TotalMargin.String(),
		report.MarginPercent.String(),
		intString(report.LineCount),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	})
	for _, line := range report.Lines {
		rows = append(rows, []string{
			"line",
			report.StartDate,
			report.EndDate,
			"",
			"",
			"",
			"",
			"",
			line.InvoiceID,
			line.InvoiceNumber,
			line.InvoiceDate,
			line.ContactID,
			line.ContactName,
			line.ProductID,
			line.ProductCode,
			line.ProductName,
			line.Description,
			line.Quantity.String(),
			line.Revenue.String(),
			line.UnitCost.String(),
			line.Cost.String(),
			line.Margin.String(),
			line.MarginPercent.String(),
		})
	}
	return rows
}

func costCenterReportRows(report *accounting.CostCenterReport) [][]string {
	periodStart := reportExportDate(report.PeriodStart)
	periodEnd := reportExportDate(report.PeriodEnd)
	rows := [][]string{{
		"row_type",
		"period_start",
		"period_end",
		"cost_center_id",
		"code",
		"name",
		"total_expenses",
		"budget_amount",
		"budget_used_percentage",
		"is_over_budget",
	}}
	for _, summary := range report.CostCenters {
		rows = append(rows, []string{
			"cost_center",
			periodStart,
			periodEnd,
			summary.CostCenter.ID,
			summary.CostCenter.Code,
			summary.CostCenter.Name,
			summary.TotalExpenses.String(),
			summary.BudgetAmount.String(),
			summary.BudgetUsed.String(),
			fmt.Sprintf("%t", summary.IsOverBudget),
		})
	}

	totalBudgetUsed := "0"
	totalOverBudget := false
	if report.TotalBudget.IsPositive() {
		totalBudgetUsed = report.TotalExpenses.Div(report.TotalBudget).Mul(decimal.NewFromInt(100)).String()
		totalOverBudget = report.TotalExpenses.GreaterThan(report.TotalBudget)
	}
	rows = append(rows, []string{
		"total",
		periodStart,
		periodEnd,
		"",
		"TOTAL",
		"Total",
		report.TotalExpenses.String(),
		report.TotalBudget.String(),
		totalBudgetUsed,
		fmt.Sprintf("%t", totalOverBudget),
	})

	return rows
}

func reportExportDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func intString(value int) string {
	return strconv.Itoa(value)
}
