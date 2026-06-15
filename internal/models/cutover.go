package models

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// MigrationExecutionRunRecord stores a tenant-scoped migration execution run snapshot.
type MigrationExecutionRunRecord struct {
	ID                 string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID           string          `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	CreatedBy          string          `gorm:"column:created_by;type:text;not null;default:''" json:"created_by,omitempty"`
	Status             string          `gorm:"size:32;not null;index" json:"status"`
	Confirmed          bool            `gorm:"not null;default:false" json:"confirmed"`
	Resumed            bool            `gorm:"not null;default:false" json:"resumed"`
	StepCount          int             `gorm:"column:step_count;not null;default:0" json:"step_count"`
	SucceededStepCount int             `gorm:"column:succeeded_step_count;not null;default:0" json:"succeeded_step_count"`
	FailedStepCount    int             `gorm:"column:failed_step_count;not null;default:0" json:"failed_step_count"`
	SkippedStepCount   int             `gorm:"column:skipped_step_count;not null;default:0" json:"skipped_step_count"`
	PlannedStepCount   int             `gorm:"column:planned_step_count;not null;default:0" json:"planned_step_count"`
	ResumedStepCount   int             `gorm:"column:resumed_step_count;not null;default:0" json:"resumed_step_count"`
	FileNames          pq.StringArray  `gorm:"column:file_names;type:text[];not null" json:"file_names,omitempty"`
	RunPayload         json.RawMessage `gorm:"column:run_payload;type:jsonb;not null;default:'{}'" json:"run_payload"`
	CreatedAt          time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time       `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (MigrationExecutionRunRecord) TableName() string {
	return "migration_execution_runs"
}
