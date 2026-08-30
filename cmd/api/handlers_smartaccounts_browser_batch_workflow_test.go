package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/stretchr/testify/assert"
)

func newSmartAccountsBrowserBatchWorkflowHandlers() *Handlers {
	// A non-nil coordinator lets these handler tests reach authentication and
	// strict decoding independently from database/bridge dependencies.
	return &Handlers{smartAccountsBrowserBatchWorkflowActions: smartaccountssync.NewBrowserBatchWorkflowActionsService(nil, nil, nil, nil, nil)}
}

func browserBatchWorkflowRequest(method, body string, claims *auth.Claims) *http.Request {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withClaims(req, claims)
	return withURLParams(req, map[string]string{"batchID": "9c52d95f-4294-4f98-90cc-a572d9f5bc33", "sourceCompanyID": "sa-browser-v1-1"})
}

func TestSmartAccountsBrowserBatchWorkflowHandlersAreUserOwnerOnlyAndStrict(t *testing.T) {
	h := newSmartAccountsBrowserBatchWorkflowHandlers()

	missing := httptest.NewRecorder()
	h.GetSmartAccountsBrowserBatchWorkflow(missing, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusUnauthorized, missing.Code)

	apiToken := httptest.NewRecorder()
	h.GetSmartAccountsBrowserBatchWorkflow(apiToken, browserBatchWorkflowRequest(http.MethodGet, "", &auth.Claims{UserID: "owner-1", TokenKind: auth.TokenKindAPIToken}))
	assert.Equal(t, http.StatusForbidden, apiToken.Code)

	advanceAPIToken := httptest.NewRecorder()
	h.AdvanceSmartAccountsBrowserBatchWorkflowSafe(advanceAPIToken, browserBatchWorkflowRequest(http.MethodPost, `{}`, &auth.Claims{UserID: "owner-1", TokenKind: auth.TokenKindAPIToken}))
	assert.Equal(t, http.StatusForbidden, advanceAPIToken.Code)

	unknown := httptest.NewRecorder()
	h.PrepareSmartAccountsBrowserBatchWorkflow(unknown, browserBatchWorkflowRequest(http.MethodPost, `{"history_from":"2020-01-01","owner_confirmed":true,"metadata_discovery_consent_confirmed":true,"header_probe_consent_confirmed":true,"capture_token":"must-not-be-accepted"}`, createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner)))
	assert.Equal(t, http.StatusBadRequest, unknown.Code)

	status := httptest.NewRecorder()
	h.GetSmartAccountsBrowserBatchWorkflow(status, browserBatchWorkflowRequest(http.MethodGet, "", createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner)))
	assert.Equal(t, http.StatusServiceUnavailable, status.Code)
	assert.NotContains(t, status.Body.String(), "capture_token")
}

func TestSmartAccountsBrowserBatchWorkflowHandlerRejectsQueryAndEmptyActionBodies(t *testing.T) {
	h := newSmartAccountsBrowserBatchWorkflowHandlers()
	claims := createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner)

	query := browserBatchWorkflowRequest(http.MethodPost, `{}`, claims)
	query.URL.RawQuery = "unexpected=true"
	queryRecorder := httptest.NewRecorder()
	h.ResumeSmartAccountsBrowserBatchWorkflow(queryRecorder, query)
	assert.Equal(t, http.StatusBadRequest, queryRecorder.Code)

	empty := browserBatchWorkflowRequest(http.MethodPost, "", claims)
	emptyRecorder := httptest.NewRecorder()
	h.OpenSmartAccountsBrowserBatchTransferConfirmation(emptyRecorder, empty)
	assert.Equal(t, http.StatusBadRequest, emptyRecorder.Code)

	advanceUnknown := httptest.NewRecorder()
	h.AdvanceSmartAccountsBrowserBatchWorkflowSafe(advanceUnknown, browserBatchWorkflowRequest(http.MethodPost, `{}`, claims))
	assert.Equal(t, http.StatusServiceUnavailable, advanceUnknown.Code)
	assert.NotContains(t, advanceUnknown.Body.String(), "capture_token")

	advanceQuery := browserBatchWorkflowRequest(http.MethodPost, `{}`, claims)
	advanceQuery.URL.RawQuery = "unexpected=true"
	advanceQueryRecorder := httptest.NewRecorder()
	h.AdvanceSmartAccountsBrowserBatchWorkflowSafe(advanceQueryRecorder, advanceQuery)
	assert.Equal(t, http.StatusBadRequest, advanceQueryRecorder.Code)
}
