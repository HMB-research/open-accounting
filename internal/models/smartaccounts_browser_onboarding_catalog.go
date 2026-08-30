package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsBrowserOnboardingCatalogReceiptRecord is the durable,
// server-issued capability/receipt for a relay-observed company picker. The
// raw capability and nonce are stored only as SHA-256 digests. Companies are
// opaque selectors plus display names, never source rows, cookies, or API
// credentials.
type SmartAccountsBrowserOnboardingCatalogReceiptRecord struct {
	ID               string          `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	WorkflowID       string          `gorm:"column:workflow_id;type:uuid;not null" json:"workflow_id"`
	OwnerID          string          `gorm:"column:owner_id;type:text;not null" json:"owner_id"`
	TokenSHA256      string          `gorm:"column:token_sha256;size:64;not null" json:"-"`
	NonceSHA256      string          `gorm:"column:nonce_sha256;size:64;not null" json:"-"`
	SchemaVersion    string          `gorm:"column:schema_version;type:text;not null" json:"schema_version"`
	IntentVersion    string          `gorm:"column:intent_version;type:text;not null" json:"intent_version"`
	SourceIDVersion  string          `gorm:"column:source_id_version;type:text;not null" json:"source_id_version"`
	DigestAlgorithm  string          `gorm:"column:digest_algorithm;type:text;not null" json:"digest_algorithm"`
	Status           string          `gorm:"column:status;type:text;not null" json:"status"`
	CatalogSHA256    *string         `gorm:"column:catalog_sha256;size:64" json:"catalog_sha256,omitempty"`
	CatalogCount     *int            `gorm:"column:catalog_count" json:"catalog_count,omitempty"`
	Companies        json.RawMessage `gorm:"column:companies;type:jsonb" json:"-"`
	ObservedAt       *time.Time      `gorm:"column:observed_at" json:"observed_at,omitempty"`
	ExpiresAt        time.Time       `gorm:"column:expires_at;not null" json:"expires_at"`
	ReceiptExpiresAt *time.Time      `gorm:"column:receipt_expires_at" json:"receipt_expires_at,omitempty"`
	AcceptedAt       *time.Time      `gorm:"column:accepted_at" json:"accepted_at,omitempty"`
	CreatedAt        time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time       `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (SmartAccountsBrowserOnboardingCatalogReceiptRecord) TableName() string {
	return "smartaccounts_browser_onboarding_catalog_receipts"
}
