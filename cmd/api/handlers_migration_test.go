package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMigrationStepExecutor struct {
	calls []cutover.MigrationExecutionStep
	err   error
}

func (f *fakeMigrationStepExecutor) ExecuteMigrationStep(_ context.Context, tenantID, schemaName, userID string, step cutover.MigrationExecutionStep, file cutover.BundleFile, _ *cutover.ExecuteMigrationRequest) (any, error) {
	f.calls = append(f.calls, step)
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{
		"kind":        step.Kind,
		"file_name":   file.FileName,
		"tenant_id":   tenantID,
		"schema_name": schemaName,
		"user_id":     userID,
	}, nil
}

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

func TestExecuteMigrationHandlerPlansWithoutConfirmation(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	h := &Handlers{migrationExecutor: executor}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{{
			Kind:       cutover.KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
		}},
		Confirm: false,
	})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.Equal(t, "needs_confirmation", run.Summary.Status)
	assert.False(t, run.Summary.Confirmed)
	assert.True(t, run.Summary.PlanReady)
	assert.Equal(t, 1, run.Summary.PlannedStepCount)
	require.Len(t, run.Steps, 1)
	assert.Equal(t, cutover.MigrationExecutionResultPlanned, run.Steps[0].Status)
	assert.Contains(t, run.Steps[0].Message, "confirm=true")
	assert.Empty(t, executor.calls)
}

func TestExecuteMigrationHandlerRunsConfirmedReadySteps(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	h := &Handlers{migrationExecutor: executor}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{
			{
				Kind:       cutover.KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "contact_code,name\nCUST-1,Customer One\n",
			},
			{
				Kind:       cutover.KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
			},
		},
		Confirm: true,
	})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.Equal(t, "succeeded", run.Summary.Status)
	assert.True(t, run.Summary.Confirmed)
	assert.Equal(t, 2, run.Summary.SucceededStepCount)
	assert.Equal(t, 0, run.Summary.FailedStepCount)
	require.Len(t, executor.calls, 2)
	assert.Equal(t, cutover.KindAccounts, executor.calls[0].Kind)
	assert.Equal(t, cutover.KindContacts, executor.calls[1].Kind)
	require.Len(t, run.Steps, 2)
	assert.Equal(t, cutover.MigrationExecutionResultSucceeded, run.Steps[0].Status)
	assert.NotNil(t, run.Steps[0].Response)
}

func TestExecuteMigrationHandlerRejectsNotReadyPlan(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	h := &Handlers{migrationExecutor: executor}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{{
			Kind:       cutover.KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount\n2026-01-02,42.50\n",
		}},
		Confirm: true,
	})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.Equal(t, "blocked", run.Summary.Status)
	assert.True(t, run.Summary.Confirmed)
	assert.Equal(t, 1, run.Summary.NeedsContextCount)
	assert.Empty(t, executor.calls)
}

func TestExecuteMigrationHandlerReportsStepFailure(t *testing.T) {
	executor := &fakeMigrationStepExecutor{err: errors.New("import failed")}
	h := &Handlers{migrationExecutor: executor}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{{
			Kind:       cutover.KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
		}},
		Confirm: true,
	})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.Equal(t, "failed", run.Summary.Status)
	assert.Equal(t, 1, run.Summary.FailedStepCount)
	require.Len(t, run.Steps, 1)
	assert.Equal(t, cutover.MigrationExecutionResultFailed, run.Steps[0].Status)
	assert.Contains(t, run.Steps[0].Error, "import failed")
	require.Len(t, executor.calls, 1)
}

func TestExecuteMigrationHandlerRejectsInvalidRequest(t *testing.T) {
	h := &Handlers{migrationExecutor: &fakeMigrationStepExecutor{}}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "at least one migration file is required")
}

func executeMigrationRequest(body cutover.ExecuteMigrationRequest) *http.Request {
	return withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/execute", body, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
}
