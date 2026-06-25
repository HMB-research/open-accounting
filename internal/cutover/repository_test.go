package cutover

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type cutoverDryRunConnPool struct{}

func (cutoverDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run cutover tests should not prepare statements")
}

func (cutoverDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run cutover tests should not execute statements")
}

func (cutoverDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run cutover tests should not query rows")
}

func (cutoverDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (cutoverDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &cutoverDryRunTx{}, nil
}

type cutoverDryRunTx struct {
	cutoverDryRunConnPool
}

func (*cutoverDryRunTx) Commit() error {
	return nil
}

func (*cutoverDryRunTx) Rollback() error {
	return nil
}

type cutoverDryRunDBOption func(t *testing.T, db *gorm.DB)

var cutoverDryRunCallbackID uint64

func newCutoverDryRunDB(t *testing.T, opts ...cutoverDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: cutoverDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withCutoverDryRunRecords(records []models.MigrationExecutionRunRecord) cutoverDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(cutoverDryRunCallbackName("records"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *[]models.MigrationExecutionRunRecord:
				*dest = append((*dest)[:0], records...)
				tx.RowsAffected = int64(len(records))
			case *models.MigrationExecutionRunRecord:
				if len(records) == 0 {
					tx.AddError(gorm.ErrRecordNotFound)
					return
				}
				*dest = records[0]
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withCutoverDryRunQueryError(expectedErr error) cutoverDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(cutoverDryRunCallbackName("query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withCutoverDryRunCreateError(expectedErr error) cutoverDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(cutoverDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func cutoverDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&cutoverDryRunCallbackID, 1)
	return fmt.Sprintf("cutover_dryrun:%d:%s", id, suffix)
}

func TestMigrationExecutionRunRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	repo := NewMigrationExecutionRunRepository(nil)

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	table, err := repo.executionRunsTable(ctx, "tenant_schema")
	require.Error(t, err)
	assert.Nil(t, table)
	assert.Contains(t, err.Error(), "database connection not available")

	var nilRepo *GORMMigrationExecutionRunRepository
	table, err = nilRepo.executionRunsTable(ctx, "tenant_schema")
	require.Error(t, err)
	assert.Nil(t, table)
	assert.Contains(t, err.Error(), "database connection not available")

	_, err = repo.SaveExecutionRun(ctx, "tenant_schema", "tenant-1", "user-1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration execution run is required")

	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true},
	}, true)
	saved, err := repo.SaveExecutionRun(ctx, "tenant_schema", "tenant-1", "user-1", run)
	require.Error(t, err)
	assert.Nil(t, saved)
	assert.Contains(t, err.Error(), "database connection not available")

	runs, err := repo.ListExecutionRuns(ctx, "tenant_schema", "tenant-1", MigrationExecutionRunFilter{
		Status: "succeeded",
		Limit:  250,
	})
	require.Error(t, err)
	assert.Nil(t, runs)
	assert.Contains(t, err.Error(), "database connection not available")

	loaded, err := repo.GetExecutionRun(ctx, "tenant_schema", "tenant-1", "run-1")
	require.Error(t, err)
	assert.Nil(t, loaded)
	assert.Contains(t, err.Error(), "database connection not available")
}

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

func TestMigrationExecutionRunRepositoryDryRunPersistence(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true, StepCount: 1, ReadyStepCount: 1},
		Steps: []MigrationExecutionStep{
			{StepNumber: 1, Kind: KindAccounts, FileName: "accounts.csv", Status: MigrationExecutionStepReady},
		},
	}, true)
	run.ID = "run-1"
	MarkMigrationExecutionStepRunning(run, 0, now)
	CompleteMigrationExecutionStep(run, 0, MigrationExecutionResultSucceeded, "done", "", nil, now.Add(time.Second))
	payload, err := marshalMigrationExecutionRunPayload(run)
	require.NoError(t, err)
	record := models.MigrationExecutionRunRecord{
		ID:                 "run-1",
		TenantID:           "tenant-1",
		CreatedBy:          "user-1",
		Status:             "succeeded",
		Confirmed:          true,
		StepCount:          1,
		SucceededStepCount: 1,
		FileNames:          []string{"accounts.csv"},
		RunPayload:         payload,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t, withCutoverDryRunRecords([]models.MigrationExecutionRunRecord{record})))
	repo.now = func() time.Time { return now }

	saved, err := repo.SaveExecutionRun(ctx, "tenant_schema", "tenant-1", "user-1", run)
	require.NoError(t, err)
	assert.Equal(t, "run-1", saved.ID)
	assert.Equal(t, "tenant-1", saved.TenantID)
	assert.Equal(t, "user-1", saved.CreatedBy)

	runs, err := repo.ListExecutionRuns(ctx, "tenant_schema", " tenant-1 ", MigrationExecutionRunFilter{Status: " succeeded ", Limit: 250})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "run-1", runs[0].ID)
	assert.Equal(t, 100, runs[0].Summary.ProgressPercent)

	loaded, err := repo.GetExecutionRun(ctx, "tenant_schema", " tenant-1 ", " run-1 ")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "run-1", loaded.ID)
	assert.Equal(t, "succeeded", loaded.Summary.Status)
}

func TestMigrationExecutionRunRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	run := NewMigrationExecutionRun(&MigrationExecutionPlan{
		Summary: MigrationExecutionPlanSummary{ValidationReady: true, Ready: true},
	}, true)

	t.Run("save wraps create error", func(t *testing.T) {
		repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t, withCutoverDryRunCreateError(assert.AnError)))
		saved, err := repo.SaveExecutionRun(ctx, "tenant_schema", "tenant-1", "user-1", run)
		require.Error(t, err)
		assert.Nil(t, saved)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "save migration execution run")
	})

	t.Run("list wraps query error", func(t *testing.T) {
		repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t, withCutoverDryRunQueryError(assert.AnError)))
		runs, err := repo.ListExecutionRuns(ctx, "tenant_schema", "tenant-1", MigrationExecutionRunFilter{})
		require.Error(t, err)
		assert.Nil(t, runs)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "list migration execution runs")
	})

	t.Run("get maps record not found", func(t *testing.T) {
		repo := NewGORMMigrationExecutionRunRepository(newCutoverDryRunDB(t, withCutoverDryRunRecords(nil)))
		loaded, err := repo.GetExecutionRun(ctx, "tenant_schema", "tenant-1", "run-1")
		require.Error(t, err)
		assert.Nil(t, loaded)
		assert.ErrorIs(t, err, ErrMigrationExecutionRunNotFound)
	})
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

func TestMigrationExecutionRunRepositoryMappingEdgeCases(t *testing.T) {
	run, err := recordToMigrationExecutionRun(nil)
	require.Error(t, err)
	assert.Nil(t, run)
	assert.ErrorIs(t, err, ErrMigrationExecutionRunNotFound)

	run, err = recordToMigrationExecutionRun(&models.MigrationExecutionRunRecord{RunPayload: json.RawMessage(`{`)})
	require.Error(t, err)
	assert.Nil(t, run)
	assert.Contains(t, err.Error(), "parse migration execution run payload")

	payload, err := marshalMigrationExecutionRunPayload(nil)
	require.Error(t, err)
	assert.Nil(t, payload)
	assert.Contains(t, err.Error(), "migration execution run is required")

	repo := &GORMMigrationExecutionRunRepository{}
	assert.False(t, repo.currentTime().IsZero())
	assert.Nil(t, migrationExecutionRunFileNames(nil))
	assert.Equal(t, []string{"accounts.csv", "contacts.csv"}, migrationExecutionRunFileNames(&MigrationExecutionRun{
		Steps: []MigrationExecutionStepRun{
			{FileName: "accounts.csv"},
			{FileName: " "},
			{FileName: "contacts.csv"},
			{FileName: "accounts.csv"},
		},
	}))
}
