package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMigrationBundleHandler(t *testing.T) {
	h := &Handlers{}
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/validate", cutover.ValidateBundleRequest{
		Files: []cutover.BundleFile{
			{
				Kind:       cutover.KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "contact_code,name\nCUST-1,Customer One\n",
			},
			{
				Kind:       cutover.KindInvoices,
				FileName:   "invoices.csv",
				CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\nINV-1,SALES,CUST-404,2026-05-30,2026-06-14,Work,1,100,22\n",
			},
		},
	}, claims), map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.ValidateMigrationBundle(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var report cutover.BundleValidationReport
	require.NoError(t, json.NewDecoder(w.Body).Decode(&report))
	assert.False(t, report.Summary.Ready)
	assert.Equal(t, 1, report.Summary.ErrorCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, cutover.KindContacts, report.Issues[0].TargetKind)
	require.NotEmpty(t, report.RemediationActions)
	assert.Contains(t, migrationRemediationCodes(report.RemediationActions), "missing_reference")
	assert.Equal(t, "migration_cutover", report.RemediationActions[0].WorkspaceQueue)
	assert.Equal(t, "high", report.RemediationActions[0].Priority)
	assert.NotEmpty(t, report.RemediationActions[0].AssignmentKey)
}

func TestValidateMigrationBundleHandlerRejectsEmptyRequest(t *testing.T) {
	h := &Handlers{}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/validate", cutover.ValidateBundleRequest{}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))

	w := httptest.NewRecorder()
	h.ValidateMigrationBundle(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "at least one migration file is required")
}

func TestValidateMigrationBundleHandlerRejectsUnsupportedEInvoiceContactMode(t *testing.T) {
	h := &Handlers{}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/validate", cutover.ValidateBundleRequest{
		EInvoiceContactMode: "partner",
		Files: []cutover.BundleFile{{
			Kind:       cutover.KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		}},
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))

	w := httptest.NewRecorder()
	h.ValidateMigrationBundle(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "unsupported e_invoice_contact_mode")
}

func migrationRemediationCodes(actions []cutover.MigrationRemediationAction) []string {
	codes := make([]string, 0, len(actions))
	for _, action := range actions {
		codes = append(codes, action.Code)
	}
	return codes
}

func TestValidateMigrationBundleHandlerRejectsUnsupportedProviderPreset(t *testing.T) {
	h := &Handlers{}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/validate", cutover.ValidateBundleRequest{
		ProviderPreset: "legacy-system",
		Files: []cutover.BundleFile{{
			Kind:       cutover.KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer One\n",
		}},
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))

	w := httptest.NewRecorder()
	h.ValidateMigrationBundle(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "unsupported provider_preset")
}

func TestPlanMigrationExecutionHandler(t *testing.T) {
	h := &Handlers{}
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/execution-plan", cutover.PlanMigrationExecutionRequest{
		OpeningBalanceEntryDate: "2026-01-01",
		Files: []cutover.BundleFile{
			{
				Kind:       cutover.KindOpeningBalances,
				FileName:   "opening.csv",
				CSVContent: "account_code,debit,credit\n1000,100,0\n3000,0,100\n",
			},
			{
				Kind:       cutover.KindBankTransactions,
				FileName:   "bank.csv",
				CSVContent: "date,amount\n2026-01-02,42.50\n",
			},
			{
				Kind:       cutover.KindBankAccounts,
				FileName:   "bank-accounts.csv",
				CSVContent: "name,account_number\nMain,EE471000001020145685\n",
			},
			{
				Kind:       cutover.KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n3000,Equity,EQUITY\n",
			},
		},
	}, claims), map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.PlanMigrationExecution(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var plan cutover.MigrationExecutionPlan
	require.NoError(t, json.NewDecoder(w.Body).Decode(&plan))
	assert.True(t, plan.Summary.ValidationReady)
	assert.False(t, plan.Summary.Ready)
	assert.Equal(t, 4, plan.Summary.StepCount)
	assert.Equal(t, 1, plan.Summary.NeedsContextCount)
	require.Len(t, plan.Steps, 4)
	assert.Equal(t, cutover.KindAccounts, plan.Steps[0].Kind)
	assert.Equal(t, cutover.MigrationExecutionStepNeedsContext, plan.Steps[2].Status)
	assert.Equal(t, []string{"bank_transaction_account_id"}, plan.Steps[2].ContextFields)
	assert.Contains(t, plan.Steps[3].CLICommand, "oa journal import-opening-balances --entry-date 2026-01-01")
	assert.Contains(t, migrationRemediationCodes(plan.RemediationActions), "ready_to_import")
}

func TestPlanMigrationExecutionHandlerRejectsInvalidRequest(t *testing.T) {
	h := &Handlers{}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/execution-plan", cutover.PlanMigrationExecutionRequest{}, createTestClaims("user-1", "user@example.com", "tenant-1", "admin"))

	w := httptest.NewRecorder()
	h.PlanMigrationExecution(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "at least one migration file is required")
}
