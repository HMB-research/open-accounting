package smartaccountsreconciliation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fullClaimMemoryResolver struct {
	inputs   map[string]EvaluationInput
	bindings []SourceBinding
}

func (r *fullClaimMemoryResolver) Resolve(_ context.Context, _ string, batchID, sourceID string) (EvaluationInput, error) {
	input, ok := r.inputs[sourceID]
	if !ok || input.BatchID != batchID {
		return EvaluationInput{}, ErrNotFound
	}
	return input, nil
}

func (r *fullClaimMemoryResolver) ListBindings(_ context.Context, _ string, batchID string) ([]SourceBinding, error) {
	if batchID != testBatch || len(r.bindings) == 0 {
		return nil, ErrNotFound
	}
	return append([]SourceBinding(nil), r.bindings...), nil
}

func fullClaimInput(sourceID, tenantID, packageID, previewID string) EvaluationInput {
	input := completeInput()
	input.SourceCompanyID = sourceID
	input.TenantID = tenantID
	input.PackageID = packageID
	input.GLPreviewID = previewID
	input.GLApplyReceipt = &GLApplyReceipt{
		TenantID: tenantID, SourceCompanyID: sourceID, PackageID: packageID,
		PreviewID: previewID, PreviewSHA256: input.GLPreviewSHA256,
		FirstAppliedAt: input.GLApplyReceipt.FirstAppliedAt, FirstAppliedBy: input.GLApplyReceipt.FirstAppliedBy,
		ExactReplayAt: input.GLApplyReceipt.ExactReplayAt, ExactReplayBy: input.GLApplyReceipt.ExactReplayBy,
		MappingSnapshotSHA256: input.GLApplyReceipt.MappingSnapshotSHA256,
		AppliedIdentitySHA256: input.GLApplyReceipt.AppliedIdentitySHA256,
		TolerancePolicySHA256: input.GLApplyReceipt.TolerancePolicySHA256,
		MappingCount:          input.GLApplyReceipt.MappingCount, AppliedIdentityCount: input.GLApplyReceipt.AppliedIdentityCount,
	}
	return input
}

func fullClaimService(inputs map[string]EvaluationInput, bindings []SourceBinding) (*Service, *memoryStore) {
	store := &memoryStore{}
	service := NewService(store, &fullClaimMemoryResolver{inputs: inputs, bindings: bindings})
	sequence := 0
	service.newID = func() string { sequence++; return "full-claim-evaluation-" + string(rune('0'+sequence)) }
	return service, store
}

func makeCurrentPass(t *testing.T, service *Service, sourceID, tenantID string) {
	t.Helper()
	evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, sourceID)
	if err != nil {
		t.Fatalf("evaluate %s: %v", sourceID, err)
	}
	passed, _, err := service.Approve(context.Background(), "independent-accountant", "accountant", tenantID, evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256})
	if err != nil || passed.Status != StatusPass {
		t.Fatalf("approve %s = %#v / %v", sourceID, passed, err)
	}
}

func TestFullClaimStatusBlocksPartialGLAndDoesNotSerializeSourceEvidence(t *testing.T) {
	input := fullClaimInput(testSource, testTenant, "package-partial", "11111111-1111-1111-1111-111111111111")
	input.ClaimKind = "partial"
	input.ExpectedCoverageState = "partial"
	service, _ := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	if _, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource); err != nil {
		t.Fatal(err)
	}

	status, err := service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != FullClaimStatusNotEligible || status.FullClaimEligible || status.SelectedCount != 1 || status.CurrentPassCount != 0 || status.CurrentPassGapCount != 1 || status.SourceCoverageGapCount != 1 {
		t.Fatalf("partial GL full-claim status = %#v", status)
	}
	if !contains(status.BlockingCodes, fullClaimBlockerCurrentPass) || !contains(status.BlockingCodes, fullClaimBlockerSourceCoverage) || !contains(status.BlockingCodes, fullClaimBlockerMatrixFilter) || !contains(status.BlockingCodes, fullClaimBlockerMatrixPageOnly) || !contains(status.BlockingCodes, fullClaimBlockerMatrixUnconsumed) || !contains(status.BlockingCodes, fullClaimBlockerMatrixMissingAPI) || !contains(status.BlockingCodes, fullClaimBlockerMatrixExport) {
		t.Fatalf("partial GL fixed blockers = %#v", status.BlockingCodes)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{testSource, testTenant, "package-partial", input.ManifestSHA256, "debit", "credit", "proof"} {
		if containsString(string(encoded), forbidden) {
			t.Fatalf("full-claim status leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestFullClaimStatusRequiresEveryOriginalSelectedOrAllBindingToPass(t *testing.T) {
	firstSource, secondSource := "sa-browser-v1-1234", "sa-browser-v1-5678"
	firstTenant, secondTenant := "tenant-1", "tenant-2"
	inputs := map[string]EvaluationInput{
		firstSource:  fullClaimInput(firstSource, firstTenant, "package-1", "11111111-1111-1111-1111-111111111111"),
		secondSource: fullClaimInput(secondSource, secondTenant, "package-2", "22222222-2222-2222-2222-222222222222"),
	}
	bindings := []SourceBinding{
		{BatchID: testBatch, SourceCompanyID: firstSource, TenantID: firstTenant, Paired: true},
		{BatchID: testBatch, SourceCompanyID: secondSource, TenantID: secondTenant, Paired: true},
	}
	service, _ := fullClaimService(inputs, bindings)
	makeCurrentPass(t, service, firstSource, firstTenant)

	status, err := service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != FullClaimStatusNotEligible || status.FullClaimEligible || status.SelectedCount != 2 || status.CurrentPassCount != 1 || status.CurrentPassGapCount != 1 {
		t.Fatalf("mixed selected/all full-claim status = %#v", status)
	}
	if !contains(status.BlockingCodes, fullClaimBlockerCurrentPass) || status.MatrixBlockerCount == 0 {
		t.Fatalf("mixed selected/all blockers = %#v", status)
	}

	makeCurrentPass(t, service, secondSource, secondTenant)
	status, err = service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != FullClaimStatusNotEligible || status.FullClaimEligible || status.CurrentPassCount != 2 || status.MatrixBlockerCount == 0 {
		t.Fatalf("current product matrix must keep complete batch from claiming full = %#v", status)
	}
}

func TestFullClaimStatusCountsCurrentTombstoneGapsWithoutIdentities(t *testing.T) {
	input := fullClaimInput(testSource, testTenant, "package-tombstone", "11111111-1111-1111-1111-111111111111")
	input.GLTombstoneUnresolved = 1
	service, _ := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	if _, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource); err != nil {
		t.Fatal(err)
	}
	status, err := service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil {
		t.Fatal(err)
	}
	if status.TombstoneGapSourceCount != 1 || !contains(status.BlockingCodes, fullClaimBlockerTombstones) {
		t.Fatalf("tombstone full-claim status = %#v", status)
	}
}

func containsString(value, want string) bool { return len(want) > 0 && strings.Contains(value, want) }
