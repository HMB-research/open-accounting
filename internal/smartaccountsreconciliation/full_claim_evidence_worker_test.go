package smartaccountsreconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/google/uuid"
)

type fullClaimArtifactMemoryResolver struct {
	artifacts map[string]QualifiedFullClaimDomainArtifact
	err       error
	calls     int
}

func (r *fullClaimArtifactMemoryResolver) ResolveQualifiedFullClaimDomainArtifact(_ context.Context, _ FullClaimEvidenceBinding, entry smartaccountssync.FullClaimDomainPlanEntry) (QualifiedFullClaimDomainArtifact, error) {
	r.calls++
	if r.err != nil {
		return QualifiedFullClaimDomainArtifact{}, r.err
	}
	artifact, found := r.artifacts[entry.DomainID]
	if !found {
		return QualifiedFullClaimDomainArtifact{}, ErrNotFound
	}
	return artifact, nil
}

func qualifiedArtifactForPlan(entry smartaccountssync.FullClaimDomainPlanEntry) QualifiedFullClaimDomainArtifact {
	return QualifiedFullClaimDomainArtifact{
		PlanVersion:        entry.PlanVersion,
		DomainID:           entry.DomainID,
		Source:             entry.Selected.Source,
		ResourceID:         entry.Selected.ResourceID,
		ContractVersion:    entry.Selected.ContractVersion,
		CollectorState:     FullClaimCollectorStateQualified,
		SchemaState:        FullClaimSchemaStateReviewed,
		CompletenessState:  FullClaimCompletenessStateVerified,
		CollectorSHA256:    digest("1"),
		SchemaReviewSHA256: digest("2"),
		CompletenessSHA256: digest("3"),
	}
}

func qualifiedWorker(t *testing.T) (*FullClaimDomainEvidenceWorker, *Service, *memoryStore, *fullClaimArtifactMemoryResolver, smartaccountssync.FullClaimDomainPlanEntry) {
	t.Helper()
	input := fullClaimInput(testSource, testTenant, "package-worker", "11111111-1111-1111-1111-111111111111")
	service, store := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	plan := syntheticFullClaimPlan()
	service.fullClaimPlan = func() []smartaccountssync.FullClaimDomainPlanEntry { return plan }
	makeCurrentPass(t, service, testSource, testTenant)
	resolver := &fullClaimArtifactMemoryResolver{artifacts: map[string]QualifiedFullClaimDomainArtifact{plan[0].DomainID: qualifiedArtifactForPlan(plan[0])}}
	worker := NewFullClaimDomainEvidenceWorker(service, resolver)
	worker.now = func() time.Time { return time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC) }
	worker.newID = uuid.NewString
	return worker, service, store, resolver, plan[0]
}

func TestFullClaimDomainEvidenceWorkerDerivesAndPersistsOnlyQualifiedArtifacts(t *testing.T) {
	worker, service, store, resolver, entry := qualifiedWorker(t)
	run, err := worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != fullClaimEvidenceRunRecorded || run.SelectedDomainCount != 1 || run.RecordedCount != 1 || run.ExistingCount != 0 || run.PendingArtifactCount != 0 || resolver.calls != 1 {
		t.Fatalf("first qualification run = %#v, calls=%d", run, resolver.calls)
	}
	if len(store.fullClaimEvidence) != 1 {
		t.Fatalf("stored evidence = %#v", store.fullClaimEvidence)
	}
	for _, value := range store.fullClaimEvidence {
		if !value.LiveSourceValidated || !value.SchemaValidated || !value.CompletenessValidated || !value.ReconciliationValidated || !value.TombstonesResolved || !value.AccountantAttested {
			t.Fatalf("worker did not derive all verified controls: %#v", value)
		}
		if value.EvidenceSHA256 != fullClaimQualifiedArtifactDigest(value.FullClaimEvidenceBinding, entry, resolver.artifacts[entry.DomainID]) {
			t.Fatalf("worker did not derive immutable artifact digest: %#v", value)
		}
	}

	run, err = worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != fullClaimEvidenceRunRecorded || run.RecordedCount != 0 || run.ExistingCount != 1 || len(store.fullClaimEvidence) != 1 {
		t.Fatalf("exact worker retry = %#v / evidence=%#v", run, store.fullClaimEvidence)
	}

	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{testSource, testTenant, "package-worker", "collector_sha", "schema_review_sha"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("safe worker telemetry leaked %q: %s", forbidden, encoded)
		}
	}

	// The worker leaves existing FullClaimStatus semantics intact: it supplies
	// durable domain evidence, rather than an out-of-band eligibility flag.
	status, err := service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil || !status.FullClaimEligible || status.Status != FullClaimStatusEligible {
		t.Fatalf("worker-backed full-claim status = %#v / %v", status, err)
	}

	// A changed reviewed artifact cannot overwrite the old receipt inside the
	// same immutable binding; a new package/scope or plan is required.
	changed := resolver.artifacts[entry.DomainID]
	changed.CompletenessSHA256 = digest("9")
	resolver.artifacts[entry.DomainID] = changed
	if _, err = worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed qualified artifact = %v, want immutable conflict", err)
	}
	if len(store.fullClaimEvidence) != 1 {
		t.Fatalf("changed artifact overwrote evidence: %#v", store.fullClaimEvidence)
	}
}

func TestFullClaimDomainEvidenceWorkerFailsClosedBeforeCollectorResolution(t *testing.T) {
	input := fullClaimInput(testSource, testTenant, "package-worker", "11111111-1111-1111-1111-111111111111")
	service, store := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	resolver := &fullClaimArtifactMemoryResolver{}
	worker := NewFullClaimDomainEvidenceWorker(service, resolver)

	// The current plan has mandatory non-API contracts and other unresolved
	// selected routes. A full recorder run must make zero collector calls until
	// every selected route has a reviewed, recordable source path.
	run, err := worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != fullClaimEvidenceRunPlanBlocked || run.PlanBlockedDomainCount == 0 || resolver.calls != 0 || len(store.fullClaimEvidence) != 0 {
		t.Fatalf("unreviewed plan invoked collector or wrote evidence: %#v, calls=%d, evidence=%#v", run, resolver.calls, store.fullClaimEvidence)
	}
}

func TestFullClaimDomainEvidenceWorkerRequiresCurrentApprovedEvaluationAndCompleteArtifacts(t *testing.T) {
	input := fullClaimInput(testSource, testTenant, "package-worker", "11111111-1111-1111-1111-111111111111")
	service, store := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	plan := syntheticFullClaimPlan()
	service.fullClaimPlan = func() []smartaccountssync.FullClaimDomainPlanEntry { return plan }
	resolver := &fullClaimArtifactMemoryResolver{artifacts: map[string]QualifiedFullClaimDomainArtifact{plan[0].DomainID: qualifiedArtifactForPlan(plan[0])}}
	worker := NewFullClaimDomainEvidenceWorker(service, resolver)

	// Even a complete-looking collector artifact cannot get ahead of OA's
	// current full PASS and independent accountant approval.
	run, err := worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != fullClaimEvidenceRunEvaluationPending || resolver.calls != 0 || len(store.fullClaimEvidence) != 0 {
		t.Fatalf("unapproved evaluation recorded evidence: %#v, calls=%d, evidence=%#v", run, resolver.calls, store.fullClaimEvidence)
	}

	makeCurrentPass(t, service, testSource, testTenant)
	malformed := resolver.artifacts[plan[0].DomainID]
	malformed.SchemaState = "ASSERTED_BY_CALLER"
	resolver.artifacts[plan[0].DomainID] = malformed
	run, err = worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != fullClaimEvidenceRunArtifactsPending || run.PendingArtifactCount != 1 || len(store.fullClaimEvidence) != 0 {
		t.Fatalf("unreviewed artifact recorded evidence: %#v, evidence=%#v", run, store.fullClaimEvidence)
	}
}

func TestFullClaimDomainEvidenceWorkerDoesNotMaskArtifactStoreFailures(t *testing.T) {
	worker, _, store, resolver, _ := qualifiedWorker(t)
	resolver.err = errors.New("database connection lost")
	if _, err := worker.RecordForSource(context.Background(), "evidence-owner", testBatch, testSource); !errors.Is(err, ErrFullClaimDomainEvidenceArtifactsUnavailable) {
		t.Fatalf("artifact store failure = %v, want safe unavailable error", err)
	}
	if len(store.fullClaimEvidence) != 0 {
		t.Fatalf("artifact store failure wrote evidence: %#v", store.fullClaimEvidence)
	}
}
