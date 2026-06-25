package cutover

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMigrationExecutionPlanOrdersReadyStepsAndMarksMissingContext(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		Files: []BundleFile{
			{
				Kind:       KindOpeningBalances,
				FileName:   "opening.csv",
				CSVContent: "account_code,debit,credit\n1000,100,0\n3000,0,100\n",
			},
			{
				Kind:       KindBankTransactions,
				FileName:   "bank.csv",
				CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n",
			},
			{
				Kind:       KindBankAccounts,
				FileName:   "bank-accounts.csv",
				CSVContent: "name,account_number\nMain,EE471000001020145685\n",
			},
			{
				Kind:       KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n3000,Equity,EQUITY\n",
			},
		},
		OpeningBalanceEntryDate: "2026-01-01",
		BankTransactionFormat:   "lhv",
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Summary.ValidationReady)
	assert.False(t, plan.Summary.Ready)
	assert.Equal(t, 4, plan.Summary.StepCount)
	assert.Equal(t, 3, plan.Summary.ReadyStepCount)
	assert.Equal(t, 1, plan.Summary.NeedsContextCount)
	assert.Equal(t, 0, plan.Summary.BlockedStepCount)
	require.Len(t, plan.Steps, 4)
	assert.Equal(t, KindAccounts, plan.Steps[0].Kind)
	assert.Equal(t, KindOpeningBalances, plan.Steps[1].Kind)
	assert.Equal(t, KindBankAccounts, plan.Steps[2].Kind)
	assert.Equal(t, KindBankTransactions, plan.Steps[3].Kind)

	openingStep := plan.Steps[1]
	assert.Equal(t, MigrationExecutionStepReady, openingStep.Status)
	assert.Equal(t, []FileKind{KindAccounts}, openingStep.DependsOn)
	assert.Contains(t, openingStep.CLICommand, "oa journal import-opening-balances --entry-date 2026-01-01")
	assert.Empty(t, openingStep.ContextFields)

	bankStep := plan.Steps[3]
	assert.Equal(t, MigrationExecutionStepNeedsContext, bankStep.Status)
	assert.Equal(t, []string{"bank_transaction_account_id"}, bankStep.ContextFields)
	assert.Equal(t, "/api/v1/tenants/{tenantID}/bank-accounts/<bank-account-id>/import", bankStep.APIPath)
	assert.Contains(t, bankStep.CLICommand, "oa banking transactions import --account-id <bank-account-id>")
	assert.Contains(t, bankStep.CLICommand, "--format lhv")
	assert.Equal(t, []FileKind{KindBankAccounts}, bankStep.DependsOn)
}

func TestBuildMigrationExecutionPlanNilRequestAndHelperFallbacks(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(nil)
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "migration execution plan request is required")

	files := []BundleFile{
		{Kind: FileKind("custom"), FileName: "z.csv"},
		{Kind: KindContacts, FileName: "contacts.csv"},
		{Kind: FileKind("custom"), FileName: "a.csv"},
	}
	sorted := sortedExecutionFiles(files)
	require.Len(t, sorted, 3)
	assert.Equal(t, KindContacts, sorted[0].Kind)
	assert.Equal(t, "a.csv", sorted[1].FileName)
	assert.Equal(t, "z.csv", sorted[2].FileName)

	unknownSpec := migrationExecutionStepSpec(FileKind("custom"), "", &PlanMigrationExecutionRequest{})
	assert.Equal(t, []string{"supported_migration_file_kind"}, unknownSpec.contextFields)
	assert.Equal(t, "oa migration validate --provider-preset generic --json", unknownSpec.cliCommand)
	assert.Contains(t, unknownSpec.message, "supported migration import kind")

	assert.Equal(t, "<e_invoices.xml>", shellPlaceholderFile("", KindEInvoices))
	assert.Equal(t, "<invoices.csv>", shellPlaceholderFile(" invoices.csv ", KindInvoices))
}

func TestMigrationExecutionStepSpecAdditionalKinds(t *testing.T) {
	req := &PlanMigrationExecutionRequest{
		EInvoiceInvoiceType:      " purchase ",
		BankTransactionAccountID: "bank-1",
		BankTransactionFormat:    "lhv",
		OpeningBalanceEntryDate:  "2026-01-01",
	}
	tests := []struct {
		name       string
		kind       FileKind
		fileName   string
		wantPath   string
		wantCLI    string
		wantDeps   []FileKind
		wantFields []string
	}{
		{name: "expenses", kind: KindExpenses, wantPath: "/api/v1/tenants/{tenantID}/expenses/import", wantCLI: "oa expenses import --file <expenses.csv>", wantDeps: []FileKind{KindAccounts, KindContacts, KindEmployees}},
		{name: "invoices", kind: KindInvoices, wantPath: "/api/v1/tenants/{tenantID}/invoices/import", wantCLI: "oa invoices import --file <invoices.csv>", wantDeps: []FileKind{KindContacts, KindProducts}},
		{name: "e-invoices", kind: KindEInvoices, fileName: "e.xml", wantPath: "/api/v1/tenants/{tenantID}/invoices/import-einvoice", wantCLI: "oa invoices import-einvoice --file <e.xml> --invoice-type PURCHASE", wantDeps: []FileKind{KindContacts}},
		{name: "payments", kind: KindPayments, wantPath: "/api/v1/tenants/{tenantID}/payments/import", wantCLI: "oa payments import --file <payments.csv>", wantDeps: []FileKind{KindContacts, KindInvoices, KindBankAccounts}},
		{name: "quotes", kind: KindQuotes, wantPath: "/api/v1/tenants/{tenantID}/quotes/import", wantCLI: "oa quotes import --file <quotes.csv>", wantDeps: []FileKind{KindContacts, KindProducts}},
		{name: "orders", kind: KindOrders, wantPath: "/api/v1/tenants/{tenantID}/orders/import", wantCLI: "oa orders import --file <orders.csv>", wantDeps: []FileKind{KindContacts, KindProducts, KindQuotes}},
		{name: "recurring invoices", kind: KindRecurringInvoices, wantPath: "/api/v1/tenants/{tenantID}/recurring-invoices/import", wantCLI: "oa recurring-invoices import --file <recurring_invoices.csv>", wantDeps: []FileKind{KindContacts, KindProducts, KindAccounts}},
		{name: "cost centers", kind: KindCostCenters, wantPath: "/api/v1/tenants/{tenantID}/cost-centers/import", wantCLI: "oa cost-centers import --file <cost_centers.csv>"},
		{name: "cost allocations", kind: KindCostAllocations, wantPath: "/api/v1/tenants/{tenantID}/cost-centers/allocations/import", wantCLI: "oa cost-centers allocations import --file <cost_allocations.csv>", wantDeps: []FileKind{KindCostCenters, KindJournalEntries}},
		{name: "product categories", kind: KindProductCategories, wantPath: "/api/v1/tenants/{tenantID}/product-categories/import", wantCLI: "oa inventory categories import --file <product_categories.csv>"},
		{name: "warehouses", kind: KindWarehouses, wantPath: "/api/v1/tenants/{tenantID}/warehouses/import", wantCLI: "oa inventory warehouses import --file <warehouses.csv>"},
		{name: "products", kind: KindProducts, wantPath: "/api/v1/tenants/{tenantID}/products/import", wantCLI: "oa inventory products import --file <products.csv>", wantDeps: []FileKind{KindProductCategories, KindContacts}},
		{name: "fixed assets", kind: KindFixedAssets, wantPath: "/api/v1/tenants/{tenantID}/assets/import", wantCLI: "oa assets import --file <fixed_assets.csv>", wantDeps: []FileKind{KindAccounts, KindContacts, KindInvoices}},
		{name: "journal entries", kind: KindJournalEntries, wantPath: "/api/v1/tenants/{tenantID}/journal-entries/import", wantCLI: "oa journal import --file <journal_entries.csv>", wantDeps: []FileKind{KindAccounts}},
		{name: "bank transactions with account", kind: KindBankTransactions, wantPath: "/api/v1/tenants/{tenantID}/bank-accounts/bank-1/import", wantCLI: "oa banking transactions import --account-id bank-1 --file <bank_transactions.csv> --format lhv", wantDeps: []FileKind{KindBankAccounts}},
		{name: "opening balances with date", kind: KindOpeningBalances, wantPath: "/api/v1/tenants/{tenantID}/journal-entries/import-opening-balances", wantCLI: "oa journal import-opening-balances --entry-date 2026-01-01 --file <opening_balances.csv>", wantDeps: []FileKind{KindAccounts}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := migrationExecutionStepSpec(tt.kind, tt.fileName, req)
			assert.Equal(t, tt.wantPath, spec.apiPath)
			assert.Equal(t, tt.wantCLI, spec.cliCommand)
			assert.Equal(t, tt.wantDeps, spec.dependsOn)
			assert.Equal(t, tt.wantFields, spec.contextFields)
			assert.NotEmpty(t, spec.message)
		})
	}
}

func TestBuildMigrationExecutionPlanRejectsInvalidOpeningBalanceEntryDate(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		Files: []BundleFile{
			{
				Kind:       KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n3000,Equity,EQUITY\n",
			},
			{
				Kind:       KindOpeningBalances,
				FileName:   "opening.csv",
				CSVContent: "account_code,debit,credit\n1000,100,0\n3000,0,100\n",
			},
		},
		OpeningBalanceEntryDate: "01-01-2026",
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Summary.ValidationReady)
	assert.False(t, plan.Summary.Ready)
	assert.Equal(t, 2, plan.Summary.StepCount)
	assert.Equal(t, 1, plan.Summary.ReadyStepCount)
	assert.Equal(t, 1, plan.Summary.NeedsContextCount)
	require.Len(t, plan.Steps, 2)

	openingStep := plan.Steps[1]
	assert.Equal(t, KindOpeningBalances, openingStep.Kind)
	assert.Equal(t, MigrationExecutionStepNeedsContext, openingStep.Status)
	assert.Equal(t, []string{"opening_balance_entry_date"}, openingStep.ContextFields)
	assert.Contains(t, openingStep.CLICommand, "--entry-date <YYYY-MM-DD>")
	assert.NotContains(t, openingStep.CLICommand, "01-01-2026")
}

func TestBuildMigrationExecutionPlanBlocksStepsWhenValidationFails(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		Files: []BundleFile{{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code\nCUST-1\n",
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.False(t, plan.Summary.ValidationReady)
	assert.False(t, plan.Summary.Ready)
	assert.Equal(t, 1, plan.Summary.BlockedStepCount)
	require.Len(t, plan.Steps, 1)
	assert.Equal(t, MigrationExecutionStepBlocked, plan.Steps[0].Status)
	assert.Contains(t, plan.Steps[0].CLICommand, "oa migration validate --contacts")
	assert.NotEmpty(t, plan.RemediationActions)
}

func TestBuildMigrationExecutionPlanTSDHistoryDependsOnPayrollHistory(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		Files: []BundleFile{
			{
				Kind:       KindTSDHistory,
				FileName:   "tsd-history.csv",
				CSVContent: "period_year,period_month,employee_number,gross_payment\n2026,5,EMP-1,2500\n",
			},
			{
				Kind:       KindPayrollHistory,
				FileName:   "payroll-history.csv",
				CSVContent: "period_year,period_month,employee_number,gross_salary\n2026,5,EMP-1,2500\n",
			},
			{
				Kind:       KindEmployees,
				FileName:   "employees.csv",
				CSVContent: "employee_number,first_name,last_name,start_date\nEMP-1,Mari,Maasikas,2026-01-01\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Summary.Ready)
	require.Len(t, plan.Steps, 3)
	assert.Equal(t, KindEmployees, plan.Steps[0].Kind)
	assert.Equal(t, KindPayrollHistory, plan.Steps[1].Kind)
	tsdStep := plan.Steps[2]
	assert.Equal(t, KindTSDHistory, tsdStep.Kind)
	assert.Equal(t, MigrationExecutionStepReady, tsdStep.Status)
	assert.Equal(t, []FileKind{KindEmployees, KindPayrollHistory}, tsdStep.DependsOn)
	assert.Contains(t, tsdStep.Message, "employees and payroll history")
}

func TestBuildMigrationExecutionPlanKMDHistoryDependsOnVATHistory(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		EInvoiceContactMode: EInvoiceContactModeBoth,
		EInvoiceInvoiceType: "purchase",
		Files: []BundleFile{
			{
				Kind:       KindKMDHistory,
				FileName:   "kmd-history.csv",
				CSVContent: "year,month,row_code,tax_base,tax_amount\n2026,5,1,100,22\n",
			},
			{
				Kind:       KindJournalEntries,
				FileName:   "journals.csv",
				CSVContent: "entry_reference,entry_date,account_code,debit,credit\nVAT-ADJ,2026-05-31,1000,22,0\nVAT-ADJ,2026-05-31,4000,0,22\n",
			},
			{
				Kind:       KindEInvoices,
				FileName:   "e-invoices.xml",
				XMLContent: cutoverEInvoiceXML("BILL-2026-001", "Supplier OU", "12345678"),
			},
			{
				Kind:       KindInvoices,
				FileName:   "invoices.csv",
				CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,product_code\nINV-1,SALES,CUST-1,2026-05-30,2026-06-14,Work,1,100,22,SKU-1\n",
			},
			{
				Kind:       KindProducts,
				FileName:   "products.csv",
				CSVContent: "product_code,name,sales_price\nSKU-1,Widget,10\n",
			},
			{
				Kind:       KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "contact_code,name,reg_code\nCUST-1,Customer One,87654321\nSUP-1,Supplier OU,12345678\n",
			},
			{
				Kind:       KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "account_code,name,account_type\n1000,Cash,ASSET\n4000,Sales,REVENUE\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Summary.Ready)
	require.Len(t, plan.Steps, 7)
	assert.Equal(t, KindAccounts, plan.Steps[0].Kind)
	assert.Equal(t, KindContacts, plan.Steps[1].Kind)
	assert.Equal(t, KindProducts, plan.Steps[2].Kind)
	assert.Equal(t, KindInvoices, plan.Steps[3].Kind)
	assert.Equal(t, KindEInvoices, plan.Steps[4].Kind)
	assert.Equal(t, KindJournalEntries, plan.Steps[5].Kind)

	kmdStep := plan.Steps[6]
	assert.Equal(t, KindKMDHistory, kmdStep.Kind)
	assert.Equal(t, MigrationExecutionStepReady, kmdStep.Status)
	assert.Equal(t, []FileKind{KindInvoices, KindEInvoices, KindJournalEntries}, kmdStep.DependsOn)
	assert.Contains(t, kmdStep.Message, "invoice, e-invoice, and journal VAT history")
}

func TestBuildMigrationExecutionPlanStockAdjustmentsDependOnlyOnInventoryMasters(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		Files: []BundleFile{
			{
				Kind:       KindStockAdjustments,
				FileName:   "stock.csv",
				CSVContent: "product_code,warehouse_code,quantity,description\nSKU-1,MAIN,5,Opening stock\n",
			},
			{
				Kind:       KindWarehouses,
				FileName:   "warehouses.csv",
				CSVContent: "warehouse_code,warehouse_name\nMAIN,Main warehouse\n",
			},
			{
				Kind:       KindProducts,
				FileName:   "products.csv",
				CSVContent: "product_code,name,sales_price\nSKU-1,Widget,10\n",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Summary.Ready)
	require.Len(t, plan.Steps, 3)
	assert.Equal(t, KindWarehouses, plan.Steps[0].Kind)
	assert.Equal(t, KindProducts, plan.Steps[1].Kind)
	stockStep := plan.Steps[2]
	assert.Equal(t, KindStockAdjustments, stockStep.Kind)
	assert.Equal(t, MigrationExecutionStepReady, stockStep.Status)
	assert.Equal(t, []FileKind{KindProducts, KindWarehouses}, stockStep.DependsOn)
	assert.NotContains(t, stockStep.DependsOn, KindCostCenters)
	assert.Contains(t, stockStep.Message, "products and warehouses")
}

func TestBuildMigrationExecutionPlanIncludesEInvoiceInvoiceTypeOverride(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
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
		},
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.Summary.Ready)
	require.Len(t, plan.Steps, 2)
	eInvoiceStep := plan.Steps[1]
	assert.Equal(t, KindEInvoices, eInvoiceStep.Kind)
	assert.Equal(t, MigrationExecutionStepReady, eInvoiceStep.Status)
	assert.Contains(t, eInvoiceStep.CLICommand, "oa invoices import-einvoice --file <e-invoices.xml> --invoice-type SALES")
}
