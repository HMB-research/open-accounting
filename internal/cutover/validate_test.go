package cutover

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBundleReportsReadyBundle(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code;account_name;type\n1000;Cash;ASSET\n3000;Owner equity;EQUITY\n4000;Sales;REVENUE\n5500;Office expenses;EXPENSE\n",
		},
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,email,reg_code\nCUST-1,Customer One,ap@example.com,12345678\n",
		},
		{
			Kind:       KindEmployees,
			FileName:   "employees.csv",
			CSVContent: "employee_number,first_name,last_name,email,start_date\nEMP-1,Mari,Maasikas,mari@example.com,2026-01-15\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount\n2026-05-30,Office Store,5500,1000,42\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,product_code\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22,SKU-1\n",
		},
		{
			Kind:       KindEInvoices,
			FileName:   "e-invoices.xml",
			XMLContent: cutoverEInvoiceXML("BILL-2026-001", "Supplier OÜ", "12345678"),
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number\nRECEIVED,2026-05-31,100,INV-1\n",
		},
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,gl_account_code\nMain bank,EE471000001020145685,1000\n",
		},
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,description\n2026-05-31,100,Customer receipt\n",
		},
		{
			Kind:       KindPayrollHistory,
			FileName:   "payroll.csv",
			CSVContent: "year,month,employee_number,gross_salary\n2026,5,EMP-1,2500\n",
		},
		{
			Kind:       KindLeaveBalances,
			FileName:   "leave.csv",
			CSVContent: "year,employee_number,absence_type_code,entitled_days\n2026,EMP-1,ANNUAL,28\n",
		},
		{
			Kind:       KindTSDHistory,
			FileName:   "tsd.csv",
			CSVContent: "year,month,employee_number,gross_payment\n2026,5,EMP-1,2500\n",
		},
		{
			Kind:       KindKMDHistory,
			FileName:   "kmd.csv",
			CSVContent: "year,month,row_code,tax_base,tax_amount\n2026,5,1,100,22\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate,product_code\nQ-1,2026-05-30,CUST-1,Work,1,100,22,SKU-1\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_code,line_description,quantity,unit_price,vat_rate,product_code\nSO-1,2026-05-30,CUST-1,Work,1,100,22,SKU-1\n",
		},
		{
			Kind:       KindRecurringInvoices,
			FileName:   "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate,product_code\nMonthly retainer,MONTHLY,2026-06-01,CUST-1,Work,1,100,22,SKU-1\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name,parent_code\nCC-ROOT,Root,\nCC-SALES,Sales,CC-ROOT\n",
		},
		{
			Kind:       KindCostAllocations,
			FileName:   "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date,notes\nCC-SALES,line-1,125.50,50,2026-05-31,Shared rent\n",
		},
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "category_name,parent_name\nRoot Cat,\nWidgets,Root Cat\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "warehouse_code,warehouse_name\nMAIN,Main warehouse\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "product_code,name,category_name,sales_price,sale_account_code,purchase_account_code,inventory_account_code\nSKU-1,Widget,Widgets,10,4000,5500,1000\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity,batch,serial,expiration_date\nSKU-1,MAIN,5,LOT-1,SN-1,2027-05-30\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,asset_account_code,depreciation_expense_account_code,accumulated_depreciation_account_code\nFA-1,Laptop,2026-05-30,1200,1000,5500,1000\n",
		},
		{
			Kind:       KindOpeningBalances,
			FileName:   "opening.csv",
			CSVContent: "account_code,debit,credit\n1000,100,0\n3000,0,100\n",
		},
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\nJE-1,2026-05-30,1000,100,0\nJE-1,2026-05-30,4000,0,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Equal(t, 32, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)

	var stockValidation FileValidation
	var eInvoiceValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindStockAdjustments {
			stockValidation = file
		}
		if file.Kind == KindEInvoices {
			eInvoiceValidation = file
		}
	}
	require.Equal(t, KindStockAdjustments, stockValidation.Kind)
	assert.Contains(t, stockValidation.Headers, "lot_number")
	assert.Contains(t, stockValidation.Headers, "serial_number")
	assert.Contains(t, stockValidation.Headers, "expiry_date")

	require.Equal(t, KindEInvoices, eInvoiceValidation.Kind)
	assert.Equal(t, 1, eInvoiceValidation.Rows)
	assert.Contains(t, eInvoiceValidation.Headers, "invoice_id")
	assert.Contains(t, eInvoiceValidation.Headers, "invoice_number")
	assert.Contains(t, eInvoiceValidation.Headers, "contact_reg_code")
	assert.Contains(t, eInvoiceValidation.Headers, "buyer_reg_code")
}

func TestValidateBundleAcceptsMeritProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "merit-accounts.csv",
				CSVContent: "konto_kood,konto_nimi,konto_tüüp\n1000,Cash,ASSET\n3000,Equity,EQUITY\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "merit-contacts.csv",
				CSVContent: "kliendi_kood,nimi,registrikood,kmkr_nr\nCUST-1,Customer One,12345678,EE12345678\n",
			},
			{
				Kind:       KindInvoices,
				FileName:   "merit-invoices.csv",
				CSVContent: "arve_nr,arve_tüüp,arve_kuupäev,due_date,kliendi_kood,rea_kirjeldus,kogus,ühiku_hind,käibemaks\nINV-1,SALES,2026-05-30,2026-06-14,CUST-1,Implementation,1,100,22\n",
			},
			{
				Kind:       KindOpeningBalances,
				FileName:   "merit-opening.csv",
				CSVContent: "konto,deebet,kreedit\n1000,100,0\n3000,0,100\n",
			},
			{
				Kind:       KindJournalEntries,
				FileName:   "merit-journal.csv",
				CSVContent: "kanne_nr,kuupäev,konto,deebet,kreedit,selgitus\nJE-1,2026-05-31,1000,50,0,Receipt\nJE-1,2026-05-31,4000,0,50,Receipt\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 5)
	assert.Contains(t, report.Files[0].Headers, "account_type")
	assert.Contains(t, report.Files[2].Headers, "invoice_number")
	assert.Contains(t, report.Files[3].Headers, "account_code")
	assert.Contains(t, report.Files[4].Headers, "entry_reference")
}

func TestValidateBundleAcceptsSmartAccountsProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: "smart-accounts",
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "smartaccounts-accounts.csv",
				CSVContent: "account_no,account_title,classification\n1000,Cash,ASSET\n3000,Equity,EQUITY\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "smartaccounts-clients.csv",
				CSVContent: "client_no,client_name,registration_no,vat_no\nCUST-1,Customer One,12345678,EE12345678\n",
			},
			{
				Kind:       KindInvoices,
				FileName:   "smartaccounts-invoices.csv",
				CSVContent: "document_no,document_type,document_date,due_date,client_no,item_description,qty,unit_price,vat_percent\nINV-1,SALES,2026-05-30,2026-06-14,CUST-1,Implementation,1,100,22\n",
			},
			{
				Kind:       KindOpeningBalances,
				FileName:   "smartaccounts-opening.csv",
				CSVContent: "account_no,debit_amount,credit_amount\n1000,100,0\n3000,0,100\n",
			},
			{
				Kind:       KindJournalEntries,
				FileName:   "smartaccounts-journal.csv",
				CSVContent: "entry_no,transaction_date,account_no,debit_amount,credit_amount,line_memo\nJE-1,2026-05-31,1000,50,0,Receipt\nJE-1,2026-05-31,4000,0,50,Receipt\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 5)
	assert.Contains(t, report.Files[0].Headers, "account_type")
	assert.Contains(t, report.Files[1].Headers, "reg_code")
	assert.Contains(t, report.Files[2].Headers, "invoice_number")
	assert.Contains(t, report.Files[4].Headers, "entry_reference")
}

func TestValidateBundleRejectsUnsupportedProviderPreset(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: "legacy-system",
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name\nSupplier OÜ\n",
			},
		},
	})

	require.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "unsupported provider_preset")
}

func TestValidateBundleRequiresInvoiceTypeForCSVInvoices(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindInvoices, report.Issues[0].Kind)
	assert.Contains(t, report.Issues[0].Message, "missing required column group: invoice_type")
}

func TestValidateBundleReportsGroupedInvoiceHeaderConflicts(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,SALES,CUST-1,2026-05-30,2026-06-14,Setup,1,100,22\n" +
				"INV-1,sale,CUST-2,2026-05-30,2026-06-14,Support,2,50,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindInvoices, report.Issues[0].Kind)
	assert.Equal(t, 3, report.Issues[0].Row)
	assert.Equal(t, "contact_code", report.Issues[0].Field)
	assert.Contains(t, report.Issues[0].Message, `contact_code must be consistent for each invoice_number/invoice_type "INV-1/SALES"`)
}

func TestValidateBundleReportsGroupedCommercialDocumentHeaderConflicts(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindQuotes,
			FileName: "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\n" +
				"Q-1,2026-05-30,CUST-1,Setup,1,100,22\n" +
				"Q-1,2026-05-31,CUST-1,Support,2,50,22\n",
		},
		{
			Kind:     KindOrders,
			FileName: "orders.csv",
			CSVContent: "order_number,order_date,contact_code,line_description,quantity,unit_price,vat_rate\n" +
				"SO-1,2026-05-30,CUST-1,Setup,1,100,22\n" +
				"SO-1,2026-05-31,CUST-1,Support,2,50,22\n",
		},
		{
			Kind:     KindRecurringInvoices,
			FileName: "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate\n" +
				"Monthly,MONTHLY,2026-06-01,CUST-1,Setup,1,100,22\n" +
				"Monthly,QUARTERLY,2026-06-01,CUST-1,Support,2,50,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindQuotes, "quote_date", "quote_date must be consistent for each quote_number")
	assertValidationIssue(t, report, KindOrders, "order_date", "order_date must be consistent for each order_number")
	assertValidationIssue(t, report, KindRecurringInvoices, "frequency", "frequency must be consistent for each template")
}

func TestValidateBundleReportsCommercialDocumentRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,discount_percent,vat_rate,exchange_rate,status,amount_paid,vat_treatment,reverse_charge\n" +
				"INV-BAD,REFUND,,bad-date,2026-05-01,,0,-1,101,-22,0,UNKNOWN,-1,bad,maybe\n" +
				"INV-RC,SALES,CUST-1,2026-05-30,2026-06-14,EU service,\"1 000,5\",100,0,0,1,SENT,0,,true\n",
		},
		{
			Kind:     KindQuotes,
			FileName: "quotes.csv",
			CSVContent: "quote_number,quote_date,valid_until,contact_code,line_description,quantity,unit_price,discount_percent,vat_rate,exchange_rate,status\n" +
				"Q-BAD,2026-06-10,2026-06-01,,Work,abc,-1,-1,-5,abc,NOPE\n",
		},
		{
			Kind:     KindOrders,
			FileName: "orders.csv",
			CSVContent: "order_number,order_date,expected_delivery,contact_code,line_description,quantity,unit_price,discount_percent,vat_rate,exchange_rate,status\n" +
				"SO-BAD,bad-date,bad-date,, ,0,-1,101,-5,0,NOPE\n",
		},
		{
			Kind:     KindRecurringInvoices,
			FileName: "recurring.csv",
			CSVContent: "name,frequency,start_date,end_date,next_generation_date,last_generated_at,contact_code,line_description,quantity,unit_price,discount_percent,vat_rate,payment_terms_days,generated_count,is_active,send_email_on_generation,attach_pdf_to_email\n" +
				",DAILY,bad-date,2026-05-01,bad-date,bad-date,, ,0,-1,101,-5,-1,nope,maybe,perhaps,nah\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 47, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "invoice_type", `invalid invoice_type "REFUND"`)
	assertValidationIssue(t, report, KindInvoices, "contact_code/contact_reg_code/contact_vat_number/contact_email/contact_name", "contact_code or contact_reg_code or contact_vat_number or contact_email or contact_name is required")
	assertValidationIssue(t, report, KindInvoices, "issue_date", "issue_date must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindInvoices, "line_description", "line_description is required")
	assertValidationIssue(t, report, KindInvoices, "quantity", "quantity must be positive")
	assertValidationIssue(t, report, KindInvoices, "unit_price", "unit_price cannot be negative")
	assertValidationIssue(t, report, KindInvoices, "discount_percent", "discount_percent must be between 0 and 100")
	assertValidationIssue(t, report, KindInvoices, "vat_rate", "vat_rate cannot be negative")
	assertValidationIssue(t, report, KindInvoices, "exchange_rate", "exchange_rate must be positive")
	assertValidationIssue(t, report, KindInvoices, "status", `invalid status "UNKNOWN"`)
	assertValidationIssue(t, report, KindInvoices, "amount_paid", "amount_paid cannot be negative")
	assertValidationIssue(t, report, KindInvoices, "reverse_charge", "invalid reverse_charge")
	assertValidationIssue(t, report, KindInvoices, "vat_rate", "reverse charge VAT rate must be positive")
	assertValidationIssue(t, report, KindQuotes, "valid_until", "valid_until cannot be before quote_date")
	assertValidationIssue(t, report, KindQuotes, "quantity", "quantity must be a decimal")
	assertValidationIssue(t, report, KindQuotes, "status", `invalid status "NOPE"`)
	assertValidationIssue(t, report, KindOrders, "order_date", "order_date must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindOrders, "expected_delivery", "expected_delivery must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindOrders, "status", `invalid status "NOPE"`)
	assertValidationIssue(t, report, KindRecurringInvoices, "name", "name is required")
	assertValidationIssue(t, report, KindRecurringInvoices, "frequency", `invalid frequency "DAILY"`)
	assertValidationIssue(t, report, KindRecurringInvoices, "start_date", "start_date must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindRecurringInvoices, "next_generation_date", "next_generation_date must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindRecurringInvoices, "last_generated_at", "last_generated_at must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindRecurringInvoices, "payment_terms_days", "payment_terms_days must be a non-negative integer")
	assertValidationIssue(t, report, KindRecurringInvoices, "generated_count", "generated_count must be a non-negative integer")
	assertValidationIssue(t, report, KindRecurringInvoices, "is_active", "is_active must be true or false")
	assertValidationIssue(t, report, KindRecurringInvoices, "send_email_on_generation", "send_email_on_generation must be true or false")
	assertValidationIssue(t, report, KindRecurringInvoices, "attach_pdf_to_email", "attach_pdf_to_email must be true or false")
}

func TestValidateBundleReportsDuplicateMasterIdentifiers(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "account_code,account_name,type\n" +
				"1000,Cash,ASSET\n" +
				"1000,Duplicate cash,ASSET\n",
		},
		{
			Kind:     KindBankAccounts,
			FileName: "bank-accounts.csv",
			CSVContent: "name,account_number\n" +
				"Main bank,EE471000001020145685\n" +
				"Duplicate main,EE47 1000 0010 2014 5685\n",
		},
		{
			Kind:     KindExpenses,
			FileName: "expenses.csv",
			CSVContent: "expense_number,expense_date,merchant,expense_account_id,payment_account_id,amount\n" +
				"EXP-1,2026-05-31,Office,acc-exp,acc-pay,42\n" +
				"EXP-1,2026-06-01,Office,acc-exp,acc-pay,43\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindAccounts, "code", `code "1000" duplicates row 2`)
	assertValidationIssue(t, report, KindBankAccounts, "account_number", `account_number "EE47 1000 0010 2014 5685" duplicates row 2`)
	assertValidationIssue(t, report, KindExpenses, "expense_number", `expense_number "EXP-1" duplicates row 2`)
}

func TestValidateBundleReportsAccountRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "code,name,account_type,parent_code\n" +
				",Missing Code,ASSET,\n" +
				"1001,,ASSET,\n" +
				"1002,Missing Type,,\n" +
				"1003,Bad Type,SALES,\n" +
				"4000,Revenue,tulu,\n" +
				"5000,Expense,EXPENSE,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindAccounts, "code", "code is required")
	assertValidationIssue(t, report, KindAccounts, "name", "name is required")
	assertValidationIssue(t, report, KindAccounts, "account_type", "account_type is required")
	assertValidationIssue(t, report, KindAccounts, "account_type", `invalid account_type "SALES"`)
}

func TestValidateBundleReportsContactRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindContacts,
			FileName: "contacts.csv",
			CSVContent: "name,contact_type,payment_terms_days,country_code,credit_limit\n" +
				",customer,14,EE,100\n" +
				"Bad Type,partner,14,EE,100\n" +
				"Bad Terms,SUPPLIER,net30,EE,100\n" +
				"Negative Terms,both,-1,EE,100\n" +
				"Bad Country,client,14,EST,100\n" +
				"Bad Credit,vendor,14,EE,not-a-number\n" +
				"Valid Supplier,tarnija,30,ee,\"1500,50\"\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 6, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindContacts, "name", "name is required")
	assertValidationIssue(t, report, KindContacts, "contact_type", `invalid contact_type "partner"`)
	assertValidationIssue(t, report, KindContacts, "payment_terms_days", "payment_terms_days must be a non-negative integer")
	assertValidationIssue(t, report, KindContacts, "country_code", "country_code must be a 2-letter code")
	assertValidationIssue(t, report, KindContacts, "credit_limit", "credit_limit must be a decimal")
}

func TestValidateBundleReportsEmployeeRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindEmployees,
			FileName: "employees.csv",
			CSVContent: "first_name,last_name,start_date,end_date,employment_type,apply_basic_exemption,basic_exemption_amount,funded_pension_rate,base_salary,salary_effective_from,is_active\n" +
				",Maasikas,2026-01-15,2026-02-01,FULL_TIME,true,700,0.02,3200,2026-01-15,true\n" +
				"Mari,,2026-01-15,2026-02-01,FULL_TIME,true,700,0.02,3200,2026-01-15,true\n" +
				"Bad,Start,not-date,,FULL_TIME,true,700,0.02,3200,2026-01-15,true\n" +
				"Bad,End,2026-02-01,2026-01-31,FULL_TIME,true,700,0.02,3200,2026-02-01,true\n" +
				"Bad,Type,2026-01-15,,intern,true,700,0.02,3200,2026-01-15,true\n" +
				"Bad,Bool,2026-01-15,,FULL_TIME,maybe,700,0.02,3200,2026-01-15,nope\n" +
				"Bad,Basic,2026-01-15,,FULL_TIME,true,nope,0.02,3200,2026-01-15,true\n" +
				"Bad,Negative,2026-01-15,,FULL_TIME,true,-1,0.02,3200,2026-01-15,true\n" +
				"Bad,Pension,2026-01-15,,FULL_TIME,true,700,1.2,3200,2026-01-15,true\n" +
				"Bad,Salary,2026-01-15,,FULL_TIME,true,700,0.02,0,2026-01-15,true\n" +
				"Bad,Effective,2026-01-15,,FULL_TIME,true,700,0.02,,2026-01-15,true\n" +
				"Bad,EffectiveDate,2026-01-15,,FULL_TIME,true,700,0.02,3200,not-date,true\n" +
				"Jaan,Tamm,01.02.2026,,too_votuleping,ja,\"700,50\",\"0,02\",\"3 200,00\",2026-02-01,ei\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 13, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindEmployees, "first_name", "first_name is required")
	assertValidationIssue(t, report, KindEmployees, "last_name", "last_name is required")
	assertValidationIssue(t, report, KindEmployees, "start_date", "start_date must be in YYYY-MM-DD format")
	assertValidationIssue(t, report, KindEmployees, "end_date", "end_date cannot be before start_date")
	assertValidationIssue(t, report, KindEmployees, "employment_type", `invalid employment_type "intern"`)
	assertValidationIssue(t, report, KindEmployees, "apply_basic_exemption", "apply_basic_exemption must be true or false")
	assertValidationIssue(t, report, KindEmployees, "is_active", "is_active must be true or false")
	assertValidationIssue(t, report, KindEmployees, "basic_exemption_amount", "basic_exemption_amount must be a decimal")
	assertValidationIssue(t, report, KindEmployees, "basic_exemption_amount", "basic_exemption_amount must be zero or greater")
	assertValidationIssue(t, report, KindEmployees, "funded_pension_rate", "funded_pension_rate must be between 0 and 1")
	assertValidationIssue(t, report, KindEmployees, "base_salary", "base_salary must be greater than zero")
	assertValidationIssue(t, report, KindEmployees, "salary_effective_from", "salary_effective_from requires base_salary")
	assertValidationIssue(t, report, KindEmployees, "salary_effective_from", "salary_effective_from must be in YYYY-MM-DD format")
}

func TestValidateBundleRequiresEmployeeImportColumns(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindEmployees,
			FileName:   "employees.csv",
			CSVContent: "employee_number,name\nEMP-1,Mari Maasikas\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.ElementsMatch(t, []string{"first_name", "last_name", "start_date"}, report.Files[0].MissingColumns)
	require.Len(t, report.Issues, 3)
	messages := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		assert.Equal(t, KindEmployees, issue.Kind)
		assert.Empty(t, issue.Field)
		messages = append(messages, issue.Message)
	}
	assert.ElementsMatch(t, []string{
		"missing required column group: first_name",
		"missing required column group: last_name",
		"missing required column group: start_date",
	}, messages)
}

func TestValidateBundleReportsPayrollHistoryRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindPayrollHistory,
			FileName: "payroll.csv",
			CSVContent: "period_year,period_month,status,payment_date,notes,employee_number,gross_salary,income_tax,unemployment_insurance_employee,funded_pension,other_deductions,net_salary,social_tax,unemployment_insurance_employer,total_employer_cost,basic_exemption_applied,payment_status,paid_at\n" +
				"2019,5,PAID,2026-01-05,May payroll,EMP-1,3200,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,13,PAID,2026-01-05,May payroll,EMP-2,3200,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,DRAFT,2026-01-05,May payroll,EMP-3,3200,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,PAID,bad-date,May payroll,EMP-4,3200,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,PAID,2026-01-05,May payroll,EMP-5,,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,PAID,2026-01-05,May payroll,EMP-6,0,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,PAID,2026-01-05,May payroll,EMP-7,3200,nope,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,PAID,2026-01-05,May payroll,EMP-8,3200,550,51.2,-1,0,2534.8,1056,25.6,4281.6,50,PAID,2026-01-05\n" +
				"2026,5,PAID,2026-01-05,May payroll,EMP-9,3200,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,VOID,2026-01-05\n" +
				"2026,5,PAID,2026-01-05,May payroll,EMP-10,3200,550,51.2,64,0,2534.8,1056,25.6,4281.6,50,PAID,bad-date\n" +
				"2026,6,PAID,2026-07-05,June payroll,EMP-11,\"3 200,00\",\"550,00\",\"51,20\",\"64,00\",0,\"2534,80\",\"1056,00\",\"25,60\",\"4281,60\",\"50,00\",PAID,05.07.2026\n" +
				"2026,6,APPROVED,2026-07-06,Changed notes,EMP-12,2800,420,44.8,56,10,2269.2,924,22.4,3746.4,40,PENDING,2026-07-06\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 13, report.Summary.ErrorCount)
	cancelledPaymentStatusMessage := "payment_status must be PENDING, PAID, or CANCELLED" //nolint:misspell // Existing API/database spelling.
	assertValidationIssue(t, report, KindPayrollHistory, "period_year", "period_year must be between 2020 and 2100")
	assertValidationIssue(t, report, KindPayrollHistory, "period_month", "period_month must be between 1 and 12")
	assertValidationIssue(t, report, KindPayrollHistory, "status", "status must be APPROVED, PAID, or DECLARED")
	assertValidationIssue(t, report, KindPayrollHistory, "payment_date", "payment_date must be in YYYY-MM-DD format")
	assertValidationIssue(t, report, KindPayrollHistory, "gross_salary", "gross_salary is required")
	assertValidationIssue(t, report, KindPayrollHistory, "gross_salary", "gross_salary must be greater than zero")
	assertValidationIssue(t, report, KindPayrollHistory, "income_tax", "income_tax must be a decimal")
	assertValidationIssue(t, report, KindPayrollHistory, "funded_pension", "funded_pension must be zero or greater")
	assertValidationIssue(t, report, KindPayrollHistory, "payment_status", cancelledPaymentStatusMessage)
	assertValidationIssue(t, report, KindPayrollHistory, "paid_at", "paid_at must be in YYYY-MM-DD format")
	assertValidationIssue(t, report, KindPayrollHistory, "status", "status must be consistent for each payroll period")
	assertValidationIssue(t, report, KindPayrollHistory, "payment_date", "payment_date must be consistent for each payroll period")
	assertValidationIssue(t, report, KindPayrollHistory, "notes", "notes must be consistent for each payroll period")
}

func TestValidateBundleReportsLeaveBalanceRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindLeaveBalances,
			FileName: "leave-balances.csv",
			CSVContent: "period_year,employee_number,absence_code,entitlement,carry_over_days,taken_days,reserved_days\n" +
				"2019,EMP-1,ANNUAL,28,0,0,0\n" +
				"2026,EMP-2,ANNUAL,nope,0,0,0\n" +
				"2026,EMP-3,ANNUAL,-1,0,0,0\n" +
				"2026,EMP-4,ANNUAL,28,-1,0,0\n" +
				"2026,EMP-5,ANNUAL,28,0,-0.5,0\n" +
				"2026,EMP-6,ANNUAL,28,0,0,-1\n" +
				"2026,EMP-7,ANNUAL,\"28,5\",\"1 000,5\",10,2\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 6, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindLeaveBalances, "year", "year must be between 2020 and 2100")
	assertValidationIssue(t, report, KindLeaveBalances, "entitled_days", "entitled_days must be a decimal")
	assertValidationIssue(t, report, KindLeaveBalances, "entitled_days", "entitled_days must be zero or greater")
	assertValidationIssue(t, report, KindLeaveBalances, "carryover_days", "carryover_days must be zero or greater")
	assertValidationIssue(t, report, KindLeaveBalances, "used_days", "used_days must be zero or greater")
	assertValidationIssue(t, report, KindLeaveBalances, "pending_days", "pending_days must be zero or greater")
}

func TestValidateBundleReportsTSDHistoryRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindTSDHistory,
			FileName: "tsd-history.csv",
			CSVContent: "declaration_year,declaration_month,declaration_status,submitted_date,emta_ref,employee_number,gross_salary,basic_exemption_applied,taxable_income,income_tax,social_tax,unemployment_employer,unemployment_employee,pension,tsd_payment_type\n" +
				"2019,5,ACCEPTED,2026-01-10,EMTA-MAY,EMP-1,3200,0,3200,0,0,0,0,0,10\n" +
				"2026,13,ACCEPTED,2026-01-10,EMTA-BAD-MONTH,EMP-2,3200,0,3200,0,0,0,0,0,10\n" +
				"2026,5,BAD,2026-01-10,EMTA-BAD-STATUS,EMP-3,3200,0,3200,0,0,0,0,0,10\n" +
				"2026,6,ACCEPTED,bad-date,EMTA-BAD-DATE,EMP-4,3200,0,3200,0,0,0,0,0,10\n" +
				"2026,7,ACCEPTED,2026-01-10,EMTA-MISSING-GROSS,EMP-5,,0,0,0,0,0,0,0,10\n" +
				"2026,8,ACCEPTED,2026-01-10,EMTA-ZERO-GROSS,EMP-6,0,0,0,0,0,0,0,0,10\n" +
				"2026,9,ACCEPTED,2026-01-10,EMTA-BAD-DECIMAL,EMP-7,3200,nope,3200,0,0,0,0,0,10\n" +
				"2026,10,ACCEPTED,2026-01-10,EMTA-NEGATIVE,EMP-8,3200,-1,-1,-1,-1,-1,-1,-1,10\n" +
				"2026,11,ACCEPTED,2026-01-10,EMTA-NOV,EMP-9,3200,0,3200,0,0,0,0,0,10\n" +
				"2026,11,SUBMITTED,2026-01-11,EMTA-CHANGED,EMP-10,3200,0,3200,0,0,0,0,0,10\n" +
				"2026,12,filed,31.12.2026,EMTA-DEC,EMP-11,\"3 200,50\",\"50,50\",\"3 150,00\",0,0,0,0,0,10\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 17, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindTSDHistory, "period_year", "period_year must be between 2020 and 2100")
	assertValidationIssue(t, report, KindTSDHistory, "period_month", "period_month must be between 1 and 12")
	assertValidationIssue(t, report, KindTSDHistory, "status", "status must be DRAFT, SUBMITTED, ACCEPTED, or REJECTED")
	assertValidationIssue(t, report, KindTSDHistory, "submitted_at", "submitted_at must be in YYYY-MM-DD format")
	assertValidationIssue(t, report, KindTSDHistory, "gross_payment", "gross_payment is required")
	assertValidationIssue(t, report, KindTSDHistory, "gross_payment", "gross_payment must be greater than zero")
	assertValidationIssue(t, report, KindTSDHistory, "basic_exemption", "basic_exemption must be a decimal")
	assertValidationIssue(t, report, KindTSDHistory, "basic_exemption", "basic_exemption must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "taxable_amount", "taxable_amount must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "income_tax", "income_tax must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "social_tax", "social_tax must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "unemployment_insurance_employer", "unemployment_insurance_employer must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "unemployment_insurance_employee", "unemployment_insurance_employee must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "funded_pension", "funded_pension must be zero or greater")
	assertValidationIssue(t, report, KindTSDHistory, "status", "status must be consistent for each TSD period")
	assertValidationIssue(t, report, KindTSDHistory, "submitted_at", "submitted_at must be consistent for each TSD period")
	assertValidationIssue(t, report, KindTSDHistory, "emta_reference", "emta_reference must be consistent for each TSD period")
}

func TestValidateBundleReportsKMDHistoryRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindKMDHistory,
			FileName: "kmd-history.csv",
			CSVContent: "declaration_year,declaration_month,declaration_status,submitted_date,row_code,tax_base,tax_amount,total_output_vat,total_input_vat\n" +
				"1899,5,ACCEPTED,2026-01-20,1,100,22,22,0\n" +
				"2026,13,ACCEPTED,2026-01-20,1,100,22,22,0\n" +
				"2026,5,BAD,2026-01-20,1,100,22,22,0\n" +
				"2026,6,ACCEPTED,bad-date,1,100,22,22,0\n" +
				"2026,7,ACCEPTED,2026-01-20,,100,22,22,0\n" +
				"2026,8,ACCEPTED,2026-01-20,1,,,22,0\n" +
				"2026,9,ACCEPTED,2026-01-20,1,nope,22,22,0\n" +
				"2026,10,ACCEPTED,2026-01-20,1,100,nope,22,0\n" +
				"2026,11,ACCEPTED,2026-01-20,1,100,22,nope,0\n" +
				"2026,12,ACCEPTED,2026-01-20,1,100,22,22,nope\n" +
				"2026,4,ACCEPTED,2026-01-20,1,100,22,22,0\n" +
				"2026,4,SUBMITTED,2026-01-21,4,50,11,23,1\n" +
				"2026,3,filed,31.12.2026,row_4,\"1 000,50\",\"220,11\",\"220,11\",0\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 14, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindKMDHistory, "year", "year must be between 1900 and 2200")
	assertValidationIssue(t, report, KindKMDHistory, "month", "month must be between 1 and 12")
	assertValidationIssue(t, report, KindKMDHistory, "status", "status must be DRAFT, SUBMITTED, or ACCEPTED")
	assertValidationIssue(t, report, KindKMDHistory, "submitted_at", "submitted_at must be in YYYY-MM-DD format")
	assertValidationIssue(t, report, KindKMDHistory, "row_code", "row_code is required")
	assertValidationIssue(t, report, KindKMDHistory, "tax_base", "tax_base or tax_amount is required")
	assertValidationIssue(t, report, KindKMDHistory, "tax_base", "tax_base must be a decimal")
	assertValidationIssue(t, report, KindKMDHistory, "tax_amount", "tax_amount must be a decimal")
	assertValidationIssue(t, report, KindKMDHistory, "total_output_vat", "total_output_vat must be a decimal")
	assertValidationIssue(t, report, KindKMDHistory, "total_input_vat", "total_input_vat must be a decimal")
	assertValidationIssue(t, report, KindKMDHistory, "status", "status must be consistent for each KMD period")
	assertValidationIssue(t, report, KindKMDHistory, "submitted_at", "submitted_at must be consistent for each KMD period")
	assertValidationIssue(t, report, KindKMDHistory, "total_output_vat", "total_output_vat must be consistent for each KMD period")
	assertValidationIssue(t, report, KindKMDHistory, "total_input_vat", "total_input_vat must be consistent for each KMD period")
}

func TestValidateBundleReportsExpenseRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindExpenses,
			FileName: "expenses.csv",
			CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount,exchange_rate,status,requires_receipt,submitted_at,approved_at,rejected_at,rejection_reason\n" +
				"EXP-1,bad-date,, , ,0,0,POSTED,maybe,,,,\n" +
				"EXP-2,2026-05-31,Office,5500,1000,abc,abc,UNKNOWN,true,,,,\n" +
				"EXP-3,2026-05-31,Office,5500,1000,10,1,REJECTED,true,bad-date,,bad-date,\n" +
				"EXP-4,2026-05-31T12:00:00Z,Office,5500,1000,10,1,APPROVED,false,bad-date,bad-date,,\n" +
				"EXP-5,2026-05-31,Office,5500,1000,\"10,50\",\"1,2\",SUBMITTED,no,2026-05-31,,,\n" +
				"EXP-6,,Office,5500,1000,10,1,DRAFT,true,,,,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 17, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindExpenses, "expense_date", "expense_date must be YYYY-MM-DD or RFC3339")
	assertValidationIssue(t, report, KindExpenses, "expense_date", "expense_date is required")
	assertValidationIssue(t, report, KindExpenses, "merchant", "merchant is required")
	assertValidationIssue(t, report, KindExpenses, "expense_account_id/expense_account_code", "expense_account_id or expense_account_code is required")
	assertValidationIssue(t, report, KindExpenses, "payment_account_id/payment_account_code", "payment_account_id or payment_account_code is required")
	assertValidationIssue(t, report, KindExpenses, "amount", "amount must be positive")
	assertValidationIssue(t, report, KindExpenses, "amount", "amount must be a decimal")
	assertValidationIssue(t, report, KindExpenses, "exchange_rate", "exchange_rate must be positive")
	assertValidationIssue(t, report, KindExpenses, "exchange_rate", "exchange_rate must be a decimal")
	assertValidationIssue(t, report, KindExpenses, "status", "posted expenses must be imported as approved and posted through the expense workflow")
	assertValidationIssue(t, report, KindExpenses, "status", `invalid status "UNKNOWN"`)
	assertValidationIssue(t, report, KindExpenses, "requires_receipt", "invalid requires_receipt")
	assertValidationIssue(t, report, KindExpenses, "rejection_reason", "rejection_reason is required")
	assertValidationIssue(t, report, KindExpenses, "submitted_at", "submitted_at must be YYYY-MM-DD or RFC3339")
	assertValidationIssue(t, report, KindExpenses, "approved_at", "approved_at must be YYYY-MM-DD or RFC3339")
	assertValidationIssue(t, report, KindExpenses, "rejected_at", "rejected_at must be YYYY-MM-DD or RFC3339")
}

func TestValidateBundleReportsPaymentRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_number,payment_type,payment_date,amount,exchange_rate,allocation_amount,invoice_number\n" +
				"PAY-1,REFUND,bad-date,0,-1,10,\n" +
				"PAY-2,RECEIVED,2026-05-31,100,1,150,INV-1\n" +
				"PAY-3,MADE,2026-05-31T12:00:00Z,100,,50,INV-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 6, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindPayments, "payment_type", `invalid payment_type "REFUND"`)
	assertValidationIssue(t, report, KindPayments, "payment_date", "payment_date must be YYYY-MM-DD or RFC3339")
	assertValidationIssue(t, report, KindPayments, "amount", "amount must be positive")
	assertValidationIssue(t, report, KindPayments, "exchange_rate", "exchange_rate must be positive")
	assertValidationIssue(t, report, KindPayments, "allocation_amount", "invoice_id or invoice_number is required when allocation_amount is provided")
	assertValidationIssue(t, report, KindPayments, "allocation_amount", "allocation_amount exceeds payment amount")
}

func TestValidateBundleReportsBankAccountRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindBankAccounts,
			FileName: "bank-accounts.csv",
			CSVContent: "name,account_number,currency,is_default,is_active\n" +
				",EE1,EUR,true,true\n" +
				"Missing number,,EUR,false,true\n" +
				"Invalid bools,EE2,EUR,maybe,nope\n" +
				"Invalid currency,EE3,EURO,yes,no\n" +
				"Valid defaults,EE4,,y,n\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 5, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindBankAccounts, "name", "name is required")
	assertValidationIssue(t, report, KindBankAccounts, "account_number", "account_number is required")
	assertValidationIssue(t, report, KindBankAccounts, "is_default", "is_default must be true or false")
	assertValidationIssue(t, report, KindBankAccounts, "is_active", "is_active must be true or false")
	assertValidationIssue(t, report, KindBankAccounts, "currency", "currency must be a 3-letter ISO code")
}

func TestValidateBundleReportsBankTransactionRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindBankTransactions,
			FileName: "bank.csv",
			CSVContent: "date,amount,description\n" +
				"bad-date,100,Invalid date\n" +
				"2026-05-31,not-a-number,Invalid amount\n" +
				",50,Missing date\n" +
				"2026-06-01,,Missing amount\n" +
				"2026-06-02,-12.50,Valid outflow\n" +
				"2026-06-03,0,Zero adjustment\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindBankTransactions, "date", "date must be YYYY-MM-DD")
	assertValidationIssue(t, report, KindBankTransactions, "amount", "amount must be a decimal")
	assertValidationIssue(t, report, KindBankTransactions, "date", "date is required")
	assertValidationIssue(t, report, KindBankTransactions, "amount", "amount is required")
}

func TestValidateBundleReportsOpeningBalanceBalanceIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n3000,Owner equity,EQUITY\n",
		},
		{
			Kind:       KindOpeningBalances,
			FileName:   "opening.csv",
			CSVContent: "account_code,debit,credit\n1000,100,0\n3000,0,90\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindOpeningBalances, report.Issues[0].Kind)
	assert.Equal(t, "debit/credit", report.Issues[0].Field)
	assert.Equal(t, "debits=100 credits=90", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "opening balances do not balance")
}

func TestValidateBundleReportsOpeningBalanceAccountCodeRowIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindOpeningBalances,
			FileName:   "opening.csv",
			CSVContent: "account_code,debit,credit\n,100,0\n3000,0,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindOpeningBalances, "account_code", "account_code is required")
}

func TestValidateBundleReportsHistoricalJournalBalanceIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
		},
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\nJE-1,2026-05-30,1000,100,0\nJE-1,2026-05-30,4000,0,90\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindJournalEntries, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "entry_reference/debit/credit", report.Issues[0].Field)
	assert.Equal(t, "JE-1", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "does not balance")
}

func TestValidateBundleReportsHistoricalJournalAmountIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
		},
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\nJE-1,2026-05-30,1000,not-a-number,0\nJE-1,2026-05-30,4000,0,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindJournalEntries, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "debit", report.Issues[0].Field)
	assert.Equal(t, "not-a-number", report.Issues[0].Value)
	assert.Equal(t, "invalid debit", report.Issues[0].Message)
}

func TestValidateBundleReportsHistoricalJournalAccountCodeRowIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\nJE-1,2026-05-30,,100,0\nJE-1,2026-05-30,4000,0,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindJournalEntries, "account_code", "account_code is required")
}

func TestValidateBundleAcceptsJournalImportAliasesAndExchangeRateBalance(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
		},
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "voucher_number,posting_date,code,debit_amount,credit_amount,exchange_rate\nJE-FX,2026-05-30,1000,100,0,1.1\nJE-FX,2026-05-30,4000,0,110,1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsExpenseAccountReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount\n2026-05-31,Office Store,5500,1000,42\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindExpenses, report.Issues[0].Kind)
	assert.Equal(t, KindAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "expense_account_code", report.Issues[0].Field)
	assert.Equal(t, "5500", report.Issues[0].Value)
}

func TestValidateBundleReportsBankAccountReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n",
		},
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,iban,ledger_account_code\nReserve bank,EE999,9999\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindBankAccounts, report.Issues[0].Kind)
	assert.Equal(t, KindAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "gl_account_id/gl_account_code", report.Issues[0].Field)
	assert.Equal(t, "9999", report.Issues[0].Value)
}

func TestValidateBundleReportsBankTransactionSourceAccountReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nMain bank,EE471000001020145685,EUR\n",
		},
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,description,source_account,currency\n2026-05-31,100,Customer receipt,EE999,EUR\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindBankTransactions, report.Issues[0].Kind)
	assert.Equal(t, KindBankAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "source_account", report.Issues[0].Field)
	assert.Equal(t, "EE999", report.Issues[0].Value)
}

func TestValidateBundleReportsBankTransactionCurrencyMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nMain bank,EE471000001020145685,EUR\n",
		},
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,details,client_account,currency\n2026-05-31,100,Customer receipt,EE471000001020145685,USD\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindBankTransactions, report.Issues[0].Kind)
	assert.Equal(t, KindBankAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "source_account/currency", report.Issues[0].Field)
	assert.Equal(t, "EE471000001020145685/USD", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `currency "USD"`)
}

func TestValidateBundleAcceptsBankTransactionStatementAccountAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number\nMain bank,EE47 1000 0010 2014 5685\n",
		},
		{
			Kind:       KindBankTransactions,
			FileName:   "lhv-bank.csv",
			CSVContent: "date,sum,details,client_account,currency,payment_reference,counterparty_iban,entry_reference\n2026-05-31,100,Customer receipt,EE471000001020145685,EUR,REF-1,EE111,ext-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)

	var bankValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindBankTransactions {
			bankValidation = file
		}
	}
	require.Equal(t, KindBankTransactions, bankValidation.Kind)
	assert.Contains(t, bankValidation.Headers, "source_account")
	assert.Contains(t, bankValidation.Headers, "amount")
	assert.Contains(t, bankValidation.Headers, "reference")
	assert.Contains(t, bankValidation.Headers, "counterparty_account")
	assert.Contains(t, bankValidation.Headers, "external_id")
}

func TestValidateBundleReportsAccountParentReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "account_code,account_name,type,parent_code\n" +
				"1000,Cash,ASSET,\n" +
				"1100,Petty Cash,ASSET,9999\n" +
				"1200,Self Parent,ASSET,1200\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Equal(t, KindAccounts, report.Issues[0].Kind)
	assert.Equal(t, KindAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "parent_code", report.Issues[0].Field)
	assert.Equal(t, "9999", report.Issues[0].Value)
	assert.Equal(t, KindAccounts, report.Issues[1].Kind)
	assert.Equal(t, KindAccounts, report.Issues[1].TargetKind)
	assert.Equal(t, "parent_code", report.Issues[1].Field)
	assert.Equal(t, "1200", report.Issues[1].Value)
	assert.Contains(t, report.Issues[1].Message, "cannot reference")
}

func TestValidateBundleReportsHierarchySelfReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name,parent_code\nOPS,Operations,OPS\n",
		},
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "name,parent_name\nWidgets,Widgets\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Equal(t, KindProductCategories, report.Issues[0].Kind)
	assert.Equal(t, "parent_name", report.Issues[0].Field)
	assert.Equal(t, "Widgets", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "cannot reference")
	assert.Equal(t, KindCostCenters, report.Issues[1].Kind)
	assert.Equal(t, "parent_code", report.Issues[1].Field)
	assert.Equal(t, "OPS", report.Issues[1].Value)
	assert.Contains(t, report.Issues[1].Message, "cannot reference")
}

func TestValidateBundleReportsCostAllocationReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\n",
		},
		{
			Kind:       KindCostAllocations,
			FileName:   "cost-allocations.csv",
			CSVContent: "cc_code,journal_line_id,allocation_amount,allocation_date\nMISSING,line-1,125.50,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindCostAllocations, report.Issues[0].Kind)
	assert.Equal(t, KindCostCenters, report.Issues[0].TargetKind)
	assert.Equal(t, "MISSING", report.Issues[0].Value)
}

func TestValidateBundleReportsCostCenterRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindCostCenters,
			FileName: "cost-centers.csv",
			CSVContent: "code,name,budget_amount,budget_period,status,is_active\n" +
				",,nope,WEEKLY,archived,maybe\n" +
				"OPS,Operations,-1,MONTHLY,,maybe\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date\n" +
				",,0,101,2026/05/30\n" +
				"OPS,line-1,nope,nope,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 15, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindCostCenters, "code", "code is required")
	assertValidationIssue(t, report, KindCostCenters, "name", "name is required")
	assertValidationIssue(t, report, KindCostCenters, "budget_amount", "budget_amount must be a decimal")
	assertValidationIssue(t, report, KindCostCenters, "budget_amount", "budget_amount cannot be negative")
	assertValidationIssue(t, report, KindCostCenters, "budget_period", "invalid budget_period")
	assertValidationIssue(t, report, KindCostCenters, "status", "invalid status")
	assertValidationIssue(t, report, KindCostCenters, "is_active", "is_active must be true or false")
	assertValidationIssue(t, report, KindCostAllocations, "cost_center_id/cost_center_code", "cost_center_id or cost_center_code is required")
	assertValidationIssue(t, report, KindCostAllocations, "journal_entry_line_id", "journal_entry_line_id is required")
	assertValidationIssue(t, report, KindCostAllocations, "amount", "amount must be greater than zero")
	assertValidationIssue(t, report, KindCostAllocations, "amount", "amount must be a decimal")
	assertValidationIssue(t, report, KindCostAllocations, "allocation_percentage", "allocation_percentage must be between 0 and 100")
	assertValidationIssue(t, report, KindCostAllocations, "allocation_percentage", "allocation_percentage must be a decimal")
	assertValidationIssue(t, report, KindCostAllocations, "allocation_date", "allocation_date must use YYYY-MM-DD")
	assertValidationIssue(t, report, KindCostAllocations, "allocation_date", "allocation_date is required")
}

func TestValidateBundleReportsTSDHistoryEmployeeReferenceIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindEmployees,
			FileName:   "employees.csv",
			CSVContent: "employee_number,first_name,last_name,start_date\nEMP-1,Mari,Maasikas,2026-01-15\n",
		},
		{
			Kind:       KindTSDHistory,
			FileName:   "tsd.csv",
			CSVContent: "year,month,employee_number,gross_payment\n2026,5,EMP-404,2500\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindTSDHistory, report.Issues[0].Kind)
	assert.Equal(t, KindEmployees, report.Issues[0].TargetKind)
	assert.Equal(t, "EMP-404", report.Issues[0].Value)
}

func TestValidateBundleReportsEmployeeReferenceRequiresCompleteNameColumns(t *testing.T) {
	tests := []struct {
		name        string
		kind        FileKind
		csvContent  string
		missingText string
	}{
		{
			name:        "payroll history",
			kind:        KindPayrollHistory,
			csvContent:  "year,month,first_name,gross_salary\n2026,5,Mari,2500\n",
			missingText: "employee_number|personal_code|email|name|last_name",
		},
		{
			name:        "leave balances",
			kind:        KindLeaveBalances,
			csvContent:  "year,first_name,absence_type_code\n2026,Mari,ANNUAL\n",
			missingText: "employee_number|personal_code|email|name|last_name",
		},
		{
			name:        "tsd history",
			kind:        KindTSDHistory,
			csvContent:  "year,month,first_name,gross_payment\n2026,5,Mari,2500\n",
			missingText: "employee_number|personal_code|email|name|last_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
				{
					Kind:       tt.kind,
					FileName:   string(tt.kind) + ".csv",
					CSVContent: tt.csvContent,
				},
			}})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.False(t, report.Summary.Ready)
			assert.Equal(t, 1, report.Summary.ErrorCount)
			require.Len(t, report.Files, 1)
			assert.Contains(t, report.Files[0].MissingColumns, tt.missingText)
			require.Len(t, report.Issues, 1)
			assert.Contains(t, report.Issues[0].Message, tt.missingText)
		})
	}
}

func TestValidateBundleAcceptsEmployeeReferenceNameColumns(t *testing.T) {
	tests := []struct {
		name       string
		kind       FileKind
		csvContent string
	}{
		{
			name:       "payroll history full name column",
			kind:       KindPayrollHistory,
			csvContent: "year,month,name,gross_salary\n2026,5,Mari Maasikas,2500\n",
		},
		{
			name:       "payroll history first and last name columns",
			kind:       KindPayrollHistory,
			csvContent: "year,month,first_name,last_name,gross_salary\n2026,5,Mari,Maasikas,2500\n",
		},
		{
			name:       "leave balances full name column",
			kind:       KindLeaveBalances,
			csvContent: "year,name,absence_type_code\n2026,Mari Maasikas,ANNUAL\n",
		},
		{
			name:       "tsd history full name column",
			kind:       KindTSDHistory,
			csvContent: "year,month,name,gross_payment\n2026,5,Mari Maasikas,2500\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
				{
					Kind:       tt.kind,
					FileName:   string(tt.kind) + ".csv",
					CSVContent: tt.csvContent,
				},
			}})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.True(t, report.Summary.Ready)
			assert.Empty(t, report.Issues)
			require.Len(t, report.Files, 1)
			assert.Empty(t, report.Files[0].MissingColumns)
		})
	}
}

func TestValidateBundleReportsInventoryReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "name\nHardware\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,category_name,sales_price\nSKU-1,Widget,Missing,10\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "code,name\nMAIN,Main warehouse\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity\nSKU-404,NOPE,1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 3)
	assert.Equal(t, KindProductCategories, report.Issues[0].TargetKind)
	assert.Equal(t, "Missing", report.Issues[0].Value)
	assert.Equal(t, KindProducts, report.Issues[1].TargetKind)
	assert.Equal(t, "SKU-404", report.Issues[1].Value)
	assert.Equal(t, KindWarehouses, report.Issues[2].TargetKind)
	assert.Equal(t, "NOPE", report.Issues[2].Value)
}

func TestValidateBundleReportsProductAccountReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "id,account_code,account_name,type\nacc-sales,4000,Sales,REVENUE\n",
		},
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "id,name\ncat-1,Hardware\n",
		},
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,category_id,sales_price,sale_account_id,purchase_account_code,inventory_account_code\n" +
				"SKU-1,Widget,cat-missing,10,missing-sales,5999,4000\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 3)
	assert.Equal(t, KindProductCategories, report.Issues[0].TargetKind)
	assert.Equal(t, "category_id/category_name", report.Issues[0].Field)
	assert.Equal(t, "cat-missing", report.Issues[0].Value)
	assert.Equal(t, KindAccounts, report.Issues[1].TargetKind)
	assert.Equal(t, "sale_account_id/sale_account_code", report.Issues[1].Field)
	assert.Equal(t, "missing-sales", report.Issues[1].Value)
	assert.Equal(t, KindAccounts, report.Issues[2].TargetKind)
	assert.Equal(t, "purchase_account_id/purchase_account_code", report.Issues[2].Field)
	assert.Equal(t, "5999", report.Issues[2].Value)
}

func TestValidateBundleReportsProductCategoryRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProductCategories,
			FileName: "categories.csv",
			CSVContent: "category_name,parent_name,description\n" +
				",,Missing name\n" +
				"Child,Parent,\n" +
				"Parent,,\n" +
				"Grandchild,Parent,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindProductCategories, "name", "name is required")
	assertValidationIssue(t, report, KindProductCategories, "parent_name", "parent_name must reference an earlier product category row")
}

func TestValidateBundleReportsInventoryRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,product_type,sales_price,purchase_price,vat_rate,min_stock_level,reorder_point,track_inventory,status,is_active,lead_time_days\n" +
				"SKU-1,,asset,-1,nope,-22,-5,-1,sometimes,archived,maybe,-3\n",
		},
		{
			Kind:     KindWarehouses,
			FileName: "warehouses.csv",
			CSVContent: "code,name,is_default,status,is_active\n" +
				",Main,sometimes,closed,maybe\n" +
				"W-2,,true,,maybe\n",
		},
		{
			Kind:     KindStockAdjustments,
			FileName: "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity,unit_cost,expiry_date\n" +
				",,0,-1,2026/01/01\n" +
				"SKU-1,W-2,nope,abc,not-date\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 23, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindProducts, "name", "name is required")
	assertValidationIssue(t, report, KindProducts, "product_type", "invalid product_type")
	assertValidationIssue(t, report, KindProducts, "sales_price", "sales_price cannot be negative")
	assertValidationIssue(t, report, KindProducts, "purchase_price", "purchase_price must be a decimal")
	assertValidationIssue(t, report, KindProducts, "vat_rate", "vat_rate cannot be negative")
	assertValidationIssue(t, report, KindProducts, "min_stock_level", "min_stock_level cannot be negative")
	assertValidationIssue(t, report, KindProducts, "reorder_point", "reorder_point cannot be negative")
	assertValidationIssue(t, report, KindProducts, "track_inventory", "track_inventory must be true or false")
	assertValidationIssue(t, report, KindProducts, "status", "invalid status")
	assertValidationIssue(t, report, KindProducts, "lead_time_days", "lead_time_days cannot be negative")
	assertValidationIssue(t, report, KindWarehouses, "code", "code is required")
	assertValidationIssue(t, report, KindWarehouses, "name", "name is required")
	assertValidationIssue(t, report, KindWarehouses, "is_default", "is_default must be true or false")
	assertValidationIssue(t, report, KindWarehouses, "status", "invalid status")
	assertValidationIssue(t, report, KindWarehouses, "is_active", "is_active must be true or false")
	assertValidationIssue(t, report, KindStockAdjustments, "product_id/product_code", "product_id or product_code is required")
	assertValidationIssue(t, report, KindStockAdjustments, "warehouse_id/warehouse_code", "warehouse_id or warehouse_code is required")
	assertValidationIssue(t, report, KindStockAdjustments, "quantity", "quantity must not be zero")
	assertValidationIssue(t, report, KindStockAdjustments, "quantity", "quantity must be a decimal")
	assertValidationIssue(t, report, KindStockAdjustments, "unit_cost", "unit_cost cannot be negative")
	assertValidationIssue(t, report, KindStockAdjustments, "unit_cost", "unit_cost must be a decimal")
	assertValidationIssue(t, report, KindStockAdjustments, "expiry_date", "expiry_date must use YYYY-MM-DD")
}

func TestValidateBundleReportsCommercialDocumentProductReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "product_code,name,sales_price\nSKU-1,Widget,10\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_name,line_description,quantity,unit_price,vat_rate,product_code\nQ-1,2026-05-30,Customer One,Work,1,100,22,SKU-404\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindQuotes, report.Issues[0].Kind)
	assert.Equal(t, KindProducts, report.Issues[0].TargetKind)
	assert.Equal(t, "product_id/product_code", report.Issues[0].Field)
	assert.Equal(t, "SKU-404", report.Issues[0].Value)
}

func TestValidateBundleReportsOrderQuoteReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "id,quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\n11111111-1111-1111-1111-111111111111,Q-1,2026-05-30,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_code,quote_id,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31,CUST-1,22222222-2222-2222-2222-222222222222,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindOrders, report.Issues[0].Kind)
	assert.Equal(t, KindQuotes, report.Issues[0].TargetKind)
	assert.Equal(t, "quote_id", report.Issues[0].Field)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", report.Issues[0].Value)
}

func TestValidateBundleAcceptsOrderQuoteIDReferences(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_id,quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\n11111111-1111-1111-1111-111111111111,Q-1,2026-05-30,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_code,quote_id,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31,CUST-1,11111111-1111-1111-1111-111111111111,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)

	var quoteValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindQuotes {
			quoteValidation = file
		}
	}
	require.Equal(t, KindQuotes, quoteValidation.Kind)
	assert.Contains(t, quoteValidation.Headers, "id")
}

func TestValidateBundleReportsFixedAssetAccountReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,asset_account_code\nFA-1,Laptop,2026-05-30,1200,9999\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "asset_account_id/asset_account_code", report.Issues[0].Field)
	assert.Equal(t, "9999", report.Issues[0].Value)
}

func TestValidateBundleReportsFixedAssetRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindFixedAssets,
			FileName: "assets.csv",
			CSVContent: "asset_number,name,status,purchase_date,purchase_cost,depreciation_method,useful_life_months,residual_value,accumulated_depreciation,book_value,depreciation_start_date,last_depreciation_date,disposal_date,disposal_method,disposal_proceeds\n" +
				"FA-1,,retired,2026/05/30,nope,accelerated,abc,nope,nope,nope,not-date,2026/06/01,bad-date,recycled,nope\n" +
				"FA-2,Lathe,ACTIVE,2026-05-30T00:00:00Z,0,DECLINING BALANCE,0,-1,-1,-1,2026-06-01 15:04:05,2026-06-02,2026-06-03,SOLD,-1\n" +
				"FA-3,Desk,DRAFT,2026-05-30,100,STRAIGHT_LINE,12,10,95,90,,,,,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 22, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindFixedAssets, "name", "name is required")
	assertValidationIssue(t, report, KindFixedAssets, "status", "invalid status")
	assertValidationIssue(t, report, KindFixedAssets, "purchase_date", "purchase_date must be a date")
	assertValidationIssue(t, report, KindFixedAssets, "purchase_cost", "purchase_cost must be a decimal")
	assertValidationIssue(t, report, KindFixedAssets, "purchase_cost", "purchase cost must be positive")
	assertValidationIssue(t, report, KindFixedAssets, "depreciation_method", "invalid depreciation_method")
	assertValidationIssue(t, report, KindFixedAssets, "useful_life_months", "useful_life_months must be an integer")
	assertValidationIssue(t, report, KindFixedAssets, "useful_life_months", "useful_life_months must be positive")
	assertValidationIssue(t, report, KindFixedAssets, "residual_value", "residual_value must be a decimal")
	assertValidationIssue(t, report, KindFixedAssets, "residual_value", "residual value cannot be negative")
	assertValidationIssue(t, report, KindFixedAssets, "accumulated_depreciation", "accumulated_depreciation must be a decimal")
	assertValidationIssue(t, report, KindFixedAssets, "accumulated_depreciation", "accumulated_depreciation cannot be negative")
	assertValidationIssue(t, report, KindFixedAssets, "accumulated_depreciation", "accumulated_depreciation cannot exceed depreciable amount")
	assertValidationIssue(t, report, KindFixedAssets, "book_value", "book_value must be a decimal")
	assertValidationIssue(t, report, KindFixedAssets, "book_value", "book_value cannot be negative")
	assertValidationIssue(t, report, KindFixedAssets, "book_value", "book_value must equal purchase_cost minus accumulated_depreciation")
	assertValidationIssue(t, report, KindFixedAssets, "depreciation_start_date", "depreciation_start_date must be a date")
	assertValidationIssue(t, report, KindFixedAssets, "last_depreciation_date", "last_depreciation_date must be a date")
	assertValidationIssue(t, report, KindFixedAssets, "disposal_date", "disposal_date must be a date")
	assertValidationIssue(t, report, KindFixedAssets, "disposal_method", "invalid disposal_method")
	assertValidationIssue(t, report, KindFixedAssets, "disposal_proceeds", "disposal_proceeds must be a decimal")
	assertValidationIssue(t, report, KindFixedAssets, "disposal_proceeds", "disposal_proceeds cannot be negative")
}

func TestValidateBundleReportsMissingColumnsAndReferences(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,vat_rate\nINV-1,SALES,CUST-404,2026-05-30,2026-06-14,Work,1,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Contains(t, report.Issues[0].Message, "missing required column group: unit_price")
	assert.Equal(t, 2, report.Issues[1].Row)
	assert.Equal(t, "CUST-404", report.Issues[1].Value)
	assert.Equal(t, KindContacts, report.Issues[1].TargetKind)
}

func TestValidateBundleAcceptsEInvoiceXMLAndPaymentReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "name,reg_code\nSupplier OÜ,12345678\n",
		},
		{
			Kind:       KindEInvoices,
			FileName:   "e-invoices.xml",
			XMLContent: cutoverEInvoiceXML("BILL-2026-001", "Supplier OÜ", "12345678"),
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number\nMADE,2026-05-31,122,BILL-2026-001\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsEInvoiceCustomerContactMode(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeCustomer,
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name,reg_code\nBuyer OÜ,87654321\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("INV-2026-001", "Seller OÜ", "12345678"),
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsEInvoiceBothContactMode(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeBoth,
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name,reg_code\nSupplier OÜ,12345678\nBuyer OÜ,87654321\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("INV-2026-001", "Supplier OÜ", "12345678"),
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsPaymentInvoiceIDReference(t *testing.T) {
	legacyInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + legacyInvoiceID + ",INV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_id\nRECEIVED,2026-05-31,100," + legacyInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)

	var invoiceValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindInvoices {
			invoiceValidation = file
		}
	}
	require.Equal(t, KindInvoices, invoiceValidation.Kind)
	assert.Contains(t, invoiceValidation.Headers, "id")
}

func TestValidateBundleAcceptsPreservedContactIDReferences(t *testing.T) {
	legacyContactID := "11111111-1111-1111-1111-111111111111"
	legacyInvoiceID := "22222222-2222-2222-2222-222222222222"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_id,contact_code,name\n" + legacyContactID + ",SUP-1,Supplier One\n",
		},
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Office expenses,EXPENSE\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount,contact_id\n2026-05-30,Supplier One,5500,1000,42," + legacyContactID + "\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + legacyInvoiceID + ",INV-1,SALES,SUP-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,contact_id,invoice_id\nRECEIVED,2026-05-31,100," + legacyContactID + "," + legacyInvoiceID + "\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_id,line_description,quantity,unit_price,vat_rate\nQ-1,2026-05-30," + legacyContactID + ",Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_id,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31," + legacyContactID + ",Work,1,100,22\n",
		},
		{
			Kind:       KindRecurringInvoices,
			FileName:   "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_id,line_description,quantity,unit_price,vat_rate\nMonthly,MONTHLY,2026-06-01," + legacyContactID + ",Work,1,100,22\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_id\nSKU-1,Widget,10," + legacyContactID + "\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_id,invoice_id\nFA-1,Laptop,2026-05-30,1200," + legacyContactID + "," + legacyInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 11, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)

	var contactsValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindContacts {
			contactsValidation = file
		}
	}
	require.Equal(t, KindContacts, contactsValidation.Kind)
	assert.Contains(t, contactsValidation.Headers, "id")
}

func TestValidateBundleReportsMissingContactIDReference(t *testing.T) {
	missingContactID := "33333333-3333-3333-3333-333333333333"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_id,name\n11111111-1111-1111-1111-111111111111,Supplier One\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_id\nSKU-1,Widget,10," + missingContactID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProducts, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "supplier_id", report.Issues[0].Field)
	assert.Equal(t, missingContactID, report.Issues[0].Value)
}

func TestValidateBundleReportsInvalidContactImportID(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_id,name\nlegacy-id,Bad Contact\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindContacts, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "id", report.Issues[0].Field)
	assert.Equal(t, "legacy-id", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "valid UUID")
}

func TestValidateBundleReportsInvalidInvoiceImportID(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nlegacy-id,INV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindInvoices, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "id", report.Issues[0].Field)
	assert.Equal(t, "legacy-id", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "valid UUID")
}

func TestValidateBundleReportsEInvoiceContactReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "name,reg_code\nOther Supplier,87654321\n",
		},
		{
			Kind:       KindEInvoices,
			FileName:   "e-invoices.xml",
			XMLContent: cutoverEInvoiceXML("BILL-2026-001", "Supplier OÜ", "12345678"),
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindEInvoices, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "12345678", report.Issues[0].Value)
}

func TestValidateBundleReportsEInvoiceCustomerContactReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeCustomer,
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name,reg_code\nSupplier OÜ,12345678\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("INV-2026-001", "Supplier OÜ", "12345678"),
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindEInvoices, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "buyer_reg_code/buyer_vat_number/buyer_contact_email/buyer_contact_name", report.Issues[0].Field)
	assert.Equal(t, "87654321", report.Issues[0].Value)
}

func TestValidateBundleRejectsUnsupportedEInvoiceContactMode(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: "partner",
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name\nSupplier OÜ\n",
			},
		},
	})

	require.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "unsupported e_invoice_contact_mode")
}

func TestValidateBundleReportsInvalidEInvoiceXML(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{Kind: KindEInvoices, FileName: "bad.xml", XMLContent: "<Invoice></Invoice>"},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0].Message, "root element must be E_Invoice")
}

func TestValidateBundleRejectsUnsupportedKind(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{Kind: "unknown", FileName: "unknown.csv", CSVContent: "id\n1\n"},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0].Message, "unsupported migration file kind")
}

func assertValidationIssue(t *testing.T, report *BundleValidationReport, kind FileKind, field, message string) {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Kind == kind && issue.Field == field && strings.Contains(issue.Message, message) {
			return
		}
	}
	assert.Failf(t, "missing validation issue", "kind=%s field=%s message containing %q issues=%v", kind, field, message, report.Issues)
}

func cutoverEInvoiceXML(number, sellerName, sellerRegCode string) string {
	return `<E_Invoice>
  <Invoice invoiceId="` + number + `">
    <InvoiceParties>
      <SellerParty>
        <Name>` + sellerName + `</Name>
        <RegNumber>` + sellerRegCode + `</RegNumber>
        <VATRegNumber>EE` + sellerRegCode + `</VATRegNumber>
        <ContactData>
          <E-mailAddress>supplier@example.com</E-mailAddress>
        </ContactData>
      </SellerParty>
      <BuyerParty>
        <Name>Buyer OÜ</Name>
        <RegNumber>87654321</RegNumber>
      </BuyerParty>
    </InvoiceParties>
    <InvoiceInformation>
      <Type type="DEB"></Type>
      <InvoiceNumber>` + number + `</InvoiceNumber>
      <InvoiceContentText>Office supplies</InvoiceContentText>
      <InvoiceDate>2026-03-15</InvoiceDate>
      <DueDate>2026-03-29</DueDate>
    </InvoiceInformation>
    <InvoiceSumGroup>
      <Currency>EUR</Currency>
    </InvoiceSumGroup>
    <InvoiceItem>
      <InvoiceItemGroup>
        <ItemEntry>
          <Description>Office chairs</Description>
          <ItemDetailInfo>
            <ItemAmount>1</ItemAmount>
            <ItemPrice>100.00</ItemPrice>
          </ItemDetailInfo>
          <VAT><VATRate>22</VATRate></VAT>
        </ItemEntry>
      </InvoiceItemGroup>
    </InvoiceItem>
    <PaymentInfo>
      <PayDueDate>2026-03-29</PayDueDate>
    </PaymentInfo>
  </Invoice>
</E_Invoice>`
}
