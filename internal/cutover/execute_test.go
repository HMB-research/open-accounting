package cutover

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteMigrationRequestPlanRequestPreservesExecutionContext(t *testing.T) {
	files := []BundleFile{{
		Kind:       KindBankTransactions,
		FileName:   "bank.csv",
		CSVContent: "date,amount,description\n2026-01-02,42.50,Customer receipt\n",
	}}
	req := ExecuteMigrationRequest{
		Files:                    files,
		EInvoiceContactMode:      EInvoiceContactModeBoth,
		EInvoiceInvoiceType:      "sales",
		ProviderPreset:           MigrationProviderPresetDirecto,
		BankTransactionAccountID: "bank-1",
		BankTransactionFormat:    "lhv",
		OpeningBalanceEntryDate:  "2026-01-01",
	}

	planReq := req.PlanRequest()

	require.NotNil(t, planReq)
	assert.Equal(t, files, planReq.Files)
	assert.Equal(t, EInvoiceContactModeBoth, planReq.EInvoiceContactMode)
	assert.Equal(t, "sales", planReq.EInvoiceInvoiceType)
	assert.Equal(t, MigrationProviderPresetDirecto, planReq.ProviderPreset)
	assert.Equal(t, "bank-1", planReq.BankTransactionAccountID)
	assert.Equal(t, "lhv", planReq.BankTransactionFormat)
	assert.Equal(t, "2026-01-01", planReq.OpeningBalanceEntryDate)
}

func TestNewResumableMigrationExecutionRunSkipsPreviouslySucceededSteps(t *testing.T) {
	plan := &MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{
			ValidationReady: true,
			Ready:           true,
			StepCount:       2,
			ReadyStepCount:  2,
		},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
			{StepNumber: 2, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionStepReady},
		},
	}
	startedAt := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(1500 * time.Millisecond)
	previous := &MigrationExecutionRun{
		Steps: []MigrationExecutionStepRun{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionResultSucceeded, Response: map[string]any{"created": 5}, StartedAt: &startedAt, CompletedAt: &completedAt, DurationMS: 1500},
			{StepNumber: 2, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionResultFailed},
		},
	}

	run := NewResumableMigrationExecutionRun(plan, true, previous)

	require.NotNil(t, run)
	assert.Equal(t, "running", run.Summary.Status)
	assert.True(t, run.Summary.Resumed)
	assert.Equal(t, 1, run.Summary.ResumedStepCount)
	assert.Equal(t, 1, run.Summary.SucceededStepCount)
	assert.Equal(t, 50, run.Summary.ProgressPercent)
	assert.Equal(t, 1, run.Summary.CompletedStepCount)
	assert.Equal(t, 1, run.Summary.RemainingStepCount)
	assert.Equal(t, int64(1500), run.Summary.DurationMS)
	assert.Equal(t, 2, run.Summary.ActiveStepNumber)
	assert.Equal(t, MigrationExecutionResultPlanned, run.Summary.ActiveStepStatus)
	require.Len(t, run.Steps, 2)
	assert.Equal(t, MigrationExecutionResultSucceeded, run.Steps[0].Status)
	assert.Contains(t, run.Steps[0].Message, "previous run")
	assert.Equal(t, startedAt, *run.Steps[0].StartedAt)
	assert.Equal(t, completedAt, *run.Steps[0].CompletedAt)
	assert.Equal(t, int64(1500), run.Steps[0].DurationMS)
	assert.Equal(t, MigrationExecutionResultPlanned, run.Steps[1].Status)
}

func TestApplyMigrationExecutionResumeIgnoresMissingOrNonSucceededSteps(t *testing.T) {
	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
		},
	}, true)

	ApplyMigrationExecutionResume(nil, &MigrationExecutionRun{})
	ApplyMigrationExecutionResume(run, nil)
	ApplyMigrationExecutionResume(run, &MigrationExecutionRun{
		Steps: []MigrationExecutionStepRun{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionResultFailed},
			{StepNumber: 2, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionResultSucceeded},
		},
	})

	assert.False(t, run.Summary.Resumed)
	assert.Equal(t, 0, run.Summary.ResumedStepCount)
	assert.Equal(t, MigrationExecutionResultPlanned, run.Steps[0].Status)
}

func TestRefreshMigrationExecutionRunProgressTracksRunningActiveStep(t *testing.T) {
	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 3, ReadyStepCount: 3},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
			{StepNumber: 2, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionStepReady},
			{StepNumber: 3, Kind: KindInvoices, FileName: "invoices.csv", Status: MigrationExecutionStepReady},
		},
	}, true)

	run.Steps[0].Status = MigrationExecutionResultSucceeded
	run.Steps[1].Status = MigrationExecutionResultRunning
	RefreshMigrationExecutionRunProgress(run)

	assert.Equal(t, "running", run.Summary.Status)
	assert.Equal(t, 1, run.Summary.SucceededStepCount)
	assert.Equal(t, 1, run.Summary.RunningStepCount)
	assert.Equal(t, 1, run.Summary.PlannedStepCount)
	assert.Equal(t, 1, run.Summary.CompletedStepCount)
	assert.Equal(t, 2, run.Summary.RemainingStepCount)
	assert.Equal(t, 33, run.Summary.ProgressPercent)
	assert.Equal(t, 2, run.Summary.ActiveStepNumber)
	assert.Equal(t, KindContacts, run.Summary.ActiveStepKind)
	assert.Equal(t, "contacts.csv", run.Summary.ActiveStepFileName)
	assert.Equal(t, MigrationExecutionResultRunning, run.Summary.ActiveStepStatus)
}

func TestMigrationExecutionStepTimingHelpersTrackDuration(t *testing.T) {
	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
		},
	}, true)
	startedAt := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(2500 * time.Millisecond)

	MarkMigrationExecutionStepRunning(run, 0, startedAt)
	require.NotNil(t, run.Steps[0].StartedAt)
	assert.Equal(t, startedAt, *run.Steps[0].StartedAt)
	assert.Nil(t, run.Steps[0].CompletedAt)
	assert.Equal(t, MigrationExecutionResultRunning, run.Summary.ActiveStepStatus)
	assert.Equal(t, startedAt, *run.Summary.ActiveStepStartedAt)

	CompleteMigrationExecutionStep(run, 0, MigrationExecutionResultSucceeded, "Import completed.", "", map[string]any{"created": 1}, completedAt)
	require.NotNil(t, run.Steps[0].CompletedAt)
	assert.Equal(t, completedAt, *run.Steps[0].CompletedAt)
	assert.Equal(t, int64(2500), run.Steps[0].DurationMS)
	assert.Equal(t, int64(2500), run.Summary.DurationMS)
	assert.Equal(t, "succeeded", run.Summary.Status)
}
