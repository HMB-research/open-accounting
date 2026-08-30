package smartaccountsreconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	testBatch  = "batch-1"
	testSource = "sa-browser-v1-1234"
	testTenant = "tenant-1"
)

type memoryStore struct {
	evaluations       map[string]*Evaluation
	approvals         map[string]*Approval
	fullClaimEvidence map[string]*FullClaimDomainEvidence
}

func evaluationKey(batch, source, tenant, digest string) string {
	return batch + "\x00" + source + "\x00" + tenant + "\x00" + digest
}
func sourceKey(batch, source, tenant string) string { return batch + "\x00" + source + "\x00" + tenant }

func (m *memoryStore) SaveEvaluation(_ context.Context, value Evaluation) (*Evaluation, bool, error) {
	if m.evaluations == nil {
		m.evaluations = map[string]*Evaluation{}
	}
	key := evaluationKey(value.BatchID, value.SourceCompanyID, value.TenantID, value.EvidenceSHA256)
	if existing := m.evaluations[key]; existing != nil {
		return existing, false, nil
	}
	copy := value
	m.evaluations[key] = &copy
	return &copy, true, nil
}
func (m *memoryStore) GetLatestEvaluation(_ context.Context, batch, source, tenant string) (*Evaluation, error) {
	values := make([]*Evaluation, 0)
	for _, value := range m.evaluations {
		if sourceKey(value.BatchID, value.SourceCompanyID, value.TenantID) == sourceKey(batch, source, tenant) {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil, ErrNotFound
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].CreatedAt.After(values[j].CreatedAt) || (values[i].CreatedAt.Equal(values[j].CreatedAt) && values[i].ID > values[j].ID)
	})
	return values[0], nil
}
func (m *memoryStore) GetEvaluation(_ context.Context, id string) (*Evaluation, error) {
	for _, value := range m.evaluations {
		if value.ID == id {
			return value, nil
		}
	}
	return nil, ErrNotFound
}
func (m *memoryStore) CreateApproval(_ context.Context, value Approval) (*Approval, bool, error) {
	if m.approvals == nil {
		m.approvals = map[string]*Approval{}
	}
	key := value.EvaluationID + "\x00" + value.EvidenceSHA256 + "\x00" + value.ToleranceSHA256
	if existing := m.approvals[key]; existing != nil {
		return existing, false, nil
	}
	copy := value
	m.approvals[key] = &copy
	return &copy, true, nil
}
func (m *memoryStore) GetApproval(_ context.Context, evaluationID string) (*Approval, error) {
	for _, value := range m.approvals {
		if value.EvaluationID == evaluationID {
			return value, nil
		}
	}
	return nil, ErrNotFound
}
func (m *memoryStore) MarkEvaluationPassed(_ context.Context, id, evidence, tolerance string, at time.Time) (*Evaluation, error) {
	value, err := m.GetEvaluation(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if value.EvidenceSHA256 != evidence || value.ToleranceSHA256 != tolerance || (value.Status != StatusReadyForAccountant && value.Status != StatusPass) {
		return nil, ErrConflict
	}
	value.Status = StatusPass
	value.AccountantApprovedAt = &at
	value.UpdatedAt = at
	return value, nil
}

func fullClaimEvidenceKey(binding FullClaimEvidenceBinding, planVersion, domainID string) string {
	return binding.BatchID + "\x00" + binding.TenantID + "\x00" + binding.SourceCompanyID + "\x00" + binding.PackageID + "\x00" + binding.ScopeSHA256 + "\x00" + binding.ReconciliationEvidenceSHA256 + "\x00" + planVersion + "\x00" + domainID
}

func (m *memoryStore) SaveFullClaimDomainEvidence(_ context.Context, value FullClaimDomainEvidence) (*FullClaimDomainEvidence, bool, error) {
	if !validFullClaimDomainEvidence(value) {
		return nil, false, ErrInvalid
	}
	if m.fullClaimEvidence == nil {
		m.fullClaimEvidence = map[string]*FullClaimDomainEvidence{}
	}
	key := fullClaimEvidenceKey(value.FullClaimEvidenceBinding, value.PlanVersion, value.DomainID)
	if existing := m.fullClaimEvidence[key]; existing != nil {
		if !sameFullClaimDomainEvidence(*existing, value) {
			return nil, false, ErrConflict
		}
		copy := *existing
		return &copy, false, nil
	}
	copy := value
	m.fullClaimEvidence[key] = &copy
	return &copy, true, nil
}

func (m *memoryStore) ListFullClaimDomainEvidence(_ context.Context, binding FullClaimEvidenceBinding, planVersion string) ([]FullClaimDomainEvidence, error) {
	if !validFullClaimEvidenceBinding(binding) || planVersion == "" {
		return nil, ErrInvalid
	}
	values := make([]FullClaimDomainEvidence, 0)
	for _, value := range m.fullClaimEvidence {
		if sameFullClaimEvidenceBinding(value.FullClaimEvidenceBinding, binding) && value.PlanVersion == planVersion {
			values = append(values, *value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].DomainID < values[j].DomainID })
	return values, nil
}

type memoryResolver struct {
	input    EvaluationInput
	bindings []SourceBinding
	err      error
}

func (r *memoryResolver) Resolve(_ context.Context, _ string, batch, source string) (EvaluationInput, error) {
	if r.err != nil {
		return EvaluationInput{}, r.err
	}
	if r.input.BatchID != batch || r.input.SourceCompanyID != source {
		return EvaluationInput{}, ErrNotFound
	}
	return r.input, nil
}
func (r *memoryResolver) ListBindings(_ context.Context, _ string, batch string) ([]SourceBinding, error) {
	if r.err != nil {
		return nil, r.err
	}
	if batch != testBatch {
		return nil, ErrNotFound
	}
	return r.bindings, nil
}

func digest(letter string) string { return strings.Repeat(letter, 64) }
func completeInput() EvaluationInput {
	replayed := time.Date(2026, 8, 28, 12, 1, 0, 0, time.UTC)
	return EvaluationInput{
		BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, BindingComplete: true,
		PackageID: "package-1", ManifestSHA256: digest("a"), RecordsSHA256: digest("b"), ScopeSHA256: digest("c"), SourceAsOfDate: "2026-08-28", CutoffAt: "2026-08-28T12:00:00Z",
		GLPreviewID: "11111111-1111-1111-1111-111111111111", GLPreviewSHA256: digest("d"), GLPreviewApplied: true,
		GLApplyReceipt:          &GLApplyReceipt{TenantID: testTenant, SourceCompanyID: testSource, PackageID: "package-1", PreviewID: "11111111-1111-1111-1111-111111111111", PreviewSHA256: digest("d"), FirstAppliedBy: "gl-operator", FirstAppliedAt: replayed.Add(-time.Minute), ExactReplayBy: "gl-replayer", ExactReplayAt: &replayed, MappingSnapshotSHA256: digest("e"), AppliedIdentitySHA256: digest("f"), TolerancePolicySHA256: digest("9"), MappingCount: 2, AppliedIdentityCount: 3},
		GLMappingSnapshotSHA256: digest("e"), GLAppliedIdentitySHA256: digest("f"),
		ProofID: "proof-1", ProofSHA256: digest("1"), ClaimSHA256: digest("2"), CoverageSHA256: digest("3"), ClaimKind: "full", ExpectedCoverageState: "full", ToleranceSHA256: digest("9"), VarianceWithinPolicy: true,
		ReferenceApplicable: true, ReferencePreviewID: "reference-preview-1", ReferencePreviewSHA256: digest("4"), ReferencePreviewApplied: true,
	}
}
func newService(input EvaluationInput) (*Service, *memoryStore, *memoryResolver) {
	store := &memoryStore{}
	resolver := &memoryResolver{input: input, bindings: []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}}}
	service := NewService(store, resolver)
	sequence := 0
	service.newID = func() string { sequence++; return "evaluation-" + string(rune('0'+sequence)) }
	service.now = func() time.Time { return time.Date(2026, 8, 28, 12, 2, 0, 0, time.UTC) }
	return service, store, resolver
}

func TestEvaluationStoresOnlyDigestBoundTechnicalEvidenceAndExactRetry(t *testing.T) {
	service, _, _ := newService(completeInput())
	first, created, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || !created || first.Status != StatusReadyForAccountant {
		t.Fatalf("first evaluation = %#v / created=%v / err=%v", first, created, err)
	}
	second, created, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("exact retry = %#v / created=%v / err=%v", second, created, err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"gl-operator", "gl-replayer", "evidence-owner", "debit", "credit", "source_row"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("safe evaluation serialized %q", forbidden)
		}
	}
}

func TestEvaluationRequiresExactReplayAndApplicableReference(t *testing.T) {
	input := completeInput()
	input.GLApplyReceipt.ExactReplayAt = nil
	service, _, _ := newService(input)
	evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || evaluation.Status != StatusEvidencePending || !contains(evaluation.Blockers, "gl_apply_replay_mapping_or_identity_receipt_missing") {
		t.Fatalf("legacy apply without replay = %#v / %v", evaluation, err)
	}
	input = completeInput()
	input.ReferencePreviewApplied = false
	service, _, _ = newService(input)
	evaluation, _, err = service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || evaluation.Status != StatusEvidencePending || !contains(evaluation.Blockers, "applicable_reference_apply_missing") {
		t.Fatalf("applicable reference = %#v / %v", evaluation, err)
	}
}

func TestEvaluationRejectsPartialCoverageAndMismatchedReceipt(t *testing.T) {
	input := completeInput()
	input.ClaimKind = "partial"
	service, _, _ := newService(input)
	evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || evaluation.Status != StatusEvidencePending || !contains(evaluation.Blockers, "claim_or_coverage_not_full") {
		t.Fatalf("partial coverage = %#v / %v", evaluation, err)
	}
	input = completeInput()
	input.GLApplyReceipt.PackageID = "other-package"
	service, _, _ = newService(input)
	evaluation, _, err = service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || evaluation.Status != StatusEvidencePending || !contains(evaluation.Blockers, "gl_apply_replay_mapping_or_identity_receipt_missing") {
		t.Fatalf("receipt mismatch = %#v / %v", evaluation, err)
	}
}

func TestAccountantApprovalRequiresIndependentAccountantAndExactDigests(t *testing.T) {
	service, _, _ := newService(completeInput())
	evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.Approve(context.Background(), "evidence-owner", "accountant", testTenant, evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256}); !errors.Is(err, ErrActorSeparation) {
		t.Fatalf("same evidence actor approval = %v", err)
	}
	if _, _, err = service.Approve(context.Background(), "independent", "owner", testTenant, evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256}); !errors.Is(err, ErrAccountantRequired) {
		t.Fatalf("non-accountant approval = %v", err)
	}
	passed, replay, err := service.Approve(context.Background(), "independent", "accountant", testTenant, evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256})
	if err != nil || replay || passed.Status != StatusPass {
		t.Fatalf("approval = %#v / replay=%v / %v", passed, replay, err)
	}
	passed, replay, err = service.Approve(context.Background(), "independent", "accountant", testTenant, evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256})
	if err != nil || !replay || passed.Status != StatusPass {
		t.Fatalf("approval replay = %#v / replay=%v / %v", passed, replay, err)
	}
}

func TestChangedPackageCreatesNewEvaluationAndSelectedRollupFailsClosed(t *testing.T) {
	input := completeInput()
	service, _, resolver := newService(input)
	first, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Approve(context.Background(), "accountant", "accountant", testTenant, first.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: first.EvidenceSHA256, ToleranceSHA256: first.ToleranceSHA256}); err != nil {
		t.Fatal(err)
	}
	rollup, err := service.Rollup(context.Background(), "evidence-owner", testBatch)
	if err != nil || rollup.Status != RollupPass {
		t.Fatalf("pass rollup = %#v / %v", rollup, err)
	}
	resolver.input.PackageID = "package-2"
	resolver.input.ManifestSHA256 = digest("5")
	resolver.input.GLApplyReceipt = nil
	resolver.input.GLPreviewApplied = false
	changed, created, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || !created || changed.Status != StatusEvidencePending {
		t.Fatalf("changed package = %#v / %v / %v", changed, created, err)
	}
	rollup, err = service.Rollup(context.Background(), "evidence-owner", testBatch)
	if err != nil || rollup.Status != RollupInProgress {
		t.Fatalf("changed package rollup = %#v / %v", rollup, err)
	}
}

func TestGetAndRollupFailClosedWhenCurrentGenerationChangesWithoutEvaluate(t *testing.T) {
	service, _, resolver := newService(completeInput())
	first, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Approve(context.Background(), "accountant", "accountant", testTenant, first.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: first.EvidenceSHA256, ToleranceSHA256: first.ToleranceSHA256}); err != nil {
		t.Fatal(err)
	}
	resolver.input.PackageID = "package-2"
	resolver.input.ManifestSHA256 = digest("5")
	current, err := service.GetForOwner(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil || current.Status != StatusNotEvaluated || !contains(current.Blockers, "evaluation_stale_current_generation") {
		t.Fatalf("stale owner status = %#v / %v", current, err)
	}
	rollup, err := service.Rollup(context.Background(), "evidence-owner", testBatch)
	if err != nil || rollup.Status != RollupInProgress {
		t.Fatalf("stale rollup = %#v / %v", rollup, err)
	}
}

func TestGetForTenantBindsTenantBatchSourceAndFailsClosedWhenCurrentEvidenceChanges(t *testing.T) {
	service, _, resolver := newService(completeInput())
	evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.GetForTenant(context.Background(), testTenant, testBatch, testSource)
	if err != nil || current == nil || current.ID != evaluation.ID || current.Status != StatusReadyForAccountant {
		t.Fatalf("tenant handoff = %#v / %v", current, err)
	}
	if _, err := service.GetForTenant(context.Background(), "other-tenant", testBatch, testSource); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant handoff = %v", err)
	}
	if _, err := service.GetForTenant(context.Background(), testTenant, "other-batch", testSource); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-batch handoff = %v", err)
	}
	resolver.input.PackageID = "package-2"
	resolver.input.ManifestSHA256 = digest("5")
	stale, err := service.GetForTenant(context.Background(), testTenant, testBatch, testSource)
	if err != nil || stale.Status != StatusNotEvaluated || !contains(stale.Blockers, "evaluation_stale_current_generation") {
		t.Fatalf("stale tenant handoff = %#v / %v", stale, err)
	}
	encoded, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"evidence-owner", "gl-operator", "debit", "credit", "source_row"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("tenant handoff serialized %q", forbidden)
		}
	}
}

func TestGetAndApprovalFailClosedWhenAnyApprovalEvidenceChanges(t *testing.T) {
	changes := map[string]func(*EvaluationInput){
		"reference_preview": func(input *EvaluationInput) { input.ReferencePreviewSHA256 = digest("7") },
		"receipt_mapping": func(input *EvaluationInput) {
			input.GLMappingSnapshotSHA256 = digest("8")
			input.GLApplyReceipt.MappingSnapshotSHA256 = digest("8")
		},
		"proof_claim": func(input *EvaluationInput) { input.ProofSHA256 = digest("6") },
		"coverage":    func(input *EvaluationInput) { input.CoverageSHA256 = digest("5") },
		"variance":    func(input *EvaluationInput) { input.VarianceWithinPolicy = false },
		"unresolved":  func(input *EvaluationInput) { input.GLRevisionUnresolved = 1 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			service, _, resolver := newService(completeInput())
			evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
			if err != nil {
				t.Fatal(err)
			}
			change(&resolver.input)
			current, err := service.GetForOwner(context.Background(), "evidence-owner", testBatch, testSource)
			if err != nil || current.Status != StatusNotEvaluated || !contains(current.Blockers, "evaluation_stale_current_generation") {
				t.Fatalf("changed %s must be stale: %#v / %v", name, current, err)
			}
			if _, _, err := service.Approve(context.Background(), "independent", "accountant", testTenant, evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256}); !errors.Is(err, ErrNotReady) {
				t.Fatalf("changed %s must block approval: %v", name, err)
			}
		})
	}
}

func TestApprovalRequiresExpectedTenant(t *testing.T) {
	service, _, _ := newService(completeInput())
	evaluation, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Approve(context.Background(), "independent", "accountant", "other-tenant", evaluation.ID, ApprovalRequest{Confirmed: true, EvidenceSHA256: evaluation.EvidenceSHA256, ToleranceSHA256: evaluation.ToleranceSHA256}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("known evaluation cross-tenant approval = %v", err)
	}
}

func TestBlockersRejectUnrecognizedOrDataBearingValues(t *testing.T) {
	if validBlockers([]string{"unresolved_gl_revisions", "raw amount 99.00"}) {
		t.Fatal("data-bearing blocker accepted")
	}
	if validBlockers([]string{"unresolved_gl_revisions", "unresolved_gl_revisions"}) {
		t.Fatal("duplicate blocker accepted")
	}
}

func TestRollupRequiresEveryOriginalSelectedBinding(t *testing.T) {
	service, _, resolver := newService(completeInput())
	resolver.bindings = append(resolver.bindings, SourceBinding{BatchID: testBatch, SourceCompanyID: "sa-browser-v1-5678", TenantID: "tenant-2", Paired: false})
	if _, _, err := service.Evaluate(context.Background(), "evidence-owner", testBatch, testSource); err != nil {
		t.Fatal(err)
	}
	rollup, err := service.Rollup(context.Background(), "evidence-owner", testBatch)
	if err != nil || rollup.Status != RollupInProgress || rollup.PendingCount != 1 {
		t.Fatalf("selected/all binding rollup = %#v / %v", rollup, err)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
