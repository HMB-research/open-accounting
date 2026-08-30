package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSmartAccountsSourceID = "sa-company-hmb-9881"

type fakeSmartAccountsSyncStore struct {
	controls map[string]smartaccountssync.Control
	dryRuns  int
}

type bridgeConnectionCall struct {
	tenantID    string
	credentials smartaccountssync.BridgeCredentials
}

type fakeSmartAccountsBridgeClient struct {
	calls           []bridgeConnectionCall
	err             error
	captureProgress smartaccountssync.CaptureProgress
}

func (c *fakeSmartAccountsBridgeClient) ConnectAndValidate(_ context.Context, tenantID string, credentials smartaccountssync.BridgeCredentials) (smartaccountssync.BridgeConnection, error) {
	c.calls = append(c.calls, bridgeConnectionCall{tenantID: tenantID, credentials: credentials})
	if c.err != nil {
		return smartaccountssync.BridgeConnection{}, c.err
	}
	connectionID := "fake-" + tenantID
	return smartaccountssync.BridgeConnection{
		ConnectionID:          connectionID,
		SecretReference:       "secret-ref://sa-bridge/" + connectionID,
		ValidationStatus:      "connected",
		AccountCount:          2,
		SourceCompanyID:       testSmartAccountsSourceID,
		SourceCompanyName:     "SmartAccounts source",
		SourceBindingStatus:   "api_key_identity_and_snapshot_validated",
		AccountSnapshotSHA256: strings.Repeat("a", 64),
	}, nil
}

func (c *fakeSmartAccountsBridgeClient) StartCapture(_ context.Context, _ string, _ string, _ smartaccountssync.CaptureRequest) (smartaccountssync.CaptureProgress, error) {
	if c.err != nil {
		return smartaccountssync.CaptureProgress{}, c.err
	}
	if c.captureProgress.RunID == "" {
		c.captureProgress = smartaccountssync.CaptureProgress{RunID: "capture-test-1", Status: "running", DateFrom: "2026-01-01", DateTo: "2026-01-31", Summary: smartaccountssync.CaptureSummary{Total: 1, Running: 1}}
	}
	return c.captureProgress, nil
}

func (c *fakeSmartAccountsBridgeClient) GetCapture(_ context.Context, _ string, _ string, _ string) (smartaccountssync.CaptureProgress, error) {
	if c.err != nil {
		return smartaccountssync.CaptureProgress{}, c.err
	}
	return c.captureProgress, nil
}

func smartAccountsControlKey(tenantID, sourceCompanyID string) string {
	return tenantID + "\x00" + sourceCompanyID
}

func (s *fakeSmartAccountsSyncStore) Get(_ context.Context, tenantID, sourceCompanyID string) (*smartaccountssync.Control, error) {
	control, ok := s.controls[smartAccountsControlKey(tenantID, sourceCompanyID)]
	if !ok {
		return nil, smartaccountssync.ErrControlNotConfigured
	}
	return &control, nil
}

func (s *fakeSmartAccountsSyncStore) Upsert(_ context.Context, control smartaccountssync.Control) (*smartaccountssync.Control, error) {
	if s.controls == nil {
		s.controls = map[string]smartaccountssync.Control{}
	}
	key := smartAccountsControlKey(control.TenantID, control.SourceCompanyID)
	if existing, ok := s.controls[key]; ok {
		control.CreatedAt = existing.CreatedAt
	} else if control.CreatedAt.IsZero() {
		control.CreatedAt = control.UpdatedAt
	}
	s.controls[key] = control
	return &control, nil
}

func (s *fakeSmartAccountsSyncStore) MarkDryRunRequested(_ context.Context, tenantID, sourceCompanyID string, requestedAt time.Time) (*smartaccountssync.Control, error) {
	key := smartAccountsControlKey(tenantID, sourceCompanyID)
	control, ok := s.controls[key]
	if !ok {
		return nil, smartaccountssync.ErrControlNotConfigured
	}
	s.dryRuns++
	control.DryRunRequestedAt = &requestedAt
	control.UpdatedAt = requestedAt
	s.controls[key] = control
	return &control, nil
}

func (s *fakeSmartAccountsSyncStore) RecordCaptureRun(_ context.Context, tenantID, sourceCompanyID, runID string, requestedAt time.Time) (*smartaccountssync.Control, error) {
	key := smartAccountsControlKey(tenantID, sourceCompanyID)
	control, ok := s.controls[key]
	if !ok {
		return nil, smartaccountssync.ErrControlNotConfigured
	}
	control.CaptureRunID = runID
	control.DryRunRequestedAt = &requestedAt
	control.UpdatedAt = requestedAt
	s.controls[key] = control
	return &control, nil
}

func testSmartAccountsCatalog() smartaccountssync.StaticBridgeCatalog {
	return smartaccountssync.StaticBridgeCatalog{Discovery: smartaccountssync.SourceDiscovery{
		BridgeAvailable:   true,
		LiveDataContacted: false,
		Sources: []smartaccountssync.SourceCandidate{{
			Provider:                   smartaccountssync.Provider,
			SourceCompanyID:            testSmartAccountsSourceID,
			SourceCompanyName:          "Hold My Beer OÜ",
			Default:                    true,
			BridgeVerified:             true,
			GeneralLedgerAuthoritative: true,
			InvoicePaymentMode:         smartaccountssync.InvoicePaymentModeNonPosting,
		}},
	}}
}

func newSmartAccountsSyncHandlers(store *fakeSmartAccountsSyncStore) (*Handlers, *fakeSmartAccountsBridgeClient) {
	bridgeClient := &fakeSmartAccountsBridgeClient{}
	return &Handlers{
		smartAccountsSyncService:  smartaccountssync.NewService(store, testSmartAccountsCatalog()),
		smartAccountsBridgeClient: bridgeClient,
	}, bridgeClient
}

func testSmartAccountsControlRequest() smartaccountssync.ConnectRequest {
	return smartaccountssync.ConnectRequest{
		APIKey:                       "test-api-public",
		APISecret:                    "test-api-secret",
		SmartAccountsGLAuthoritative: true,
		InvoicePaymentMode:           smartaccountssync.InvoicePaymentModeNonPosting,
	}
}

func TestDiscoverSmartAccountsSyncSourcesReturnsBridgeVerifiedHoldMyBeerChoice(t *testing.T) {
	h, _ := newSmartAccountsSyncHandlers(&fakeSmartAccountsSyncStore{})
	w := httptest.NewRecorder()
	req := withURLParams(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{"tenantID": "tenant-1"})

	h.DiscoverSmartAccountsSyncSources(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var discovery smartaccountssync.SourceDiscovery
	require.NoError(t, decodeJSONResponse(w.Body, &discovery))
	assert.False(t, discovery.LiveDataContacted)
	require.Len(t, discovery.Sources, 1)
	assert.Equal(t, testSmartAccountsSourceID, discovery.Sources[0].SourceCompanyID)
	assert.Equal(t, "Hold My Beer OÜ", discovery.Sources[0].SourceCompanyName)
	assert.True(t, discovery.Sources[0].Default)
	assert.True(t, discovery.Sources[0].BridgeVerified)
}

func TestConfigureSmartAccountsSyncBridgesTransientCredentialsAndNeverEchoesThem(t *testing.T) {
	store := &fakeSmartAccountsSyncStore{}
	h, bridgeClient := newSmartAccountsSyncHandlers(store)
	apiSecret := "test-api-secret"
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/control", testSmartAccountsControlRequest(), createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ConfigureSmartAccountsSync(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), apiSecret)
	assert.NotContains(t, w.Body.String(), "test-api-public")
	require.Len(t, bridgeClient.calls, 1)
	assert.Equal(t, "tenant-1", bridgeClient.calls[0].tenantID)
	assert.Equal(t, apiSecret, bridgeClient.calls[0].credentials.APISecret)
	assert.Equal(t, "secret-ref://sa-bridge/fake-tenant-1", store.controls[smartAccountsControlKey("tenant-1", testSmartAccountsSourceID)].SecretReference)
	assert.Contains(t, w.Body.String(), `"invoice_payment_mode":"NON_POSTING"`)
}

func TestConfigureSmartAccountsSyncRejectsUnsafePolicyBeforeBridgeCall(t *testing.T) {
	h, bridgeClient := newSmartAccountsSyncHandlers(&fakeSmartAccountsSyncStore{})
	requestBody := testSmartAccountsControlRequest()
	requestBody.SmartAccountsGLAuthoritative = false
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/control", requestBody, createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ConfigureSmartAccountsSync(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Empty(t, bridgeClient.calls)
}

func TestConfigureSmartAccountsSyncBridgeFailureIsAtomicAndRedacted(t *testing.T) {
	store := &fakeSmartAccountsSyncStore{}
	h, bridgeClient := newSmartAccountsSyncHandlers(store)
	bridgeClient.err = errors.New("bridge rejected test-api-secret")
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/control", testSmartAccountsControlRequest(), createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ConfigureSmartAccountsSync(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.NotContains(t, w.Body.String(), "test-api-secret")
	assert.NotContains(t, w.Body.String(), "test-api-public")
	assert.Empty(t, store.controls)
	require.Len(t, bridgeClient.calls, 1)
}

func TestConfigureSmartAccountsSyncRejectsUnavailableBridgeWithoutPersistingCredentials(t *testing.T) {
	store := &fakeSmartAccountsSyncStore{}
	h, _ := newSmartAccountsSyncHandlers(store)
	h.smartAccountsBridgeClient = smartaccountssync.UnavailableBridgeClient{}
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/control", testSmartAccountsControlRequest(), createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ConfigureSmartAccountsSync(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.NotContains(t, w.Body.String(), "test-api-public")
	assert.NotContains(t, w.Body.String(), "test-api-secret")
	assert.Empty(t, store.controls)
}

func TestConfigureSmartAccountsSyncIsolatesBridgeBindingsByTargetTenant(t *testing.T) {
	store := &fakeSmartAccountsSyncStore{}
	h, bridgeClient := newSmartAccountsSyncHandlers(store)

	for _, tenantID := range []string{"tenant-1", "tenant-2"} {
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID+"/smartaccounts-sync/control", testSmartAccountsControlRequest(), createTestClaims("user-1", "user@example.com", tenantID, "owner")), map[string]string{"tenantID": tenantID})
		w := httptest.NewRecorder()
		h.ConfigureSmartAccountsSync(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	require.Len(t, bridgeClient.calls, 2)
	assert.NotEqual(t, bridgeClient.calls[0].tenantID, bridgeClient.calls[1].tenantID)
	require.Len(t, store.controls, 2)
	assert.NotEqual(t, store.controls[smartAccountsControlKey("tenant-1", testSmartAccountsSourceID)].SecretReference, store.controls[smartAccountsControlKey("tenant-2", testSmartAccountsSourceID)].SecretReference)
}

func TestSmartAccountsSyncDryRunStatusHasNoFinancialWrite(t *testing.T) {
	store := &fakeSmartAccountsSyncStore{controls: map[string]smartaccountssync.Control{
		smartAccountsControlKey("tenant-1", testSmartAccountsSourceID): {
			TenantID:          "tenant-1",
			SourceCompanyID:   testSmartAccountsSourceID,
			SourceCompanyName: "Hold My Beer OÜ",
			SecretReference:   "secret-ref://sa-bridge/test-connection",
		},
	}}
	h, _ := newSmartAccountsSyncHandlers(store)
	req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/dry-run?source_company_id="+testSmartAccountsSourceID, smartaccountssync.CaptureRequest{DateFrom: "2026-01-01", DateTo: "2026-01-31"}, createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.RequestSmartAccountsSyncDryRun(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var status smartaccountssync.SyncStatus
	require.NoError(t, decodeJSONResponse(w.Body, &status))
	assert.Equal(t, "running", status.CaptureStatus)
	assert.Equal(t, "capture-test-1", status.CaptureRunID)
	assert.Equal(t, smartaccountssync.PlanStatusAwaitingCapturedInput, status.PlanStatus)
	assert.Equal(t, smartaccountssync.ReconciliationStatusPending, status.ReconciliationStatus)
	assert.False(t, status.FinancialWritesStarted)
	assert.False(t, status.FinancialApplyEligible)
	assert.Equal(t, 0, store.dryRuns)
}

func TestConfirmSmartAccountsFinancialApplyRequiresExplicitAndRemainsBlocked(t *testing.T) {
	store := &fakeSmartAccountsSyncStore{controls: map[string]smartaccountssync.Control{
		smartAccountsControlKey("tenant-1", testSmartAccountsSourceID): {
			TenantID:          "tenant-1",
			SourceCompanyID:   testSmartAccountsSourceID,
			SourceCompanyName: "Hold My Beer OÜ",
			SecretReference:   "secret-ref://sa-bridge/test-connection",
		},
	}}
	h, _ := newSmartAccountsSyncHandlers(store)
	unauthorizedConfirmation := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/apply?source_company_id="+testSmartAccountsSourceID, smartaccountssync.ConfirmApplyRequest{}, createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	unauthorizedRecorder := httptest.NewRecorder()

	h.ConfirmSmartAccountsFinancialApply(unauthorizedRecorder, unauthorizedConfirmation)

	assert.Equal(t, http.StatusBadRequest, unauthorizedRecorder.Code)
	confirmed := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/smartaccounts-sync/apply?source_company_id="+testSmartAccountsSourceID, smartaccountssync.ConfirmApplyRequest{Confirm: true}, createTestClaims("user-1", "user@example.com", "tenant-1", "owner")), map[string]string{"tenantID": "tenant-1"})
	confirmedRecorder := httptest.NewRecorder()

	h.ConfirmSmartAccountsFinancialApply(confirmedRecorder, confirmed)

	require.Equal(t, http.StatusConflict, confirmedRecorder.Code)
	assert.Contains(t, confirmedRecorder.Body.String(), `"financial_writes_started":false`)
}

func TestSmartAccountsSyncStatusRequiresExplicitSourceCompanyID(t *testing.T) {
	h, _ := newSmartAccountsSyncHandlers(&fakeSmartAccountsSyncStore{})
	req := withURLParams(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetSmartAccountsSyncStatus(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
