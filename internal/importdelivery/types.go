// Package importdelivery owns the bounded, server-to-server archive-delivery
// boundary for SmartAccounts packages. It stages immutable source evidence;
// it has no accounting, invoice, payment, or journal-writing dependency.
package importdelivery

import "time"

const (
	ProviderSmartAccounts = "smartaccounts"
	SchemaVersionV1       = "v1"
	StatusReceiving       = "RECEIVING"
	StatusStagedReview    = "STAGED_REVIEW_REQUIRED"
	maxChunkBytes         = 1 << 20
)

type SourceIdentity struct {
	ID                       string `json:"id"`
	ValidationSnapshotSHA256 string `json:"validation_snapshot_sha256"`
}

type Authority struct {
	GeneralLedgerAuthority       string `json:"general_ledger_authority"`
	SmartAccountsGLAuthoritative bool   `json:"smartaccounts_gl_authoritative"`
}

// Scope carries only selection metadata. It deliberately contains no source
// query parameters. Dates are supplied by the bridge's proven capture policy.
type Scope struct {
	Mode           string `json:"mode"`
	DateFrom       string `json:"date_from,omitempty"`
	DateTo         string `json:"date_to,omitempty"`
	// ResourceIDs binds a package to the exact documented source services
	// selected for this run. It is selection metadata only, never a source
	// query or identifier.
	ResourceIDs    []string `json:"resource_ids,omitempty"`
	SourceAsOfDate string `json:"source_as_of_date,omitempty"`
	CutoffAt       string `json:"cutoff_at"`
}

type ResourceSummary struct {
	ResourceID  string `json:"resource_id"`
	RecordCount int    `json:"record_count"`
	SHA256      string `json:"sha256,omitempty"`
	ScopeSHA256 string `json:"scope_sha256,omitempty"`
}

type ArtifactManifest struct {
	ArtifactID      string `json:"artifact_id"`
	SHA256          string `json:"sha256"`
	ByteCount       int64  `json:"byte_count"`
	MediaType       string `json:"media_type"`
	ContentEncoding string `json:"content_encoding"`
}

// Manifest is accepted before any record or artifact bytes. Its hashes bind
// a selected source identity to exactly one OA target tenant and package.
type Manifest struct {
	SchemaVersion   string             `json:"schema_version"`
	PackageID       string             `json:"package_id"`
	ManifestSHA256  string             `json:"manifest_sha256"`
	PackageSHA256   string             `json:"package_sha256"`
	RecordsSHA256   string             `json:"records_sha256"`
	Provider        string             `json:"provider"`
	SourceCompanyID string             `json:"source_company_id"`
	SourceIdentity  SourceIdentity     `json:"source_identity"`
	Authority       Authority          `json:"authority"`
	Scope           Scope              `json:"scope"`
	RecordCount     int                `json:"record_count"`
	Resources       []ResourceSummary  `json:"resource_summaries"`
	Artifacts       []ArtifactManifest `json:"artifacts"`
}

// RecordChunk contains bounded base64-encoded NDJSON. The bytes are retained
// only in tenant archive storage and are never returned by this package.
type RecordChunk struct {
	Sequence        int    `json:"sequence"`
	RecordCount     int    `json:"record_count"`
	SHA256          string `json:"sha256"`
	ContentEncoding string `json:"content_encoding"`
	DataBase64      string `json:"data_base64"`
}

type ArtifactChunk struct {
	Sequence        int    `json:"sequence"`
	ChunkCount      int    `json:"chunk_count"`
	SHA256          string `json:"sha256"`
	ContentEncoding string `json:"content_encoding"`
	DataBase64      string `json:"data_base64"`
}

type FinalizeRequest struct {
	ManifestSHA256 string `json:"manifest_sha256"`
	PackageSHA256  string `json:"package_sha256"`
	RecordsSHA256  string `json:"records_sha256"`
	RecordCount    int    `json:"record_count"`
	ArtifactCount  int    `json:"artifact_count"`
}

// Status is the only delivery view returned by the internal handler. It has
// digest/count/status metadata, never raw records, artifact bytes, credentials
// paths, or an accounting posting plan.
type Status struct {
	PackageID          string    `json:"package_id"`
	TenantID           string    `json:"tenant_id"`
	SourceCompanyID    string    `json:"source_company_id"`
	Status             string    `json:"status"`
	ManifestSHA256     string    `json:"manifest_sha256"`
	PackageSHA256      string    `json:"package_sha256"`
	RecordCount        int       `json:"record_count"`
	RecordChunks       int       `json:"record_chunks"`
	NextRecordSequence int       `json:"next_record_sequence"`
	ArtifactCount      int       `json:"artifact_count"`
	ArtifactsComplete  int       `json:"artifacts_complete"`
	StagedSessionID    string    `json:"staged_session_id,omitempty"`
	Created            bool      `json:"created"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ChunkResult struct {
	Status             string `json:"status"`
	NextRecordSequence int    `json:"next_record_sequence,omitempty"`
	Created            bool   `json:"created"`
}
