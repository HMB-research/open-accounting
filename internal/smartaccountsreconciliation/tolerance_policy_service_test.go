package smartaccountsreconciliation

import (
	"context"
	"errors"
	"testing"
)

type memoryPolicyStore struct{ value *TolerancePolicy }

func (s *memoryPolicyStore) SaveResolvedTolerancePolicy(_ context.Context, value TolerancePolicy) (*TolerancePolicy, bool, error) {
	if value.ID == "" {
		value.ID = "11111111-1111-1111-1111-111111111111"
	}
	copy := value
	s.value = &copy
	return &copy, true, nil
}

func (s *memoryPolicyStore) GetResolvedTolerancePolicy(_ context.Context, expected TolerancePolicy) (*TolerancePolicy, error) {
	if s.value == nil || s.value.AlgorithmVersion != expected.AlgorithmVersion || s.value.TenantID != expected.TenantID || s.value.SourceCompanyID != expected.SourceCompanyID || s.value.PackageID != expected.PackageID || s.value.ScopeSHA256 != expected.ScopeSHA256 || s.value.PreviewSHA256 != expected.PreviewSHA256 || s.value.TolerancePolicySHA256 != expected.TolerancePolicySHA256 {
		return nil, ErrNotFound
	}
	copy := *s.value
	return &copy, nil
}

type memoryScopeResolver struct {
	binding TolerancePolicyBinding
	err     error
	calls   [][4]string
}

func (r *memoryScopeResolver) ResolveTolerancePolicyBinding(_ context.Context, tenant, source, packageID, previewID string) (TolerancePolicyBinding, error) {
	r.calls = append(r.calls, [4]string{tenant, source, packageID, previewID})
	return r.binding, r.err
}

func TestTolerancePolicyApprovalDerivesScopeServerSide(t *testing.T) {
	store, resolver := &memoryPolicyStore{}, &memoryScopeResolver{binding: TolerancePolicyBinding{ScopeSHA256: digest("a"), PreviewSHA256: digest("c"), CurrencySetSHA256: digest("d"), PreviewStatus: "PREVIEW_READY"}}
	service := NewTolerancePolicyService(store, resolver)
	previewID := "11111111-1111-1111-1111-111111111111"
	candidate, err := service.Candidate(context.Background(), "accountant", testTenant, testSource, TolerancePolicyCandidateRequest{PackageID: "package-1", PreviewID: previewID})
	if err != nil || candidate.AlgorithmVersion != ExactMatchTolerancePolicyVersion || candidate.Label != ExactMatchTolerancePolicyLabel || !safeDigest(candidate.CandidateSHA256) || len(resolver.calls) != 1 {
		t.Fatalf("server-derived candidate = %#v / %v", candidate, err)
	}
	if _, _, err := service.Approve(context.Background(), "accountant-1", "accountant", testTenant, testSource, TolerancePolicyApprovalRequest{Confirmed: true, PackageID: "package-1", PreviewID: previewID, ExpectedCandidateSHA256: digest("b")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("arbitrary digest must not become a policy: %v", err)
	}
	policy, created, err := service.Approve(context.Background(), "accountant-1", "accountant", testTenant, testSource, TolerancePolicyApprovalRequest{Confirmed: true, PackageID: "package-1", PreviewID: previewID, ExpectedCandidateSHA256: candidate.CandidateSHA256})
	if err != nil || !created || policy.AlgorithmVersion != ExactMatchTolerancePolicyVersion || policy.ScopeSHA256 != digest("a") || policy.PreviewSHA256 != digest("c") || policy.TolerancePolicySHA256 != candidate.CandidateSHA256 || len(resolver.calls) != 3 {
		t.Fatalf("server-derived policy = %#v / %v", policy, err)
	}
	resolved, err := service.Resolve(context.Background(), testTenant, testSource, TolerancePolicyCandidateRequest{PackageID: "package-1", PreviewID: previewID})
	if err != nil || resolved.PolicyID != policy.ID || resolved.TolerancePolicySHA256 != candidate.CandidateSHA256 || resolved.Label != ExactMatchTolerancePolicyLabel {
		t.Fatalf("resolved approved policy = %#v / %v", resolved, err)
	}
	resolver.binding.CurrencySetSHA256 = digest("e")
	changedCurrency, err := service.Candidate(context.Background(), "accountant", testTenant, testSource, TolerancePolicyCandidateRequest{PackageID: "package-1", PreviewID: previewID})
	if err != nil || changedCurrency.CandidateSHA256 == candidate.CandidateSHA256 {
		t.Fatalf("server-derived currency set must alter candidate = %#v / %v", changedCurrency, err)
	}
	resolver.binding.CurrencySetSHA256 = digest("d")
	resolver.err = errors.New("other package")
	if _, _, err = service.Approve(context.Background(), "accountant-1", "accountant", testTenant, testSource, TolerancePolicyApprovalRequest{Confirmed: true, PackageID: "package-other", PreviewID: previewID, ExpectedCandidateSHA256: candidate.CandidateSHA256}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unresolved package policy = %v", err)
	}
	if _, _, err = service.Approve(context.Background(), "owner", "owner", testTenant, testSource, TolerancePolicyApprovalRequest{Confirmed: true, PackageID: "package-1", PreviewID: previewID, ExpectedCandidateSHA256: candidate.CandidateSHA256}); !errors.Is(err, ErrAccountantRequired) {
		t.Fatalf("non-accountant policy = %v", err)
	}
	resolver.err = nil
	resolver.binding.PreviewStatus = "APPLIED"
	if replayPolicy, replayErr := service.Resolve(context.Background(), testTenant, testSource, TolerancePolicyCandidateRequest{PackageID: "package-1", PreviewID: previewID}); replayErr != nil || replayPolicy.PolicyID != policy.ID {
		t.Fatalf("applied preview must resolve immutable replay policy = %#v / %v", replayPolicy, replayErr)
	}
	if _, _, err := service.Approve(context.Background(), "accountant-1", "accountant", testTenant, testSource, TolerancePolicyApprovalRequest{Confirmed: true, PackageID: "package-1", PreviewID: previewID, ExpectedCandidateSHA256: candidate.CandidateSHA256}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("applied preview cannot receive a new approval = %v", err)
	}
	resolver.binding.PreviewStatus = "PREVIEW_READY"
	resolver.binding.PreviewSHA256 = digest("d")
	if _, resolveErr := service.Resolve(context.Background(), testTenant, testSource, TolerancePolicyCandidateRequest{PackageID: "package-1", PreviewID: previewID}); !errors.Is(resolveErr, ErrNotFound) {
		t.Fatalf("changed preview must not resolve an old policy = %v", resolveErr)
	}
	if _, _, err := service.Approve(context.Background(), "accountant-1", "accountant", testTenant, testSource, TolerancePolicyApprovalRequest{Confirmed: true, PackageID: "package-1", PreviewID: previewID, ExpectedCandidateSHA256: candidate.CandidateSHA256}); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed preview must invalidate candidate = %v", err)
	}
}
