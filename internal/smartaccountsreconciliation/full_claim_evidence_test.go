package smartaccountsreconciliation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func syntheticFullClaimPlan() []smartaccountssync.FullClaimDomainPlanEntry {
	return []smartaccountssync.FullClaimDomainPlanEntry{{
		PlanVersion: smartaccountssync.FullClaimCoveragePlanVersion,
		DomainID:    "synthetic_domain",
		Selected: smartaccountssync.FullClaimCoverageRow{
			Source:          "synthetic_source",
			ResourceID:      "synthetic_resource",
			ContractVersion: "synthetic-contract-v1",
			Disposition:     smartaccountssync.FullClaimDispositionResolved,
		},
	}}
}

func completeFullClaimDomainEvidence(binding FullClaimEvidenceBinding) FullClaimDomainEvidence {
	return FullClaimDomainEvidence{
		ID:                       uuid.NewString(),
		FullClaimEvidenceBinding: binding,
		PlanVersion:              smartaccountssync.FullClaimCoveragePlanVersion,
		DomainID:                 "synthetic_domain",
		Source:                   "synthetic_source",
		ResourceID:               "synthetic_resource",
		ContractVersion:          "synthetic-contract-v1",
		LiveSourceValidated:      true,
		SchemaValidated:          true,
		CompletenessValidated:    true,
		ReconciliationValidated:  true,
		TombstonesResolved:       true,
		AccountantAttested:       true,
		EvidenceSHA256:           digest("8"),
		RecordedAt:               time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC),
	}
}

func TestFullClaimDomainEvidenceEvaluatorFailsClosedAcrossEveryBindingAndGate(t *testing.T) {
	plan := syntheticFullClaimPlan()
	binding := FullClaimEvidenceBinding{BatchID: testBatch, TenantID: testTenant, SourceCompanyID: testSource, PackageID: "package-1", ScopeSHA256: digest("c"), ReconciliationEvidenceSHA256: digest("e")}
	complete := completeFullClaimDomainEvidence(binding)

	if result := EvaluateFullClaimDomainEvidence(plan, binding, []FullClaimDomainEvidence{complete}, 0); !result.FullClaimEligible {
		t.Fatalf("complete bound evidence = %#v", result)
	}

	for _, mutate := range []struct {
		name string
		fn   func(*FullClaimDomainEvidence)
	}{
		{"source", func(value *FullClaimDomainEvidence) { value.SourceCompanyID = "sa-browser-v1-5678" }},
		{"package", func(value *FullClaimDomainEvidence) { value.PackageID = "package-2" }},
		{"scope", func(value *FullClaimDomainEvidence) { value.ScopeSHA256 = digest("7") }},
		{"reconciliation generation", func(value *FullClaimDomainEvidence) { value.ReconciliationEvidenceSHA256 = digest("4") }},
		{"source validation", func(value *FullClaimDomainEvidence) { value.LiveSourceValidated = false }},
		{"schema validation", func(value *FullClaimDomainEvidence) { value.SchemaValidated = false }},
		{"completeness validation", func(value *FullClaimDomainEvidence) { value.CompletenessValidated = false }},
		{"reconciliation validation", func(value *FullClaimDomainEvidence) { value.ReconciliationValidated = false }},
		{"tombstone validation", func(value *FullClaimDomainEvidence) { value.TombstonesResolved = false }},
		{"accountant attestation", func(value *FullClaimDomainEvidence) { value.AccountantAttested = false }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			value := complete
			mutate.fn(&value)
			if result := EvaluateFullClaimDomainEvidence(plan, binding, []FullClaimDomainEvidence{value}, 0); result.FullClaimEligible {
				t.Fatalf("tampered %s unexpectedly eligible: %#v", mutate.name, result)
			}
		})
	}

	if result := EvaluateFullClaimDomainEvidence(plan, binding, []FullClaimDomainEvidence{complete, complete}, 0); result.FullClaimEligible {
		t.Fatalf("duplicate domain receipt unexpectedly eligible: %#v", result)
	}
	if result := EvaluateFullClaimDomainEvidence(plan, binding, []FullClaimDomainEvidence{complete}, 1); result.FullClaimEligible {
		t.Fatalf("unresolved tombstone unexpectedly eligible: %#v", result)
	}
}

func TestFullClaimDomainEvidenceRepositoryProjectionIsBoundedAndRoundTrips(t *testing.T) {
	binding := FullClaimEvidenceBinding{BatchID: testBatch, TenantID: testTenant, SourceCompanyID: testSource, PackageID: "package-1", ScopeSHA256: digest("c"), ReconciliationEvidenceSHA256: digest("e")}
	original := completeFullClaimDomainEvidence(binding)
	record, err := fullClaimDomainEvidenceToRecord(original)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := fullClaimDomainEvidenceFromRecord(record)
	if err != nil || !sameFullClaimDomainEvidence(*restored, original) {
		t.Fatalf("record round trip = %#v / %#v / %v", restored, original, err)
	}

	record.AccountantAttested = false
	if _, err := fullClaimDomainEvidenceFromRecord(record); !errors.Is(err, ErrInvalid) {
		t.Fatalf("incomplete persisted receipt = %v, want ErrInvalid", err)
	}
}

func TestFullClaimDomainEvidenceRepositoryUsesImmutableBindingConflictKey(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=postgres dbname=postgres sslmode=disable"}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var statement string
	if err := db.Callback().Create().After("gorm:create").Register("full_claim_evidence_capture", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
		tx.RowsAffected = 1
	}); err != nil {
		t.Fatal(err)
	}

	binding := FullClaimEvidenceBinding{BatchID: testBatch, TenantID: testTenant, SourceCompanyID: testSource, PackageID: "package-1", ScopeSHA256: digest("c"), ReconciliationEvidenceSHA256: digest("e")}
	value, created, err := NewGORMRepository(db).SaveFullClaimDomainEvidence(context.Background(), completeFullClaimDomainEvidence(binding))
	if err != nil || !created || value == nil {
		t.Fatalf("save durable receipt = %#v / created=%v / %v", value, created, err)
	}
	for _, column := range []string{"batch_id", "tenant_id", "source_company_id", "package_id", "scope_sha256", "reconciliation_evidence_sha256", "plan_version", "domain_id", "live_source_validated", "accountant_attested", "evidence_sha256"} {
		if !strings.Contains(statement, column) {
			t.Fatalf("insert omits immutable receipt column %q: %s", column, statement)
		}
	}
	for _, forbidden := range []string{"amount", "source_row", "cookie", "credential", "request_body", "response_body", "note"} {
		if strings.Contains(strings.ToLower(statement), forbidden) {
			t.Fatalf("insert leaks forbidden data column %q: %s", forbidden, statement)
		}
	}
}

func TestFullClaimStatusRequiresDurableEvidenceForEveryCurrentPass(t *testing.T) {
	input := fullClaimInput(testSource, testTenant, "package-1", "11111111-1111-1111-1111-111111111111")
	service, _ := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	service.fullClaimPlan = syntheticFullClaimPlan
	makeCurrentPass(t, service, testSource, testTenant)

	status, err := service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil {
		t.Fatal(err)
	}
	if status.FullClaimEligible || status.DomainEvidenceGapSourceCount != 1 || !contains(status.BlockingCodes, fullClaimBlockerDomainEvidence) {
		t.Fatalf("PASS without durable domain evidence = %#v", status)
	}

	evaluation, err := service.GetForOwner(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	binding := FullClaimEvidenceBinding{BatchID: testBatch, TenantID: testTenant, SourceCompanyID: testSource, PackageID: input.PackageID, ScopeSHA256: input.ScopeSHA256, ReconciliationEvidenceSHA256: evaluation.EvidenceSHA256}
	if _, created, err := service.RecordFullClaimDomainEvidence(context.Background(), completeFullClaimDomainEvidence(binding)); err != nil || !created {
		t.Fatalf("record complete domain evidence = created:%v err:%v", created, err)
	}
	status, err = service.FullClaimStatus(context.Background(), "evidence-owner", testBatch)
	if err != nil || !status.FullClaimEligible || status.Status != FullClaimStatusEligible || status.DomainEvidenceGapSourceCount != 0 {
		t.Fatalf("fully evidenced current PASS = %#v / %v", status, err)
	}
}

func TestFullClaimDomainEvidenceIsAppendOnlyAndExactRetryOnly(t *testing.T) {
	input := fullClaimInput(testSource, testTenant, "package-1", "11111111-1111-1111-1111-111111111111")
	service, _ := fullClaimService(map[string]EvaluationInput{testSource: input}, []SourceBinding{{BatchID: testBatch, SourceCompanyID: testSource, TenantID: testTenant, Paired: true}})
	service.fullClaimPlan = syntheticFullClaimPlan
	makeCurrentPass(t, service, testSource, testTenant)
	// The immutable route receipt must bind to the exact approved evaluation.
	evaluation, err := service.GetForOwner(context.Background(), "evidence-owner", testBatch, testSource)
	if err != nil {
		t.Fatal(err)
	}
	binding := FullClaimEvidenceBinding{BatchID: testBatch, TenantID: testTenant, SourceCompanyID: testSource, PackageID: input.PackageID, ScopeSHA256: input.ScopeSHA256, ReconciliationEvidenceSHA256: evaluation.EvidenceSHA256}
	evidence := completeFullClaimDomainEvidence(binding)
	if _, created, err := service.RecordFullClaimDomainEvidence(context.Background(), evidence); err != nil || !created {
		t.Fatalf("first record = created:%v err:%v", created, err)
	}
	if _, created, err := service.RecordFullClaimDomainEvidence(context.Background(), evidence); err != nil || created {
		t.Fatalf("exact retry = created:%v err:%v", created, err)
	}
	tampered := evidence
	tampered.ID = uuid.NewString()
	tampered.EvidenceSHA256 = digest("6")
	if _, _, err := service.RecordFullClaimDomainEvidence(context.Background(), tampered); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed immutable receipt = %v, want ErrConflict", err)
	}
}
