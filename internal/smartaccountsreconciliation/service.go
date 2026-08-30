package smartaccountsreconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/google/uuid"
)

var (
	ErrInvalid            = errors.New("SmartAccounts reconciliation input is invalid")
	ErrNotFound           = errors.New("SmartAccounts reconciliation evaluation was not found")
	ErrConflict           = errors.New("SmartAccounts reconciliation evidence conflicts with immutable state")
	ErrNotReady           = errors.New("SmartAccounts reconciliation is not ready for accountant approval")
	ErrActorSeparation    = errors.New("SmartAccounts reconciliation accountant must be independent from evidence and GL apply actors")
	ErrAccountantRequired = errors.New("SmartAccounts reconciliation requires an accountant role")
)

// EvidenceResolver is an API/server-only adapter around the existing immutable
// batch workflow, archive, executor and reference stores. It must never
// surface source rows or raw proofs to this package.
type EvidenceResolver interface {
	Resolve(context.Context, string, string, string) (EvaluationInput, error)
	ListBindings(context.Context, string, string) ([]SourceBinding, error)
}

type Store interface {
	SaveEvaluation(context.Context, Evaluation) (*Evaluation, bool, error)
	GetLatestEvaluation(context.Context, string, string, string) (*Evaluation, error)
	GetEvaluation(context.Context, string) (*Evaluation, error)
	CreateApproval(context.Context, Approval) (*Approval, bool, error)
	GetApproval(context.Context, string) (*Approval, error)
	MarkEvaluationPassed(context.Context, string, string, string, time.Time) (*Evaluation, error)
	FullClaimEvidenceStore
}

// Service creates an immutable technical evidence snapshot from existing OA
// state and a digest-only owner attestation. It does not write accounting.
type Service struct {
	store         Store
	resolver      EvidenceResolver
	now           func() time.Time
	newID         func() string
	fullClaimPlan func() []smartaccountssync.FullClaimDomainPlanEntry
}

func NewService(store Store, resolver EvidenceResolver) *Service {
	return &Service{store: store, resolver: resolver, now: time.Now, newID: uuid.NewString, fullClaimPlan: smartaccountssync.CurrentFullClaimDomainPlan}
}

func (s *Service) Evaluate(ctx context.Context, ownerID, batchID, sourceID string) (*Evaluation, bool, error) {
	if s == nil || s.store == nil || s.resolver == nil || !safeActor(ownerID) || !safeID(batchID) || !safeSource(sourceID) {
		return nil, false, ErrInvalid
	}
	input, err := s.resolver.Resolve(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), strings.TrimSpace(sourceID))
	if err != nil || input.BatchID != strings.TrimSpace(batchID) || input.SourceCompanyID != strings.TrimSpace(sourceID) || !validInputBinding(input) {
		return nil, false, ErrInvalid
	}
	evaluation, err := newEvaluation(s.newID(), input, strings.TrimSpace(ownerID), s.currentTime())
	if err != nil {
		return nil, false, err
	}
	return s.store.SaveEvaluation(ctx, *evaluation)
}

func (s *Service) Get(ctx context.Context, tenantID, batchID, sourceID string) (*Evaluation, error) {
	if s == nil || s.store == nil || !safeID(tenantID) || !safeID(batchID) || !safeSource(sourceID) {
		return nil, ErrNotFound
	}
	return s.store.GetLatestEvaluation(ctx, strings.TrimSpace(batchID), strings.TrimSpace(sourceID), strings.TrimSpace(tenantID))
}

// GetForOwner resolves the selected source's tenant from the immutable batch
// manifest before reading a tenant-scoped evaluation. It prevents callers from
// choosing a different tenant in an owner-level batch status route.
func (s *Service) GetForOwner(ctx context.Context, ownerID, batchID, sourceID string) (*Evaluation, error) {
	if s == nil || s.resolver == nil || !safeActor(ownerID) || !safeID(batchID) || !safeSource(sourceID) {
		return nil, ErrNotFound
	}
	bindings, err := s.resolver.ListBindings(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if binding.SourceCompanyID == strings.TrimSpace(sourceID) && safeID(binding.TenantID) {
			evaluation, getErr := s.Get(ctx, binding.TenantID, batchID, sourceID)
			if getErr != nil {
				return nil, getErr
			}
			current, resolveErr := s.resolver.Resolve(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), strings.TrimSpace(sourceID))
			if resolveErr != nil || !evaluationMatchesCurrent(evaluation, current) {
				return staleEvaluation(evaluation, current), nil
			}
			return evaluation, nil
		}
	}
	return nil, ErrNotFound
}

// GetForTenant is the accountant-safe counterpart to GetForOwner. The route
// caller has already been constrained to the tenant and accountant role; this
// method additionally requires the stored evaluation to bind that exact
// tenant, batch, and opaque source. It then rebuilds current server-side
// evidence using the original evidence submitter only as an internal resolver
// credential. No owner identity or source payload is returned. A changed or
// unavailable current generation is represented as a safe stale evaluation,
// never as an old READY/PASS attestation.
func (s *Service) GetForTenant(ctx context.Context, tenantID, batchID, sourceID string) (*Evaluation, error) {
	if s == nil || s.store == nil || s.resolver == nil || !safeID(tenantID) || !safeID(batchID) || !safeSource(sourceID) {
		return nil, ErrNotFound
	}
	evaluation, err := s.Get(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(batchID), strings.TrimSpace(sourceID))
	if err != nil || evaluation == nil || evaluation.TenantID != strings.TrimSpace(tenantID) || evaluation.BatchID != strings.TrimSpace(batchID) || evaluation.SourceCompanyID != strings.TrimSpace(sourceID) || !safeActor(evaluation.EvidenceSubmittedBy) {
		return nil, ErrNotFound
	}
	current, resolveErr := s.resolver.Resolve(ctx, evaluation.EvidenceSubmittedBy, evaluation.BatchID, evaluation.SourceCompanyID)
	if resolveErr != nil || !evaluationMatchesCurrent(evaluation, current) {
		return staleEvaluation(evaluation, current), nil
	}
	return evaluation, nil
}

// Approve is intentionally role-aware at the final service boundary. The HTTP
// route must also require current tenant membership before calling it.
func (s *Service) Approve(ctx context.Context, actorID, role, tenantID, evaluationID string, request ApprovalRequest) (*Evaluation, bool, error) {
	if s == nil || s.store == nil || s.resolver == nil || !safeActor(actorID) || !safeID(tenantID) || !safeID(evaluationID) || strings.TrimSpace(role) != "accountant" {
		if strings.TrimSpace(role) != "accountant" {
			return nil, false, ErrAccountantRequired
		}
		return nil, false, ErrInvalid
	}
	if !request.Confirmed || !safeDigest(request.EvidenceSHA256) || !safeDigest(request.ToleranceSHA256) {
		return nil, false, ErrNotReady
	}
	evaluation, err := s.store.GetEvaluation(ctx, strings.TrimSpace(evaluationID))
	if err != nil || evaluation == nil {
		return nil, false, ErrNotFound
	}
	if evaluation.TenantID != strings.TrimSpace(tenantID) {
		return nil, false, ErrNotFound
	}
	current, resolveErr := s.resolver.Resolve(ctx, evaluation.EvidenceSubmittedBy, evaluation.BatchID, evaluation.SourceCompanyID)
	if resolveErr != nil || !evaluationMatchesCurrent(evaluation, current) {
		return nil, false, ErrNotReady
	}
	if evaluation.EvidenceSHA256 != request.EvidenceSHA256 || evaluation.ToleranceSHA256 != request.ToleranceSHA256 {
		return nil, false, ErrConflict
	}
	if evaluation.Status == StatusPass {
		approval, approvalErr := s.store.GetApproval(ctx, evaluation.ID)
		if approvalErr == nil && approval != nil && approval.EvidenceSHA256 == request.EvidenceSHA256 && approval.ToleranceSHA256 == request.ToleranceSHA256 && approval.ApprovedBy == strings.TrimSpace(actorID) {
			return evaluation, true, nil
		}
		return nil, false, ErrConflict
	}
	if evaluation.Status != StatusReadyForAccountant {
		return evaluation, false, ErrNotReady
	}
	if !independentActor(strings.TrimSpace(actorID), evaluation) {
		return evaluation, false, ErrActorSeparation
	}
	now := s.currentTime()
	approval, created, err := s.store.CreateApproval(ctx, Approval{ID: s.newID(), EvaluationID: evaluation.ID, EvidenceSHA256: request.EvidenceSHA256, ToleranceSHA256: request.ToleranceSHA256, ApprovedBy: strings.TrimSpace(actorID), ApprovedAt: now})
	if err != nil || approval == nil {
		if errors.Is(err, ErrConflict) {
			return nil, false, ErrConflict
		}
		return nil, false, err
	}
	if !created {
		if approval.ApprovedBy != strings.TrimSpace(actorID) || approval.EvidenceSHA256 != request.EvidenceSHA256 || approval.ToleranceSHA256 != request.ToleranceSHA256 {
			return nil, false, ErrConflict
		}
	}
	updated, err := s.store.MarkEvaluationPassed(ctx, evaluation.ID, request.EvidenceSHA256, request.ToleranceSHA256, approval.ApprovedAt)
	if err != nil {
		return nil, false, err
	}
	return updated, !created, nil
}

func (s *Service) Rollup(ctx context.Context, ownerID, batchID string) (*Rollup, error) {
	if s == nil || s.store == nil || s.resolver == nil || !safeActor(ownerID) || !safeID(batchID) {
		return nil, ErrInvalid
	}
	bindings, err := s.resolver.ListBindings(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil || len(bindings) == 0 {
		return nil, ErrNotFound
	}
	rollup := &Rollup{BatchID: strings.TrimSpace(batchID), Status: RollupInProgress, SelectedCount: len(bindings)}
	for _, binding := range bindings {
		if binding.BatchID != rollup.BatchID || !safeSource(binding.SourceCompanyID) || !safeID(binding.TenantID) || !binding.Paired {
			rollup.PendingCount++
			continue
		}
		evaluation, getErr := s.GetForOwner(ctx, ownerID, rollup.BatchID, binding.SourceCompanyID)
		if getErr != nil || evaluation == nil || evaluation.TenantID != binding.TenantID {
			rollup.PendingCount++
			continue
		}
		switch evaluation.Status {
		case StatusPass:
			rollup.PassCount++
		case StatusReadyForAccountant:
			rollup.ReviewCount++
		case StatusPartialFailure:
			rollup.FailureCount++
		default:
			rollup.PendingCount++
		}
	}
	switch {
	case rollup.FailureCount > 0 && rollup.PendingCount == 0 && rollup.ReviewCount == 0:
		rollup.Status = RollupPartialFailure
	case rollup.PassCount == rollup.SelectedCount:
		rollup.Status = RollupPass
	case rollup.ReviewCount > 0 && rollup.PendingCount == 0:
		rollup.Status = RollupAccountantReviewRequired
	default:
		rollup.Status = RollupInProgress
	}
	return rollup, nil
}

func newEvaluation(id string, input EvaluationInput, actor string, now time.Time) (*Evaluation, error) {
	if !safeID(id) || !validInputBinding(input) || !safeActor(actor) || !validInputMetadata(input) {
		return nil, ErrInvalid
	}
	e := &Evaluation{
		ID: id, BatchID: input.BatchID, SourceCompanyID: input.SourceCompanyID, TenantID: input.TenantID,
		PackageID: input.PackageID, ManifestSHA256: input.ManifestSHA256, RecordsSHA256: input.RecordsSHA256, ScopeSHA256: input.ScopeSHA256,
		SourceAsOfDate: input.SourceAsOfDate, CutoffAt: input.CutoffAt,
		GLPreviewID: input.GLPreviewID, GLPreviewSHA256: input.GLPreviewSHA256,
		GLMappingSnapshotSHA256: input.GLMappingSnapshotSHA256, GLAppliedIdentitySHA256: input.GLAppliedIdentitySHA256,
		ReferencePreviewID: input.ReferencePreviewID, ReferencePreviewSHA256: input.ReferencePreviewSHA256,
		ProofID: input.ProofID, ProofSHA256: input.ProofSHA256, ClaimSHA256: input.ClaimSHA256, CoverageSHA256: input.CoverageSHA256, ClaimKind: input.ClaimKind, ExpectedCoverageState: input.ExpectedCoverageState, ToleranceSHA256: input.ToleranceSHA256, VarianceWithinPolicy: input.VarianceWithinPolicy,
		GLRevisionUnresolved: input.GLRevisionUnresolved, GLTombstoneUnresolved: input.GLTombstoneUnresolved, ReferenceRevisionUnresolved: input.ReferenceRevisionUnresolved, ReferenceTombstoneUnresolved: input.ReferenceTombstoneUnresolved,
		EvidenceSubmittedBy: actor, CreatedAt: now.UTC(), UpdatedAt: now.UTC(), Status: StatusNotEvaluated,
	}
	if input.GLApplyReceipt != nil {
		e.GLFirstAppliedBy = input.GLApplyReceipt.FirstAppliedBy
		e.GLExactReplayBy = input.GLApplyReceipt.ExactReplayBy
		if e.GLMappingSnapshotSHA256 == "" {
			e.GLMappingSnapshotSHA256 = input.GLApplyReceipt.MappingSnapshotSHA256
		}
		if e.GLAppliedIdentitySHA256 == "" {
			e.GLAppliedIdentitySHA256 = input.GLApplyReceipt.AppliedIdentitySHA256
		}
	}
	setEvaluationState(e, input)
	digest, err := evidenceDigest(*e)
	if err != nil {
		return nil, ErrInvalid
	}
	e.EvidenceSHA256 = digest
	return e, nil
}

func setEvaluationState(e *Evaluation, input EvaluationInput) {
	blockers := make([]string, 0, 8)
	if !input.BindingComplete {
		blockers = append(blockers, "source_binding_incomplete")
	}
	if !validPackageEvidence(input) {
		blockers = append(blockers, "staged_package_evidence_missing")
	}
	if input.GLPreviewApplied && exactReceiptMatchesInput(input) && input.GLApplyReceipt.ExactReplayAt != nil && safeDigest(e.GLMappingSnapshotSHA256) && safeDigest(e.GLAppliedIdentitySHA256) && input.GLApplyReceipt.MappingCount > 0 && input.GLApplyReceipt.AppliedIdentityCount > 0 {
		e.GLState = GLStateAppliedReplayVerified
	} else if input.GLPreviewApplied {
		e.GLState = GLStateApplied
		blockers = append(blockers, "gl_apply_replay_mapping_or_identity_receipt_missing")
	} else {
		e.GLState = GLStateEvidencePending
		blockers = append(blockers, "gl_apply_or_exact_replay_receipt_missing")
	}
	if input.ReferenceApplicable {
		if input.ReferencePreviewApplied {
			e.ReferenceState = ReferenceStateApplied
		} else {
			e.ReferenceState = ReferenceStateEvidencePending
			blockers = append(blockers, "applicable_reference_apply_missing")
		}
	} else {
		e.ReferenceState = ReferenceStateNotApplicable
	}
	if !completeProof(input) {
		blockers = append(blockers, "private_proof_claim_coverage_or_tolerance_missing")
	} else if input.ClaimKind != "full" || input.ExpectedCoverageState != "full" {
		blockers = append(blockers, "claim_or_coverage_not_full")
	}
	if completeProof(input) && !input.VarianceWithinPolicy {
		blockers = append(blockers, "variance_outside_policy")
	}
	if input.GLRevisionUnresolved > 0 {
		blockers = append(blockers, "unresolved_gl_revisions")
	}
	if input.GLTombstoneUnresolved > 0 {
		blockers = append(blockers, "unresolved_gl_tombstones")
	}
	if input.ReferenceRevisionUnresolved > 0 {
		blockers = append(blockers, "unresolved_reference_revisions")
	}
	if input.ReferenceTombstoneUnresolved > 0 {
		blockers = append(blockers, "unresolved_reference_tombstones")
	}
	sort.Strings(blockers)
	e.Blockers = blockers
	if len(blockers) == 0 {
		e.Status = StatusReadyForAccountant
		return
	}
	if !input.BindingComplete || !validPackageEvidence(input) || !completeProof(input) || input.ClaimKind != "full" || input.ExpectedCoverageState != "full" || !input.GLPreviewApplied || !exactReceiptMatchesInput(input) || input.GLApplyReceipt == nil || input.GLApplyReceipt.ExactReplayAt == nil || (input.ReferenceApplicable && !input.ReferencePreviewApplied) {
		e.Status = StatusEvidencePending
		return
	}
	e.Status = StatusPartialFailure
}

func evidenceDigest(e Evaluation) (string, error) {
	// Keep the canonical payload intentionally closed and free of actor IDs,
	// timestamps, proof bytes, monetary amounts, source rows, and notes.
	v := struct {
		Version       string   `json:"version"`
		Batch         string   `json:"batch"`
		Source        string   `json:"source"`
		Tenant        string   `json:"tenant"`
		Package       string   `json:"package"`
		Manifest      string   `json:"manifest"`
		Records       string   `json:"records"`
		Scope         string   `json:"scope"`
		AsOf          string   `json:"as_of"`
		Cutoff        string   `json:"cutoff"`
		GLPreview     string   `json:"gl_preview"`
		GLPreviewSHA  string   `json:"gl_preview_sha"`
		GLState       string   `json:"gl_state"`
		Mapping       string   `json:"mapping"`
		Identities    string   `json:"identities"`
		RefPreview    string   `json:"ref_preview"`
		RefPreviewSHA string   `json:"ref_preview_sha"`
		RefState      string   `json:"ref_state"`
		ProofID       string   `json:"proof_id"`
		Proof         string   `json:"proof"`
		Claim         string   `json:"claim"`
		Coverage      string   `json:"coverage"`
		ClaimKind     string   `json:"claim_kind"`
		ExpectedCover string   `json:"expected_coverage_state"`
		Tolerance     string   `json:"tolerance"`
		Variance      bool     `json:"variance"`
		GLRevision    int      `json:"gl_revision"`
		GLTombstone   int      `json:"gl_tombstone"`
		RefRevision   int      `json:"ref_revision"`
		RefTombstone  int      `json:"ref_tombstone"`
		Blockers      []string `json:"blockers"`
	}{"smartaccounts-reconciliation-evidence-v1", e.BatchID, e.SourceCompanyID, e.TenantID, e.PackageID, e.ManifestSHA256, e.RecordsSHA256, e.ScopeSHA256, e.SourceAsOfDate, e.CutoffAt, e.GLPreviewID, e.GLPreviewSHA256, e.GLState, e.GLMappingSnapshotSHA256, e.GLAppliedIdentitySHA256, e.ReferencePreviewID, e.ReferencePreviewSHA256, e.ReferenceState, e.ProofID, e.ProofSHA256, e.ClaimSHA256, e.CoverageSHA256, e.ClaimKind, e.ExpectedCoverageState, e.ToleranceSHA256, e.VarianceWithinPolicy, e.GLRevisionUnresolved, e.GLTombstoneUnresolved, e.ReferenceRevisionUnresolved, e.ReferenceTombstoneUnresolved, e.Blockers}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func validInputBinding(input EvaluationInput) bool {
	return safeID(input.BatchID) && safeSource(input.SourceCompanyID) && safeID(input.TenantID)
}
func validInputMetadata(input EvaluationInput) bool {
	return input.GLRevisionUnresolved >= 0 && input.GLTombstoneUnresolved >= 0 && input.ReferenceRevisionUnresolved >= 0 && input.ReferenceTombstoneUnresolved >= 0 && validCoverageState(input.ClaimKind, input.ExpectedCoverageState)
}
func validPackageEvidence(input EvaluationInput) bool {
	return safeID(input.PackageID) && safeDigest(input.ManifestSHA256) && safeDigest(input.RecordsSHA256) && safeDigest(input.ScopeSHA256) && validRFC3339(input.CutoffAt)
}
func completeProof(input EvaluationInput) bool {
	return safeID(input.ProofID) && safeDigest(input.ProofSHA256) && safeDigest(input.ClaimSHA256) && safeDigest(input.CoverageSHA256) && safeDigest(input.ToleranceSHA256) && exactReceiptMatchesInput(input) && input.GLApplyReceipt.TolerancePolicySHA256 == input.ToleranceSHA256
}

func exactReceiptMatchesInput(input EvaluationInput) bool {
	r := input.GLApplyReceipt
	return r != nil && safeUUID(input.GLPreviewID) && safeDigest(input.GLPreviewSHA256) && r.TenantID == input.TenantID && r.SourceCompanyID == input.SourceCompanyID && r.PackageID == input.PackageID && r.PreviewID == input.GLPreviewID && r.PreviewSHA256 == input.GLPreviewSHA256 && safeDigest(r.MappingSnapshotSHA256) && safeDigest(r.AppliedIdentitySHA256) && safeDigest(r.TolerancePolicySHA256) && safeActor(r.FirstAppliedBy)
}

func validCoverageState(claimKind, expected string) bool {
	return (claimKind == "" || claimKind == "full" || claimKind == "partial") && (expected == "" || expected == "full" || expected == "partial" || expected == "unknown")
}

var allowedBlockers = map[string]struct{}{
	"source_binding_incomplete": {}, "staged_package_evidence_missing": {}, "gl_apply_replay_mapping_or_identity_receipt_missing": {}, "gl_apply_or_exact_replay_receipt_missing": {}, "applicable_reference_apply_missing": {}, "private_proof_claim_coverage_or_tolerance_missing": {}, "claim_or_coverage_not_full": {}, "variance_outside_policy": {}, "unresolved_gl_revisions": {}, "unresolved_gl_tombstones": {}, "unresolved_reference_revisions": {}, "unresolved_reference_tombstones": {}, "evaluation_stale_current_generation": {},
}

func validBlockers(values []string) bool {
	if len(values) > len(allowedBlockers) {
		return false
	}
	previous := ""
	for _, value := range values {
		if _, ok := allowedBlockers[value]; !ok || value <= previous {
			return false
		}
		previous = value
	}
	return true
}

func safeUUID(v string) bool   { _, err := uuid.Parse(strings.TrimSpace(v)); return err == nil }
func safeDigest(v string) bool { return len(v) == 64 && strings.Trim(v, "0123456789abcdef") == "" }
func safeID(v string) bool {
	v = strings.TrimSpace(v)
	return len(v) >= 1 && len(v) <= 255 && !strings.ContainsAny(v, "\x00\r\n")
}
func safeSource(v string) bool {
	return len(v) >= len("sa-browser-v1-1") && len(v) <= 64 && strings.HasPrefix(v, "sa-browser-v1-") && strings.Trim(v[len("sa-browser-v1-"):], "0123456789") == ""
}
func safeActor(v string) bool {
	return len(strings.TrimSpace(v)) >= 1 && len(strings.TrimSpace(v)) <= 255
}
func validRFC3339(v string) bool { _, err := time.Parse(time.RFC3339, v); return err == nil }
func independentActor(actor string, e *Evaluation) bool {
	if e == nil || actor == e.EvidenceSubmittedBy {
		return false
	}
	if e.GLFirstAppliedBy == actor || e.GLExactReplayBy == actor {
		return false
	}
	return true
}

func evaluationMatchesCurrent(e *Evaluation, current EvaluationInput) bool {
	if e == nil || !current.BindingComplete || !validPackageEvidence(current) {
		return false
	}
	// Rebuild the complete closed evidence projection from live server-side
	// state and compare its canonical digest. Checking only the package and GL
	// preview would leave a PASS visible after a changed reference preview,
	// mapping/identity receipt, tolerance binding, proof/coverage, variance, or
	// unresolved revision count. The digest intentionally excludes actor IDs
	// and timestamps, but includes every approval-relevant technical handle.
	currentEvaluation, err := newEvaluation(e.ID, current, e.EvidenceSubmittedBy, e.CreatedAt)
	if err != nil {
		return false
	}
	return e.EvidenceSHA256 == currentEvaluation.EvidenceSHA256
}

func staleEvaluation(previous *Evaluation, current EvaluationInput) *Evaluation {
	if previous == nil {
		return &Evaluation{Status: StatusNotEvaluated, Blockers: []string{"evaluation_stale_current_generation"}}
	}
	return &Evaluation{BatchID: previous.BatchID, SourceCompanyID: previous.SourceCompanyID, TenantID: previous.TenantID, PackageID: current.PackageID, ManifestSHA256: current.ManifestSHA256, RecordsSHA256: current.RecordsSHA256, ScopeSHA256: current.ScopeSHA256, Status: StatusNotEvaluated, GLState: GLStateEvidencePending, ReferenceState: ReferenceStateEvidencePending, Blockers: []string{"evaluation_stale_current_generation"}}
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
