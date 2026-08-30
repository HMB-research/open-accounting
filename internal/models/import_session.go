package models

import (
	"encoding/json"
	"time"
)

// ImportSessionRecord is the durable, tenant-scoped receipt for a validated
// external canonical import package. It intentionally stores package metadata
// and validation results only; raw source records remain outside this table.
type ImportSessionRecord struct {
	ID                 string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID           string          `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Provider           string          `gorm:"column:provider;size:64;not null" json:"provider"`
	SourceCompanyID    string          `gorm:"column:source_company_id;size:255;not null" json:"source_company_id"`
	SchemaVersion      string          `gorm:"column:schema_version;size:32;not null" json:"schema_version"`
	PackageSHA256      string          `gorm:"column:package_sha256;size:64;not null" json:"package_sha256"`
	Status             string          `gorm:"column:status;size:32;not null" json:"status"`
	RecordCount        int             `gorm:"column:record_count;not null;default:0" json:"record_count"`
	EntityCounts       json.RawMessage `gorm:"column:entity_counts;type:jsonb;not null;default:'{}'" json:"entity_counts"`
	LedgerVerification json.RawMessage `gorm:"column:ledger_verification;type:jsonb;not null;default:'{}'" json:"ledger_verification"`
	LedgerPlanInput    json.RawMessage `gorm:"column:ledger_plan_input;type:jsonb;not null;default:'[]'" json:"-"`
	Validation         json.RawMessage `gorm:"column:validation;type:jsonb;not null;default:'{}'" json:"validation"`
	CreatedBy          string          `gorm:"column:created_by;type:text;not null;default:''" json:"created_by,omitempty"`
	CreatedAt          time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

// TableName returns the tenant-local import session table name.
func (ImportSessionRecord) TableName() string {
	return "import_sessions"
}

// ImportSourceBinding binds one external provider/company identity to one Open
// Accounting tenant. It is intentionally public-schema metadata so the same
// source company cannot be received into multiple tenant schemas by mistake.
type ImportSourceBinding struct {
	Provider        string    `gorm:"column:provider;size:64;primaryKey" json:"provider"`
	SourceCompanyID string    `gorm:"column:source_company_id;size:255;primaryKey" json:"source_company_id"`
	TenantID        string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

// TableName returns the public binding registry table name.
func (ImportSourceBinding) TableName() string {
	return "import_source_bindings"
}
