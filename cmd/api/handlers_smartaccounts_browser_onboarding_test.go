package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type onboardingHandlerStore struct {
	fakeBrowserPairingStore
	bindings map[string]smartaccountssync.BrowserOnboardingBinding
}

func (s *onboardingHandlerStore) GetBrowserOnboarding(_ context.Context, sourceCompanyID string) (*smartaccountssync.BrowserOnboardingBinding, error) {
	binding, exists := s.bindings[sourceCompanyID]
	if !exists {
		return nil, smartaccountssync.ErrBrowserOnboardingNotFound
	}
	return &binding, nil
}

func (s *onboardingHandlerStore) CreateBrowserOnboarding(_ context.Context, binding smartaccountssync.BrowserOnboardingBinding) (*smartaccountssync.BrowserOnboardingBinding, bool, error) {
	if s.bindings == nil {
		s.bindings = map[string]smartaccountssync.BrowserOnboardingBinding{}
	}
	if _, exists := s.bindings[binding.SourceCompanyID]; exists {
		return nil, false, nil
	}
	s.bindings[binding.SourceCompanyID] = binding
	return &binding, true, nil
}

func (s *onboardingHandlerStore) SetBrowserOnboardingTarget(_ context.Context, sourceCompanyID, tenantID, tenantName string) (*smartaccountssync.BrowserOnboardingBinding, error) {
	binding, exists := s.bindings[sourceCompanyID]
	if !exists {
		return nil, smartaccountssync.ErrBrowserOnboardingNotFound
	}
	if binding.TenantID == "" {
		binding.TenantID, binding.TenantName, binding.Status = tenantID, tenantName, smartaccountssync.BrowserOnboardingTargetReady
		s.bindings[sourceCompanyID] = binding
	}
	return &binding, nil
}

func (s *onboardingHandlerStore) SetBrowserOnboardingPairing(_ context.Context, sourceCompanyID, pairingID string) (*smartaccountssync.BrowserOnboardingBinding, error) {
	binding, exists := s.bindings[sourceCompanyID]
	if !exists || binding.TenantID == "" {
		return nil, smartaccountssync.ErrBrowserOnboardingNotFound
	}
	binding.PairingID, binding.Status = pairingID, smartaccountssync.BrowserOnboardingPairingIssued
	s.bindings[sourceCompanyID] = binding
	return &binding, nil
}

func (s *onboardingHandlerStore) FindBrowserOnboardingTargets(_ context.Context, _ string) ([]smartaccountssync.BrowserOnboardingBinding, error) {
	return nil, nil
}

func newSmartAccountsBrowserOnboardingHandlers() (*Handlers, *onboardingHandlerStore) {
	store := &onboardingHandlerStore{}
	tenantRepo := newMockTenantRepository()
	tenantService := tenant.NewServiceWithRepository(tenantRepo)
	syncService := smartaccountssync.NewService(store, smartaccountssync.UnavailableBridgeCatalog{})
	pairingService := smartaccountssync.NewBrowserPairingService(store, syncService)
	onboardingService := smartaccountssync.NewBrowserOnboardingService(store, tenantService, pairingService)
	return &Handlers{
		tenantService:                         tenantService,
		smartAccountsSyncService:              syncService,
		smartAccountsBrowserPairingService:    pairingService,
		smartAccountsBrowserOnboardingService: onboardingService,
	}, store
}

func TestSmartAccountsBrowserOnboardingCreatesTargetThenRequiresExpectedSourceClaim(t *testing.T) {
	h, store := newSmartAccountsBrowserOnboardingHandlers()
	body := `{"sources":[{"source_company_id":"sa-browser-v1-123456","source_company_name":"Hold My Beer OÜ"}],"create_missing_tenants_confirmed":true}`
	request := makeAuthenticatedRequest(http.MethodPost, "/smartaccounts-sync/browser-onboarding", json.RawMessage(body), createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner))
	request.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()

	h.StartSmartAccountsBrowserOnboarding(created, request)

	require.Equal(t, http.StatusOK, created.Code, created.Body.String())
	require.Equal(t, "no-store", created.Header().Get("Cache-Control"))
	var response smartaccountssync.BrowserOnboardingResponse
	require.NoError(t, json.NewDecoder(created.Body).Decode(&response))
	require.Len(t, response.Bindings, 1)
	binding := response.Bindings[0]
	require.NotNil(t, binding.Pairing)
	assert.True(t, binding.TenantCreated)
	assert.Equal(t, smartaccountssync.BrowserOnboardingPairingIssued, binding.Status)
	assert.Equal(t, "sa-browser-v1-123456", store.pairings[binding.Pairing.PairingID].ExpectedSourceCompanyID)

	wrongClaim := withURLParams(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"source_company_id":"sa-browser-v1-654321"}`)), map[string]string{"pairingID": binding.Pairing.PairingID})
	wrongClaim.Header.Set("Content-Type", "application/json")
	wrongClaim.Header.Set("Origin", testBraveExtensionOrigin)
	wrongClaim.Header.Set("Authorization", "Bearer "+binding.Pairing.PairingToken)
	wrongClaimRecorder := httptest.NewRecorder()
	h.ClaimSmartAccountsBrowserPairing(wrongClaimRecorder, wrongClaim)
	assert.Equal(t, http.StatusUnauthorized, wrongClaimRecorder.Code)
	assert.Empty(t, store.controls)

	claim := withURLParams(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"source_company_id":"sa-browser-v1-123456"}`)), map[string]string{"pairingID": binding.Pairing.PairingID})
	claim.Header.Set("Content-Type", "application/json")
	claim.Header.Set("Origin", testBraveExtensionOrigin)
	claim.Header.Set("Authorization", "Bearer "+binding.Pairing.PairingToken)
	claimRecorder := httptest.NewRecorder()
	h.ClaimSmartAccountsBrowserPairing(claimRecorder, claim)
	require.Equal(t, http.StatusOK, claimRecorder.Code)
	assert.Len(t, store.controls, 1)

	status := makeAuthenticatedRequest(http.MethodGet, "/", nil, createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner))
	status = withURLParams(status, map[string]string{"sourceCompanyID": "sa-browser-v1-123456"})
	statusRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserOnboarding(statusRecorder, status)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	assert.Contains(t, statusRecorder.Body.String(), `"status":"PAIRED"`)
	assert.NotContains(t, statusRecorder.Body.String(), binding.Pairing.PairingToken)
	assert.NotContains(t, statusRecorder.Body.String(), store.pairings[binding.Pairing.PairingID].TokenSHA256)
}

func TestSmartAccountsBrowserOnboardingRejectsUnconfirmedAndAPITokenRequests(t *testing.T) {
	h, _ := newSmartAccountsBrowserOnboardingHandlers()
	for _, request := range []*http.Request{
		makeAuthenticatedRequest(http.MethodPost, "/", json.RawMessage(`{"sources":[{"source_company_id":"sa-browser-v1-123456","source_company_name":"Hold My Beer OÜ"}]}`), createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner)),
		makeAuthenticatedRequest(http.MethodPost, "/", json.RawMessage(`{"sources":[{"source_company_id":"sa-browser-v1-123456","source_company_name":"Hold My Beer OÜ"}],"create_missing_tenants_confirmed":true}`), &auth.Claims{UserID: "owner-1", TokenKind: auth.TokenKindAPIToken}),
	} {
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.StartSmartAccountsBrowserOnboarding(recorder, request)
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, recorder.Code)
	}
}
