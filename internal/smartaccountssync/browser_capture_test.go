package smartaccountssync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const browserCaptureRunID = "fce8f327-58d9-4fbb-a899-102ea318d9b4"

type browserCaptureMemoryStore struct {
	controls map[string]Control
	auth     map[string]BrowserCaptureAuthorization
}

func (s *browserCaptureMemoryStore) Get(_ context.Context, tenantID, sourceID string) (*Control, error) {
	control, ok := s.controls[controlKey(tenantID, sourceID)]
	if !ok {
		return nil, ErrControlNotConfigured
	}
	return &control, nil
}

func (s *browserCaptureMemoryStore) CreateBrowserCaptureAuthorization(_ context.Context, authorization BrowserCaptureAuthorization) error {
	if s.auth == nil {
		s.auth = map[string]BrowserCaptureAuthorization{}
	}
	key := authorization.TenantID + "/" + authorization.RunID
	if _, found := s.auth[key]; found {
		return errors.New("duplicate authorization")
	}
	s.auth[key] = authorization
	return nil
}

func (s *browserCaptureMemoryStore) GetBrowserCaptureAuthorization(_ context.Context, runID, tenantID string) (*BrowserCaptureAuthorization, error) {
	authorization, ok := s.auth[tenantID+"/"+runID]
	if !ok {
		return nil, ErrBrowserCaptureUnauthorized
	}
	return &authorization, nil
}

func (s *browserCaptureMemoryStore) RotateBrowserCaptureAuthorization(_ context.Context, authorization BrowserCaptureAuthorization) error {
	key := authorization.TenantID + "/" + authorization.RunID
	if _, found := s.auth[key]; !found {
		return ErrBrowserCaptureUnauthorized
	}
	s.auth[key] = authorization
	return nil
}

type browserCaptureBridgeStub struct {
	startedTenant string
	startedRun    string
	started       BrowserCaptureStartRequest
	uploaded      bool
}

func (s *browserCaptureBridgeStub) StartBrowserCapture(_ context.Context, tenantID, runID string, request BrowserCaptureStartRequest) (BrowserCaptureStatus, error) {
	s.startedTenant, s.startedRun, s.started = tenantID, runID, request
	return BrowserCaptureStatus{RunID: runID, Status: "open", ManifestVersion: request.ManifestVersion, Scope: request.Scope, Resources: []BrowserCaptureResourceStatus{{ResourceID: request.Scope.ResourceIDs[0], Coverage: "export_csv", Status: "pending"}}}, nil
}

func (s *browserCaptureBridgeStub) GetBrowserCapture(_ context.Context, _ string, runID string) (BrowserCaptureStatus, error) {
	return BrowserCaptureStatus{RunID: runID, Status: "open", ManifestVersion: BrowserCaptureManifestVersion, Scope: s.started.Scope, Resources: []BrowserCaptureResourceStatus{{ResourceID: s.started.Scope.ResourceIDs[0], Coverage: "export_csv", Status: "pending"}}}, nil
}

func (s *browserCaptureBridgeStub) UploadBrowserCaptureResource(_ context.Context, _ string, _ string, resourceID, _ string, contentType string, _ []byte) (BrowserCaptureResourceStatus, error) {
	if contentType != "text/csv" {
		return BrowserCaptureResourceStatus{}, errors.New("unexpected content type")
	}
	s.uploaded = true
	return BrowserCaptureResourceStatus{ResourceID: resourceID, Coverage: "export_csv", Status: "sealed", Created: true}, nil
}

func (s *browserCaptureBridgeStub) FinalizeBrowserCapture(_ context.Context, _ string, runID string) (BrowserCaptureStatus, error) {
	return BrowserCaptureStatus{RunID: runID, Status: "finalized_partial", ManifestVersion: BrowserCaptureManifestVersion, Scope: s.started.Scope, Resources: []BrowserCaptureResourceStatus{{ResourceID: BrowserGeneralLedgerResourceID, Coverage: "export_csv", Status: "completed"}}, Receipt: &BrowserCaptureCoverageReceipt{Status: "partial_coverage_recorded", CompletedExportCount: 1, RequiredExportCount: 1, FinalizedAt: "2026-08-28T10:01:00Z"}}, nil
}

func newBrowserCaptureTestService() (*BrowserCaptureService, *browserCaptureMemoryStore, *browserCaptureBridgeStub) {
	store := &browserCaptureMemoryStore{controls: map[string]Control{
		controlKey(browserPairingTenantID, browserSourceID): {
			TenantID: browserPairingTenantID, SourceCompanyID: browserSourceID, SecretReference: browserSessionReference(browserPairingID),
		},
	}}
	bridge := &browserCaptureBridgeStub{}
	service := NewBrowserCaptureService(store, store, bridge)
	service.now = func() time.Time { return time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC) }
	service.newRun = func() string { return browserCaptureRunID }
	service.newToken = func() (string, error) { return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq", nil }
	return service, store, bridge
}

func validBrowserCaptureTestRequest() BrowserCaptureStartRequest {
	return BrowserCaptureStartRequest{SourceCompanyID: browserSourceID, ManifestVersion: BrowserCaptureManifestVersion, Scope: BrowserCaptureScope{
		Mode: "partial", FromInclusive: "2024-01-01", ToInclusive: "2024-12-31", CutoffAt: "2026-08-28T10:00:00Z", ResourceIDs: []string{BrowserGeneralLedgerResourceID},
	}}
}

func TestBrowserCaptureIssuesHashOnlyAndRelaysOnlyAuthorizedResource(t *testing.T) {
	service, store, bridge := newBrowserCaptureTestService()
	issue, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1", validBrowserCaptureTestRequest())

	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, browserPairingTenantID, issue.TenantID)
	assert.True(t, issue.TransferConsent.Confirmed)
	assert.Equal(t, 1, issue.TransferConsent.Version)
	assert.Equal(t, "open", issue.Status)
	assert.Equal(t, browserPairingTenantID, bridge.startedTenant)
	assert.Equal(t, browserCaptureRunID, bridge.startedRun)
	stored := store.auth[browserPairingTenantID+"/"+browserCaptureRunID]
	assert.Equal(t, browserCaptureTokenSHA256(issue.CaptureToken), stored.TokenSHA256)
	assert.NotContains(t, mustJSON(t, stored), issue.CaptureToken)
	progress, err := service.Status(context.Background(), browserPairingTenantID, issue.RunID, issue.CaptureToken)
	require.NoError(t, err)
	assert.Equal(t, "open", progress.Status)
	assert.Equal(t, browserPairingTenantID, progress.TenantID)
	assert.Equal(t, browserSourceID, progress.SourceCompanyID)

	body := []byte("header\nvalue\n")
	resource, err := service.Upload(context.Background(), browserPairingTenantID, issue.RunID, BrowserGeneralLedgerResourceID, issue.CaptureToken, browserCaptureTokenSHA256(string(body)), body)
	require.NoError(t, err)
	assert.True(t, bridge.uploaded)
	assert.Equal(t, "sealed", resource.Status)
	_, err = service.Upload(context.Background(), browserPairingTenantID, issue.RunID, "payments", issue.CaptureToken, browserCaptureTokenSHA256(string(body)), body)
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)

	final, err := service.Finalize(context.Background(), browserPairingTenantID, issue.RunID, issue.CaptureToken)
	require.NoError(t, err)
	assert.Equal(t, "finalized_partial", final.Status)
	require.NotNil(t, final.Receipt)
	assert.Equal(t, "partial_coverage_recorded", final.Receipt.Status)
}

func TestBrowserCaptureOwnerStatusBindsExistingRunWithoutCapability(t *testing.T) {
	service, _, _ := newBrowserCaptureTestService()
	issue, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1", validBrowserCaptureTestRequest())
	require.NoError(t, err)
	status, err := service.OwnerStatus(context.Background(), browserPairingTenantID, issue.RunID)
	require.NoError(t, err)
	assert.Equal(t, issue.RunID, status.RunID)
	assert.Equal(t, browserPairingTenantID, status.TenantID)
	assert.Equal(t, browserSourceID, status.SourceCompanyID)
	assert.NotContains(t, mustJSON(t, status), issue.CaptureToken)
	_, err = service.OwnerStatus(context.Background(), "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", issue.RunID)
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)
}

func TestBrowserCaptureRejectsCrossTenantTokenReplayExpiryAndInvalidScope(t *testing.T) {
	service, _, _ := newBrowserCaptureTestService()
	issue, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1", validBrowserCaptureTestRequest())
	require.NoError(t, err)

	_, err = service.Status(context.Background(), "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", issue.RunID, issue.CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)
	_, err = service.Status(context.Background(), browserPairingTenantID, issue.RunID, strings.Repeat("a", 43))
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)
	service.now = func() time.Time { return issue.ExpiresAt.Add(time.Second) }
	_, err = service.Status(context.Background(), browserPairingTenantID, issue.RunID, issue.CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)

	request := validBrowserCaptureTestRequest()
	request.Scope.ResourceIDs = []string{"sales_invoices", "sales_invoices"}
	assert.False(t, validBrowserCaptureRequest(request))
	request = validBrowserCaptureTestRequest()
	request.Scope.ResourceIDs = []string{"sales_invoices"}
	assert.False(t, validBrowserCaptureRequest(request))
	_, err = service.Issue(context.Background(), browserPairingTenantID, "owner-1", request)
	assert.ErrorIs(t, err, ErrBrowserCaptureInvalid)
	request = validBrowserCaptureTestRequest()
	request.Scope.Mode = "full"
	assert.False(t, validBrowserCaptureRequest(request))
	request = validBrowserCaptureTestRequest()
	request.Scope.FromInclusive = ""
	assert.False(t, validBrowserCaptureRequest(request))
	request = validBrowserCaptureTestRequest()
	request.Scope.ToInclusive = ""
	assert.False(t, validBrowserCaptureRequest(request))
}

func TestBrowserCaptureRequiresExistingBrowserSessionBinding(t *testing.T) {
	service, store, _ := newBrowserCaptureTestService()
	store.controls[controlKey(browserPairingTenantID, browserSourceID)] = Control{TenantID: browserPairingTenantID, SourceCompanyID: browserSourceID, SecretReference: "secret-ref://sa-bridge/not-browser"}
	_, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1", validBrowserCaptureTestRequest())
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)
}

func TestBrowserCaptureResumeRotatesOnlySameRunCapabilityAfterRenewedConsent(t *testing.T) {
	service, _, _ := newBrowserCaptureTestService()
	issue, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1", validBrowserCaptureTestRequest())
	require.NoError(t, err)
	service.now = func() time.Time { return issue.ExpiresAt.Add(time.Second) }
	service.newToken = func() (string, error) { return "0123456789_abcdefghijklmnopqrstuvwxyzABCDEF", nil }
	_, err = service.Resume(context.Background(), browserPairingTenantID, "owner-1", issue.RunID, BrowserCaptureResumeRequest{})
	assert.ErrorIs(t, err, ErrBrowserCaptureConsent)
	resumed, err := service.Resume(context.Background(), browserPairingTenantID, "owner-1", issue.RunID, BrowserCaptureResumeRequest{TransferConsentConfirmed: true})
	require.NoError(t, err)
	assert.Equal(t, issue.RunID, resumed.RunID)
	assert.Equal(t, issue.Scope, resumed.Scope)
	assert.Equal(t, issue.SourceCompanyID, resumed.SourceCompanyID)
	assert.NotEqual(t, issue.CaptureToken, resumed.CaptureToken)
	_, err = service.Status(context.Background(), browserPairingTenantID, issue.RunID, issue.CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserCaptureUnauthorized)
	_, err = service.Status(context.Background(), browserPairingTenantID, resumed.RunID, resumed.CaptureToken)
	require.NoError(t, err)
}
