package main

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"time"

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

func reportExportDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func intString(value int) string {
	return strconv.Itoa(value)
}
