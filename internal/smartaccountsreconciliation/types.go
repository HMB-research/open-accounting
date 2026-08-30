// Package smartaccountsreconciliation records only safe, digest-bound
// technical reconciliation evidence. It never receives source rows, raw
// proofs, financial amounts, browser credentials, or posting instructions.
package smartaccountsreconciliation

import (
	"context"
	"time"
)

const (
	// ExactMatchTolerancePolicyVersion is the only current policy algorithm.
	// It permits no variance: the streaming proof must show exact original- and
	// base-currency debit/credit equality for the server-derived preview.
	ExactMatchTolerancePolicyVersion = "smartaccounts-exact-match-v1"
	ExactMatchTolerancePolicyLabel   = "Exact match — zero variance"

	StatusNotEvaluated       = "NOT_EVALUATED"
	StatusEvidencePending    = "EVIDENCE_PENDING"
	StatusReadyForAccountant = "READY_FOR_ACCOUNTANT"
	StatusPass               = "PASS"
	StatusPartialFailure     = "PARTIAL_FAILURE"

	RollupInProgress               = "IN_PROGRESS"
	RollupAccountantReviewRequired = "ACCOUNTANT_REVIEW_REQUIRED"
	RollupPass                     = "PASS"
	RollupPartialFailure           = "PARTIAL_FAILURE"

	// FullClaimStatusEligible is intentionally distinct from a reconciliation
	// PASS. It is a read-only product-coverage check over the immutable batch
	// and the reviewed source-surface matrix; it never authorizes an apply.
	FullClaimStatusEligible    = "ELIGIBLE"
	FullClaimStatusNotEligible = "NOT_ELIGIBLE"

	GLStateApplied                = "APPLIED"
	GLStateAppliedReplayVerified  = "APPLIED_REPLAY_VERIFIED"
	GLStateEvidencePending        = "EVIDENCE_PENDING"
	ReferenceStateNotApplicable   = "NOT_APPLICABLE"
	ReferenceStateApplied         = "APPLIED"
	ReferenceStateEvidencePending = "EVIDENCE_PENDING"
)

// GLApplyReceipt is written by the executor only after an explicit financial
// apply succeeds. A later exact confirmed replay is separately recorded; an
// old applied preview without that receipt is deliberately not backfilled.
type GLApplyReceipt struct {
	TenantID              string     `json:"tenant_id"`
	SourceCompanyID       string     `json:"source_company_id"`
	PackageID             string     `json:"package_id"`
	PreviewID             string     `json:"preview_id"`
	PreviewSHA256         string     `json:"preview_sha256"`
	FirstAppliedAt        time.Time  `json:"first_applied_at"`
	FirstAppliedBy        string     `json:"-"`
	ExactReplayAt         *time.Time `json:"exact_replay_at,omitempty"`
	ExactReplayBy         string     `json:"-"`
	MappingSnapshotSHA256 string     `json:"mapping_snapshot_sha256"`
	AppliedIdentitySHA256 string     `json:"applied_identity_sha256"`
	TolerancePolicySHA256 string     `json:"tolerance_policy_sha256"`
	MappingCount          int        `json:"mapping_count"`
	AppliedIdentityCount  int        `json:"applied_identity_count"`
}

// TolerancePolicy is an immutable accountant approval of an opaque policy
// digest for one staged source/package/scope. It contains neither rules nor
// monetary amounts, so it is safe to use as an executor verifier result.
type TolerancePolicy struct {
	ID                    string    `json:"policy_id"`
	AlgorithmVersion      string    `json:"algorithm_version"`
	TenantID              string    `json:"tenant_id"`
	SourceCompanyID       string    `json:"source_company_id"`
	PackageID             string    `json:"package_id"`
	ScopeSHA256           string    `json:"scope_sha256"`
	PreviewSHA256         string    `json:"preview_sha256"`
	TolerancePolicySHA256 string    `json:"tolerance_policy_sha256"`
	ApprovedBy            string    `json:"-"`
	ApprovedAt            time.Time `json:"approved_at"`
}

type TolerancePolicyApprovalRequest struct {
	Confirmed               bool   `json:"confirmed"`
	PackageID               string `json:"package_id"`
	PreviewID               string `json:"preview_id"`
	ExpectedCandidateSHA256 string `json:"expected_candidate_sha256"`
}

// TolerancePolicyCandidateRequest selects existing server-side state only. It
// contains no rates, amounts, tolerance rules, or arbitrary policy digest.
type TolerancePolicyCandidateRequest struct {
	PackageID string `json:"package_id"`
	PreviewID string `json:"preview_id"`
}

// TolerancePolicyCandidate is a safe, conservative policy choice derived
// entirely from the staged package, persisted preview, scope, and currency
// set. Its digest is the only value the later approval and GL apply use.
type TolerancePolicyCandidate struct {
	AlgorithmVersion string `json:"algorithm_version"`
	Label            string `json:"label"`
	CandidateSHA256  string `json:"candidate_sha256"`
}

// ResolvedTolerancePolicy is the safe handoff from an accountant approval to
// a different interactive financial actor. It intentionally contains only an
// immutable policy ID, algorithm label, digest, and approval time; it never
// carries a rule, rate, amount, or source row.
type ResolvedTolerancePolicy struct {
	PolicyID              string    `json:"policy_id"`
	AlgorithmVersion      string    `json:"algorithm_version"`
	Label                 string    `json:"label"`
	TolerancePolicySHA256 string    `json:"tolerance_policy_sha256"`
	ApprovedAt            time.Time `json:"approved_at"`
}

// TolerancePolicyScopeResolver derives the immutable scope digest from the
// staged package server-side. It never accepts a browser-provided scope.
type TolerancePolicyScopeResolver interface {
	ResolveTolerancePolicyBinding(context.Context, string, string, string, string) (TolerancePolicyBinding, error)
}

// TolerancePolicyBinding is derived from staged manifest and persisted
// preview state rather than caller input. It closes policy reuse across a
// changed mapping/preview even inside one package and capture scope.
type TolerancePolicyBinding struct {
	ScopeSHA256       string
	PreviewSHA256     string
	CurrencySetSHA256 string
	PreviewStatus     string
}

// EvaluationInput is produced entirely by a server-side resolver. Its proof
// fields are the digest-only result of streaming archive/posted-journal
// reconciliation in memory; callers cannot submit source data or amounts.
type EvaluationInput struct {
	BatchID                      string
	SourceCompanyID              string
	TenantID                     string
	PackageID                    string
	ManifestSHA256               string
	RecordsSHA256                string
	ScopeSHA256                  string
	SourceAsOfDate               string
	CutoffAt                     string
	GLPreviewID                  string
	GLPreviewSHA256              string
	GLPreviewApplied             bool
	GLApplyReceipt               *GLApplyReceipt
	GLMappingSnapshotSHA256      string
	GLAppliedIdentitySHA256      string
	ProofID                      string
	ProofSHA256                  string
	ClaimSHA256                  string
	CoverageSHA256               string
	ClaimKind                    string
	ExpectedCoverageState        string
	ToleranceSHA256              string
	VarianceWithinPolicy         bool
	ReferenceApplicable          bool
	ReferencePreviewID           string
	ReferencePreviewSHA256       string
	ReferencePreviewApplied      bool
	GLRevisionUnresolved         int
	GLTombstoneUnresolved        int
	ReferenceRevisionUnresolved  int
	ReferenceTombstoneUnresolved int
	BindingComplete              bool
}

// Evaluation is immutable technical evidence except for its monotonic
// accountant status transition. It intentionally exposes neither proof nor
// financial amount values.
type Evaluation struct {
	ID                           string     `json:"evaluation_id"`
	BatchID                      string     `json:"batch_id"`
	SourceCompanyID              string     `json:"source_company_id"`
	TenantID                     string     `json:"tenant_id"`
	PackageID                    string     `json:"package_id,omitempty"`
	ManifestSHA256               string     `json:"manifest_sha256,omitempty"`
	RecordsSHA256                string     `json:"records_sha256,omitempty"`
	ScopeSHA256                  string     `json:"scope_sha256,omitempty"`
	SourceAsOfDate               string     `json:"source_as_of_date,omitempty"`
	CutoffAt                     string     `json:"cutoff_at,omitempty"`
	GLPreviewID                  string     `json:"gl_preview_id,omitempty"`
	GLPreviewSHA256              string     `json:"gl_preview_sha256,omitempty"`
	GLState                      string     `json:"gl_state"`
	GLMappingSnapshotSHA256      string     `json:"gl_mapping_snapshot_sha256,omitempty"`
	GLAppliedIdentitySHA256      string     `json:"gl_applied_identity_sha256,omitempty"`
	ReferencePreviewID           string     `json:"reference_preview_id,omitempty"`
	ReferencePreviewSHA256       string     `json:"reference_preview_sha256,omitempty"`
	ReferenceState               string     `json:"reference_state"`
	ProofID                      string     `json:"proof_id,omitempty"`
	ProofSHA256                  string     `json:"proof_sha256,omitempty"`
	ClaimSHA256                  string     `json:"claim_sha256,omitempty"`
	CoverageSHA256               string     `json:"coverage_sha256,omitempty"`
	ClaimKind                    string     `json:"claim_kind,omitempty"`
	ExpectedCoverageState        string     `json:"expected_coverage_state,omitempty"`
	ToleranceSHA256              string     `json:"tolerance_sha256,omitempty"`
	VarianceWithinPolicy         bool       `json:"variance_within_policy"`
	GLRevisionUnresolved         int        `json:"gl_revision_unresolved"`
	GLTombstoneUnresolved        int        `json:"gl_tombstone_unresolved"`
	ReferenceRevisionUnresolved  int        `json:"reference_revision_unresolved"`
	ReferenceTombstoneUnresolved int        `json:"reference_tombstone_unresolved"`
	Blockers                     []string   `json:"blockers,omitempty"`
	EvidenceSHA256               string     `json:"evidence_sha256,omitempty"`
	EvidenceSubmittedBy          string     `json:"-"`
	GLFirstAppliedBy             string     `json:"-"`
	GLExactReplayBy              string     `json:"-"`
	Status                       string     `json:"status"`
	CreatedAt                    time.Time  `json:"created_at"`
	UpdatedAt                    time.Time  `json:"updated_at"`
	AccountantApprovedAt         *time.Time `json:"accountant_approved_at,omitempty"`
}

type ApprovalRequest struct {
	Confirmed       bool   `json:"confirmed"`
	EvidenceSHA256  string `json:"evidence_sha256"`
	ToleranceSHA256 string `json:"tolerance_sha256"`
}

type Approval struct {
	ID              string    `json:"approval_id"`
	EvaluationID    string    `json:"evaluation_id"`
	EvidenceSHA256  string    `json:"evidence_sha256"`
	ToleranceSHA256 string    `json:"tolerance_sha256"`
	ApprovedBy      string    `json:"-"`
	ApprovedAt      time.Time `json:"approved_at"`
}

type SourceBinding struct {
	BatchID         string
	SourceCompanyID string
	TenantID        string
	Paired          bool
}

// FullClaimEvidenceBinding is the immutable boundary for one source's
// per-domain coverage evidence. It deliberately carries only opaque IDs and a
// scope digest: the evidence ledger must never be reusable across tenants,
// source companies, staged packages, or capture scopes.
type FullClaimEvidenceBinding struct {
	BatchID         string
	TenantID        string
	SourceCompanyID string
	PackageID       string
	ScopeSHA256     string
	// ReconciliationEvidenceSHA256 binds the route receipt to the exact
	// independently approved reconciliation generation, not just to a package
	// that could later be re-evaluated with different proof state.
	ReconciliationEvidenceSHA256 string
}

// FullClaimDomainEvidence is an append-only, digest-only attestation that all
// six required proof dimensions were independently verified for one selected
// route. It contains no source records, values, names, URLs, credentials,
// cookies, schemas, notes, or free-form operator input.
//
// This type is server-internal. There is intentionally no HTTP request or
// response model for it: a browser or API caller cannot submit a "full" claim
// by asserting these flags.
type FullClaimDomainEvidence struct {
	ID string
	FullClaimEvidenceBinding
	PlanVersion             string
	DomainID                string
	Source                  string
	ResourceID              string
	ContractVersion         string
	LiveSourceValidated     bool
	SchemaValidated         bool
	CompletenessValidated   bool
	ReconciliationValidated bool
	TombstonesResolved      bool
	AccountantAttested      bool
	EvidenceSHA256          string
	RecordedAt              time.Time
}

// StreamingProofComputer is the future zero-file reconciliation seam. An
// implementation streams the staged archive and exact posted journal IDs,
// compares original- and base-currency debit/credit in memory, then returns
// only these safe handles. It must never persist its input amounts or rows.
type StreamingProofComputer interface {
	ComputeProof(context.Context, ProofMaterial) (ComputedProof, error)
}

type ProofMaterial struct {
	SchemaName            string
	TenantID              string
	SourceCompanyID       string
	PackageID             string
	ManifestSHA256        string
	RecordsSHA256         string
	ScopeSHA256           string
	MappingSnapshotSHA256 string
	AppliedIdentitySHA256 string
	ToleranceSHA256       string
	PreviewID             string
	PreviewSHA256         string
	ExpectedCoverageState string
}

type ComputedProof struct {
	ProofID               string
	ProofSHA256           string
	ClaimSHA256           string
	CoverageSHA256        string
	ClaimKind             string
	ExpectedCoverageState string
	ToleranceSHA256       string
	VarianceWithinPolicy  bool
}

type Rollup struct {
	BatchID       string `json:"batch_id"`
	Status        string `json:"status"`
	SelectedCount int    `json:"selected_count"`
	PassCount     int    `json:"pass_count"`
	PendingCount  int    `json:"pending_count"`
	ReviewCount   int    `json:"review_count"`
	FailureCount  int    `json:"failure_count"`
}

// FullClaimStatus is a count-only, owner-safe eligibility result for an
// immutable selected/all batch. It deliberately excludes source identities,
// names, packages, digests, proof material, amounts, and capabilities.
//
// ELIGIBLE is possible only when every originally selected source has a
// current reconciliation PASS and every domain in the immutable selected
// source plan has no unresolved route/evidence gap. Alternative browser/API
// pathways remain auditable but do not duplicate the selected requirement. It
// is neither a financial action nor an accountant approval.
type FullClaimStatus struct {
	Status                       string   `json:"status"`
	FullClaimEligible            bool     `json:"full_claim_eligible"`
	SelectedCount                int      `json:"selected_count"`
	CurrentPassCount             int      `json:"current_pass_count"`
	CurrentPassGapCount          int      `json:"current_pass_gap_count"`
	TombstoneGapSourceCount      int      `json:"tombstone_gap_source_count"`
	SourceCoverageGapCount       int      `json:"source_coverage_gap_count"`
	DomainEvidenceGapSourceCount int      `json:"domain_evidence_gap_source_count"`
	MatrixBlockerCount           int      `json:"matrix_blocker_count"`
	MatrixFilterContractGapCount int      `json:"matrix_filter_contract_gap_count"`
	MatrixPageOnlyGapCount       int      `json:"matrix_page_only_gap_count"`
	MatrixReviewRequiredCount    int      `json:"matrix_review_required_count"`
	MatrixUnconsumedCount        int      `json:"matrix_unconsumed_count"`
	MatrixMissingEndpointCount   int      `json:"matrix_missing_endpoint_count"`
	MatrixSchemaGapCount         int      `json:"matrix_schema_gap_count"`
	MatrixCoverageGapCount       int      `json:"matrix_coverage_gap_count"`
	BlockingCodes                []string `json:"blocking_codes,omitempty"`
}
