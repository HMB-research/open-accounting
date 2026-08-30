package models

import (
	"encoding/json"
	"time"
)

// SmartAccountsSyncControlRecord stores only a secret-manager reference, never
// a SmartAccounts credential. API response models intentionally omit that
// reference; the bridge resolves it later in a private execution environment.
type SmartAccountsSyncControlRecord struct {
	TenantID          string     `gorm:"column:tenant_id;type:uuid;primaryKey" json:"tenant_id"`
	SourceCompanyID   string     `gorm:"column:source_company_id;size:255;not null;primaryKey" json:"source_company_id"`
	SourceCompanyName string     `gorm:"column:source_company_name;type:text;not null" json:"source_company_name"`
	SecretReference   string     `gorm:"column:secret_reference;type:text;not null" json:"-"`
	CreatedBy         string     `gorm:"column:created_by;type:text;not null;default:''" json:"created_by,omitempty"`
	DryRunRequestedAt *time.Time `gorm:"column:dry_run_requested_at" json:"dry_run_requested_at,omitempty"`
	CaptureRunID      string     `gorm:"column:capture_run_id;type:text" json:"-"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

// SmartAccountsBrowserPairingRecord stores the one-time token hash used by a
// Brave browser relay. SourceCompanyID is only the opaque UI-company selector
// needed to let the authenticated OA page resume its pairing; it is not a
// source-company name, credential, cookie, source record, or token plaintext.
type SmartAccountsBrowserPairingRecord struct {
	ID                      string  `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	TenantID                string  `gorm:"column:tenant_id;type:uuid;not null" json:"tenant_id"`
	TokenSHA256             string  `gorm:"column:token_sha256;size:64;not null" json:"-"`
	ExpectedSourceCompanyID *string `gorm:"column:expected_source_company_id;size:255" json:"-"`
	// A browser pairing has no source selector until the Brave relay claims it.
	// Keep that distinction as SQL NULL: the pairing table's state constraint
	// uses NULL to make an unclaimed token incapable of naming a source.
	SourceCompanyID *string    `gorm:"column:source_company_id;size:255" json:"-"`
	CreatedBy       string     `gorm:"column:created_by;type:text;not null;default:''" json:"created_by,omitempty"`
	Status          string     `gorm:"column:status;type:text;not null" json:"status"`
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	ClaimedAt       *time.Time `gorm:"column:claimed_at" json:"claimed_at,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

func (SmartAccountsBrowserPairingRecord) TableName() string {
	return "smartaccounts_browser_pairings"
}

// SmartAccountsBrowserOnboardingBindingRecord reserves one opaque browser
// source selector for exactly one OA tenant. It carries only source/target
// metadata and a pairing UUID; no token, cookie, CSV, or financial state.
type SmartAccountsBrowserOnboardingBindingRecord struct {
	SourceCompanyID   string    `gorm:"column:source_company_id;size:255;primaryKey" json:"source_company_id"`
	SourceCompanyName string    `gorm:"column:source_company_name;type:text;not null" json:"source_company_name"`
	TenantID          *string   `gorm:"column:tenant_id;type:uuid" json:"tenant_id,omitempty"`
	TenantName        string    `gorm:"column:tenant_name;type:text;not null;default:''" json:"tenant_name"`
	PairingID         *string   `gorm:"column:pairing_id;type:uuid" json:"pairing_id,omitempty"`
	Status            string    `gorm:"column:status;type:text;not null" json:"status"`
	CreatedBy         string    `gorm:"column:created_by;type:text;not null;default:''" json:"created_by,omitempty"`
	CreatedAt         time.Time `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
}

func (SmartAccountsBrowserOnboardingBindingRecord) TableName() string {
	return "smartaccounts_browser_onboarding_bindings"
}

// SmartAccountsBrowserCaptureAuthorizationRecord stores the hash of a
// short-lived browser-relay capability and its immutable tenant/source/run
// scope. It never stores a cookie, API key, CSV bytes, browser header, or
// raw capability.
type SmartAccountsBrowserCaptureAuthorizationRecord struct {
	RunID           string          `gorm:"column:run_id;type:uuid;primaryKey"`
	TenantID        string          `gorm:"column:tenant_id;type:uuid;primaryKey"`
	SourceCompanyID string          `gorm:"column:source_company_id;size:255;not null"`
	ManifestVersion string          `gorm:"column:manifest_version;size:128;not null"`
	Scope           json.RawMessage `gorm:"column:scope;type:jsonb;not null"`
	TokenSHA256     string          `gorm:"column:token_sha256;size:64;not null"`
	CreatedBy       string          `gorm:"column:created_by;type:text;not null"`
	ExpiresAt       time.Time       `gorm:"column:expires_at;not null"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null"`
}

func (SmartAccountsBrowserCaptureAuthorizationRecord) TableName() string {
	return "smartaccounts_browser_capture_authorizations"
}

// SmartAccountsBrowserDiscoveryAuthorizationRecord contains only durable
// action-time discovery consent and the bridge-safe aggregate receipt. It
// never stores a browser result, source row, header value/name, cookie,
// credential, query, control identifier, relay capability, or export body.
type SmartAccountsBrowserDiscoveryAuthorizationRecord struct {
	DiscoveryID                  string          `gorm:"column:discovery_id;type:uuid;primaryKey"`
	TenantID                     string          `gorm:"column:tenant_id;type:uuid;primaryKey"`
	SourceCompanyID              string          `gorm:"column:source_company_id;size:255;not null"`
	ManifestVersion              string          `gorm:"column:manifest_version;type:text;not null"`
	ContractVersion              string          `gorm:"column:contract_version;type:text;not null"`
	ResourceIDs                  json.RawMessage `gorm:"column:resource_ids;type:jsonb;not null"`
	MetadataOnlyConsentConfirmed bool            `gorm:"column:metadata_only_consent_confirmed;not null"`
	HeaderProbeConsentConfirmed  bool            `gorm:"column:header_probe_consent_confirmed;not null"`
	ConsentedAt                  time.Time       `gorm:"column:consented_at;not null"`
	CreatedBy                    string          `gorm:"column:created_by;type:text;not null"`
	ExpiresAt                    time.Time       `gorm:"column:expires_at;not null"`
	ReceiptStatus                *string         `gorm:"column:receipt_status;type:text"`
	ContractSHA256               *string         `gorm:"column:contract_sha256;size:64"`
	ResourceCount                *int            `gorm:"column:resource_count"`
	CaptureReadyCount            *int            `gorm:"column:capture_ready_count"`
	FilterRequiredCount          *int            `gorm:"column:filter_contract_required_count"`
	PageOnlyRequiredCount        *int            `gorm:"column:page_only_contract_required_count"`
	PrivateEndpointCount         *int            `gorm:"column:private_endpoint_required_count"`
	BindingBlockedCount          *int            `gorm:"column:binding_blocked_count"`
	ReceiptRecordedAt            *time.Time      `gorm:"column:receipt_recorded_at"`
	CreatedAt                    time.Time       `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt                    time.Time       `gorm:"column:updated_at;not null;default:now()"`
}

func (SmartAccountsBrowserDiscoveryAuthorizationRecord) TableName() string {
	return "smartaccounts_browser_discovery_authorizations"
}

// SmartAccountsBrowserCaptureWorkflowRecord persists only the operator's
// explicit historical-start policy and server-derived capture bounds. It is a
// resumable control record: no relay capability, cookie, CSV, source row, or
// private bridge state is stored here.
type SmartAccountsBrowserCaptureWorkflowRecord struct {
	ID              string    `gorm:"column:id;type:uuid;primaryKey"`
	TenantID        string    `gorm:"column:tenant_id;type:uuid;not null"`
	SourceCompanyID string    `gorm:"column:source_company_id;size:255;not null"`
	FromInclusive   time.Time `gorm:"column:from_inclusive;type:date;not null"`
	ToInclusive     time.Time `gorm:"column:to_inclusive;type:date;not null"`
	CutoffAt        time.Time `gorm:"column:cutoff_at;not null"`
	CaptureRunID    *string   `gorm:"column:capture_run_id;type:uuid"`
	Status          string    `gorm:"column:status;type:text;not null"`
	CreatedBy       string    `gorm:"column:created_by;type:text;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (SmartAccountsBrowserCaptureWorkflowRecord) TableName() string {
	return "smartaccounts_browser_capture_workflows"
}

// TableName returns the public, source-to-target-tenant keyed sync-control table.
func (SmartAccountsSyncControlRecord) TableName() string {
	return "smartaccounts_sync_controls"
}
