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
	assert.Equal(t, KindBankAccounts, plan.Steps[1].Kind)
	assert.Equal(t, KindBankTransactions, plan.Steps[2].Kind)
	assert.Equal(t, KindOpeningBalances, plan.Steps[3].Kind)

	bankStep := plan.Steps[2]
	assert.Equal(t, MigrationExecutionStepNeedsContext, bankStep.Status)
	assert.Equal(t, []string{"bank_transaction_account_id"}, bankStep.ContextFields)
	assert.Equal(t, "/api/v1/tenants/{tenantID}/bank-accounts/<bank-account-id>/import", bankStep.APIPath)
	assert.Contains(t, bankStep.CLICommand, "oa banking transactions import --account-id <bank-account-id>")
	assert.Equal(t, []FileKind{KindBankAccounts}, bankStep.DependsOn)

	openingStep := plan.Steps[3]
	assert.Equal(t, MigrationExecutionStepReady, openingStep.Status)
	assert.Contains(t, openingStep.CLICommand, "oa journal import-opening-balances --entry-date 2026-01-01")
	assert.Empty(t, openingStep.ContextFields)
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
