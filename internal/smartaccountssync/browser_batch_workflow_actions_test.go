package smartaccountssync

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type browserBatchDiscoveryActionsStub struct {
	issued        int
	receiptStatus string
	resourceCount int
}

func (s *browserBatchDiscoveryActionsStub) Issue(_ context.Context, tenantID, _ string, request BrowserDiscoveryStartRequest) (*BrowserDiscoveryIssue, error) {
	s.issued++
	return &BrowserDiscoveryIssue{DiscoveryID: uuid.NewString(), TenantID: tenantID, SourceCompanyID: request.SourceCompanyID, ManifestVersion: BrowserDiscoveryManifestVersion, ResourceIDs: browserDiscoveryResourceIDs(), ExpiresAt: time.Now().UTC().Add(time.Minute), DiscoveryConsent: BrowserDiscoveryConsent{Version: 1, Confirmed: true, ConfirmedAt: time.Now().UTC(), Scope: "metadata_and_header_probe", ResponseHeaderProbeConfirmed: request.ResponseHeaderProbeConfirmed}}, nil
}

func (s *browserBatchDiscoveryActionsStub) Receive(_ context.Context, _ string, discoveryID string, _ BrowserDiscoveryRelayResult) (BrowserDiscoveryReceipt, error) {
	status, count := s.receiptStatus, s.resourceCount
	if status == "" {
		status = "completed"
	}
	if count == 0 {
		count = len(browserDiscoveryResourceIDs())
	}
	return BrowserDiscoveryReceipt{DiscoveryID: discoveryID, Status: status, ManifestVersion: BrowserDiscoveryManifestVersion, ContractVersion: BrowserDiscoveryContractVersion, ContractSHA256: strings.Repeat("a", 64), ResourceCount: count, FilterRequiredCount: count}, nil
}

type browserBatchSchemaActionsStub struct {
	approvals map[string]BrowserCSVSchemaApprovalResponse
}

func (s *browserBatchSchemaActionsStub) Review(_ context.Context, _ string, _ string, discoveryID, resourceID, schemaID string, request BrowserCSVSchemaApprovalRequest) (BrowserCSVSchemaApprovalResponse, bool, error) {
	if !request.ReviewConfirmed || resourceID != BrowserGeneralLedgerResourceID || schemaID != BrowserGeneralLedgerCSVSchemaID {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalInvalid
	}
	if s.approvals == nil {
		s.approvals = map[string]BrowserCSVSchemaApprovalResponse{}
	}
	approval, found := s.approvals[discoveryID]
	if !found {
		approval = BrowserCSVSchemaApprovalResponse{ResourceID: resourceID, SchemaID: schemaID, Status: "registered", ApprovalSHA256: strings.Repeat(string(rune('b'+len(s.approvals))), 64)}
		s.approvals[discoveryID] = approval
		return approval, true, nil
	}
	return approval, false, nil
}

func (s *browserBatchSchemaActionsStub) Status(_ context.Context, _ string, discoveryID, resourceID, schemaID string) (BrowserCSVSchemaApprovalResponse, error) {
	approval, found := s.approvals[discoveryID]
	if !found || approval.ResourceID != resourceID || approval.SchemaID != schemaID {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalNotFound
	}
	return approval, nil
}

type browserBatchCaptureActionsStub struct {
	issues   []BrowserCaptureIssue
	progress map[string]BrowserCaptureStatus
}

func (s *browserBatchCaptureActionsStub) IssueForRun(_ context.Context, tenantID, _ string, runID string, request BrowserCaptureStartRequest) (*BrowserCaptureIssue, error) {
	issue := BrowserCaptureIssue{RunID: runID, TenantID: tenantID, CaptureToken: "transient-token-" + runID, ExpiresAt: time.Now().UTC().Add(time.Minute), SourceCompanyID: request.SourceCompanyID, ManifestVersion: request.ManifestVersion, Scope: request.Scope, Status: "open", TransferConsent: BrowserCaptureTransferConsent{Version: 1, Confirmed: true, ConfirmedAt: time.Now().UTC()}}
	s.issues = append(s.issues, issue)
	return &issue, nil
}

func (s *browserBatchCaptureActionsStub) OwnerStatus(_ context.Context, tenantID, runID string) (BrowserCaptureStatus, error) {
	status, found := s.progress[runID]
	if !found || status.TenantID != tenantID {
		return BrowserCaptureStatus{}, ErrBrowserCaptureUnauthorized
	}
	return status, nil
}

type browserBatchPreviewActionsStub struct{ review bool }

func (s browserBatchPreviewActionsStub) Preview(_ context.Context, _ string, _ string, _ string, _ bool) (BrowserBatchPreviewReceipt, error) {
	receipt := BrowserBatchPreviewReceipt{PreviewID: uuid.NewString(), PreviewSHA256: strings.Repeat("e", 64), Status: "PREVIEW_READY"}
	if s.review {
		receipt.Status = "REVIEW_REQUIRED"
		return receipt, ErrBrowserBatchPreviewReviewRequired
	}
	return receipt, nil
}

func newBrowserBatchWorkflowActionsFixture(t *testing.T) (*BrowserBatchWorkflowActionsService, *BrowserBatchWorkflowService, *browserBatchCaptureActionsStub, time.Time) {
	t.Helper()
	workflow, _, now := newBrowserBatchWorkflowFixture(t)
	captures := &browserBatchCaptureActionsStub{progress: map[string]BrowserCaptureStatus{}}
	actions := NewBrowserBatchWorkflowActionsService(workflow, &browserBatchDiscoveryActionsStub{}, &browserBatchSchemaActionsStub{}, captures, browserBatchPreviewActionsStub{})
	return actions, workflow, captures, now
}

func prepareAndApproveBrowserBatchActions(t *testing.T, actions *BrowserBatchWorkflowActionsService) *BrowserBatchWorkflowStatus {
	t.Helper()
	status, err := actions.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true, HeaderProbeConsentConfirmed: true})
	require.NoError(t, err)
	for {
		action, acquireErr := actions.AcquireDiscovery(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true})
		if errors.Is(acquireErr, ErrBrowserBatchWorkflowNotReady) {
			break
		}
		require.NoError(t, acquireErr)
		completed, completionErr := actions.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, action.Source.SourceCompanyID, BrowserBatchDiscoveryCompleteRequest{LeaseID: action.Source.LeaseID, PhaseGeneration: action.Source.PhaseGeneration, DiscoveryID: action.Discovery.DiscoveryID, Result: BrowserDiscoveryRelayResult{DiscoveryID: action.Discovery.DiscoveryID}})
		require.NoError(t, completionErr)
		review, reviewErr := actions.RequireSchemaReview(context.Background(), "owner-1", batchWorkflowID, completed.SourceCompanyID, BrowserBatchSchemaPhaseRequest{PhaseGeneration: completed.PhaseGeneration})
		require.NoError(t, reviewErr)
		_, confirmErr := actions.ConfirmSchema(context.Background(), "owner-1", batchWorkflowID, review.SourceCompanyID, "owner-1", BrowserBatchSchemaConfirmRequest{PhaseGeneration: review.PhaseGeneration, ReviewConfirmed: true})
		require.NoError(t, confirmErr)
	}
	status, err = actions.OpenTransferConfirmation(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	_, err = actions.ConfirmTransfer(context.Background(), "owner-1", batchWorkflowID, BrowserBatchTransferConfirmationRequest{OwnerConfirmed: true, ExpectedSchemaSHA256: status.SchemaReadinessSHA256})
	require.NoError(t, err)
	return status
}

func TestBrowserBatchWorkflowActionsAdvanceOnlySafeStepsAndExposeConsentGates(t *testing.T) {
	actions, _, captures, _ := newBrowserBatchWorkflowActionsFixture(t)
	prepared, err := actions.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{
		HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true,
	})
	require.NoError(t, err)
	require.Equal(t, BrowserBatchNextStepAcquireDiscovery, prepared.NextStep.Action)
	assert.True(t, prepared.NextStep.OwnerConfirmationRequired)
	assert.True(t, prepared.NextStep.FreshConsentRequired)
	assert.False(t, prepared.NextStep.Automatic)

	for completedCount := 0; completedCount < 2; completedCount++ {
		discovery, acquireErr := actions.AcquireDiscovery(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true})
		require.NoError(t, acquireErr)
		completed, completionErr := actions.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, discovery.Source.SourceCompanyID, BrowserBatchDiscoveryCompleteRequest{
			LeaseID: discovery.Source.LeaseID, PhaseGeneration: discovery.Source.PhaseGeneration, DiscoveryID: discovery.Discovery.DiscoveryID,
			Result: BrowserDiscoveryRelayResult{DiscoveryID: discovery.Discovery.DiscoveryID},
		})
		require.NoError(t, completionErr)
		// The server progresses into a review-required state, but cannot review
		// or register a schema without this separate owner-confirmed call.
		require.Equal(t, BrowserBatchPhaseSchemaReviewRequired, completed.Phase)
		status, statusErr := actions.Status(context.Background(), "owner-1", batchWorkflowID)
		require.NoError(t, statusErr)
		require.Equal(t, BrowserBatchNextStepConfirmSchemaReview, status.NextStep.Action)
		assert.True(t, status.NextStep.OwnerConfirmationRequired)
		assert.False(t, status.NextStep.FreshConsentRequired)
		assert.Len(t, captures.issues, 0)

		approved, approvalErr := actions.ConfirmSchema(context.Background(), "owner-1", batchWorkflowID, completed.SourceCompanyID, "owner-1", BrowserBatchSchemaConfirmRequest{PhaseGeneration: completed.PhaseGeneration, ReviewConfirmed: true})
		require.NoError(t, approvalErr)
		require.Equal(t, BrowserBatchPhaseSchemaApproved, approved.Phase)
	}

	opened, err := actions.Status(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, BrowserBatchPhaseTransferConfirmationRequired, opened.Status)
	require.Equal(t, BrowserBatchNextStepConfirmTransferScope, opened.NextStep.Action)
	assert.True(t, opened.NextStep.OwnerConfirmationRequired)
	assert.False(t, opened.NextStep.FreshConsentRequired)
	assert.Empty(t, opened.Workflow.TransferManifestSHA256, "opening the confirmation must not freeze or authorize transfer")
	assert.Len(t, captures.issues, 0)

	// Safe progression is idempotent once the confirmation state is open.
	replay, err := actions.AdvanceSafe(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	assert.Equal(t, opened.Status, replay.Status)
	assert.Equal(t, opened.Sources, replay.Sources)

	frozen, err := actions.ConfirmTransfer(context.Background(), "owner-1", batchWorkflowID, BrowserBatchTransferConfirmationRequest{OwnerConfirmed: true, ExpectedSchemaSHA256: opened.SchemaReadinessSHA256})
	require.NoError(t, err)
	require.Equal(t, BrowserBatchNextStepAcquireCapture, frozen.NextStep.Action)
	assert.True(t, frozen.NextStep.OwnerConfirmationRequired)
	assert.True(t, frozen.NextStep.FreshConsentRequired)
	assert.Len(t, captures.issues, 0, "status/advance/transfer confirmation never issue a capture capability")
}

func TestBrowserBatchWorkflowAdvanceSafeRecoversDiscoveryCompleteIdempotently(t *testing.T) {
	service, _, _ := newBrowserBatchWorkflowFixture(t)
	prepareBrowserBatchWorkflow(t, service)
	claimed, err := service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	_, err = service.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, BrowserBatchDiscoveryCompletion{
		LeaseID: claimed.LeaseID, PhaseGeneration: claimed.PhaseGeneration, DiscoveryID: uuid.NewString(), DiscoveryContractSHA256: strings.Repeat("a", 64), DiscoveryReceiptSHA256: strings.Repeat("b", 64),
	})
	require.NoError(t, err)

	pending, err := service.Status(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, BrowserBatchNextStepAdvanceSchemaReview, pending.NextStep.Action)
	require.True(t, pending.NextStep.Automatic)

	advanced, err := service.AdvanceSafe(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, BrowserBatchPhaseSchemaReviewRequired, advanced.Status)
	require.Equal(t, BrowserBatchNextStepConfirmSchemaReview, advanced.NextStep.Action)

	replay, err := service.AdvanceSafe(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	assert.Equal(t, advanced.Sources, replay.Sources)
	assert.Equal(t, advanced.NextStep, replay.NextStep)
}

func TestBrowserBatchDiscoveryCanRunWithoutOptionalHeaderProbe(t *testing.T) {
	actions, _, _, _ := newBrowserBatchWorkflowActionsFixture(t)
	prepared, err := actions.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true, HeaderProbeConsentConfirmed: false})
	require.NoError(t, err)
	assert.False(t, prepared.Workflow.HeaderProbeConsentConfirmed)

	action, err := actions.AcquireDiscovery(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: false})
	require.NoError(t, err)
	assert.False(t, action.Discovery.DiscoveryConsent.ResponseHeaderProbeConfirmed)
	_, err = actions.AcquireDiscovery(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true})
	assert.ErrorIs(t, err, ErrBrowserBatchWorkflowInvalid)
}

func TestBrowserBatchDiscoveryReissueRotatesRunningLeaseAndRejectsLateCompletion(t *testing.T) {
	actions, _, _, _ := newBrowserBatchWorkflowActionsFixture(t)
	_, err := actions.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true})
	require.NoError(t, err)
	first, err := actions.AcquireDiscovery(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true})
	require.NoError(t, err)
	reissued, err := actions.ReissueDiscovery(context.Background(), "owner-1", batchWorkflowID, first.Source.SourceCompanyID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true})
	require.NoError(t, err)
	assert.Equal(t, first.Source.SourceCompanyID, reissued.Source.SourceCompanyID)
	assert.Equal(t, first.Source.TenantID, reissued.Source.TenantID)
	assert.Equal(t, BrowserBatchPhaseDiscoveryRunning, reissued.Source.Phase)
	assert.NotEqual(t, first.Source.LeaseID, reissued.Source.LeaseID)
	assert.Equal(t, first.Source.PhaseGeneration+1, reissued.Source.PhaseGeneration)
	assert.NotEqual(t, first.Discovery.DiscoveryID, reissued.Discovery.DiscoveryID)

	_, err = actions.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, first.Source.SourceCompanyID, BrowserBatchDiscoveryCompleteRequest{LeaseID: first.Source.LeaseID, PhaseGeneration: first.Source.PhaseGeneration, DiscoveryID: first.Discovery.DiscoveryID, Result: BrowserDiscoveryRelayResult{DiscoveryID: first.Discovery.DiscoveryID}})
	assert.ErrorIs(t, err, ErrBrowserBatchWorkflowConflict)
	_, err = actions.ReissueDiscovery(context.Background(), "owner-1", batchWorkflowID, first.Source.SourceCompanyID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true})
	assert.ErrorIs(t, err, ErrBrowserBatchWorkflowInvalid)
}

func TestBrowserBatchWorkflowActionsRunSafelyFromDiscoveryThroughPreview(t *testing.T) {
	actions, _, captures, _ := newBrowserBatchWorkflowActionsFixture(t)
	prepared := prepareAndApproveBrowserBatchActions(t, actions)
	require.NotEmpty(t, prepared.SchemaReadinessSHA256)

	captureAction, err := actions.AcquireCapture(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchCaptureAcquireRequest{TransferConsentConfirmed: true})
	require.NoError(t, err)
	require.NotEmpty(t, captureAction.Source.LeaseID)
	require.NotEmpty(t, captureAction.Source.CaptureRunID)
	assert.Equal(t, captureAction.Source.CaptureRunID, captureAction.Capture.RunID)
	assert.Contains(t, captureAction.Capture.CaptureToken, "transient-token-")

	// A compiling private package is only pollable progress. It cannot stage or
	// produce a financial preview.
	captures.progress[captureAction.Source.CaptureRunID] = BrowserCaptureStatus{RunID: captureAction.Source.CaptureRunID, TenantID: captureAction.Source.TenantID, SourceCompanyID: captureAction.Source.SourceCompanyID, Staging: &BrowserCaptureStaging{Status: "compiling"}}
	compiling, err := actions.CompleteCapture(context.Background(), "owner-1", batchWorkflowID, captureAction.Source.SourceCompanyID, BrowserBatchCaptureCompleteRequest{LeaseID: captureAction.Source.LeaseID, PhaseGeneration: captureAction.Source.PhaseGeneration})
	require.NoError(t, err)
	assert.Equal(t, BrowserBatchPhaseCaptureRunning, compiling.Source.Phase)

	captures.progress[captureAction.Source.CaptureRunID] = BrowserCaptureStatus{RunID: captureAction.Source.CaptureRunID, TenantID: captureAction.Source.TenantID, SourceCompanyID: captureAction.Source.SourceCompanyID, Staging: &BrowserCaptureStaging{PackageID: "package-1", PackageSHA256: strings.Repeat("c", 64), Status: "staged_review_required", Finalized: true}}
	staged, err := actions.CompleteCapture(context.Background(), "owner-1", batchWorkflowID, captureAction.Source.SourceCompanyID, BrowserBatchCaptureCompleteRequest{LeaseID: captureAction.Source.LeaseID, PhaseGeneration: captureAction.Source.PhaseGeneration})
	require.NoError(t, err)
	require.Equal(t, BrowserBatchPhaseStaged, staged.Source.Phase)
	assert.Equal(t, "package-1", staged.Source.PackageID)

	previewed, err := actions.Preview(context.Background(), "owner-1", batchWorkflowID, staged.Source.SourceCompanyID, "owner-1", BrowserBatchPreviewRequest{PhaseGeneration: staged.Source.PhaseGeneration, UseSourceChart: true})
	require.NoError(t, err)
	assert.Equal(t, BrowserBatchPhasePreviewReady, previewed.Phase)
	assert.NotEmpty(t, previewed.PreviewID)

	status, err := actions.Status(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	encoded, marshalErr := json.Marshal(status)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(encoded), captureAction.Capture.CaptureToken, "GET/status must never expose an action capability")
}

func TestBrowserBatchWorkflowActionsKeepSameCaptureRunAcrossLeaseRecovery(t *testing.T) {
	actions, workflow, captures, now := newBrowserBatchWorkflowActionsFixture(t)
	prepareAndApproveBrowserBatchActions(t, actions)
	first, err := actions.AcquireCapture(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchCaptureAcquireRequest{TransferConsentConfirmed: true})
	require.NoError(t, err)
	workflow.now = func() time.Time { return now.Add(browserBatchWorkflowLeaseLifetime + time.Second) }
	_, err = actions.Resume(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	second, err := actions.AcquireCapture(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchCaptureAcquireRequest{TransferConsentConfirmed: true})
	require.NoError(t, err)
	assert.NotEqual(t, first.Source.LeaseID, second.Source.LeaseID)
	assert.Equal(t, first.Source.CaptureRunID, second.Source.CaptureRunID)
	assert.Equal(t, first.Capture.RunID, second.Capture.RunID)
	assert.Len(t, captures.issues, 2)
}

func TestBrowserBatchWorkflowActionsRejectPartialDiscoveryAndStaleLease(t *testing.T) {
	workflow, _, _ := newBrowserBatchWorkflowFixture(t)
	discovery := &browserBatchDiscoveryActionsStub{receiptStatus: "expired", resourceCount: 1}
	actions := NewBrowserBatchWorkflowActionsService(workflow, discovery, &browserBatchSchemaActionsStub{}, &browserBatchCaptureActionsStub{progress: map[string]BrowserCaptureStatus{}}, browserBatchPreviewActionsStub{})
	_, err := actions.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true, HeaderProbeConsentConfirmed: true})
	require.NoError(t, err)
	action, err := actions.AcquireDiscovery(context.Background(), "owner-1", batchWorkflowID, "owner-1", BrowserBatchDiscoveryAcquireRequest{MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true})
	require.NoError(t, err)
	_, err = actions.CompleteDiscovery(context.Background(), "other-owner", batchWorkflowID, action.Source.SourceCompanyID, BrowserBatchDiscoveryCompleteRequest{LeaseID: action.Source.LeaseID, PhaseGeneration: action.Source.PhaseGeneration, DiscoveryID: action.Discovery.DiscoveryID, Result: BrowserDiscoveryRelayResult{DiscoveryID: action.Discovery.DiscoveryID}})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowNotFound)
	_, err = actions.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, action.Source.SourceCompanyID, BrowserBatchDiscoveryCompleteRequest{LeaseID: action.Source.LeaseID, PhaseGeneration: action.Source.PhaseGeneration, DiscoveryID: action.Discovery.DiscoveryID, Result: BrowserDiscoveryRelayResult{DiscoveryID: action.Discovery.DiscoveryID}})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowNotReady, "partial/expired discovery is durable evidence but cannot advance batch completeness")
	_, err = actions.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, action.Source.SourceCompanyID, BrowserBatchDiscoveryCompleteRequest{LeaseID: uuid.NewString(), PhaseGeneration: action.Source.PhaseGeneration, DiscoveryID: action.Discovery.DiscoveryID, Result: BrowserDiscoveryRelayResult{DiscoveryID: action.Discovery.DiscoveryID}})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowConflict)
}

func TestBrowserBatchWorkflowActionsRejectFutureHistoryBoundary(t *testing.T) {
	actions, _, _, now := newBrowserBatchWorkflowActionsFixture(t)
	_, err := actions.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: now.AddDate(0, 0, 1).Format(time.DateOnly), OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true, HeaderProbeConsentConfirmed: true})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowInvalid)
}
