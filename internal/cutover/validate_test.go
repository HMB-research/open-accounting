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
			CSVContent: "account_code;account_name;type\n1000;Cash;ASSET\n4000;Sales;REVENUE\n",
		},
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,email\nCUST-1,Customer One,ap@example.com\n",
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
			CSVContent: "invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\nINV-1,CUST-1,2026-05-30,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number\nRECEIVED,2026-05-31,100,INV-1\n",
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
			Kind:       KindKMDHistory,
			FileName:   "kmd.csv",
			CSVContent: "year,month,row_code,tax_base,tax_amount\n2026,5,1,100,22\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\nQ-1,2026-05-30,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_code,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-30,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindRecurringInvoices,
			FileName:   "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate\nMonthly retainer,MONTHLY,2026-06-01,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name,parent_code\nCC-ROOT,Root,\nCC-SALES,Sales,CC-ROOT\n",
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
			CSVContent: "product_code,name,category_name,sales_price\nSKU-1,Widget,Widgets,10\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity,batch,serial,expiration_date\nSKU-1,MAIN,5,LOT-1,SN-1,2027-05-30\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost\nFA-1,Laptop,2026-05-30,1200\n",
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
	assert.Equal(t, 25, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)

	var stockValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindStockAdjustments {
			stockValidation = file
			break
		}
	}
	require.Equal(t, KindStockAdjustments, stockValidation.Kind)
	assert.Contains(t, stockValidation.Headers, "lot_number")
	assert.Contains(t, stockValidation.Headers, "serial_number")
	assert.Contains(t, stockValidation.Headers, "expiry_date")
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
