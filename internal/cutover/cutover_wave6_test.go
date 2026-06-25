package cutover

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/invoicing/mappers/einvoice"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWave6ExecutionRepositoryAndCanonicalizeEdges(t *testing.T) {
	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
		},
	}, true)
	ApplyMigrationExecutionResume(run, &MigrationExecutionRun{Steps: []MigrationExecutionStepRun{
		{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionResultFailed},
	}})
	assert.False(t, run.Summary.Resumed)
	assert.Equal(t, 0, run.Summary.ResumedStepCount)

	blocked := &MigrationExecutionRun{Summary: MigrationExecutionRunSummary{ValidationReady: false, PlanReady: false}}
	RefreshMigrationExecutionRunProgress(blocked)
	assert.Equal(t, "blocked", blocked.Summary.Status)

	config, err := pgxpool.ParseConfig("postgres://127.0.0.1:1/open_accounting_test?connect_timeout=1")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	defer pool.Close()
	assert.Panics(t, func() {
		NewMigrationExecutionRunRepository(pool)
	})

	_, err = canonicalizeCSVHeaders("\ufeff", fileSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "csv_content is required")
}

func TestWave6DerivedKMDAndCostAllocationGuards(t *testing.T) {
	headerSet := map[string]bool{"period_code": true, "period_year": true}
	headers := applyDerivedMigrationHeaders(KindPayrollHistory, headerSet, []string{"period_code", "period_year"})
	assert.Equal(t, []string{"period_code", "period_year", "period_month"}, headers)

	year, month, ok := migrationPeriodYearMonth(map[string]string{"period_code": "2026"})
	assert.False(t, ok)
	assert.Empty(t, year)
	assert.Empty(t, month)

	group := &kmdHistoryVATReconciliationGroup{period: "2026-05"}
	group.addDeclaredVAT(wave6ParsedFile(KindKMDHistory, "kmd.csv", []string{"year"}), parsedRow{number: 2}, "unexpected", decimal.NewFromInt(1))
	report := &BundleValidationReport{}
	group.validateVAT(report, "output", "total_output_vat",
		&kmdHistoryVATReconciliationValue{amount: decimal.NewFromInt(10), fileName: "kmd.csv", row: 2, field: "total_output_vat", value: "10"},
		false,
		decimal.Zero,
		false,
		&kmdHistoryVATReconciliationValue{amount: decimal.NewFromInt(11), fileName: "kmd.csv", row: 3, field: "tax_amount", value: "11"},
	)
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0].Message, "does not match KMD row 8")

	lineID := "11111111-1111-4111-8111-111111111111"
	files := []parsedFile{
		wave6ParsedFile(KindJournalEntries, "journals.csv", []string{"line_id", "debit", "credit"},
			map[string]string{"line_id": "bad", "debit": "10", "credit": "0"},
			map[string]string{"line_id": lineID, "debit": "bad", "credit": "0"},
			map[string]string{"line_id": "22222222-2222-4222-8222-222222222222", "debit": "0", "credit": "0"},
			map[string]string{"line_id": lineID, "debit": "100", "credit": "0"},
		),
		wave6ParsedFile(KindCostAllocations, "allocations.csv", []string{"journal_entry_line_id", "amount", "allocation_percentage"},
			map[string]string{"journal_entry_line_id": "bad", "amount": "10", "allocation_percentage": "10"},
			map[string]string{"journal_entry_line_id": lineID, "amount": "10", "allocation_percentage": "bad"},
		),
	}
	targets := buildCutoverJournalLineAmountTargets(files)
	require.Len(t, targets, 1)
	assert.Equal(t, decimal.NewFromInt(100).String(), targets[normalizedValue(lineID)].amount.String())

	report = &BundleValidationReport{}
	validateCostAllocationJournalLinePercentages(report, files)
	validateCostAllocationAmountPercentageConsistency(report, files)
	assert.Empty(t, report.Issues)
}

func TestWave6CrossFileInvoiceAndAssetGuards(t *testing.T) {
	invoiceTargets := map[string]cutoverInvoiceAllocationTarget{}
	addCutoverInvoiceAllocationTarget(invoiceTargets, KindInvoices, "invoice_number", " ", decimal.NewFromInt(100), decimal.Zero, false, "EUR", "SALES", "", time.Time{}, false, nil)
	assert.Empty(t, invoiceTargets)

	parsedFiles := []parsedFile{
		wave6ParsedFile(KindInvoices, "invoices.csv", []string{"invoice_number", "invoice_type", "issue_date", "contact_code", "quantity", "unit_price"},
			map[string]string{"invoice_number": "INV-1", "invoice_type": "SALES", "issue_date": "2026-05-10", "contact_code": "CUST-1", "quantity": "1", "unit_price": "100"},
		),
		wave6ParsedFile(KindPayments, "payments.csv", []string{"invoice_number", "amount"},
			map[string]string{"amount": "25"},
			map[string]string{"invoice_number": "INV-1", "amount": "bad"},
		),
		wave6ParsedFile(KindQuotes, "quotes.csv", []string{"id", "quote_number", "contact_code", "quantity", "unit_price"},
			map[string]string{"id": "33333333-3333-4333-8333-333333333333", "quote_number": "Q-1", "contact_code": "CUST-1", "quantity": "1", "unit_price": "100"},
		),
		wave6ParsedFile(KindOrders, "orders.csv", []string{"quote_id", "contact_code"},
			map[string]string{"quote_id": "not-a-uuid", "contact_code": "CUST-2"},
		),
	}
	report := &BundleValidationReport{}
	validateCrossFileConsistency(report, parsedFiles, EInvoiceContactModeBoth)
	assert.Empty(t, report.Issues)

	orderFile := wave6ParsedFile(KindOrders, "orders.csv", []string{"contact_code"}, nil)
	assert.False(t, hasOrderQuoteContactMismatch(report, orderFile, parsedRow{values: map[string]string{"contact_code": "CUST-1"}}, cutoverQuoteReferenceTarget{}))
	assert.False(t, hasOrderQuoteContactMismatch(report, orderFile, parsedRow{values: map[string]string{}}, cutoverQuoteReferenceTarget{
		contactReferences: cutoverContactReferences{"contact_code": {display: "CUST-1", normalized: "cust-1"}},
	}))

	assetFile := wave6ParsedFile(KindFixedAssets, "assets.csv", []string{"purchase_date", "supplier_code"}, nil)
	assetRow := parsedRow{number: 2, values: map[string]string{"purchase_date": "bad"}}
	assert.False(t, hasFixedAssetInvoiceSupplierMismatch(report, assetFile, assetRow, cutoverInvoiceAllocationTarget{
		contactReferences: cutoverContactReferences{"contact_code": {display: "CUST-1", normalized: "cust-1"}},
	}))
	assert.False(t, hasFixedAssetBeforeInvoiceIssueDate(report, assetFile, assetRow, cutoverInvoiceAllocationTarget{}))
	assert.False(t, hasFixedAssetBeforeInvoiceIssueDate(report, assetFile, assetRow, cutoverInvoiceAllocationTarget{
		issueDateSpecified: true,
		issueDate:          time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC),
	}))
}

func TestWave6PayrollKMDAndReferenceGuardBranches(t *testing.T) {
	report := &BundleValidationReport{}
	kmdFile := wave6ParsedFile(KindKMDHistory, "kmd.csv", []string{}, map[string]string{})
	kmdRow := kmdFile.rows[0]
	_, ok := checkKMDHistoryYear(report, kmdFile, kmdRow)
	assert.False(t, ok)
	_, ok = checkKMDHistoryMonth(report, kmdFile, kmdRow)
	assert.False(t, ok)
	checkKMDHistoryRowCode(report, kmdFile, kmdRow)
	assert.Empty(t, report.Issues)

	payrollFile := wave6ParsedFile(KindPayrollHistory, "payroll.csv", []string{"status", "payment_date", "gross_salary"},
		map[string]string{"status": "", "payment_date": "", "gross_salary": "bad"},
	)
	payrollRow := payrollFile.rows[0]
	_, ok = checkPayrollHistoryPeriodYear(report, payrollFile, payrollRow)
	assert.False(t, ok)
	_, ok = checkPayrollHistoryPeriodMonth(report, payrollFile, payrollRow)
	assert.False(t, ok)
	status, ok := checkPayrollHistoryStatus(report, payrollFile, payrollRow)
	assert.True(t, ok)
	assert.Equal(t, "PAID", status)
	_, ok = checkPayrollHistoryOptionalDate(report, payrollFile, payrollRow, "payment_date")
	assert.True(t, ok)
	checkPayrollHistoryRequiredPositiveDecimal(report, wave6ParsedFile(KindPayrollHistory, "payroll.csv", []string{}, map[string]string{}), payrollRow, "gross_salary")
	checkPayrollHistoryRequiredPositiveDecimal(report, payrollFile, payrollRow, "gross_salary")
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "gross_salary", report.Issues[0].Field)

	_, _, ok = payrollTSDHistoryCrossFileKey(parsedRow{values: map[string]string{"period_year": "2026"}})
	assert.False(t, ok)
	_, _, ok = payrollTSDHistoryCrossFileKey(parsedRow{values: map[string]string{"period_year": "2026", "period_month": "5"}})
	assert.False(t, ok)
	assert.Equal(t, "fallback", payrollTSDHistoryPeriodDisplay(map[string]string{"period_month": "bad"}, "fallback"))

	report = &BundleValidationReport{}
	checkContactIDReference(report, bundleIndexes{files: map[FileKind]bool{KindContacts: true}}, wave6ParsedFile(KindInvoices, "invoices.csv", []string{"contact_id"}, nil), parsedRow{values: map[string]string{"contact_id": ""}}, "contact_id")
	assert.Empty(t, report.Issues)
}

func TestWave6InvoicePaymentAndCommercialHelperBranches(t *testing.T) {
	report := &BundleValidationReport{}
	invoiceFile := wave6ParsedFile(KindInvoices, "invoices.csv", []string{"invoice_number", "issue_date", "due_date", "contact_code"},
		map[string]string{"invoice_number": "INV-1", "issue_date": "2026-05-10", "due_date": "2026-05-01", "contact_code": "CUST-1"},
	)
	checkInvoiceDocumentRow(report, invoiceFile, invoiceFile.rows[0])
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "due_date", report.Issues[0].Field)

	report = &BundleValidationReport{}
	recurringFile := wave6ParsedFile(KindRecurringInvoices, "recurring.csv", []string{"contact_code", "start_date", "end_date"},
		map[string]string{"contact_code": "CUST-1", "start_date": "2026-06-01", "end_date": "2026-05-01"},
	)
	checkRecurringDocumentRow(report, recurringFile, recurringFile.rows[0])
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "end_date", report.Issues[0].Field)

	report = &BundleValidationReport{}
	validateImportedInvoiceAmountPaidStatus(report, invoiceFile, cutoverInvoiceGroup{display: "INV-1"}, decimal.NewFromInt(100), decimal.Zero, parsedRow{number: 2}, true, "PARTIALLY_PAID", parsedRow{number: 3})
	require.Len(t, report.Issues, 1)
	assert.Contains(t, report.Issues[0].Message, "PARTIALLY_PAID")
	validateImportedInvoiceAmountPaidStatus(report, invoiceFile, cutoverInvoiceGroup{display: "INV-2"}, decimal.NewFromInt(100), decimal.Zero, parsedRow{number: 4}, false, "PAID", parsedRow{number: 5})
	assert.Len(t, report.Issues, 1)

	_, ok := cutoverEInvoiceRowTotal(parsedRow{values: map[string]string{"invoice_total": "0"}})
	assert.False(t, ok)
	_, ok = cutoverEInvoiceTotal(einvoice.Invoice{})
	assert.False(t, ok)
	_, ok = cutoverEInvoiceTotal(einvoice.Invoice{Lines: []einvoice.Line{{
		Quantity:        decimal.NewFromInt(1),
		UnitPrice:       decimal.NewFromInt(10),
		DiscountPercent: decimal.Zero,
		VATRate:         decimal.NewFromInt(-1),
	}}})
	assert.False(t, ok)

	assert.Equal(t, "CREDIT_NOTE", cutoverEInvoiceInvoiceType("credit_note", ""))
	assert.Equal(t, "bad", normalizeCutoverIntComparable("bad"))

	paymentFile := wave6ParsedFile(KindPayments, "payments.csv", []string{"invoice_number"}, nil)
	assert.False(t, hasPaymentInvoiceContactMismatch(report, paymentFile, parsedRow{values: map[string]string{"contact_code": "CUST-1"}}, cutoverInvoiceAllocationTarget{}))
	assert.False(t, hasPaymentBeforeInvoiceIssueDate(report, paymentFile, parsedRow{values: map[string]string{"payment_date": "2026-05-01"}}, cutoverInvoiceAllocationTarget{}))
	report = &BundleValidationReport{}
	assert.True(t, hasPaymentInvoiceStatusMismatch(report, paymentFile, parsedRow{number: 2, values: map[string]string{}}, cutoverInvoiceAllocationTarget{
		display:    "INV-1",
		status:     "DRAFT",
		targetKind: KindInvoices,
	}))
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "INV-1", report.Issues[0].Value)
}

func TestWave6FieldValidationGuardBranches(t *testing.T) {
	report := &BundleValidationReport{}
	contacts := wave6ParsedFile(KindContacts, "contacts.csv", []string{"credit_limit"}, map[string]string{"credit_limit": ""})
	checkContactCreditLimit(report, contacts, contacts.rows[0])

	leave := wave6ParsedFile(KindLeaveBalances, "leave.csv", []string{}, map[string]string{})
	checkLeaveBalanceYear(report, leave, leave.rows[0])

	commercial := wave6ParsedFile(KindInvoices, "commercial.csv", []string{"optional_date", "status", "exchange_rate", "active"}, map[string]string{
		"optional_date": "",
		"status":        "custom",
		"exchange_rate": "",
		"active":        "yes",
	})
	row := commercial.rows[0]
	_, ok := checkCommercialOptionalDate(report, commercial, row, "optional_date")
	assert.False(t, ok)
	checkCommercialStatus(report, commercial, row, "status", normalizeCutoverUpper)
	checkRecurringFrequency(report, commercial, row)
	checkCommercialOptionalPositiveDecimal(report, commercial, row, "exchange_rate")
	checkCommercialBool(report, commercial, row, "active")

	inventory := wave6ParsedFile(KindStockAdjustments, "stock.csv", []string{"unit_cost"}, map[string]string{"unit_cost": ""})
	checkInventoryNonNegativeDecimal(report, inventory, inventory.rows[0], "unit_cost", false)

	assets := wave6ParsedFile(KindFixedAssets, "assets.csv", []string{"purchase_cost", "residual_value"},
		map[string]string{"purchase_cost": "100", "residual_value": "120"},
	)
	checkFixedAssetRows(report, assets)

	expenses := wave6ParsedFile(KindExpenses, "expenses.csv", []string{"approved_at"}, map[string]string{"approved_at": ""})
	checkExpenseOptionalTimestamp(report, expenses, expenses.rows[0], "approved_at")

	checkRequiredCutoverFieldGroup(report, wave6ParsedFile(KindContacts, "contacts.csv", []string{}, map[string]string{}), parsedRow{values: map[string]string{}}, "contact_code", "contact_name")
	checkOpeningBalanceTotals(report, wave6ParsedFile(KindOpeningBalances, "opening.csv", []string{}, map[string]string{}))
	checkOpeningBalanceTotals(report, wave6ParsedFile(KindOpeningBalances, "opening.csv", []string{"debit", "credit"}))

	_, _, amountIssue := parseCutoverDebitCredit(parsedRow{values: map[string]string{"debit": "1", "credit": "bad"}})
	require.NotNil(t, amountIssue)
	assert.Equal(t, "credit", amountIssue.field)
	_, amountIssue = parseCutoverPositiveNormalizedDecimal("", "amount")
	require.NotNil(t, amountIssue)
	assert.Equal(t, "amount", amountIssue.field)

	require.Len(t, report.Issues, 1)
	assert.Equal(t, "residual_value", report.Issues[0].Field)
}

func wave6ParsedFile(kind FileKind, fileName string, headers []string, rowValues ...map[string]string) parsedFile {
	rows := make([]parsedRow, 0, len(rowValues))
	for index, values := range rowValues {
		if values == nil {
			continue
		}
		rows = append(rows, parsedRow{
			number: index + 2,
			values: values,
		})
	}
	return parsedFile{
		kind:     kind,
		fileName: fileName,
		headers:  headers,
		rows:     rows,
	}
}
