package models

import "time"

// SmartAccountsBrowserCSVSchemaApprovalRecord is the durable, aggregate-only
// owner-review binding for one discovery/resource/schema tuple. It deliberately
// has no source selector, header names, CSV data, cookies, credentials, bridge
// token, or field-mapping expression; those stay private to the bridge.
type SmartAccountsBrowserCSVSchemaApprovalRecord struct {
	TenantID       string    `gorm:"column:tenant_id;type:uuid;primaryKey"`
	DiscoveryID    string    `gorm:"column:discovery_id;type:uuid;primaryKey"`
	ResourceID     string    `gorm:"column:resource_id;size:80;primaryKey"`
	SchemaID       string    `gorm:"column:schema_id;size:80;not null"`
	ReviewVersion  string    `gorm:"column:review_version;size:128;not null"`
	Confirmed      bool      `gorm:"column:confirmed;not null"`
	ReviewedAt     time.Time `gorm:"column:reviewed_at;not null"`
	ReviewAuditID  string    `gorm:"column:review_audit_id;type:uuid;not null"`
	ReviewedBy     string    `gorm:"column:reviewed_by;type:text;not null"`
	Status         string    `gorm:"column:status;size:32;not null"`
	ApprovalSHA256 *string   `gorm:"column:approval_sha256;size:64"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;default:now()"`
}

func (SmartAccountsBrowserCSVSchemaApprovalRecord) TableName() string {
	return "smartaccounts_browser_csv_schema_approvals"
}
