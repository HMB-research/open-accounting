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
			CSVContent: "expense_date,merchant,expense_account_id,payment_account_id,amount\n2026-05-30,Office Store,expense-account,cash-account,42\n",
		},
		{
			Kind:       KindInvoices,
			FileName:   "invoices.csv",
			CSVContent: "invoice_number,contact_code,issue_date,line_description,quantity,unit_price,vat_rate\nINV-1,CUST-1,2026-05-30,Work,1,100,22\n",
		},
		{
			Kind:       KindPayrollHistory,
			FileName:   "payroll.csv",
			CSVContent: "year,month,employee_number,gross_salary\n2026,5,EMP-1,2500\n",
		},
		{
			Kind:       KindOpeningBalances,
			FileName:   "opening.csv",
			CSVContent: "account_code,debit,credit\n1000,100,0\n",
		},
	}})

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.True(t, report.Summary.Ready)
	assert.Equal(t, 0, report.Summary.ErrorCount)
	assert.Equal(t, 8, report.Summary.RowsValidated)
	assert.Empty(t, report.Issues)
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
