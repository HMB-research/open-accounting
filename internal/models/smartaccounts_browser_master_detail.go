package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsBrowserMasterDetailAuthorizationRecord holds one protected,
// short-lived relay capability digest for one source/resource snapshot. It has
// no browser data, cookies, source rows, raw token, or financial plan.
type SmartAccountsBrowserMasterDetailAuthorizationRecord struct {
	RunID           string          `gorm:"column:run_id;type:uuid;primaryKey"`
	TenantID        string          `gorm:"column:tenant_id;type:uuid;primaryKey"`
	BatchID         string          `gorm:"column:batch_id;type:uuid;not null"`
	SourceCompanyID string          `gorm:"column:source_company_id;type:text;not null"`
	SnapshotDate    string          `gorm:"column:snapshot_date;type:date;not null"`
	ManifestVersion string          `gorm:"column:manifest_version;type:text;not null"`
	ResourceID      string          `gorm:"column:resource_id;type:text;not null"`
	SchemaID        string          `gorm:"column:schema_id;type:text;not null"`
	SourceSchema    string          `gorm:"column:source_schema;type:text;not null"`
	Contract        json.RawMessage `gorm:"column:contract;type:jsonb;not null"`
	ContractSHA256  string          `gorm:"column:contract_sha256;size:64;not null"`
	ApprovalSHA256  string          `gorm:"column:approval_sha256;size:64;not null"`
	Scope           json.RawMessage `gorm:"column:scope;type:jsonb;not null"`
	TokenSHA256     string          `gorm:"column:token_sha256;size:64;not null"`
	CreatedBy       string          `gorm:"column:created_by;type:text;not null"`
	ExpiresAt       time.Time       `gorm:"column:expires_at;not null"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null;default:now()"`
}

func (SmartAccountsBrowserMasterDetailAuthorizationRecord) TableName() string {
	return "smartaccounts_browser_master_detail_authorizations"
}
