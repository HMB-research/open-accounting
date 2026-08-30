package main

import (
	"context"
	"encoding/json"
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

const (
	testBrowserPairingTenantID = "b436c224-5df5-4b4d-a772-1897f9147400"
	testBrowserSourceID        = "sa-browser-v1-123456"
	testBraveExtensionOrigin   = "chrome-extension://abcdefghijklmnopabcdefghijklmnop"
)

type fakeBrowserPairingStore struct {
	fakeSmartAccountsSyncStore
	pairings map[string]smartaccountssync.BrowserPairing
}

func (s *fakeBrowserPairingStore) CreateBrowserPairing(_ context.Context, pairing smartaccountssync.BrowserPairing) error {
	if s.pairings == nil {
		s.pairings = map[string]smartaccountssync.BrowserPairing{}
	}
	if _, exists := s.pairings[pairing.ID]; exists {
		return errors.New("pairing already exists")
	}
	s.pairings[pairing.ID] = pairing
	return nil
}

func (s *fakeBrowserPairingStore) GetBrowserPairing(_ context.Context, pairingID, tenantID string) (*smartaccountssync.BrowserPairing, error) {
	pairing, exists := s.pairings[pairingID]
	if !exists || pairing.TenantID != tenantID {
		return nil, smartaccountssync.ErrBrowserPairingNotClaimable
	}
	return &pairing, nil
}

func (s *fakeBrowserPairingStore) ClaimBrowserPairing(_ context.Context, pairingID, tokenSHA256, sourceCompanyID string, claimedAt time.Time) (*smartaccountssync.BrowserPairing, error) {
	pairing, exists := s.pairings[pairingID]
	if !exists || pairing.Status != smartaccountssync.BrowserPairingStatusIssued || pairing.TokenSHA256 != tokenSHA256 || !pairing.ExpiresAt.After(claimedAt) || (pairing.ExpectedSourceCompanyID != "" && pairing.ExpectedSourceCompanyID != sourceCompanyID) {
		return nil, smartaccountssync.ErrBrowserPairingNotClaimable
	}
	pairing.Status = smartaccountssync.BrowserPairingStatusClaimed
	pairing.SourceCompanyID = sourceCompanyID
	pairing.ClaimedAt = &claimedAt
	s.pairings[pairingID] = pairing
	return &pairing, nil
}

func newSmartAccountsBrowserPairingHandlers() (*Handlers, *fakeBrowserPairingStore) {
	store := &fakeBrowserPairingStore{}
	syncService := smartaccountssync.NewService(store, smartaccountssync.UnavailableBridgeCatalog{})
	pairingService := smartaccountssync.NewBrowserPairingService(store, syncService)
	return &Handlers{smartAccountsSyncService: syncService, smartAccountsBrowserPairingService: pairingService}, store
}

func TestSmartAccountsBrowserPairingIssueClaimAndOwnerStatusAreMinimalAndRedacted(t *testing.T) {
	h, store := newSmartAccountsBrowserPairingHandlers()
	issueRequest := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+testBrowserPairingTenantID+"/smartaccounts-sync/browser-pairings", nil, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner")), map[string]string{"tenantID": testBrowserPairingTenantID})
	issueRecorder := httptest.NewRecorder()

	h.IssueSmartAccountsBrowserPairing(issueRecorder, issueRequest)

	require.Equal(t, http.StatusCreated, issueRecorder.Code)
	require.Equal(t, "no-store", issueRecorder.Header().Get("Cache-Control"))
	var issue smartaccountssync.BrowserPairingIssue
	require.NoError(t, json.NewDecoder(issueRecorder.Body).Decode(&issue))
	assert.NotEmpty(t, issue.PairingID)
	assert.Len(t, issue.PairingToken, 43)
	stored := store.pairings[issue.PairingID]
	assert.NotEqual(t, issue.PairingToken, stored.TokenSHA256)
	assert.NotContains(t, issueRecorder.Body.String(), stored.TokenSHA256)

	claimRequest := withURLParams(httptest.NewRequest(http.MethodPost, "/api/v1/smartaccounts-browser-pairings/"+issue.PairingID+"/claim", strings.NewReader(`{"source_company_id":"`+testBrowserSourceID+`"}`)), map[string]string{"pairingID": issue.PairingID})
	claimRequest.Header.Set("Content-Type", "application/json")
	claimRequest.Header.Set("Origin", testBraveExtensionOrigin)
	claimRequest.Header.Set("Authorization", "Bearer "+issue.PairingToken)
	claimRecorder := httptest.NewRecorder()

	h.ClaimSmartAccountsBrowserPairing(claimRecorder, claimRequest)

	require.Equal(t, http.StatusOK, claimRecorder.Code)
	assert.Equal(t, testBraveExtensionOrigin, claimRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "no-store", claimRecorder.Header().Get("Cache-Control"))
	assert.JSONEq(t, `{"status":"CLAIMED"}`, claimRecorder.Body.String())
	assert.NotContains(t, claimRecorder.Body.String(), issue.PairingToken)
	assert.NotContains(t, claimRecorder.Body.String(), testBrowserSourceID)
	control := store.controls[smartAccountsControlKey(testBrowserPairingTenantID, testBrowserSourceID)]
	assert.Equal(t, "brave-session://"+issue.PairingID, control.SecretReference)
	assert.NotContains(t, mustMarshalJSON(t, control), issue.PairingToken)

	statusRequest := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+testBrowserPairingTenantID+"/smartaccounts-sync/browser-pairings/"+issue.PairingID, nil, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner")), map[string]string{"tenantID": testBrowserPairingTenantID, "pairingID": issue.PairingID})
	statusRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserPairing(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	assert.Contains(t, statusRecorder.Body.String(), testBrowserSourceID)
	assert.NotContains(t, statusRecorder.Body.String(), issue.PairingToken)
	assert.NotContains(t, statusRecorder.Body.String(), stored.TokenSHA256)
}

func TestSmartAccountsBrowserPairingClaimRejectsNonExtensionWrongAndReplayTokens(t *testing.T) {
	h, _ := newSmartAccountsBrowserPairingHandlers()
	issueRequest := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/", nil, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner")), map[string]string{"tenantID": testBrowserPairingTenantID})
	issueRecorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserPairing(issueRecorder, issueRequest)
	require.Equal(t, http.StatusCreated, issueRecorder.Code)
	var issue smartaccountssync.BrowserPairingIssue
	require.NoError(t, json.NewDecoder(issueRecorder.Body).Decode(&issue))

	makeClaim := func(origin, token string) *httptest.ResponseRecorder {
		req := withURLParams(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"source_company_id":"`+testBrowserSourceID+`"}`)), map[string]string{"pairingID": issue.PairingID})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ClaimSmartAccountsBrowserPairing(w, req)
		return w
	}

	assert.Equal(t, http.StatusForbidden, makeClaim("http://server-nuc", issue.PairingToken).Code)
	assert.Equal(t, http.StatusUnauthorized, makeClaim(testBraveExtensionOrigin, "wrong").Code)
	require.Equal(t, http.StatusOK, makeClaim(testBraveExtensionOrigin, issue.PairingToken).Code)
	replay := makeClaim(testBraveExtensionOrigin, issue.PairingToken)
	assert.Equal(t, http.StatusUnauthorized, replay.Code)
	assert.NotContains(t, replay.Body.String(), issue.PairingToken)
}

func TestSmartAccountsBrowserPairingOptionsAllowsOnlyBraveExtensionOrigin(t *testing.T) {
	h, _ := newSmartAccountsBrowserPairingHandlers()
	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodOptions, "/", nil)
	allowedRequest.Header.Set("Origin", testBraveExtensionOrigin)
	h.OptionsSmartAccountsBrowserPairingClaim(allowed, allowedRequest)
	assert.Equal(t, http.StatusNoContent, allowed.Code)
	assert.Equal(t, testBraveExtensionOrigin, allowed.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, allowed.Header().Get("Access-Control-Allow-Headers"), "Authorization")

	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodOptions, "/", nil)
	deniedRequest.Header.Set("Origin", "https://sa.smartaccounts.eu")
	h.OptionsSmartAccountsBrowserPairingClaim(denied, deniedRequest)
	assert.Equal(t, http.StatusForbidden, denied.Code)
}

func TestSmartAccountsBrowserPairingClaimIsReachableThroughRouterCORS(t *testing.T) {
	h, _ := newSmartAccountsBrowserPairingHandlers()
	issueRequest := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/", nil, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner")), map[string]string{"tenantID": testBrowserPairingTenantID})
	issueRecorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserPairing(issueRecorder, issueRequest)
	require.Equal(t, http.StatusCreated, issueRecorder.Code)
	var issue smartaccountssync.BrowserPairingIssue
	require.NoError(t, json.NewDecoder(issueRecorder.Body).Decode(&issue))

	router := setupRouter(&Config{}, h, nil)
	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/smartaccounts-browser-pairings/"+issue.PairingID+"/claim", nil)
	preflight.Header.Set("Origin", testBraveExtensionOrigin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightRecorder := httptest.NewRecorder()
	router.ServeHTTP(preflightRecorder, preflight)
	require.Equal(t, http.StatusOK, preflightRecorder.Code)
	assert.Equal(t, testBraveExtensionOrigin, preflightRecorder.Header().Get("Access-Control-Allow-Origin"))

	claim := httptest.NewRequest(http.MethodPost, "/api/v1/smartaccounts-browser-pairings/"+issue.PairingID+"/claim", strings.NewReader(`{"source_company_id":"`+testBrowserSourceID+`"}`))
	claim.Header.Set("Origin", testBraveExtensionOrigin)
	claim.Header.Set("Content-Type", "application/json")
	claim.Header.Set("Authorization", "Bearer "+issue.PairingToken)
	claimRecorder := httptest.NewRecorder()
	router.ServeHTTP(claimRecorder, claim)
	assert.Equal(t, http.StatusOK, claimRecorder.Code)
	assert.Equal(t, testBraveExtensionOrigin, claimRecorder.Header().Get("Access-Control-Allow-Origin"))
}

func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
