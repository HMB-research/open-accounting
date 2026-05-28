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
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
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
	account := accounting.Account{
		ID:          "account-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: accounting.AccountTypeAsset,
		IsActive:    true,
		Description: "Cash on hand",
	}
	printAccountsTable(&accountBuf, []accounting.Account{account})
	assert.Contains(t, accountBuf.String(), "CODE")
	assert.Contains(t, accountBuf.String(), "1000")

	var accountDetailBuf bytes.Buffer
	printAccount(&accountDetailBuf, &account)
	assert.Contains(t, accountDetailBuf.String(), "Account 1000 Cash")
	assert.Contains(t, accountDetailBuf.String(), "Cash on hand")

	var contactBuf bytes.Buffer
	contact := contacts.Contact{
		ID:          "contact-1",
		Name:        "Acme Corp",
		ContactType: contacts.ContactTypeCustomer,
		Email:       "hello@example.com",
		Phone:       "+372 555 1234",
		CountryCode: "EE",
		IsActive:    true,
	}
	printContactsTable(&contactBuf, []contacts.Contact{contact})
	assert.Contains(t, contactBuf.String(), "NAME")
	assert.Contains(t, contactBuf.String(), "Acme Corp")

	var contactDetailBuf bytes.Buffer
	printContact(&contactDetailBuf, &contact)
	assert.Contains(t, contactDetailBuf.String(), "Contact Acme Corp")
	assert.Contains(t, contactDetailBuf.String(), "hello@example.com")

	var employeeBuf bytes.Buffer
	employee := payroll.Employee{
		ID:                "employee-1",
		EmployeeNumber:    "EMP-001",
		FirstName:         "Mari",
		LastName:          "Maasikas",
		EmploymentType:    payroll.EmploymentFullTime,
		Email:             "mari@example.com",
		StartDate:         now,
		Position:          "Accountant",
		Department:        "Finance",
		FundedPensionRate: decimal.NewFromFloat(0.02),
		IsActive:          true,
	}
	printEmployeesTable(&employeeBuf, []payroll.Employee{employee})
	assert.Contains(t, employeeBuf.String(), "NUMBER")
	assert.Contains(t, employeeBuf.String(), "Mari Maasikas")

	var employeeDetailBuf bytes.Buffer
	printEmployee(&employeeDetailBuf, &employee)
	assert.Contains(t, employeeDetailBuf.String(), "Employee Mari Maasikas")
	assert.Contains(t, employeeDetailBuf.String(), "Position: Accountant")

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

func TestPrintPaymentOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	contactID := "contact-1"
	payment := payments.Payment{
		ID:             "pay-1",
		TenantID:       "tenant-1",
		PaymentNumber:  "PMT-00001",
		PaymentType:    payments.PaymentTypeReceived,
		ContactID:      &contactID,
		PaymentDate:    now,
		Amount:         decimal.NewFromInt(100),
		Currency:       "EUR",
		ExchangeRate:   decimal.NewFromInt(1),
		BaseAmount:     decimal.NewFromInt(100),
		PaymentMethod:  "BANK_TRANSFER",
		BankAccount:    "EE471000001020145685",
		Reference:      "REF-1",
		Notes:          "March receipt",
		JournalEntryID: nil,
		CreatedAt:      now,
		CreatedBy:      "user-1",
		Allocations: []payments.PaymentAllocation{{
			ID:        "alloc-1",
			TenantID:  "tenant-1",
			PaymentID: "pay-1",
			InvoiceID: "inv-1",
			Amount:    decimal.NewFromInt(60),
			CreatedAt: now,
		}},
	}

	var paymentsBuf bytes.Buffer
	printPaymentsTable(&paymentsBuf, []payments.Payment{payment})
	assert.Contains(t, paymentsBuf.String(), "PMT-00001")
	assert.Contains(t, paymentsBuf.String(), "40")

	var paymentBuf bytes.Buffer
	printPayment(&paymentBuf, &payment)
	assert.Contains(t, paymentBuf.String(), "Payment PMT-00001")
	assert.Contains(t, paymentBuf.String(), "Unallocated: 40")
	assert.Contains(t, paymentBuf.String(), "inv-1")
}

func TestPrintInvoiceOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	invoice := invoicing.Invoice{
		ID:            "inv-1",
		TenantID:      "tenant-1",
		InvoiceNumber: "INV-00001",
		InvoiceType:   invoicing.InvoiceTypeSales,
		ContactID:     "contact-1",
		Contact:       &contacts.Contact{Name: "Acme"},
		IssueDate:     now,
		DueDate:       now.AddDate(0, 0, 14),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		Subtotal:      decimal.NewFromInt(180),
		VATAmount:     decimal.NewFromFloat(39.6),
		Total:         decimal.NewFromFloat(219.6),
		BaseSubtotal:  decimal.NewFromInt(180),
		BaseVATAmount: decimal.NewFromFloat(39.6),
		BaseTotal:     decimal.NewFromFloat(219.6),
		AmountPaid:    decimal.NewFromInt(20),
		Status:        invoicing.StatusDraft,
		Reference:     "REF-1",
		Notes:         "March services",
		CreatedAt:     now,
		CreatedBy:     "user-1",
		UpdatedAt:     now,
		Lines: []invoicing.InvoiceLine{{
			LineNumber:   1,
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(2),
			Unit:         "hour",
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(22),
			LineSubtotal: decimal.NewFromInt(180),
			LineVAT:      decimal.NewFromFloat(39.6),
			LineTotal:    decimal.NewFromFloat(219.6),
		}},
	}

	var invoicesBuf bytes.Buffer
	printInvoicesTable(&invoicesBuf, []invoicing.Invoice{invoice})
	assert.Contains(t, invoicesBuf.String(), "INV-00001")
	assert.Contains(t, invoicesBuf.String(), "199.6")

	var invoiceBuf bytes.Buffer
	printInvoice(&invoiceBuf, &invoice)
	assert.Contains(t, invoiceBuf.String(), "Invoice INV-00001")
	assert.Contains(t, invoiceBuf.String(), "Due amount: 199.6")
	assert.Contains(t, invoiceBuf.String(), "Consulting")
}

func TestPrintQuoteOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	validUntil := now.AddDate(0, 0, 30)
	quote := quotes.Quote{
		ID:           "quote-1",
		TenantID:     "tenant-1",
		QuoteNumber:  "QUO-00001",
		ContactID:    "contact-1",
		Contact:      &contacts.Contact{Name: "Acme"},
		QuoteDate:    now,
		ValidUntil:   &validUntil,
		Status:       quotes.QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromInt(180),
		VATAmount:    decimal.NewFromFloat(39.6),
		Total:        decimal.NewFromFloat(219.6),
		Notes:        "March offer",
		CreatedAt:    now,
		CreatedBy:    "user-1",
		UpdatedAt:    now,
		Lines: []quotes.QuoteLine{{
			LineNumber:   1,
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(2),
			Unit:         "hour",
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(22),
			LineSubtotal: decimal.NewFromInt(180),
			LineVAT:      decimal.NewFromFloat(39.6),
			LineTotal:    decimal.NewFromFloat(219.6),
		}},
	}

	var quotesBuf bytes.Buffer
	printQuotesTable(&quotesBuf, []quotes.Quote{quote})
	assert.Contains(t, quotesBuf.String(), "QUO-00001")
	assert.Contains(t, quotesBuf.String(), "Acme")

	var quoteBuf bytes.Buffer
	printQuote(&quoteBuf, &quote)
	assert.Contains(t, quoteBuf.String(), "Quote QUO-00001")
	assert.Contains(t, quoteBuf.String(), "Valid until: 2026-04-14")
	assert.Contains(t, quoteBuf.String(), "Consulting")
}

func TestPrintOrderOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	expectedDelivery := now.AddDate(0, 0, 7)
	quoteID := "quote-1"
	invoiceID := "inv-1"
	order := orders.Order{
		ID:                   "order-1",
		TenantID:             "tenant-1",
		OrderNumber:          "ORD-00001",
		ContactID:            "contact-1",
		Contact:              &contacts.Contact{Name: "Acme"},
		OrderDate:            now,
		ExpectedDelivery:     &expectedDelivery,
		Status:               orders.OrderStatusConfirmed,
		Currency:             "EUR",
		ExchangeRate:         decimal.NewFromInt(1),
		Subtotal:             decimal.NewFromInt(180),
		VATAmount:            decimal.NewFromFloat(39.6),
		Total:                decimal.NewFromFloat(219.6),
		Notes:                "March order",
		QuoteID:              &quoteID,
		ConvertedToInvoiceID: &invoiceID,
		CreatedAt:            now,
		CreatedBy:            "user-1",
		UpdatedAt:            now,
		Lines: []orders.OrderLine{{
			LineNumber:   1,
			Description:  "Consulting",
			Quantity:     decimal.NewFromInt(2),
			Unit:         "hour",
			UnitPrice:    decimal.NewFromInt(100),
			VATRate:      decimal.NewFromInt(22),
			LineSubtotal: decimal.NewFromInt(180),
			LineVAT:      decimal.NewFromFloat(39.6),
			LineTotal:    decimal.NewFromFloat(219.6),
		}},
	}

	var ordersBuf bytes.Buffer
	printOrdersTable(&ordersBuf, []orders.Order{order})
	assert.Contains(t, ordersBuf.String(), "ORD-00001")
	assert.Contains(t, ordersBuf.String(), "Acme")

	var orderBuf bytes.Buffer
	printOrder(&orderBuf, &order)
	assert.Contains(t, orderBuf.String(), "Order ORD-00001")
	assert.Contains(t, orderBuf.String(), "Expected delivery: 2026-03-22")
	assert.Contains(t, orderBuf.String(), "Converted invoice: inv-1")
	assert.Contains(t, orderBuf.String(), "Consulting")
}

func TestPrintAssetOutputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	category := assets.AssetCategory{
		ID:                          "cat-1",
		TenantID:                    "tenant-1",
		Name:                        "Equipment",
		Description:                 "Office equipment",
		DepreciationMethod:          assets.DepreciationStraightLine,
		DefaultUsefulLifeMonths:     60,
		DefaultResidualValuePercent: decimal.NewFromInt(10),
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
	categoryID := "cat-1"
	supplierID := "supplier-1"
	depStart := now
	lastDep := now.AddDate(0, 1, 0)
	asset := assets.FixedAsset{
		ID:                      "asset-1",
		TenantID:                "tenant-1",
		AssetNumber:             "FA-00001",
		Name:                    "Laptop",
		Description:             "Developer laptop",
		CategoryID:              &categoryID,
		Status:                  assets.AssetStatusActive,
		PurchaseDate:            now,
		PurchaseCost:            decimal.NewFromInt(1200),
		SupplierID:              &supplierID,
		SerialNumber:            "SN-1",
		Location:                "Tallinn",
		DepreciationMethod:      assets.DepreciationStraightLine,
		UsefulLifeMonths:        36,
		ResidualValue:           decimal.NewFromInt(100),
		DepreciationStartDate:   &depStart,
		AccumulatedDepreciation: decimal.NewFromInt(50),
		BookValue:               decimal.NewFromInt(1150),
		LastDepreciationDate:    &lastDep,
		CreatedAt:               now,
		CreatedBy:               "user-1",
		UpdatedAt:               now,
	}
	entry := assets.DepreciationEntry{
		ID:                 "dep-1",
		TenantID:           "tenant-1",
		AssetID:            "asset-1",
		DepreciationDate:   now,
		PeriodStart:        now,
		PeriodEnd:          now.AddDate(0, 1, -1),
		DepreciationAmount: decimal.NewFromInt(25),
		AccumulatedTotal:   decimal.NewFromInt(75),
		BookValueAfter:     decimal.NewFromInt(1125),
		CreatedAt:          now,
		CreatedBy:          "user-1",
	}

	var categoriesBuf bytes.Buffer
	printAssetCategoriesTable(&categoriesBuf, []assets.AssetCategory{category})
	assert.Contains(t, categoriesBuf.String(), "Equipment")

	var categoryBuf bytes.Buffer
	printAssetCategory(&categoryBuf, &category)
	assert.Contains(t, categoryBuf.String(), "Office equipment")

	var assetsBuf bytes.Buffer
	printAssetsTable(&assetsBuf, []assets.FixedAsset{asset})
	assert.Contains(t, assetsBuf.String(), "FA-00001")
	assert.Contains(t, assetsBuf.String(), "1150")

	var assetBuf bytes.Buffer
	printAsset(&assetBuf, &asset)
	assert.Contains(t, assetBuf.String(), "Asset FA-00001 Laptop")
	assert.Contains(t, assetBuf.String(), "Serial number: SN-1")

	var depreciationBuf bytes.Buffer
	printDepreciationEntriesTable(&depreciationBuf, []assets.DepreciationEntry{entry})
	assert.Contains(t, depreciationBuf.String(), "dep-1")
	assert.Contains(t, depreciationBuf.String(), "25")
}

func TestPrintPayrollOutputs(t *testing.T) {
	t.Parallel()

	paymentDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	payslip := payroll.Payslip{
		ID:                "payslip-1",
		EmployeeID:        "emp-1",
		GrossSalary:       decimal.NewFromInt(3200),
		NetSalary:         decimal.NewFromFloat(2534.8),
		IncomeTax:         decimal.NewFromInt(550),
		SocialTax:         decimal.NewFromInt(1056),
		TotalEmployerCost: decimal.NewFromFloat(4281.6),
		PaymentStatus:     "PENDING",
		CreatedAt:         now,
		Employee:          &payroll.Employee{FirstName: "Mari", LastName: "Maasikas"},
	}
	run := payroll.PayrollRun{
		ID:                "run-1",
		PeriodYear:        2026,
		PeriodMonth:       3,
		Status:            payroll.PayrollCalculated,
		PaymentDate:       &paymentDate,
		TotalGross:        decimal.NewFromInt(3200),
		TotalNet:          decimal.NewFromFloat(2534.8),
		TotalEmployerCost: decimal.NewFromFloat(4281.6),
		Notes:             "March payroll",
		CreatedAt:         now,
		UpdatedAt:         now,
		Payslips:          []payroll.Payslip{payslip},
	}

	var runsBuf bytes.Buffer
	printPayrollRunsTable(&runsBuf, []payroll.PayrollRun{run})
	assert.Contains(t, runsBuf.String(), "2026-03")
	assert.Contains(t, runsBuf.String(), "CALCULATED")

	var runBuf bytes.Buffer
	printPayrollRun(&runBuf, &run)
	assert.Contains(t, runBuf.String(), "Payroll run 2026-03")
	assert.Contains(t, runBuf.String(), "Mari Maasikas")

	var payslipsBuf bytes.Buffer
	printPayslipsTable(&payslipsBuf, []payroll.Payslip{payslip})
	assert.Contains(t, payslipsBuf.String(), "Mari Maasikas")
	assert.Contains(t, payslipsBuf.String(), "2534.8")

	var taxBuf bytes.Buffer
	printTaxCalculation(&taxBuf, &payroll.TaxCalculation{
		GrossSalary:       decimal.NewFromInt(3200),
		BasicExemption:    decimal.NewFromInt(700),
		TaxableIncome:     decimal.NewFromInt(2500),
		IncomeTax:         decimal.NewFromInt(550),
		UnemploymentEE:    decimal.NewFromFloat(51.2),
		FundedPension:     decimal.NewFromInt(64),
		TotalDeductions:   decimal.NewFromFloat(665.2),
		NetSalary:         decimal.NewFromFloat(2534.8),
		SocialTax:         decimal.NewFromInt(1056),
		UnemploymentER:    decimal.NewFromFloat(25.6),
		TotalEmployerCost: decimal.NewFromFloat(4281.6),
	})
	assert.Contains(t, taxBuf.String(), "Net salary: 2534.8")
	assert.Contains(t, taxBuf.String(), "Total employer cost: 4281.6")

	assert.Equal(t, "emp-2", payslipEmployeeName(payroll.Payslip{EmployeeID: "emp-2"}))
	assert.Equal(t, "2026-03-31", formatDatePtr(&paymentDate))
	assert.Equal(t, "-", formatDatePtr(nil))
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

func TestPrintJournalEntries(t *testing.T) {
	t.Parallel()

	entryDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	entry := accounting.JournalEntry{
		ID:          "je-1",
		EntryNumber: "JE-2026-001",
		EntryDate:   entryDate,
		Description: "Manual accrual",
		Reference:   "ACC-1",
		Status:      accounting.StatusDraft,
		Lines: []accounting.JournalEntryLine{
			{
				AccountID:   "acc-1",
				Description: "Expense",
				DebitAmount: decimal.NewFromInt(100),
				Currency:    "EUR",
				BaseDebit:   decimal.NewFromInt(100),
				Account:     &accounting.Account{Code: "6000", Name: "Expenses"},
			},
			{
				AccountID:    "acc-2",
				Description:  "Accrual",
				CreditAmount: decimal.NewFromInt(100),
				Currency:     "EUR",
				BaseCredit:   decimal.NewFromInt(100),
			},
		},
	}

	var entriesBuf bytes.Buffer
	printJournalEntriesTable(&entriesBuf, []accounting.JournalEntry{entry})
	assert.Contains(t, entriesBuf.String(), "JE-2026-001")
	assert.Contains(t, entriesBuf.String(), "Manual accrual")

	var entryBuf bytes.Buffer
	printJournalEntry(&entryBuf, &entry)
	assert.Contains(t, entryBuf.String(), "Balanced: true")
	assert.Contains(t, entryBuf.String(), "6000 Expenses")
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
