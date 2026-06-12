package cutover

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBundleReportsReadyBundle(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code;account_name;type\n1000;Cash;ASSET\n4000;Sales;REVENUE\n5500;Office expenses;EXPENSE\n",
		},
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,email,reg_code\nCUST-1,Customer One,ap@example.com,12345678\n",
		},
		{
			Kind:       KindEmployees,
			FileName:   "employees.csv",
			CSVContent: "employee_number,first_name,last_name,email\nEMP-1,Mari,Maasikas,mari@example.com\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount\n2026-05-30,Office Store,5500,1000,42\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate,product_code\nINV-1,CUST-1,2026-05-30,Work,1,100,22,SKU-1\n",
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
			CSVContent: "account_code,debit,credit\n1000,100,0\n",
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
	assert.Equal(t, 30, report.Summary.RowsValidated)
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
	assert.Contains(t, eInvoiceValidation.Headers, "invoice_number")
	assert.Contains(t, eInvoiceValidation.Headers, "contact_reg_code")
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

func TestValidateBundleReportsTSDHistoryEmployeeReferenceIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindEmployees,
			FileName:   "employees.csv",
			CSVContent: "employee_number,first_name,last_name\nEMP-1,Mari,Maasikas\n",
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
			CSVContent: "invoice_number,contact_code,issue_date,line_description,quantity,vat_rate\nINV-1,CUST-404,2026-05-30,Work,1,22\n",
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
			CSVContent: "payment_type,payment_date,amount,invoice_number\nPAID,2026-05-31,122,BILL-2026-001\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
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
			CSVContent: "invoice_id,invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\n" + legacyInvoiceID + ",INV-1,CUST-1,2026-05-30,Work,1,100,22\n",
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
			CSVContent: "invoice_id,invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\n" + legacyInvoiceID + ",INV-1,SUP-1,2026-05-30,Work,1,100,22\n",
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
			CSVContent: "invoice_id,invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\nlegacy-id,INV-1,CUST-1,2026-05-30,Work,1,100,22\n",
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
