package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSmartAccountsBrowserCaptureWorkflowHandlers() (*Handlers, *fakeBrowserCaptureStore) {
	handlers, store, _ := newSmartAccountsBrowserCaptureHandlers()
	workflow := smartaccountssync.NewBrowserCaptureWorkflowService(store, store, handlers.smartAccountsBrowserCaptureService)
	handlers.smartAccountsBrowserCaptureWorkflowService = workflow
	return handlers, store
}

func browserCaptureWorkflowHTTPRequest(consent bool, from string) *http.Request {
	body := `{"source_company_id":"` + testBrowserSourceID + `","from_inclusive":"` + from + `","transfer_consent_confirmed":` + map[bool]string{true: "true", false: "false"}[consent] + `}`
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request = withClaims(request, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
	request = withURLParams(request, map[string]string{"tenantID": testBrowserPairingTenantID})
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestSmartAccountsBrowserCaptureWorkflowDerivesPlanIssuesOnlyWithConsentAndResumesSameRun(t *testing.T) {
	handlers, store := newSmartAccountsBrowserCaptureWorkflowHandlers()
	readyRecorder := httptest.NewRecorder()
	handlers.StartSmartAccountsBrowserCaptureWorkflow(readyRecorder, browserCaptureWorkflowHTTPRequest(false, "2024-01-01"))
	require.Equal(t, http.StatusOK, readyRecorder.Code)
	assert.Equal(t, "no-store", readyRecorder.Header().Get("Cache-Control"))
	var ready smartaccountssync.BrowserCaptureWorkflowStatus
	require.NoError(t, json.NewDecoder(readyRecorder.Body).Decode(&ready))
	assert.Nil(t, ready.Capture)
	assert.Equal(t, smartaccountssync.BrowserCaptureWorkflowReady, ready.Status)
	assert.Equal(t, time.Now().UTC().Format(time.DateOnly), ready.Plan.Scope.ToInclusive)
	assert.NotEmpty(t, ready.Plan.Scope.CutoffAt)
	assert.Equal(t, []string{smartaccountssync.BrowserGeneralLedgerResourceID}, ready.Plan.EligibleResourceIDs)

	issuedRecorder := httptest.NewRecorder()
	handlers.StartSmartAccountsBrowserCaptureWorkflow(issuedRecorder, browserCaptureWorkflowHTTPRequest(true, "2024-01-01"))
	require.Equal(t, http.StatusOK, issuedRecorder.Code)
	issuedBody := issuedRecorder.Body.String()
	var issued smartaccountssync.BrowserCaptureWorkflowStatus
	require.NoError(t, json.NewDecoder(issuedRecorder.Body).Decode(&issued))
	require.NotNil(t, issued.Capture)
	assert.Equal(t, ready.WorkflowID, issued.WorkflowID)
	assert.Equal(t, []string{smartaccountssync.BrowserGeneralLedgerResourceID}, issued.Plan.Scope.ResourceIDs)
	assert.Contains(t, issuedBody, `"transfer_consent"`)
	assert.NotContains(t, issuedBody, store.auth[testBrowserPairingTenantID+"/"+issued.Capture.RunID].TokenSHA256)

	retryRecorder := httptest.NewRecorder()
	handlers.StartSmartAccountsBrowserCaptureWorkflow(retryRecorder, browserCaptureWorkflowHTTPRequest(true, "2024-01-01"))
	require.Equal(t, http.StatusOK, retryRecorder.Code)
	var retry smartaccountssync.BrowserCaptureWorkflowStatus
	require.NoError(t, json.NewDecoder(retryRecorder.Body).Decode(&retry))
	require.NotNil(t, retry.Capture)
	assert.Equal(t, issued.WorkflowID, retry.WorkflowID)
	assert.Equal(t, issued.Capture.RunID, retry.Capture.RunID)
	assert.Len(t, store.workflowsByID, 1)
	assert.Len(t, store.auth, 1)
}

func TestSmartAccountsBrowserCaptureWorkflowStatusIsOwnerScopedAndInvalidBoundaryFailsClosed(t *testing.T) {
	handlers, _ := newSmartAccountsBrowserCaptureWorkflowHandlers()
	invalid := httptest.NewRecorder()
	handlers.StartSmartAccountsBrowserCaptureWorkflow(invalid, browserCaptureWorkflowHTTPRequest(false, "2027-01-01"))
	assert.Equal(t, http.StatusBadRequest, invalid.Code)

	created := httptest.NewRecorder()
	handlers.StartSmartAccountsBrowserCaptureWorkflow(created, browserCaptureWorkflowHTTPRequest(true, "2024-01-01"))
	require.Equal(t, http.StatusOK, created.Code)
	var workflow smartaccountssync.BrowserCaptureWorkflowStatus
	require.NoError(t, json.NewDecoder(created.Body).Decode(&workflow))

	status := httptest.NewRequest(http.MethodGet, "/", nil)
	status = withClaims(status, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
	status = withURLParams(status, map[string]string{"tenantID": testBrowserPairingTenantID, "workflowID": workflow.WorkflowID})
	statusRecorder := httptest.NewRecorder()
	handlers.GetSmartAccountsBrowserCaptureWorkflowStatus(statusRecorder, status)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	assert.NotContains(t, statusRecorder.Body.String(), `capture_token`)
	assert.NotContains(t, statusRecorder.Body.String(), workflow.Capture.CaptureToken)
	assert.Contains(t, statusRecorder.Body.String(), `"source_company_id":"`+testBrowserSourceID+`"`)

	crossTenant := httptest.NewRequest(http.MethodGet, "/", nil)
	crossTenant = withClaims(crossTenant, createTestClaims("owner-2", "owner2@example.com", "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", "owner"))
	crossTenant = withURLParams(crossTenant, map[string]string{"tenantID": "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", "workflowID": workflow.WorkflowID})
	crossTenantRecorder := httptest.NewRecorder()
	handlers.GetSmartAccountsBrowserCaptureWorkflowStatus(crossTenantRecorder, crossTenant)
	assert.Equal(t, http.StatusNotFound, crossTenantRecorder.Code)
}
