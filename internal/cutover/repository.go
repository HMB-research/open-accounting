package cutover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrMigrationExecutionRunNotFound is returned when a saved run does not belong to the tenant.
var ErrMigrationExecutionRunNotFound = errors.New("migration execution run not found")

// MigrationExecutionRunFilter controls saved execution-run listing.
type MigrationExecutionRunFilter struct {
	Status string
	Limit  int
}

// MigrationExecutionRunStore persists migration execution runs for dashboard and operator resume workflows.
type MigrationExecutionRunStore interface {
	SaveExecutionRun(ctx context.Context, schemaName, tenantID, createdBy string, run *MigrationExecutionRun) (*MigrationExecutionRun, error)
	ListExecutionRuns(ctx context.Context, schemaName, tenantID string, filter MigrationExecutionRunFilter) ([]MigrationExecutionRun, error)
	GetExecutionRun(ctx context.Context, schemaName, tenantID, runID string) (*MigrationExecutionRun, error)
}

// GORMMigrationExecutionRunRepository implements MigrationExecutionRunStore using tenant-scoped tables.
type GORMMigrationExecutionRunRepository struct {
	db  *gorm.DB
	now func() time.Time
}

// NewMigrationExecutionRunRepository creates a migration execution run repository from the shared pgx pool.
func NewMigrationExecutionRunRepository(pool *pgxpool.Pool) *GORMMigrationExecutionRunRepository {
	if pool == nil {
		return &GORMMigrationExecutionRunRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create migration execution run repository: %w", err))
	}
	return NewGORMMigrationExecutionRunRepository(gormDB)
}

// NewGORMMigrationExecutionRunRepository creates a repository from a GORM connection.
func NewGORMMigrationExecutionRunRepository(db *gorm.DB) *GORMMigrationExecutionRunRepository {
	return &GORMMigrationExecutionRunRepository{db: db, now: time.Now}
}

func (r *GORMMigrationExecutionRunRepository) executionRunsTable(ctx context.Context, schemaName string) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}
	return database.TenantTable(r.db.WithContext(ctx), schemaName, "migration_execution_runs")
}

// SaveExecutionRun creates or updates a run snapshot.
func (r *GORMMigrationExecutionRunRepository) SaveExecutionRun(ctx context.Context, schemaName, tenantID, createdBy string, run *MigrationExecutionRun) (*MigrationExecutionRun, error) {
	if run == nil {
		return nil, fmt.Errorf("migration execution run is required")
	}
	table, err := r.executionRunsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify migration execution runs table: %w", err)
	}

	record, err := r.runToRecord(tenantID, createdBy, run)
	if err != nil {
		return nil, err
	}
	if err := table.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"created_by":           record.CreatedBy,
			"status":               record.Status,
			"confirmed":            record.Confirmed,
			"resumed":              record.Resumed,
			"step_count":           record.StepCount,
			"succeeded_step_count": record.SucceededStepCount,
			"failed_step_count":    record.FailedStepCount,
			"skipped_step_count":   record.SkippedStepCount,
			"planned_step_count":   record.PlannedStepCount,
			"resumed_step_count":   record.ResumedStepCount,
			"file_names":           record.FileNames,
			"run_payload":          record.RunPayload,
			"updated_at":           record.UpdatedAt,
		}),
	}).Create(record).Error; err != nil {
		return nil, fmt.Errorf("save migration execution run: %w", err)
	}
	return run, nil
}

// ListExecutionRuns returns recent saved run snapshots.
func (r *GORMMigrationExecutionRunRepository) ListExecutionRuns(ctx context.Context, schemaName, tenantID string, filter MigrationExecutionRunFilter) ([]MigrationExecutionRun, error) {
	table, err := r.executionRunsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify migration execution runs table: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := table.Where("tenant_id = ?", strings.TrimSpace(tenantID))
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}

	var records []models.MigrationExecutionRunRecord
	if err := query.Order("created_at DESC, id ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list migration execution runs: %w", err)
	}
	runs := make([]MigrationExecutionRun, 0, len(records))
	for i := range records {
		run, err := recordToMigrationExecutionRun(&records[i])
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, nil
}

// GetExecutionRun returns one saved run snapshot.
func (r *GORMMigrationExecutionRunRepository) GetExecutionRun(ctx context.Context, schemaName, tenantID, runID string) (*MigrationExecutionRun, error) {
	table, err := r.executionRunsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify migration execution runs table: %w", err)
	}

	var record models.MigrationExecutionRunRecord
	err = table.Where("tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(runID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMigrationExecutionRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get migration execution run: %w", err)
	}
	return recordToMigrationExecutionRun(&record)
}

func (r *GORMMigrationExecutionRunRepository) runToRecord(tenantID, createdBy string, run *MigrationExecutionRun) (*models.MigrationExecutionRunRecord, error) {
	if strings.TrimSpace(run.ID) == "" {
		run.ID = uuid.New().String()
	}
	now := r.currentTime()
	if run.CreatedAt == nil {
		createdAt := now
		run.CreatedAt = &createdAt
	}
	updatedAt := now
	run.UpdatedAt = &updatedAt
	run.TenantID = strings.TrimSpace(tenantID)
	if strings.TrimSpace(run.CreatedBy) == "" {
		run.CreatedBy = strings.TrimSpace(createdBy)
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return nil, fmt.Errorf("marshal migration execution run: %w", err)
	}
	return &models.MigrationExecutionRunRecord{
		ID:                 run.ID,
		TenantID:           run.TenantID,
		CreatedBy:          run.CreatedBy,
		Status:             run.Summary.Status,
		Confirmed:          run.Summary.Confirmed,
		Resumed:            run.Summary.Resumed,
		StepCount:          run.Summary.StepCount,
		SucceededStepCount: run.Summary.SucceededStepCount,
		FailedStepCount:    run.Summary.FailedStepCount,
		SkippedStepCount:   run.Summary.SkippedStepCount,
		PlannedStepCount:   run.Summary.PlannedStepCount,
		ResumedStepCount:   run.Summary.ResumedStepCount,
		FileNames:          migrationExecutionRunFileNames(run),
		RunPayload:         payload,
		CreatedAt:          *run.CreatedAt,
		UpdatedAt:          *run.UpdatedAt,
	}, nil
}

func recordToMigrationExecutionRun(record *models.MigrationExecutionRunRecord) (*MigrationExecutionRun, error) {
	if record == nil {
		return nil, ErrMigrationExecutionRunNotFound
	}
	var run MigrationExecutionRun
	if len(record.RunPayload) > 0 {
		if err := json.Unmarshal(record.RunPayload, &run); err != nil {
			return nil, fmt.Errorf("parse migration execution run payload: %w", err)
		}
	}
	run.ID = record.ID
	run.TenantID = record.TenantID
	run.CreatedBy = record.CreatedBy
	run.CreatedAt = timePtr(record.CreatedAt)
	run.UpdatedAt = timePtr(record.UpdatedAt)
	if run.Summary.Status == "" {
		run.Summary.Status = record.Status
		run.Summary.Confirmed = record.Confirmed
		run.Summary.Resumed = record.Resumed
		run.Summary.StepCount = record.StepCount
		run.Summary.SucceededStepCount = record.SucceededStepCount
		run.Summary.FailedStepCount = record.FailedStepCount
		run.Summary.SkippedStepCount = record.SkippedStepCount
		run.Summary.PlannedStepCount = record.PlannedStepCount
		run.Summary.ResumedStepCount = record.ResumedStepCount
	}
	return &run, nil
}

func (r *GORMMigrationExecutionRunRepository) currentTime() time.Time {
	if r.now == nil {
		return time.Now().UTC()
	}
	return r.now().UTC()
}

func migrationExecutionRunFileNames(run *MigrationExecutionRun) []string {
	if run == nil {
		return nil
	}
	seen := map[string]bool{}
	fileNames := make([]string, 0, len(run.Steps))
	for _, step := range run.Steps {
		fileName := strings.TrimSpace(step.FileName)
		if fileName == "" || seen[fileName] {
			continue
		}
		seen[fileName] = true
		fileNames = append(fileNames, fileName)
	}
	return fileNames
}

func timePtr(value time.Time) *time.Time {
	copied := value
	return &copied
}
