package models

import "time"

// SmartAccountsBrowserCommercialDetailAuthorizationRecord is durable control
// state for one browser-only commercial evidence relay. It contains no source
// records, display names, monetary values, reviewed route contract, or raw
// capability. Contract and consent are represented by immutable digests only.
type SmartAccountsBrowserCommercialDetailAuthorizationRecord struct {
	RunID           string     `gorm:"column:run_id;type:uuid;primaryKey"`
	TenantID        string     `gorm:"column:tenant_id;type:uuid;primaryKey"`
	BatchID         string     `gorm:"column:batch_id;type:uuid;not null"`
	WorkflowID      string     `gorm:"column:workflow_id;type:uuid;not null"`
	SourceCompanyID string     `gorm:"column:source_company_id;type:text;not null"`
	ManifestVersion string     `gorm:"column:manifest_version;type:text;not null"`
	ResourceID      string     `gorm:"column:resource_id;type:text;not null"`
	Sequence        int        `gorm:"column:sequence;not null"`
	SchemaID        string     `gorm:"column:schema_id;type:text;not null"`
	SourceSchema    string     `gorm:"column:source_schema;type:text;not null"`
	ReviewAuditID   string     `gorm:"column:review_audit_id;type:uuid;not null"`
	ReviewedAt      time.Time  `gorm:"column:reviewed_at;not null"`
	ContractSHA256  string     `gorm:"column:contract_sha256;size:64;not null"`
	RouteSHA256     string     `gorm:"column:route_sha256;size:64;not null"`
	ConsentSHA256   string     `gorm:"column:consent_sha256;size:64;not null"`
	FromInclusive   string     `gorm:"column:from_inclusive;type:date;not null"`
	ToInclusive     string     `gorm:"column:to_inclusive;type:date;not null"`
	CutoffAt        time.Time  `gorm:"column:cutoff_at;not null"`
	TokenSHA256     string     `gorm:"column:token_sha256;size:64;not null"`
	Status          string     `gorm:"column:status;type:text;not null"`
	NDJSONSHA256    string     `gorm:"column:ndjson_sha256;size:64"`
	RecordCount     int        `gorm:"column:record_count;not null;default:0"`
	ReviewRequired  int        `gorm:"column:review_required;not null;default:0"`
	PackageID       string     `gorm:"column:package_id;type:text"`
	PackageSHA256   string     `gorm:"column:package_sha256;size:64"`
	BridgeStartedAt *time.Time `gorm:"column:bridge_started_at"`
	CreatedBy       string     `gorm:"column:created_by;type:text;not null"`
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;default:now()"`
}

func (SmartAccountsBrowserCommercialDetailAuthorizationRecord) TableName() string {
	return "smartaccounts_browser_commercial_detail_authorizations"
}
