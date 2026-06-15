package cutover

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cutoverJournalLineID1 = "11111111-1111-4111-8111-111111111111"
	cutoverJournalLineID2 = "22222222-2222-4222-8222-222222222222"
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
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date,notes\n" +
				"CC-SALES," + cutoverJournalLineID1 + ",125.50,50,2026-05-31,Shared rent\n",
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
			CSVContent: "product_code,warehouse_code,quantity,batch,serial,expiration_date\nSKU-1,MAIN,1,LOT-1,SN-1,2027-05-30\n",
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
	require.Len(t, report.RemediationActions, 1)
	assert.Equal(t, "ready_to_import", report.RemediationActions[0].Code)
	assert.Equal(t, "ACTION", report.RemediationActions[0].Severity)
	assert.Equal(t, "migration_cutover", report.RemediationActions[0].WorkspaceQueue)
	assert.Equal(t, "migration:ready-to-import:-:-:-:-", report.RemediationActions[0].AssignmentKey)
	assert.Equal(t, "low", report.RemediationActions[0].Priority)
	assert.Equal(t, "/migration", report.RemediationActions[0].UIPath)

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

func TestValidateBundleBuildsRemediationActions(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code\nCUST-1\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-404,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)

	codes := migrationRemediationActionCodes(report.RemediationActions)
	assert.Contains(t, codes, "missing_required_columns")
	assert.Contains(t, codes, "missing_reference")

	var referenceAction MigrationRemediationAction
	for _, action := range report.RemediationActions {
		if action.Code == "missing_reference" {
			referenceAction = action
			break
		}
	}
	require.Equal(t, "missing_reference", referenceAction.Code)
	assert.Equal(t, "BLOCKER", referenceAction.Severity)
	assert.Equal(t, KindInvoices, referenceAction.Kind)
	assert.Equal(t, KindContacts, referenceAction.TargetKind)
	assert.Contains(t, referenceAction.Field, "contact_code")
	assert.Equal(t, 1, referenceAction.IssueCount)
	assert.Equal(t, "migration_cutover", referenceAction.WorkspaceQueue)
	assert.Equal(t, "migration:missing-reference:invoices:invoices-csv:contact-code:contacts", referenceAction.AssignmentKey)
	assert.Equal(t, "high", referenceAction.Priority)
	assert.Equal(t, 1, referenceAction.DueInDays)
	assert.Contains(t, referenceAction.CLICommand, "--invoices")
}

func TestBuildMigrationRemediationActionsClassifiesWarnings(t *testing.T) {
	report := &BundleValidationReport{Summary: BundleValidationSummary{Ready: true}}
	report.addIssue(ValidationIssue{
		Severity: SeverityWarning,
		Kind:     KindInvoices,
		FileName: "invoices.csv",
		Field:    "currency",
		Message:  "uses fallback currency",
	})

	actions := BuildMigrationRemediationActions(report)

	require.Len(t, actions, 1)
	assert.Equal(t, "warning_review", actions[0].Code)
	assert.Equal(t, "WARNING", actions[0].Severity)
	assert.Equal(t, KindInvoices, actions[0].Kind)
	assert.Equal(t, "currency", actions[0].Field)
	assert.Equal(t, 1, actions[0].IssueCount)
	assert.Equal(t, "migration_cutover", actions[0].WorkspaceQueue)
	assert.Equal(t, "normal", actions[0].Priority)
	assert.Equal(t, 3, actions[0].DueInDays)
	assert.Contains(t, actions[0].Action, "Review the warning")
}

func TestMigrationAssignmentPriorityDefaultsNonBlockingSeverities(t *testing.T) {
	priority, dueInDays := migrationAssignmentPriority("ACTION")
	assert.Equal(t, "low", priority)
	assert.Equal(t, 0, dueInDays)
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

func TestValidateBundleAcceptsProviderOpeningBalanceAmountAliases(t *testing.T) {
	tests := []struct {
		name            string
		provider        MigrationProviderPreset
		accountsContent string
		openingContent  string
	}{
		{
			name:            "merit",
			provider:        MigrationProviderPresetMerit,
			accountsContent: "konto_kood,konto_nimi,konto_tüüp\n1000,Cash,ASSET\n3000,Equity,EQUITY\n",
			openingContent:  "konto_kood,deebetsumma,kreeditsumma,selgitus\n1000,100,0,Cash brought forward\n3000,0,100,Equity brought forward\n",
		},
		{
			name:            "smartaccounts",
			provider:        MigrationProviderPresetSmartAccounts,
			accountsContent: "account_no,account_title,classification\n1000,Cash,ASSET\n3000,Equity,EQUITY\n",
			openingContent:  "gl_account_no,opening_debit,opening_credit,memo\n1000,100,0,Cash brought forward\n3000,0,100,Equity brought forward\n",
		},
		{
			name:            "directo",
			provider:        MigrationProviderPresetDirecto,
			accountsContent: "konto,kirjeldus,klass\n1000,Cash,ASSET\n3000,Equity,EQUITY\n",
			openingContent:  "konto_nr,algsaldo_deebet,algsaldo_kreedit,selgitus\n1000,100,0,Cash brought forward\n3000,0,100,Equity brought forward\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{
				ProviderPreset: tt.provider,
				Files: []BundleFile{
					{
						Kind:       KindAccounts,
						FileName:   tt.name + "-accounts.csv",
						CSVContent: tt.accountsContent,
					},
					{
						Kind:       KindOpeningBalances,
						FileName:   tt.name + "-opening-balances.csv",
						CSVContent: tt.openingContent,
					},
				},
			})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.True(t, report.Summary.Ready)
			assert.Equal(t, 0, report.Summary.ErrorCount)
			require.Len(t, report.Files, 2)
			assert.Contains(t, report.Files[1].Headers, "account_code")
			assert.Contains(t, report.Files[1].Headers, "debit")
			assert.Contains(t, report.Files[1].Headers, "credit")
			assert.Contains(t, report.Files[1].Headers, "description")
		})
	}
}

func TestValidateBundleAcceptsMeritInventoryAndAssetProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:     KindProductCategories,
				FileName: "merit-product-groups.csv",
				CSVContent: "grupp,ylemgrupp\n" +
					"Root,\n" +
					"Widgets,Root\n",
			},
			{
				Kind:       KindWarehouses,
				FileName:   "merit-warehouses.csv",
				CSVContent: "ladu_kood,ladu_nimi,vaikimisi\nMAIN,Main warehouse,yes\n",
			},
			{
				Kind:       KindProducts,
				FileName:   "merit-products.csv",
				CSVContent: "artikkel,toote_nimi,tootegrupp,myygihind,ostuhind,kaibemaks\nSKU-1,Widget,Widgets,12,6,22\n",
			},
			{
				Kind:       KindStockAdjustments,
				FileName:   "merit-stock.csv",
				CSVContent: "artikkel,ladu,kogus,omahind,partii,seerianumber,aegumiskuupaev\nSKU-1,MAIN,1,6,LOT-1,SN-1,2027-01-31\n",
			},
			{
				Kind:       KindFixedAssets,
				FileName:   "merit-fixed-assets.csv",
				CSVContent: "pohivara_nr,nimetus,soetuskuupaev,soetusmaksumus,kasulik_elu_kuud,akumuleeritud_kulum,jaakmaksumus\nFA-1,Laptop,2026-01-01,1200,36,200,1000\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 5)
	assert.Contains(t, report.Files[0].Headers, "parent_name")
	assert.Contains(t, report.Files[1].Headers, "code")
	assert.Contains(t, report.Files[2].Headers, "category_name")
	assert.Contains(t, report.Files[3].Headers, "serial_number")
	assert.Contains(t, report.Files[4].Headers, "purchase_cost")
	assert.Contains(t, report.Files[4].Headers, "book_value")
}

func TestValidateBundleAcceptsSmartAccountsInventoryAndAssetProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:     KindProductCategories,
				FileName: "smartaccounts-product-groups.csv",
				CSVContent: "item_group_name,parent_group\n" +
					"Root,\n" +
					"Widgets,Root\n",
			},
			{
				Kind:       KindWarehouses,
				FileName:   "smartaccounts-warehouses.csv",
				CSVContent: "location_no,location_name,default_warehouse\nMAIN,Main warehouse,true\n",
			},
			{
				Kind:       KindProducts,
				FileName:   "smartaccounts-products.csv",
				CSVContent: "item_no,item_name,item_group_name,sales_price,purchase_price,vat_percent\nSKU-1,Widget,Widgets,12,6,22\n",
			},
			{
				Kind:       KindStockAdjustments,
				FileName:   "smartaccounts-stock.csv",
				CSVContent: "item_no,location_no,quantity_on_hand,average_cost,batch_no,serial_no,best_before\nSKU-1,MAIN,1,6,LOT-1,SN-1,2027-01-31\n",
			},
			{
				Kind:       KindFixedAssets,
				FileName:   "smartaccounts-fixed-assets.csv",
				CSVContent: "fixed_asset_no,asset_description,purchase_date,purchase_value,depreciation_months,accumulated_depreciation,book_value\nFA-1,Laptop,2026-01-01,1200,36,200,1000\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 5)
	assert.Contains(t, report.Files[0].Headers, "parent_name")
	assert.Contains(t, report.Files[1].Headers, "code")
	assert.Contains(t, report.Files[2].Headers, "category_name")
	assert.Contains(t, report.Files[3].Headers, "serial_number")
	assert.Contains(t, report.Files[4].Headers, "purchase_cost")
	assert.Contains(t, report.Files[4].Headers, "useful_life_months")
}

func TestValidateBundleAcceptsSmartAccountsCommercialContactIdentityAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:     KindContacts,
				FileName: "smartaccounts-contacts.csv",
				CSVContent: "client_no,client_name,registration_no,vat_no,email_address\n" +
					"CUST-1,Customer One,12345678,EE123456789,billing@example.test\n",
			},
			{
				Kind:     KindQuotes,
				FileName: "smartaccounts-quotes.csv",
				CSVContent: "document_no,document_date,customer_vat_no,item_description,qty,unit_price,vat_percent\n" +
					"Q-1,2026-05-30,EE123456789,Work,1,100,22\n",
			},
			{
				Kind:     KindOrders,
				FileName: "smartaccounts-orders.csv",
				CSVContent: "document_no,document_date,customer_reg_no,item_description,qty,unit_price,vat_percent\n" +
					"SO-1,2026-05-31,12345678,Work,1,100,22\n",
			},
			{
				Kind:     KindRecurringInvoices,
				FileName: "smartaccounts-recurring.csv",
				CSVContent: "document_no,document_date,customer_email,item_description,qty,unit_price,vat_percent,frequency\n" +
					"Monthly support,2026-06-01,billing@example.test,Support,1,100,22,monthly\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 4)

	var quoteValidation, orderValidation, recurringValidation FileValidation
	for _, file := range report.Files {
		switch file.Kind {
		case KindQuotes:
			quoteValidation = file
		case KindOrders:
			orderValidation = file
		case KindRecurringInvoices:
			recurringValidation = file
		}
	}
	require.Equal(t, KindQuotes, quoteValidation.Kind)
	assert.Contains(t, quoteValidation.Headers, "contact_vat_number")
	require.Equal(t, KindOrders, orderValidation.Kind)
	assert.Contains(t, orderValidation.Headers, "contact_reg_code")
	require.Equal(t, KindRecurringInvoices, recurringValidation.Kind)
	assert.Contains(t, recurringValidation.Headers, "contact_email")
}

func TestValidateBundleAcceptsEstonianCommercialContactIdentityProviderAliases(t *testing.T) {
	tests := []struct {
		name         string
		preset       MigrationProviderPreset
		contactsCSV  string
		quotesCSV    string
		ordersCSV    string
		recurringCSV string
	}{
		{
			name:        "merit",
			preset:      MigrationProviderPresetMerit,
			contactsCSV: "kliendi_kood,nimi,registri_kood,kmkr,e_post\nCUST-1,Customer One,12345678,EE123456789,billing@example.test\n",
			quotesCSV: "pakkumise_nr,pakkumise_kuupaev,kliendi_kmkr,rea_kirjeldus,kogus,yhiku_hind,kaibemaks\n" +
				"Q-1,2026-05-30,EE123456789,Work,1,100,22\n",
			ordersCSV: "tellimuse_nr,tellimuse_kuupaev,kliendi_reg_kood,rea_kirjeldus,kogus,yhiku_hind,kaibemaks\n" +
				"SO-1,2026-05-31,12345678,Work,1,100,22\n",
			recurringCSV: "name,kuupaev,kliendi_epost,rea_kirjeldus,kogus,yhiku_hind,kaibemaks,frequency\n" +
				"Monthly support,2026-06-01,billing@example.test,Support,1,100,22,monthly\n",
		},
		{
			name:        "directo",
			preset:      MigrationProviderPresetDirecto,
			contactsCSV: "kliendikood,nimi,reg_kood,kmkr,epost\nCUST-1,Customer One,12345678,EE123456789,billing@example.test\n",
			quotesCSV: "pakkumine,pakkumise_kuupaev,kliendi_kmkr,sisu,kogus,hind,km\n" +
				"Q-1,2026-05-30,EE123456789,Work,1,100,22\n",
			ordersCSV: "tellimus,tellimuse_kuupaev,kliendi_reg_kood,sisu,kogus,hind,km\n" +
				"SO-1,2026-05-31,12345678,Work,1,100,22\n",
			recurringCSV: "leping,alguskuupaev,kliendi_epost,sisu,kogus,hind,km,frequency\n" +
				"Monthly support,2026-06-01,billing@example.test,Support,1,100,22,monthly\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{
				ProviderPreset: tt.preset,
				Files: []BundleFile{
					{
						Kind:       KindContacts,
						FileName:   tt.name + "-contacts.csv",
						CSVContent: tt.contactsCSV,
					},
					{
						Kind:       KindQuotes,
						FileName:   tt.name + "-quotes.csv",
						CSVContent: tt.quotesCSV,
					},
					{
						Kind:       KindOrders,
						FileName:   tt.name + "-orders.csv",
						CSVContent: tt.ordersCSV,
					},
					{
						Kind:       KindRecurringInvoices,
						FileName:   tt.name + "-recurring.csv",
						CSVContent: tt.recurringCSV,
					},
				},
			})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.True(t, report.Summary.Ready)
			assert.Equal(t, 0, report.Summary.ErrorCount)
			require.Len(t, report.Files, 4)

			var quoteValidation, orderValidation, recurringValidation FileValidation
			for _, file := range report.Files {
				switch file.Kind {
				case KindQuotes:
					quoteValidation = file
				case KindOrders:
					orderValidation = file
				case KindRecurringInvoices:
					recurringValidation = file
				}
			}
			require.Equal(t, KindQuotes, quoteValidation.Kind)
			assert.Contains(t, quoteValidation.Headers, "contact_vat_number")
			require.Equal(t, KindOrders, orderValidation.Kind)
			assert.Contains(t, orderValidation.Headers, "contact_reg_code")
			require.Equal(t, KindRecurringInvoices, recurringValidation.Kind)
			assert.Contains(t, recurringValidation.Headers, "contact_email")
		})
	}
}

func TestValidateBundleAcceptsMeritKMDHistoryProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:     KindKMDHistory,
				FileName: "merit-kmd-history.csv",
				CSVContent: "aasta,kuu,staatus,esitamise_kuupaev,rea_kood,rea_nimetus,maksustatav_kaive,kaibemaks,valjundkaibemaks,sisendkaibemaks\n" +
					"2025,12,accepted,2026-01-20,1,Taxable sales,1000,220,220,80\n" +
					"2025,12,accepted,2026-01-20,4,Input VAT,363.64,80,220,80\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "year")
	assert.Contains(t, report.Files[0].Headers, "month")
	assert.Contains(t, report.Files[0].Headers, "submitted_at")
	assert.Contains(t, report.Files[0].Headers, "row_code")
	assert.Contains(t, report.Files[0].Headers, "tax_base")
	assert.Contains(t, report.Files[0].Headers, "tax_amount")
	assert.Contains(t, report.Files[0].Headers, "total_output_vat")
	assert.Contains(t, report.Files[0].Headers, "total_input_vat")
}

func TestValidateBundleAcceptsSmartAccountsKMDHistoryProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:     KindKMDHistory,
				FileName: "smartaccounts-kmd-history.csv",
				CSVContent: "vat_return_year,vat_return_month,vat_return_status,filing_date,vat_row,line_description,vat_base,vat_sum,output_tax,input_tax\n" +
					"2025,12,accepted,2026-01-20,1,Taxable sales,1000,220,220,80\n" +
					"2025,12,accepted,2026-01-20,4,Input VAT,363.64,80,220,80\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "year")
	assert.Contains(t, report.Files[0].Headers, "month")
	assert.Contains(t, report.Files[0].Headers, "submitted_at")
	assert.Contains(t, report.Files[0].Headers, "row_code")
	assert.Contains(t, report.Files[0].Headers, "description")
	assert.Contains(t, report.Files[0].Headers, "tax_base")
	assert.Contains(t, report.Files[0].Headers, "tax_amount")
	assert.Contains(t, report.Files[0].Headers, "total_output_vat")
	assert.Contains(t, report.Files[0].Headers, "total_input_vat")
}

func TestValidateBundleAcceptsSmartAccountsPayrollYearMonthTaxAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:       KindEmployees,
				FileName:   "smartaccounts-employees.csv",
				CSVContent: "employee_no,first_name,last_name,start_date\nEMP-1,Mari,Maasikas,2026-01-15\n",
			},
			{
				Kind:     KindPayrollHistory,
				FileName: "smartaccounts-payroll-history.csv",
				CSVContent: "payroll_year,payroll_month,employee_no,payroll_status,paid_date,gross_amount,taxable_amount,income_tax_amount,social_tax_amount,employee_unemployment_amount,employer_unemployment_amount,pension_amount,net_amount,employer_cost\n" +
					"2026,5,EMP-1,paid,2026-05-31,2500,1716,343.20,825,40,20,50,1910,3345\n",
			},
			{
				Kind:     KindTSDHistory,
				FileName: "smartaccounts-tsd-history.csv",
				CSVContent: "pay_period_year,pay_period_month,employee_no,declaration_status,filing_date,payment_kind,gross_amount,basic_exemption_amount,taxable_amount,income_tax_amount,social_tax_amount,employee_unemployment_amount,employer_unemployment_amount,pension_amount\n" +
					"2026,5,EMP-1,accepted,2026-06-10,SALARY,2500,784,1716,343.20,825,40,20,50\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 3)

	payroll := report.Files[1]
	assert.Contains(t, payroll.Headers, "period_year")
	assert.Contains(t, payroll.Headers, "period_month")
	assert.Contains(t, payroll.Headers, "status")
	assert.Contains(t, payroll.Headers, "payment_date")
	assert.Contains(t, payroll.Headers, "taxable_income")
	assert.Contains(t, payroll.Headers, "unemployment_insurance_employee")
	assert.Contains(t, payroll.Headers, "unemployment_insurance_employer")
	assert.Contains(t, payroll.Headers, "funded_pension")

	tsd := report.Files[2]
	assert.Contains(t, tsd.Headers, "period_year")
	assert.Contains(t, tsd.Headers, "period_month")
	assert.Contains(t, tsd.Headers, "submitted_at")
	assert.Contains(t, tsd.Headers, "payment_type")
	assert.Contains(t, tsd.Headers, "basic_exemption")
	assert.Contains(t, tsd.Headers, "unemployment_insurance_employee")
	assert.Contains(t, tsd.Headers, "unemployment_insurance_employer")
	assert.Contains(t, tsd.Headers, "funded_pension")
}

func TestValidateBundleAcceptsMeritExpenseProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "merit-accounts.csv",
				CSVContent: "konto_kood,konto_nimi,konto_tüüp\n5500,Office expenses,EXPENSE\n1000,Cash,ASSET\n",
			},
			{
				Kind:     KindExpenses,
				FileName: "merit-expenses.csv",
				CSVContent: "kulu_nr,kulu_kuupaev,hankija,kulu_konto,makse_konto,summa,valuuta,valuutakurss,vajab_tsekki,staatus,esitatud,kinnitatud,selgitus\n" + //nolint:misspell // Estonian CSV header aliases.
					"EXP-1,2026-05-30,Office Store,5500,1000,42,EUR,1,true,approved,2026-05-30,2026-05-31,Receipt\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 2)
	assert.Contains(t, report.Files[1].Headers, "expense_number")
	assert.Contains(t, report.Files[1].Headers, "expense_date")
	assert.Contains(t, report.Files[1].Headers, "merchant")
	assert.Contains(t, report.Files[1].Headers, "expense_account_code")
	assert.Contains(t, report.Files[1].Headers, "payment_account_code")
	assert.Contains(t, report.Files[1].Headers, "amount")
	assert.Contains(t, report.Files[1].Headers, "requires_receipt")
	assert.Contains(t, report.Files[1].Headers, "status")
	assert.Contains(t, report.Files[1].Headers, "approved_at")
}

func TestValidateBundleAcceptsSmartAccountsExpenseProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "smartaccounts-accounts.csv",
				CSVContent: "account_no,account_title,classification\n5500,Office expenses,EXPENSE\n1000,Cash,ASSET\n",
			},
			{
				Kind:     KindExpenses,
				FileName: "smartaccounts-expenses.csv",
				CSVContent: "document_no,document_date,vendor_name,expense_account_no,paid_from_account_no,expense_amount,currency_code,exchange_rate,receipt_required,expense_status,submitted_date,approved_date,memo\n" +
					"EXP-1,2026-05-30,Office Store,5500,1000,42,EUR,1,true,approved,2026-05-30,2026-05-31,Receipt\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 2)
	assert.Contains(t, report.Files[1].Headers, "expense_number")
	assert.Contains(t, report.Files[1].Headers, "expense_date")
	assert.Contains(t, report.Files[1].Headers, "merchant")
	assert.Contains(t, report.Files[1].Headers, "expense_account_code")
	assert.Contains(t, report.Files[1].Headers, "payment_account_code")
	assert.Contains(t, report.Files[1].Headers, "amount")
	assert.Contains(t, report.Files[1].Headers, "requires_receipt")
	assert.Contains(t, report.Files[1].Headers, "status")
	assert.Contains(t, report.Files[1].Headers, "approved_at")
}

func TestValidateBundleAcceptsMeritCostAllocationProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:       KindCostCenters,
				FileName:   "merit-cost-centers.csv",
				CSVContent: "kulukoha_kood,kulukoha_nimi\nOPS,Operations\n",
			},
			{
				Kind:     KindCostAllocations,
				FileName: "merit-cost-allocations.csv",
				CSVContent: "kulukoht,kande_rea_id,jaotuse_summa,jaotuse_protsent,kuupaev,selgitus\n" +
					"OPS," + cutoverJournalLineID1 + ",125.50,50,2026-05-31,Shared rent\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 2)
	assert.Contains(t, report.Files[0].Headers, "code")
	assert.Contains(t, report.Files[1].Headers, "cost_center_code")
	assert.Contains(t, report.Files[1].Headers, "journal_entry_line_id")
	assert.Contains(t, report.Files[1].Headers, "amount")
	assert.Contains(t, report.Files[1].Headers, "allocation_percentage")
	assert.Contains(t, report.Files[1].Headers, "allocation_date")
	assert.Contains(t, report.Files[1].Headers, "notes")
}

func TestValidateBundleAcceptsSmartAccountsCostAllocationProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:       KindCostCenters,
				FileName:   "smartaccounts-cost-centers.csv",
				CSVContent: "department_no,department_name\nOPS,Operations\n",
			},
			{
				Kind:     KindCostAllocations,
				FileName: "smartaccounts-cost-allocations.csv",
				CSVContent: "department_no,journal_line_id,allocated_amount,allocated_percent,posting_date,line_memo\n" +
					"OPS," + cutoverJournalLineID1 + ",125.50,50,2026-05-31,Shared rent\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 2)
	assert.Contains(t, report.Files[0].Headers, "code")
	assert.Contains(t, report.Files[1].Headers, "cost_center_code")
	assert.Contains(t, report.Files[1].Headers, "journal_entry_line_id")
	assert.Contains(t, report.Files[1].Headers, "amount")
	assert.Contains(t, report.Files[1].Headers, "allocation_percentage")
	assert.Contains(t, report.Files[1].Headers, "allocation_date")
	assert.Contains(t, report.Files[1].Headers, "notes")
}

func TestValidateBundleAcceptsMeritJournalLineCostAllocationPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "merit-accounts.csv",
				CSVContent: "konto_kood,konto_nimi,konto_tüüp\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindCostCenters,
				FileName:   "merit-cost-centers.csv",
				CSVContent: "kulukoha_kood,kulukoha_nimi\nOPS,Operations\n",
			},
			{
				Kind:     KindJournalEntries,
				FileName: "merit-journal.csv",
				CSVContent: "kanne_nr,kuupäev,kanne_rea_id,konto,deebet,kreedit,valuuta,selgitus\n" +
					"JE-1,2026-05-31," + cutoverJournalLineID1 + ",1000,125.50,0,EUR,Receipt\n" +
					"JE-1,2026-05-31," + cutoverJournalLineID2 + ",4000,0,125.50,EUR,Receipt\n",
			},
			{
				Kind:     KindCostAllocations,
				FileName: "merit-cost-allocations.csv",
				CSVContent: "kulukoht,kanne_rea_id,jaotuse_summa,kuupaev\n" +
					"OPS," + cutoverJournalLineID1 + ",125.50,2026-05-31\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 4)
	assert.Contains(t, report.Files[2].Headers, "line_id")
	assert.Contains(t, report.Files[2].Headers, "currency")
	assert.Contains(t, report.Files[3].Headers, "journal_entry_line_id")
}

func TestValidateBundleAcceptsSmartAccountsJournalLineCostAllocationPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "smartaccounts-accounts.csv",
				CSVContent: "account_no,account_title,classification\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindCostCenters,
				FileName:   "smartaccounts-cost-centers.csv",
				CSVContent: "department_no,department_name\nOPS,Operations\n",
			},
			{
				Kind:     KindJournalEntries,
				FileName: "smartaccounts-journal.csv",
				CSVContent: "entry_no,transaction_date,entry_line_id,account_no,debit_amount,credit_amount,currency_code,line_memo\n" +
					"JE-1,2026-05-31," + cutoverJournalLineID1 + ",1000,125.50,0,EUR,Receipt\n" +
					"JE-1,2026-05-31," + cutoverJournalLineID2 + ",4000,0,125.50,EUR,Receipt\n",
			},
			{
				Kind:     KindCostAllocations,
				FileName: "smartaccounts-cost-allocations.csv",
				CSVContent: "department_no,entry_line_id,allocated_amount,posting_date\n" +
					"OPS," + cutoverJournalLineID1 + ",125.50,2026-05-31\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 4)
	assert.Contains(t, report.Files[2].Headers, "line_id")
	assert.Contains(t, report.Files[2].Headers, "currency")
	assert.Contains(t, report.Files[3].Headers, "journal_entry_line_id")
}

func TestValidateBundleAcceptsMeritPaymentAndBankProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "merit-accounts.csv",
				CSVContent: "konto_kood,konto_nimi,konto_tüüp\n1000,Main bank,ASSET\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "merit-contacts.csv",
				CSVContent: "kliendi_kood,nimi\nCUST-1,Customer One\n",
			},
			{
				Kind:       KindInvoices,
				FileName:   "merit-invoices.csv",
				CSVContent: "arve_nr,arve_tyyp,arve_kuupaev,due_date,kliendi_kood,rea_kirjeldus,kogus,yhiku_hind,kaibemaks\nINV-1,SALES,2026-05-30,2026-06-14,CUST-1,Work,1,100,22\n",
			},
			{
				Kind:     KindPayments,
				FileName: "merit-payments.csv",
				CSVContent: "makse_nr,makse_tyyp,kuupaev,summa,valuuta,arve_nr,viitenumber,selgitus,makseviis,pangakonto,jaotuse_summa\n" + //nolint:misspell // Estonian CSV header aliases.
					"PAY-1,RECEIVED,2026-05-31,100,EUR,INV-1,REF-1,Customer payment,BANK_TRANSFER,EE471000001020145685,100\n",
			},
			{
				Kind:       KindBankAccounts,
				FileName:   "merit-bank-accounts.csv",
				CSVContent: "konto_nimi,pangakonto,pank,valuuta,konto_kood,vaikimisi,aktiivne\nMain bank,EE471000001020145685,LHV,EUR,1000,true,true\n",
			},
			{
				Kind:       KindBankTransactions,
				FileName:   "merit-bank-transactions.csv",
				CSVContent: "kuupaev,summa,konto,valuuta,selgitus,viitenumber,vastaspool,vastaspoole_konto\n2026-05-31,100,EE471000001020145685,EUR,Customer payment,REF-1,Customer One,EE111\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 6)
	assert.Contains(t, report.Files[3].Headers, "payment_number")
	assert.Contains(t, report.Files[3].Headers, "currency")
	assert.Contains(t, report.Files[3].Headers, "payment_method")
	assert.Contains(t, report.Files[3].Headers, "bank_account")
	assert.Contains(t, report.Files[3].Headers, "allocation_amount")
	assert.Contains(t, report.Files[4].Headers, "account_number")
	assert.Contains(t, report.Files[4].Headers, "currency")
	assert.Contains(t, report.Files[4].Headers, "gl_account_code")
	assert.Contains(t, report.Files[5].Headers, "source_account")
	assert.Contains(t, report.Files[5].Headers, "reference")
	assert.Contains(t, report.Files[5].Headers, "counterparty_account")
}

func TestValidateBundleAcceptsSmartAccountsPaymentAndBankProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "smartaccounts-accounts.csv",
				CSVContent: "account_no,account_title,classification\n1000,Main bank,ASSET\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "smartaccounts-contacts.csv",
				CSVContent: "client_no,client_name\nCUST-1,Customer One\n",
			},
			{
				Kind:       KindInvoices,
				FileName:   "smartaccounts-invoices.csv",
				CSVContent: "document_no,document_type,document_date,due_date,client_no,item_description,qty,unit_price,vat_percent\nINV-1,SALES,2026-05-30,2026-06-14,CUST-1,Work,1,100,22\n",
			},
			{
				Kind:     KindPayments,
				FileName: "smartaccounts-payments.csv",
				CSVContent: "payment_no,payment_kind,payment_date,paid_amount,currency_code,document_no,payment_method,bank_account_no,reference_no,payment_memo,allocated_amount\n" +
					"PAY-1,RECEIVED,2026-05-31,100,EUR,INV-1,BANK_TRANSFER,EE471000001020145685,REF-1,Customer payment,100\n",
			},
			{
				Kind:       KindBankAccounts,
				FileName:   "smartaccounts-bank-accounts.csv",
				CSVContent: "bank_account_name,bank_account_no,bank_name,currency_code,cash_account_no,default_bank_account,active\nMain bank,EE471000001020145685,LHV,EUR,1000,true,true\n",
			},
			{
				Kind:       KindBankTransactions,
				FileName:   "smartaccounts-bank-transactions.csv",
				CSVContent: "transaction_date,transaction_sum,bank_account_no,currency_code,transaction_text,transaction_reference,counterparty_name,counterparty_account_no,external_id\n2026-05-31,100,EE471000001020145685,EUR,Customer payment,REF-1,Customer One,EE111,bank-ext-1\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 6)
	assert.Contains(t, report.Files[3].Headers, "payment_number")
	assert.Contains(t, report.Files[3].Headers, "currency")
	assert.Contains(t, report.Files[3].Headers, "payment_method")
	assert.Contains(t, report.Files[3].Headers, "bank_account")
	assert.Contains(t, report.Files[3].Headers, "allocation_amount")
	assert.Contains(t, report.Files[4].Headers, "account_number")
	assert.Contains(t, report.Files[4].Headers, "currency")
	assert.Contains(t, report.Files[4].Headers, "gl_account_code")
	assert.Contains(t, report.Files[5].Headers, "source_account")
	assert.Contains(t, report.Files[5].Headers, "currency")
	assert.Contains(t, report.Files[5].Headers, "reference")
	assert.Contains(t, report.Files[5].Headers, "counterparty_account")
	assert.Contains(t, report.Files[5].Headers, "external_id")
}

func TestValidateBundleAcceptsDirectoCommercialBankAndJournalProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetDirecto,
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "directo-accounts.csv",
				CSVContent: "konto,kirjeldus,klass\n1000,Main bank,ASSET\n3000,Equity,EQUITY\n4000,Sales,REVENUE\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "directo-contacts.csv",
				CSVContent: "kliendikood,nimi,registrikood,kmkr\nCUST-1,Customer One,12345678,EE12345678\n",
			},
			{
				Kind:       KindInvoices,
				FileName:   "directo-invoices.csv",
				CSVContent: "arve,arve_liik,aeg,tahtaeg,klient,sisu,kogus,hind,km\nINV-1,SALES,2026-05-30,2026-06-14,CUST-1,Work,1,100,22\n",
			},
			{
				Kind:     KindPayments,
				FileName: "directo-payments.csv",
				CSVContent: "laekumine,makse_tyyp,kuupaev,summa,valuuta,arve,makseviis,pangakonto,viitenr,selgitus,jaotatud_summa\n" + //nolint:misspell // Directo CSV header alias.
					"PAY-1,RECEIVED,2026-05-31,100,EUR,INV-1,BANK_TRANSFER,EE471000001020145685,REF-1,Customer payment,100\n",
			},
			{
				Kind:       KindBankAccounts,
				FileName:   "directo-bank-accounts.csv",
				CSVContent: "nimi,pangakonto,pank,valuuta,konto_kood,vaikimisi,aktiivne\nMain bank,EE471000001020145685,LHV,EUR,1000,true,true\n",
			},
			{
				Kind:       KindBankTransactions,
				FileName:   "directo-bank-transactions.csv",
				CSVContent: "kuupaev,summa,konto,valuuta,sisu,viitenr,vastaspool,vastaspoole_konto\n2026-05-31,100,EE471000001020145685,EUR,Customer payment,REF-1,Customer One,EE111\n",
			},
			{
				Kind:       KindCostCenters,
				FileName:   "directo-objects.csv",
				CSVContent: "objekt,nimi\nOPS,Operations\n",
			},
			{
				Kind:     KindJournalEntries,
				FileName: "directo-journal.csv",
				CSVContent: "kanne,kuupaev,rea_id,konto,deebet,kreedit,valuuta,selgitus\n" +
					"JE-1,2026-05-31," + cutoverJournalLineID1 + ",1000,125.50,0,EUR,Receipt\n" +
					"JE-1,2026-05-31," + cutoverJournalLineID2 + ",4000,0,125.50,EUR,Receipt\n",
			},
			{
				Kind:     KindCostAllocations,
				FileName: "directo-cost-allocations.csv",
				CSVContent: "objekt,rea_id,jaotuse_summa,kuupaev,selgitus\n" +
					"OPS," + cutoverJournalLineID1 + ",125.50,2026-05-31,Shared rent\n",
			},
			{
				Kind:       KindOpeningBalances,
				FileName:   "directo-opening-balances.csv",
				CSVContent: "konto,deebet,kreedit\n1000,100,0\n3000,0,100\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 10)
	assert.Contains(t, report.Files[0].Headers, "account_type")
	assert.Contains(t, report.Files[2].Headers, "invoice_number")
	assert.Contains(t, report.Files[3].Headers, "payment_number")
	assert.Contains(t, report.Files[3].Headers, "currency")
	assert.Contains(t, report.Files[4].Headers, "account_number")
	assert.Contains(t, report.Files[5].Headers, "source_account")
	assert.Contains(t, report.Files[7].Headers, "line_id")
	assert.Contains(t, report.Files[8].Headers, "journal_entry_line_id")
	assert.Contains(t, report.Files[9].Headers, "account_code")
}

func TestValidateBundleDefaultsDisplayFileNames(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			CSVContent: "name,reg_code\nSupplier OÜ,12345678\n",
		},
		{
			Kind:       KindEInvoices,
			XMLContent: cutoverEInvoiceXML("BILL-2026-001", "Supplier OÜ", "12345678"),
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 2)
	assert.Equal(t, "contacts.csv", report.Files[0].FileName)
	assert.Equal(t, "e_invoices.xml", report.Files[1].FileName)
}

func TestValidateBundleAcceptsMeritPayrollProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetMerit,
		Files: []BundleFile{
			{
				Kind:       KindEmployees,
				FileName:   "merit-employees.csv",
				CSVContent: "ImportCode,FirstName,SurName,PersonalCode,StartDate,Amount\nEMP-1,Mari,Maasikas,48901010001,2026-01-15,3200\n",
			},
			{
				Kind:       KindPayrollHistory,
				FileName:   "merit-salary-report.csv",
				CSVContent: "ContractCode,EmployeeFullName,Month6,Sum,SocialTax,EmployerUnempInsurance\nEMP-1,Mari Maasikas,202605,2500,825,20\n",
			},
			{
				Kind:       KindLeaveBalances,
				FileName:   "merit-vacation-balance.csv",
				CSVContent: "ContractCode,Date,TypeCode,Days,InitBalance,DaysAcquired\nEMP-1,2026-12-31,1,28,4,6\n",
			},
			{
				Kind:       KindTSDHistory,
				FileName:   "merit-tsd-history.csv",
				CSVContent: "PersonalCode,Month6,Sum,IncomeTax,SocialTax\n48901010001,202605,2500,500,825\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 4)
	assert.Contains(t, report.Files[0].Headers, "employee_number")
	assert.Contains(t, report.Files[0].Headers, "base_salary")
	assert.Contains(t, report.Files[1].Headers, "period_year")
	assert.Contains(t, report.Files[1].Headers, "period_month")
	assert.Contains(t, report.Files[1].Headers, "gross_salary")
	assert.Contains(t, report.Files[2].Headers, "year")
	assert.Contains(t, report.Files[2].Headers, "absence_type_code")
	assert.Contains(t, report.Files[2].Headers, "carryover_days")
	assert.Contains(t, report.Files[3].Headers, "period_year")
	assert.Contains(t, report.Files[3].Headers, "gross_payment")
}

func TestValidateBundleAcceptsDirectoPayrollInventoryAndTaxProviderPresetAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: "directo-erp",
		Files: []BundleFile{
			{
				Kind:       KindEmployees,
				FileName:   "directo-employees.csv",
				CSVContent: "tootaja,eesnimi,perenimi,isikukood,algus,brutopalk\nEMP-1,Mari,Maasikas,48901010001,2026-01-15,3200\n",
			},
			{
				Kind:       KindPayrollHistory,
				FileName:   "directo-payroll.csv",
				CSVContent: "tootaja,aasta,kuu,bruto,sotsiaalmaks,tulumaks\nEMP-1,2026,5,2500,825,500\n",
			},
			{
				Kind:       KindLeaveBalances,
				FileName:   "directo-leave.csv",
				CSVContent: "tootaja,aasta,puudumise_liik,paevi,jaak,kasutatud\nEMP-1,2026,ANNUAL,28,4,6\n",
			},
			{
				Kind:       KindTSDHistory,
				FileName:   "directo-tsd.csv",
				CSVContent: "isikukood,aasta,kuu,bruto,tulumaks,sotsiaalmaks\n48901010001,2026,5,2500,500,825\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "directo-contacts.csv",
				CSVContent: "hankijakood,nimi,registrikood\nSUP-1,Supplier One,12345678\n",
			},
			{
				Kind:     KindProductCategories,
				FileName: "directo-product-classes.csv",
				CSVContent: "klass,ylemklass\n" +
					"Root,\n" +
					"Widgets,Root\n",
			},
			{
				Kind:       KindWarehouses,
				FileName:   "directo-warehouses.csv",
				CSVContent: "ladu,nimi,vaikimisi\nMAIN,Main warehouse,true\n",
			},
			{
				Kind:       KindProducts,
				FileName:   "directo-products.csv",
				CSVContent: "artikkel,nimetus,klass,myygihind,ostuhind,kaibemaks,hankija_nimi\nSKU-1,Widget,Widgets,12,6,22,Supplier One\n",
			},
			{
				Kind:       KindStockAdjustments,
				FileName:   "directo-stock.csv",
				CSVContent: "artikkel,ladu,kogus,hind,partii,sn,aegumisaeg\nSKU-1,MAIN,1,6,LOT-1,SN-1,2027-01-31\n",
			},
			{
				Kind:       KindFixedAssets,
				FileName:   "directo-fixed-assets.csv",
				CSVContent: "inventar,nimetus,soetuskuupaev,soetusmaksumus,eluiga_kuud,akumuleeritud_kulum,jaakmaksumus,hankija_registrikood\nFA-1,Laptop,2026-01-01,1200,36,200,1000,12345678\n",
			},
			{
				Kind:     KindKMDHistory,
				FileName: "directo-kmd.csv",
				CSVContent: "aasta,kuu,staatus,rea_kood,rea_nimetus,maksustatav_kaive,kaibemaks,valjundkaibemaks,sisendkaibemaks\n" +
					"2025,12,accepted,1,Taxable sales,1000,220,220,80\n" +
					"2025,12,accepted,4,Input VAT,363.64,80,220,80\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 11)
	assert.Contains(t, report.Files[0].Headers, "employee_number")
	assert.Contains(t, report.Files[1].Headers, "gross_salary")
	assert.Contains(t, report.Files[2].Headers, "absence_type_code")
	assert.Contains(t, report.Files[3].Headers, "gross_payment")
	assert.Contains(t, report.Files[4].Headers, "code")
	assert.Contains(t, report.Files[5].Headers, "parent_name")
	assert.Contains(t, report.Files[7].Headers, "category_name")
	assert.Contains(t, report.Files[7].Headers, "supplier_name")
	assert.Contains(t, report.Files[8].Headers, "serial_number")
	assert.Contains(t, report.Files[9].Headers, "purchase_cost")
	assert.Contains(t, report.Files[9].Headers, "supplier_reg_code")
	assert.Contains(t, report.Files[10].Headers, "total_output_vat")
}

func TestValidateBundleDerivesLeaveBalanceYearFromBalanceDate(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:     KindLeaveBalances,
				FileName: "smartaccounts-leave-balances.csv",
				CSVContent: "employee_no,balance_date,leave_type_code,entitlement_days\n" +
					"EMP-1,2026,ANNUAL,28\n" +
					"EMP-2,2026-12-31,SICK,5\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "year")
}

func TestValidateBundleDerivesKMDHistoryPeriodFromAlias(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindKMDHistory,
			FileName: "kmd-history.csv",
			CSVContent: "period_code,row_code,tax_base,tax_amount\n" +
				"2026-05,1,100,22\n" +
				"202606,2,50,11\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "year")
	assert.Contains(t, report.Files[0].Headers, "month")
}

func TestValidateBundleReportsLeaveBalanceBalanceDateWithoutDerivableYear(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		ProviderPreset: MigrationProviderPresetSmartAccounts,
		Files: []BundleFile{
			{
				Kind:     KindLeaveBalances,
				FileName: "smartaccounts-leave-balances.csv",
				CSVContent: "employee_no,balance_date,leave_type_code,entitlement_days\n" +
					"EMP-1,20AB,ANNUAL,28\n" +
					"EMP-2,FY2026,SICK,5\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindLeaveBalances, "year", "year must be between 2020 and 2100")
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

func TestNormalizeMigrationProviderPresetVariants(t *testing.T) {
	tests := []struct {
		name string
		in   MigrationProviderPreset
		want MigrationProviderPreset
	}{
		{name: "empty defaults to generic", in: "", want: MigrationProviderPresetGeneric},
		{name: "generic with whitespace", in: " generic ", want: MigrationProviderPresetGeneric},
		{name: "merit uppercase", in: "MERIT", want: MigrationProviderPresetMerit},
		{name: "smartaccounts canonical", in: MigrationProviderPresetSmartAccounts, want: MigrationProviderPresetSmartAccounts},
		{name: "smart accounts spaced", in: "Smart Accounts", want: MigrationProviderPresetSmartAccounts},
		{name: "smart account singular", in: "smart-account", want: MigrationProviderPresetSmartAccounts},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMigrationProviderPreset(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileSpecForProviderPresetMergesAliasesWithoutMutatingGeneric(t *testing.T) {
	genericAccounts := fileSpecForProviderPreset(KindAccounts, MigrationProviderPresetGeneric)
	meritAccounts := fileSpecForProviderPreset(KindAccounts, MigrationProviderPresetMerit)

	assert.Equal(t, "code", meritAccounts.aliases["konto_kood"])
	assert.Equal(t, "code", meritAccounts.aliases["account_code"])
	_, genericHasMeritAlias := genericAccounts.aliases["konto_kood"]
	assert.False(t, genericHasMeritAlias)
	assert.Equal(t, "code", genericAccounts.aliases["account_code"])
}

func TestFileSpecForProviderPresetFallsBackWhenProviderHasNoAliases(t *testing.T) {
	genericSpec := fileSpecForProviderPreset(KindEInvoices, MigrationProviderPresetGeneric)
	meritSpec := fileSpecForProviderPreset(KindEInvoices, MigrationProviderPresetMerit)
	smartAccountsSpec := fileSpecForProviderPreset(KindEInvoices, MigrationProviderPresetSmartAccounts)

	assert.Equal(t, genericSpec.aliases, meritSpec.aliases)
	assert.Equal(t, genericSpec.requiredGroups, meritSpec.requiredGroups)
	assert.Equal(t, genericSpec.aliases, smartAccountsSpec.aliases)
	assert.Equal(t, genericSpec.requiredGroups, smartAccountsSpec.requiredGroups)
}

func TestListMigrationProviderPresetsReturnsOperatorCatalog(t *testing.T) {
	presets := ListMigrationProviderPresets()

	require.Len(t, presets, 4)
	assert.Equal(t, MigrationProviderPresetGeneric, presets[0].Preset)
	assert.Equal(t, MigrationProviderPresetMerit, presets[1].Preset)
	assert.Equal(t, MigrationProviderPresetSmartAccounts, presets[2].Preset)
	assert.Equal(t, MigrationProviderPresetDirecto, presets[3].Preset)
	assert.Equal(t, 0, presets[0].PresetAliasCount)
	assert.Greater(t, presets[1].PresetAliasCount, 0)
	assert.Greater(t, presets[2].PresetAliasCount, 0)
	assert.Greater(t, presets[3].PresetAliasCount, 0)

	var meritAccounts *MigrationProviderPresetKindInfo
	for i := range presets[1].FileKinds {
		if presets[1].FileKinds[i].Kind == KindAccounts {
			meritAccounts = &presets[1].FileKinds[i]
			break
		}
	}
	require.NotNil(t, meritAccounts)
	assert.Contains(t, meritAccounts.RequiredColumnGroups, []string{"code"})
	assert.Greater(t, meritAccounts.PresetAliasCount, 0)
	assert.Contains(t, meritAccounts.SampleAliases, MigrationProviderPresetAlias{
		SourceHeader:    "konto",
		CanonicalHeader: "code",
	})

	var directoInvoices *MigrationProviderPresetKindInfo
	for i := range presets[3].FileKinds {
		if presets[3].FileKinds[i].Kind == KindInvoices {
			directoInvoices = &presets[3].FileKinds[i]
			break
		}
	}
	require.NotNil(t, directoInvoices)
	assert.Greater(t, directoInvoices.PresetAliasCount, 0)
	assert.Contains(t, directoInvoices.SampleAliases, MigrationProviderPresetAlias{
		SourceHeader:    "aeg",
		CanonicalHeader: "issue_date",
	})
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

func TestValidateBundleReportsBlankInvoiceType(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindInvoices, report.Issues[0].Kind)
	assert.Equal(t, "invoice_type", report.Issues[0].Field)
	assert.Equal(t, "invoice_type is required", report.Issues[0].Message)
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

func TestValidateBundleAcceptsQuoteAndOrderStatusAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindQuotes,
			FileName: "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate,status\n" +
				"Q-1,2026-05-30,CUST-1,Setup,1,100,22,issued\n",
		},
		{
			Kind:     KindOrders,
			FileName: "orders.csv",
			CSVContent: "order_number,order_date,contact_code,line_description,quantity,unit_price,vat_rate,status\n" +
				"SO-1,2026-05-30,CUST-1,Setup,1,100,22,open\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Zero(t, report.Summary.ErrorCount)
}

func TestValidateBundleReportsGroupedDocumentPreservedIDIssues(t *testing.T) {
	sharedInvoiceID := "11111111-1111-1111-1111-111111111111"
	sharedQuoteID := "22222222-2222-2222-2222-222222222222"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				sharedInvoiceID + ",INV-1,SALES,CUST-1,2026-05-30,2026-06-14,Setup,1,100,22\n" +
				sharedInvoiceID + ",INV-1,sale,CUST-1,2026-05-30,2026-06-14,Support,2,50,22\n" +
				sharedInvoiceID + ",INV-2,SALES,CUST-1,2026-05-31,2026-06-15,Duplicate ID,1,75,22\n",
		},
		{
			Kind:     KindQuotes,
			FileName: "quotes.csv",
			CSVContent: "quote_id,quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\n" +
				sharedQuoteID + ",Q-1,2026-05-30,CUST-1,Setup,1,100,22\n" +
				sharedQuoteID + ",Q-1,2026-05-30,CUST-1,Support,2,50,22\n" +
				sharedQuoteID + ",Q-2,2026-05-31,CUST-1,Duplicate ID,1,75,22\n" +
				"not-a-uuid,Q-BAD,2026-06-01,CUST-1,Bad ID,1,10,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "id", "duplicates row")
	assertValidationIssue(t, report, KindQuotes, "id", "duplicates row")
	assertValidationIssue(t, report, KindQuotes, "id", "id must be a valid UUID")
}

func TestValidateBundleReportsMissingCommercialRequiredDateColumn(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,SALES,CUST-1,2026-05-30,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindInvoices, report.Issues[0].Kind)
	assert.Contains(t, report.Issues[0].Message, "missing required column group: due_date")
}

func TestValidateBundleReportsBlankCommercialRequiredDate(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,SALES,CUST-1,,2026-06-14,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "issue_date", "issue_date is required")
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

func TestValidateBundleReportsCommercialNonNegativeDecimalParsingIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-BAD,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,not-a-decimal,nope\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "unit_price", "unit_price must be a decimal")
	assertValidationIssue(t, report, KindInvoices, "vat_rate", "vat_rate must be a decimal")
}

func TestValidateBundleReportsReverseChargeVATRateParsingOnce(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,reverse_charge\n" +
				"INV-RC,SALES,CUST-1,2026-05-30,2026-06-14,EU service,1,100,not-a-decimal,true\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "vat_rate", "vat_rate must be a decimal")
}

func TestCheckInvoiceReverseChargeRateSkipsMissingVATRateColumn(t *testing.T) {
	report := &BundleValidationReport{}

	checkInvoiceReverseChargeRate(report, parsedFile{
		kind:     KindInvoices,
		fileName: "invoices.csv",
		headers:  []string{"invoice_number", "reverse_charge"},
	}, parsedRow{
		number: 2,
		values: map[string]string{
			"invoice_number": "INV-RC",
			"reverse_charge": "true",
		},
	})

	assert.Empty(t, report.Issues)
}

func TestCanonicalizeBundleFileCSVUsesProviderPresetAliases(t *testing.T) {
	tests := []struct {
		name       string
		preset     MigrationProviderPreset
		kind       FileKind
		content    string
		wantHeader string
	}{
		{
			name:       "merit fixed asset name alias",
			preset:     MigrationProviderPresetMerit,
			kind:       KindFixedAssets,
			content:    "põhivara_nr;põhivara;soetuskuupäev;soetusmaksumus\nFA-1;Laptop;2026-01-10;1200\n",
			wantHeader: "asset_number,name,purchase_date,purchase_cost",
		},
		{
			name:       "directo fixed asset number alias",
			preset:     MigrationProviderPresetDirecto,
			kind:       KindFixedAssets,
			content:    "põhivara;nimetus;soetusaeg;soetushind\nFA-2;Printer;2026-02-10;800\n",
			wantHeader: "asset_number,name,purchase_date,purchase_cost",
		},
		{
			name:       "smartaccounts stock aliases",
			preset:     MigrationProviderPresetSmartAccounts,
			kind:       KindStockAdjustments,
			content:    "item_no,warehouse_no,balance_qty,average_cost,batch_no,serial_no,best_before\nSKU-1,MAIN,1,10,LOT-1,SN-1,2027-01-01\n",
			wantHeader: "product_code,warehouse_code,quantity,unit_cost,lot_number,serial_number,expiry_date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := CanonicalizeBundleFileCSV(BundleFile{
				Kind:       tt.kind,
				FileName:   "provider.csv",
				CSVContent: tt.content,
			}, tt.preset)

			require.NoError(t, err)
			header, _, _ := strings.Cut(file.CSVContent, "\n")
			assert.Equal(t, tt.wantHeader, header)
		})
	}
}

func TestValidateBundleAcceptsInvoiceVATTreatmentAndDiscountEdges(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,discount_percent,vat_rate,vat_treatment,reverse_charge\n" +
				"INV-STANDARD,SALES,CUST-1,2026-05-30,2026-06-14,Standard sale,1,100,,22,standard,\n" +
				"INV-NORMAL,SALES,CUST-1,2026-05-30,2026-06-14,Normal sale,1,100,0,22,normal,\n" +
				"INV-RC-TREATMENT,SALES,CUST-1,2026-05-30,2026-06-14,Reverse charge service,1,100,100,20,reverse charge,\n" +
				"INV-RC-FALSE,SALES,CUST-1,2026-05-30,2026-06-14,Local zero-rated line,1,100,\"12,5\",0,,false\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsInvoiceVATTreatmentAndDiscountParsingIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,discount_percent,vat_rate,vat_treatment\n" +
				"INV-BAD,SALES,CUST-1,2026-05-30,2026-06-14,Bad treatment,1,100,not-a-decimal,22,margin\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "discount_percent", "discount_percent must be a decimal")
	assertValidationIssue(t, report, KindInvoices, "vat_treatment", `invalid vat_treatment "margin"`)
}

func TestValidateBundleReportsDuplicateMasterIdentifiers(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "account_code,account_name,type\n" +
				"1000,Cash,ASSET\n" +
				"1000,Duplicate cash,ASSET\n" +
				"5500,Office expenses,EXPENSE\n",
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
			CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount\n" +
				"EXP-1,2026-05-31,Office,5500,1000,42\n" +
				"EXP-1,2026-06-01,Office,5500,1000,43\n",
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

func TestValidateBundleReportsPreservedAccountIDIssues(t *testing.T) {
	accountID := "11111111-1111-1111-1111-111111111111"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{{
		Kind:     KindAccounts,
		FileName: "accounts.csv",
		CSVContent: "id,account_code,account_name,type\n" +
			accountID + ",1000,Cash,ASSET\n" +
			accountID + ",1010,Duplicate Cash,ASSET\n" +
			"not-a-uuid,1020,Bad ID,ASSET\n",
	}}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindAccounts, "id", "duplicates row")
	assertValidationIssue(t, report, KindAccounts, "id", "id must be a valid UUID")
}

func TestValidateBundleAcceptsBankAccountAccountNumberAlias(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "name,account\nMain bank,EE471000001020145685\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "account_number")
	assert.Empty(t, report.Files[0].MissingColumns)
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

func TestValidateBundleAcceptsContactImporterAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindContacts,
			FileName: "contacts.csv",
			CSVContent: "contact_id,company,role,payment_days,country,vat,telephone,address,address_line_2,postcode,credit_limit\n" +
				"11111111-1111-1111-1111-111111111111,Supplier One,vendor,21,EE,EE123456789,+3725550100,Main 1,Suite 2,10115,\"1500,50\"\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Empty(t, report.Issues)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "id")
	assert.Contains(t, report.Files[0].Headers, "name")
	assert.Contains(t, report.Files[0].Headers, "contact_type")
	assert.Contains(t, report.Files[0].Headers, "payment_terms_days")
	assert.Contains(t, report.Files[0].Headers, "country_code")
	assert.Contains(t, report.Files[0].Headers, "vat_number")
	assert.Contains(t, report.Files[0].Headers, "phone")
	assert.Contains(t, report.Files[0].Headers, "address_line1")
	assert.Contains(t, report.Files[0].Headers, "address_line2")
	assert.Contains(t, report.Files[0].Headers, "postal_code")
	assert.NotContains(t, report.Files[0].Headers, "company")
	assert.NotContains(t, report.Files[0].Headers, "role")
	assert.NotContains(t, report.Files[0].Headers, "payment_days")
}

func TestValidateBundleReportsContactImporterAliasRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindContacts,
			FileName: "contacts.csv",
			CSVContent: "company,role,payment_days,country,credit_limit\n" +
				"Bad Contact,partner,-1,EST,not-a-number\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindContacts, "contact_type", `invalid contact_type "partner"`)
	assertValidationIssue(t, report, KindContacts, "payment_terms_days", "payment_terms_days must be a non-negative integer")
	assertValidationIssue(t, report, KindContacts, "country_code", "country_code must be a 2-letter code")
	assertValidationIssue(t, report, KindContacts, "credit_limit", "credit_limit must be a decimal")
}

func TestValidateBundleAcceptsContactImportThousandsCreditLimit(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "name,credit_limit\nCustomer One,\"1,500.50\"\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsEmployeeRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindEmployees,
			FileName: "employees.csv",
			CSVContent: "first_name,last_name,start_date,end_date,employment_type,apply_basic_exemption,basic_exemption_amount,funded_pension_rate,base_salary,salary_effective_from,is_active\n" +
				",Maasikas,2026-01-15,2026-02-01,FULL_TIME,true,700,0.02,3200,2026-01-15,true\n" +
				"Mari,,2026-01-15,2026-02-01,FULL_TIME,true,700,0.02,3200,2026-01-15,true\n" +
				"Bad,MissingStart,,,FULL_TIME,true,700,0.02,3200,2026-01-15,true\n" +
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
	assert.Equal(t, 14, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindEmployees, "first_name", "first_name is required")
	assertValidationIssue(t, report, KindEmployees, "last_name", "last_name is required")
	assertValidationIssue(t, report, KindEmployees, "start_date", "start_date is required")
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

func TestValidateBundleReportsInvalidLeaveBalanceAbsenceTypeIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindLeaveBalances,
			FileName:   "leave-balances.csv",
			CSVContent: "year,employee_number,absence_type_id,entitled_days\n2026,EMP-1,legacy-type,28\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindLeaveBalances, "absence_type_id", "absence_type_id must be a valid UUID")
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

func TestValidateBundleAcceptsTSDHistorySubmittedAtDefaults(t *testing.T) {
	tests := []struct {
		name string
		csv  string
	}{
		{
			name: "missing column",
			csv: "year,month,status,employee_number,gross_payment\n" +
				"2026,5,DRAFT,EMP-1,3200\n" +
				"2026,6,SUBMITTED,EMP-2,3200\n" +
				"2026,7,ACCEPTED,EMP-3,3200\n",
		},
		{
			name: "blank column",
			csv: "year,month,status,submitted_at,employee_number,gross_payment\n" +
				"2026,5,DRAFT,,EMP-1,3200\n" +
				"2026,6,SUBMITTED,,EMP-2,3200\n" +
				"2026,7,ACCEPTED,,EMP-3,3200\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
				{
					Kind:       KindTSDHistory,
					FileName:   "tsd-history.csv",
					CSVContent: tt.csv,
				},
			}})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.True(t, report.Summary.Ready)
			assert.Empty(t, report.Issues)
		})
	}
}

func TestValidateBundleReportsPayrollTSDHistoryMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindEmployees,
			FileName:   "employees.csv",
			CSVContent: "employee_number,first_name,last_name,start_date\nEMP-1,Mari,Maasikas,2026-01-01\n",
		},
		{
			Kind:     KindPayrollHistory,
			FileName: "payroll-history.csv",
			CSVContent: "period_year,period_month,employee_number,gross_salary,taxable_income,income_tax,social_tax,unemployment_insurance_employee,unemployment_insurance_employer,funded_pension\n" +
				"2026,5,EMP-1,2500,2400,480,825,40,20,48\n",
		},
		{
			Kind:     KindTSDHistory,
			FileName: "tsd-history.csv",
			CSVContent: "period_year,period_month,employee_number,gross_payment,taxable_amount,income_tax,social_tax,unemployment_insurance_employee,unemployment_insurance_employer,funded_pension\n" +
				"2026,5,EMP-1,2600,2400,480,825,40,20,48\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindTSDHistory, "gross_payment", "gross_payment must match payroll_history gross_salary")
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayrollHistory, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, "employee EMP-1 in period 2026-05")
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
				"2026,3,filed,31.12.2026,row_4,\"1 000,50\",\"220,11\",\"220,11\",\"220,11\"\n",
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

func TestValidateBundleAcceptsKMDHistoryOptionalGroupValuesAfterBlankFirstRow(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindKMDHistory,
			FileName: "kmd-history.csv",
			CSVContent: "year,month,status,submitted_at,row_code,tax_base,tax_amount,total_output_vat,total_input_vat\n" +
				"2026,5,ACCEPTED,,1,100,22,,\n" +
				"2026,5,ACCEPTED,2026-01-20,2,50,11,38.5,5\n" +
				"2026,5,ACCEPTED,2026-01-20,3,25,5.5,38.5,5\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsKMDHistoryVATReconciliationIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindKMDHistory,
			FileName: "kmd-history.csv",
			CSVContent: "year,month,status,submitted_at,row_code,tax_base,tax_amount,total_output_vat,total_input_vat\n" +
				"2026,5,ACCEPTED,2026-01-20,1,100,22,30,5\n" +
				"2026,5,ACCEPTED,2026-01-20,2,50,11,30,5\n" +
				"2026,5,ACCEPTED,2026-01-20,4,25,5,30,5\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assertValidationIssue(t, report, KindKMDHistory, "total_output_vat", "does not match supporting KMD output VAT rows")
}

func TestValidateBundleReportsHistoryCompositeDuplicateIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindPayrollHistory,
			FileName: "payroll-history.csv",
			CSVContent: "period_year,period_month,employee_number,gross_salary\n" +
				"2026,5,EMP-1,2500\n" +
				"2026,05,EMP-1,2600\n",
		},
		{
			Kind:     KindLeaveBalances,
			FileName: "leave-balances.csv",
			CSVContent: "year,employee_number,absence_type_code,entitled_days\n" +
				"2026,EMP-1,ANNUAL,28\n" +
				"2026,EMP-1,ANNUAL,30\n",
		},
		{
			Kind:     KindTSDHistory,
			FileName: "tsd-history.csv",
			CSVContent: "period_year,period_month,employee_number,gross_payment\n" +
				"2026,5,EMP-1,2500\n" +
				"2026,05,EMP-1,2600\n",
		},
		{
			Kind:     KindKMDHistory,
			FileName: "kmd-history.csv",
			CSVContent: "year,month,row_code,tax_base,tax_amount\n" +
				"2026,5,row_1,100,22\n" +
				"2026,05,1,200,44\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindPayrollHistory, "period_year/period_month/employee", "duplicates row")
	assertValidationIssue(t, report, KindLeaveBalances, "year/employee/absence_type", "duplicates row")
	assertValidationIssue(t, report, KindTSDHistory, "period_year/period_month/employee", "duplicates row")
	assertValidationIssue(t, report, KindKMDHistory, "year/month/row_code", "duplicates row")
}

func TestValidateBundleReportsExpenseRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindExpenses,
			FileName: "expenses.csv",
			CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount,currency,exchange_rate,employee_id,status,requires_receipt,submitted_at,approved_at,rejected_at,rejection_reason\n" +
				"EXP-1,bad-date,, , ,0,EU1,0,legacy-employee,POSTED,maybe,,,,\n" +
				"EXP-2,2026-05-31,Office,5500,1000,abc,,abc,,UNKNOWN,true,,,,\n" +
				"EXP-3,2026-05-31,Office,5500,1000,10,EUR,1,,REJECTED,true,bad-date,,bad-date,\n" +
				"EXP-4,2026-05-31T12:00:00Z,Office,5500,1000,10,EUR,1,,APPROVED,false,bad-date,bad-date,,\n" +
				"EXP-5,2026-05-31,Office,5500,1000,\"10,50\",EUR,\"1,2\",,SUBMITTED,no,2026-05-31,,,\n" +
				"EXP-6,,Office,5500,1000,10,EUR,1,,DRAFT,true,,,,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 19, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindExpenses, "expense_date", "expense_date must be YYYY-MM-DD or RFC3339")
	assertValidationIssue(t, report, KindExpenses, "expense_date", "expense_date is required")
	assertValidationIssue(t, report, KindExpenses, "merchant", "merchant is required")
	assertValidationIssue(t, report, KindExpenses, "expense_account_id/expense_account_code", "expense_account_id or expense_account_code is required")
	assertValidationIssue(t, report, KindExpenses, "payment_account_id/payment_account_code", "payment_account_id or payment_account_code is required")
	assertValidationIssue(t, report, KindExpenses, "amount", "amount must be positive")
	assertValidationIssue(t, report, KindExpenses, "amount", "amount must be a decimal")
	assertValidationIssue(t, report, KindExpenses, "currency", "currency must be a 3-letter ISO code")
	assertValidationIssue(t, report, KindExpenses, "exchange_rate", "exchange_rate must be positive")
	assertValidationIssue(t, report, KindExpenses, "exchange_rate", "exchange_rate must be a decimal")
	assertValidationIssue(t, report, KindExpenses, "employee_id", "employee_id must be a valid UUID")
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

func TestValidateBundleReportsPaymentAllocationAmountDecimalIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_number,payment_type,payment_date,amount,allocation_amount,invoice_number\nPAY-1,RECEIVED,2026-05-31,100,abc,INV-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "allocation_amount", report.Issues[0].Field)
	assert.Equal(t, "abc", report.Issues[0].Value)
	assert.Equal(t, "allocation_amount must be a decimal", report.Issues[0].Message)
}

func TestValidateBundleReportsPaymentAmountRequiredAndDecimalIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_number,payment_type,payment_date,amount\n" +
				"PAY-1,RECEIVED,2026-05-31,\n" +
				"PAY-2,RECEIVED,2026-05-31,abc\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindPayments, "amount", "amount is required")
	assertValidationIssue(t, report, KindPayments, "amount", "amount must be a decimal")
}

func TestValidateBundleReportsBlankPaymentDate(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_number,payment_type,payment_date,amount\nPAY-1,RECEIVED,,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindPayments, "payment_date", "payment_date is required")
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

func TestValidateBundleReportsBankAccountCurrencyCodeWithNonLetters(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "name,account_number,currency,is_default,is_active\nMain bank,EE471000001020145685,EU1,true,true\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindBankAccounts, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "currency", report.Issues[0].Field)
	assert.Equal(t, "EU1", report.Issues[0].Value)
	assert.Equal(t, "currency must be a 3-letter ISO code", report.Issues[0].Message)
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

func TestValidateBundleReportsBankTransactionDescriptionRequired(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount\n2026-05-31,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].MissingColumns, "description")
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0].Message, "missing required column group: description")
}

func TestValidateBundleReportsBankTransactionBlankDescription(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,description\n2026-05-31,100,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindBankTransactions, "description", "description is required")
}

func TestValidateBundleAcceptsBankTransactionSelgitusAlias(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,selgitus\n2026-05-31,100,Customer receipt\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "description")
}

func TestValidateBundleAcceptsLHVBankTransactionDescriptionFallback(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nMain bank,EE457700771000676899,EUR\n",
		},
		{
			Kind:     KindBankTransactions,
			FileName: "lhv-bank.csv",
			CSVContent: "Client account;Document number;Date;Beneficiary's/remitter's account;Beneficiary's/remitter's name;Debit/Credit (D/C);Amount;Reference number;Archival ID;Currency;Personal identification code or registry code;Beneficiary's/remitter's bank's BIC;Payment initiator's name;Entry reference;Account service provider's reference\n" +
				"EE457700771000676899;123;2026-03-15;EE111;Acme;C;100.00;REF-1;202603150001;EUR;12345678;LHVBEE22;;ENTRY-1;ext-1\n",
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
	assert.Contains(t, bankValidation.Headers, "description")
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

func TestValidateBundleReportsOpeningBalanceSingleSidedTotals(t *testing.T) {
	tests := []struct {
		name  string
		rows  string
		value string
	}{
		{
			name:  "debit only",
			rows:  "1000,100,0\n",
			value: "debits=100 credits=0",
		},
		{
			name:  "credit only",
			rows:  "3000,0,100\n",
			value: "debits=0 credits=100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
				{
					Kind:       KindAccounts,
					FileName:   "accounts.csv",
					CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n3000,Owner equity,EQUITY\n",
				},
				{
					Kind:       KindOpeningBalances,
					FileName:   "opening.csv",
					CSVContent: "account_code,debit,credit\n" + tt.rows,
				},
			}})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.False(t, report.Summary.Ready)
			assert.Equal(t, 1, report.Summary.ErrorCount)
			require.Len(t, report.Issues, 1)
			assert.Equal(t, KindOpeningBalances, report.Issues[0].Kind)
			assert.Equal(t, "debit/credit", report.Issues[0].Field)
			assert.Equal(t, tt.value, report.Issues[0].Value)
			assert.Equal(t, "opening balances must include both debit and credit totals", report.Issues[0].Message)
		})
	}
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

func TestValidateBundleReportsOpeningBalanceDebitCreditRowValueIssues(t *testing.T) {
	tests := []struct {
		name    string
		row     string
		value   string
		message string
	}{
		{
			name:    "negative amount",
			row:     "1000,-1.25,0\n",
			value:   "debit=-1.25 credit=0",
			message: "amounts cannot be negative",
		},
		{
			name:    "zero amount",
			row:     "1000,0,0\n",
			value:   "debit=0 credit=0",
			message: "either debit or credit is required",
		},
		{
			name:    "both sides",
			row:     "1000,100,25\n",
			value:   "debit=100 credit=25",
			message: "row cannot contain both debit and credit amounts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
				{
					Kind:       KindOpeningBalances,
					FileName:   "opening.csv",
					CSVContent: "account_code,debit,credit\n" + tt.row,
				},
			}})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.False(t, report.Summary.Ready)
			assert.Equal(t, 1, report.Summary.ErrorCount)
			require.Len(t, report.Issues, 1)
			assert.Equal(t, KindOpeningBalances, report.Issues[0].Kind)
			assert.Equal(t, 2, report.Issues[0].Row)
			assert.Equal(t, "debit/credit", report.Issues[0].Field)
			assert.Equal(t, tt.value, report.Issues[0].Value)
			assert.Equal(t, tt.message, report.Issues[0].Message)
		})
	}
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

func TestValidateBundleReportsHistoricalJournalInvalidSourceID(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit,source_id\nJE-1,2026-05-30,1000,100,0,legacy-source\nJE-1,2026-05-30,4000,0,100,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindJournalEntries, "source_id", "source_id must be a valid UUID")
}

func TestValidateBundleReportsHistoricalJournalReferenceAndDateGroupIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\n" +
				",2026-05-30,1000,100,0\n" +
				"JE-DATE,2026-05-30,1000,100,0\n" +
				"JE-DATE,2026-05-31,4000,0,100\n" +
				"JE-MISSING-DATE,,1000,100,0\n" +
				"JE-MISSING-DATE,2026-05-30,4000,0,100\n" +
				"JE-BAD-DATE,30/05/2026,1000,100,0\n" +
				"JE-BAD-DATE,2026-05-30,4000,0,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 4)
	assertValidationIssue(t, report, KindJournalEntries, "entry_reference", "entry_reference is required")
	assertValidationIssue(t, report, KindJournalEntries, "entry_date", "entry_date must match the group date 2026-05-30")
	assertValidationIssue(t, report, KindJournalEntries, "entry_date", "entry_date is required")
	assertValidationIssue(t, report, KindJournalEntries, "entry_date", "entry_date must be in YYYY-MM-DD format")
}

func TestValidateBundleReportsHistoricalJournalShapeAndExchangeRateGroupIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit,exchange_rate\n" +
				"JE-SINGLE,2026-05-30,1000,25,0,1\n" +
				"JE-CREDIT-ONLY,2026-05-30,4000,0,50,1\n" +
				"JE-CREDIT-ONLY,2026-05-30,4010,0,25,1\n" +
				"JE-BAD-FX,2026-05-30,1000,100,0,-1\n" +
				"JE-BAD-FX,2026-05-30,4000,0,100,1\n" +
				"JE-FX-TEXT,2026-05-30,1000,100,0,abc\n" +
				"JE-FX-TEXT,2026-05-30,4000,0,100,1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 4)
	assertValidationIssue(t, report, KindJournalEntries, "entry_reference", "must have at least two lines")
	assertValidationIssue(t, report, KindJournalEntries, "entry_reference/debit/credit", "cannot have zero amounts")
	assertValidationIssue(t, report, KindJournalEntries, "exchange_rate", "exchange_rate cannot be negative")
	assertValidationIssue(t, report, KindJournalEntries, "exchange_rate", "invalid exchange_rate")
}

func TestValidateBundleSkipsHistoricalJournalGroupingWithoutCompleteHeaders(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,account_code,debit,credit\n" +
				"JE-1,1000,100,0\n" +
				"JE-1,4000,0,90\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindJournalEntries, report.Issues[0].Kind)
	assert.Contains(t, report.Issues[0].Message, "entry_date")
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

func TestValidateBundleAcceptsExpenseAccountTypeReferences(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n2000,Reimbursements,LIABILITY\n5500,Office expenses,EXPENSE\n",
		},
		{
			Kind:     KindExpenses,
			FileName: "expenses.csv",
			CSVContent: "expense_number,expense_date,merchant,expense_account_code,payment_account_code,amount\n" +
				"EXP-1,2026-05-31,Office Store,5500,1000,42\n" +
				"EXP-2,2026-05-31,Employee Claim,5500,2000,35\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsExpenseAccountTypeMismatches(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n5500,Office expenses,EXPENSE\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount\n2026-05-31,Office Store,1000,4000,42\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Equal(t, KindExpenses, report.Issues[0].Kind)
	assert.Equal(t, KindAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "expense_account_code", report.Issues[0].Field)
	assert.Equal(t, "1000/ASSET", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "expected EXPENSE")
	assert.Equal(t, KindExpenses, report.Issues[1].Kind)
	assert.Equal(t, KindAccounts, report.Issues[1].TargetKind)
	assert.Equal(t, "payment_account_code", report.Issues[1].Field)
	assert.Equal(t, "4000/REVENUE", report.Issues[1].Value)
	assert.Contains(t, report.Issues[1].Message, "expected ASSET or LIABILITY")
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
	assert.Equal(t, "gl_account_code", report.Issues[0].Field)
	assert.Equal(t, "9999", report.Issues[0].Value)
}

func TestValidateBundleAcceptsBankAccountAccountTypeReferences(t *testing.T) {
	cashAccountID := "11111111-1111-1111-1111-111111111111"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" +
				cashAccountID + ",1000,Main bank,ASSET\n" +
				"22222222-2222-2222-2222-222222222222,1010,Reserve bank,ASSET\n",
		},
		{
			Kind:     KindBankAccounts,
			FileName: "bank-accounts.csv",
			CSVContent: "name,account_number,currency,gl_account_id,gl_account_code\n" +
				"Main bank,EE471000001020145685,EUR," + cashAccountID + ",\n" +
				"Reserve bank,EE382200221020145685,EUR,,1010\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsBankAccountAccountTypeMismatches(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "account_code,account_name,type\n" +
				"1000,Main loan,LIABILITY\n" +
				"5500,Bank fees,EXPENSE\n",
		},
		{
			Kind:     KindBankAccounts,
			FileName: "bank-accounts.csv",
			CSVContent: "name,account_number,currency,gl_account_code\n" +
				"Main bank,EE471000001020145685,EUR,1000\n" +
				"Reserve bank,EE382200221020145685,EUR,5500\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindBankAccounts, "gl_account_code", `bank account GL account "1000" is LIABILITY; expected ASSET account`)
	assertValidationIssue(t, report, KindBankAccounts, "gl_account_code", `bank account GL account "5500" is EXPENSE; expected ASSET account`)
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

func TestValidateBundleReportsRecurringInvoiceAccountReferenceIssues(t *testing.T) {
	accountID := "11111111-1111-1111-1111-111111111111"
	missingAccountID := "22222222-2222-2222-2222-222222222222"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" + accountID + ",4000,Sales,REVENUE\n",
		},
		{
			Kind:     KindRecurringInvoices,
			FileName: "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate,account_id\n" +
				"Monthly,MONTHLY,2026-06-01,CUST-1,Setup,1,100,22," + accountID + "\n" +
				"Monthly,MONTHLY,2026-06-01,CUST-1,Missing,1,100,22," + missingAccountID + "\n" +
				"Monthly,MONTHLY,2026-06-01,CUST-1,Bad ID,1,100,22,not-a-uuid\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindRecurringInvoices, "account_id", "reference")
	assertValidationIssue(t, report, KindRecurringInvoices, "account_id", "account_id must be a valid UUID")
}

func TestValidateBundleAcceptsRecurringInvoiceAccountTypeReferences(t *testing.T) {
	revenueAccountID := "11111111-1111-1111-1111-111111111111"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" +
				revenueAccountID + ",4000,Subscription revenue,REVENUE\n",
		},
		{
			Kind:     KindRecurringInvoices,
			FileName: "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate,account_id\n" +
				"Monthly support,MONTHLY,2026-06-01,CUST-1,Support retainer,1,100,22," + revenueAccountID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsRecurringInvoiceAccountTypeMismatches(t *testing.T) {
	assetAccountID := "11111111-1111-1111-1111-111111111111"
	expenseAccountID := "22222222-2222-2222-2222-222222222222"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" +
				assetAccountID + ",1000,Cash,ASSET\n" +
				expenseAccountID + ",5500,Service costs,EXPENSE\n",
		},
		{
			Kind:     KindRecurringInvoices,
			FileName: "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate,account_id\n" +
				"Monthly support,MONTHLY,2026-06-01,CUST-1,Support retainer,1,100,22," + assetAccountID + "\n" +
				"Quarterly service,QUARTERLY,2026-06-01,CUST-1,Service retainer,1,250,22," + expenseAccountID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindRecurringInvoices, "account_id", `recurring invoice line account "11111111-1111-1111-1111-111111111111" is ASSET; expected REVENUE account`)
	assertValidationIssue(t, report, KindRecurringInvoices, "account_id", `recurring invoice line account "22222222-2222-2222-2222-222222222222" is EXPENSE; expected REVENUE account`)
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

func TestValidateBundleAcceptsBankTransactionOmittedCurrencyEURAccount(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nMain bank,EE471000001020145685,EUR\n",
		},
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,details,client_account\n2026-05-31,100,Customer receipt,EE471000001020145685\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsBankTransactionOmittedCurrencyNonEURAccount(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nReserve bank,EE471000001020145685,USD\n",
		},
		{
			Kind:       KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,details,client_account\n2026-05-31,100,Customer receipt,EE471000001020145685\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
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

func TestValidateBundleAcceptsAccountImporterAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "number,name,category,parent_account\n" +
				"1000,Cash,assets,\n" +
				"1100,Petty Cash,asset,1000\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
	require.Len(t, report.Files, 1)
	assert.Contains(t, report.Files[0].Headers, "code")
	assert.Contains(t, report.Files[0].Headers, "account_type")
	assert.Contains(t, report.Files[0].Headers, "parent_code")
	assert.NotContains(t, report.Files[0].Headers, "number")
	assert.NotContains(t, report.Files[0].Headers, "category")
	assert.NotContains(t, report.Files[0].Headers, "parent_account")
}

func TestValidateBundleReportsAccountParentAliasReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "number,name,category,parent\n" +
				"1000,Cash,asset,\n" +
				"1100,Petty Cash,asset,9999\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindAccounts, report.Issues[0].Kind)
	assert.Equal(t, KindAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "parent_code", report.Issues[0].Field)
	assert.Equal(t, "9999", report.Issues[0].Value)
}

func TestValidateBundleTreatsPreservedAccountIDsAndCodesSeparately(t *testing.T) {
	accountID := "11111111-1111-1111-1111-111111111111"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "id,account_code,account_name,type,parent_code\n" +
				accountID + ",1000,Cash,ASSET," + accountID + "\n",
		},
		{
			Kind:       KindOpeningBalances,
			FileName:   "opening.csv",
			CSVContent: "account_code,debit,credit\n" + accountID + ",100,0\n1000,0,100\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\n" +
				"JE-1,2026-05-30," + accountID + ",100,0\n" +
				"JE-1,2026-05-30,1000,0,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindAccounts, "parent_code", "reference")
	assertValidationIssue(t, report, KindOpeningBalances, "account_code", "reference")
	assertValidationIssue(t, report, KindJournalEntries, "account_code", "reference")
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
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cc_code,journal_line_id,allocation_amount,allocation_date\n" +
				"MISSING," + cutoverJournalLineID1 + ",125.50,2026-05-31\n",
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

func TestValidateBundleReportsInvalidCostAllocationJournalEntryLineIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostAllocations,
			FileName:   "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\nOPS,legacy-line,125.50,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindCostAllocations, "journal_entry_line_id", "journal_entry_line_id must be a valid UUID")
}

func TestValidateBundleChecksCostAllocationJournalLineReferences(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",1000,125.50,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",4000,0,125.50\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",125.50,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsCostAllocationsWithinJournalLineAmount(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Payroll expense,EXPENSE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\nADM,Administration\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",5500,100.00,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",1000,0,100.00\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",40.00,2026-05-31\n" +
				"ADM," + cutoverJournalLineID1 + ",50.00,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsCostAllocationsExceedJournalLineAmount(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Payroll expense,EXPENSE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\nADM,Administration\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",5500,100.00,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",1000,0,100.00\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",70.00,2026-05-31\n" +
				"ADM," + cutoverJournalLineID1 + ",40.00,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindCostAllocations, report.Issues[0].Kind)
	assert.Equal(t, KindJournalEntries, report.Issues[0].TargetKind)
	assert.Equal(t, "amount", report.Issues[0].Field)
	assert.Equal(t, "40", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "cost allocations for journal line")
	assert.Contains(t, report.Issues[0].Message, "allocations=110")
	assert.Contains(t, report.Issues[0].Message, "line_amount=100")
}

func TestValidateBundleAcceptsCostAllocationPercentagesWithinJournalLineLimit(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Payroll expense,EXPENSE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\nADM,Administration\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",5500,100.00,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",1000,0,100.00\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",40.00,40,2026-05-31\n" +
				"ADM," + cutoverJournalLineID1 + ",60.00,60,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsCostAllocationPercentagesExceedJournalLineLimit(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\nADM,Administration\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",30.00,70,2026-05-31\n" +
				"ADM," + cutoverJournalLineID1 + ",40.00,40,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindCostAllocations, report.Issues[0].Kind)
	assert.Equal(t, KindJournalEntries, report.Issues[0].TargetKind)
	assert.Equal(t, "allocation_percentage", report.Issues[0].Field)
	assert.Equal(t, "40", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "cost allocation percentages for journal line")
	assert.Contains(t, report.Issues[0].Message, "percentages=110")
	assert.Contains(t, report.Issues[0].Message, "limit=100")
}

func TestValidateBundleAcceptsCostAllocationAmountPercentageMatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Payroll expense,EXPENSE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\nADM,Administration\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",5500,125.50,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",1000,0,125.50\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",62.75,50,2026-05-31\n" +
				"ADM," + cutoverJournalLineID1 + ",62.75,50,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsCostAllocationAmountPercentageMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Payroll expense,EXPENSE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\nADM,Administration\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",5500,125.50,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",1000,0,125.50\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date\n" +
				"OPS," + cutoverJournalLineID1 + ",60.00,50,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindCostAllocations, report.Issues[0].Kind)
	assert.Equal(t, KindJournalEntries, report.Issues[0].TargetKind)
	assert.Equal(t, "amount/allocation_percentage", report.Issues[0].Field)
	assert.Equal(t, "amount=60 percentage=50", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "amount and percentage")
	assert.Contains(t, report.Issues[0].Message, "expected_amount=62.75")
	assert.Contains(t, report.Issues[0].Message, "line_amount=125.5")
}

func TestValidateBundleReportsMissingCostAllocationJournalLineReference(t *testing.T) {
	missingJournalLineID := "33333333-3333-4333-8333-333333333333"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
		},
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name\nOPS,Operations\n",
		},
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",1000,125.50,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID2 + ",4000,0,125.50\n",
		},
		{
			Kind:       KindCostAllocations,
			FileName:   "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\nOPS," + missingJournalLineID + ",125.50,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindCostAllocations, report.Issues[0].Kind)
	assert.Equal(t, KindJournalEntries, report.Issues[0].TargetKind)
	assert.Equal(t, "journal_entry_line_id", report.Issues[0].Field)
	assert.Equal(t, missingJournalLineID, report.Issues[0].Value)
}

func TestValidateBundleReportsInvalidJournalLineIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindJournalEntries,
			FileName:   "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\nJE-1,2026-05-31,legacy-line,1000,125.50,0\nJE-1,2026-05-31,,4000,0,125.50\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assertValidationIssue(t, report, KindJournalEntries, "line_id", "line_id must be a valid UUID")
}

func TestValidateBundleReportsDuplicateJournalLineIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindJournalEntries,
			FileName: "journals.csv",
			CSVContent: "entry_reference,entry_date,line_id,account_code,debit,credit\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",1000,125.50,0\n" +
				"JE-1,2026-05-31," + cutoverJournalLineID1 + ",4000,0,125.50\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assertValidationIssue(t, report, KindJournalEntries, "line_id", "duplicates row")
}

func TestValidateBundleReportsCostAllocationIDsThatMatchGeneratedCostCenterImportIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "id,cost_center_code,name\nlegacy-cc,SALES,Sales\nlegacy-cc,OPS,Operations\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,cost_center_id,journal_entry_line_id,amount,allocation_date\n" +
				"OPS,," + cutoverJournalLineID1 + ",125.50,2026-05-31\n" +
				",legacy-cc," + cutoverJournalLineID2 + ",75.00,2026-05-31\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindCostAllocations, "cost_center_id", "cost_center_id must be a valid UUID")
}

func TestValidateBundleReportsInvalidExistingCostCenterParentIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "code,name,parent_id,parent_code\nSALES,Sales,legacy-parent,\nONLINE,Online,,SALES\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindCostCenters, "parent_id", "parent_id must be a valid UUID")
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
				"OPS," + cutoverJournalLineID1 + ",nope,nope,\n",
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

func TestValidateBundleDoesNotTreatProductAndWarehouseImportIDsAsPreserved(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "id,product_code,name,sales_price\nlegacy-product,SKU-1,Widget,10\nlegacy-product,SKU-2,Gadget,11\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "id,warehouse_code,warehouse_name\nlegacy-warehouse,MAIN,Main warehouse\nlegacy-warehouse,SECOND,Secondary warehouse\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity\nSKU-2,SECOND,3\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsStockAdjustmentForTrackedGoodsProduct(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,product_type,track_inventory,sales_price\nSKU-GOODS,Widget,GOODS,true,10\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "code,name\nMAIN,Main warehouse\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity,unit_cost\nSKU-GOODS,MAIN,3,4.50\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsStockAdjustmentForUnstockableProduct(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,product_type,track_inventory,sales_price\n" +
				"SKU-SERVICE,Consulting,SERVICE,,10\n" +
				"SKU-NOTRACK,Widget,GOODS,false,10\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "code,name\nMAIN,Main warehouse\n",
		},
		{
			Kind:     KindStockAdjustments,
			FileName: "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity\n" +
				"SKU-SERVICE,MAIN,1\n" +
				"SKU-NOTRACK,MAIN,2\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Equal(t, KindStockAdjustments, report.Issues[0].Kind)
	assert.Equal(t, KindProducts, report.Issues[0].TargetKind)
	assert.Equal(t, "product_code", report.Issues[0].Field)
	assert.Equal(t, "SKU-SERVICE", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `stock adjustment product_code "SKU-SERVICE" references SERVICE product`)
	assert.Equal(t, KindStockAdjustments, report.Issues[1].Kind)
	assert.Equal(t, KindProducts, report.Issues[1].TargetKind)
	assert.Equal(t, "product_code", report.Issues[1].Field)
	assert.Equal(t, "SKU-NOTRACK", report.Issues[1].Value)
	assert.Contains(t, report.Issues[1].Message, `stock adjustment product_code "SKU-NOTRACK" references product with track_inventory=false`)
}

func TestValidateBundleReportsProductAccountReferenceIssues(t *testing.T) {
	accountID := "22222222-2222-2222-2222-222222222222"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" + accountID + ",4000,Sales,REVENUE\n33333333-3333-3333-3333-333333333333,1400,Inventory,ASSET\n",
		},
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "id,name\n11111111-1111-1111-1111-111111111111,Hardware\n",
		},
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,category_id,sales_price,sale_account_id,purchase_account_code,inventory_account_code\n" +
				"SKU-1,Widget,33333333-3333-3333-3333-333333333333,10,missing-sales,5999,1400\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindProducts, "category_id", "reference")
	assertValidationIssue(t, report, KindProducts, "sale_account_id", "sale_account_id must be a valid UUID")
	assertValidationIssue(t, report, KindProducts, "purchase_account_code", "reference")
}

func TestValidateBundleAcceptsProductAccountTypeReferences(t *testing.T) {
	saleAccountID := "11111111-1111-1111-1111-111111111111"
	purchaseAccountID := "22222222-2222-2222-2222-222222222222"
	inventoryAccountID := "33333333-3333-3333-3333-333333333333"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" +
				saleAccountID + ",4000,Sales,REVENUE\n" +
				purchaseAccountID + ",5500,Cost of goods sold,EXPENSE\n" +
				inventoryAccountID + ",1400,Inventory,ASSET\n",
		},
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,sales_price,sale_account_code,purchase_account_id,inventory_account_code\n" +
				"SKU-1,Widget,10,4000," + purchaseAccountID + ",1400\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsProductAccountTypeMismatches(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "account_code,account_name,type\n" +
				"4000,Sales holding,ASSET\n" +
				"5500,Purchase clearing,REVENUE\n" +
				"1400,Inventory expense,EXPENSE\n",
		},
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,sales_price,sale_account_code,purchase_account_code,inventory_account_code\n" +
				"SKU-1,Widget,10,4000,5500,1400\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindProducts, "sale_account_code", `sale account "4000" is ASSET; expected REVENUE account`)
	assertValidationIssue(t, report, KindProducts, "purchase_account_code", `purchase account "5500" is REVENUE; expected EXPENSE account`)
	assertValidationIssue(t, report, KindProducts, "inventory_account_code", `inventory account "1400" is EXPENSE; expected ASSET account`)
}

func TestValidateBundleAcceptsProductTypeValuesAndBlank(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,product_type,sales_price\n" +
				"SKU-GOODS,Widget, goods ,10\n" +
				"SKU-SERVICE,Consulting,sErViCe,0\n" +
				"SKU-BLANK,Untyped,,5\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsInvalidProductType(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,product_type,sales_price\nSKU-BUNDLE,Starter bundle,bundle,10\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProducts, report.Issues[0].Kind)
	assert.Equal(t, "product_type", report.Issues[0].Field)
	assert.Equal(t, "bundle", report.Issues[0].Value)
	assert.Equal(t, `invalid product_type "bundle"`, report.Issues[0].Message)
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

func TestValidateBundleAcceptsProductCategoryPreservedIDReferences(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProductCategories,
			FileName: "categories.csv",
			CSVContent: "category_id,category_name,parent_category_id\n" +
				"11111111-1111-1111-1111-111111111111,Hardware,\n" +
				"22222222-2222-2222-2222-222222222222,Widgets,11111111-1111-1111-1111-111111111111\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,category_id,sales_price\nSKU-1,Widget,22222222-2222-2222-2222-222222222222,10\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Empty(t, report.Issues)

	var categoriesValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindProductCategories {
			categoriesValidation = file
		}
	}
	require.Equal(t, KindProductCategories, categoriesValidation.Kind)
	assert.Contains(t, categoriesValidation.Headers, "id")
	assert.Contains(t, categoriesValidation.Headers, "parent_id")
	assert.NotContains(t, categoriesValidation.Headers, "category_id")
	assert.NotContains(t, categoriesValidation.Headers, "parent_category_id")
}

func TestValidateBundleReportsInvalidProductCategoryImportID(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "category_id,name\nlegacy-id,Hardware\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProductCategories, report.Issues[0].Kind)
	assert.Equal(t, "id", report.Issues[0].Field)
	assert.Equal(t, "legacy-id", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "valid UUID")
}

func TestValidateBundleReportsProductCategoryParentIDReferenceIssue(t *testing.T) {
	missingParentID := "33333333-3333-3333-3333-333333333333"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProductCategories,
			FileName:   "categories.csv",
			CSVContent: "category_id,name,parent_category_id\n11111111-1111-1111-1111-111111111111,Hardware," + missingParentID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProductCategories, report.Issues[0].Kind)
	assert.Equal(t, KindProductCategories, report.Issues[0].TargetKind)
	assert.Equal(t, "parent_id", report.Issues[0].Field)
	assert.Equal(t, missingParentID, report.Issues[0].Value)
}

func TestValidateBundleReportsProductCategoryIDReferenceWhenValueMatchesName(t *testing.T) {
	legacyCategoryID := "11111111-1111-1111-1111-111111111111"
	categoryNameThatLooksLikeID := "22222222-2222-2222-2222-222222222222"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProductCategories,
			FileName: "categories.csv",
			CSVContent: "category_id,category_name,parent_category_id\n" +
				legacyCategoryID + "," + categoryNameThatLooksLikeID + ",\n" +
				"33333333-3333-3333-3333-333333333333,Child," + categoryNameThatLooksLikeID + "\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,category_id,sales_price\nSKU-1,Widget," + categoryNameThatLooksLikeID + ",10\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindProductCategories, "parent_id", "reference")
	assertValidationIssue(t, report, KindProducts, "category_id", "reference")
}

func TestValidateBundleReportsInventoryRowValueIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindProducts,
			FileName: "products.csv",
			CSVContent: "code,name,product_type,sales_price,purchase_price,vat_rate,min_stock_level,reorder_point,track_inventory,status,is_active,lead_time_days\n" +
				"SKU-1,,asset,-1,nope,-22,-5,-1,sometimes,archived,maybe,-3\n" +
				"SKU-2,Widget,GOODS,10,5,22,0,0,true,ACTIVE,true,soon\n",
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
	assert.Equal(t, 24, report.Summary.ErrorCount)
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
	assertValidationIssue(t, report, KindProducts, "lead_time_days", "lead_time_days must be an integer")
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

func TestValidateBundleReportsSerializedStockImportIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "product_code,name,sales_price\nSKU-1,Widget,10\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "warehouse_code,warehouse_name\nMAIN,Main warehouse\n",
		},
		{
			Kind:     KindStockAdjustments,
			FileName: "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity,serial_number\n" +
				"SKU-1,MAIN,2,SN-001\n" +
				"SKU-1,MAIN,1,SN-001\n" +
				"SKU-1,MAIN,-1,SN-002\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindStockAdjustments, "serial_number", "serial_number requires quantity 1 or -1")
	assertValidationIssue(t, report, KindStockAdjustments, "product/serial_number", "duplicates row")
}

func TestValidateBundleReportsInvalidProductCategoryID(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,category_id,sales_price\nSKU-1,Widget,legacy-id,10\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProducts, report.Issues[0].Kind)
	assert.Equal(t, "category_id", report.Issues[0].Field)
	assert.Equal(t, "legacy-id", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, "valid UUID")
}

func TestValidateBundleReportsProductAndWarehouseIDValuesThatMatchCodes(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "product_code,name,sales_price\nSKU-1,Widget,10\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "warehouse_code,warehouse_name\nMAIN,Main warehouse\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_name,line_description,quantity,unit_price,vat_rate,product_id\nQ-1,2026-05-30,Customer One,Work,1,100,22,SKU-1\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_id,warehouse_id,quantity\nSKU-1,MAIN,5\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindQuotes, "product_id", "product_id must be a valid UUID")
	assertValidationIssue(t, report, KindStockAdjustments, "product_id", "product_id must be a valid UUID")
	assertValidationIssue(t, report, KindStockAdjustments, "warehouse_id", "warehouse_id must be a valid UUID")
}

func TestValidateBundleCanonicalizesImporterDescriptionMemoAliases(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindCostCenters,
			FileName:   "cost-centers.csv",
			CSVContent: "cost_center_code,name\nSALES,Sales\n",
		},
		{
			Kind:     KindCostAllocations,
			FileName: "cost-allocations.csv",
			CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date,description\n" +
				"SALES," + cutoverJournalLineID1 + ",125.50,2026-05-31,Shared rent\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "product_code,name,sales_price\nSKU-1,Widget,10\n",
		},
		{
			Kind:       KindWarehouses,
			FileName:   "warehouses.csv",
			CSVContent: "warehouse_code,warehouse_name\nMAIN,Main warehouse\n",
		},
		{
			Kind:       KindStockAdjustments,
			FileName:   "stock.csv",
			CSVContent: "product_code,warehouse_code,quantity,description\nSKU-1,MAIN,5,Opening stock\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Empty(t, report.Issues)

	var allocationValidation FileValidation
	var stockValidation FileValidation
	for _, file := range report.Files {
		switch file.Kind {
		case KindCostAllocations:
			allocationValidation = file
		case KindStockAdjustments:
			stockValidation = file
		}
	}

	require.Equal(t, KindCostAllocations, allocationValidation.Kind)
	assert.Contains(t, allocationValidation.Headers, "notes")
	assert.NotContains(t, allocationValidation.Headers, "description")

	require.Equal(t, KindStockAdjustments, stockValidation.Kind)
	assert.Contains(t, stockValidation.Headers, "reason")
	assert.NotContains(t, stockValidation.Headers, "description")
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
	assert.Equal(t, "product_code", report.Issues[0].Field)
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

func TestValidateBundleReportsOrderQuoteContactMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\nCUST-2,Customer Two\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_id,quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\n11111111-1111-1111-1111-111111111111,Q-1,2026-05-30,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_code,quote_id,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31,CUST-2,11111111-1111-1111-1111-111111111111,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindOrders, report.Issues[0].Kind)
	assert.Equal(t, KindQuotes, report.Issues[0].TargetKind)
	assert.Equal(t, "contact_code", report.Issues[0].Field)
	assert.Equal(t, "CUST-2", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `imported quote "Q-1" contact_code "CUST-1"`)
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
	assert.Equal(t, "asset_account_code", report.Issues[0].Field)
	assert.Equal(t, "9999", report.Issues[0].Value)
}

func TestValidateBundleAcceptsFixedAssetAccountTypeReferences(t *testing.T) {
	depreciationExpenseAccountID := "11111111-1111-1111-1111-111111111111"
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "id,account_code,account_name,type\n" +
				"22222222-2222-2222-2222-222222222222,1200,Fixed assets,ASSET\n" +
				depreciationExpenseAccountID + ",6200,Depreciation expense,EXPENSE\n" +
				"33333333-3333-3333-3333-333333333333,1290,Accumulated depreciation,ASSET\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,asset_account_code,depreciation_expense_account_id,accumulated_depreciation_account_code\nFA-1,Laptop,2026-05-30,1200,1200," + depreciationExpenseAccountID + ",1290\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsFixedAssetAccountTypeMismatches(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindAccounts,
			FileName: "accounts.csv",
			CSVContent: "account_code,account_name,type\n" +
				"1200,Fixed asset expense,EXPENSE\n" +
				"6200,Depreciation holding,ASSET\n" +
				"1290,Accumulated depreciation expense,EXPENSE\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,asset_account_code,depreciation_expense_account_code,accumulated_depreciation_account_code\nFA-1,Laptop,2026-05-30,1200,1200,6200,1290\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindFixedAssets, "asset_account_code", `asset account "1200" is EXPENSE; expected ASSET account`)
	assertValidationIssue(t, report, KindFixedAssets, "depreciation_expense_account_code", `depreciation expense account "6200" is ASSET; expected EXPENSE account`)
	assertValidationIssue(t, report, KindFixedAssets, "accumulated_depreciation_account_code", `accumulated depreciation account "1290" is EXPENSE; expected ASSET account`)
}

func TestValidateBundleAcceptsFixedAssetPurchaseDateFormats(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:     KindFixedAssets,
			FileName: "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost\n" +
				"FA-1,Laptop,2026-05-30,1200\n" +
				"FA-2,Lathe,2026-05-30T00:00:00Z,2400\n" +
				"FA-3,Desk,2026-06-01 15:04:05,300\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsMissingFixedAssetPurchaseDate(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost\nFA-1,Laptop,,1200\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, "purchase_date", report.Issues[0].Field)
	assert.Equal(t, "purchase_date is required", report.Issues[0].Message)
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
	require.Len(t, report.Files, 3)
	assert.Contains(t, report.Files[1].Headers, "issue_date")
	assert.Contains(t, report.Files[1].Headers, "due_date")
	assert.Contains(t, report.Files[1].Headers, "invoice_total")
	assert.Contains(t, report.Files[1].Headers, "currency")
}

func TestValidateBundleAcceptsSplitPaymentAllocationsWithinEInvoiceTotal(t *testing.T) {
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
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\n" +
				"MADE,2026-05-31,60,BILL-2026-001,60\n" +
				"MADE,2026-06-01,62,BILL-2026-001,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsPaymentAllocationBeforeEInvoiceIssueDate(t *testing.T) {
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
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nMADE,2026-03-14,100,BILL-2026-001,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "payment_date", report.Issues[0].Field)
	assert.Equal(t, "2026-03-14", report.Issues[0].Value)
	assert.Equal(t, KindEInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment_date "2026-03-14" cannot be before imported invoice "BILL-2026-001" issue_date "2026-03-15"`)
}

func TestValidateBundleReportsPaymentAllocationsExceedEInvoiceTotal(t *testing.T) {
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
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number\n" +
				"MADE,2026-05-31,100,BILL-2026-001\n" +
				"MADE,2026-06-01,30,BILL-2026-001\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 3, report.Issues[0].Row)
	assert.Equal(t, "allocation_amount", report.Issues[0].Field)
	assert.Equal(t, KindEInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, "payment allocations for invoice \"BILL-2026-001\" exceed imported invoice total")
	assert.Contains(t, report.Issues[0].Message, "allocations=130")
	assert.Contains(t, report.Issues[0].Message, "invoice_total=122")
}

func TestValidateBundleReportsPaymentTypeMismatchForImportedSalesInvoice(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nMADE,2026-05-31,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "payment_type", report.Issues[0].Field)
	assert.Equal(t, "MADE", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment_type "MADE" does not match imported sales invoice "INV-1"`)
	assert.Contains(t, report.Issues[0].Message, "expected RECEIVED")
}

func TestValidateBundleReportsPaymentTypeMismatchForImportedPurchaseInvoice(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nBILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,100,BILL-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "payment_type", report.Issues[0].Field)
	assert.Equal(t, "RECEIVED", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment_type "RECEIVED" does not match imported purchase invoice "BILL-1"`)
	assert.Contains(t, report.Issues[0].Message, "expected MADE")
}

func TestValidateBundleReportsPaymentTypeMismatchForDefaultPurchaseEInvoice(t *testing.T) {
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
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,100,BILL-2026-001,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "payment_type", report.Issues[0].Field)
	assert.Equal(t, "RECEIVED", report.Issues[0].Value)
	assert.Equal(t, KindEInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment_type "RECEIVED" does not match imported purchase invoice "BILL-2026-001"`)
	assert.Contains(t, report.Issues[0].Message, "expected MADE")
}

func TestValidateBundleAcceptsSalesEInvoicePaymentTypeOverride(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeBoth,
		EInvoiceInvoiceType: " sales ",
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
			{
				Kind:       KindPayments,
				FileName:   "payments.csv",
				CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,100,INV-2026-001,100\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsSalesEInvoicePaymentCustomerContactMatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeCustomer,
		EInvoiceInvoiceType: " sales ",
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
			{
				Kind:       KindPayments,
				FileName:   "payments.csv",
				CSVContent: "payment_type,payment_date,contact_reg_code,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,87654321,100,INV-2026-001,100\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsSalesEInvoicePaymentCustomerContactMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeCustomer,
		EInvoiceInvoiceType: " sales ",
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name,reg_code\nBuyer OÜ,87654321\nOther Buyer,99999999\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("INV-2026-001", "Seller OÜ", "12345678"),
			},
			{
				Kind:       KindPayments,
				FileName:   "payments.csv",
				CSVContent: "payment_type,payment_date,contact_reg_code,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,99999999,100,INV-2026-001,100\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "contact_reg_code", report.Issues[0].Field)
	assert.Equal(t, "99999999", report.Issues[0].Value)
	assert.Equal(t, KindEInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment contact_reg_code "99999999" does not match imported invoice "INV-2026-001" contact_reg_code "87654321"`)
}

func TestValidateBundleAcceptsCreditNoteEInvoicePaymentCustomerContactMatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceContactMode: EInvoiceContactModeCustomer,
		EInvoiceInvoiceType: " credit_note ",
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name,reg_code\nBuyer OÜ,87654321\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("CN-2026-001", "Seller OÜ", "12345678"),
			},
			{
				Kind:       KindPayments,
				FileName:   "payments.csv",
				CSVContent: "payment_type,payment_date,contact_reg_code,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,87654321,100,CN-2026-001,100\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsCreditNoteEInvoicePaymentSupplierContactMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceInvoiceType: " credit_note ",
		Files: []BundleFile{
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "name,reg_code\nSupplier OÜ,12345678\nOther Supplier,99999999\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("CN-2026-001", "Supplier OÜ", "12345678"),
			},
			{
				Kind:       KindPayments,
				FileName:   "payments.csv",
				CSVContent: "payment_type,payment_date,contact_reg_code,amount,invoice_number,allocation_amount\nMADE,2026-05-31,99999999,100,CN-2026-001,100\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "contact_reg_code", report.Issues[0].Field)
	assert.Equal(t, "99999999", report.Issues[0].Value)
	assert.Equal(t, KindEInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment contact_reg_code "99999999" does not match imported invoice "CN-2026-001" contact_reg_code "12345678"`)
}

func TestValidateBundleRejectsUnsupportedEInvoiceInvoiceTypeOverride(t *testing.T) {
	_, err := ValidateBundle(&ValidateBundleRequest{
		EInvoiceInvoiceType: "memo",
		Files: []BundleFile{{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
		}},
	})

	require.EqualError(t, err, `invalid e_invoice_invoice_type "memo"`)
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

func TestValidateBundleAcceptsSplitPaymentAllocationsWithinImportedInvoiceTotal(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\n" +
				"RECEIVED,2026-05-31,60,INV-1,60\n" +
				"RECEIVED,2026-06-01,62,INV-1,\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsPaymentContactMatchForImportedInvoice(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,contact_code,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,CUST-1,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsPaymentContactMismatchForImportedInvoice(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\nCUST-2,Customer Two\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,contact_code,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,CUST-2,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "contact_code", report.Issues[0].Field)
	assert.Equal(t, "CUST-2", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment contact_code "CUST-2" does not match imported invoice "INV-1" contact_code "CUST-1"`)
}

func TestValidateBundleAcceptsPaymentAllocationOnImportedInvoiceIssueDate(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-30,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsPaymentAllocationBeforeImportedInvoiceIssueDate(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-29,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "payment_date", report.Issues[0].Field)
	assert.Equal(t, "2026-05-29", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment_date "2026-05-29" cannot be before imported invoice "INV-1" issue_date "2026-05-30"`)
}

func TestValidateBundleReportsMalformedPaymentAllocationDateWithoutOrderingIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,not-a-date,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "payment_date", report.Issues[0].Field)
	assert.Equal(t, "not-a-date", report.Issues[0].Value)
	assert.Empty(t, report.Issues[0].TargetKind)
	assert.Equal(t, "payment_date must be YYYY-MM-DD or RFC3339", report.Issues[0].Message)
}

func TestValidateBundleAcceptsPaymentAllocationToSentImportedInvoice(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,SENT,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,100,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsPaymentAllocationToInactiveImportedInvoiceStatus(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,line_description,quantity,unit_price,vat_rate\n" +
				"INV-DRAFT,SALES,CUST-1,2026-05-30,2026-06-14,DRAFT,Work,1,100,22\n" +
				"INV-VOID,SALES,CUST-1,2026-05-30,2026-06-14,VOIDED,Work,1,100,22\n",
		},
		{
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\n" +
				"RECEIVED,2026-05-31,100,INV-DRAFT,100\n" +
				"RECEIVED,2026-05-31,100,INV-VOID,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "invoice_number", report.Issues[0].Field)
	assert.Equal(t, "INV-DRAFT", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `imported invoice "INV-DRAFT" with status DRAFT`)
	assert.Equal(t, KindPayments, report.Issues[1].Kind)
	assert.Equal(t, 3, report.Issues[1].Row)
	assert.Equal(t, "invoice_number", report.Issues[1].Field)
	assert.Equal(t, "INV-VOID", report.Issues[1].Value)
	assert.Equal(t, KindInvoices, report.Issues[1].TargetKind)
	assert.Contains(t, report.Issues[1].Message, `imported invoice "INV-VOID" with status VOIDED`)
}

func TestValidateBundleReportsPaymentAllocationToInactiveImportedInvoiceStatusByInvoiceID(t *testing.T) {
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
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,status,line_description,quantity,unit_price,vat_rate\n" + legacyInvoiceID + ",INV-VOID,SALES,CUST-1,2026-05-30,2026-06-14,VOIDED,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_id,allocation_amount\nRECEIVED,2026-05-31,100," + legacyInvoiceID + ",100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "invoice_id", report.Issues[0].Field)
	assert.Equal(t, legacyInvoiceID, report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `imported invoice "`+legacyInvoiceID+`" with status VOIDED`)
}

func TestValidateBundleAcceptsImportedInvoiceAmountPaidWithinTotal(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,PARTIALLY_PAID,60,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsCombinedImportedAmountPaidAndPaymentAllocationsWithinTotal(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,PARTIALLY_PAID,40,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,82,INV-1,82\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsImportedInvoiceAmountPaidExceedsTotal(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,PARTIALLY_PAID,130,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindInvoices, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "amount_paid", report.Issues[0].Field)
	assert.Contains(t, report.Issues[0].Message, `amount_paid for invoice "INV-1/SALES" exceeds imported invoice total`)
	assert.Contains(t, report.Issues[0].Message, "amount_paid=130")
	assert.Contains(t, report.Issues[0].Message, "invoice_total=122")
}

func TestValidateBundleReportsCombinedImportedAmountPaidAndPaymentAllocationsExceedTotal(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,PARTIALLY_PAID,80,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number,allocation_amount\nRECEIVED,2026-05-31,50,INV-1,50\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 2, report.Issues[0].Row)
	assert.Equal(t, "allocation_amount", report.Issues[0].Field)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `imported invoice paid amount plus payment allocations for invoice "INV-1" exceed imported invoice total`)
	assert.Contains(t, report.Issues[0].Message, "amount_paid=80")
	assert.Contains(t, report.Issues[0].Message, "allocations=50")
	assert.Contains(t, report.Issues[0].Message, "combined_paid=130")
	assert.Contains(t, report.Issues[0].Message, "invoice_total=122")
}

func TestValidateBundleReportsImportedInvoiceAmountPaidStatusMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,amount_paid,line_description,quantity,unit_price,vat_rate\n" +
				"INV-SENT,SALES,CUST-1,2026-05-30,2026-06-14,SENT,10,Work,1,100,22\n" +
				"INV-PAID,SALES,CUST-1,2026-05-30,2026-06-14,PAID,60,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assertValidationIssue(t, report, KindInvoices, "amount_paid", "amount_paid must be zero when status is SENT")
	assertValidationIssue(t, report, KindInvoices, "amount_paid", "must equal imported invoice total when status is PAID")
}

func TestValidateBundleReportsPaymentAllocationsExceedImportedInvoiceTotal(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number\n" +
				"RECEIVED,2026-05-31,100,INV-1\n" +
				"RECEIVED,2026-06-01,30,INV-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, 3, report.Issues[0].Row)
	assert.Equal(t, "allocation_amount", report.Issues[0].Field)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, "payment allocations for invoice \"INV-1\" exceed imported invoice total")
	assert.Contains(t, report.Issues[0].Message, "allocations=130")
	assert.Contains(t, report.Issues[0].Message, "invoice_total=122")
}

func TestValidateBundleReportsPaymentAllocationCurrencyMismatchForImportedInvoice(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,currency,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,USD,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,currency,invoice_number,allocation_amount\nRECEIVED,2026-05-31,100,EUR,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "currency", report.Issues[0].Field)
	assert.Equal(t, "EUR", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment currency "EUR" does not match imported invoice "INV-1" currency "USD"`)
}

func TestValidateBundleReportsPaymentAllocationCurrencyMismatchForEInvoice(t *testing.T) {
	eInvoiceXML := strings.Replace(
		cutoverEInvoiceXML("BILL-2026-001", "Supplier OÜ", "12345678"),
		"<Currency>EUR</Currency>",
		"<Currency>USD</Currency>",
		1,
	)
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "name,reg_code\nSupplier OÜ,12345678\n",
		},
		{
			Kind:       KindEInvoices,
			FileName:   "e-invoices.xml",
			XMLContent: eInvoiceXML,
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,currency,invoice_number,allocation_amount\nMADE,2026-05-31,100,EUR,BILL-2026-001,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "currency", report.Issues[0].Field)
	assert.Equal(t, "EUR", report.Issues[0].Value)
	assert.Equal(t, KindEInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `payment currency "EUR" does not match imported invoice "BILL-2026-001" currency "USD"`)
}

func TestValidateBundleReportsPaymentCurrencyCodeIssue(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,currency\nRECEIVED,2026-05-31,100,EU1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "currency", report.Issues[0].Field)
	assert.Equal(t, "EU1", report.Issues[0].Value)
	assert.Equal(t, "currency must be a 3-letter ISO code", report.Issues[0].Message)
}

func TestValidateBundleReportsAmbiguousPaymentInvoiceNumberReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\nSUP-1,Supplier One\n",
		},
		{
			Kind:     KindInvoices,
			FileName: "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,SALES,CUST-1,2026-05-30,2026-06-14,Sale,1,100,22\n" +
				"INV-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Purchase,1,50,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,invoice_number\nRECEIVED,2026-05-31,10,INV-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, "invoice_number", report.Issues[0].Field)
	assert.Equal(t, "INV-1", report.Issues[0].Value)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Contains(t, report.Issues[0].Message, `invoice_number "INV-1" matched multiple imported invoices`)
}

func TestValidateBundleAcceptsPaymentImporterReferenceAliases(t *testing.T) {
	legacyContactID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_id,contact_code,name\n" + legacyContactID + ",SUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Work,1,100,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,supplier_id,amount,method,description,invoice_no,allocation_amount\nMADE,2026-05-31," + legacyContactID + ",100,BANK_TRANSFER,Imported payment,INV-1,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 3, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)

	var paymentValidation FileValidation
	for _, file := range report.Files {
		if file.Kind == KindPayments {
			paymentValidation = file
		}
	}
	require.Equal(t, KindPayments, paymentValidation.Kind)
	assert.Contains(t, paymentValidation.Headers, "contact_id")
	assert.Contains(t, paymentValidation.Headers, "payment_method")
	assert.Contains(t, paymentValidation.Headers, "notes")
	assert.Contains(t, paymentValidation.Headers, "invoice_number")
	assert.NotContains(t, paymentValidation.Headers, "supplier_id")
	assert.NotContains(t, paymentValidation.Headers, "method")
	assert.NotContains(t, paymentValidation.Headers, "description")
	assert.NotContains(t, paymentValidation.Headers, "invoice_no")
}

func TestValidateBundleReportsPaymentCustomerAliasReferenceIssues(t *testing.T) {
	missingContactID := "22222222-2222-2222-2222-222222222222"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_id,name\n11111111-1111-1111-1111-111111111111,Customer One\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,customer_id,amount\nRECEIVED,2026-05-31," + missingContactID + ",100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "contact_id", report.Issues[0].Field)
	assert.Equal(t, missingContactID, report.Issues[0].Value)
}

func TestValidateBundleAcceptsPaymentContactIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,email\nCUST-1,Customer One,billing@example.test\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,contact_email,amount\nRECEIVED,2026-05-31,billing@example.test,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Zero(t, report.Summary.ErrorCount)
}

func TestValidateBundleAcceptsPaymentContactVATNumberReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,vat_number\nCUST-1,Customer One,EE123456789\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,contact_vat_number,amount\nRECEIVED,2026-05-31,EE123456789,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Zero(t, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsMissingPaymentContactIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,contact_name,amount\nRECEIVED,2026-05-31,Missing Customer,100\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "contact_name", report.Issues[0].Field)
	assert.Equal(t, "Missing Customer", report.Issues[0].Value)
}

func TestValidateBundleAcceptsExpenseContactIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,reg_code\nSUP-1,Supplier One,12345678\n",
		},
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Office expenses,EXPENSE\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount,contact_reg_code\n2026-05-30,Supplier One,5500,1000,42,12345678\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Zero(t, report.Summary.ErrorCount)
}

func TestValidateBundleReportsMissingExpenseContactIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Office expenses,EXPENSE\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount,contact_name\n2026-05-30,Supplier One,5500,1000,42,Missing Supplier\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindExpenses, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "contact_name", report.Issues[0].Field)
	assert.Equal(t, "Missing Supplier", report.Issues[0].Value)
}

func TestValidateBundleReportsPaymentBankAccountReferenceIssues(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nMain bank,EE471000001020145685,EUR\n",
		},
		{
			Kind:     KindPayments,
			FileName: "payments.csv",
			CSVContent: "payment_type,payment_date,amount,bank_account,currency\n" +
				"RECEIVED,2026-05-31,100,EE999,EUR\n" +
				"MADE,2026-05-31,50,EE471000001020145685,USD\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 2, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, KindBankAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "bank_account", report.Issues[0].Field)
	assert.Equal(t, "EE999", report.Issues[0].Value)
	assert.Equal(t, KindPayments, report.Issues[1].Kind)
	assert.Equal(t, KindBankAccounts, report.Issues[1].TargetKind)
	assert.Equal(t, "bank_account/currency", report.Issues[1].Field)
	assert.Equal(t, "EE471000001020145685/USD", report.Issues[1].Value)
	assert.Contains(t, report.Issues[1].Message, `currency "USD"`)
}

func TestValidateBundleAcceptsPaymentBankAccountDefaultCurrencyMatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nMain bank,EE471000001020145685,EUR\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,bank_account\nRECEIVED,2026-05-31,100,EE471000001020145685\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsPaymentBankAccountDefaultCurrencyMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindBankAccounts,
			FileName:   "bank-accounts.csv",
			CSVContent: "account_name,account_number,currency\nUSD bank,EE471000001020145685,USD\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,bank_account\nRECEIVED,2026-05-31,100,EE471000001020145685\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindPayments, report.Issues[0].Kind)
	assert.Equal(t, KindBankAccounts, report.Issues[0].TargetKind)
	assert.Equal(t, "bank_account/currency", report.Issues[0].Field)
	assert.Equal(t, "EE471000001020145685/EUR", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `bank_account "EE471000001020145685" uses currency "EUR"`)
	assert.Contains(t, report.Issues[0].Message, `bank_accounts file has currency "USD"`)
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
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + legacyInvoiceID + ",INV-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Work,1,1000,22\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,contact_id,invoice_id\nMADE,2026-05-31,100," + legacyContactID + "," + legacyInvoiceID + "\n",
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

func TestValidateBundleAcceptsProductSupplierCodeReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_code\nSKU-1,Widget,10,SUP-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsMissingProductSupplierCodeReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,SUP-404\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_code\nSKU-1,Widget,10,SUP-404\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProducts, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "supplier_code", report.Issues[0].Field)
	assert.Equal(t, "SUP-404", report.Issues[0].Value)
}

func TestValidateBundleAcceptsProductSupplierIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,reg_code,email\nSUP-1,Supplier One,12345678,billing@supplier.example\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_reg_code\nSKU-1,Widget,10,12345678\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsProductSupplierVATNumberReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,vat_number\nSUP-1,Supplier One,EE123456789\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_vat_number\nSKU-1,Widget,10,EE123456789\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestContactReferenceIndexSelectsVATAndFallbackIndexes(t *testing.T) {
	indexes := bundleIndexes{
		contacts:          map[string]bool{"fallback": true},
		contactVATNumbers: map[string]bool{"EE123456789": true},
	}

	assert.True(t, contactReferenceIndex(indexes, "contact_vat_number")["EE123456789"])
	assert.True(t, contactReferenceIndex(indexes, "contact_custom")["fallback"])
}

func TestSupplierContactIndexSelectsVATAndFallbackIndexes(t *testing.T) {
	indexes := bundleIndexes{
		contacts:          map[string]bool{"fallback": true},
		contactVATNumbers: map[string]bool{"EE123456789": true},
	}

	assert.True(t, supplierContactIndex(indexes, "supplier_vat_number")["EE123456789"])
	assert.True(t, supplierContactIndex(indexes, "supplier_custom")["fallback"])
}

func TestValidateBundleReportsMissingProductSupplierIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_name\nSKU-1,Widget,10,Missing Supplier\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindProducts, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "supplier_name", report.Issues[0].Field)
	assert.Equal(t, "Missing Supplier", report.Issues[0].Value)
}

func TestValidateBundleAcceptsFixedAssetSupplierCodeReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code\nFA-1,Laptop,2026-05-30,1200,SUP-1\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsMissingFixedAssetSupplierCodeReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,SUP-404\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code\nFA-1,Laptop,2026-05-30,1200,SUP-404\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "supplier_code", report.Issues[0].Field)
	assert.Equal(t, "SUP-404", report.Issues[0].Value)
}

func TestValidateBundleAcceptsFixedAssetSupplierIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,email\nSUP-1,Supplier One,billing@supplier.example\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_email\nFA-1,Laptop,2026-05-30,1200,billing@supplier.example\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleAcceptsFixedAssetSupplierSourceInvoiceMatch(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",BILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Laptop,1,1200,22\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\nFA-1,Laptop,2026-05-30,1200,SUP-1," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsFixedAssetSupplierSourceInvoiceMismatch(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\nSUP-2,Supplier Two\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",BILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Laptop,1,1200,22\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\nFA-1,Laptop,2026-05-30,1200,SUP-2," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Equal(t, "supplier_code", report.Issues[0].Field)
	assert.Equal(t, "SUP-2", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `fixed asset supplier_code "SUP-2" does not match imported invoice "`+sourceInvoiceID+`" contact_code "SUP-1"`)
}

func TestValidateBundleReportsFixedAssetSupplierSourceInvoiceIdentityMismatches(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		name              string
		invoiceField      string
		invoiceValue      string
		invoiceExtraField string
		invoiceExtraValue string
		assetField        string
		assetValue        string
	}{
		{
			name:              "supplier id",
			invoiceField:      "contact_id",
			invoiceValue:      "22222222-2222-2222-2222-222222222222",
			invoiceExtraField: "contact_code",
			invoiceExtraValue: "SUP-1",
			assetField:        "supplier_id",
			assetValue:        "33333333-3333-3333-3333-333333333333",
		},
		{
			name:         "supplier registry code",
			invoiceField: "contact_reg_code",
			invoiceValue: "12345678",
			assetField:   "supplier_reg_code",
			assetValue:   "87654321",
		},
		{
			name:         "supplier VAT number",
			invoiceField: "contact_vat_number",
			invoiceValue: "EE123456789",
			assetField:   "supplier_vat_number",
			assetValue:   "EE987654321",
		},
		{
			name:         "supplier email",
			invoiceField: "contact_email",
			invoiceValue: "billing@supplier.example",
			assetField:   "supplier_email",
			assetValue:   "assets@supplier.example",
		},
		{
			name:         "supplier name",
			invoiceField: "contact_name",
			invoiceValue: "Supplier One",
			assetField:   "supplier_name",
			assetValue:   "Supplier Two",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoiceExtraHeader := ""
			invoiceExtraValue := ""
			if tt.invoiceExtraField != "" {
				invoiceExtraHeader = "," + tt.invoiceExtraField
				invoiceExtraValue = "," + tt.invoiceExtraValue
			}
			report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
				{
					Kind:     KindInvoices,
					FileName: "invoices.csv",
					CSVContent: "invoice_id,invoice_number,invoice_type," + tt.invoiceField + invoiceExtraHeader + ",issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
						sourceInvoiceID + ",BILL-1,PURCHASE," + tt.invoiceValue + invoiceExtraValue + ",2026-05-30,2026-06-14,Laptop,1,1200,22\n",
				},
				{
					Kind:     KindFixedAssets,
					FileName: "assets.csv",
					CSVContent: "asset_number,name,purchase_date,purchase_cost," + tt.assetField + ",invoice_id\n" +
						"FA-1,Laptop,2026-05-30,1200," + tt.assetValue + "," + sourceInvoiceID + "\n",
				},
			}})

			require.NoError(t, err)
			require.NotNil(t, report)
			assert.False(t, report.Summary.Ready)
			assert.Equal(t, 1, report.Summary.ErrorCount)
			require.Len(t, report.Issues, 1)
			assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
			assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
			assert.Equal(t, tt.assetField, report.Issues[0].Field)
			assert.Equal(t, tt.assetValue, report.Issues[0].Value)
			assert.Contains(t, report.Issues[0].Message, `fixed asset `+tt.assetField+` "`+tt.assetValue+`" does not match imported invoice "`+sourceInvoiceID+`" `+tt.invoiceField+` "`+tt.invoiceValue+`"`)
		})
	}
}

func TestValidateBundleReportsFixedAssetSalesSourceInvoice(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",INV-1,SALES,SUP-1,2026-05-30,2026-06-14,Laptop,1,1200,22\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\nFA-1,Laptop,2026-05-30,1200,SUP-1," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Equal(t, "invoice_id", report.Issues[0].Field)
	assert.Equal(t, sourceInvoiceID, report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `fixed asset source invoice "`+sourceInvoiceID+`" is SALES; expected PURCHASE invoice`)
}

func TestValidateBundleAcceptsFixedAssetPurchaseDateAfterSourceInvoiceIssueDate(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",BILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Laptop,1,1200,22\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\nFA-1,Laptop,2026-05-31,1200,SUP-1," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsFixedAssetPurchaseDateBeforeSourceInvoiceIssueDate(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",BILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Laptop,1,1200,22\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\nFA-1,Laptop,2026-05-29,1200,SUP-1," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Equal(t, "purchase_date", report.Issues[0].Field)
	assert.Equal(t, "2026-05-29", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `fixed asset purchase_date "2026-05-29" cannot be before imported source invoice "`+sourceInvoiceID+`" issue_date "2026-05-30"`)
}

func TestValidateBundleAcceptsFixedAssetPurchaseCostsWithinSourceInvoiceTotal(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",BILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Hardware bundle,1,1000,22\n",
		},
		{
			Kind:     KindFixedAssets,
			FileName: "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\n" +
				"FA-1,Laptop,2026-05-30,600,SUP-1," + sourceInvoiceID + "\n" +
				"FA-2,Monitor,2026-05-30,500,SUP-1," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsFixedAssetPurchaseCostsExceedSourceInvoiceTotal(t *testing.T) {
	sourceInvoiceID := "11111111-1111-1111-1111-111111111111"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" + sourceInvoiceID + ",BILL-1,PURCHASE,SUP-1,2026-05-30,2026-06-14,Hardware bundle,1,1000,22\n",
		},
		{
			Kind:     KindFixedAssets,
			FileName: "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code,invoice_id\n" +
				"FA-1,Laptop,2026-05-30,800,SUP-1," + sourceInvoiceID + "\n" +
				"FA-2,Monitor,2026-05-30,500,SUP-1," + sourceInvoiceID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindInvoices, report.Issues[0].TargetKind)
	assert.Equal(t, "purchase_cost", report.Issues[0].Field)
	assert.Equal(t, "500", report.Issues[0].Value)
	assert.Contains(t, report.Issues[0].Message, `fixed asset purchase costs for source invoice "`+sourceInvoiceID+`" exceed imported invoice total`)
	assert.Contains(t, report.Issues[0].Message, "purchase_costs=1300")
	assert.Contains(t, report.Issues[0].Message, "invoice_total=1220")
}

func TestValidateBundleReportsMissingFixedAssetSupplierIdentityReference(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nSUP-1,Supplier One\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_name\nFA-1,Laptop,2026-05-30,1200,Missing Supplier\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, KindFixedAssets, report.Issues[0].Kind)
	assert.Equal(t, KindContacts, report.Issues[0].TargetKind)
	assert.Equal(t, "supplier_name", report.Issues[0].Field)
	assert.Equal(t, "Missing Supplier", report.Issues[0].Value)
}

func TestValidateBundleReportsCommercialContactIdentityFieldMismatch(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,issue_date,due_date,contact_code,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,2026-05-30,2026-06-14,Customer One,Work,1,100,22\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_code,line_description,quantity,unit_price,vat_rate\nQ-1,2026-05-30,Customer One,Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_code,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31,Customer One,Work,1,100,22\n",
		},
		{
			Kind:       KindRecurringInvoices,
			FileName:   "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_code,line_description,quantity,unit_price,vat_rate\nMonthly,MONTHLY,2026-06-01,Customer One,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindInvoices, "contact_code", "reference")
	assertValidationIssue(t, report, KindQuotes, "contact_code", "reference")
	assertValidationIssue(t, report, KindOrders, "contact_code", "reference")
	assertValidationIssue(t, report, KindRecurringInvoices, "contact_code", "reference")
}

func TestValidateBundleAcceptsCommercialContactIdentityFields(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name,email,reg_code,vat_number\nCUST-1,Customer One,customer@example.test,12345678,EE123456789\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,invoice_type,issue_date,due_date,contact_code,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,2026-05-30,2026-06-14,CUST-1,Work,1,100,22\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_reg_code,line_description,quantity,unit_price,vat_rate\nQ-1,2026-05-30,12345678,Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_email,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31,customer@example.test,Work,1,100,22\n",
		},
		{
			Kind:       KindRecurringInvoices,
			FileName:   "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_name,line_description,quantity,unit_price,vat_rate\nMonthly,MONTHLY,2026-06-01,Customer One,Work,1,100,22\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
}

func TestValidateBundleReportsContactIDReferenceWhenValueMatchesLookupField(t *testing.T) {
	legacyContactID := "11111111-1111-1111-1111-111111111111"
	contactCodeThatLooksLikeID := "22222222-2222-2222-2222-222222222222"

	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_id,contact_code,name\n" + legacyContactID + "," + contactCodeThatLooksLikeID + ",Supplier One\n",
		},
		{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "account_code,account_name,type\n1000,Cash,ASSET\n5500,Office expenses,EXPENSE\n",
		},
		{
			Kind:       KindExpenses,
			FileName:   "expenses.csv",
			CSVContent: "expense_date,merchant,expense_account_code,payment_account_code,amount,contact_id\n2026-05-30,Supplier One,5500,1000,42," + contactCodeThatLooksLikeID + "\n",
		},
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_type,payment_date,amount,contact_id\nRECEIVED,2026-05-31,100," + contactCodeThatLooksLikeID + "\n",
		},
		{
			Kind:       KindQuotes,
			FileName:   "quotes.csv",
			CSVContent: "quote_number,quote_date,contact_id,line_description,quantity,unit_price,vat_rate\nQ-1,2026-05-30," + contactCodeThatLooksLikeID + ",Work,1,100,22\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_id,line_description,quantity,unit_price,vat_rate\nSO-1,2026-05-31," + contactCodeThatLooksLikeID + ",Work,1,100,22\n",
		},
		{
			Kind:       KindRecurringInvoices,
			FileName:   "recurring.csv",
			CSVContent: "name,frequency,start_date,contact_id,line_description,quantity,unit_price,vat_rate\nMonthly,MONTHLY,2026-06-01," + contactCodeThatLooksLikeID + ",Work,1,100,22\n",
		},
		{
			Kind:       KindProducts,
			FileName:   "products.csv",
			CSVContent: "code,name,sales_price,supplier_id\nSKU-1,Widget,10," + contactCodeThatLooksLikeID + "\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_id\nFA-1,Laptop,2026-05-30,1200," + contactCodeThatLooksLikeID + "\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 7, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindExpenses, "contact_id", "reference")
	assertValidationIssue(t, report, KindPayments, "contact_id", "reference")
	assertValidationIssue(t, report, KindQuotes, "contact_id", "reference")
	assertValidationIssue(t, report, KindOrders, "contact_id", "reference")
	assertValidationIssue(t, report, KindRecurringInvoices, "contact_id", "reference")
	assertValidationIssue(t, report, KindProducts, "supplier_id", "reference")
	assertValidationIssue(t, report, KindFixedAssets, "supplier_id", "reference")
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

func TestValidateBundleReportsInvalidCutoverForeignKeyIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_number,payment_type,payment_date,amount,invoice_id\nPAY-1,RECEIVED,2026-05-31,100,legacy-invoice\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_name,line_description,quantity,unit_price,vat_rate,quote_id\nSO-1,2026-05-31,Customer One,Work,1,100,22,legacy-quote\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,category_id,invoice_id\nFA-1,Laptop,2026-05-30,1200,legacy-category,legacy-invoice\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 4, report.Summary.ErrorCount)
	assertValidationIssue(t, report, KindPayments, "invoice_id", "invoice_id must be a valid UUID")
	assertValidationIssue(t, report, KindOrders, "quote_id", "quote_id must be a valid UUID")
	assertValidationIssue(t, report, KindFixedAssets, "category_id", "category_id must be a valid UUID")
	assertValidationIssue(t, report, KindFixedAssets, "invoice_id", "invoice_id must be a valid UUID")
}

func TestValidateBundleAcceptsValidExternalCutoverForeignKeyIDs(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{
			Kind:       KindPayments,
			FileName:   "payments.csv",
			CSVContent: "payment_number,payment_type,payment_date,amount,invoice_id\nPAY-1,RECEIVED,2026-05-31,100,11111111-1111-1111-1111-111111111111\n",
		},
		{
			Kind:       KindOrders,
			FileName:   "orders.csv",
			CSVContent: "order_number,order_date,contact_name,line_description,quantity,unit_price,vat_rate,quote_id\nSO-1,2026-05-31,Customer One,Work,1,100,22,22222222-2222-2222-2222-222222222222\n",
		},
		{
			Kind:       KindFixedAssets,
			FileName:   "assets.csv",
			CSVContent: "asset_number,name,purchase_date,purchase_cost,category_id,invoice_id\nFA-1,Laptop,2026-05-30,1200,33333333-3333-3333-3333-333333333333,44444444-4444-4444-4444-444444444444\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Empty(t, report.Issues)
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

func TestDuplicatePeriodKeyHandlesMissingAndTextualPeriods(t *testing.T) {
	key, display, ok := duplicatePeriodKey(map[string]string{
		"year":  "2026",
		"month": "",
	}, "year", "month")
	assert.False(t, ok)
	assert.Empty(t, key)
	assert.Empty(t, display)

	key, display, ok = duplicatePeriodKey(map[string]string{
		"year":  "",
		"month": "5",
	}, "year", "month")
	assert.False(t, ok)
	assert.Empty(t, key)
	assert.Empty(t, display)

	key, display, ok = duplicatePeriodKey(map[string]string{
		"year":  "FY2026",
		"month": "05A",
	}, "year", "month")
	assert.True(t, ok)
	assert.Equal(t, "fy2026-05a", key)
	assert.Equal(t, "fy2026-05a", display)
}

func TestBundleValidationReportAddIssueTracksWarningsSeparately(t *testing.T) {
	var report BundleValidationReport

	report.addIssue(ValidationIssue{Severity: SeverityWarning, Message: "check this row"})
	report.addIssue(ValidationIssue{Severity: SeverityError, Message: "fix this row"})

	assert.Equal(t, 1, report.Summary.WarningCount)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 2)
}

func migrationRemediationActionCodes(actions []MigrationRemediationAction) []string {
	codes := make([]string, 0, len(actions))
	for _, action := range actions {
		codes = append(codes, action.Code)
	}
	return codes
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
