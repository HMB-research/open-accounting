package smartaccountsreconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/google/uuid"
)

const (
	// FullClaimDomainEvidenceWorkerVersion is part of the receipt digest. A
	// change to the worker's derivation rules must produce fresh, immutable
	// evidence rather than making an earlier receipt look stronger.
	FullClaimDomainEvidenceWorkerVersion = "smartaccounts-full-claim-domain-evidence-worker-v1"

	// These are fixed, reviewed states emitted by server-side collector and
	// contract registries. They are deliberately strings rather than caller
	// supplied booleans: the worker only turns these exact terminal states into
	// evidence, and treats every unknown or pending value as not qualified.
	FullClaimCollectorStateQualified      = "QUALIFIED"
	FullClaimSchemaStateReviewed          = "REVIEWED"
	FullClaimCompletenessStateVerified    = "VERIFIED"
	fullClaimEvidenceRunRecorded          = "RECORDED"
	fullClaimEvidenceRunEvaluationPending = "EVALUATION_PENDING"
	fullClaimEvidenceRunPlanBlocked       = "PLAN_BLOCKED"
	fullClaimEvidenceRunArtifactsPending  = "ARTIFACTS_PENDING"
)

var (
	ErrFullClaimDomainEvidenceWorkerUnavailable    = errors.New("SmartAccounts full-claim domain evidence worker is unavailable")
	ErrFullClaimDomainEvidenceArtifactsUnavailable = errors.New("SmartAccounts full-claim domain evidence artifacts are unavailable")
)

// QualifiedFullClaimDomainArtifact is a digest-only projection of three
// independently reviewed domain controls. It intentionally contains neither
// source records nor URLs, selectors, credentials, cookies, user input, or
// free-form assertions. Implementations must read it from server-controlled
// collector/contract state; it has no HTTP request or response representation.
//
// Reconciliation, tombstone, and accountant gates do not appear here because
// the worker derives those directly from the current, immutable OA evaluation.
type QualifiedFullClaimDomainArtifact struct {
	PlanVersion        string
	DomainID           string
	Source             string
	ResourceID         string
	ContractVersion    string
	CollectorState     string
	SchemaState        string
	CompletenessState  string
	CollectorSHA256    string
	SchemaReviewSHA256 string
	CompletenessSHA256 string
}

// FullClaimDomainArtifactResolver is server-only. It may read OA's reviewed
// collector and contract registries, but must not call the source system,
// submit browser data, or inspect raw archive content. A full run first checks
// that every selected plan route is recordable, so this resolver is never
// invoked while the plan still contains an unresolved source-contract gap.
type FullClaimDomainArtifactResolver interface {
	ResolveQualifiedFullClaimDomainArtifact(context.Context, FullClaimEvidenceBinding, smartaccountssync.FullClaimDomainPlanEntry) (QualifiedFullClaimDomainArtifact, error)
}

// FullClaimDomainEvidenceRun is safe operational telemetry for a server job.
// It intentionally provides only counts and fixed statuses; a job log or
// future status endpoint cannot use it to reveal source/domain details.
type FullClaimDomainEvidenceRun struct {
	Status                 string
	SelectedDomainCount    int
	RecordedCount          int
	ExistingCount          int
	PendingArtifactCount   int
	PlanBlockedDomainCount int
}

// FullClaimDomainEvidenceWorker converts qualified, server-side collector
// artifacts into immutable full-claim-domain receipts. It cannot accept a
// user assertion of eligibility: the exact batch/source binding and current
// PASS evaluation are read through Service, while route metadata comes from
// the immutable selected plan and collector proof comes from the resolver.
//
// The worker is intentionally not wired to HTTP or a browser callback. A
// trusted server scheduler or orchestrator may invoke RecordForSource after a
// collector/contract registry has reached its reviewed terminal states.
type FullClaimDomainEvidenceWorker struct {
	service   *Service
	artifacts FullClaimDomainArtifactResolver
	now       func() time.Time
	newID     func() string
}

func NewFullClaimDomainEvidenceWorker(service *Service, artifacts FullClaimDomainArtifactResolver) *FullClaimDomainEvidenceWorker {
	return &FullClaimDomainEvidenceWorker{service: service, artifacts: artifacts, now: time.Now, newID: uuid.NewString}
}

// RecordForSource records every selected domain atomically at the logical
// level: it reads and validates every artifact before writing the first
// immutable receipt. Database interruption may leave an append-only prefix,
// but an exact retry is safe and cannot overwrite a receipt. It returns a
// non-error pending result for ordinary, unqualified work and fails closed
// without resolving artifacts when the plan itself is incomplete.
func (w *FullClaimDomainEvidenceWorker) RecordForSource(ctx context.Context, ownerID, batchID, sourceID string) (*FullClaimDomainEvidenceRun, error) {
	if w == nil || w.service == nil || w.artifacts == nil || w.newID == nil || !safeActor(ownerID) || !safeID(batchID) || !safeSource(sourceID) {
		return nil, ErrFullClaimDomainEvidenceWorkerUnavailable
	}
	ownerID, batchID, sourceID = strings.TrimSpace(ownerID), strings.TrimSpace(batchID), strings.TrimSpace(sourceID)
	plan := w.service.currentFullClaimPlan()
	run := &FullClaimDomainEvidenceRun{SelectedDomainCount: len(plan)}
	if blocked := fullClaimPlanBlockedForEvidenceWorker(plan); blocked > 0 {
		run.Status = fullClaimEvidenceRunPlanBlocked
		run.PlanBlockedDomainCount = blocked
		return run, nil
	}

	evaluation, err := w.service.GetForOwner(ctx, ownerID, batchID, sourceID)
	if err != nil || !fullClaimEvaluationReadyForDomainEvidence(evaluation, batchID, sourceID) {
		run.Status = fullClaimEvidenceRunEvaluationPending
		return run, nil
	}
	binding := FullClaimEvidenceBinding{
		BatchID:                      batchID,
		TenantID:                     evaluation.TenantID,
		SourceCompanyID:              sourceID,
		PackageID:                    evaluation.PackageID,
		ScopeSHA256:                  evaluation.ScopeSHA256,
		ReconciliationEvidenceSHA256: evaluation.EvidenceSHA256,
	}
	if !validFullClaimEvidenceBinding(binding) {
		return nil, ErrInvalid
	}

	// Fetch and validate every artifact before writing any receipt. This keeps a
	// normal qualification run all-or-nothing until its selected collector
	// domains are ready, while retaining append-only retry semantics once it is.
	artifacts := make([]QualifiedFullClaimDomainArtifact, 0, len(plan))
	for _, entry := range plan {
		artifact, artifactErr := w.artifacts.ResolveQualifiedFullClaimDomainArtifact(ctx, binding, entry)
		if artifactErr != nil {
			if !errors.Is(artifactErr, ErrNotFound) && !errors.Is(artifactErr, ErrNotReady) {
				return nil, ErrFullClaimDomainEvidenceArtifactsUnavailable
			}
			run.Status = fullClaimEvidenceRunArtifactsPending
			run.PendingArtifactCount++
			continue
		}
		if !qualifiedFullClaimDomainArtifactMatches(entry, artifact) {
			run.Status = fullClaimEvidenceRunArtifactsPending
			run.PendingArtifactCount++
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	if run.PendingArtifactCount > 0 {
		return run, nil
	}
	if len(artifacts) != len(plan) {
		return nil, ErrInvalid
	}

	for index, entry := range plan {
		evidence := fullClaimEvidenceFromQualifiedArtifact(binding, entry, artifacts[index], w.nextID(), w.currentTime())
		stored, created, storeErr := w.service.RecordFullClaimDomainEvidence(ctx, evidence)
		if storeErr != nil || stored == nil {
			return nil, storeErr
		}
		if created {
			run.RecordedCount++
		} else {
			run.ExistingCount++
		}
	}
	run.Status = fullClaimEvidenceRunRecorded
	return run, nil
}

func fullClaimPlanBlockedForEvidenceWorker(plan []smartaccountssync.FullClaimDomainPlanEntry) int {
	if len(plan) == 0 {
		return 1
	}
	seen := make(map[string]struct{}, len(plan))
	blocked := 0
	for _, entry := range plan {
		if entry.PlanVersion != smartaccountssync.FullClaimCoveragePlanVersion || !safeID(entry.DomainID) || !safeID(entry.Selected.Source) || !safeID(entry.Selected.ResourceID) || !safeID(entry.Selected.ContractVersion) {
			blocked++
			continue
		}
		if _, duplicate := seen[entry.DomainID]; duplicate {
			blocked++
			continue
		}
		seen[entry.DomainID] = struct{}{}
		switch entry.Selected.Disposition {
		case smartaccountssync.FullClaimDispositionGLApplyGated,
			smartaccountssync.FullClaimDispositionReferenceApplyGated,
			smartaccountssync.FullClaimDispositionArchiveOnly,
			smartaccountssync.FullClaimDispositionResolved:
			// A reviewed artifact and a current approved evaluation are still
			// required below. The disposition merely makes a route recordable.
		default:
			blocked++
		}
	}
	return blocked
}

func fullClaimEvaluationReadyForDomainEvidence(evaluation *Evaluation, batchID, sourceID string) bool {
	return evaluation != nil && evaluation.BatchID == batchID && evaluation.SourceCompanyID == sourceID && safeID(evaluation.TenantID) && safeID(evaluation.PackageID) && safeDigest(evaluation.ScopeSHA256) && safeDigest(evaluation.EvidenceSHA256) && evaluation.Status == StatusPass && evaluation.ClaimKind == "full" && evaluation.ExpectedCoverageState == "full" && evaluation.GLTombstoneUnresolved == 0 && evaluation.ReferenceTombstoneUnresolved == 0 && evaluation.GLRevisionUnresolved == 0 && evaluation.ReferenceRevisionUnresolved == 0 && len(evaluation.Blockers) == 0 && evaluation.AccountantApprovedAt != nil && !evaluation.AccountantApprovedAt.IsZero()
}

func qualifiedFullClaimDomainArtifactMatches(entry smartaccountssync.FullClaimDomainPlanEntry, artifact QualifiedFullClaimDomainArtifact) bool {
	return artifact.PlanVersion == entry.PlanVersion && artifact.DomainID == entry.DomainID && artifact.Source == entry.Selected.Source && artifact.ResourceID == entry.Selected.ResourceID && artifact.ContractVersion == entry.Selected.ContractVersion && artifact.CollectorState == FullClaimCollectorStateQualified && artifact.SchemaState == FullClaimSchemaStateReviewed && artifact.CompletenessState == FullClaimCompletenessStateVerified && safeDigest(artifact.CollectorSHA256) && safeDigest(artifact.SchemaReviewSHA256) && safeDigest(artifact.CompletenessSHA256)
}

func fullClaimEvidenceFromQualifiedArtifact(binding FullClaimEvidenceBinding, entry smartaccountssync.FullClaimDomainPlanEntry, artifact QualifiedFullClaimDomainArtifact, id string, recordedAt time.Time) FullClaimDomainEvidence {
	return FullClaimDomainEvidence{
		ID:                       id,
		FullClaimEvidenceBinding: binding,
		PlanVersion:              entry.PlanVersion,
		DomainID:                 entry.DomainID,
		Source:                   entry.Selected.Source,
		ResourceID:               entry.Selected.ResourceID,
		ContractVersion:          entry.Selected.ContractVersion,
		LiveSourceValidated:      true,
		SchemaValidated:          true,
		CompletenessValidated:    true,
		ReconciliationValidated:  true,
		TombstonesResolved:       true,
		AccountantAttested:       true,
		EvidenceSHA256:           fullClaimQualifiedArtifactDigest(binding, entry, artifact),
		RecordedAt:               recordedAt.UTC(),
	}
}

func fullClaimQualifiedArtifactDigest(binding FullClaimEvidenceBinding, entry smartaccountssync.FullClaimDomainPlanEntry, artifact QualifiedFullClaimDomainArtifact) string {
	// Every value is either a validated opaque identifier/digest, fixed route
	// metadata, or fixed reviewed state. The canonical payload intentionally
	// excludes source records, amounts, names, cookies, credentials, URLs, and
	// user/operator input.
	value := struct {
		Version              string `json:"version"`
		BatchID              string `json:"batch_id"`
		TenantID             string `json:"tenant_id"`
		SourceCompanyID      string `json:"source_company_id"`
		PackageID            string `json:"package_id"`
		ScopeSHA256          string `json:"scope_sha256"`
		ReconciliationSHA256 string `json:"reconciliation_evidence_sha256"`
		PlanVersion          string `json:"plan_version"`
		DomainID             string `json:"domain_id"`
		Source               string `json:"source"`
		ResourceID           string `json:"resource_id"`
		ContractVersion      string `json:"contract_version"`
		CollectorState       string `json:"collector_state"`
		SchemaState          string `json:"schema_state"`
		CompletenessState    string `json:"completeness_state"`
		CollectorSHA256      string `json:"collector_sha256"`
		SchemaReviewSHA256   string `json:"schema_review_sha256"`
		CompletenessSHA256   string `json:"completeness_sha256"`
	}{
		FullClaimDomainEvidenceWorkerVersion,
		binding.BatchID,
		binding.TenantID,
		binding.SourceCompanyID,
		binding.PackageID,
		binding.ScopeSHA256,
		binding.ReconciliationEvidenceSHA256,
		entry.PlanVersion,
		entry.DomainID,
		entry.Selected.Source,
		entry.Selected.ResourceID,
		entry.Selected.ContractVersion,
		artifact.CollectorState,
		artifact.SchemaState,
		artifact.CompletenessState,
		artifact.CollectorSHA256,
		artifact.SchemaReviewSHA256,
		artifact.CompletenessSHA256,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		// The concrete canonical value cannot fail JSON marshaling. Returning an
		// invalid digest is safer than a panic if that ever changes: persistence
		// rejects it through validFullClaimDomainEvidence.
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (w *FullClaimDomainEvidenceWorker) currentTime() time.Time {
	if w != nil && w.now != nil {
		return w.now().UTC()
	}
	return time.Now().UTC()
}

func (w *FullClaimDomainEvidenceWorker) nextID() string {
	if w != nil && w.newID != nil {
		return w.newID()
	}
	return uuid.NewString()
}
