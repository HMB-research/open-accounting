package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBrowserMasterDetailStore struct {
	controls map[string]smartaccountssync.Control
	byRun    map[string]smartaccountssync.BrowserMasterDetailAuthorization
	byBatch  map[string]string
}

func (s *fakeBrowserMasterDetailStore) Get(_ context.Context, tenantID, sourceID string) (*smartaccountssync.Control, error) {
	control, found := s.controls[smartAccountsControlKey(tenantID, sourceID)]
	if !found {
		return nil, smartaccountssync.ErrControlNotConfigured
	}
	return &control, nil
}

func (s *fakeBrowserMasterDetailStore) FindOrCreateBrowserMasterDetailAuthorization(_ context.Context, value smartaccountssync.BrowserMasterDetailAuthorization) (*smartaccountssync.BrowserMasterDetailAuthorization, bool, error) {
	if s.byRun == nil {
		s.byRun, s.byBatch = map[string]smartaccountssync.BrowserMasterDetailAuthorization{}, map[string]string{}
	}
	key := value.TenantID + "/" + value.BatchID + "/" + value.ResourceID
	if runID, found := s.byBatch[key]; found {
		stored := s.byRun[runID]
		return &stored, false, nil
	}
	s.byRun[value.RunID] = value
	s.byBatch[key] = value.RunID
	return &value, true, nil
}

func (s *fakeBrowserMasterDetailStore) GetBrowserMasterDetailAuthorization(_ context.Context, runID, tenantID string) (*smartaccountssync.BrowserMasterDetailAuthorization, error) {
	value, found := s.byRun[runID]
	if !found || value.TenantID != tenantID {
		return nil, smartaccountssync.ErrBrowserMasterDetailUnauthorized
	}
	return &value, nil
}

func (s *fakeBrowserMasterDetailStore) RotateBrowserMasterDetailAuthorization(_ context.Context, value smartaccountssync.BrowserMasterDetailAuthorization) error {
	if _, found := s.byRun[value.RunID]; !found {
		return smartaccountssync.ErrBrowserMasterDetailUnauthorized
	}
	s.byRun[value.RunID] = value
	return nil
}

type fakeBrowserMasterDetailBridge struct {
	starts map[string]smartaccountssync.BrowserMasterDetailStartRequest
	upload int
}

func (s *fakeBrowserMasterDetailBridge) StartBrowserMasterDetail(_ context.Context, _ string, runID string, request smartaccountssync.BrowserMasterDetailStartRequest) (smartaccountssync.BrowserMasterDetailStatus, error) {
	if s.starts == nil {
		s.starts = map[string]smartaccountssync.BrowserMasterDetailStartRequest{}
	}
	s.starts[runID] = request
	return fakeBrowserMasterDetailBridgeStatus(runID, request, "open"), nil
}

func (s *fakeBrowserMasterDetailBridge) GetBrowserMasterDetail(_ context.Context, _ string, runID string) (smartaccountssync.BrowserMasterDetailStatus, error) {
	request, found := s.starts[runID]
	if !found {
		return smartaccountssync.BrowserMasterDetailStatus{}, errors.New("not found")
	}
	return fakeBrowserMasterDetailBridgeStatus(runID, request, "open"), nil
}

func (s *fakeBrowserMasterDetailBridge) UploadBrowserMasterDetail(_ context.Context, _ string, runID, _ string, _ []byte) (smartaccountssync.BrowserMasterDetailUploadResult, error) {
	s.upload++
	return smartaccountssync.BrowserMasterDetailUploadResult{RunID: runID, Status: "accepted", Created: true}, nil
}

func (s *fakeBrowserMasterDetailBridge) FinalizeBrowserMasterDetail(_ context.Context, _ string, runID string) (smartaccountssync.BrowserMasterDetailStatus, error) {
	request, found := s.starts[runID]
	if !found {
		return smartaccountssync.BrowserMasterDetailStatus{}, errors.New("not found")
	}
	status := fakeBrowserMasterDetailBridgeStatus(runID, request, "finalized")
	status.NDJSONSHA256, status.RecordCount = strings.Repeat("a", 64), 2
	status.PackageID, status.PackageSHA256 = "master-detail-package", strings.Repeat("b", 64)
	return status, nil
}

func fakeBrowserMasterDetailBridgeStatus(runID string, request smartaccountssync.BrowserMasterDetailStartRequest, state string) smartaccountssync.BrowserMasterDetailStatus {
	encoded, _ := json.Marshal(request.Contract)
	sum := sha256.Sum256(encoded)
	return smartaccountssync.BrowserMasterDetailStatus{RunID: runID, Status: state, ManifestVersion: request.ManifestVersion, ResourceID: request.ResourceID, SchemaID: request.SchemaID, SourceSchema: request.ManifestVersion + "/" + request.SchemaID, ContractSHA256: hex.EncodeToString(sum[:])}
}

func newSmartAccountsBrowserMasterDetailHandlers() (*Handlers, *fakeBrowserMasterDetailStore, *fakeBrowserMasterDetailBridge) {
	store := &fakeBrowserMasterDetailStore{controls: map[string]smartaccountssync.Control{
		smartAccountsControlKey(testBrowserPairingTenantID, testBrowserSourceID): {TenantID: testBrowserPairingTenantID, SourceCompanyID: testBrowserSourceID, SecretReference: "brave-session://0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3"},
	}}
	bridge := &fakeBrowserMasterDetailBridge{}
	return &Handlers{smartAccountsBrowserMasterDetailService: smartaccountssync.NewBrowserMasterDetailService(store, store, bridge)}, store, bridge
}

func TestSmartAccountsBrowserMasterDetailOwnerIssueAndExtensionRelayAreScoped(t *testing.T) {
	handlers, store, bridge := newSmartAccountsBrowserMasterDetailHandlers()
	issueRequest := withURLParams(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"source_company_id":"`+testBrowserSourceID+`","transfer_consent_confirmed":true}`)), map[string]string{"tenantID": testBrowserPairingTenantID})
	issueRequest = withClaims(issueRequest, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
	issueRequest.Header.Set("Content-Type", "application/json")
	issued := httptest.NewRecorder()
	handlers.IssueSmartAccountsBrowserMasterDetails(issued, issueRequest)
	require.Equal(t, http.StatusCreated, issued.Code)
	require.Equal(t, "no-store", issued.Header().Get("Cache-Control"))
	var issueSet smartaccountssync.BrowserMasterDetailIssueSet
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&issueSet))
	require.Len(t, issueSet.Issues, 3)
	assert.Equal(t, []string{"clients", "vendors", "articles"}, []string{issueSet.Issues[0].ResourceID, issueSet.Issues[1].ResourceID, issueSet.Issues[2].ResourceID})
	persisted, err := json.Marshal(store.byRun)
	require.NoError(t, err)
	assert.NotContains(t, string(persisted), issueSet.Issues[0].CaptureToken)
	assert.Len(t, bridge.starts, 3)

	body := []byte(`{"synthetic":"protected"}` + "\n")
	sum := sha256.Sum256(body)
	upload := withURLParams(httptest.NewRequest(http.MethodPut, "/", strings.NewReader(string(body))), map[string]string{"tenantID": testBrowserPairingTenantID, "runID": issueSet.Issues[0].RunID})
	upload.Header.Set("Origin", testBraveExtensionOrigin)
	upload.Header.Set("Authorization", "Bearer "+issueSet.Issues[0].CaptureToken)
	upload.Header.Set("Content-Type", "application/x-ndjson")
	upload.Header.Set("X-SA-Browser-Resource-SHA256", hex.EncodeToString(sum[:]))
	uploaded := httptest.NewRecorder()
	handlers.UploadSmartAccountsBrowserMasterDetail(uploaded, upload)
	require.Equal(t, http.StatusOK, uploaded.Code)
	assert.Equal(t, 1, bridge.upload)

	finalize := withURLParams(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), map[string]string{"tenantID": testBrowserPairingTenantID, "runID": issueSet.Issues[0].RunID})
	finalize.Header.Set("Origin", testBraveExtensionOrigin)
	finalize.Header.Set("Authorization", "Bearer "+issueSet.Issues[0].CaptureToken)
	finalize.Header.Set("Content-Type", "application/json")
	finalized := httptest.NewRecorder()
	handlers.FinalizeSmartAccountsBrowserMasterDetail(finalized, finalize)
	require.Equal(t, http.StatusOK, finalized.Code)
	assert.Contains(t, finalized.Body.String(), `"status":"finalized_archived_evidence"`)
	assert.NotContains(t, finalized.Body.String(), `"capture_token"`)
	assert.NotContains(t, finalized.Body.String(), `preview`)

	crossTenant := withURLParams(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{"tenantID": "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", "runID": issueSet.Issues[0].RunID})
	crossTenant.Header.Set("Origin", testBraveExtensionOrigin)
	crossTenant.Header.Set("Authorization", "Bearer "+issueSet.Issues[0].CaptureToken)
	crossTenantRecorder := httptest.NewRecorder()
	handlers.GetSmartAccountsBrowserMasterDetail(crossTenantRecorder, crossTenant)
	assert.Equal(t, http.StatusUnauthorized, crossTenantRecorder.Code)
}

func TestSmartAccountsBrowserMasterDetailRejectsNonOwnerAndMalformedFinalize(t *testing.T) {
	handlers, _, _ := newSmartAccountsBrowserMasterDetailHandlers()
	nonOwner := withURLParams(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"source_company_id":"`+testBrowserSourceID+`","transfer_consent_confirmed":true}`)), map[string]string{"tenantID": testBrowserPairingTenantID})
	nonOwner = withClaims(nonOwner, createTestClaims("admin-1", "admin@example.com", testBrowserPairingTenantID, "admin"))
	nonOwner.Header.Set("Content-Type", "application/json")
	denied := httptest.NewRecorder()
	handlers.IssueSmartAccountsBrowserMasterDetails(denied, nonOwner)
	assert.Equal(t, http.StatusForbidden, denied.Code)

	badFinalize := withURLParams(httptest.NewRequest(http.MethodPost, "/?bad=1", strings.NewReader(`{"source":"data"}`)), map[string]string{"tenantID": testBrowserPairingTenantID, "runID": "389f6fec-1994-4cfe-8ea6-bb7281d3050f"})
	badFinalize.Header.Set("Origin", testBraveExtensionOrigin)
	badFinalize.Header.Set("Content-Type", "application/json")
	bad := httptest.NewRecorder()
	handlers.FinalizeSmartAccountsBrowserMasterDetail(bad, badFinalize)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
}
