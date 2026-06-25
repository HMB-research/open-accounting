package cutover

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWave4CanonicalizeAndParseErrorBranches(t *testing.T) {
	_, err := CanonicalizeBundleFileCSV(BundleFile{
		Kind:       KindAccounts,
		FileName:   "accounts.csv",
		CSVContent: "\"unterminated",
	}, MigrationProviderPresetDirecto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse csv header")

	_, err = canonicalizeCSVHeaders("code\n\"unterminated", fileSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse csv row")

	_, validation, err := parseBundleFile(BundleFile{Kind: KindContacts, FileName: "contacts.csv"}, fileSpecs[KindContacts])
	require.Error(t, err)
	assert.Equal(t, "contacts.csv", validation.FileName)
	assert.Contains(t, err.Error(), "csv_content is required")

	_, _, err = parseBundleFile(BundleFile{
		Kind:       KindContacts,
		FileName:   "contacts.csv",
		CSVContent: "name\n\"unterminated",
	}, fileSpecs[KindContacts])
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse csv row 2")

	parsed, validation, err := parseBundleFile(BundleFile{
		Kind:       KindContacts,
		FileName:   "contacts.csv",
		CSVContent: ",name\n,Customer One\n,\n",
	}, fileSpecs[KindContacts])
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, validation.Headers)
	assert.Equal(t, 1, validation.Rows)
	require.Len(t, parsed.rows, 1)
	assert.Equal(t, "Customer One", parsed.rows[0].values["name"])

	payroll, validation, err := parseBundleFile(BundleFile{
		Kind:     KindPayrollHistory,
		FileName: "payroll.csv",
		CSVContent: "period_code,employee_number,gross_salary\n" +
			"2026-05-01,EMP-1,1000\n" +
			"202606,EMP-2,1200\n",
	}, fileSpecs[KindPayrollHistory])
	require.NoError(t, err)
	assert.Contains(t, validation.Headers, "period_year")
	assert.Contains(t, validation.Headers, "period_month")
	require.Len(t, payroll.rows, 2)
	assert.Equal(t, "2026", payroll.rows[0].values["period_year"])
	assert.Equal(t, "05", payroll.rows[0].values["period_month"])
	assert.Equal(t, "06", payroll.rows[1].values["period_month"])

	leave, _, err := parseBundleFile(BundleFile{
		Kind:       KindLeaveBalances,
		FileName:   "leave.csv",
		CSVContent: "balance_date,employee_number,absence_type_code,entitled_days\n2026,EMP-1,ANNUAL,28\n",
	}, fileSpecs[KindLeaveBalances])
	require.NoError(t, err)
	require.Len(t, leave.rows, 1)
	assert.Equal(t, "2026", leave.rows[0].values["year"])
}

func TestWave4RemediationActionsCoverOrderingAndMessages(t *testing.T) {
	assert.Nil(t, BuildMigrationRemediationActions(nil))
	assert.Nil(t, BuildMigrationRemediationActions(&BundleValidationReport{}))
	assert.Equal(t, "migration:fix-123:payments:-:field-9:-", migrationAssignmentKey(" Fix 123 ", KindPayments, "!!!", " Field 9 ", ""))
	assert.Equal(t, "-", normalizeAssignmentKeyPart("!!!"))

	report := &BundleValidationReport{Issues: []ValidationIssue{
		{Severity: SeverityWarning, Kind: KindPayments, Message: "manual warning"},
		{Severity: SeverityError, Kind: FileKind("legacy"), Message: "unsupported migration file kind"},
		{Severity: SeverityError, Kind: KindContacts, FileName: "a.csv", Message: "missing required column group: name"},
		{Severity: SeverityError, Kind: KindInvoices, FileName: "a.csv", Message: "invoice_number duplicates row 2"},
		{Severity: SeverityError, Kind: KindInvoices, FileName: "a.csv", Field: "id", Message: "id duplicates row 2"},
		{Severity: SeverityError, Kind: KindInvoices, FileName: "a.csv", Field: "contact_code", Message: "reference was not found"},
		{Severity: SeverityError, Kind: KindInvoices, FileName: "a.csv", Field: "contact_code", TargetKind: KindContacts, Message: "reference was not found"},
		{Severity: SeverityError, Kind: KindOrders, FileName: "b.csv", Message: "status must be consistent"},
		{Severity: SeverityError, Kind: KindAccounts, FileName: "c.csv", Message: "id must be a valid UUID"},
		{Severity: SeverityError, Kind: KindProducts, FileName: "d.csv", Field: "sales_price", Message: "sales_price must be positive"},
	}}

	actions := BuildMigrationRemediationActions(report)
	require.Len(t, actions, len(report.Issues))
	assert.Equal(t, "unsupported_file_kind", actions[0].Code)
	assert.Equal(t, FileKind("legacy"), actions[0].Kind)
	assert.Equal(t, "WARNING", actions[len(actions)-1].Severity)

	byCodeAndField := map[string]MigrationRemediationAction{}
	for _, action := range actions {
		byCodeAndField[action.Code+"|"+action.Field+"|"+string(action.TargetKind)] = action
	}
	assert.Contains(t, byCodeAndField["missing_required_columns||"].Message, "Required migration columns")
	assert.Contains(t, byCodeAndField["missing_reference|contact_code|"].Message, "unresolved migration references")
	assert.Contains(t, byCodeAndField["missing_reference|contact_code|contacts"].Message, "contacts")
	assert.Contains(t, byCodeAndField["duplicate_identifier||"].Message, "duplicate migration identifiers")
	assert.Contains(t, byCodeAndField["duplicate_identifier|id|"].Message, "duplicate values")
	assert.Contains(t, byCodeAndField["grouped_consistency||"].Message, "inconsistent grouped")
	assert.Contains(t, byCodeAndField["invalid_identifier||"].Message, "malformed migration IDs")
	assert.Contains(t, byCodeAndField["invalid_row_value|sales_price|"].Message, "sales_price")
	assert.Contains(t, byCodeAndField["warning_review||"].Message, "migration warnings")
}

func TestWave4ExecutionRunEdgeTransitions(t *testing.T) {
	plan := &MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{
			ValidationReady: true,
			Ready:           true,
			StepCount:       2,
			ReadyStepCount:  1,
		},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
			{StepNumber: 2, Kind: KindBankTransactions, FileName: "bank.csv", Status: MigrationExecutionStepNeedsContext, Message: "Select a bank account."},
		},
	}

	run := NewMigrationExecutionRun(plan, false)
	require.NotNil(t, run)
	assert.Equal(t, "needs_confirmation", run.Summary.Status)
	require.Len(t, run.Steps, 2)
	assert.Equal(t, MigrationExecutionResultPlanned, run.Steps[0].Status)
	assert.Equal(t, "Pass confirm=true to run this import.", run.Steps[0].Message)
	assert.Equal(t, MigrationExecutionResultSkipped, run.Steps[1].Status)
	assert.Equal(t, "Select a bank account.", run.Steps[1].Message)

	ApplyMigrationExecutionResume(run, &MigrationExecutionRun{Steps: []MigrationExecutionStepRun{
		{StepNumber: 2, Kind: KindBankTransactions, FileName: "bank.csv", Status: MigrationExecutionResultSucceeded},
	}})
	assert.False(t, run.Summary.Resumed)
	assert.Equal(t, 0, run.Summary.ResumedStepCount)

	completedAt := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	completeRun := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
		Steps:   []MigrationExecutionStep{{StepNumber: 1, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionStepReady}},
	}, true)
	CompleteMigrationExecutionStep(completeRun, 0, MigrationExecutionResultSucceeded, "done", "", nil, completedAt)
	require.NotNil(t, completeRun.Steps[0].StartedAt)
	assert.Equal(t, completedAt, *completeRun.Steps[0].StartedAt)
	assert.Equal(t, "succeeded", completeRun.Summary.Status)

	overCompleted := &MigrationExecutionRun{Summary: MigrationExecutionRunSummary{
		PlanReady:          true,
		ValidationReady:    true,
		StepCount:          1,
		SucceededStepCount: 3,
	}}
	RefreshMigrationExecutionRunProgress(overCompleted)
	assert.Equal(t, 0, overCompleted.Summary.RemainingStepCount)
}

func TestWave4RepositoryErrorBranches(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)

	t.Run("save returns run payload marshal errors", func(t *testing.T) {
		repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t))
		repo.now = func() time.Time { return now }
		run := NewMigrationExecutionRun(&MigrationExecutionPlan{
			Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
			Steps:   []MigrationExecutionStep{{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady}},
		}, true)
		run.Steps[0].Response = make(chan int)

		saved, err := repo.SaveExecutionRun(ctx, "tenant_schema", "tenant-1", "user-1", run)
		require.Error(t, err)
		assert.Nil(t, saved)
		assert.Contains(t, err.Error(), "marshal migration execution run")
	})

	t.Run("list returns payload parse errors", func(t *testing.T) {
		repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t, withCutoverDryRunRecords([]models.MigrationExecutionRunRecord{{
			ID:         "run-bad",
			TenantID:   "tenant-1",
			RunPayload: json.RawMessage(`{`),
			CreatedAt:  now,
			UpdatedAt:  now,
		}})))

		runs, err := repo.ListExecutionRuns(ctx, "tenant_schema", "tenant-1", MigrationExecutionRunFilter{Limit: 1})
		require.Error(t, err)
		assert.Nil(t, runs)
		assert.Contains(t, err.Error(), "parse migration execution run payload")
	})

	t.Run("get wraps query errors", func(t *testing.T) {
		repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t, withCutoverDryRunQueryError(assert.AnError)))

		loaded, err := repo.GetExecutionRun(ctx, "tenant_schema", "tenant-1", "run-1")
		require.Error(t, err)
		assert.Nil(t, loaded)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "get migration execution run")
	})
}

func TestWave4PlanAndPresetEdges(t *testing.T) {
	plan, err := BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		ProviderPreset: MigrationProviderPreset("unknown"),
		Files: []BundleFile{{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "name\nCustomer One\n",
		}},
	})
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "unsupported provider_preset")

	plan, err = BuildMigrationExecutionPlan(&PlanMigrationExecutionRequest{
		EInvoiceInvoiceType: "memo",
		Files: []BundleFile{{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "name\nCustomer One\n",
		}},
	})
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "invalid e_invoice_invoice_type")

	spec := migrationExecutionStepSpec(KindLeaveBalances, "leave.csv", &PlanMigrationExecutionRequest{})
	assert.Equal(t, "/api/v1/tenants/{tenantID}/leave-balances/import", spec.apiPath)
	assert.Equal(t, "oa leave balances import --file <leave.csv>", spec.cliCommand)
	assert.Equal(t, []FileKind{KindEmployees}, spec.dependsOn)

	assert.Nil(t, cloneRequiredColumnGroups(nil))
	groups := [][]string{{"code", "name"}}
	cloned := cloneRequiredColumnGroups(groups)
	require.Equal(t, groups, cloned)
	cloned[0][0] = "changed"
	assert.Equal(t, "code", groups[0][0])
}

func TestWave4ValidationDuplicateAndPeriodEdges(t *testing.T) {
	report, err := ValidateBundle(nil)
	require.Error(t, err)
	assert.Nil(t, report)

	report, err = ValidateBundle(&ValidateBundleRequest{})
	require.Error(t, err)
	assert.Nil(t, report)

	assert.Equal(t, "", normalizedDuplicateIdentifierValue(duplicateIdentifierSpec{normalize: func(string) string { return "" }}, "value"))
	assert.Equal(t, "name:mari maasikas", func() string {
		key, _, ok := duplicateEmployeeKey(map[string]string{"first_name": "Mari", "last_name": "Maasikas"})
		require.True(t, ok)
		return key
	}())
	_, _, ok := duplicateEmployeeKey(map[string]string{})
	assert.False(t, ok)
	_, _, ok = duplicateAbsenceTypeKey(map[string]string{})
	assert.False(t, ok)

	duplicateReport := &BundleValidationReport{}
	validateDuplicateIdentifierPreflight(duplicateReport, wave4ParsedFile(KindContacts, "contacts.csv", []string{"email"},
		map[string]string{"email": ""},
		map[string]string{"email": "INFO@EXAMPLE.COM"},
		map[string]string{"email": "info@example.com"},
	))
	require.Len(t, duplicateReport.Issues, 1)
	assert.Equal(t, "email", duplicateReport.Issues[0].Field)

	compositeReport := &BundleValidationReport{}
	validateCompositeDuplicatePreflight(compositeReport, wave4ParsedFile(KindPayrollHistory, "payroll.csv", []string{"period_year", "period_month", "employee_number"},
		map[string]string{"employee_number": "EMP-1"},
		map[string]string{"period_year": "2026", "period_month": "5"},
		map[string]string{"period_year": "2026", "period_month": "5", "employee_number": "EMP-1"},
		map[string]string{"period_year": "2026", "period_month": "05", "employee_number": "EMP-1"},
	))
	validateCompositeDuplicatePreflight(compositeReport, wave4ParsedFile(KindLeaveBalances, "leave.csv", []string{"year", "employee_number", "absence_type_code"},
		map[string]string{"employee_number": "EMP-1", "absence_type_code": "ANNUAL"},
		map[string]string{"year": "2026", "absence_type_code": "ANNUAL"},
		map[string]string{"year": "2026", "employee_number": "EMP-1"},
		map[string]string{"year": "2026", "employee_number": "EMP-1", "absence_type_code": "ANNUAL"},
		map[string]string{"year": "2026", "employee_number": "EMP-1", "absence_type_code": "annual"},
	))
	validateCompositeDuplicatePreflight(compositeReport, wave4ParsedFile(KindTSDHistory, "tsd.csv", []string{"period_year", "period_month", "employee_number"},
		map[string]string{"employee_number": "EMP-2"},
		map[string]string{"period_year": "2026", "period_month": "5"},
		map[string]string{"period_year": "2026", "period_month": "5", "employee_number": "EMP-2"},
		map[string]string{"period_year": "2026", "period_month": "05", "employee_number": "EMP-2"},
	))
	validateCompositeDuplicatePreflight(compositeReport, wave4ParsedFile(KindKMDHistory, "kmd.csv", []string{"year", "month", "row_code"},
		map[string]string{"row_code": "1"},
		map[string]string{"year": "2026", "month": "5"},
		map[string]string{"year": "2026", "month": "5", "row_code": "1"},
		map[string]string{"year": "2026", "month": "05", "row_code": "row_1"},
	))
	require.Len(t, compositeReport.Issues, 4)
	assert.Contains(t, compositeReport.Issues[0].Message, "payroll period")
	assert.Contains(t, compositeReport.Issues[1].Message, "leave-balance year")
	assert.Contains(t, compositeReport.Issues[2].Message, "TSD period")
	assert.Contains(t, compositeReport.Issues[3].Message, "KMD period")

	groupedReport := &BundleValidationReport{}
	validateGroupedDocumentPreservedIDs(groupedReport, wave4ParsedFile(KindAccounts, "accounts.csv", []string{"id"}, map[string]string{"id": "id-1"}))
	validateGroupedDocumentPreflight(groupedReport, wave4ParsedFile(KindAccounts, "accounts.csv", []string{"id"}, map[string]string{"id": "id-1"}))
	validateGroupedDocumentPreservedIDs(groupedReport, wave4ParsedFile(KindInvoices, "invoices.csv", []string{"id", "invoice_number", "invoice_type"},
		map[string]string{"id": "11111111-1111-4111-8111-111111111111"},
		map[string]string{"id": "11111111-1111-4111-8111-111111111111", "invoice_number": "INV-1", "invoice_type": "SALES"},
		map[string]string{"id": "11111111-1111-4111-8111-111111111111", "invoice_number": "INV-2", "invoice_type": "SALES"},
	))
	require.Len(t, groupedReport.Issues, 1)
	assert.Equal(t, "id", groupedReport.Issues[0].Field)
}

func TestWave4ValidationCrossFileAccountingEdges(t *testing.T) {
	t.Run("kmd vat reconciliation reports declared and total mismatches", func(t *testing.T) {
		report := &BundleValidationReport{}
		validateKMDHistoryVATReconciliation(report, []parsedFile{wave4ParsedFile(KindKMDHistory, "kmd.csv",
			[]string{"year", "month", "row_code", "status", "tax_amount", "total_output_vat", "total_input_vat"},
			map[string]string{"year": "2026", "month": "5", "row_code": "1", "status": "ACCEPTED", "tax_amount": "10", "total_output_vat": "9"},
			map[string]string{"year": "2026", "month": "5", "row_code": "8", "status": "ACCEPTED", "tax_amount": "12"},
			map[string]string{"year": "2026", "month": "5", "row_code": "4", "status": "ACCEPTED", "tax_amount": "5", "total_input_vat": "4"},
			map[string]string{"year": "2026", "month": "5", "row_code": "9", "status": "ACCEPTED", "tax_amount": "6"},
			map[string]string{"year": "bad", "month": "5", "row_code": "1", "status": "ACCEPTED", "tax_amount": "1"},
			map[string]string{"year": "2026", "month": "5", "row_code": "2", "status": "ACCEPTED", "tax_amount": "bad"},
		)})
		require.Len(t, report.Issues, 4)
		assert.Contains(t, wave4IssueMessages(report.Issues), "supporting KMD output VAT rows")
		assert.Contains(t, wave4IssueMessages(report.Issues), "supporting KMD input VAT rows")
	})

	t.Run("cost allocation skip and mismatch branches", func(t *testing.T) {
		lineID := "11111111-1111-4111-8111-111111111111"
		report := &BundleValidationReport{}
		files := []parsedFile{
			wave4ParsedFile(KindJournalEntries, "journal.csv", []string{"line_id", "debit", "credit"},
				map[string]string{"line_id": "", "debit": "100", "credit": "0"},
				map[string]string{"line_id": "bad", "debit": "100", "credit": "0"},
				map[string]string{"line_id": lineID, "debit": "100", "credit": "0"},
			),
			wave4ParsedFile(KindCostAllocations, "allocations.csv", []string{"journal_entry_line_id", "amount", "allocation_percentage"},
				map[string]string{"journal_entry_line_id": "", "amount": "10", "allocation_percentage": "10"},
				map[string]string{"journal_entry_line_id": "22222222-2222-4222-8222-222222222222", "amount": "10", "allocation_percentage": "10"},
				map[string]string{"journal_entry_line_id": lineID, "amount": "bad", "allocation_percentage": "10"},
				map[string]string{"journal_entry_line_id": lineID, "amount": "60", "allocation_percentage": "50"},
				map[string]string{"journal_entry_line_id": lineID, "amount": "60", "allocation_percentage": "60"},
			),
		}
		validateCostAllocationJournalLineTotals(report, files)
		validateCostAllocationJournalLinePercentages(report, files)
		validateCostAllocationAmountPercentageConsistency(report, files)
		require.Len(t, report.Issues, 3)
		assert.Contains(t, wave4IssueMessages(report.Issues), "exceed imported journal line amount")
		assert.Contains(t, wave4IssueMessages(report.Issues), "exceed 100")
		assert.Contains(t, wave4IssueMessages(report.Issues), "disagree")
	})

	t.Run("product stockability and invoice allocation helper branches", func(t *testing.T) {
		targets := buildCutoverProductStockabilityTargets([]parsedFile{wave4ParsedFile(KindProducts, "products.csv",
			[]string{"code", "product_type", "track_inventory"},
			map[string]string{"product_type": "GOODS", "track_inventory": "yes"},
			map[string]string{"code": "BADTYPE", "product_type": "bundle", "track_inventory": "yes"},
			map[string]string{"code": "BADBOOL", "product_type": "GOODS", "track_inventory": "maybe"},
			map[string]string{"code": "SERVICE", "product_type": "SERVICE"},
			map[string]string{"code": "GOODS", "product_type": "GOODS"},
		)})
		require.Len(t, targets, 2)
		assert.False(t, targets["service"].trackInventory)
		assert.True(t, targets["goods"].trackInventory)

		row := parsedRow{number: 2, values: map[string]string{"invoice_number": "INV-1", "amount": "20"}}
		targetMap := map[string]cutoverInvoiceAllocationTarget{
			cutoverInvoiceAllocationTargetKey("invoice_number", "INV-1"): {key: "invoice_number:inv-1", display: "INV-1"},
		}
		target, ok := cutoverPaymentAllocationTarget(targetMap, row)
		require.True(t, ok)
		assert.Equal(t, "INV-1", target.display)
		amount, ok := cutoverPaymentAllocationAmount(row)
		require.True(t, ok)
		assert.True(t, amount.Equal(decimal.NewFromInt(20)))
		_, ok = cutoverPaymentAllocationTarget(targetMap, parsedRow{values: map[string]string{}})
		assert.False(t, ok)
	})
}

func TestWave4ValidationInvoicePaymentAndAssetHelpers(t *testing.T) {
	file := wave4ParsedFile(KindPayments, "payments.csv", []string{"invoice_number"}, nil)
	row := parsedRow{number: 3, values: map[string]string{
		"invoice_number": "INV-1",
		"payment_date":   "2026-05-01",
		"payment_type":   "MADE",
		"currency":       "USD",
		"contact_code":   "PAY-CUST",
	}}
	target := cutoverInvoiceAllocationTarget{
		key:                "invoice_number:inv-1",
		display:            "INV-1",
		total:              decimal.NewFromInt(100),
		currency:           "EUR",
		invoiceType:        "SALES",
		status:             "DRAFT",
		issueDate:          time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC),
		issueDateSpecified: true,
		contactReferences:  cutoverContactReferences{"contact_code": {display: "INV-CUST", normalized: "inv-cust"}},
		targetKind:         KindInvoices,
	}

	report := &BundleValidationReport{}
	assert.False(t, hasPaymentInvoiceCurrencyMismatch(report, file, row, cutoverInvoiceAllocationTarget{}))
	assert.True(t, hasPaymentInvoiceCurrencyMismatch(report, file, row, target))
	assert.True(t, hasPaymentInvoiceTypeMismatch(report, file, row, target))
	assert.True(t, hasPaymentInvoiceContactMismatch(report, file, row, target))
	assert.True(t, hasPaymentBeforeInvoiceIssueDate(report, file, row, target))
	assert.True(t, hasPaymentInvoiceStatusMismatch(report, file, row, target))
	assert.Len(t, report.Issues, 5)

	report = &BundleValidationReport{}
	assert.False(t, hasPaymentInvoiceTypeMismatch(report, file, parsedRow{values: map[string]string{"payment_type": "OTHER"}}, cutoverInvoiceAllocationTarget{invoiceType: "MEMO"}))
	assert.False(t, hasPaymentInvoiceContactMismatch(report, file, parsedRow{values: map[string]string{}}, target))
	assert.False(t, hasPaymentBeforeInvoiceIssueDate(report, file, parsedRow{values: map[string]string{"payment_date": "bad"}}, target))
	assert.False(t, hasPaymentInvoiceStatusMismatch(report, file, parsedRow{values: map[string]string{}}, cutoverInvoiceAllocationTarget{status: "PAID"}))
	assert.Empty(t, report.Issues)

	group := cutoverInvoiceGroup{display: "INV-1", rows: []parsedRow{
		{number: 2, values: map[string]string{"amount_paid": "bad", "status": "unknown"}},
	}}
	_, _, amountOK := cutoverInvoiceGroupAmountPaid(group)
	assert.False(t, amountOK)
	_, _, statusOK := cutoverInvoiceGroupStatus(group)
	assert.False(t, statusOK)
	assert.Equal(t, "", cutoverInvoiceRowStatus(parsedRow{values: map[string]string{"status": "unknown"}}))
	assert.Equal(t, "EUR", cutoverInvoiceRowCurrency(parsedRow{values: map[string]string{"currency": ""}}))

	assetFile := wave4ParsedFile(KindFixedAssets, "assets.csv", []string{"invoice_number"}, nil)
	assetRow := parsedRow{number: 4, values: map[string]string{
		"invoice_number":    "INV-1",
		"supplier_code":     "ASSET-SUP",
		"purchase_date":     "2026-05-01",
		"purchase_cost":     "0",
		"supplier_reg_code": "ASSET-REG",
	}}
	assetTarget := target
	assetTarget.invoiceType = "SALES"
	assetTarget.contactReferences = cutoverContactReferences{
		"contact_code":     {display: "INV-SUP", normalized: "inv-sup"},
		"contact_reg_code": {display: "INV-REG", normalized: "inv-reg"},
	}

	report = &BundleValidationReport{}
	assert.True(t, hasFixedAssetInvoiceTypeMismatch(report, assetFile, assetRow, assetTarget, "invoice_number", "INV-1"))
	assetTarget.invoiceType = "PURCHASE"
	assert.True(t, hasFixedAssetInvoiceSupplierMismatch(report, assetFile, assetRow, assetTarget))
	assetTarget.contactReferences = nil
	assert.False(t, hasFixedAssetInvoiceSupplierMismatch(report, assetFile, assetRow, assetTarget))
	assert.True(t, hasFixedAssetBeforeInvoiceIssueDate(report, assetFile, assetRow, assetTarget))
	_, ok := cutoverFixedAssetPurchaseCost(assetRow)
	assert.False(t, ok)
	assert.Len(t, report.Issues, 3)
	assert.Equal(t, "supplier_code", cutoverSupplierContactSourceField("contact_code"))
	assert.Equal(t, "custom", cutoverSupplierContactSourceField("custom"))
	assert.Nil(t, copyCutoverContactReferences(nil))
}

func wave4ParsedFile(kind FileKind, fileName string, headers []string, rowValues ...map[string]string) parsedFile {
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

func wave4IssueMessages(issues []ValidationIssue) string {
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	return strings.Join(messages, "\n")
}
