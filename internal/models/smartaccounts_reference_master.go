package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsReferencePreviewRecord contains only projected reference-master
// actions. Canonical source payloads stay in external_import_record_chunks.
type SmartAccountsReferencePreviewRecord struct {
	ID              string          `gorm:"column:id;type:uuid;primaryKey"`
	TenantID        string          `gorm:"column:tenant_id;type:uuid;not null"`
	PackageID       string          `gorm:"column:package_id;size:255;not null"`
	SourceCompanyID string          `gorm:"column:source_company_id;size:255;not null"`
	PreviewSHA256   string          `gorm:"column:preview_sha256;size:64;not null"`
	Status          string          `gorm:"column:status;size:32;not null"`
	Plan            json.RawMessage `gorm:"column:plan;type:jsonb;not null"`
	Reconciliation  json.RawMessage `gorm:"column:reconciliation;type:jsonb;not null"`
	Issues          json.RawMessage `gorm:"column:issues;type:jsonb;not null"`
	CreatedBy       string          `gorm:"column:created_by;type:uuid"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null"`
	AppliedAt       *time.Time      `gorm:"column:applied_at"`
}

func (SmartAccountsReferencePreviewRecord) TableName() string {
	return "smartaccounts_reference_previews"
}

// SmartAccountsReferenceIdentityRecord is the durable tenant/source external
// identity and per-record resume state for non-financial reference masters.
type SmartAccountsReferenceIdentityRecord struct {
	ID              string     `gorm:"column:id;type:uuid;primaryKey"`
	TenantID        string     `gorm:"column:tenant_id;type:uuid;not null"`
	Provider        string     `gorm:"column:provider;size:64;not null"`
	SourceCompanyID string     `gorm:"column:source_company_id;size:255;not null"`
	EntityType      string     `gorm:"column:entity_type;size:64;not null"`
	ExternalID      string     `gorm:"column:external_id;size:255;not null"`
	Revision        string     `gorm:"column:revision;size:64;not null"`
	TargetID        string     `gorm:"column:target_id;type:uuid;not null"`
	Status          string     `gorm:"column:status;size:32;not null"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
	AppliedAt       *time.Time `gorm:"column:applied_at"`
}

func (SmartAccountsReferenceIdentityRecord) TableName() string {
	return "smartaccounts_reference_identities"
}
