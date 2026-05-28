package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tax"
)

func TestPrintJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := printJSON(&buf, map[string]string{"status": "ok"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "\"status\": \"ok\"")
}

func TestPrintTables(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)

	var tokenBuf bytes.Buffer
	printAPITokensTable(&tokenBuf, []apitoken.APIToken{{
		ID:          "token-1",
		Name:        "CLI",
		TokenPrefix: "oa_tok",
		CreatedAt:   now,
	}})
	assert.Contains(t, tokenBuf.String(), "ID")
	assert.Contains(t, tokenBuf.String(), "CLI")

	var accountBuf bytes.Buffer
	printAccountsTable(&accountBuf, []accounting.Account{{
		ID:          "account-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: accounting.AccountTypeAsset,
		IsActive:    true,
	}})
	assert.Contains(t, accountBuf.String(), "CODE")
	assert.Contains(t, accountBuf.String(), "1000")

	var contactBuf bytes.Buffer
	printContactsTable(&contactBuf, []contacts.Contact{{
		ID:          "contact-1",
		Name:        "Acme Corp",
		ContactType: contacts.ContactTypeCustomer,
		Email:       "hello@example.com",
		IsActive:    true,
	}})
	assert.Contains(t, contactBuf.String(), "NAME")
	assert.Contains(t, contactBuf.String(), "Acme Corp")

	var employeeBuf bytes.Buffer
	printEmployeesTable(&employeeBuf, []payroll.Employee{{
		ID:             "employee-1",
		EmployeeNumber: "EMP-001",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		EmploymentType: payroll.EmploymentFullTime,
		Email:          "mari@example.com",
		IsActive:       true,
	}})
	assert.Contains(t, employeeBuf.String(), "NUMBER")
	assert.Contains(t, employeeBuf.String(), "Mari Maasikas")

	var documentBuf bytes.Buffer
	printDocumentsTable(&documentBuf, []documents.Document{{
		ID:           "doc-1",
		EntityType:   documents.EntityTypeBankTxn,
		EntityID:     "txn-1",
		DocumentType: documents.DocumentTypeReconciliation,
		FileName:     "statement.pdf",
		ReviewStatus: documents.ReviewStatusPending,
		CreatedAt:    now,
	}})
	assert.Contains(t, documentBuf.String(), "ENTITY")
	assert.Contains(t, documentBuf.String(), "statement.pdf")
}

func TestPrintReports(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	balances := []accounting.AccountBalance{{
		AccountID:     "account-1",
		AccountCode:   "1000",
		AccountName:   "Cash",
		AccountType:   accounting.AccountTypeAsset,
		DebitBalance:  decimal.NewFromInt(500),
		CreditBalance: decimal.Zero,
		NetBalance:    decimal.NewFromInt(500),
	}}

	var trialBuf bytes.Buffer
	printTrialBalance(&trialBuf, &accounting.TrialBalance{
		AsOfDate:     asOf,
		Accounts:     balances,
		TotalDebits:  decimal.NewFromInt(500),
		TotalCredits: decimal.NewFromInt(500),
		IsBalanced:   true,
	})
	assert.Contains(t, trialBuf.String(), "Trial balance as of 2026-03-31")
	assert.Contains(t, trialBuf.String(), "1000")

	var accountBalanceBuf bytes.Buffer
	printAccountBalance(&accountBalanceBuf, &accountBalanceReport{
		AccountID: "account-1",
		AsOfDate:  "2026-03-31",
		Balance:   "500.00",
	})
	assert.Contains(t, accountBalanceBuf.String(), "ACCOUNT ID")
	assert.Contains(t, accountBalanceBuf.String(), "500.00")

	var balanceSheetBuf bytes.Buffer
	printBalanceSheet(&balanceSheetBuf, &accounting.BalanceSheet{
		AsOfDate:         asOf,
		Assets:           balances,
		TotalAssets:      decimal.NewFromInt(500),
		TotalLiabilities: decimal.NewFromInt(200),
		TotalEquity:      decimal.NewFromInt(300),
		IsBalanced:       true,
	})
	assert.Contains(t, balanceSheetBuf.String(), "Balance sheet as of 2026-03-31")
	assert.Contains(t, balanceSheetBuf.String(), "Total assets: 500")

	var incomeBuf bytes.Buffer
	printIncomeStatement(&incomeBuf, &accounting.IncomeStatement{
		StartDate:     asOf,
		EndDate:       asOf,
		Revenue:       balances,
		TotalRevenue:  decimal.NewFromInt(1200),
		TotalExpenses: decimal.NewFromInt(700),
		NetIncome:     decimal.NewFromInt(500),
	})
	assert.Contains(t, incomeBuf.String(), "Income statement")
	assert.Contains(t, incomeBuf.String(), "Net income: 500")

	var cashFlowBuf bytes.Buffer
	printCashFlowStatement(&cashFlowBuf, &reports.CashFlowStatement{
		StartDate: "2026-01-01",
		EndDate:   "2026-03-31",
		OperatingActivities: []reports.CashFlowItem{{
			Code:        reports.CFOperTotal,
			Description: "Operating total",
			Amount:      decimal.NewFromInt(500),
		}},
		ClosingCash: decimal.NewFromInt(500),
	})
	assert.Contains(t, cashFlowBuf.String(), "Cash flow 2026-01-01 to 2026-03-31")
	assert.Contains(t, cashFlowBuf.String(), "Closing cash: 500")

	var agingBuf bytes.Buffer
	printAgingReport(&agingBuf, &analytics.AgingReport{
		ReportType: "receivables",
		AsOfDate:   asOf,
		Total:      decimal.NewFromInt(900),
		Buckets: []analytics.AgingBucket{{
			Label:  "Current",
			Amount: decimal.NewFromInt(900),
			Count:  2,
		}},
		ByContact: []analytics.ContactAging{{
			ContactName: "Acme",
			Total:       decimal.NewFromInt(900),
		}},
	})
	assert.Contains(t, agingBuf.String(), "Receivables aging")
	assert.Contains(t, agingBuf.String(), "Acme")

	var confirmationSummaryBuf bytes.Buffer
	printBalanceConfirmationSummary(&confirmationSummaryBuf, &reports.BalanceConfirmationSummary{
		Type:         reports.BalanceTypeReceivable,
		AsOfDate:     "2026-03-31",
		TotalBalance: decimal.NewFromInt(900),
		ContactCount: 1,
		InvoiceCount: 2,
		Contacts: []reports.ContactBalance{{
			ContactName:  "Acme",
			ContactCode:  "CUST-1",
			ContactEmail: "billing@example.com",
			Balance:      decimal.NewFromInt(900),
			InvoiceCount: 2,
		}},
	})
	assert.Contains(t, confirmationSummaryBuf.String(), "RECEIVABLE balance confirmations")
	assert.Contains(t, confirmationSummaryBuf.String(), "Total balance: 900")

	var confirmationBuf bytes.Buffer
	printBalanceConfirmation(&confirmationBuf, &reports.BalanceConfirmation{
		Type:         reports.BalanceTypeReceivable,
		ContactName:  "Acme",
		AsOfDate:     "2026-03-31",
		TotalBalance: decimal.NewFromInt(900),
		Invoices: []reports.BalanceInvoice{{
			InvoiceNumber:     "INV-1",
			InvoiceDate:       "2026-03-01",
			DueDate:           "2026-03-15",
			TotalAmount:       decimal.NewFromInt(1000),
			AmountPaid:        decimal.NewFromInt(100),
			OutstandingAmount: decimal.NewFromInt(900),
			DaysOverdue:       16,
		}},
	})
	assert.Contains(t, confirmationBuf.String(), "INV-1")
	assert.Contains(t, confirmationBuf.String(), "Total balance: 900")
}

func TestPrintTaxReports(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)

	tsd := payroll.TSDDeclaration{
		ID:                  "tsd-1",
		PeriodYear:          2026,
		PeriodMonth:         3,
		TotalPayments:       decimal.NewFromInt(3200),
		TotalIncomeTax:      decimal.NewFromInt(500),
		TotalSocialTax:      decimal.NewFromInt(1056),
		TotalUnemploymentER: decimal.NewFromFloat(25.6),
		TotalUnemploymentEE: decimal.NewFromFloat(51.2),
		TotalFundedPension:  decimal.NewFromInt(64),
		Status:              payroll.TSDDraft,
		CreatedAt:           now,
		UpdatedAt:           now,
		Rows: []payroll.TSDRow{{
			FirstName:     "Mari",
			LastName:      "Maasikas",
			PaymentType:   "10",
			GrossPayment:  decimal.NewFromInt(3200),
			TaxableAmount: decimal.NewFromInt(2500),
			IncomeTax:     decimal.NewFromInt(500),
			SocialTax:     decimal.NewFromInt(1056),
		}},
	}

	var tsdListBuf bytes.Buffer
	printTSDDeclarationsTable(&tsdListBuf, []payroll.TSDDeclaration{tsd})
	assert.Contains(t, tsdListBuf.String(), "2026-03")
	assert.Contains(t, tsdListBuf.String(), "3200")

	var tsdBuf bytes.Buffer
	printTSDDeclaration(&tsdBuf, &tsd)
	assert.Contains(t, tsdBuf.String(), "TSD 2026-03")
	assert.Contains(t, tsdBuf.String(), "Mari Maasikas")

	kmd := tax.KMDDeclaration{
		ID:             "kmd-1",
		Year:           2026,
		Month:          3,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(220),
		TotalInputVAT:  decimal.NewFromInt(80),
		Rows: []tax.KMDRow{{
			Code:        tax.KMDRow1,
			Description: "Taxable sales",
			TaxBase:     decimal.NewFromInt(1000),
			TaxAmount:   decimal.NewFromInt(220),
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}

	var kmdListBuf bytes.Buffer
	printKMDDeclarationsTable(&kmdListBuf, []tax.KMDDeclaration{kmd})
	assert.Contains(t, kmdListBuf.String(), "2026-03")
	assert.Contains(t, kmdListBuf.String(), "140")

	var kmdBuf bytes.Buffer
	printKMDDeclaration(&kmdBuf, &kmd)
	assert.Contains(t, kmdBuf.String(), "KMD 2026-03")
	assert.Contains(t, kmdBuf.String(), "Taxable sales")
}

func TestFormatHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "-", formatTimePtr(nil))

	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, now.Format(time.RFC3339), formatTimePtr(&now))
	assert.Equal(t, "2026-03-12", formatDate(now))
	assert.Equal(t, "-", formatDate(time.Time{}))
	assert.Equal(t, "Receivables", titleLabel("receivables"))

	assert.Equal(t, "oa_token_12345...", tokenPreview("oa_token_1234567890"))
	assert.Equal(t, "short-token", tokenPreview("short-token"))
	assert.Equal(t, "tenant-slug", normalizeSelector("  Tenant-Slug "))
}
