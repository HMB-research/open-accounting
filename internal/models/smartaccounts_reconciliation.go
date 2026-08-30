package models

import "time"

// SmartAccountsGLApplyReceiptRecord is public control evidence for an exact
// confirmed GL apply and its later exact replay. It deliberately holds only
// IDs, digests, actors and timestamps—never journal lines or amounts.
type SmartAccountsGLApplyReceiptRecord struct {
	ID                    string     `gorm:"column:id;type:uuid;primaryKey"`
	TenantID              string     `gorm:"column:tenant_id;type:uuid;not null"`
	SourceCompanyID       string     `gorm:"column:source_company_id;type:text;not null"`
	PackageID             string     `gorm:"column:package_id;type:text;not null"`
	PreviewID             string     `gorm:"column:preview_id;type:uuid;not null"`
	PreviewSHA256         string     `gorm:"column:preview_sha256;size:64;not null"`
	MappingSnapshotSHA256 string     `gorm:"column:mapping_snapshot_sha256;size:64;not null"`
	AppliedIdentitySHA256 string     `gorm:"column:applied_identity_sha256;size:64;not null"`
	TolerancePolicySHA256 string     `gorm:"column:tolerance_policy_sha256;size:64;not null"`
	MappingCount          int        `gorm:"column:mapping_count;not null"`
	AppliedIdentityCount  int        `gorm:"column:applied_identity_count;not null"`
	FirstAppliedBy        string     `gorm:"column:first_applied_by;type:text;not null"`
	FirstAppliedAt        time.Time  `gorm:"column:first_applied_at;not null"`
	ExactReplayBy         *string    `gorm:"column:exact_replay_by;type:text"`
	ExactReplayAt         *time.Time `gorm:"column:exact_replay_at"`
}

func (SmartAccountsGLApplyReceiptRecord) TableName() string { return "smartaccounts_gl_apply_receipts" }

// SmartAccountsGLApplyReceiptMappingRecord is append-only ID-only evidence
// for the mapping set used at apply. Account names, amounts and source rows
// are deliberately excluded.
type SmartAccountsGLApplyReceiptMappingRecord struct {
	ReceiptID               string `gorm:"column:receipt_id;type:uuid;primaryKey"`
	SourceAccountExternalID string `gorm:"column:source_account_external_id;type:text;primaryKey"`
	TargetAccountID         string `gorm:"column:target_account_id;type:uuid;not null"`
}

func (SmartAccountsGLApplyReceiptMappingRecord) TableName() string {
	return "smartaccounts_gl_apply_receipt_mappings"
}

// SmartAccountsGLApplyReceiptIdentityRecord is an append-only ID/revision
// snapshot of journals actually marked posted for a receipt. It contains no
// journal body or amounts.
type SmartAccountsGLApplyReceiptIdentityRecord struct {
	ReceiptID     string `gorm:"column:receipt_id;type:uuid;primaryKey"`
	ExternalID    string `gorm:"column:external_id;type:text;primaryKey"`
	Revision      string `gorm:"column:revision;size:64;not null"`
	ReservationID string `gorm:"column:reservation_id;type:uuid;not null"`
	JournalID     string `gorm:"column:journal_id;type:uuid;not null"`
}

func (SmartAccountsGLApplyReceiptIdentityRecord) TableName() string {
	return "smartaccounts_gl_apply_receipt_identities"
}

// SmartAccountsGLTolerancePolicyRecord is an immutable accountant-approved
// policy handle. It intentionally retains only digests and source/package
// scope binding, never tolerance rules or financial amounts.
type SmartAccountsGLTolerancePolicyRecord struct {
	ID                    string    `gorm:"column:id;type:uuid;primaryKey"`
	AlgorithmVersion      string    `gorm:"column:algorithm_version;type:text;not null"`
	TenantID              string    `gorm:"column:tenant_id;type:uuid;not null"`
	SourceCompanyID       string    `gorm:"column:source_company_id;type:text;not null"`
	PackageID             string    `gorm:"column:package_id;type:text;not null"`
	ScopeSHA256           string    `gorm:"column:scope_sha256;size:64;not null"`
	PreviewSHA256         string    `gorm:"column:preview_sha256;size:64;not null"`
	TolerancePolicySHA256 string    `gorm:"column:tolerance_policy_sha256;size:64;not null"`
	ApprovedBy            string    `gorm:"column:approved_by;type:text;not null"`
	ApprovedAt            time.Time `gorm:"column:approved_at;not null"`
}

func (SmartAccountsGLTolerancePolicyRecord) TableName() string {
	return "smartaccounts_gl_tolerance_policies"
}

// SmartAccountsReconciliationEvaluationRecord contains a closed set of
// non-sensitive technical evidence handles. Blockers are fixed codes only.
type SmartAccountsReconciliationEvaluationRecord struct {
	ID                           string     `gorm:"column:id;type:uuid;primaryKey"`
	BatchID                      string     `gorm:"column:batch_id;type:uuid;not null"`
	SourceCompanyID              string     `gorm:"column:source_company_id;type:text;not null"`
	TenantID                     string     `gorm:"column:tenant_id;type:uuid;not null"`
	PackageID                    string     `gorm:"column:package_id;type:text;not null"`
	ManifestSHA256               string     `gorm:"column:manifest_sha256;size:64;not null"`
	RecordsSHA256                string     `gorm:"column:records_sha256;size:64;not null"`
	ScopeSHA256                  string     `gorm:"column:scope_sha256;size:64;not null"`
	SourceAsOfDate               *time.Time `gorm:"column:source_as_of_date;type:date"`
	CutoffAt                     *time.Time `gorm:"column:cutoff_at"`
	GLPreviewID                  *string    `gorm:"column:gl_preview_id;type:uuid"`
	GLPreviewSHA256              string     `gorm:"column:gl_preview_sha256;size:64;not null"`
	GLState                      string     `gorm:"column:gl_state;type:text;not null"`
	GLMappingSnapshotSHA256      string     `gorm:"column:gl_mapping_snapshot_sha256;size:64;not null"`
	GLAppliedIdentitySHA256      string     `gorm:"column:gl_applied_identity_sha256;size:64;not null"`
	ReferencePreviewID           *string    `gorm:"column:reference_preview_id;type:uuid"`
	ReferencePreviewSHA256       *string    `gorm:"column:reference_preview_sha256;size:64"`
	ReferenceState               string     `gorm:"column:reference_state;type:text;not null"`
	ProofID                      string     `gorm:"column:proof_id;type:text;not null"`
	ProofSHA256                  string     `gorm:"column:proof_sha256;size:64;not null"`
	ClaimSHA256                  string     `gorm:"column:claim_sha256;size:64;not null"`
	CoverageSHA256               string     `gorm:"column:coverage_sha256;size:64;not null"`
	ClaimKind                    string     `gorm:"column:claim_kind;type:text;not null"`
	ExpectedCoverageState        string     `gorm:"column:expected_coverage_state;type:text;not null"`
	ToleranceSHA256              string     `gorm:"column:tolerance_sha256;size:64;not null"`
	VarianceWithinPolicy         bool       `gorm:"column:variance_within_policy;not null"`
	GLRevisionUnresolved         int        `gorm:"column:gl_revision_unresolved;not null"`
	GLTombstoneUnresolved        int        `gorm:"column:gl_tombstone_unresolved;not null"`
	ReferenceRevisionUnresolved  int        `gorm:"column:reference_revision_unresolved;not null"`
	ReferenceTombstoneUnresolved int        `gorm:"column:reference_tombstone_unresolved;not null"`
	Blockers                     []byte     `gorm:"column:blockers;type:jsonb;not null"`
	EvidenceSHA256               string     `gorm:"column:evidence_sha256;size:64;not null"`
	EvidenceSubmittedBy          string     `gorm:"column:evidence_submitted_by;type:text;not null"`
	GLFirstAppliedBy             string     `gorm:"column:gl_first_applied_by;type:text;not null"`
	GLExactReplayBy              string     `gorm:"column:gl_exact_replay_by;type:text;not null"`
	Status                       string     `gorm:"column:status;type:text;not null"`
	AccountantApprovedAt         *time.Time `gorm:"column:accountant_approved_at"`
	CreatedAt                    time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt                    time.Time  `gorm:"column:updated_at;not null"`
}

func (SmartAccountsReconciliationEvaluationRecord) TableName() string {
	return "smartaccounts_reconciliation_evaluations"
}

type SmartAccountsReconciliationApprovalRecord struct {
	ID              string    `gorm:"column:id;type:uuid;primaryKey"`
	EvaluationID    string    `gorm:"column:evaluation_id;type:uuid;not null"`
	EvidenceSHA256  string    `gorm:"column:evidence_sha256;size:64;not null"`
	ToleranceSHA256 string    `gorm:"column:tolerance_sha256;size:64;not null"`
	ApprovedBy      string    `gorm:"column:approved_by;type:text;not null"`
	ApprovedAt      time.Time `gorm:"column:approved_at;not null"`
}

func (SmartAccountsReconciliationApprovalRecord) TableName() string {
	return "smartaccounts_reconciliation_approvals"
}

// SmartAccountsFullClaimDomainEvidenceRecord is an immutable public-control
// receipt for one selected full-claim domain. It retains only tenant/source /
// package/scope bindings, reviewed route metadata, six boolean proof gates,
// and an opaque evidence digest. It deliberately excludes source rows,
// financial values, object names, request/response bodies, credentials,
// cookies, URLs, and free-form notes.
type SmartAccountsFullClaimDomainEvidenceRecord struct {
	ID                           string    `gorm:"column:id;type:uuid;primaryKey"`
	BatchID                      string    `gorm:"column:batch_id;type:uuid;not null"`
	TenantID                     string    `gorm:"column:tenant_id;type:uuid;not null"`
	SourceCompanyID              string    `gorm:"column:source_company_id;type:text;not null"`
	PackageID                    string    `gorm:"column:package_id;type:text;not null"`
	ScopeSHA256                  string    `gorm:"column:scope_sha256;size:64;not null"`
	ReconciliationEvidenceSHA256 string    `gorm:"column:reconciliation_evidence_sha256;size:64;not null"`
	PlanVersion                  string    `gorm:"column:plan_version;type:text;not null"`
	DomainID                     string    `gorm:"column:domain_id;type:text;not null"`
	Source                       string    `gorm:"column:source;type:text;not null"`
	ResourceID                   string    `gorm:"column:resource_id;type:text;not null"`
	ContractVersion              string    `gorm:"column:contract_version;type:text;not null"`
	LiveSourceValidated          bool      `gorm:"column:live_source_validated;not null"`
	SchemaValidated              bool      `gorm:"column:schema_validated;not null"`
	CompletenessValidated        bool      `gorm:"column:completeness_validated;not null"`
	ReconciliationValidated      bool      `gorm:"column:reconciliation_validated;not null"`
	TombstonesResolved           bool      `gorm:"column:tombstones_resolved;not null"`
	AccountantAttested           bool      `gorm:"column:accountant_attested;not null"`
	EvidenceSHA256               string    `gorm:"column:evidence_sha256;size:64;not null"`
	RecordedAt                   time.Time `gorm:"column:recorded_at;not null"`
}

func (SmartAccountsFullClaimDomainEvidenceRecord) TableName() string {
	return "smartaccounts_full_claim_domain_evidence"
}
