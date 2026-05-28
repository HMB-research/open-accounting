package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/reports"
)

func printJSON(w io.Writer, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(payload))
	return err
}

func printAPITokensTable(w io.Writer, tokens []apitoken.APIToken) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tPREFIX\tEXPIRES\tLAST USED\tCREATED")
	for _, token := range tokens {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			token.ID,
			token.Name,
			token.TokenPrefix,
			formatTimePtr(token.ExpiresAt),
			formatTimePtr(token.LastUsedAt),
			token.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printAccountsTable(w io.Writer, accounts []accounting.Account) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCODE\tNAME\tTYPE\tACTIVE")
	for _, account := range accounts {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", account.ID, account.Code, account.Name, account.AccountType, account.IsActive)
	}
	_ = tw.Flush()
}

func printContactsTable(w io.Writer, contactsList []contacts.Contact) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTYPE\tEMAIL\tACTIVE")
	for _, contact := range contactsList {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", contact.ID, contact.Name, contact.ContactType, contact.Email, contact.IsActive)
	}
	_ = tw.Flush()
}

func printEmployeesTable(w io.Writer, employees []payroll.Employee) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNUMBER\tNAME\tTYPE\tEMAIL\tACTIVE")
	for _, employee := range employees {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%t\n",
			employee.ID,
			employee.EmployeeNumber,
			employee.FullName(),
			employee.EmploymentType,
			employee.Email,
			employee.IsActive,
		)
	}
	_ = tw.Flush()
}

func printDocumentsTable(w io.Writer, docs []documents.Document) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tENTITY\tTYPE\tFILE\tREVIEW\tRETENTION\tCREATED")
	for _, doc := range docs {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s:%s\t%s\t%s\t%s\t%s\t%s\n",
			doc.ID,
			doc.EntityType,
			doc.EntityID,
			doc.DocumentType,
			doc.FileName,
			doc.ReviewStatus,
			formatTimePtr(doc.RetentionUntil),
			doc.CreatedAt.Format(time.RFC3339),
		)
	}
	_ = tw.Flush()
}

func printTrialBalance(w io.Writer, report *accounting.TrialBalance) {
	_, _ = fmt.Fprintf(w, "Trial balance as of %s\n", formatDate(report.AsOfDate))
	printAccountBalances(w, report.Accounts)
	_, _ = fmt.Fprintf(w, "Total debits: %s\n", report.TotalDebits.String())
	_, _ = fmt.Fprintf(w, "Total credits: %s\n", report.TotalCredits.String())
	_, _ = fmt.Fprintf(w, "Balanced: %t\n", report.IsBalanced)
}

func printAccountBalance(w io.Writer, report *accountBalanceReport) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ACCOUNT ID\tAS OF\tBALANCE")
	_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", report.AccountID, report.AsOfDate, report.Balance)
	_ = tw.Flush()
}

func printBalanceSheet(w io.Writer, report *accounting.BalanceSheet) {
	_, _ = fmt.Fprintf(w, "Balance sheet as of %s\n", formatDate(report.AsOfDate))
	printReportSection(w, "Assets", report.Assets)
	printReportSection(w, "Liabilities", report.Liabilities)
	printReportSection(w, "Equity", report.Equity)
	_, _ = fmt.Fprintf(w, "Retained earnings: %s\n", report.RetainedEarnings.String())
	_, _ = fmt.Fprintf(w, "Total assets: %s\n", report.TotalAssets.String())
	_, _ = fmt.Fprintf(w, "Total liabilities: %s\n", report.TotalLiabilities.String())
	_, _ = fmt.Fprintf(w, "Total equity: %s\n", report.TotalEquity.String())
	_, _ = fmt.Fprintf(w, "Balanced: %t\n", report.IsBalanced)
}

func printIncomeStatement(w io.Writer, report *accounting.IncomeStatement) {
	_, _ = fmt.Fprintf(w, "Income statement %s to %s\n", formatDate(report.StartDate), formatDate(report.EndDate))
	printReportSection(w, "Revenue", report.Revenue)
	printReportSection(w, "Expenses", report.Expenses)
	_, _ = fmt.Fprintf(w, "Total revenue: %s\n", report.TotalRevenue.String())
	_, _ = fmt.Fprintf(w, "Total expenses: %s\n", report.TotalExpenses.String())
	_, _ = fmt.Fprintf(w, "Net income: %s\n", report.NetIncome.String())
}

func printCashFlowStatement(w io.Writer, report *reports.CashFlowStatement) {
	_, _ = fmt.Fprintf(w, "Cash flow %s to %s\n", report.StartDate, report.EndDate)
	printCashFlowItems(w, "Operating activities", report.OperatingActivities)
	printCashFlowItems(w, "Investing activities", report.InvestingActivities)
	printCashFlowItems(w, "Financing activities", report.FinancingActivities)
	_, _ = fmt.Fprintf(w, "Total operating: %s\n", report.TotalOperating.String())
	_, _ = fmt.Fprintf(w, "Total investing: %s\n", report.TotalInvesting.String())
	_, _ = fmt.Fprintf(w, "Total financing: %s\n", report.TotalFinancing.String())
	_, _ = fmt.Fprintf(w, "Net cash change: %s\n", report.NetCashChange.String())
	_, _ = fmt.Fprintf(w, "Opening cash: %s\n", report.OpeningCash.String())
	_, _ = fmt.Fprintf(w, "Closing cash: %s\n", report.ClosingCash.String())
}

func printAgingReport(w io.Writer, report *analytics.AgingReport) {
	_, _ = fmt.Fprintf(w, "%s aging as of %s\n", titleLabel(report.ReportType), formatDate(report.AsOfDate))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "BUCKET\tAMOUNT\tCOUNT")
	for _, bucket := range report.Buckets {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\n", bucket.Label, bucket.Amount.String(), bucket.Count)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Total: %s\n", report.Total.String())

	if len(report.ByContact) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w, "By contact:")
	tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTACT\tCURRENT\t1-30\t31-60\t61-90\t90+\tTOTAL")
	for _, contact := range report.ByContact {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			contact.ContactName,
			contact.Current.String(),
			contact.Days1to30.String(),
			contact.Days31to60.String(),
			contact.Days61to90.String(),
			contact.Days90Plus.String(),
			contact.Total.String(),
		)
	}
	_ = tw.Flush()
}

func printBalanceConfirmationSummary(w io.Writer, report *reports.BalanceConfirmationSummary) {
	_, _ = fmt.Fprintf(w, "%s balance confirmations as of %s\n", report.Type, report.AsOfDate)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTACT\tCODE\tEMAIL\tBALANCE\tINVOICES\tOLDEST")
	for _, contact := range report.Contacts {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%d\t%s\n",
			contact.ContactName,
			contact.ContactCode,
			contact.ContactEmail,
			contact.Balance.String(),
			contact.InvoiceCount,
			contact.OldestInvoice,
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Contacts: %d\n", report.ContactCount)
	_, _ = fmt.Fprintf(w, "Invoices: %d\n", report.InvoiceCount)
	_, _ = fmt.Fprintf(w, "Total balance: %s\n", report.TotalBalance.String())
}

func printBalanceConfirmation(w io.Writer, report *reports.BalanceConfirmation) {
	_, _ = fmt.Fprintf(w, "%s balance confirmation for %s as of %s\n", report.Type, report.ContactName, report.AsOfDate)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "INVOICE\tDATE\tDUE\tTOTAL\tPAID\tOUTSTANDING\tOVERDUE")
	for _, invoice := range report.Invoices {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			invoice.InvoiceNumber,
			invoice.InvoiceDate,
			invoice.DueDate,
			invoice.TotalAmount.String(),
			invoice.AmountPaid.String(),
			invoice.OutstandingAmount.String(),
			invoice.DaysOverdue,
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintf(w, "Total balance: %s\n", report.TotalBalance.String())
}

func printAccountBalances(w io.Writer, balances []accounting.AccountBalance) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CODE\tNAME\tTYPE\tDEBIT\tCREDIT\tNET")
	for _, balance := range balances {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			balance.AccountCode,
			balance.AccountName,
			balance.AccountType,
			balance.DebitBalance.String(),
			balance.CreditBalance.String(),
			balance.NetBalance.String(),
		)
	}
	_ = tw.Flush()
}

func printReportSection(w io.Writer, title string, balances []accounting.AccountBalance) {
	_, _ = fmt.Fprintf(w, "%s:\n", title)
	if len(balances) == 0 {
		_, _ = fmt.Fprintln(w, "  -")
		return
	}
	printAccountBalances(w, balances)
}

func printCashFlowItems(w io.Writer, title string, items []reports.CashFlowItem) {
	_, _ = fmt.Fprintf(w, "%s:\n", title)
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, "  -")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CODE\tDESCRIPTION\tAMOUNT")
	for _, item := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Code, item.Description, item.Amount.String())
	}
	_ = tw.Flush()
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02")
}

func titleLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:])
}

func tokenPreview(rawToken string) string {
	if len(rawToken) <= 14 {
		return rawToken
	}
	return rawToken[:14] + "..."
}

func normalizeSelector(selector string) string {
	return strings.ToLower(strings.TrimSpace(selector))
}
