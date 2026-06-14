package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

type fakeMigrationRunStore struct {
	saved      []cutover.MigrationExecutionRun
	listRuns   []cutover.MigrationExecutionRun
	getRun     *cutover.MigrationExecutionRun
	getRuns    []*cutover.MigrationExecutionRun
	getCalls   int
	saveErr    error
	listErr    error
	getErr     error
	lastFilter cutover.MigrationExecutionRunFilter
}

func (f *fakeMigrationRunStore) SaveExecutionRun(_ context.Context, _, tenantID, createdBy string, run *cutover.MigrationExecutionRun) (*cutover.MigrationExecutionRun, error) {
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	if run.ID == "" {
		run.ID = "run-1"
	}
	run.TenantID = tenantID
	run.CreatedBy = createdBy
	f.saved = append(f.saved, *run)
	return run, nil
}

func (f *fakeMigrationRunStore) ListExecutionRuns(_ context.Context, _, _ string, filter cutover.MigrationExecutionRunFilter) ([]cutover.MigrationExecutionRun, error) {
	f.lastFilter = filter
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listRuns, nil
}

func (f *fakeMigrationRunStore) GetExecutionRun(_ context.Context, _, _, _ string) (*cutover.MigrationExecutionRun, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if len(f.getRuns) > 0 {
		index := f.getCalls
		f.getCalls++
		if index >= len(f.getRuns) {
			index = len(f.getRuns) - 1
		}
		return f.getRuns[index], nil
	}
	return f.getRun, nil
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

func TestListMigrationProviderPresetsHandler(t *testing.T) {
	h := &Handlers{}
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/provider-presets", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.ListMigrationProviderPresets(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var presets []cutover.MigrationProviderPresetInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&presets))
	require.Len(t, presets, 4)
	assert.Equal(t, cutover.MigrationProviderPresetGeneric, presets[0].Preset)
	assert.Equal(t, cutover.MigrationProviderPresetMerit, presets[1].Preset)
	assert.Equal(t, cutover.MigrationProviderPresetSmartAccounts, presets[2].Preset)
	assert.Equal(t, cutover.MigrationProviderPresetDirecto, presets[3].Preset)
	assert.Greater(t, presets[1].PresetAliasCount, 0)
	assert.NotEmpty(t, presets[1].FileKinds)
	assert.Greater(t, presets[3].PresetAliasCount, 0)
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
				CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n",
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
	assert.Equal(t, 2, run.Summary.CompletedStepCount)
	assert.Equal(t, 0, run.Summary.RemainingStepCount)
	assert.Equal(t, 100, run.Summary.ProgressPercent)
	require.Len(t, executor.calls, 2)
	assert.Equal(t, cutover.KindAccounts, executor.calls[0].Kind)
	assert.Equal(t, cutover.KindContacts, executor.calls[1].Kind)
	require.Len(t, run.Steps, 2)
	assert.Equal(t, cutover.MigrationExecutionResultSucceeded, run.Steps[0].Status)
	assert.NotNil(t, run.Steps[0].StartedAt)
	assert.NotNil(t, run.Steps[0].CompletedAt)
	assert.NotNil(t, run.Steps[0].Response)
}

func TestExecuteMigrationHandlerResumesPreviouslySucceededSteps(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	h := &Handlers{migrationExecutor: executor}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{
			{
				Kind:       cutover.KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
			},
			{
				Kind:       cutover.KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "contact_code,name\nCUST-1,Customer One\n",
			},
		},
		Confirm: true,
		ResumeFromRun: &cutover.MigrationExecutionRun{
			Steps: []cutover.MigrationExecutionStepRun{
				{StepNumber: 1, Kind: cutover.KindAccounts, FileName: "accounts.csv", Status: cutover.MigrationExecutionResultSucceeded, Response: map[string]any{"created": 1}},
				{StepNumber: 2, Kind: cutover.KindContacts, FileName: "contacts.csv", Status: cutover.MigrationExecutionResultFailed},
			},
		},
	})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.Equal(t, "succeeded", run.Summary.Status)
	assert.True(t, run.Summary.Resumed)
	assert.Equal(t, 1, run.Summary.ResumedStepCount)
	assert.Equal(t, 2, run.Summary.SucceededStepCount)
	assert.Equal(t, 100, run.Summary.ProgressPercent)
	require.Len(t, executor.calls, 1)
	assert.Equal(t, cutover.KindContacts, executor.calls[0].Kind)
	require.Len(t, run.Steps, 2)
	assert.Equal(t, cutover.MigrationExecutionResultSucceeded, run.Steps[0].Status)
	assert.Contains(t, run.Steps[0].Message, "previous run")
	assert.Equal(t, cutover.MigrationExecutionResultSucceeded, run.Steps[1].Status)
}

func TestExecuteMigrationHandlerPersistsRunSnapshots(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	store := &fakeMigrationRunStore{}
	h := &Handlers{migrationExecutor: executor, migrationRunStore: store}
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

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.Equal(t, "run-1", run.ID)
	assert.Equal(t, "tenant-1", run.TenantID)
	assert.Equal(t, "user-1", run.CreatedBy)
	require.GreaterOrEqual(t, len(store.saved), 2)
	assert.Equal(t, "running", store.saved[0].Summary.Status)
	assert.Equal(t, 1, store.saved[0].Summary.PlannedStepCount)
	assert.Equal(t, 0, store.saved[0].Summary.ProgressPercent)
	assert.Equal(t, cutover.MigrationExecutionResultPlanned, store.saved[0].Summary.ActiveStepStatus)
	assert.Equal(t, "running", store.saved[1].Summary.Status)
	assert.Equal(t, 1, store.saved[1].Summary.RunningStepCount)
	assert.Equal(t, cutover.MigrationExecutionResultRunning, store.saved[1].Summary.ActiveStepStatus)
	assert.NotNil(t, store.saved[1].Summary.ActiveStepStartedAt)
	assert.Equal(t, "succeeded", store.saved[len(store.saved)-1].Summary.Status)
	assert.Equal(t, 1, store.saved[len(store.saved)-1].Summary.SucceededStepCount)
	assert.Equal(t, 100, store.saved[len(store.saved)-1].Summary.ProgressPercent)
	assert.NotNil(t, run.Steps[0].StartedAt)
	assert.NotNil(t, run.Steps[0].CompletedAt)
}

func TestExecuteMigrationHandlerResumesSavedRunID(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	store := &fakeMigrationRunStore{
		getRun: &cutover.MigrationExecutionRun{
			ID: "previous-run",
			Steps: []cutover.MigrationExecutionStepRun{
				{StepNumber: 1, Kind: cutover.KindAccounts, FileName: "accounts.csv", Status: cutover.MigrationExecutionResultSucceeded, Response: map[string]any{"created": 1}},
				{StepNumber: 2, Kind: cutover.KindContacts, FileName: "contacts.csv", Status: cutover.MigrationExecutionResultFailed},
			},
		},
	}
	h := &Handlers{migrationExecutor: executor, migrationRunStore: store}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{
			{
				Kind:       cutover.KindAccounts,
				FileName:   "accounts.csv",
				CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
			},
			{
				Kind:       cutover.KindContacts,
				FileName:   "contacts.csv",
				CSVContent: "contact_code,name\nCUST-1,Customer One\n",
			},
		},
		Confirm:         true,
		ResumeFromRunID: "previous-run",
	})

	w := httptest.NewRecorder()
	h.ExecuteMigration(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(w.Body).Decode(&run))
	assert.True(t, run.Summary.Resumed)
	assert.Equal(t, 1, run.Summary.ResumedStepCount)
	require.Len(t, executor.calls, 1)
	assert.Equal(t, cutover.KindContacts, executor.calls[0].Kind)
}

func TestExecuteMigrationHandlerRejectsNotReadyPlan(t *testing.T) {
	executor := &fakeMigrationStepExecutor{}
	h := &Handlers{migrationExecutor: executor}
	req := executeMigrationRequest(cutover.ExecuteMigrationRequest{
		Files: []cutover.BundleFile{{
			Kind:       cutover.KindBankTransactions,
			FileName:   "bank.csv",
			CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n",
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
	assert.Equal(t, 0, run.Summary.ProgressPercent)
	assert.Equal(t, 1, run.Summary.RemainingStepCount)
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
	assert.Equal(t, 100, run.Summary.ProgressPercent)
	assert.Equal(t, cutover.MigrationExecutionResultFailed, run.Summary.ActiveStepStatus)
	require.Len(t, run.Steps, 1)
	assert.Equal(t, cutover.MigrationExecutionResultFailed, run.Steps[0].Status)
	assert.NotNil(t, run.Steps[0].StartedAt)
	assert.NotNil(t, run.Steps[0].CompletedAt)
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

func TestMigrationExecutionRunHandlersListAndGetSavedRuns(t *testing.T) {
	store := &fakeMigrationRunStore{
		listRuns: []cutover.MigrationExecutionRun{{
			ID:      "run-1",
			Summary: cutover.MigrationExecutionRunSummary{Status: "succeeded", StepCount: 1, SucceededStepCount: 1},
		}},
		getRun: &cutover.MigrationExecutionRun{
			ID:      "run-1",
			Summary: cutover.MigrationExecutionRunSummary{Status: "succeeded", StepCount: 1, SucceededStepCount: 1},
		},
	}
	h := &Handlers{migrationRunStore: store}

	listReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs?status=succeeded&limit=10", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
	listW := httptest.NewRecorder()
	h.ListMigrationExecutionRuns(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code, listW.Body.String())
	var runs []cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(listW.Body).Decode(&runs))
	require.Len(t, runs, 1)
	assert.Equal(t, "run-1", runs[0].ID)
	assert.Equal(t, "succeeded", store.lastFilter.Status)
	assert.Equal(t, 10, store.lastFilter.Limit)

	getReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/run-1", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1", "runID": "run-1"})
	getW := httptest.NewRecorder()
	h.GetMigrationExecutionRun(getW, getReq)

	require.Equal(t, http.StatusOK, getW.Code, getW.Body.String())
	var run cutover.MigrationExecutionRun
	require.NoError(t, json.NewDecoder(getW.Body).Decode(&run))
	assert.Equal(t, "run-1", run.ID)
}

func TestStreamMigrationExecutionRunHandlerEmitsRunSnapshots(t *testing.T) {
	runningRun := &cutover.MigrationExecutionRun{
		ID: "run-1",
		Summary: cutover.MigrationExecutionRunSummary{
			Status:             "running",
			Confirmed:          true,
			StepCount:          2,
			SucceededStepCount: 1,
			RunningStepCount:   1,
			ProgressPercent:    50,
			ActiveStepNumber:   2,
			ActiveStepKind:     cutover.KindContacts,
			ActiveStepFileName: "contacts.csv",
			ActiveStepStatus:   cutover.MigrationExecutionResultRunning,
		},
	}
	succeededRun := &cutover.MigrationExecutionRun{
		ID: "run-1",
		Summary: cutover.MigrationExecutionRunSummary{
			Status:             "succeeded",
			Confirmed:          true,
			StepCount:          2,
			SucceededStepCount: 2,
			ProgressPercent:    100,
		},
	}
	store := &fakeMigrationRunStore{getRuns: []*cutover.MigrationExecutionRun{runningRun, succeededRun}}
	h := &Handlers{migrationRunStore: store}

	req := withURLParams(
		makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/run-1/events?interval_ms=1&max_events=3", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")),
		map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
	)
	w := httptest.NewRecorder()
	h.StreamMigrationExecutionRun(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream"))
	body := w.Body.String()
	assert.Contains(t, body, "event: snapshot")
	assert.Contains(t, body, `"sequence":1`)
	assert.Contains(t, body, `"status":"running"`)
	assert.Contains(t, body, `"active_step_kind":"contacts"`)
	assert.Contains(t, body, "event: complete")
	assert.Contains(t, body, `"sequence":2`)
	assert.Contains(t, body, `"status":"succeeded"`)
	assert.Equal(t, 2, store.getCalls)
}

func TestGetMigrationExecutionRunHandlerReturnsNotFound(t *testing.T) {
	h := &Handlers{migrationRunStore: &fakeMigrationRunStore{getErr: cutover.ErrMigrationExecutionRunNotFound}}
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/migration/execution-runs/missing", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1", "runID": "missing"})

	w := httptest.NewRecorder()
	h.GetMigrationExecutionRun(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "migration execution run not found")
}

func executeMigrationRequest(body cutover.ExecuteMigrationRequest) *http.Request {
	return withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/migration/execute", body, createTestClaims("user-1", "user@example.com", "tenant-1", "admin")), map[string]string{"tenantID": "tenant-1"})
}
