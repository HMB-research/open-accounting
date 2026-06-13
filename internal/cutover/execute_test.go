package cutover

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	previous := &MigrationExecutionRun{
		Steps: []MigrationExecutionStepRun{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionResultSucceeded, Response: map[string]any{"created": 5}},
			{StepNumber: 2, Kind: KindContacts, FileName: "contacts.csv", Status: MigrationExecutionResultFailed},
		},
	}

	run := NewResumableMigrationExecutionRun(plan, true, previous)

	require.NotNil(t, run)
	assert.Equal(t, "running", run.Summary.Status)
	assert.True(t, run.Summary.Resumed)
	assert.Equal(t, 1, run.Summary.ResumedStepCount)
	assert.Equal(t, 1, run.Summary.SucceededStepCount)
	require.Len(t, run.Steps, 2)
	assert.Equal(t, MigrationExecutionResultSucceeded, run.Steps[0].Status)
	assert.Contains(t, run.Steps[0].Message, "previous run")
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
