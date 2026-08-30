package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsBrowserBatchWorkflowRecord is the non-financial orchestration
// layer above an immutable onboarding batch. It deliberately holds only
// consent/digest/scope control metadata; relay capabilities and source bytes
// must never be persisted here.
type SmartAccountsBrowserBatchWorkflowRecord struct {
	BatchID                     string          `gorm:"column:batch_id;type:uuid;primaryKey" json:"batch_id"`
	OwnerID                     string          `gorm:"column:owner_id;type:text;not null" json:"owner_id"`
	SchemaVersion               string          `gorm:"column:schema_version;type:text;not null" json:"schema_version"`
	HistoryFrom                 string          `gorm:"column:history_from;type:date;not null" json:"history_from"`
	HeaderProbeConsentConfirmed bool            `gorm:"column:header_probe_consent_confirmed;not null;default:false" json:"header_probe_consent_confirmed"`
	PreparatoryManifestSHA256   string          `gorm:"column:preparatory_manifest_sha256;size:64;not null" json:"preparatory_manifest_sha256"`
	PreparatoryConsentedAt      time.Time       `gorm:"column:preparatory_consented_at;not null" json:"preparatory_consented_at"`
	TransferManifestSHA256      string          `gorm:"column:transfer_manifest_sha256;size:64" json:"transfer_manifest_sha256,omitempty"`
	TransferScope               json.RawMessage `gorm:"column:transfer_scope;type:jsonb" json:"transfer_scope,omitempty"`
	TransferConfirmedAt         *time.Time      `gorm:"column:transfer_confirmed_at" json:"transfer_confirmed_at,omitempty"`
	CreatedAt                   time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt                   time.Time       `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (SmartAccountsBrowserBatchWorkflowRecord) TableName() string {
	return "smartaccounts_browser_batch_workflows"
}

// SmartAccountsBrowserBatchSourceWorkflowRecord is a tenant-isolated source
// phase checkpoint. Lease identifiers are concurrency control UUIDs only;
// they are not browser, bridge, or source credentials.
type SmartAccountsBrowserBatchSourceWorkflowRecord struct {
	BatchID         string     `gorm:"column:batch_id;type:uuid;primaryKey" json:"batch_id"`
	SourceCompanyID string     `gorm:"column:source_company_id;type:text;primaryKey" json:"source_company_id"`
	TenantID        string     `gorm:"column:tenant_id;type:uuid;not null" json:"tenant_id"`
	Ordinal         int        `gorm:"column:ordinal;not null" json:"ordinal"`
	Phase           string     `gorm:"column:phase;type:text;not null" json:"phase"`
	PhaseGeneration int64      `gorm:"column:phase_generation;not null" json:"phase_generation"`
	AttemptCount    int        `gorm:"column:attempt_count;not null" json:"attempt_count"`
	LeaseID         *string    `gorm:"column:lease_id;type:uuid" json:"lease_id,omitempty"`
	LeaseExpiresAt  *time.Time `gorm:"column:lease_expires_at" json:"lease_expires_at,omitempty"`
	// CaptureRunID is a durable, non-secret bridge run identifier. It survives
	// a short-lived orchestration lease so an owner can rotate a relay
	// capability for the exact same capture after a browser restart.
	CaptureRunID            *string   `gorm:"column:capture_run_id;type:uuid" json:"capture_run_id,omitempty"`
	DiscoveryID             *string   `gorm:"column:discovery_id;type:uuid" json:"discovery_id,omitempty"`
	DiscoveryContractSHA256 string    `gorm:"column:discovery_contract_sha256;size:64" json:"discovery_contract_sha256,omitempty"`
	DiscoveryReceiptSHA256  string    `gorm:"column:discovery_receipt_sha256;size:64" json:"discovery_receipt_sha256,omitempty"`
	SchemaID                string    `gorm:"column:schema_id;type:text" json:"schema_id,omitempty"`
	SchemaApprovalSHA256    string    `gorm:"column:schema_approval_sha256;size:64" json:"schema_approval_sha256,omitempty"`
	PackageID               string    `gorm:"column:package_id;type:text" json:"package_id,omitempty"`
	PackageSHA256           string    `gorm:"column:package_sha256;size:64" json:"package_sha256,omitempty"`
	PreviewID               *string   `gorm:"column:preview_id;type:uuid" json:"preview_id,omitempty"`
	PreviewSHA256           string    `gorm:"column:preview_sha256;size:64" json:"preview_sha256,omitempty"`
	ReasonCode              string    `gorm:"column:reason_code;type:text;not null;default:''" json:"reason_code,omitempty"`
	CreatedAt               time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt               time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (SmartAccountsBrowserBatchSourceWorkflowRecord) TableName() string {
	return "smartaccounts_browser_batch_source_workflows"
}
