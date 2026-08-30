package smartaccountssync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const browserCaptureWorkflowID = "4810bc55-e8c5-406d-9ca4-19060b132557"
const browserCaptureWorkflowNextDayID = "e4b8f01d-56e1-4747-88e8-1e2604a0f0d2"
const browserCaptureNextDayRunID = "0f8a09e1-017b-44a3-a36b-488feb4e99ce"

type browserCaptureWorkflowMemoryStore struct {
	*browserCaptureMemoryStore
	workflowsByID  map[string]BrowserCaptureWorkflow
	workflowsByKey map[string]string
}

func (s *browserCaptureWorkflowMemoryStore) FindOrCreateBrowserCaptureWorkflow(_ context.Context, workflow BrowserCaptureWorkflow) (*BrowserCaptureWorkflow, bool, error) {
	if s.workflowsByID == nil {
		s.workflowsByID = map[string]BrowserCaptureWorkflow{}
		s.workflowsByKey = map[string]string{}
	}
	key := workflow.TenantID + "/" + workflow.SourceCompanyID + "/" + workflow.FromInclusive + "/" + workflow.ToInclusive
	if existingID, found := s.workflowsByKey[key]; found {
		existing := s.workflowsByID[existingID]
		return &existing, false, nil
	}
	s.workflowsByID[workflow.ID] = workflow
	s.workflowsByKey[key] = workflow.ID
	return &workflow, true, nil
}

func (s *browserCaptureWorkflowMemoryStore) GetBrowserCaptureWorkflow(_ context.Context, workflowID, tenantID string) (*BrowserCaptureWorkflow, error) {
	workflow, found := s.workflowsByID[workflowID]
	if !found || workflow.TenantID != tenantID {
		return nil, ErrBrowserCaptureWorkflowNotFound
	}
	return &workflow, nil
}

func (s *browserCaptureWorkflowMemoryStore) SetBrowserCaptureWorkflowRun(_ context.Context, tenantID, workflowID, runID string, updatedAt time.Time) (*BrowserCaptureWorkflow, error) {
	workflow, found := s.workflowsByID[workflowID]
	if !found || workflow.TenantID != tenantID || (workflow.CaptureRunID != "" && workflow.CaptureRunID != runID) {
		return nil, ErrBrowserCaptureWorkflowNotFound
	}
	workflow.CaptureRunID = runID
	workflow.Status = BrowserCaptureWorkflowIssued
	workflow.UpdatedAt = updatedAt.UTC()
	s.workflowsByID[workflowID] = workflow
	return &workflow, nil
}

func newBrowserCaptureWorkflowTestService() (*BrowserCaptureWorkflowService, *browserCaptureWorkflowMemoryStore, *browserCaptureBridgeStub) {
	captures, captureStore, bridge := newBrowserCaptureTestService()
	store := &browserCaptureWorkflowMemoryStore{browserCaptureMemoryStore: captureStore}
	service := NewBrowserCaptureWorkflowService(store, store, captures)
	service.now = func() time.Time { return time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC) }
	service.newID = func() string { return browserCaptureWorkflowID }
	service.newRun = func() string { return browserCaptureRunID }
	return service, store, bridge
}

func browserCaptureWorkflowRequest(consent bool) BrowserCaptureWorkflowRequest {
	return BrowserCaptureWorkflowRequest{SourceCompanyID: browserSourceID, FromInclusive: "2024-01-01", TransferConsentConfirmed: consent}
}

func TestBrowserCaptureWorkflowDerivesJournalOnlyPlanAndRequiresActionConsent(t *testing.T) {
	service, store, bridge := newBrowserCaptureWorkflowTestService()
	ready, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(false))
	require.NoError(t, err)
	require.NotNil(t, ready)
	assert.Equal(t, BrowserCaptureWorkflowReady, ready.Status)
	assert.Nil(t, ready.Capture)
	assert.Equal(t, BrowserCapturePlanVersion, ready.Plan.Version)
	assert.Equal(t, BrowserCaptureFromDatePolicy, ready.Plan.FromDatePolicy)
	assert.Equal(t, "2024-01-01", ready.Plan.Scope.FromInclusive)
	assert.Equal(t, "2026-08-28", ready.Plan.Scope.ToInclusive)
	assert.Equal(t, "2026-08-28T10:00:00Z", ready.Plan.Scope.CutoffAt)
	assert.Equal(t, []string{BrowserCaptureProvenGeneralLedgerCSV}, ready.Plan.EligibleResourceIDs)
	assert.Equal(t, []string{BrowserCaptureProvenGeneralLedgerCSV}, ready.Plan.Scope.ResourceIDs)
	assert.Empty(t, bridge.startedRun)
	assert.Len(t, store.workflowsByID, 1)

	issued, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(true))
	require.NoError(t, err)
	require.NotNil(t, issued.Capture)
	assert.Equal(t, ready.WorkflowID, issued.WorkflowID)
	assert.Equal(t, BrowserCaptureWorkflowIssued, issued.Status)
	assert.Equal(t, browserCaptureRunID, issued.Plan.RunID)
	assert.Equal(t, browserCaptureRunID, issued.Capture.RunID)
	assert.Equal(t, []string{BrowserCaptureProvenGeneralLedgerCSV}, bridge.started.Scope.ResourceIDs)
	assert.NotContains(t, mustJSON(t, store.workflowsByID[issued.WorkflowID]), issued.Capture.CaptureToken)
}

func TestBrowserCaptureWorkflowRetryRotatesCapabilityWithoutDuplicateRun(t *testing.T) {
	service, store, _ := newBrowserCaptureWorkflowTestService()
	first, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(true))
	require.NoError(t, err)
	service.captures.newToken = func() (string, error) { return "0123456789_abcdefghijklmnopqrstuvwxyzABCDEF", nil }
	second, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(true))
	require.NoError(t, err)
	assert.Equal(t, first.WorkflowID, second.WorkflowID)
	assert.Equal(t, first.Capture.RunID, second.Capture.RunID)
	assert.NotEqual(t, first.Capture.CaptureToken, second.Capture.CaptureToken)
	assert.Len(t, store.workflowsByID, 1)
	assert.Len(t, store.auth, 1)
	_, err = service.captures.Status(context.Background(), browserPairingTenantID, first.Capture.RunID, first.Capture.CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)
}

func TestBrowserCaptureWorkflowSameDayRetriesReuseButNextDayCreatesImmutableGeneration(t *testing.T) {
	service, store, _ := newBrowserCaptureWorkflowTestService()
	first, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(false))
	require.NoError(t, err)
	sameDay, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(false))
	require.NoError(t, err)
	assert.Equal(t, first.WorkflowID, sameDay.WorkflowID)
	assert.Equal(t, "2026-08-28", sameDay.Plan.Scope.ToInclusive)

	service.now = func() time.Time { return time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC) }
	service.captures.now = service.now
	service.newID = func() string { return browserCaptureWorkflowNextDayID }
	service.newRun = func() string { return browserCaptureNextDayRunID }
	nextDay, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(true))
	require.NoError(t, err)
	require.NotNil(t, nextDay.Capture)
	assert.NotEqual(t, first.WorkflowID, nextDay.WorkflowID)
	assert.Equal(t, "2026-08-29", nextDay.Plan.Scope.ToInclusive)
	assert.Equal(t, browserCaptureNextDayRunID, nextDay.Capture.RunID)
	assert.Equal(t, "2026-08-28", store.workflowsByID[first.WorkflowID].ToInclusive)
	assert.Len(t, store.workflowsByID, 2)
}

func TestBrowserCaptureWorkflowRejectsUnpairedFutureAndCrossTenantStatus(t *testing.T) {
	service, store, _ := newBrowserCaptureWorkflowTestService()
	_, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", BrowserCaptureWorkflowRequest{SourceCompanyID: browserSourceID, FromInclusive: "2026-08-29"})
	assert.ErrorIs(t, err, ErrBrowserCaptureWorkflowInvalid)
	_, err = service.Start(context.Background(), browserPairingTenantID, "owner-1", BrowserCaptureWorkflowRequest{SourceCompanyID: "sa-browser-v1-99", FromInclusive: "2024-01-01"})
	assert.ErrorIs(t, err, ErrBrowserCaptureWorkflowNotPaired)
	ready, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(false))
	require.NoError(t, err)
	_, err = service.Status(context.Background(), "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", ready.WorkflowID)
	assert.ErrorIs(t, err, ErrBrowserCaptureWorkflowNotFound)
	assert.Len(t, store.workflowsByID, 1)
}

func TestBrowserCaptureWorkflowStatusProjectsPartialCaptureWithoutCapability(t *testing.T) {
	service, _, _ := newBrowserCaptureWorkflowTestService()
	issued, err := service.Start(context.Background(), browserPairingTenantID, "owner-1", browserCaptureWorkflowRequest(true))
	require.NoError(t, err)
	status, err := service.Status(context.Background(), browserPairingTenantID, issued.WorkflowID)
	require.NoError(t, err)
	require.NotNil(t, status.Progress)
	assert.Equal(t, "partial", status.Plan.Scope.Mode)
	assert.Equal(t, browserCaptureRunID, status.Progress.RunID)
	assert.NotContains(t, mustJSON(t, status), issued.Capture.CaptureToken)
}
