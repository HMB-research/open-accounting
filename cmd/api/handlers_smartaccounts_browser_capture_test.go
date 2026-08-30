package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBrowserCaptureRunID = "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2"

type fakeBrowserCaptureStore struct {
	controls       map[string]smartaccountssync.Control
	auth           map[string]smartaccountssync.BrowserCaptureAuthorization
	workflowsByID  map[string]smartaccountssync.BrowserCaptureWorkflow
	workflowsByKey map[string]string
}

func (s *fakeBrowserCaptureStore) Get(_ context.Context, tenantID, sourceID string) (*smartaccountssync.Control, error) {
	control, ok := s.controls[smartAccountsControlKey(tenantID, sourceID)]
	if !ok {
		return nil, smartaccountssync.ErrControlNotConfigured
	}
	return &control, nil
}

func (s *fakeBrowserCaptureStore) CreateBrowserCaptureAuthorization(_ context.Context, authorization smartaccountssync.BrowserCaptureAuthorization) error {
	if s.auth == nil {
		s.auth = map[string]smartaccountssync.BrowserCaptureAuthorization{}
	}
	key := authorization.TenantID + "/" + authorization.RunID
	if _, exists := s.auth[key]; exists {
		return errors.New("duplicate")
	}
	s.auth[key] = authorization
	return nil
}

func (s *fakeBrowserCaptureStore) GetBrowserCaptureAuthorization(_ context.Context, runID, tenantID string) (*smartaccountssync.BrowserCaptureAuthorization, error) {
	authorization, ok := s.auth[tenantID+"/"+runID]
	if !ok {
		return nil, smartaccountssync.ErrBrowserCaptureUnauthorized
	}
	return &authorization, nil
}

func (s *fakeBrowserCaptureStore) RotateBrowserCaptureAuthorization(_ context.Context, authorization smartaccountssync.BrowserCaptureAuthorization) error {
	key := authorization.TenantID + "/" + authorization.RunID
	if _, exists := s.auth[key]; !exists {
		return smartaccountssync.ErrBrowserCaptureUnauthorized
	}
	s.auth[key] = authorization
	return nil
}

func (s *fakeBrowserCaptureStore) FindOrCreateBrowserCaptureWorkflow(_ context.Context, workflow smartaccountssync.BrowserCaptureWorkflow) (*smartaccountssync.BrowserCaptureWorkflow, bool, error) {
	if s.workflowsByID == nil {
		s.workflowsByID = map[string]smartaccountssync.BrowserCaptureWorkflow{}
		s.workflowsByKey = map[string]string{}
	}
	key := workflow.TenantID + "/" + workflow.SourceCompanyID + "/" + workflow.FromInclusive + "/" + workflow.ToInclusive
	if id, found := s.workflowsByKey[key]; found {
		existing := s.workflowsByID[id]
		return &existing, false, nil
	}
	s.workflowsByID[workflow.ID] = workflow
	s.workflowsByKey[key] = workflow.ID
	return &workflow, true, nil
}

func (s *fakeBrowserCaptureStore) GetBrowserCaptureWorkflow(_ context.Context, workflowID, tenantID string) (*smartaccountssync.BrowserCaptureWorkflow, error) {
	workflow, found := s.workflowsByID[workflowID]
	if !found || workflow.TenantID != tenantID {
		return nil, smartaccountssync.ErrBrowserCaptureWorkflowNotFound
	}
	return &workflow, nil
}

func (s *fakeBrowserCaptureStore) SetBrowserCaptureWorkflowRun(_ context.Context, tenantID, workflowID, runID string, updatedAt time.Time) (*smartaccountssync.BrowserCaptureWorkflow, error) {
	workflow, found := s.workflowsByID[workflowID]
	if !found || workflow.TenantID != tenantID || (workflow.CaptureRunID != "" && workflow.CaptureRunID != runID) {
		return nil, smartaccountssync.ErrBrowserCaptureWorkflowNotFound
	}
	workflow.CaptureRunID = runID
	workflow.Status = smartaccountssync.BrowserCaptureWorkflowIssued
	workflow.UpdatedAt = updatedAt.UTC()
	s.workflowsByID[workflowID] = workflow
	return &workflow, nil
}

type fakeBrowserCaptureBridge struct {
	uploads int
	started smartaccountssync.BrowserCaptureStartRequest
}

func (s *fakeBrowserCaptureBridge) StartBrowserCapture(_ context.Context, _ string, runID string, request smartaccountssync.BrowserCaptureStartRequest) (smartaccountssync.BrowserCaptureStatus, error) {
	s.started = request
	return smartaccountssync.BrowserCaptureStatus{RunID: runID, Status: "open", ManifestVersion: request.ManifestVersion, Scope: request.Scope, Resources: []smartaccountssync.BrowserCaptureResourceStatus{{ResourceID: request.Scope.ResourceIDs[0], Coverage: "export_csv", Status: "pending"}}}, nil
}

func (s *fakeBrowserCaptureBridge) GetBrowserCapture(_ context.Context, _ string, runID string) (smartaccountssync.BrowserCaptureStatus, error) {
	return smartaccountssync.BrowserCaptureStatus{RunID: runID, Status: "open", ManifestVersion: smartaccountssync.BrowserCaptureManifestVersion, Scope: s.started.Scope, Resources: []smartaccountssync.BrowserCaptureResourceStatus{{ResourceID: s.started.Scope.ResourceIDs[0], Coverage: "export_csv", Status: "pending"}}}, nil
}

func (s *fakeBrowserCaptureBridge) UploadBrowserCaptureResource(_ context.Context, _ string, _ string, resourceID, _ string, _ string, _ []byte) (smartaccountssync.BrowserCaptureResourceStatus, error) {
	s.uploads++
	return smartaccountssync.BrowserCaptureResourceStatus{ResourceID: resourceID, Coverage: "export_csv", Status: "sealed", Created: true}, nil
}

func (s *fakeBrowserCaptureBridge) FinalizeBrowserCapture(_ context.Context, _ string, runID string) (smartaccountssync.BrowserCaptureStatus, error) {
	return smartaccountssync.BrowserCaptureStatus{RunID: runID, Status: "finalized_partial", ManifestVersion: smartaccountssync.BrowserCaptureManifestVersion, Scope: smartaccountssync.BrowserCaptureScope{Mode: "partial", FromInclusive: "2024-01-01", ToInclusive: "2024-01-31", CutoffAt: "2026-08-28T10:00:00Z", ResourceIDs: []string{smartaccountssync.BrowserGeneralLedgerResourceID}}, Resources: []smartaccountssync.BrowserCaptureResourceStatus{{ResourceID: smartaccountssync.BrowserGeneralLedgerResourceID, Coverage: "export_csv", Status: "completed"}}, Receipt: &smartaccountssync.BrowserCaptureCoverageReceipt{Status: "partial_coverage_recorded", CompletedExportCount: 1, RequiredExportCount: 1, FinalizedAt: "2026-08-28T10:01:00Z"}}, nil
}

func newSmartAccountsBrowserCaptureHandlers() (*Handlers, *fakeBrowserCaptureStore, *fakeBrowserCaptureBridge) {
	store := &fakeBrowserCaptureStore{controls: map[string]smartaccountssync.Control{
		smartAccountsControlKey(testBrowserPairingTenantID, testBrowserSourceID): {TenantID: testBrowserPairingTenantID, SourceCompanyID: testBrowserSourceID, SecretReference: "brave-session://0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3"},
	}}
	bridge := &fakeBrowserCaptureBridge{}
	service := smartaccountssync.NewBrowserCaptureService(store, store, bridge)
	// Use a stable UUID and token only for these focused handler tests.
	// The public handler never logs either field.
	return &Handlers{smartAccountsBrowserCaptureService: service}, store, bridge
}

func browserCaptureIssueRequest() *http.Request {
	body := `{"source_company_id":"` + testBrowserSourceID + `","manifest_version":"` + smartaccountssync.BrowserCaptureManifestVersion + `","scope":{"mode":"partial","from_inclusive":"2024-01-01","to_inclusive":"2024-01-31","cutoff_at":"2026-08-28T10:00:00Z","resource_ids":["` + smartaccountssync.BrowserGeneralLedgerResourceID + `"]}}`
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request = withClaims(request, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
	request = withURLParams(request, map[string]string{"tenantID": testBrowserPairingTenantID})
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestSmartAccountsBrowserCaptureOwnerIssueAndExtensionRelayAreScoped(t *testing.T) {
	h, store, bridge := newSmartAccountsBrowserCaptureHandlers()
	issueRecorder := httptest.NewRecorder()
	issueRequest := browserCaptureIssueRequest()
	require.Equal(t, testBrowserPairingTenantID, chi.URLParam(issueRequest, "tenantID"))
	h.IssueSmartAccountsBrowserCapture(issueRecorder, issueRequest)
	require.Equal(t, http.StatusCreated, issueRecorder.Code)
	assert.Equal(t, "no-store", issueRecorder.Header().Get("Cache-Control"))
	assert.Contains(t, issueRecorder.Body.String(), `"transfer_consent"`)
	assert.Contains(t, issueRecorder.Body.String(), `"tenant_id":"`+testBrowserPairingTenantID+`"`)

	var issue smartaccountssync.BrowserCaptureIssue
	require.NoError(t, json.NewDecoder(issueRecorder.Body).Decode(&issue))
	stored := store.auth[testBrowserPairingTenantID+"/"+issue.RunID]
	assert.NotEqual(t, issue.CaptureToken, stored.TokenSHA256)
	assert.NotContains(t, issueRecorder.Body.String(), stored.TokenSHA256)

	body := []byte("date,amount\n2024-01-01,1\n")
	digest := sha256.Sum256(body)
	upload := withURLParams(httptest.NewRequest(http.MethodPut, "/", strings.NewReader(string(body))), map[string]string{"tenantID": testBrowserPairingTenantID, "runID": issue.RunID, "resourceID": smartaccountssync.BrowserGeneralLedgerResourceID})
	upload.Header.Set("Origin", testBraveExtensionOrigin)
	upload.Header.Set("Authorization", "Bearer "+issue.CaptureToken)
	upload.Header.Set("Content-Type", "text/csv")
	upload.Header.Set("X-SA-Browser-Resource-SHA256", hex.EncodeToString(digest[:]))
	uploadRecorder := httptest.NewRecorder()
	h.UploadSmartAccountsBrowserCaptureResource(uploadRecorder, upload)
	require.Equal(t, http.StatusOK, uploadRecorder.Code)
	assert.Equal(t, testBraveExtensionOrigin, uploadRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, 1, bridge.uploads)

	crossTenant := withURLParams(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{"tenantID": "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", "runID": issue.RunID})
	crossTenant.Header.Set("Origin", testBraveExtensionOrigin)
	crossTenant.Header.Set("Authorization", "Bearer "+issue.CaptureToken)
	crossTenantRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserCapture(crossTenantRecorder, crossTenant)
	assert.Equal(t, http.StatusUnauthorized, crossTenantRecorder.Code)
}

func TestSmartAccountsBrowserCaptureRejectsNonOwnerOriginAndScopeBypass(t *testing.T) {
	h, _, bridge := newSmartAccountsBrowserCaptureHandlers()
	nonOwner := browserCaptureIssueRequest()
	nonOwner = withClaims(nonOwner, createTestClaims("admin-1", "admin@example.com", testBrowserPairingTenantID, "admin"))
	nonOwnerRecorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserCapture(nonOwnerRecorder, nonOwner)
	assert.Equal(t, http.StatusForbidden, nonOwnerRecorder.Code)

	issueRecorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserCapture(issueRecorder, browserCaptureIssueRequest())
	require.Equal(t, http.StatusCreated, issueRecorder.Code)
	var issue smartaccountssync.BrowserCaptureIssue
	require.NoError(t, json.NewDecoder(issueRecorder.Body).Decode(&issue))
	body := []byte("x\n")
	digest := sha256.Sum256(body)
	request := withURLParams(httptest.NewRequest(http.MethodPut, "/?nope=1", strings.NewReader(string(body))), map[string]string{"tenantID": testBrowserPairingTenantID, "runID": issue.RunID, "resourceID": "payments"})
	request.Header.Set("Origin", "https://smartaccounts.eu")
	request.Header.Set("Authorization", "Bearer "+issue.CaptureToken)
	request.Header.Set("Content-Type", "text/csv")
	request.Header.Set("X-SA-Browser-Resource-SHA256", hex.EncodeToString(digest[:]))
	recorder := httptest.NewRecorder()
	h.UploadSmartAccountsBrowserCaptureResource(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Zero(t, bridge.uploads)
}

func TestSmartAccountsBrowserCaptureRejectsUnreviewedSalesInvoiceScope(t *testing.T) {
	h, _, bridge := newSmartAccountsBrowserCaptureHandlers()
	request := browserCaptureIssueRequest()
	request.Body = io.NopCloser(strings.NewReader(`{"source_company_id":"` + testBrowserSourceID + `","manifest_version":"` + smartaccountssync.BrowserCaptureManifestVersion + `","scope":{"mode":"partial","from_inclusive":"2024-01-01","to_inclusive":"2024-01-31","cutoff_at":"2026-08-28T10:00:00Z","resource_ids":["sales_invoices"]}}`))
	recorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserCapture(recorder, request)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, bridge.uploads)
	assert.Empty(t, bridge.started.Scope.ResourceIDs)
}

func TestSmartAccountsBrowserCaptureResumeRequiresOwnerConsentAndKeepsRun(t *testing.T) {
	h, _, _ := newSmartAccountsBrowserCaptureHandlers()
	issued := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserCapture(issued, browserCaptureIssueRequest())
	require.Equal(t, http.StatusCreated, issued.Code)
	var original smartaccountssync.BrowserCaptureIssue
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&original))

	resume := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request = withClaims(request, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
		request = withURLParams(request, map[string]string{"tenantID": testBrowserPairingTenantID, "runID": original.RunID})
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.ResumeSmartAccountsBrowserCapture(recorder, request)
		return recorder
	}
	assert.Equal(t, http.StatusBadRequest, resume(`{"transfer_consent_confirmed":false}`).Code)
	resumed := resume(`{"transfer_consent_confirmed":true}`)
	require.Equal(t, http.StatusCreated, resumed.Code)
	var result smartaccountssync.BrowserCaptureIssue
	require.NoError(t, json.NewDecoder(resumed.Body).Decode(&result))
	assert.Equal(t, original.RunID, result.RunID)
	assert.Equal(t, original.Scope, result.Scope)
	assert.NotEqual(t, original.CaptureToken, result.CaptureToken)
}

func TestSmartAccountsBrowserCaptureOwnerStatusIsBoundAndOmitsCapability(t *testing.T) {
	h, _, _ := newSmartAccountsBrowserCaptureHandlers()
	issued := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserCapture(issued, browserCaptureIssueRequest())
	require.Equal(t, http.StatusCreated, issued.Code)
	var issue smartaccountssync.BrowserCaptureIssue
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&issue))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = withClaims(request, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
	request = withURLParams(request, map[string]string{"tenantID": testBrowserPairingTenantID, "runID": issue.RunID})
	recorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserCaptureOwnerStatus(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Contains(t, recorder.Body.String(), `"tenant_id":"`+testBrowserPairingTenantID+`"`)
	assert.Contains(t, recorder.Body.String(), `"source_company_id":"`+testBrowserSourceID+`"`)
	assert.NotContains(t, recorder.Body.String(), issue.CaptureToken)
	assert.NotContains(t, recorder.Body.String(), `capture_token`)

	nonOwner := httptest.NewRequest(http.MethodGet, "/", nil)
	nonOwner = withClaims(nonOwner, createTestClaims("admin-1", "admin@example.com", testBrowserPairingTenantID, "admin"))
	nonOwner = withURLParams(nonOwner, map[string]string{"tenantID": testBrowserPairingTenantID, "runID": issue.RunID})
	nonOwnerRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserCaptureOwnerStatus(nonOwnerRecorder, nonOwner)
	assert.Equal(t, http.StatusForbidden, nonOwnerRecorder.Code)
}

func TestSmartAccountsBrowserCaptureRouteCORS(t *testing.T) {
	h, _, _ := newSmartAccountsBrowserCaptureHandlers()
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/smartaccounts-browser-captures/tenants/"+testBrowserPairingTenantID+"/runs/"+testBrowserCaptureRunID, nil)
	request.Header.Set("Origin", testBraveExtensionOrigin)
	recorder := httptest.NewRecorder()
	h.OptionsSmartAccountsBrowserCapture(recorder, request)
	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), "PUT")
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "X-SA-Browser-Resource-SHA256")
}
