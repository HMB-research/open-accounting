package cutover

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationExecutionRunRepositoryMappingRoundTripsPayload(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	repo := NewGORMMigrationExecutionRunRepository(nil)
	repo.now = func() time.Time { return now }

	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 2, ReadyStepCount: 2},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
			{StepNumber: 2, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionStepReady},
		},
	}, true)
	run.ExecutionRequest = NewStoredMigrationExecutionRequest(&ExecuteMigrationRequest{
		Files: []BundleFile{{
			Kind:       KindAccounts,
			FileName:   "accounts.csv",
			CSVContent: "code,name,account_type\n1000,Cash,ASSET\n",
		}},
		ProviderPreset:          MigrationProviderPresetDirecto,
		BankTransactionFormat:   "lhv",
		OpeningBalanceEntryDate: "2026-01-01",
		Confirm:                 true,
		ResumeFromRunID:         "previous-run",
	})
	MarkMigrationExecutionStepRunning(run, 0, now)
	CompleteMigrationExecutionStep(run, 0, MigrationExecutionResultSucceeded, "Import completed.", "", json.RawMessage(`{"created":1}`), now.Add(1200*time.Millisecond))
	MarkMigrationExecutionStepRunning(run, 1, now.Add(2*time.Second))
	CompleteMigrationExecutionStep(run, 1, MigrationExecutionResultSucceeded, "Import completed.", "", nil, now.Add(2500*time.Millisecond))

	record, err := repo.runToRecord("tenant-1", "user-1", run)
	require.NoError(t, err)
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, "tenant-1", record.TenantID)
	assert.Equal(t, "user-1", record.CreatedBy)
	assert.Equal(t, "succeeded", record.Status)
	assert.Equal(t, 2, record.SucceededStepCount)
	assert.Equal(t, []string{"accounts.csv", "contacts.csv"}, []string(record.FileNames))
	assert.Equal(t, now, record.CreatedAt)
	assert.Equal(t, now, record.UpdatedAt)

	roundTripped, err := recordToMigrationExecutionRun(record)
	require.NoError(t, err)
	assert.Equal(t, record.ID, roundTripped.ID)
	assert.Equal(t, "tenant-1", roundTripped.TenantID)
	assert.Equal(t, "user-1", roundTripped.CreatedBy)
	assert.Equal(t, "succeeded", roundTripped.Summary.Status)
	assert.Equal(t, 2, roundTripped.Summary.SucceededStepCount)
	assert.Equal(t, 2, roundTripped.Summary.CompletedStepCount)
	assert.Equal(t, 0, roundTripped.Summary.RemainingStepCount)
	assert.Equal(t, 100, roundTripped.Summary.ProgressPercent)
	assert.Equal(t, int64(1700), roundTripped.Summary.DurationMS)
	require.Len(t, roundTripped.Steps, 2)
	assert.Equal(t, MigrationExecutionResultSucceeded, roundTripped.Steps[0].Status)
	assert.Equal(t, int64(1200), roundTripped.Steps[0].DurationMS)
	assert.Equal(t, now, *roundTripped.CreatedAt)
	assert.Equal(t, now, *roundTripped.UpdatedAt)
	require.NotNil(t, roundTripped.ExecutionRequest)
	require.Len(t, roundTripped.ExecutionRequest.Files, 1)
	assert.Equal(t, "accounts.csv", roundTripped.ExecutionRequest.Files[0].FileName)
	assert.Contains(t, roundTripped.ExecutionRequest.Files[0].CSVContent, "1000,Cash")
	assert.Equal(t, MigrationProviderPresetDirecto, roundTripped.ExecutionRequest.ProviderPreset)
	assert.Equal(t, "lhv", roundTripped.ExecutionRequest.BankTransactionFormat)
	assert.Equal(t, "2026-01-01", roundTripped.ExecutionRequest.OpeningBalanceEntryDate)
	assert.False(t, roundTripped.ExecutionRequest.Confirm)
	assert.Empty(t, roundTripped.ExecutionRequest.ResumeFromRunID)

	publicPayload, err := json.Marshal(roundTripped)
	require.NoError(t, err)
	assert.NotContains(t, string(publicPayload), "csv_content")
	assert.NotContains(t, string(publicPayload), "execution_request")
}

func TestRecordToMigrationExecutionRunFallsBackToIndexedSummary(t *testing.T) {
	createdAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	record := &models.MigrationExecutionRunRecord{
		ID:                 "run-1",
		TenantID:           "tenant-1",
		CreatedBy:          "user-1",
		Status:             "failed",
		Confirmed:          true,
		Resumed:            true,
		StepCount:          3,
		SucceededStepCount: 1,
		FailedStepCount:    1,
		SkippedStepCount:   1,
		PlannedStepCount:   0,
		ResumedStepCount:   1,
		RunPayload:         json.RawMessage(`{}`),
		CreatedAt:          createdAt,
		UpdatedAt:          createdAt,
	}

	run, err := recordToMigrationExecutionRun(record)
	require.NoError(t, err)
	assert.Equal(t, "run-1", run.ID)
	assert.Equal(t, "failed", run.Summary.Status)
	assert.True(t, run.Summary.Confirmed)
	assert.True(t, run.Summary.Resumed)
	assert.Equal(t, 3, run.Summary.StepCount)
	assert.Equal(t, 1, run.Summary.ResumedStepCount)
	assert.Equal(t, 2, run.Summary.CompletedStepCount)
	assert.Equal(t, 1, run.Summary.RemainingStepCount)
	assert.Equal(t, 66, run.Summary.ProgressPercent)
}
