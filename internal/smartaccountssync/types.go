// Package smartaccountssync owns the tenant-scoped control boundary for a
// future SmartAccounts bridge. It intentionally cannot contact SmartAccounts
// or apply accounting entries.
package smartaccountssync

import "time"

const (
	Provider                        = "smartaccounts"
	GeneralLedgerAuthority          = "smartaccounts"
	InvoicePaymentModeNonPosting    = "NON_POSTING"
	CaptureStatusNotRequested       = "NOT_REQUESTED"
	CaptureStatusAwaitingBridge     = "AWAITING_BRIDGE_CAPTURE"
	PlanStatusNotRequested          = "NOT_REQUESTED"
	PlanStatusAwaitingCapturedInput = "AWAITING_CAPTURE"
	ReconciliationStatusPending     = "PENDING_SOURCE_EVIDENCE"
)

// SourceCandidate is a bridge-discovered source choice. In v1 it represents
// configured bridge catalog metadata only; no live provider call occurs.
type SourceCandidate struct {
	Provider                   string `json:"provider"`
	SourceCompanyID            string `json:"source_company_id"`
	SourceCompanyName          string `json:"source_company_name"`
	Default                    bool   `json:"default"`
	BridgeVerified             bool   `json:"bridge_verified"`
	GeneralLedgerAuthoritative bool   `json:"general_ledger_authoritative"`
	InvoicePaymentMode         string `json:"invoice_payment_mode"`
}

// SourceDiscovery identifies what this Open Accounting deployment can stage
// with the bridge. It is intentionally explicit that no source data was read.
type SourceDiscovery struct {
	BridgeAvailable   bool              `json:"bridge_available"`
	LiveDataContacted bool              `json:"live_data_contacted"`
	Sources           []SourceCandidate `json:"sources"`
}

// ConfigureRequest is internal-only: it accepts the opaque reference returned
// by the private bridge after it has received and validated transient source
// credentials. It must never contain a raw SmartAccounts key.
type ConfigureRequest struct {
	SourceCompanyID              string `json:"source_company_id"`
	SecretReference              string `json:"secret_reference"`
	SmartAccountsGLAuthoritative bool   `json:"smartaccounts_gl_authoritative"`
	InvoicePaymentMode           string `json:"invoice_payment_mode"`
}

// ConnectRequest is accepted by the OA connection endpoint. APIKey and
// APISecret are transient: handlers pass them only to the private bridge and
// must not store, log, return, or retain them after the request completes.
type ConnectRequest struct {
	APIKey                       string `json:"api_key"`
	APISecret                    string `json:"api_secret"`
	SmartAccountsGLAuthoritative bool   `json:"smartaccounts_gl_authoritative"`
	InvoicePaymentMode           string `json:"invoice_payment_mode"`
}

// CaptureRequest starts or resumes a bridge-owned read-only source capture.
// It intentionally has no source_company_id: the bridge derives and enforces
// the source identity from the validated connection.
type CaptureRequest struct {
	ScopeMode string `json:"scope_mode,omitempty"`
	DateFrom  string `json:"date_from,omitempty"`
	DateTo    string `json:"date_to,omitempty"`
	// ResourceIDs is an explicit documented subset for a date-window follow-up.
	// An omitted list is the bridge's complete documented catalog; it never
	// implies another tenant or source company.
	ResourceIDs []string `json:"resource_ids,omitempty"`
	MaxPages    int      `json:"max_pages,omitempty"`
	RateBudget  int      `json:"rate_budget,omitempty"`
	ResumeRunID string   `json:"resume_run_id,omitempty"`
}

// CaptureProgress is a safe UI-facing summary of a bridge run. It never
// contains source records, credentials, source queries, evidence paths, or
// bridge cursor values.
type CaptureProgress struct {
	RunID          string                  `json:"run_id"`
	Status         string                  `json:"status"`
	ScopeMode      string                  `json:"scope_mode"`
	DateFrom       string                  `json:"date_from,omitempty"`
	DateTo         string                  `json:"date_to,omitempty"`
	ResourceIDs    []string                `json:"resource_ids"`
	SourceAsOfDate string                  `json:"source_as_of_date"`
	CutoffAt       string                  `json:"cutoff_at"`
	Resources      []CaptureResourceStatus `json:"resources"`
	Summary        CaptureSummary          `json:"summary"`
	Staging        *CaptureStaging         `json:"staging,omitempty"`
}

// CaptureStaging is bridge-safe delivery metadata. It contains no source
// rows, filenames, credentials, cursor, or archive path.
type CaptureStaging struct {
	PackageID                  string `json:"package_id"`
	PackageSHA256              string `json:"package_sha256"`
	Status                     string `json:"status"`
	RecordChunksAcknowledged   int    `json:"record_chunks_acknowledged"`
	ArtifactChunksAcknowledged int    `json:"artifact_chunks_acknowledged"`
	Finalized                  bool   `json:"finalized"`
}

type CaptureResourceStatus struct {
	ResourceID     string `json:"resource_id"`
	EndpointStatus string `json:"endpoint_status"`
	Status         string `json:"status"`
	ReasonCode     string `json:"reason_code,omitempty"`
	PageCount      int    `json:"page_count,omitempty"`
	DeletedCount   int    `json:"deleted_count,omitempty"`
	ByteCount      int64  `json:"byte_count,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	ScopeSHA256    string `json:"scope_sha256,omitempty"`
	NextEligibleAt string `json:"next_eligible_at,omitempty"`
}

type CaptureSummary struct {
	Total                  int `json:"total"`
	Completed              int `json:"completed"`
	Running                int `json:"running"`
	Interrupted            int `json:"interrupted"`
	RateLimited            int `json:"rate_limited"`
	ReviewRequired         int `json:"review_required"`
	DependencyRequired     int `json:"dependency_required"`
	BraveDiscoveryRequired int `json:"brave_discovery_required"`
}

// ConfirmApplyRequest is deliberately explicit: a future executor may only
// be invoked through this confirmation request. v1 still rejects it because
// no live capture or financial apply implementation exists.
type ConfirmApplyRequest struct {
	Confirm bool `json:"confirm"`
}

// Control stores the opaque bridge reference internally. It is never returned
// directly to callers or logged by this package.
type Control struct {
	TenantID          string
	SourceCompanyID   string
	SourceCompanyName string
	SecretReference   string
	CreatedBy         string
	DryRunRequestedAt *time.Time
	CaptureRunID      string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SyncStatus is the safe progress view returned to the web UI. It includes no
// raw key, secret reference, package payload, or financial data.
type SyncStatus struct {
	Provider                     string           `json:"provider"`
	SourceCompanyID              string           `json:"source_company_id,omitempty"`
	SourceCompanyName            string           `json:"source_company_name,omitempty"`
	Configured                   bool             `json:"configured"`
	SecretReferenceConfigured    bool             `json:"secret_reference_configured"`
	SmartAccountsGLAuthoritative bool             `json:"smartaccounts_gl_authoritative"`
	InvoicePaymentMode           string           `json:"invoice_payment_mode"`
	CaptureStatus                string           `json:"capture_status"`
	PlanStatus                   string           `json:"plan_status"`
	ReconciliationStatus         string           `json:"reconciliation_status"`
	FinancialApplyEligible       bool             `json:"financial_apply_eligible"`
	ExplicitConfirmationRequired bool             `json:"explicit_confirmation_required"`
	FinancialWritesStarted       bool             `json:"financial_writes_started"`
	DryRunRequestedAt            *time.Time       `json:"dry_run_requested_at,omitempty"`
	CaptureRunID                 string           `json:"capture_run_id,omitempty"`
	CaptureProgress              *CaptureProgress `json:"capture_progress,omitempty"`
	// CaptureProgresses is durable safe metadata for every capture requested
	// for this binding, newest first. It lets a date-window follow-up coexist
	// with an already-finalized full-history package.
	CaptureProgresses []CaptureProgress `json:"capture_progresses,omitempty"`
	NextAction        string            `json:"next_action"`
}
