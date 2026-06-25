package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/reports"
)

var (
	reportCSVWriteRecord = func(writer *csv.Writer, row []string) error {
		return writer.Write(row)
	}
	reportCSVFlush = func(writer *csv.Writer) error {
		writer.Flush()
		return writer.Error()
	}

	exportTrialBalanceCSV      = trialBalanceCSV
	exportBalanceSheetCSV      = balanceSheetCSV
	exportIncomeStatementCSV   = incomeStatementCSV
	exportCashFlowStatementCSV = cashFlowStatementCSV
	exportAccountBalanceCSV    = accountBalanceCSV
)

func reportResponseFormat(r *http.Request) (string, error) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		return "json", nil
	}
	switch format {
	case "json", "csv", "xlsx", "pdf":
		return format, nil
	default:
		return "", fmt.Errorf("format must be json, csv, xlsx, or pdf")
	}
}

func respondReportCSV(w http.ResponseWriter, fileName string, content []byte) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func respondReportXLSX(w http.ResponseWriter, fileName string, content []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func respondReportPDF(w http.ResponseWriter, fileName string, content []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func respondReportXML(w http.ResponseWriter, fileName string, content []byte) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func trialBalanceCSV(report *accounting.TrialBalance) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := reportCSVWriteRecord(writer, []string{"account_code", "account_name", "account_type", "debit_balance", "credit_balance", "net_balance"}); err != nil {
		return nil, err
	}
	for _, account := range report.Accounts {
		if err := reportCSVWriteRecord(writer, accountBalanceCSVRow(account)); err != nil {
			return nil, err
		}
	}
	if err := reportCSVWriteRecord(writer, []string{"TOTAL", "Total", "", report.TotalDebits.String(), report.TotalCredits.String(), ""}); err != nil {
		return nil, err
	}
	if err := reportCSVFlush(writer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func balanceSheetCSV(report *accounting.BalanceSheet) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := reportCSVWriteRecord(writer, []string{"section", "account_code", "account_name", "account_type", "debit_balance", "credit_balance", "net_balance"}); err != nil {
		return nil, err
	}
	if err := writeAccountSectionCSV(writer, "assets", report.Assets); err != nil {
		return nil, err
	}
	if err := writeAccountSectionCSV(writer, "liabilities", report.Liabilities); err != nil {
		return nil, err
	}
	if err := writeAccountSectionCSV(writer, "equity", report.Equity); err != nil {
		return nil, err
	}
	for _, row := range [][]string{
		{"total_assets", "", "Total assets", "", "", "", report.TotalAssets.String()},
		{"total_liabilities", "", "Total liabilities", "", "", "", report.TotalLiabilities.String()},
		{"retained_earnings", "", "Retained earnings", "", "", "", report.RetainedEarnings.String()},
		{"total_equity", "", "Total equity", "", "", "", report.TotalEquity.String()},
	} {
		if err := reportCSVWriteRecord(writer, row); err != nil {
			return nil, err
		}
	}
	if err := reportCSVFlush(writer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func incomeStatementCSV(report *accounting.IncomeStatement) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := reportCSVWriteRecord(writer, []string{"section", "account_code", "account_name", "account_type", "debit_balance", "credit_balance", "net_balance"}); err != nil {
		return nil, err
	}
	if err := writeAccountSectionCSV(writer, "revenue", report.Revenue); err != nil {
		return nil, err
	}
	if err := writeAccountSectionCSV(writer, "expenses", report.Expenses); err != nil {
		return nil, err
	}
	for _, row := range [][]string{
		{"total_revenue", "", "Total revenue", "", "", "", report.TotalRevenue.String()},
		{"total_expenses", "", "Total expenses", "", "", "", report.TotalExpenses.String()},
		{"net_income", "", "Net income", "", "", "", report.NetIncome.String()},
	} {
		if err := reportCSVWriteRecord(writer, row); err != nil {
			return nil, err
		}
	}
	if err := reportCSVFlush(writer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func cashFlowStatementCSV(report *reports.CashFlowStatement) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := reportCSVWriteRecord(writer, []string{"section", "code", "description", "description_et", "amount", "is_subtotal"}); err != nil {
		return nil, err
	}
	if err := writeCashFlowSectionCSV(writer, "operating", report.OperatingActivities); err != nil {
		return nil, err
	}
	if err := writeCashFlowSectionCSV(writer, "investing", report.InvestingActivities); err != nil {
		return nil, err
	}
	if err := writeCashFlowSectionCSV(writer, "financing", report.FinancingActivities); err != nil {
		return nil, err
	}
	for _, row := range [][]string{
		{"summary", "opening_cash", "Opening cash", "", report.OpeningCash.String(), "true"},
		{"summary", "net_cash_change", "Net cash change", "", report.NetCashChange.String(), "true"},
		{"summary", "closing_cash", "Closing cash", "", report.ClosingCash.String(), "true"},
	} {
		if err := reportCSVWriteRecord(writer, row); err != nil {
			return nil, err
		}
	}
	if err := reportCSVFlush(writer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func accountBalanceCSV(accountID, asOfDate, balance string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	if err := reportCSVWriteRecord(writer, []string{"account_id", "as_of_date", "balance"}); err != nil {
		return nil, err
	}
	if err := reportCSVWriteRecord(writer, []string{accountID, asOfDate, balance}); err != nil {
		return nil, err
	}
	if err := reportCSVFlush(writer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeAccountSectionCSV(writer *csv.Writer, section string, accounts []accounting.AccountBalance) error {
	for _, account := range accounts {
		row := append([]string{section}, accountBalanceCSVRow(account)...)
		if err := reportCSVWriteRecord(writer, row); err != nil {
			return err
		}
	}
	return nil
}

func accountBalanceCSVRow(account accounting.AccountBalance) []string {
	return []string{
		account.AccountCode,
		account.AccountName,
		string(account.AccountType),
		account.DebitBalance.String(),
		account.CreditBalance.String(),
		account.NetBalance.String(),
	}
}

func writeCashFlowSectionCSV(writer *csv.Writer, section string, items []reports.CashFlowItem) error {
	for _, item := range items {
		if err := reportCSVWriteRecord(writer, []string{
			section,
			item.Code,
			item.Description,
			item.DescriptionET,
			item.Amount.String(),
			fmt.Sprintf("%t", item.IsSubtotal),
		}); err != nil {
			return err
		}
	}
	return nil
}
