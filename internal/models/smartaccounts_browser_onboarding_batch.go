package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsBrowserOnboardingBatchRecord is an immutable, owner-confirmed
// selection manifest. It intentionally stores only opaque source selectors,
// display metadata needed to create a target tenant, digests, and progress;
// it never stores browser/session credentials, source rows, or accounting
// instructions.
type SmartAccountsBrowserOnboardingBatchRecord struct {
	ID                    string          `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	OwnerID               string          `gorm:"column:owner_id;type:text;not null" json:"owner_id"`
	CatalogReceiptID      string          `gorm:"column:catalog_receipt_id;type:uuid;not null" json:"catalog_receipt_id"`
	RelayObservedAt       time.Time       `gorm:"column:relay_observed_at;not null" json:"relay_observed_at"`
	Mode                  string          `gorm:"column:mode;type:text;not null" json:"mode"`
	SelectedSources       json.RawMessage `gorm:"column:selected_sources;type:jsonb;not null" json:"selected_sources"`
	ObservedSourceIDs     json.RawMessage `gorm:"column:observed_source_ids;type:jsonb;not null" json:"observed_source_ids"`
	ObservedSourcesSHA256 string          `gorm:"column:observed_sources_sha256;size:64;not null" json:"observed_sources_sha256"`
	ManifestSHA256        string          `gorm:"column:manifest_sha256;size:64;not null" json:"manifest_sha256"`
	Status                string          `gorm:"column:status;type:text;not null" json:"status"`
	CreatedAt             time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt             time.Time       `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (SmartAccountsBrowserOnboardingBatchRecord) TableName() string {
	return "smartaccounts_browser_onboarding_batches"
}

// SmartAccountsBrowserOnboardingBatchOutcomeRecord is safe, per-source
// onboarding progress. Pairing IDs are opaque control UUIDs; no pairing token
// or source data is retained.
type SmartAccountsBrowserOnboardingBatchOutcomeRecord struct {
	BatchID           string    `gorm:"column:batch_id;type:uuid;primaryKey" json:"batch_id"`
	SourceCompanyID   string    `gorm:"column:source_company_id;type:text;primaryKey" json:"source_company_id"`
	SourceCompanyName string    `gorm:"column:source_company_name;type:text;not null" json:"source_company_name"`
	TenantID          *string   `gorm:"column:tenant_id;type:uuid" json:"tenant_id,omitempty"`
	TenantName        string    `gorm:"column:tenant_name;type:text;not null;default:''" json:"tenant_name"`
	PairingID         *string   `gorm:"column:pairing_id;type:uuid" json:"pairing_id,omitempty"`
	Status            string    `gorm:"column:status;type:text;not null" json:"status"`
	TenantCreated     bool      `gorm:"column:tenant_created;not null" json:"tenant_created"`
	TenantReused      bool      `gorm:"column:tenant_reused;not null" json:"tenant_reused"`
	ReasonCode        string    `gorm:"column:reason_code;type:text;not null;default:''" json:"reason_code,omitempty"`
	CreatedAt         time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (SmartAccountsBrowserOnboardingBatchOutcomeRecord) TableName() string {
	return "smartaccounts_browser_onboarding_batch_outcomes"
}
