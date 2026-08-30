package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrBrowserBatchPreviewReviewRequired preserves an executor preview as a
// receipt while stopping the batch for human review. It is not a financial
// apply signal.
var ErrBrowserBatchPreviewReviewRequired = errors.New("SmartAccounts browser batch preview requires review")

// BrowserBatchDiscoveryActions narrows the existing discovery service to the
// only operations batch orchestration needs. Implementations must not persist
// relay events outside the existing discovery service.
type BrowserBatchDiscoveryActions interface {
	Issue(context.Context, string, string, BrowserDiscoveryStartRequest) (*BrowserDiscoveryIssue, error)
	Receive(context.Context, string, string, BrowserDiscoveryRelayResult) (BrowserDiscoveryReceipt, error)
}

// BrowserBatchSchemaActions narrows the reviewed-schema registry. The batch
// never receives headers, source values, audit identifiers, or bridge tokens.
type BrowserBatchSchemaActions interface {
	Review(context.Context, string, string, string, string, string, BrowserCSVSchemaApprovalRequest) (BrowserCSVSchemaApprovalResponse, bool, error)
	Status(context.Context, string, string, string, string) (BrowserCSVSchemaApprovalResponse, error)
}

// BrowserBatchCaptureActions is deliberately limited to a persisted safe run
// identity, action-response capability issuance, and owner-safe progress.
type BrowserBatchCaptureActions interface {
	IssueForRun(context.Context, string, string, string, BrowserCaptureStartRequest) (*BrowserCaptureIssue, error)
	OwnerStatus(context.Context, string, string) (BrowserCaptureStatus, error)
}

// BrowserBatchPreviewReceipt is the only executor projection stored in the
// workflow. It permits a later explicit tenant preview/apply decision without
// placing planned lines or source records in batch control state.
type BrowserBatchPreviewReceipt struct {
	PreviewID     string `json:"preview_id"`
	PreviewSHA256 string `json:"preview_sha256"`
	Status        string `json:"status"`
}

// BrowserBatchPreviewActions adapts the existing staged-package planner. It
// cannot apply financial writes.
type BrowserBatchPreviewActions interface {
	Preview(context.Context, string, string, string, bool) (BrowserBatchPreviewReceipt, error)
}

// BrowserBatchWorkflowActionsService is the owner-only composition layer over
// the pure 082 phase machine. It consumes only already-safe receipts from the
// existing discovery, review, capture, staging, and preview services.
type BrowserBatchWorkflowActionsService struct {
	workflow  *BrowserBatchWorkflowService
	discovery BrowserBatchDiscoveryActions
	schemas   BrowserBatchSchemaActions
	captures  BrowserBatchCaptureActions
	preview   BrowserBatchPreviewActions
	now       func() time.Time
}

func NewBrowserBatchWorkflowActionsService(workflow *BrowserBatchWorkflowService, discovery BrowserBatchDiscoveryActions, schemas BrowserBatchSchemaActions, captures BrowserBatchCaptureActions, preview BrowserBatchPreviewActions) *BrowserBatchWorkflowActionsService {
	return &BrowserBatchWorkflowActionsService{workflow: workflow, discovery: discovery, schemas: schemas, captures: captures, preview: preview, now: time.Now}
}

type BrowserBatchDiscoveryAcquireRequest struct {
	MetadataOnlyConsentConfirmed bool `json:"metadata_only_consent_confirmed"`
	ResponseHeaderProbeConfirmed bool `json:"response_header_probe_confirmed"`
}

// BrowserBatchDiscoveryAction is action-response-only. The discovery issue
// has no relay capability; it is still omitted from GET/status so an old page
// event cannot be replayed as current work.
type BrowserBatchDiscoveryAction struct {
	Source    BrowserBatchSourceWorkflow `json:"source"`
	Discovery BrowserDiscoveryIssue      `json:"discovery"`
}

type BrowserBatchDiscoveryCompleteRequest struct {
	LeaseID         string                      `json:"lease_id"`
	PhaseGeneration int64                       `json:"phase_generation"`
	DiscoveryID     string                      `json:"discovery_id"`
	Result          BrowserDiscoveryRelayResult `json:"result"`
}

type BrowserBatchSchemaPhaseRequest struct {
	PhaseGeneration int64 `json:"phase_generation"`
}

type BrowserBatchSchemaConfirmRequest struct {
	PhaseGeneration int64 `json:"phase_generation"`
	ReviewConfirmed bool  `json:"review_confirmed"`
}

type BrowserBatchCaptureAcquireRequest struct {
	TransferConsentConfirmed bool `json:"transfer_consent_confirmed"`
}

// BrowserBatchCaptureAction carries an existing short-lived relay capability
// only in the immediate owner action response. It is never stored by this
// service and never present in safe workflow status.
type BrowserBatchCaptureAction struct {
	Source  BrowserBatchSourceWorkflow `json:"source"`
	Capture BrowserCaptureIssue        `json:"capture"`
}

type BrowserBatchCaptureCompleteRequest struct {
	LeaseID         string `json:"lease_id"`
	PhaseGeneration int64  `json:"phase_generation"`
}

type BrowserBatchCaptureCompletion struct {
	Source   BrowserBatchSourceWorkflow `json:"source"`
	Progress BrowserCaptureStatus       `json:"progress"`
}

type BrowserBatchPreviewRequest struct {
	PhaseGeneration int64 `json:"phase_generation"`
	UseSourceChart  bool  `json:"use_source_chart"`
}

func (s *BrowserBatchWorkflowActionsService) Prepare(ctx context.Context, ownerID, batchID string, request BrowserBatchPreparationRequest) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.Prepare(ctx, ownerID, batchID, request)
}

func (s *BrowserBatchWorkflowActionsService) Status(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.Status(ctx, ownerID, batchID)
}

func (s *BrowserBatchWorkflowActionsService) Resume(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.RecoverExpiredLeases(ctx, ownerID, batchID)
}

// AdvanceSafe is an owner-safe, idempotent recovery aid for phase changes
// which need neither a fresh consent nor a reviewed/financial decision. It
// never issues a browser capability. In particular it cannot confirm source
// transfer, acquire capture, create a preview, apply financial data, or
// approve a tolerance/accountant decision.
func (s *BrowserBatchWorkflowActionsService) AdvanceSafe(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.AdvanceSafe(ctx, ownerID, batchID)
}

func (s *BrowserBatchWorkflowActionsService) AcquireDiscovery(ctx context.Context, ownerID, batchID, actor string, request BrowserBatchDiscoveryAcquireRequest) (*BrowserBatchDiscoveryAction, error) {
	if s == nil || s.workflow == nil || s.discovery == nil || !request.MetadataOnlyConsentConfirmed {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	workflow, err := s.workflow.Status(ctx, ownerID, batchID)
	if err != nil || workflow.Workflow.HeaderProbeConsentConfirmed != request.ResponseHeaderProbeConfirmed {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	source, err := s.workflow.ClaimNextDiscovery(ctx, ownerID, batchID)
	if err != nil {
		return nil, err
	}
	issue, err := s.discovery.Issue(ctx, source.TenantID, actor, BrowserDiscoveryStartRequest{
		SourceCompanyID:              source.SourceCompanyID,
		MetadataOnlyConsentConfirmed: true,
		ResponseHeaderProbeConfirmed: request.ResponseHeaderProbeConfirmed,
	})
	if err != nil || issue == nil || issue.TenantID != source.TenantID || issue.SourceCompanyID != source.SourceCompanyID || !validBrowserDiscoveryID(issue.DiscoveryID) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return &BrowserBatchDiscoveryAction{Source: *source, Discovery: *issue}, nil
}

// ReissueDiscovery rotates a lost page/relay action for the exact running
// source. It requires fresh owner consent and deliberately issues a new
// discovery receipt binding; stale completion requests are rejected by the
// new lease and generation.
func (s *BrowserBatchWorkflowActionsService) ReissueDiscovery(ctx context.Context, ownerID, batchID, sourceID, actor string, request BrowserBatchDiscoveryAcquireRequest) (*BrowserBatchDiscoveryAction, error) {
	if s == nil || s.workflow == nil || s.discovery == nil || !request.MetadataOnlyConsentConfirmed {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	workflow, err := s.workflow.Status(ctx, ownerID, batchID)
	if err != nil || workflow.Workflow.HeaderProbeConsentConfirmed != request.ResponseHeaderProbeConfirmed {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	source, err := s.workflow.ReissueDiscovery(ctx, ownerID, batchID, sourceID)
	if err != nil {
		return nil, err
	}
	issue, err := s.discovery.Issue(ctx, source.TenantID, actor, BrowserDiscoveryStartRequest{
		SourceCompanyID:              source.SourceCompanyID,
		MetadataOnlyConsentConfirmed: true,
		ResponseHeaderProbeConfirmed: request.ResponseHeaderProbeConfirmed,
	})
	if err != nil || issue == nil || issue.TenantID != source.TenantID || issue.SourceCompanyID != source.SourceCompanyID || !validBrowserDiscoveryID(issue.DiscoveryID) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return &BrowserBatchDiscoveryAction{Source: *source, Discovery: *issue}, nil
}

func (s *BrowserBatchWorkflowActionsService) CompleteDiscovery(ctx context.Context, ownerID, batchID, sourceID string, request BrowserBatchDiscoveryCompleteRequest) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.workflow == nil || s.discovery == nil || request.Result.DiscoveryID != strings.TrimSpace(request.DiscoveryID) {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	source, err := s.currentSource(ctx, ownerID, batchID, sourceID)
	if err != nil || source.Phase != BrowserBatchPhaseDiscoveryRunning || source.LeaseID != request.LeaseID || source.PhaseGeneration != request.PhaseGeneration {
		if err != nil {
			return nil, err
		}
		return nil, ErrBrowserBatchWorkflowConflict
	}
	receipt, err := s.discovery.Receive(ctx, source.TenantID, request.DiscoveryID, request.Result)
	if err != nil {
		return nil, err
	}
	// A partial/expired discovery receipt remains durable evidence in its own
	// service, but cannot advance a selected/all batch to schema review.
	if receipt.Status != "completed" || receipt.ResourceCount != len(browserDiscoveryResourceIDs()) {
		return nil, ErrBrowserBatchWorkflowNotReady
	}
	receiptSHA, err := browserBatchDiscoveryReceiptSHA256(receipt)
	if err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	completed, err := s.workflow.CompleteDiscovery(ctx, ownerID, batchID, sourceID, BrowserBatchDiscoveryCompletion{
		LeaseID: request.LeaseID, PhaseGeneration: request.PhaseGeneration, DiscoveryID: request.DiscoveryID,
		DiscoveryContractSHA256: receipt.ContractSHA256, DiscoveryReceiptSHA256: receiptSHA,
	})
	if err != nil {
		return nil, err
	}
	// Schema review remains an explicit owner action. Advancing into the review
	// required state itself is safe and removes a client-side bookkeeping call.
	if _, err := s.workflow.AdvanceSafe(ctx, ownerID, batchID); err != nil {
		return nil, err
	}
	advanced, err := s.currentSource(ctx, ownerID, batchID, completed.SourceCompanyID)
	if err != nil {
		return nil, err
	}
	return &advanced, nil
}

func (s *BrowserBatchWorkflowActionsService) RequireSchemaReview(ctx context.Context, ownerID, batchID, sourceID string, request BrowserBatchSchemaPhaseRequest) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.RequireSchemaReview(ctx, ownerID, batchID, sourceID, request.PhaseGeneration)
}

// RefreshSchemaReadiness records an already-registered immutable review after
// a response-loss retry. It never creates an approval or accepts schema data.
func (s *BrowserBatchWorkflowActionsService) RefreshSchemaReadiness(ctx context.Context, ownerID, batchID, sourceID string, request BrowserBatchSchemaPhaseRequest) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.workflow == nil || s.schemas == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	source, err := s.currentSource(ctx, ownerID, batchID, sourceID)
	if err != nil || source.Phase != BrowserBatchPhaseSchemaReviewRequired || source.PhaseGeneration != request.PhaseGeneration {
		if err != nil {
			return nil, err
		}
		return nil, ErrBrowserBatchWorkflowConflict
	}
	approval, err := s.schemas.Status(ctx, source.TenantID, source.DiscoveryID, BrowserGeneralLedgerResourceID, BrowserGeneralLedgerCSVSchemaID)
	if err != nil || approval.Status != "registered" || !approvedBrowserGeneralLedgerSchema(approval.ResourceID, approval.SchemaID) || !validSHA256(approval.ApprovalSHA256) {
		return nil, ErrBrowserBatchWorkflowNotReady
	}
	return s.recordSchemaApprovalAndAdvance(ctx, ownerID, batchID, sourceID, BrowserBatchSchemaApproval{PhaseGeneration: request.PhaseGeneration, SchemaID: approval.SchemaID, SchemaApprovalSHA256: approval.ApprovalSHA256})
}

func (s *BrowserBatchWorkflowActionsService) ConfirmSchema(ctx context.Context, ownerID, batchID, sourceID, actor string, request BrowserBatchSchemaConfirmRequest) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.workflow == nil || s.schemas == nil || !request.ReviewConfirmed {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	source, err := s.currentSource(ctx, ownerID, batchID, sourceID)
	if err != nil || source.Phase != BrowserBatchPhaseSchemaReviewRequired || source.PhaseGeneration != request.PhaseGeneration {
		if err != nil {
			return nil, err
		}
		return nil, ErrBrowserBatchWorkflowConflict
	}
	approval, _, err := s.schemas.Review(ctx, source.TenantID, actor, source.DiscoveryID, BrowserGeneralLedgerResourceID, BrowserGeneralLedgerCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	if err != nil || approval.Status != "registered" || !approvedBrowserGeneralLedgerSchema(approval.ResourceID, approval.SchemaID) || !validSHA256(approval.ApprovalSHA256) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.recordSchemaApprovalAndAdvance(ctx, ownerID, batchID, sourceID, BrowserBatchSchemaApproval{PhaseGeneration: request.PhaseGeneration, SchemaID: approval.SchemaID, SchemaApprovalSHA256: approval.ApprovalSHA256})
}

func (s *BrowserBatchWorkflowActionsService) recordSchemaApprovalAndAdvance(ctx context.Context, ownerID, batchID, sourceID string, input BrowserBatchSchemaApproval) (*BrowserBatchSourceWorkflow, error) {
	approved, err := s.workflow.RecordSchemaApproval(ctx, ownerID, batchID, sourceID, input)
	if err != nil {
		return nil, err
	}
	// The only possible follow-up here is opening the transfer-confirmation
	// state after every immutable selected source is approved. It freezes no
	// source scope and does not issue a transfer capability; ConfirmTransfer
	// remains the separate action-time owner confirmation.
	if _, err := s.workflow.AdvanceSafe(ctx, ownerID, batchID); err != nil {
		return nil, err
	}
	return approved, nil
}

func (s *BrowserBatchWorkflowActionsService) OpenTransferConfirmation(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.OpenTransferConfirmation(ctx, ownerID, batchID)
}

func (s *BrowserBatchWorkflowActionsService) ConfirmTransfer(ctx context.Context, ownerID, batchID string, request BrowserBatchTransferConfirmationRequest) (*BrowserBatchWorkflowStatus, error) {
	if s == nil || s.workflow == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return s.workflow.ConfirmTransfer(ctx, ownerID, batchID, request)
}

func (s *BrowserBatchWorkflowActionsService) AcquireCapture(ctx context.Context, ownerID, batchID, actor string, request BrowserBatchCaptureAcquireRequest) (*BrowserBatchCaptureAction, error) {
	if s == nil || s.workflow == nil || s.captures == nil || !request.TransferConsentConfirmed {
		return nil, ErrBrowserBatchWorkflowInvalid
	}
	status, err := s.workflow.Status(ctx, ownerID, batchID)
	if err != nil {
		return nil, err
	}
	var source *BrowserBatchSourceWorkflow
	for index := range status.Sources {
		candidate := status.Sources[index]
		if candidate.Phase == BrowserBatchPhaseCaptureRunning && candidate.CaptureRunID != "" {
			source = &candidate
			break
		}
	}
	if source == nil {
		source, err = s.workflow.ClaimNextCapture(ctx, ownerID, batchID)
		if err != nil {
			return nil, err
		}
	}
	if source.CaptureRunID == "" || status.Workflow.TransferManifestSHA256 == "" {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	issue, err := s.captures.IssueForRun(ctx, source.TenantID, actor, source.CaptureRunID, browserBatchCaptureStartRequest(*source, status.Workflow.TransferScope))
	if err != nil || issue == nil || issue.RunID != source.CaptureRunID || issue.TenantID != source.TenantID || issue.SourceCompanyID != source.SourceCompanyID {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return &BrowserBatchCaptureAction{Source: *source, Capture: *issue}, nil
}

func (s *BrowserBatchWorkflowActionsService) CompleteCapture(ctx context.Context, ownerID, batchID, sourceID string, request BrowserBatchCaptureCompleteRequest) (*BrowserBatchCaptureCompletion, error) {
	if s == nil || s.workflow == nil || s.captures == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	source, err := s.currentSource(ctx, ownerID, batchID, sourceID)
	if err != nil || source.Phase != BrowserBatchPhaseCaptureRunning || source.LeaseID != request.LeaseID || source.PhaseGeneration != request.PhaseGeneration || source.CaptureRunID == "" {
		if err != nil {
			return nil, err
		}
		return nil, ErrBrowserBatchWorkflowConflict
	}
	progress, err := s.captures.OwnerStatus(ctx, source.TenantID, source.CaptureRunID)
	if err != nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	if progress.Staging == nil || progress.Staging.Status == "compiling" || progress.Staging.Status == "compiled_private" || progress.Staging.Status == "pending_receiver_configuration" || progress.Staging.Status == "staging" || progress.Staging.Status == "staging_retry_required" {
		return &BrowserBatchCaptureCompletion{Source: source, Progress: progress}, nil
	}
	if progress.Staging.Status == "review_required" {
		next, markErr := s.workflow.MarkReviewRequired(ctx, ownerID, batchID, sourceID, BrowserBatchPhaseCaptureRunning, request.PhaseGeneration, "bridge_review_required")
		if markErr != nil {
			return nil, markErr
		}
		return &BrowserBatchCaptureCompletion{Source: *next, Progress: progress}, nil
	}
	if progress.Staging.Status != "staged_review_required" || !progress.Staging.Finalized || !safeBridgeID(progress.Staging.PackageID) || !validSHA256(progress.Staging.PackageSHA256) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	next, err := s.workflow.RecordStagedPackage(ctx, ownerID, batchID, sourceID, BrowserBatchStagedPackage{LeaseID: request.LeaseID, PhaseGeneration: request.PhaseGeneration, PackageID: progress.Staging.PackageID, PackageSHA256: progress.Staging.PackageSHA256})
	if err != nil {
		return nil, err
	}
	return &BrowserBatchCaptureCompletion{Source: *next, Progress: progress}, nil
}

func (s *BrowserBatchWorkflowActionsService) Preview(ctx context.Context, ownerID, batchID, sourceID, actor string, request BrowserBatchPreviewRequest) (*BrowserBatchSourceWorkflow, error) {
	if s == nil || s.workflow == nil || s.preview == nil {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	source, err := s.currentSource(ctx, ownerID, batchID, sourceID)
	if err != nil || source.Phase != BrowserBatchPhaseStaged || source.PhaseGeneration != request.PhaseGeneration {
		if err != nil {
			return nil, err
		}
		return nil, ErrBrowserBatchWorkflowConflict
	}
	receipt, previewErr := s.preview.Preview(ctx, source.TenantID, source.PackageID, actor, request.UseSourceChart)
	if !validBrowserBatchPreviewReceipt(receipt) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	next, err := s.workflow.RecordPreviewReady(ctx, ownerID, batchID, sourceID, BrowserBatchPreviewReady{PhaseGeneration: request.PhaseGeneration, PreviewID: receipt.PreviewID, PreviewSHA256: receipt.PreviewSHA256})
	if err != nil {
		return nil, err
	}
	if errors.Is(previewErr, ErrBrowserBatchPreviewReviewRequired) || receipt.Status == "REVIEW_REQUIRED" {
		return s.workflow.MarkReviewRequired(ctx, ownerID, batchID, sourceID, BrowserBatchPhasePreviewReady, next.PhaseGeneration, "preview_review_required")
	}
	if previewErr != nil || receipt.Status != "PREVIEW_READY" {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return next, nil
}

func (s *BrowserBatchWorkflowActionsService) currentSource(ctx context.Context, ownerID, batchID, sourceID string) (BrowserBatchSourceWorkflow, error) {
	status, err := s.Status(ctx, ownerID, batchID)
	if err != nil {
		return BrowserBatchSourceWorkflow{}, err
	}
	for _, source := range status.Sources {
		if source.SourceCompanyID == strings.TrimSpace(sourceID) {
			return source, nil
		}
	}
	return BrowserBatchSourceWorkflow{}, ErrBrowserBatchWorkflowNotFound
}

func browserBatchCaptureStartRequest(source BrowserBatchSourceWorkflow, scope BrowserBatchTransferScope) BrowserCaptureStartRequest {
	return BrowserCaptureStartRequest{SourceCompanyID: source.SourceCompanyID, ManifestVersion: BrowserCaptureManifestVersion, Scope: BrowserCaptureScope{Mode: scope.Mode, FromInclusive: scope.FromInclusive, ToInclusive: scope.ToInclusive, CutoffAt: scope.CutoffAt, ResourceIDs: append([]string(nil), scope.ResourceIDs...)}}
}

func browserBatchDiscoveryReceiptSHA256(receipt BrowserDiscoveryReceipt) (string, error) {
	encoded, err := json.Marshal(struct {
		DiscoveryID           string `json:"discovery_id"`
		Status                string `json:"status"`
		ManifestVersion       string `json:"manifest_version"`
		ContractVersion       string `json:"contract_version"`
		ContractSHA256        string `json:"contract_sha256"`
		ResourceCount         int    `json:"resource_count"`
		CaptureReadyCount     int    `json:"capture_ready_count"`
		FilterRequiredCount   int    `json:"filter_contract_required_count"`
		PageOnlyRequiredCount int    `json:"page_only_contract_required_count"`
		PrivateEndpointCount  int    `json:"private_endpoint_required_count"`
		BindingBlockedCount   int    `json:"binding_blocked_count"`
	}{receipt.DiscoveryID, receipt.Status, receipt.ManifestVersion, receipt.ContractVersion, receipt.ContractSHA256, receipt.ResourceCount, receipt.CaptureReadyCount, receipt.FilterRequiredCount, receipt.PageOnlyRequiredCount, receipt.PrivateEndpointCount, receipt.BindingBlockedCount})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validBrowserBatchPreviewReceipt(receipt BrowserBatchPreviewReceipt) bool {
	return validBrowserPairingID(receipt.PreviewID) && validSHA256(receipt.PreviewSHA256) && (receipt.Status == "PREVIEW_READY" || receipt.Status == "REVIEW_REQUIRED")
}
