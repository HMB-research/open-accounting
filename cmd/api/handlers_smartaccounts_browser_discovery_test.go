package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type discoveryHandlerStore struct {
	controls       map[string]smartaccountssync.Control
	authorizations map[string]smartaccountssync.BrowserDiscoveryAuthorization
	receipts       map[string]smartaccountssync.BrowserDiscoveryReceipt
}

func (s *discoveryHandlerStore) Get(_ context.Context, tenantID, sourceCompanyID string) (*smartaccountssync.Control, error) {
	control, found := s.controls[smartAccountsControlKey(tenantID, sourceCompanyID)]
	if !found {
		return nil, smartaccountssync.ErrControlNotConfigured
	}
	return &control, nil
}

func (s *discoveryHandlerStore) CreateBrowserDiscoveryAuthorization(_ context.Context, authorization smartaccountssync.BrowserDiscoveryAuthorization) error {
	if s.authorizations == nil {
		s.authorizations = map[string]smartaccountssync.BrowserDiscoveryAuthorization{}
	}
	key := authorization.TenantID + "/" + authorization.DiscoveryID
	if _, found := s.authorizations[key]; found {
		return errors.New("duplicate")
	}
	s.authorizations[key] = authorization
	return nil
}

func (s *discoveryHandlerStore) GetBrowserDiscoveryAuthorization(_ context.Context, tenantID, discoveryID string) (*smartaccountssync.BrowserDiscoveryAuthorization, error) {
	authorization, found := s.authorizations[tenantID+"/"+discoveryID]
	if !found {
		return nil, smartaccountssync.ErrBrowserDiscoveryUnauthorized
	}
	return &authorization, nil
}

func (s *discoveryHandlerStore) SaveBrowserDiscoveryReceipt(_ context.Context, tenantID, discoveryID string, receipt smartaccountssync.BrowserDiscoveryReceipt, _ time.Time) error {
	if _, found := s.authorizations[tenantID+"/"+discoveryID]; !found {
		return smartaccountssync.ErrBrowserDiscoveryUnauthorized
	}
	if s.receipts == nil {
		s.receipts = map[string]smartaccountssync.BrowserDiscoveryReceipt{}
	}
	s.receipts[tenantID+"/"+discoveryID] = receipt
	return nil
}

type discoveryHandlerBridge struct {
	recordCalls int
	getCalls    int
	tenantID    string
	sourceID    string
	discoveryID string
	request     smartaccountssync.BrowserDiscoveryBridgeReceiptRequest
}

func (b *discoveryHandlerBridge) RecordBrowserDiscoveryReceipt(_ context.Context, tenantID, sourceCompanyID, discoveryID string, request smartaccountssync.BrowserDiscoveryBridgeReceiptRequest) (smartaccountssync.BrowserDiscoveryReceipt, error) {
	b.recordCalls++
	b.tenantID, b.sourceID, b.discoveryID, b.request = tenantID, sourceCompanyID, discoveryID, request
	return discoveryHandlerReceipt(discoveryID), nil
}

func (b *discoveryHandlerBridge) GetBrowserDiscoveryReceipt(_ context.Context, tenantID, sourceCompanyID, discoveryID string) (smartaccountssync.BrowserDiscoveryReceipt, error) {
	b.getCalls++
	b.tenantID, b.sourceID, b.discoveryID = tenantID, sourceCompanyID, discoveryID
	return discoveryHandlerReceipt(discoveryID), nil
}

func discoveryHandlerReceipt(discoveryID string) smartaccountssync.BrowserDiscoveryReceipt {
	return smartaccountssync.BrowserDiscoveryReceipt{
		DiscoveryID: discoveryID, Status: "completed", ManifestVersion: smartaccountssync.BrowserDiscoveryManifestVersion,
		ContractVersion: smartaccountssync.BrowserDiscoveryContractVersion, ContractSHA256: strings.Repeat("a", 64),
		ResourceCount: 31, FilterRequiredCount: 24, PageOnlyRequiredCount: 7,
	}
}

func newSmartAccountsBrowserDiscoveryHandlers() (*Handlers, *discoveryHandlerStore, *discoveryHandlerBridge) {
	store := &discoveryHandlerStore{controls: map[string]smartaccountssync.Control{
		smartAccountsControlKey(testBrowserPairingTenantID, testBrowserSourceID): {
			TenantID: testBrowserPairingTenantID, SourceCompanyID: testBrowserSourceID,
			SecretReference: "brave-session://0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
		},
	}}
	bridge := &discoveryHandlerBridge{}
	service := smartaccountssync.NewBrowserDiscoveryService(store, store, bridge)
	return &Handlers{smartAccountsBrowserDiscoveryService: service}, store, bridge
}

func ownerDiscoveryRequest(method, body string, discoveryID string) *http.Request {
	request := httptest.NewRequest(method, "/", strings.NewReader(body))
	request = withClaims(request, createTestClaims("owner-1", "owner@example.com", testBrowserPairingTenantID, "owner"))
	params := map[string]string{"tenantID": testBrowserPairingTenantID}
	if discoveryID != "" {
		params["discoveryID"] = discoveryID
	}
	request = withURLParams(request, params)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestSmartAccountsBrowserDiscoveryIssueReceiptAndStatusAreOwnerBoundAndRedacted(t *testing.T) {
	handlers, store, bridge := newSmartAccountsBrowserDiscoveryHandlers()
	issued := httptest.NewRecorder()
	handlers.IssueSmartAccountsBrowserDiscovery(issued, ownerDiscoveryRequest(http.MethodPost, `{"source_company_id":"`+testBrowserSourceID+`","metadata_only_consent_confirmed":true,"response_header_probe_confirmed":false}`, ""))
	require.Equal(t, http.StatusCreated, issued.Code, issued.Body.String())
	require.Equal(t, "no-store", issued.Header().Get("Cache-Control"))
	var issue smartaccountssync.BrowserDiscoveryIssue
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&issue))
	assert.Equal(t, testBrowserPairingTenantID, issue.TenantID)
	assert.Equal(t, testBrowserSourceID, issue.SourceCompanyID)
	assert.Len(t, issue.ResourceIDs, 31)
	assert.Contains(t, issue.ResourceIDs, "warehouse_movements_report")
	assert.NotEqual(t, []string{"journal_entries"}, issue.ResourceIDs)
	assert.Equal(t, "metadata_only", issue.DiscoveryConsent.Scope)
	assert.NotContains(t, issued.Body.String(), "pairing_token")

	result := smartaccountssync.BrowserDiscoveryRelayResult{
		Source: "smartaccounts-browser-relay", Type: "smartaccounts-browser-relay.discovery-result.v1", Version: 1,
		DiscoveryID: issue.DiscoveryID, ManifestVersion: issue.ManifestVersion,
		ContractVersion: smartaccountssync.BrowserDiscoveryContractVersion, Status: "completed",
		Resources: handlerDiscoveryResources(issue.ResourceIDs),
	}
	resultJSON, err := json.Marshal(result)
	require.NoError(t, err)
	received := httptest.NewRecorder()
	handlers.ReceiveSmartAccountsBrowserDiscoveryReceipt(received, ownerDiscoveryRequest(http.MethodPost, string(resultJSON), issue.DiscoveryID))
	require.Equal(t, http.StatusOK, received.Code, received.Body.String())
	require.Equal(t, "no-store", received.Header().Get("Cache-Control"))
	assert.Contains(t, received.Body.String(), `"resource_count":31`)
	assert.NotContains(t, received.Body.String(), testBrowserSourceID)
	assert.NotContains(t, received.Body.String(), "filterResults")
	assert.NotContains(t, received.Body.String(), "Entry number")
	assert.Equal(t, 1, bridge.recordCalls)
	assert.Equal(t, testBrowserPairingTenantID, bridge.tenantID)
	assert.Equal(t, testBrowserSourceID, bridge.sourceID, "source binding is resolved only server-side")
	assert.Equal(t, testBrowserSourceID, bridge.request.SourceCompanyID)
	assert.Len(t, bridge.request.Resources, 31)
	assert.Equal(t, discoveryHandlerReceipt(issue.DiscoveryID), store.receipts[testBrowserPairingTenantID+"/"+issue.DiscoveryID])

	status := httptest.NewRecorder()
	handlers.GetSmartAccountsBrowserDiscoveryReceipt(status, ownerDiscoveryRequest(http.MethodGet, "", issue.DiscoveryID))
	require.Equal(t, http.StatusOK, status.Code, status.Body.String())
	assert.Equal(t, "no-store", status.Header().Get("Cache-Control"))
	assert.Contains(t, status.Body.String(), `"contract_sha256":"`+strings.Repeat("a", 64)+`"`)
	assert.NotContains(t, status.Body.String(), testBrowserSourceID)
	assert.NotContains(t, status.Body.String(), "filterResults")
	assert.Equal(t, 1, bridge.getCalls)
}

func TestSmartAccountsBrowserDiscoveryRejectsDataBearingAndCrossTenantReceiptRequests(t *testing.T) {
	handlers, _, bridge := newSmartAccountsBrowserDiscoveryHandlers()
	issued := httptest.NewRecorder()
	handlers.IssueSmartAccountsBrowserDiscovery(issued, ownerDiscoveryRequest(http.MethodPost, `{"source_company_id":"`+testBrowserSourceID+`","metadata_only_consent_confirmed":true,"response_header_probe_confirmed":false}`, ""))
	require.Equal(t, http.StatusCreated, issued.Code)
	var issue smartaccountssync.BrowserDiscoveryIssue
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&issue))

	dataBearing := httptest.NewRecorder()
	handlers.ReceiveSmartAccountsBrowserDiscoveryReceipt(dataBearing, ownerDiscoveryRequest(http.MethodPost, `{"source":"smartaccounts-browser-relay","type":"smartaccounts-browser-relay.discovery-result.v1","version":1,"discovery_id":"`+issue.DiscoveryID+`","manifest_version":"`+smartaccountssync.BrowserDiscoveryManifestVersion+`","contract_version":"smartaccounts-brave-discovery-contract-v1","status":"completed","resources":[],"cookies":"must-not-persist"}`, issue.DiscoveryID))
	assert.Equal(t, http.StatusBadRequest, dataBearing.Code)
	assert.Zero(t, bridge.recordCalls)
	assert.NotContains(t, dataBearing.Body.String(), "must-not-persist")

	crossTenant := ownerDiscoveryRequest(http.MethodGet, "", issue.DiscoveryID)
	crossTenant = withClaims(crossTenant, createTestClaims("owner-2", "owner@example.com", "other-tenant", "owner"))
	crossTenant = withURLParams(crossTenant, map[string]string{"tenantID": "other-tenant", "discoveryID": issue.DiscoveryID})
	crossTenantRecorder := httptest.NewRecorder()
	handlers.GetSmartAccountsBrowserDiscoveryReceipt(crossTenantRecorder, crossTenant)
	assert.Equal(t, http.StatusNotFound, crossTenantRecorder.Code)
	assert.Zero(t, bridge.getCalls)
}

func TestSmartAccountsBrowserDiscoveryReceiptBodyLimitMatchesPrivateBridge(t *testing.T) {
	handlers, _, bridge := newSmartAccountsBrowserDiscoveryHandlers()
	issued := httptest.NewRecorder()
	handlers.IssueSmartAccountsBrowserDiscovery(issued, ownerDiscoveryRequest(http.MethodPost, `{"source_company_id":"`+testBrowserSourceID+`","metadata_only_consent_confirmed":true,"response_header_probe_confirmed":true}`, ""))
	require.Equal(t, http.StatusCreated, issued.Code, issued.Body.String())
	var issue smartaccountssync.BrowserDiscoveryIssue
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&issue))

	headerNames := make([]string, 128)
	for index := range headerNames {
		headerNames[index] = fmt.Sprintf("Header-%03d-%s", index, strings.Repeat("x", 100))
	}
	result := smartaccountssync.BrowserDiscoveryRelayResult{
		Source: "smartaccounts-browser-relay", Type: "smartaccounts-browser-relay.discovery-result.v1", Version: 1,
		DiscoveryID: issue.DiscoveryID, ManifestVersion: issue.ManifestVersion,
		ContractVersion: smartaccountssync.BrowserDiscoveryContractVersion, Status: "completed",
		Resources: handlerDiscoveryResourcesWithHeaders(issue.ResourceIDs, headerNames),
	}
	validBody, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Greater(t, len(validBody), 64<<10, "a valid full discovery result can exceed the former 64 KiB OA limit")
	assert.Less(t, len(validBody), smartaccountssync.BrowserDiscoveryMaxReceiptBytes)

	accepted := httptest.NewRecorder()
	handlers.ReceiveSmartAccountsBrowserDiscoveryReceipt(accepted, ownerDiscoveryRequest(http.MethodPost, string(validBody), issue.DiscoveryID))
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	assert.Equal(t, 1, bridge.recordCalls)

	tooLarge := `{"source":"` + strings.Repeat("x", smartaccountssync.BrowserDiscoveryMaxReceiptBytes) + `"}`
	rejected := httptest.NewRecorder()
	handlers.ReceiveSmartAccountsBrowserDiscoveryReceipt(rejected, ownerDiscoveryRequest(http.MethodPost, tooLarge, issue.DiscoveryID))
	assert.Equal(t, http.StatusBadRequest, rejected.Code, rejected.Body.String())
	assert.Equal(t, 1, bridge.recordCalls, "oversize input must fail before bridge relay")
}

func handlerDiscoveryResources(resourceIDs []string) []smartaccountssync.BrowserDiscoveryResource {
	resources := make([]smartaccountssync.BrowserDiscoveryResource, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		resources = append(resources, smartaccountssync.BrowserDiscoveryResource{
			ResourceID: resourceID, CaptureStatus: "filter_contract_required",
			Binding: smartaccountssync.BrowserDiscoveryBinding{Session: "verified", Company: "verified", Page: "verified"},
			Contract: smartaccountssync.BrowserDiscoveryResourceContract{
				Version: smartaccountssync.BrowserDiscoveryContractVersion, PagePath: "/discovery/" + resourceID,
				Pagination: smartaccountssync.BrowserDiscoveryPagination{Kind: "unobserved", ControlIDs: []string{}},
				Response:   smartaccountssync.BrowserDiscoveryResponseContract{Observation: "unobserved", ContentType: "unobserved", HeaderNames: []string{}},
			},
		})
	}
	return resources
}

func handlerDiscoveryResourcesWithHeaders(resourceIDs, headerNames []string) []smartaccountssync.BrowserDiscoveryResource {
	resources := handlerDiscoveryResources(resourceIDs)
	for index := range resources {
		resources[index].Contract.Response = smartaccountssync.BrowserDiscoveryResponseContract{
			Observation: "range_header", ContentType: "text/csv", HeaderNames: headerNames,
		}
	}
	return resources
}
