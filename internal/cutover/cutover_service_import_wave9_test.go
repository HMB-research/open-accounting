package cutover

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCutoverWave9ValidateBundleSortsIssueSeverity(t *testing.T) {
	report, err := ValidateBundle(&ValidateBundleRequest{Files: []BundleFile{
		{Kind: KindContacts, FileName: "b.csv", CSVContent: "contact_code\nC-1\n"},
		{Kind: FileKind("unsupported"), FileName: "a.csv", CSVContent: "x\n1\n"},
	}})

	require.NoError(t, err)
	require.NotEmpty(t, report.Issues)
	for i := 1; i < len(report.Issues); i++ {
		if report.Issues[i-1].Severity == report.Issues[i].Severity {
			assert.LessOrEqual(t, report.Issues[i-1].FileName, report.Issues[i].FileName)
		}
	}
}

func TestCutoverWave9ParseAndDuplicateGuards(t *testing.T) {
	_, validation, err := parseBundleFile(BundleFile{Kind: KindContacts, FileName: "contacts.csv", CSVContent: `"unterminated`}, fileSpecs[KindContacts])
	require.ErrorContains(t, err, "parse csv header")
	assert.Equal(t, "contacts.csv", validation.FileName)

	report := &BundleValidationReport{}
	validateDuplicateIdentifierPreflight(report, wave6ParsedFile(KindContacts, "contacts.csv", []string{"contact_code"},
		map[string]string{"contact_code": " "},
	))
	assert.Empty(t, report.Issues)
}

func TestCutoverWave9AccountingHelperSkips(t *testing.T) {
	lineID := "11111111-1111-4111-8111-111111111111"
	targets := buildCutoverJournalLineAmountTargets([]parsedFile{
		wave6ParsedFile(KindJournalEntries, "journal.csv", []string{"line_id", "debit", "credit"},
			map[string]string{"line_id": " ", "debit": "10", "credit": "0"},
			map[string]string{"line_id": "bad", "debit": "10", "credit": "0"},
			map[string]string{"line_id": lineID, "debit": "0", "credit": "0"},
		),
	})
	assert.Empty(t, targets)

	accountType, ok := cutoverNormalizedAccountType("income")
	require.True(t, ok)
	assert.Equal(t, "REVENUE", accountType)

	accountType, ok = cutoverNormalizedAccountType("ASSET")
	require.True(t, ok)
	assert.Equal(t, "ASSET", accountType)
}

func TestCutoverWave9CrossFileSkipBranches(t *testing.T) {
	report := &BundleValidationReport{}
	validateKMDHistoryVATReconciliation(report, []parsedFile{
		wave6ParsedFile(KindKMDHistory, "kmd.csv", []string{"year", "month", "row_code", "tax_amount"},
			map[string]string{"year": "", "month": "", "row_code": "1", "tax_amount": "22"},
		),
	}, nil)
	assert.Empty(t, report.Issues)

	report = &BundleValidationReport{}
	validatePayrollTSDHistoryConsistency(report, []parsedFile{
		wave6ParsedFile(KindPayrollHistory, "payroll.csv", []string{"employee_code", "period_code", "gross_salary"},
			map[string]string{"employee_code": "", "period_code": "2026-05", "gross_salary": "100"},
			map[string]string{"employee_code": "EMP-1", "period_code": "2026-05", "gross_salary": ""},
		),
		wave6ParsedFile(KindTSDHistory, "tsd.csv", []string{"employee_code", "period_code", "gross_salary"},
			map[string]string{"employee_code": "", "period_code": "2026-05", "gross_salary": "100"},
		),
	})
	assert.Empty(t, report.Issues)

	duplicates := payrollTSDHistoryDuplicateKeys([]parsedFile{
		wave6ParsedFile(KindTSDHistory, "tsd.csv", []string{"employee_code", "period_code"},
			map[string]string{"employee_code": "", "period_code": "2026-05"},
		),
	}, KindTSDHistory)
	assert.Empty(t, duplicates)
}

func TestCutoverWave9InvoiceHelpers(t *testing.T) {
	report := &BundleValidationReport{}
	validateImportedInvoiceAmountPaidConsistency(report, []parsedFile{
		wave6ParsedFile(KindInvoices, "invoices.csv", []string{"invoice_number", "amount_paid", "quantity", "unit_price", "vat_rate"},
			map[string]string{"invoice_number": "INV-1", "amount_paid": "10", "quantity": "1", "unit_price": "10", "vat_rate": "0"},
		),
	})
	assert.Empty(t, report.Issues)

	currency := cutoverInvoiceRowCurrency(parsedRow{values: map[string]string{"currency": ""}})
	assert.Equal(t, "EUR", currency)

	report = &BundleValidationReport{}
	file := wave6ParsedFile(KindOrders, "orders.csv", []string{"status"}, map[string]string{"status": ""})
	checkCommercialStatus(report, file, file.rows[0], "status", normalizeCutoverUpper, "PENDING")
	assert.Empty(t, report.Issues)
}

func TestCutoverWave9AmountHelpersKeepPositiveTargets(t *testing.T) {
	lineID := "22222222-2222-4222-8222-222222222222"
	targets := buildCutoverJournalLineAmountTargets([]parsedFile{
		wave6ParsedFile(KindJournalEntries, "journal.csv", []string{"line_id", "debit", "credit"},
			map[string]string{"line_id": lineID, "debit": "12.50", "credit": "0"},
		),
	})
	require.Contains(t, targets, normalizedValue(lineID))
	assert.True(t, targets[normalizedValue(lineID)].amount.Equal(decimal.RequireFromString("12.50")))
}
