// Package smartaccountsreferences applies only explicitly mapped, non-financial
// SmartAccounts reference masters. It never creates a journal, invoice, or
// payment. Raw source payloads remain in the tenant-scoped import archive.
package smartaccountsreferences

import "time"

const (
	Provider = "smartaccounts"

	EntityAccount  = "account"
	EntityCustomer = "customer"
	EntityVendor   = "vendor"
	EntityItem     = "item"

	// Browser master-detail v1 is a separately reviewed, non-financial source
	// contract. It must not be confused with the older API capture schemas.
	BrowserMasterDetailManifestVersion = "smartaccounts-browser-master-detail-v1"
	BrowserClientsDetailSchema         = BrowserMasterDetailManifestVersion + "/clients_detail_v1"
	BrowserVendorsDetailSchema         = BrowserMasterDetailManifestVersion + "/vendors_detail_v1"
	BrowserArticlesDetailSchema        = BrowserMasterDetailManifestVersion + "/articles_detail_v1"
	BrowserDetailExternalIDMode        = "smartaccounts_browser_detail_link_id"
	BrowserDetailRelationship          = "non_posting_reference_to_authoritative_gl"

	StatusPreviewReady   = "PREVIEW_READY"
	StatusReviewRequired = "REVIEW_REQUIRED"
	StatusApplied        = "APPLIED"

	IdentityPending = "PENDING"
	IdentityApplied = "APPLIED"
)

// PreviewRequest limits a review to a caller-selected subset. Empty selects
// every supported reference-master entity present in the package.
type PreviewRequest struct {
	EntityTypes []string `json:"entity_types,omitempty"`
}

type ConfirmRequest struct {
	Confirm       bool   `json:"confirm"`
	PreviewID     string `json:"preview_id"`
	PreviewSHA256 string `json:"preview_sha256"`
}

// Action contains only safe projected fields. It never contains the canonical
// payload, source contact list, address, attachment, or financial data.
type Action struct {
	EntityType string `json:"entity_type"`
	ExternalID string `json:"external_id"`
	TargetID   string `json:"target_id"`
	Revision   string `json:"revision"`
	Action     string `json:"action"`
}

type Reconciliation struct {
	EntityType     string `json:"entity_type"`
	SourceRecords  int    `json:"source_records"`
	CreatePlanned  int    `json:"create_planned"`
	AlreadyApplied int    `json:"already_applied"`
	ReviewRequired int    `json:"review_required"`
	Tombstones     int    `json:"tombstones"`
}

type Issue struct {
	Code       string `json:"code"`
	EntityType string `json:"entity_type,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	Message    string `json:"message"`
}

type Preview struct {
	ID              string           `json:"id"`
	TenantID        string           `json:"tenant_id"`
	PackageID       string           `json:"package_id"`
	SourceCompanyID string           `json:"source_company_id"`
	Status          string           `json:"status"`
	PreviewSHA256   string           `json:"preview_sha256"`
	Actions         []Action         `json:"actions,omitempty"`
	Reconciliation  []Reconciliation `json:"reconciliation"`
	Issues          []Issue          `json:"issues,omitempty"`
	AppliedAt       *time.Time       `json:"applied_at,omitempty"`
}

// Identity is durable per tenant/provider/source/entity/external ID progress.
// A changed revision or tombstone is deliberately reviewed, not overwritten.
type Identity struct {
	EntityType string
	ExternalID string
	Revision   string
	TargetID   string
	Status     string
}
