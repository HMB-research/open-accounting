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
		PostJournalEntries:       true,
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
	assert.True(t, planReq.PostJournalEntries)
}

func TestStoredMigrationExecutionRequestMergesSavedBundleContext(t *testing.T) {
	saved := NewStoredMigrationExecutionRequest(&ExecuteMigrationRequest{
		Files: []BundleFile{{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
		}},
		EInvoiceContactMode:      EInvoiceContactModeBoth,
		EInvoiceInvoiceType:      "sales",
		ProviderPreset:           MigrationProviderPresetMerit,
		BankTransactionAccountID: "bank-1",
		BankTransactionFormat:    "lhv",
		OpeningBalanceEntryDate:  "2026-01-01",
		PostJournalEntries:       true,
		Confirm:                  true,
		ResumeFromRunID:          "run-1",
	})
	require.NotNil(t, saved)
	assert.False(t, saved.Confirm)
	assert.Empty(t, saved.ResumeFromRunID)

	req := &ExecuteMigrationRequest{Confirm: true, ResumeFromRunID: "run-1"}
	MergeSavedMigrationExecutionRequest(req, saved)

	require.Len(t, req.Files, 1)
	assert.Equal(t, KindAccounts, req.Files[0].Kind)
	assert.Equal(t, EInvoiceContactModeBoth, req.EInvoiceContactMode)
	assert.Equal(t, "sales", req.EInvoiceInvoiceType)
	assert.Equal(t, MigrationProviderPresetMerit, req.ProviderPreset)
	assert.Equal(t, "bank-1", req.BankTransactionAccountID)
	assert.Equal(t, "lhv", req.BankTransactionFormat)
	assert.Equal(t, "2026-01-01", req.OpeningBalanceEntryDate)
	assert.True(t, req.PostJournalEntries)
	assert.True(t, req.Confirm)
	assert.Equal(t, "run-1", req.ResumeFromRunID)

	req.Files[0].CSVContent = "changed"
	assert.Contains(t, saved.Files[0].CSVContent, "1000,Cash")
}

func TestStoredMigrationExecutionRequestDoesNotInheritPostingForReplacementFiles(t *testing.T) {
	saved := NewStoredMigrationExecutionRequest(&ExecuteMigrationRequest{
		Files: []BundleFile{{
			Kind:       KindJournalEntries,
			FileName:   "saved-journal.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\nSAVED,2026-01-01,1000,1,0\nSAVED,2026-01-01,3000,0,1\n",
		}},
		ProviderPreset:     MigrationProviderPresetSmartAccounts,
		PostJournalEntries: true,
	})
	require.NotNil(t, saved)

	req := &ExecuteMigrationRequest{
		Files: []BundleFile{{
			Kind:       KindJournalEntries,
			FileName:   "replacement-journal.csv",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\nLOCAL,2026-01-01,1000,1,0\nLOCAL,2026-01-01,3000,0,1\n",
		}},
	}
	MergeSavedMigrationExecutionRequest(req, saved)

	require.Len(t, req.Files, 1)
	assert.Equal(t, "replacement-journal.csv", req.Files[0].FileName)
	assert.False(t, req.PostJournalEntries)
	assert.Equal(t, MigrationProviderPresetSmartAccounts, req.ProviderPreset)
}

func TestStoredMigrationExecutionRequestNilAndExplicitMergeValues(t *testing.T) {
	assert.Nil(t, NewStoredMigrationExecutionRequest(nil))

	MergeSavedMigrationExecutionRequest(nil, &ExecuteMigrationRequest{})
	MergeSavedMigrationExecutionRequest(&ExecuteMigrationRequest{}, nil)

	req := &ExecuteMigrationRequest{
		Files: []BundleFile{{
			Kind:       KindContacts,
			FileName:   "contacts.csv",
			CSVContent: "contact_code,name\nCUST-1,Customer\n",
		}},
		EInvoiceContactMode:      EInvoiceContactModeCustomer,
		EInvoiceInvoiceType:      "sales",
		ProviderPreset:           MigrationProviderPresetGeneric,
		BankTransactionAccountID: "bank-existing",
		BankTransactionFormat:    "csv",
		OpeningBalanceEntryDate:  "2026-02-01",
		PostJournalEntries:       true,
	}
	saved := &ExecuteMigrationRequest{
		Files: []BundleFile{{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
		}},
		EInvoiceContactMode:      EInvoiceContactModeSupplier,
		EInvoiceInvoiceType:      "purchase",
		ProviderPreset:           MigrationProviderPresetDirecto,
		BankTransactionAccountID: "bank-saved",
		BankTransactionFormat:    "lhv",
		OpeningBalanceEntryDate:  "2026-01-01",
		PostJournalEntries:       true,
	}

	MergeSavedMigrationExecutionRequest(req, saved)

	require.Len(t, req.Files, 1)
	assert.Equal(t, KindContacts, req.Files[0].Kind)
	assert.Equal(t, EInvoiceContactModeCustomer, req.EInvoiceContactMode)
	assert.Equal(t, "sales", req.EInvoiceInvoiceType)
	assert.Equal(t, MigrationProviderPresetGeneric, req.ProviderPreset)
	assert.Equal(t, "bank-existing", req.BankTransactionAccountID)
	assert.Equal(t, "csv", req.BankTransactionFormat)
	assert.Equal(t, "2026-02-01", req.OpeningBalanceEntryDate)
	assert.True(t, req.PostJournalEntries)
	assert.Nil(t, cloneMigrationBundleFiles(nil))
	assert.Nil(t, cloneMigrationBundleFiles([]BundleFile{}))
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

func TestMigrationExecutionRunAndTimingEdgeCases(t *testing.T) {
	nilPlanRun := NewMigrationExecutionRun(nil, true)
	require.NotNil(t, nilPlanRun)
	assert.Equal(t, "blocked", nilPlanRun.Summary.Status)
	assert.Empty(t, nilPlanRun.Steps)

	RefreshMigrationExecutionRunProgress(nil)
	MarkMigrationExecutionStepRunning(nil, 0, time.Now())
	CompleteMigrationExecutionStep(nil, 0, MigrationExecutionResultSucceeded, "", "", nil, time.Now())

	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
		},
	}, true)

	MarkMigrationExecutionStepRunning(run, -1, time.Now())
	CompleteMigrationExecutionStep(run, 99, MigrationExecutionResultSucceeded, "", "", nil, time.Now())
	assert.Equal(t, MigrationExecutionResultPlanned, run.Steps[0].Status)

	MarkMigrationExecutionStepRunning(run, 0, time.Time{})
	require.NotNil(t, run.Steps[0].StartedAt)
	assert.False(t, run.Steps[0].StartedAt.IsZero())
	assert.Equal(t, time.UTC, run.Steps[0].StartedAt.Location())

	completedAt := run.Steps[0].StartedAt.Add(-time.Second)
	CompleteMigrationExecutionStep(run, 0, MigrationExecutionResultFailed, "failed", "boom", nil, completedAt)
	assert.Equal(t, int64(0), run.Steps[0].DurationMS)
	assert.Equal(t, "failed", run.Summary.Status)
	assert.Equal(t, 1, run.Summary.FailedStepCount)
	assert.Equal(t, 1, run.Summary.ActiveStepNumber)
	assert.Equal(t, MigrationExecutionResultFailed, run.Summary.ActiveStepStatus)

	start := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.FixedZone("EET", 2*60*60))
	normalized := normalizeMigrationExecutionTime(start)
	assert.Equal(t, time.UTC, normalized.Location())
	assert.Equal(t, start.UTC(), normalized)

	end := start.Add(time.Nanosecond)
	assert.Equal(t, int64(1), migrationExecutionStepDurationMS(&start, &end))
	assert.Equal(t, int64(0), migrationExecutionStepDurationMS(nil, &end))
	assert.Equal(t, int64(0), migrationExecutionStepDurationMS(&end, &start))
}
