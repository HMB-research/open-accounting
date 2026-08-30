package smartaccountsreconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// TolerancePolicyStore is deliberately smaller than Repository so the policy
// approval action can be tested without exposing receipt/evaluation writes.
type TolerancePolicyStore interface {
	SaveResolvedTolerancePolicy(context.Context, TolerancePolicy) (*TolerancePolicy, bool, error)
	GetResolvedTolerancePolicy(context.Context, TolerancePolicy) (*TolerancePolicy, error)
}

// TolerancePolicyService is the only policy-registration boundary used by an
// HTTP handler. It takes a package ID as a selection, then derives the scope
// from staged OA state. Request scope hashes are intentionally impossible.
type TolerancePolicyService struct {
	store    TolerancePolicyStore
	resolver TolerancePolicyScopeResolver
	now      func() time.Time
}

func NewTolerancePolicyService(store TolerancePolicyStore, resolver TolerancePolicyScopeResolver) *TolerancePolicyService {
	return &TolerancePolicyService{store: store, resolver: resolver, now: time.Now}
}

// Candidate derives the only supported conservative policy. It intentionally
// returns a version, plain-language label, and opaque digest only: no numeric
// thresholds, source values, or caller-controlled policy construction.
func (s *TolerancePolicyService) Candidate(ctx context.Context, role, tenantID, sourceID string, request TolerancePolicyCandidateRequest) (*TolerancePolicyCandidate, error) {
	if s == nil || s.resolver == nil || strings.TrimSpace(role) != "accountant" {
		if strings.TrimSpace(role) != "accountant" {
			return nil, ErrAccountantRequired
		}
		return nil, ErrInvalid
	}
	if !safeID(tenantID) || !safeSource(sourceID) || !safeID(request.PackageID) || !safeUUID(request.PreviewID) {
		return nil, ErrInvalid
	}
	binding, err := s.resolveBinding(ctx, tenantID, sourceID, request.PackageID, request.PreviewID)
	if err != nil {
		return nil, err
	}
	if binding.PreviewStatus != "PREVIEW_READY" {
		return nil, ErrNotReady
	}
	candidate, err := exactMatchCandidate(tenantID, sourceID, request.PackageID, binding)
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (s *TolerancePolicyService) Approve(ctx context.Context, actorID, role, tenantID, sourceID string, request TolerancePolicyApprovalRequest) (*TolerancePolicy, bool, error) {
	if s == nil || s.store == nil || s.resolver == nil || strings.TrimSpace(role) != "accountant" {
		if strings.TrimSpace(role) != "accountant" {
			return nil, false, ErrAccountantRequired
		}
		return nil, false, ErrInvalid
	}
	if !safeActor(actorID) || !safeID(tenantID) || !safeSource(sourceID) || !request.Confirmed || !safeID(request.PackageID) || !safeUUID(request.PreviewID) || !safeDigest(request.ExpectedCandidateSHA256) {
		return nil, false, ErrInvalid
	}
	binding, err := s.resolveBinding(ctx, tenantID, sourceID, request.PackageID, request.PreviewID)
	if err != nil {
		return nil, false, err
	}
	if binding.PreviewStatus != "PREVIEW_READY" {
		return nil, false, ErrNotReady
	}
	candidate, err := exactMatchCandidate(tenantID, sourceID, request.PackageID, binding)
	if err != nil {
		return nil, false, err
	}
	if candidate.CandidateSHA256 != strings.TrimSpace(request.ExpectedCandidateSHA256) {
		return nil, false, ErrConflict
	}
	return s.store.SaveResolvedTolerancePolicy(ctx, TolerancePolicy{AlgorithmVersion: candidate.AlgorithmVersion, TenantID: strings.TrimSpace(tenantID), SourceCompanyID: strings.TrimSpace(sourceID), PackageID: strings.TrimSpace(request.PackageID), ScopeSHA256: binding.ScopeSHA256, PreviewSHA256: binding.PreviewSHA256, TolerancePolicySHA256: candidate.CandidateSHA256, ApprovedBy: strings.TrimSpace(actorID), ApprovedAt: s.currentTime()})
}

// Resolve returns the current immutable accountant approval for the exact
// staged package and persisted preview. Owners and accountants can use this
// read-only action to hand the policy to a separate financial actor without
// copying a digest from a UI. An APPLIED preview remains resolvable solely for
// an exact replay; it cannot receive a new candidate or approval.
func (s *TolerancePolicyService) Resolve(ctx context.Context, tenantID, sourceID string, request TolerancePolicyCandidateRequest) (*ResolvedTolerancePolicy, error) {
	if s == nil || s.store == nil || s.resolver == nil || !safeID(tenantID) || !safeSource(sourceID) || !safeID(request.PackageID) || !safeUUID(request.PreviewID) {
		return nil, ErrInvalid
	}
	binding, err := s.resolveBinding(ctx, tenantID, sourceID, request.PackageID, request.PreviewID)
	if err != nil {
		return nil, err
	}
	if binding.PreviewStatus != "PREVIEW_READY" && binding.PreviewStatus != "APPLIED" {
		return nil, ErrNotReady
	}
	candidate, err := exactMatchCandidate(tenantID, sourceID, request.PackageID, binding)
	if err != nil {
		return nil, err
	}
	policy, err := s.store.GetResolvedTolerancePolicy(ctx, TolerancePolicy{AlgorithmVersion: candidate.AlgorithmVersion, TenantID: strings.TrimSpace(tenantID), SourceCompanyID: strings.TrimSpace(sourceID), PackageID: strings.TrimSpace(request.PackageID), ScopeSHA256: binding.ScopeSHA256, PreviewSHA256: binding.PreviewSHA256, TolerancePolicySHA256: candidate.CandidateSHA256})
	if err != nil || policy == nil || policy.ID == "" || policy.AlgorithmVersion != candidate.AlgorithmVersion || policy.TolerancePolicySHA256 != candidate.CandidateSHA256 || !safeActor(policy.ApprovedBy) || policy.ApprovedAt.IsZero() {
		return nil, ErrNotFound
	}
	return &ResolvedTolerancePolicy{PolicyID: policy.ID, AlgorithmVersion: candidate.AlgorithmVersion, Label: candidate.Label, TolerancePolicySHA256: policy.TolerancePolicySHA256, ApprovedAt: policy.ApprovedAt.UTC()}, nil
}

func (s *TolerancePolicyService) resolveBinding(ctx context.Context, tenantID, sourceID, packageID, previewID string) (TolerancePolicyBinding, error) {
	binding, err := s.resolver.ResolveTolerancePolicyBinding(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceID), strings.TrimSpace(packageID), strings.TrimSpace(previewID))
	if err != nil || !safeDigest(binding.ScopeSHA256) || !safeDigest(binding.PreviewSHA256) || !safeDigest(binding.CurrencySetSHA256) || (binding.PreviewStatus != "PREVIEW_READY" && binding.PreviewStatus != "APPLIED") {
		return TolerancePolicyBinding{}, ErrNotFound
	}
	return binding, nil
}

func exactMatchCandidate(tenantID, sourceID, packageID string, binding TolerancePolicyBinding) (TolerancePolicyCandidate, error) {
	if !safeID(tenantID) || !safeSource(sourceID) || !safeID(packageID) || !safeDigest(binding.ScopeSHA256) || !safeDigest(binding.PreviewSHA256) || !safeDigest(binding.CurrencySetSHA256) {
		return TolerancePolicyCandidate{}, ErrInvalid
	}
	// This closed payload binds the candidate to the tenant/source/package,
	// current preview/scope, and server-derived currency set. The algorithm
	// itself is exact match: zero variance in original and base currency.
	canonical, err := json.Marshal(struct {
		Version       string `json:"version"`
		TenantID      string `json:"tenant_id"`
		SourceID      string `json:"source_company_id"`
		PackageID     string `json:"package_id"`
		ScopeSHA256   string `json:"scope_sha256"`
		PreviewSHA256 string `json:"preview_sha256"`
		CurrencySet   string `json:"currency_set_sha256"`
	}{ExactMatchTolerancePolicyVersion, strings.TrimSpace(tenantID), strings.TrimSpace(sourceID), strings.TrimSpace(packageID), binding.ScopeSHA256, binding.PreviewSHA256, binding.CurrencySetSHA256})
	if err != nil {
		return TolerancePolicyCandidate{}, ErrInvalid
	}
	hash := sha256.Sum256(canonical)
	return TolerancePolicyCandidate{AlgorithmVersion: ExactMatchTolerancePolicyVersion, Label: ExactMatchTolerancePolicyLabel, CandidateSHA256: hex.EncodeToString(hash[:])}, nil
}

func (s *TolerancePolicyService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
