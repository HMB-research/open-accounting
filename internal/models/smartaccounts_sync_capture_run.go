package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsSyncCaptureRunRecord retains only safe bridge progress. The
// complete source archive remains in the tenant-isolated import-delivery
// tables; this public control metadata must never receive source rows,
// credentials, cursors, queries, or private paths.
type SmartAccountsSyncCaptureRunRecord struct {
	TenantID        string          `gorm:"column:tenant_id;type:uuid;primaryKey"`
	SourceCompanyID string          `gorm:"column:source_company_id;size:255;primaryKey"`
	RunID           string          `gorm:"column:run_id;size:128;primaryKey"`
	Progress        json.RawMessage `gorm:"column:progress;type:jsonb;not null"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;not null;default:now()"`
}

func (SmartAccountsSyncCaptureRunRecord) TableName() string {
	return "smartaccounts_sync_capture_run_history"
}
