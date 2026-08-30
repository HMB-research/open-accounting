package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BrowserBatchWorkflowSchemaVersion = "smartaccounts-browser-batch-workflow-v1"

	BrowserBatchPhasePaired                       = "PAIRED"
	BrowserBatchPhaseDiscoveryRequired            = "DISCOVERY_REQUIRED"
	BrowserBatchPhaseDiscoveryRunning             = "DISCOVERY_RUNNING"
	BrowserBatchPhaseDiscoveryComplete            = "DISCOVERY_COMPLETE"
	BrowserBatchPhaseSchemaReviewRequired         = "SCHEMA_REVIEW_REQUIRED"
	BrowserBatchPhaseSchemaApproved               = "SCHEMA_APPROVED"
	BrowserBatchPhaseTransferConfirmationRequired = "TRANSFER_CONFIRMATION_REQUIRED"
	BrowserBatchPhaseCaptureRunning               = "CAPTURE_RUNNING"
	BrowserBatchPhaseStaged                       = "STAGED"
	BrowserBatchPhasePreviewReady                 = "PREVIEW_READY"
	BrowserBatchPhaseReviewRequired               = "REVIEW_REQUIRED"
	BrowserBatchPhaseFailedRetryable              = "FAILED_RETRYABLE"
	BrowserBatchPhaseBlocked                      = "BLOCKED"

	// BrowserBatchNextStep values are a closed, safe status projection. They
	// are deliberately advisory: they never carry an action capability or
	// authorize source transfer, financial apply, or accountant approval.
	BrowserBatchNextStepAcquireDiscovery         = "ACQUIRE_DISCOVERY"
	BrowserBatchNextStepAwaitDiscovery           = "AWAIT_DISCOVERY"
	BrowserBatchNextStepAdvanceSchemaReview      = "ADVANCE_SCHEMA_REVIEW"
	BrowserBatchNextStepConfirmSchemaReview      = "CONFIRM_SCHEMA_REVIEW"
	BrowserBatchNextStepOpenTransferConfirmation = "OPEN_TRANSFER_CONFIRMATION"
	BrowserBatchNextStepConfirmTransferScope     = "CONFIRM_TRANSFER_SCOPE"
	BrowserBatchNextStepAcquireCapture           = "ACQUIRE_CAPTURE"
	BrowserBatchNextStepAwaitStaging             = "AWAIT_STAGING"
	BrowserBatchNextStepPreparePreview           = "PREPARE_PREVIEW"
	BrowserBatchNextStepReviewFinancialPreview   = "REVIEW_FINANCIAL_PREVIEW"
	BrowserBatchNextStepResolveReview            = "RESOLVE_REVIEW"
	BrowserBatchNextStepRecoverRetryable         = "RECOVER_RETRYABLE"
	BrowserBatchNextStepResolveBlocker           = "RESOLVE_BLOCKER"

	browserBatchWorkflowLeaseLifetime = 10 * time.Minute
)

var (
	ErrBrowserBatchWorkflowInvalid     = errors.New("SmartAccounts browser batch workflow is invalid")
	ErrBrowserBatchWorkflowConflict    = errors.New("SmartAccounts browser batch workflow conflicts with immutable state")
	ErrBrowserBatchWorkflowNotFound    = errors.New("SmartAccounts browser batch workflow was not found")
	ErrBrowserBatchWorkflowUnavailable = errors.New("SmartAccounts browser batch workflow is unavailable")
	ErrBrowserBatchWorkflowNotReady    = errors.New("SmartAccounts browser batch workflow is not ready for this phase")
)

// BrowserBatchWorkflow is the immutable preparation and later transfer
// authorization envelope for a single 081 selected/all onboarding batch. It
// intentionally has no capability, credential, source row, header, browser
// state, or financial-apply field.
type BrowserBatchWorkflow struct {
	BatchID                     string                    `json:"batch_id"`
	OwnerID                     string                    `json:"-"`
	SchemaVersion               string                    `json:"schema_version"`
	HistoryFrom                 string                    `json:"history_from"`
	HeaderProbeConsentConfirmed bool                      `json:"header_probe_consent_confirmed"`
	PreparatoryManifestSHA256   string                    `json:"preparatory_manifest_sha256"`
	PreparatoryConsentedAt      time.Time                 `json:"preparatory_consented_at"`
	TransferManifestSHA256      string                    `json:"transfer_manifest_sha256,omitempty"`
	TransferScope               BrowserBatchTransferScope `json:"transfer_scope,omitempty"`
	TransferConfirmedAt         *time.Time                `json:"transfer_confirmed_at,omitempty"`
	CreatedAt                   time.Time                 `json:"created_at"`
	UpdatedAt                   time.Time                 `json:"updated_at"`
}

// BrowserBatchTransferScope is frozen at the second owner confirmation. Every
// serial source capture uses the exact same range and cutoff.
type BrowserBatchTransferScope struct {
	Mode          string   `json:"mode"`
	FromInclusive string   `json:"from_inclusive"`
	ToInclusive   string   `json:"to_inclusive"`
	CutoffAt      string   `json:"cutoff_at"`
	ResourceIDs   []string `json:"resource_ids"`
}

// BrowserBatchSourceWorkflow is safe control-plane state for one paired
// source. Lease fields are optimistic-concurrency control only, never relay
// authorizations.
type BrowserBatchSourceWorkflow struct {
	BatchID         string     `json:"batch_id"`
	SourceCompanyID string     `json:"source_company_id"`
	TenantID        string     `json:"tenant_id"`
	Ordinal         int        `json:"ordinal"`
	Phase           string     `json:"phase"`
	PhaseGeneration int64      `json:"phase_generation"`
	AttemptCount    int        `json:"attempt_count"`
	LeaseID         string     `json:"lease_id,omitempty"`
	LeaseExpiresAt  *time.Time `json:"lease_expires_at,omitempty"`
	// CaptureRunID is safe durable coordination metadata, never a capability.
	// It lets an owner re-issue a short-lived relay token for the same immutable
	// scope after the serial capture lease has expired.
	CaptureRunID            string    `json:"capture_run_id,omitempty"`
	DiscoveryID             string    `json:"discovery_id,omitempty"`
	DiscoveryContractSHA256 string    `json:"discovery_contract_sha256,omitempty"`
	DiscoveryReceiptSHA256  string    `json:"discovery_receipt_sha256,omitempty"`
	SchemaID                string    `json:"schema_id,omitempty"`
	SchemaApprovalSHA256    string    `json:"schema_approval_sha256,omitempty"`
	PackageID               string    `json:"package_id,omitempty"`
	PackageSHA256           string    `json:"package_sha256,omitempty"`
	PreviewID               string    `json:"preview_id,omitempty"`
	PreviewSHA256           string    `json:"preview_sha256,omitempty"`
	ReasonCode              string    `json:"reason_code,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type BrowserBatchWorkflowStatus struct {
	Workflow              BrowserBatchWorkflow         `json:"workflow"`
	Status                string                       `json:"status"`
	SchemaReadinessSHA256 string                       `json:"schema_readiness_sha256,omitempty"`
	NextStep              BrowserBatchWorkflowNextStep `json:"next_step"`
	Sources               []BrowserBatchSourceWorkflow `json:"sources"`
}

// BrowserBatchWorkflowNextStep is safe owner-facing guidance for the one
// server-owned next transition. It includes no source rows, names, amounts,
// digests, credentials, or capabilities. `automatic` only describes a
// non-sensitive phase transition which can be made by AdvanceSafe; it never
// covers a consent, transfer, financial apply, or accountant action.
type BrowserBatchWorkflowNextStep struct {
	Action                             string `json:"action"`
	Automatic                          bool   `json:"automatic"`
	OwnerConfirmationRequired          bool   `json:"owner_confirmation_required"`
	FreshConsentRequired               bool   `json:"fresh_consent_required"`
	FinancialApplyConfirmationRequired bool   `json:"financial_apply_confirmation_required"`
}

type BrowserBatchPreparationRequest struct {
	HistoryFrom                       string `json:"history_from"`
	OwnerConfirmed                    bool   `json:"owner_confirmed"`
	MetadataDiscoveryConsentConfirmed bool   `json:"metadata_discovery_consent_confirmed"`
	HeaderProbeConsentConfirmed       bool   `json:"header_probe_consent_confirmed"`
}

type BrowserBatchTransferConfirmationRequest struct {
	OwnerConfirmed       bool   `json:"owner_confirmed"`
	ExpectedSchemaSHA256 string `json:"expected_schema_sha256"`
}

type BrowserBatchDiscoveryCompletion struct {
	LeaseID                 string `json:"lease_id"`
	PhaseGeneration         int64  `json:"phase_generation"`
	DiscoveryID             string `json:"discovery_id"`
	DiscoveryContractSHA256 string `json:"discovery_contract_sha256"`
	DiscoveryReceiptSHA256  string `json:"discovery_receipt_sha256"`
}

type BrowserBatchSchemaApproval struct {
	PhaseGeneration      int64  `json:"phase_generation"`
	SchemaID             string `json:"schema_id"`
	SchemaApprovalSHA256 string `json:"schema_approval_sha256"`
}

type BrowserBatchStagedPackage struct {
	LeaseID         string `json:"lease_id"`
	PhaseGeneration int64  `json:"phase_generation"`
	PackageID       string `json:"package_id"`
	PackageSHA256   string `json:"package_sha256"`
}

type BrowserBatchPreviewReady struct {
	PhaseGeneration int64  `json:"phase_generation"`
	PreviewID       string `json:"preview_id"`
	PreviewSHA256   string `json:"preview_sha256"`
}

// BrowserBatchOnboardingReader is satisfied by the 081 repository. The 082
// core reads existing immutable selection/pairing state but cannot create a
// tenant or issue a pairing itself.
type BrowserBatchOnboardingReader interface {
	GetBrowserOnboardingBatch(context.Context, string, string) (*BrowserOnboardingBatch, error)
	ListBrowserOnboardingBatchOutcomes(context.Context, string, string) ([]BrowserOnboardingBatchOutcome, error)
}

// BrowserBatchWorkflowStore persists only safe control state. Implementations
// must compare the expected phase, generation, and lease when changing one
// source. No method accepts a relay token or source payload.
type BrowserBatchWorkflowStore interface {
	CreateBrowserBatchWorkflow(context.Context, BrowserBatchWorkflow, []BrowserBatchSourceWorkflow) (*BrowserBatchWorkflow, bool, error)
	GetBrowserBatchWorkflow(context.Context, string, string) (*BrowserBatchWorkflow, error)
	ListBrowserBatchSourceWorkflows(context.Context, string, string) ([]BrowserBatchSourceWorkflow, error)
	CompareAndSwapBrowserBatchSource(context.Context, string, string, string, string, int64, string, BrowserBatchSourceWorkflow) (*BrowserBatchSourceWorkflow, bool, error)
	AcquireNextBrowserBatchLease(context.Context, string, string, string, string, time.Time, time.Time) (*BrowserBatchSourceWorkflow, bool, error)
	RecoverExpiredBrowserBatchLeases(context.Context, string, string, time.Time) (int, error)
	OpenBrowserBatchTransferConfirmation(context.Context, string, string, string, time.Time) (*BrowserBatchWorkflow, bool, error)
	ConfirmBrowserBatchTransfer(context.Context, string, string, string, BrowserBatchTransferScope, time.Time) (*BrowserBatchWorkflow, bool, error)
}

// BrowserBatchWorkflowService owns state transitions only. Integrators later
// translate claimed safe work into action-response relay capabilities; this
// isolated core has no bridge/source/financial dependency.
type BrowserBatchWorkflowService struct {
	store      BrowserBatchWorkflowStore
	onboarding BrowserBatchOnboardingReader
	now        func() time.Time
	newLeaseID func() string
}

func NewBrowserBatchWorkflowService(store BrowserBatchWorkflowStore, onboarding BrowserBatchOnboardingReader) *BrowserBatchWorkflowService {
	return &BrowserBatchWorkflowService{store: store, onboarding: onboarding, now: time.Now, newLeaseID: uuid.NewString}
}

// Prepare creates an immutable non-financial workflow only after all selected
// 081 sources are paired to distinct owner tenants. It does not issue a relay
// capability and moves each source to DISCOVERY_REQUIRED.
func (s *BrowserBatchWorkflowService) Prepare(ctx context.Context, ownerID, batchID string, request BrowserBatchPreparationRequest) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.store == nil || s.onboarding == nil || !validBrowserBatchPreparationRequest(request) || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(batchID) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	batch, err := s.onboarding.GetBrowserOnboardingBatch(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil || batch == nil {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	outcomes, err := s.onboarding.ListBrowserOnboardingBatchOutcomes(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	now := s.currentTime()
	historyFrom, _ := time.Parse(time.DateOnly, strings.TrimSpace(request.HistoryFrom))
	if historyFrom.After(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	workflow, sources, err := newBrowserBatchWorkflow(*batch, outcomes, strings.TrimSpace(ownerID), request, now)
	if err != nil {
		return nil, err
	}
	persisted, created, err := s.store.CreateBrowserBatchWorkflow(ctx, workflow, sources)
	if err != nil || persisted == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	if !created {
		if persisted.PreparatoryManifestSHA256 != workflow.PreparatoryManifestSHA256 || persisted.HistoryFrom != workflow.HistoryFrom {
			return nil, ErrBrowserBatchWorkflowConflict
		}
	}
	storedSources, listErr := s.store.ListBrowserBatchSourceWorkflows(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if listErr != nil || !validBrowserBatchSourceSet(storedSources, strings.TrimSpace(batchID)) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	// A transient process interruption after Create must not strand one source
	// in PAIRED. Retrying the same immutable preparatory manifest only advances
	// that source to DISCOVERY_REQUIRED; later phases remain untouched.
	for _, source := range storedSources {
		if source.Phase != BrowserBatchPhasePaired {
			continue
		}
		next := source
		next.Phase = BrowserBatchPhaseDiscoveryRequired
		next.PhaseGeneration++
		next.UpdatedAt = now
		if _, swapped, swapErr := s.store.CompareAndSwapBrowserBatchSource(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), source.SourceCompanyID, BrowserBatchPhasePaired, source.PhaseGeneration, "", next); swapErr != nil || !swapped {
			return nil, ErrBrowserBatchWorkflowUnavailable
		}
	}
	return s.Status(ctx, ownerID, batchID)
}

func (s *BrowserBatchWorkflowService) Status(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.store == nil || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(batchID) {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	workflow, err := s.store.GetBrowserBatchWorkflow(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil || workflow == nil {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	sources, err := s.store.ListBrowserBatchSourceWorkflows(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil || !validBrowserBatchWorkflow(*workflow) || !validBrowserBatchSourceSet(sources, workflow.BatchID) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	readiness, _ := browserBatchSchemaReadinessSHA256(sources)
	status := browserBatchWorkflowStatus(sources)
	return &BrowserBatchWorkflowStatus{Workflow: *workflow, Status: status, SchemaReadinessSHA256: readiness, NextStep: browserBatchWorkflowNextStep(*workflow, sources, status), Sources: sources}, nil
}

// ClaimNextDiscovery serializes discovery work. The returned lease is safe
// control metadata only; a future handler can exchange it for an ephemeral
// action response but cannot persist that response in this core.
func (s *BrowserBatchWorkflowService) ClaimNextDiscovery(ctx context.Context, ownerID, batchID string) (*BrowserBatchSourceWorkflow, error) {
	return s.claimNext(ctx, ownerID, batchID, BrowserBatchPhaseDiscoveryRequired, BrowserBatchPhaseDiscoveryRunning)
}

// ReissueDiscovery rotates the *control* lease for a still-running discovery
// action. A page/relay restart must not wait for the old ten-minute lease to
// elapse. The source, batch, and preparatory consent stay immutable; only a
// fresh action response may be sent to the relay. Incrementing the phase
// generation makes every late completion carrying the old lease fail closed.
func (s *BrowserBatchWorkflowService) ReissueDiscovery(ctx context.Context, ownerID, batchID, sourceID string) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.store == nil || s.newLeaseID == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	now := s.currentTime()
	if current.Phase != BrowserBatchPhaseDiscoveryRunning || current.LeaseID == "" || current.LeaseExpiresAt == nil || !current.LeaseExpiresAt.After(now) {
		return nil, ErrBrowserBatchWorkflowNotReady
	}
	leaseID := s.newLeaseID()
	if !validBrowserPairingID(leaseID) || leaseID == current.LeaseID {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	next := current
	next.PhaseGeneration++
	next.AttemptCount++
	next.LeaseID = leaseID
	expiresAt := now.Add(browserBatchWorkflowLeaseLifetime)
	next.LeaseExpiresAt = &expiresAt
	next.ReasonCode = ""
	next.UpdatedAt = now
	return s.swap(ctx, ownerID, batchID, current, BrowserBatchPhaseDiscoveryRunning, current.PhaseGeneration, current.LeaseID, next)
}

// CompleteDiscovery records only private receipt digests. It intentionally
// stops at DISCOVERY_COMPLETE so schema review is a distinct auditable phase.
func (s *BrowserBatchWorkflowService) CompleteDiscovery(ctx context.Context, ownerID, batchID, sourceID string, input BrowserBatchDiscoveryCompletion) (*BrowserBatchSourceWorkflow, error) {
	if !validBrowserBatchDiscoveryCompletion(input) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	next := current
	next.Phase = BrowserBatchPhaseDiscoveryComplete
	next.PhaseGeneration++
	next.LeaseID, next.LeaseExpiresAt = "", nil
	next.DiscoveryID = input.DiscoveryID
	next.DiscoveryContractSHA256 = input.DiscoveryContractSHA256
	next.DiscoveryReceiptSHA256 = input.DiscoveryReceiptSHA256
	next.ReasonCode = ""
	next.UpdatedAt = s.currentTime()
	return s.swap(ctx, ownerID, batchID, current, BrowserBatchPhaseDiscoveryRunning, input.PhaseGeneration, input.LeaseID, next)
}

func (s *BrowserBatchWorkflowService) RequireSchemaReview(ctx context.Context, ownerID, batchID, sourceID string, phaseGeneration int64) (*BrowserBatchSourceWorkflow, error) {
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	// This makes the server-owned safe progression resilient to response loss.
	// The phase still does not approve a schema; that needs the separate
	// owner-confirmed schema action.
	if current.Phase == BrowserBatchPhaseSchemaReviewRequired && current.PhaseGeneration == phaseGeneration {
		return &current, nil
	}
	next := current
	next.Phase = BrowserBatchPhaseSchemaReviewRequired
	next.PhaseGeneration++
	next.UpdatedAt = s.currentTime()
	return s.swap(ctx, ownerID, batchID, current, BrowserBatchPhaseDiscoveryComplete, phaseGeneration, "", next)
}

// AdvanceSafe performs at most one idempotent, non-sensitive phase change.
// It cannot issue a browser capability, make a source transfer, create a
// preview, apply financial data, or approve a schema. Keeping this server
// side prevents clients from having to coordinate crash-recovery transitions
// such as DISCOVERY_COMPLETE -> SCHEMA_REVIEW_REQUIRED and the final
// SCHEMA_APPROVED -> TRANSFER_CONFIRMATION_REQUIRED opening.
func (s *BrowserBatchWorkflowService) AdvanceSafe(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.store == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	for attempt := 0; attempt < 3; attempt++ {
		status, err := s.Status(ctx, ownerID, batchID)
		if err != nil {
			return nil, err
		}
		switch status.NextStep.Action {
		case BrowserBatchNextStepAdvanceSchemaReview:
			var source *BrowserBatchSourceWorkflow
			for index := range status.Sources {
				if status.Sources[index].Phase == BrowserBatchPhaseDiscoveryComplete {
					source = &status.Sources[index]
					break
				}
			}
			if source == nil {
				continue
			}
			if _, err := s.RequireSchemaReview(ctx, ownerID, batchID, source.SourceCompanyID, source.PhaseGeneration); err != nil {
				if errors.Is(err, ErrBrowserBatchWorkflowConflict) {
					continue
				}
				return nil, err
			}
		case BrowserBatchNextStepOpenTransferConfirmation:
			if _, err := s.OpenTransferConfirmation(ctx, ownerID, batchID); err != nil {
				if errors.Is(err, ErrBrowserBatchWorkflowConflict) || errors.Is(err, ErrBrowserBatchWorkflowNotReady) {
					continue
				}
				return nil, err
			}
		default:
			return status, nil
		}
	}
	return s.Status(ctx, ownerID, batchID)
}

// RecordSchemaApproval is called only by a later owner-authenticated proxy
// after its exact discovery/adapter review. The core accepts digest metadata,
// not headers or source content.
func (s *BrowserBatchWorkflowService) RecordSchemaApproval(ctx context.Context, ownerID, batchID, sourceID string, input BrowserBatchSchemaApproval) (*BrowserBatchSourceWorkflow, error) {
	if !validBrowserBatchSchemaApproval(input) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	next := current
	next.Phase = BrowserBatchPhaseSchemaApproved
	next.PhaseGeneration++
	next.SchemaID, next.SchemaApprovalSHA256 = input.SchemaID, input.SchemaApprovalSHA256
	next.ReasonCode = ""
	next.UpdatedAt = s.currentTime()
	return s.swap(ctx, ownerID, batchID, current, BrowserBatchPhaseSchemaReviewRequired, input.PhaseGeneration, "", next)
}

// OpenTransferConfirmation advances only when every original selected source
// is schema-approved. This preserves the 081 `all` invariant: a failing
// source cannot silently be omitted from a later transfer.
func (s *BrowserBatchWorkflowService) OpenTransferConfirmation(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	status, err := s.Status(ctx, ownerID, batchID)
	if err != nil {
		return nil, err
	}
	digest, ok := browserBatchSchemaReadinessSHA256(status.Sources)
	if !ok {
		return nil, ErrBrowserBatchWorkflowNotReady
	}
	if _, changed, err := s.store.OpenBrowserBatchTransferConfirmation(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), digest, s.currentTime()); err != nil || !changed {
		if err != nil {
			return nil, ErrBrowserBatchWorkflowUnavailable
		}
		return s.Status(ctx, ownerID, batchID)
	}
	return s.Status(ctx, ownerID, batchID)
}

// ConfirmTransfer freezes one dated partial journal scope and a digest of all
// reviewed source/schema bindings. It issues no browser/bridge capability.
func (s *BrowserBatchWorkflowService) ConfirmTransfer(ctx context.Context, ownerID, batchID string, request BrowserBatchTransferConfirmationRequest) (*BrowserBatchWorkflowStatus, error) {
	if !request.OwnerConfirmed || !validSHA256(request.ExpectedSchemaSHA256) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	status, err := s.Status(ctx, ownerID, batchID)
	if err != nil {
		return nil, err
	}
	readiness, ok := browserBatchSchemaReadinessSHA256(status.Sources)
	if !ok || readiness != request.ExpectedSchemaSHA256 {
		return nil, ErrBrowserBatchWorkflowConflict
	}
	if status.Workflow.TransferManifestSHA256 != "" {
		return s.Status(ctx, ownerID, batchID)
	}
	now := s.currentTime()
	scope := BrowserBatchTransferScope{Mode: "partial", FromInclusive: status.Workflow.HistoryFrom, ToInclusive: now.Format(time.DateOnly), CutoffAt: now.Format(time.RFC3339), ResourceIDs: []string{BrowserGeneralLedgerResourceID}}
	if !validBrowserBatchTransferScope(scope) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	manifest, err := browserBatchTransferManifestSHA256(status.Workflow, status.Sources, readiness, scope)
	if err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	if _, _, err := s.store.ConfirmBrowserBatchTransfer(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), manifest, scope, now); err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.Status(ctx, ownerID, batchID)
}

func (s *BrowserBatchWorkflowService) ClaimNextCapture(ctx context.Context, ownerID, batchID string) (*BrowserBatchSourceWorkflow, error) {
	status, err := s.Status(ctx, ownerID, batchID)
	if err != nil {
		return nil, err
	}
	if status.Workflow.TransferManifestSHA256 == "" {
		return nil, ErrBrowserBatchWorkflowNotReady
	}
	return s.claimNext(ctx, ownerID, batchID, BrowserBatchPhaseTransferConfirmationRequired, BrowserBatchPhaseCaptureRunning)
}

// RecoverExpiredLeases makes a crashed serial action safely retryable. It
// cannot issue a replacement relay capability; a later caller must claim the
// same immutable phase again. Completion carrying the old lease/generation
// then conflicts through CompareAndSwapBrowserBatchSource.
func (s *BrowserBatchWorkflowService) RecoverExpiredLeases(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.store == nil || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(batchID) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	if _, err := s.store.RecoverExpiredBrowserBatchLeases(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), s.currentTime()); err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.Status(ctx, ownerID, batchID)
}

// RecordStagedPackage is a safe receipt-only checkpoint. It neither reads nor
// writes a source and cannot apply a journal.
func (s *BrowserBatchWorkflowService) RecordStagedPackage(ctx context.Context, ownerID, batchID, sourceID string, input BrowserBatchStagedPackage) (*BrowserBatchSourceWorkflow, error) {
	if !validBrowserBatchStagedPackage(input) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	next := current
	next.Phase = BrowserBatchPhaseStaged
	next.PhaseGeneration++
	next.LeaseID, next.LeaseExpiresAt = "", nil
	next.PackageID, next.PackageSHA256 = input.PackageID, input.PackageSHA256
	next.ReasonCode = ""
	next.UpdatedAt = s.currentTime()
	return s.swap(ctx, ownerID, batchID, current, BrowserBatchPhaseCaptureRunning, input.PhaseGeneration, input.LeaseID, next)
}

func (s *BrowserBatchWorkflowService) RecordPreviewReady(ctx context.Context, ownerID, batchID, sourceID string, input BrowserBatchPreviewReady) (*BrowserBatchSourceWorkflow, error) {
	if !validBrowserBatchPreviewReady(input) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	next := current
	next.Phase = BrowserBatchPhasePreviewReady
	next.PhaseGeneration++
	next.PreviewID, next.PreviewSHA256 = input.PreviewID, input.PreviewSHA256
	next.ReasonCode = ""
	next.UpdatedAt = s.currentTime()
	return s.swap(ctx, ownerID, batchID, current, BrowserBatchPhaseStaged, input.PhaseGeneration, "", next)
}

// MarkReviewRequired captures only a fixed reason code. It is intentionally
// not a route to financial apply and cannot carry an untrusted error body.
func (s *BrowserBatchWorkflowService) MarkReviewRequired(ctx context.Context, ownerID, batchID, sourceID, expectedPhase string, generation int64, reasonCode string) (*BrowserBatchSourceWorkflow, error) {
	if !validBrowserBatchPhase(expectedPhase) || !validBrowserBatchReasonCode(reasonCode) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	current, err := s.source(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	next := current
	next.Phase = BrowserBatchPhaseReviewRequired
	next.PhaseGeneration++
	next.LeaseID, next.LeaseExpiresAt = "", nil
	next.ReasonCode = reasonCode
	next.UpdatedAt = s.currentTime()
	return s.swap(ctx, ownerID, batchID, current, expectedPhase, generation, current.LeaseID, next)
}

func (s *BrowserBatchWorkflowService) claimNext(ctx context.Context, ownerID, batchID, requiredPhase, runningPhase string) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.store == nil || s.newLeaseID == nil || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(batchID) || !validBrowserBatchPhase(requiredPhase) || !validBrowserBatchPhase(runningPhase) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	now := s.currentTime()
	if _, err := s.store.RecoverExpiredBrowserBatchLeases(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), now); err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	leaseID := s.newLeaseID()
	if !validBrowserPairingID(leaseID) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	source, acquired, err := s.store.AcquireNextBrowserBatchLease(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), requiredPhase, leaseID, now, now.Add(browserBatchWorkflowLeaseLifetime))
	if err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	if !acquired || source == nil {
		return nil, ErrBrowserBatchWorkflowNotReady
	}
	if source.Phase != runningPhase || source.LeaseID != leaseID || source.LeaseExpiresAt == nil || !source.LeaseExpiresAt.After(now) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return source, nil
}

func (s *BrowserBatchWorkflowService) source(ctx context.Context, ownerID, batchID, sourceID string) (BrowserBatchSourceWorkflow, error) {
	if s == nil || s.store == nil || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(batchID) || !validBrowserSourceCompanyID(sourceID) {
		return BrowserBatchSourceWorkflow{}, ErrBrowserBatchWorkflowInvalid
	}
	sources, err := s.store.ListBrowserBatchSourceWorkflows(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID))
	if err != nil {
		return BrowserBatchSourceWorkflow{}, ErrBrowserBatchWorkflowUnavailable
	}
	for _, source := range sources {
		if source.SourceCompanyID == strings.TrimSpace(sourceID) {
			return source, nil
		}
	}
	return BrowserBatchSourceWorkflow{}, ErrBrowserBatchWorkflowNotFound
}

func (s *BrowserBatchWorkflowService) swap(ctx context.Context, ownerID, batchID string, current BrowserBatchSourceWorkflow, expectedPhase string, generation int64, leaseID string, next BrowserBatchSourceWorkflow) (*BrowserBatchSourceWorkflow, error) {
	if current.Phase != expectedPhase || current.PhaseGeneration != generation || current.LeaseID != leaseID || !validBrowserBatchSourceWorkflow(next) {
		return nil, ErrBrowserBatchWorkflowConflict
	}
	persisted, swapped, err := s.store.CompareAndSwapBrowserBatchSource(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(batchID), current.SourceCompanyID, expectedPhase, generation, leaseID, next)
	if err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	if !swapped || persisted == nil {
		return nil, ErrBrowserBatchWorkflowConflict
	}
	return persisted, nil
}

func newBrowserBatchWorkflow(batch BrowserOnboardingBatch, outcomes []BrowserOnboardingBatchOutcome, ownerID string, request BrowserBatchPreparationRequest, now time.Time) (BrowserBatchWorkflow, []BrowserBatchSourceWorkflow, error) {
	if !validBrowserOnboardingBatch(batch) || batch.OwnerID != strings.TrimSpace(ownerID) || !validBrowserBatchPreparationRequest(request) || now.IsZero() {
		return BrowserBatchWorkflow{}, nil, ErrBrowserBatchWorkflowInvalid
	}
	sources, ok := browserBatchPairedSources(batch, outcomes, now)
	if !ok {
		return BrowserBatchWorkflow{}, nil, ErrBrowserBatchWorkflowNotReady
	}
	digest, err := browserBatchPreparatoryManifestSHA256(batch, sources, request)
	if err != nil {
		return BrowserBatchWorkflow{}, nil, ErrBrowserBatchWorkflowUnavailable
	}
	workflow := BrowserBatchWorkflow{BatchID: batch.ID, OwnerID: batch.OwnerID, SchemaVersion: BrowserBatchWorkflowSchemaVersion, HistoryFrom: request.HistoryFrom, HeaderProbeConsentConfirmed: request.HeaderProbeConsentConfirmed, PreparatoryManifestSHA256: digest, PreparatoryConsentedAt: now.UTC(), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	if !validBrowserBatchWorkflow(workflow) {
		return BrowserBatchWorkflow{}, nil, ErrBrowserBatchWorkflowInvalid
	}
	return workflow, sources, nil
}

func browserBatchPairedSources(batch BrowserOnboardingBatch, outcomes []BrowserOnboardingBatchOutcome, now time.Time) ([]BrowserBatchSourceWorkflow, bool) {
	if len(batch.SelectedSources) == 0 || len(batch.SelectedSources) != len(outcomes) {
		return nil, false
	}
	bySource := make(map[string]BrowserOnboardingBatchOutcome, len(outcomes))
	seenTenants := make(map[string]struct{}, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.Status != BrowserOnboardingPaired || !validBrowserSourceCompanyID(outcome.SourceCompanyID) || !safeBridgeID(outcome.TenantID) || outcome.TenantID == "" {
			return nil, false
		}
		if _, duplicate := bySource[outcome.SourceCompanyID]; duplicate {
			return nil, false
		}
		if _, duplicate := seenTenants[outcome.TenantID]; duplicate {
			return nil, false
		}
		bySource[outcome.SourceCompanyID] = outcome
		seenTenants[outcome.TenantID] = struct{}{}
	}
	sources := make([]BrowserBatchSourceWorkflow, 0, len(batch.SelectedSources))
	for ordinal, selected := range batch.SelectedSources {
		outcome, found := bySource[selected.SourceCompanyID]
		if !found || selected.SourceCompanyName != outcome.SourceCompanyName {
			return nil, false
		}
		sources = append(sources, BrowserBatchSourceWorkflow{BatchID: batch.ID, SourceCompanyID: selected.SourceCompanyID, TenantID: outcome.TenantID, Ordinal: ordinal, Phase: BrowserBatchPhasePaired, CreatedAt: now.UTC(), UpdatedAt: now.UTC()})
	}
	return sources, validBrowserBatchSourceSet(sources, batch.ID)
}

func browserBatchPreparatoryManifestSHA256(batch BrowserOnboardingBatch, sources []BrowserBatchSourceWorkflow, request BrowserBatchPreparationRequest) (string, error) {
	bound := make([]struct {
		SourceCompanyID string `json:"source_company_id"`
		TenantID        string `json:"tenant_id"`
		Ordinal         int    `json:"ordinal"`
	}, 0, len(sources))
	for _, source := range sources {
		bound = append(bound, struct {
			SourceCompanyID string `json:"source_company_id"`
			TenantID        string `json:"tenant_id"`
			Ordinal         int    `json:"ordinal"`
		}{source.SourceCompanyID, source.TenantID, source.Ordinal})
	}
	return browserBatchDigest(struct {
		Version                  string `json:"version"`
		OnboardingManifestSHA256 string `json:"onboarding_manifest_sha256"`
		ObservedSourcesSHA256    string `json:"observed_sources_sha256"`
		Mode                     string `json:"mode"`
		HistoryFrom              string `json:"history_from"`
		MetadataDiscovery        bool   `json:"metadata_discovery"`
		HeaderProbe              bool   `json:"header_probe"`
		Sources                  any    `json:"sources"`
	}{BrowserBatchWorkflowSchemaVersion, batch.ManifestSHA256, batch.ObservedSourcesSHA256, batch.Mode, request.HistoryFrom, request.MetadataDiscoveryConsentConfirmed, request.HeaderProbeConsentConfirmed, bound})
}

func browserBatchSchemaReadinessSHA256(sources []BrowserBatchSourceWorkflow) (string, bool) {
	if len(sources) == 0 {
		return "", false
	}
	bound := append([]BrowserBatchSourceWorkflow(nil), sources...)
	sort.Slice(bound, func(i, j int) bool { return bound[i].Ordinal < bound[j].Ordinal })
	projection := make([]struct {
		SourceCompanyID         string `json:"source_company_id"`
		TenantID                string `json:"tenant_id"`
		Ordinal                 int    `json:"ordinal"`
		DiscoveryID             string `json:"discovery_id"`
		DiscoveryContractSHA256 string `json:"discovery_contract_sha256"`
		DiscoveryReceiptSHA256  string `json:"discovery_receipt_sha256"`
		SchemaID                string `json:"schema_id"`
		SchemaApprovalSHA256    string `json:"schema_approval_sha256"`
	}, 0, len(bound))
	for _, source := range bound {
		if !browserBatchPhaseRetainsSchemaBinding(source.Phase) || !validBrowserSourceCompanyID(source.SourceCompanyID) || !safeBridgeID(source.TenantID) || !validBrowserPairingID(source.DiscoveryID) || !validSHA256(source.DiscoveryContractSHA256) || !validSHA256(source.DiscoveryReceiptSHA256) || source.SchemaID != BrowserGeneralLedgerCSVSchemaID || !validSHA256(source.SchemaApprovalSHA256) {
			return "", false
		}
		projection = append(projection, struct {
			SourceCompanyID         string `json:"source_company_id"`
			TenantID                string `json:"tenant_id"`
			Ordinal                 int    `json:"ordinal"`
			DiscoveryID             string `json:"discovery_id"`
			DiscoveryContractSHA256 string `json:"discovery_contract_sha256"`
			DiscoveryReceiptSHA256  string `json:"discovery_receipt_sha256"`
			SchemaID                string `json:"schema_id"`
			SchemaApprovalSHA256    string `json:"schema_approval_sha256"`
		}{source.SourceCompanyID, source.TenantID, source.Ordinal, source.DiscoveryID, source.DiscoveryContractSHA256, source.DiscoveryReceiptSHA256, source.SchemaID, source.SchemaApprovalSHA256})
	}
	digest, err := browserBatchDigest(struct {
		Version string `json:"version"`
		Sources any    `json:"sources"`
	}{BrowserBatchWorkflowSchemaVersion, projection})
	return digest, err == nil
}

func browserBatchPhaseRetainsSchemaBinding(phase string) bool {
	switch phase {
	case BrowserBatchPhaseSchemaApproved, BrowserBatchPhaseTransferConfirmationRequired, BrowserBatchPhaseCaptureRunning, BrowserBatchPhaseStaged, BrowserBatchPhasePreviewReady:
		return true
	default:
		return false
	}
}

func browserBatchTransferManifestSHA256(workflow BrowserBatchWorkflow, sources []BrowserBatchSourceWorkflow, readiness string, scope BrowserBatchTransferScope) (string, error) {
	return browserBatchDigest(struct {
		Version                   string                       `json:"version"`
		PreparatoryManifestSHA256 string                       `json:"preparatory_manifest_sha256"`
		SchemaReadinessSHA256     string                       `json:"schema_readiness_sha256"`
		Scope                     BrowserBatchTransferScope    `json:"scope"`
		LedgerAuthority           string                       `json:"ledger_authority"`
		Sources                   []BrowserBatchSourceWorkflow `json:"sources"`
	}{BrowserBatchWorkflowSchemaVersion, workflow.PreparatoryManifestSHA256, readiness, scope, "smartaccounts_gl_authoritative", canonicalBrowserBatchSourcesForDigest(sources)})
}

func canonicalBrowserBatchSourcesForDigest(input []BrowserBatchSourceWorkflow) []BrowserBatchSourceWorkflow {
	result := append([]BrowserBatchSourceWorkflow(nil), input...)
	sort.Slice(result, func(i, j int) bool { return result[i].Ordinal < result[j].Ordinal })
	for index := range result {
		result[index].LeaseID, result[index].LeaseExpiresAt = "", nil
		result[index].CaptureRunID = ""
		result[index].PackageID, result[index].PackageSHA256 = "", ""
		result[index].PreviewID, result[index].PreviewSHA256 = "", ""
		result[index].ReasonCode = ""
		result[index].CreatedAt, result[index].UpdatedAt = time.Time{}, time.Time{}
	}
	return result
}

func browserBatchDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func browserBatchWorkflowStatus(sources []BrowserBatchSourceWorkflow) string {
	if len(sources) == 0 {
		return BrowserBatchPhaseBlocked
	}
	all := func(phase string) bool {
		for _, source := range sources {
			if source.Phase != phase {
				return false
			}
		}
		return true
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseReviewRequired || source.Phase == BrowserBatchPhaseBlocked {
			return BrowserBatchPhaseReviewRequired
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseFailedRetryable {
			return BrowserBatchPhaseFailedRetryable
		}
	}
	if all(BrowserBatchPhasePreviewReady) {
		return BrowserBatchPhasePreviewReady
	}
	allStagedOrPreviewReady := true
	for _, source := range sources {
		if source.Phase != BrowserBatchPhaseStaged && source.Phase != BrowserBatchPhasePreviewReady {
			allStagedOrPreviewReady = false
			break
		}
	}
	if allStagedOrPreviewReady {
		return BrowserBatchPhaseStaged
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseCaptureRunning {
			return BrowserBatchPhaseCaptureRunning
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseTransferConfirmationRequired {
			return BrowserBatchPhaseTransferConfirmationRequired
		}
	}
	if all(BrowserBatchPhaseSchemaApproved) {
		return BrowserBatchPhaseSchemaApproved
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseSchemaReviewRequired {
			return BrowserBatchPhaseSchemaReviewRequired
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseDiscoveryRunning {
			return BrowserBatchPhaseDiscoveryRunning
		}
	}
	if all(BrowserBatchPhaseDiscoveryComplete) {
		return BrowserBatchPhaseDiscoveryComplete
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseDiscoveryComplete {
			return BrowserBatchPhaseDiscoveryRequired
		}
	}
	if all(BrowserBatchPhasePaired) {
		return BrowserBatchPhasePaired
	}
	return BrowserBatchPhaseDiscoveryRequired
}

func browserBatchWorkflowNextStep(workflow BrowserBatchWorkflow, sources []BrowserBatchSourceWorkflow, status string) BrowserBatchWorkflowNextStep {
	// A review/blocker takes precedence over every convenience transition. The
	// projection is intentionally conservative: a caller must resolve the
	// listed review before a later safe phase can be advanced.
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseReviewRequired {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepResolveReview, OwnerConfirmationRequired: true}
		}
		if source.Phase == BrowserBatchPhaseBlocked {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepResolveBlocker, OwnerConfirmationRequired: true}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseFailedRetryable {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepRecoverRetryable, OwnerConfirmationRequired: true}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseDiscoveryRunning {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepAwaitDiscovery}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseDiscoveryComplete {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepAdvanceSchemaReview, Automatic: true}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseSchemaReviewRequired {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepConfirmSchemaReview, OwnerConfirmationRequired: true}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseDiscoveryRequired || source.Phase == BrowserBatchPhasePaired {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepAcquireDiscovery, OwnerConfirmationRequired: true, FreshConsentRequired: true}
		}
	}
	allSchemaApproved := len(sources) > 0
	for _, source := range sources {
		if source.Phase != BrowserBatchPhaseSchemaApproved {
			allSchemaApproved = false
			break
		}
	}
	if allSchemaApproved && workflow.TransferManifestSHA256 == "" {
		return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepOpenTransferConfirmation, Automatic: true}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseCaptureRunning {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepAwaitStaging}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseStaged {
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepPreparePreview, OwnerConfirmationRequired: true}
		}
	}
	for _, source := range sources {
		if source.Phase == BrowserBatchPhaseTransferConfirmationRequired {
			if workflow.TransferManifestSHA256 == "" {
				return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepConfirmTransferScope, OwnerConfirmationRequired: true}
			}
			return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepAcquireCapture, OwnerConfirmationRequired: true, FreshConsentRequired: true}
		}
	}
	if status == BrowserBatchPhasePreviewReady {
		return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepReviewFinancialPreview, OwnerConfirmationRequired: true, FinancialApplyConfirmationRequired: true}
	}
	return BrowserBatchWorkflowNextStep{Action: BrowserBatchNextStepResolveBlocker, OwnerConfirmationRequired: true}
}

func validBrowserBatchPreparationRequest(request BrowserBatchPreparationRequest) bool {
	if !request.OwnerConfirmed || !request.MetadataDiscoveryConsentConfirmed {
		return false
	}
	_, err := time.Parse(time.DateOnly, strings.TrimSpace(request.HistoryFrom))
	return err == nil
}

func validBrowserBatchWorkflow(workflow BrowserBatchWorkflow) bool {
	if !validBrowserPairingID(workflow.BatchID) || strings.TrimSpace(workflow.OwnerID) == "" || workflow.SchemaVersion != BrowserBatchWorkflowSchemaVersion || !validSHA256(workflow.PreparatoryManifestSHA256) || workflow.PreparatoryConsentedAt.IsZero() || workflow.CreatedAt.IsZero() || workflow.UpdatedAt.IsZero() {
		return false
	}
	if _, err := time.Parse(time.DateOnly, workflow.HistoryFrom); err != nil {
		return false
	}
	if workflow.TransferManifestSHA256 == "" {
		return workflow.TransferConfirmedAt == nil && workflow.TransferScope.Mode == "" && workflow.TransferScope.FromInclusive == "" && workflow.TransferScope.ToInclusive == "" && workflow.TransferScope.CutoffAt == "" && len(workflow.TransferScope.ResourceIDs) == 0
	}
	return validSHA256(workflow.TransferManifestSHA256) && workflow.TransferConfirmedAt != nil && validBrowserBatchTransferScope(workflow.TransferScope)
}

func validBrowserBatchSourceSet(sources []BrowserBatchSourceWorkflow, batchID string) bool {
	if len(sources) == 0 || len(sources) > BrowserOnboardingMaxSources {
		return false
	}
	seenSources, seenTenants, seenOrdinals := map[string]struct{}{}, map[string]struct{}{}, map[int]struct{}{}
	for _, source := range sources {
		if source.BatchID != batchID || !validBrowserBatchSourceWorkflow(source) {
			return false
		}
		if _, found := seenSources[source.SourceCompanyID]; found {
			return false
		}
		if _, found := seenTenants[source.TenantID]; found {
			return false
		}
		if _, found := seenOrdinals[source.Ordinal]; found {
			return false
		}
		seenSources[source.SourceCompanyID], seenTenants[source.TenantID], seenOrdinals[source.Ordinal] = struct{}{}, struct{}{}, struct{}{}
	}
	return true
}

func validBrowserBatchSourceWorkflow(source BrowserBatchSourceWorkflow) bool {
	if !validBrowserPairingID(source.BatchID) || !validBrowserSourceCompanyID(source.SourceCompanyID) || !safeBridgeID(source.TenantID) || source.Ordinal < 0 || source.Ordinal >= BrowserOnboardingMaxSources || !validBrowserBatchPhase(source.Phase) || source.PhaseGeneration < 0 || source.AttemptCount < 0 || !validBrowserBatchReasonCode(source.ReasonCode) || source.CreatedAt.IsZero() || source.UpdatedAt.IsZero() {
		return false
	}
	if source.LeaseID == "" {
		if source.LeaseExpiresAt != nil {
			return false
		}
	} else if !validBrowserPairingID(source.LeaseID) || source.LeaseExpiresAt == nil {
		return false
	}
	if source.CaptureRunID != "" && !validBrowserPairingID(source.CaptureRunID) {
		return false
	}
	if source.DiscoveryID == "" {
		if source.DiscoveryContractSHA256 != "" || source.DiscoveryReceiptSHA256 != "" {
			return false
		}
	} else if !validBrowserPairingID(source.DiscoveryID) || !validSHA256(source.DiscoveryContractSHA256) || !validSHA256(source.DiscoveryReceiptSHA256) {
		return false
	}
	if source.SchemaID == "" {
		if source.SchemaApprovalSHA256 != "" {
			return false
		}
	} else if !validBrowserBatchSchemaID(source.SchemaID) || !validSHA256(source.SchemaApprovalSHA256) {
		return false
	}
	if source.PackageID == "" {
		if source.PackageSHA256 != "" {
			return false
		}
	} else if !safeBridgeID(source.PackageID) || !validSHA256(source.PackageSHA256) {
		return false
	}
	if source.PreviewID == "" {
		if source.PreviewSHA256 != "" {
			return false
		}
	} else if !validBrowserPairingID(source.PreviewID) || !validSHA256(source.PreviewSHA256) {
		return false
	}
	if (source.Phase == BrowserBatchPhaseDiscoveryRunning || source.Phase == BrowserBatchPhaseCaptureRunning) != (source.LeaseID != "") {
		return false
	}
	if browserBatchPhaseRequiresDiscovery(source.Phase) && source.DiscoveryID == "" {
		return false
	}
	if browserBatchPhaseRequiresSchema(source.Phase) && source.SchemaID == "" {
		return false
	}
	if browserBatchPhaseRequiresPackage(source.Phase) && source.PackageID == "" {
		return false
	}
	if browserBatchPhaseRequiresCaptureRun(source.Phase) && source.CaptureRunID == "" {
		return false
	}
	if source.Phase == BrowserBatchPhasePreviewReady && source.PreviewID == "" {
		return false
	}
	return true
}

func browserBatchPhaseRequiresDiscovery(phase string) bool {
	switch phase {
	case BrowserBatchPhaseDiscoveryComplete, BrowserBatchPhaseSchemaReviewRequired, BrowserBatchPhaseSchemaApproved, BrowserBatchPhaseTransferConfirmationRequired, BrowserBatchPhaseCaptureRunning, BrowserBatchPhaseStaged, BrowserBatchPhasePreviewReady:
		return true
	default:
		return false
	}
}

func browserBatchPhaseRequiresSchema(phase string) bool {
	switch phase {
	case BrowserBatchPhaseSchemaApproved, BrowserBatchPhaseTransferConfirmationRequired, BrowserBatchPhaseCaptureRunning, BrowserBatchPhaseStaged, BrowserBatchPhasePreviewReady:
		return true
	default:
		return false
	}
}

func browserBatchPhaseRequiresPackage(phase string) bool {
	return phase == BrowserBatchPhaseStaged || phase == BrowserBatchPhasePreviewReady
}

func browserBatchPhaseRequiresCaptureRun(phase string) bool {
	switch phase {
	case BrowserBatchPhaseCaptureRunning, BrowserBatchPhaseStaged, BrowserBatchPhasePreviewReady:
		return true
	default:
		return false
	}
}

func validBrowserBatchPhase(phase string) bool {
	switch phase {
	case BrowserBatchPhasePaired, BrowserBatchPhaseDiscoveryRequired, BrowserBatchPhaseDiscoveryRunning, BrowserBatchPhaseDiscoveryComplete, BrowserBatchPhaseSchemaReviewRequired, BrowserBatchPhaseSchemaApproved, BrowserBatchPhaseTransferConfirmationRequired, BrowserBatchPhaseCaptureRunning, BrowserBatchPhaseStaged, BrowserBatchPhasePreviewReady, BrowserBatchPhaseReviewRequired, BrowserBatchPhaseFailedRetryable, BrowserBatchPhaseBlocked:
		return true
	default:
		return false
	}
}

func validBrowserBatchTransferScope(scope BrowserBatchTransferScope) bool {
	if scope.Mode != "partial" || !validCaptureDateRange(scope.FromInclusive, scope.ToInclusive) || !validRFC3339(scope.CutoffAt) || len(scope.ResourceIDs) != 1 || scope.ResourceIDs[0] != BrowserGeneralLedgerResourceID {
		return false
	}
	cutoff, err := time.Parse(time.RFC3339, scope.CutoffAt)
	return err == nil && cutoff.UTC().Format(time.DateOnly) == scope.ToInclusive
}

func validBrowserBatchDiscoveryCompletion(input BrowserBatchDiscoveryCompletion) bool {
	return validBrowserPairingID(input.LeaseID) && input.PhaseGeneration > 0 && validBrowserPairingID(input.DiscoveryID) && validSHA256(input.DiscoveryContractSHA256) && validSHA256(input.DiscoveryReceiptSHA256)
}

func validBrowserBatchSchemaApproval(input BrowserBatchSchemaApproval) bool {
	return input.PhaseGeneration > 0 && strings.TrimSpace(input.SchemaID) == BrowserGeneralLedgerCSVSchemaID && validSHA256(input.SchemaApprovalSHA256)
}

func validBrowserBatchStagedPackage(input BrowserBatchStagedPackage) bool {
	return validBrowserPairingID(input.LeaseID) && input.PhaseGeneration > 0 && safeBridgeID(input.PackageID) && validSHA256(input.PackageSHA256)
}

func validBrowserBatchPreviewReady(input BrowserBatchPreviewReady) bool {
	return input.PhaseGeneration > 0 && validBrowserPairingID(input.PreviewID) && validSHA256(input.PreviewSHA256)
}

func validBrowserBatchSchemaID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > 120 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-' || character == '/' {
			continue
		}
		return false
	}
	return true
}

func validBrowserBatchReasonCode(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if len(value) > 120 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func (s *BrowserBatchWorkflowService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
