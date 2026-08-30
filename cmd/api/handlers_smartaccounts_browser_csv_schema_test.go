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

const testBrowserCSVSchemaID = smartaccountssync.BrowserGeneralLedgerCSVSchemaID

type browserCSVSchemaHandlerStore struct {
	discoveryHandlerStore
	approvals map[string]smartaccountssync.BrowserCSVSchemaApproval
}

func (s *browserCSVSchemaHandlerStore) FindOrCreateBrowserCSVSchemaApproval(_ context.Context, approval smartaccountssync.BrowserCSVSchemaApproval) (*smartaccountssync.BrowserCSVSchemaApproval, bool, error) {
	if s.approvals == nil {
		s.approvals = map[string]smartaccountssync.BrowserCSVSchemaApproval{}
	}
	key := browserCSVSchemaHandlerKey(approval.TenantID, approval.DiscoveryID, approval.ResourceID)
	if existing, found := s.approvals[key]; found {
		return &existing, false, nil
	}
	s.approvals[key] = approval
	return &approval, true, nil
}

func (s *browserCSVSchemaHandlerStore) GetBrowserCSVSchemaApproval(_ context.Context, tenantID, discoveryID, resourceID string) (*smartaccountssync.BrowserCSVSchemaApproval, error) {
	approval, found := s.approvals[browserCSVSchemaHandlerKey(tenantID, discoveryID, resourceID)]
	if !found {
		return nil, smartaccountssync.ErrBrowserCSVSchemaApprovalNotFound
	}
	return &approval, nil
}

func (s *browserCSVSchemaHandlerStore) MarkBrowserCSVSchemaApprovalRegistered(_ context.Context, approval smartaccountssync.BrowserCSVSchemaApproval, observedAt time.Time) (*smartaccountssync.BrowserCSVSchemaApproval, error) {
	key := browserCSVSchemaHandlerKey(approval.TenantID, approval.DiscoveryID, approval.ResourceID)
	stored, found := s.approvals[key]
	if !found || stored.SchemaID != approval.SchemaID || stored.Review.AuditID != approval.Review.AuditID {
		return nil, smartaccountssync.ErrBrowserCSVSchemaApprovalConflict
	}
	approval.UpdatedAt = observedAt.UTC()
	s.approvals[key] = approval
	return &approval, nil
}

func browserCSVSchemaHandlerKey(tenantID, discoveryID, resourceID string) string {
	return tenantID + "/" + discoveryID + "/" + resourceID
}

type browserCSVSchemaHandlerBridge struct {
	registerCalls int
	getCalls      int
	tenantID      string
	sourceID      string
	resourceID    string
	schemaID      string
	review        smartaccountssync.BrowserCSVSchemaApprovalBridgeRequest
	response      smartaccountssync.BrowserCSVSchemaApprovalResponse
	err           error
}

func (b *browserCSVSchemaHandlerBridge) RegisterBrowserCSVSchemaApproval(_ context.Context, tenantID, sourceID, resourceID, schemaID string, review smartaccountssync.BrowserCSVSchemaApprovalBridgeRequest) (smartaccountssync.BrowserCSVSchemaApprovalResponse, error) {
	b.registerCalls++
	b.tenantID, b.sourceID, b.resourceID, b.schemaID, b.review = tenantID, sourceID, resourceID, schemaID, review
	if b.err != nil {
		return smartaccountssync.BrowserCSVSchemaApprovalResponse{}, b.err
	}
	return b.response, nil
}

func (b *browserCSVSchemaHandlerBridge) GetBrowserCSVSchemaApproval(_ context.Context, tenantID, sourceID, resourceID, schemaID string) (smartaccountssync.BrowserCSVSchemaApprovalResponse, error) {
	b.getCalls++
	b.tenantID, b.sourceID, b.resourceID, b.schemaID = tenantID, sourceID, resourceID, schemaID
	if b.err != nil {
		return smartaccountssync.BrowserCSVSchemaApprovalResponse{}, b.err
	}
	return b.response, nil
}

func newSmartAccountsBrowserCSVSchemaHandlers() (*Handlers, *browserCSVSchemaHandlerStore, *browserCSVSchemaHandlerBridge, string) {
	store := &browserCSVSchemaHandlerStore{discoveryHandlerStore: discoveryHandlerStore{
		controls: map[string]smartaccountssync.Control{
			smartAccountsControlKey(testBrowserPairingTenantID, testBrowserSourceID): {
				TenantID: testBrowserPairingTenantID, SourceCompanyID: testBrowserSourceID,
				SecretReference: "brave-session://0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
			},
		},
	}}
	discoveryService := smartaccountssync.NewBrowserDiscoveryService(&store.discoveryHandlerStore, &store.discoveryHandlerStore, &discoveryHandlerBridge{})
	issue, err := discoveryService.Issue(context.Background(), testBrowserPairingTenantID, "owner-1", smartaccountssync.BrowserDiscoveryStartRequest{SourceCompanyID: testBrowserSourceID, MetadataOnlyConsentConfirmed: true})
	if err != nil {
		panic(err)
	}
	bridge := &browserCSVSchemaHandlerBridge{response: smartaccountssync.BrowserCSVSchemaApprovalResponse{
		ResourceID: smartaccountssync.BrowserGeneralLedgerResourceID, SchemaID: testBrowserCSVSchemaID, Status: "registered", ApprovalSHA256: strings.Repeat("a", 64),
	}}
	service := smartaccountssync.NewBrowserCSVSchemaApprovalService(store, store, bridge)
	return &Handlers{smartAccountsBrowserCSVSchemaApprovalService: service}, store, bridge, issue.DiscoveryID
}

func ownerBrowserCSVSchemaRequest(method, body, tenantID, discoveryID, resourceID, schemaID string) *http.Request {
	request := httptest.NewRequest(method, "/", strings.NewReader(body))
	request = withClaims(request, createTestClaims("owner-1", "owner@example.com", tenantID, "owner"))
	request = withURLParams(request, map[string]string{
		"tenantID": tenantID, "discoveryID": discoveryID, "resourceID": resourceID, "schemaID": schemaID,
	})
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestSmartAccountsBrowserCSVSchemaReviewIsOwnerBoundAggregateOnlyAndReplaySafe(t *testing.T) {
	handlers, store, bridge, discoveryID := newSmartAccountsBrowserCSVSchemaHandlers()
	request := ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID)
	first := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(first, request)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	require.Equal(t, "no-store", first.Header().Get("Cache-Control"))
	assert.Contains(t, first.Body.String(), `"resource_id":"general_ledger"`)
	assert.Contains(t, first.Body.String(), `"approval_sha256":"`+strings.Repeat("a", 64)+`"`)
	for _, forbidden := range []string{testBrowserSourceID, "header", "cookie", "credential", "token", "audit_id", "reviewed_by", "source_company_id"} {
		assert.NotContains(t, first.Body.String(), forbidden)
	}
	assert.Equal(t, 1, bridge.registerCalls)
	assert.Equal(t, testBrowserPairingTenantID, bridge.tenantID)
	assert.Equal(t, testBrowserSourceID, bridge.sourceID, "source binding is server-side only")
	assert.Equal(t, discoveryID, bridge.review.DiscoveryID)
	assert.True(t, bridge.review.Review.Confirmed)
	assert.NotEmpty(t, bridge.review.Review.AuditID)
	assert.Len(t, store.approvals, 1)

	replay := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(replay, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID))
	require.Equal(t, http.StatusOK, replay.Code, replay.Body.String())
	assert.Equal(t, 2, bridge.registerCalls)

	status := httptest.NewRecorder()
	handlers.GetSmartAccountsBrowserCSVSchemaReview(status, ownerBrowserCSVSchemaRequest(http.MethodGet, "", testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID))
	require.Equal(t, http.StatusOK, status.Code, status.Body.String())
	assert.Equal(t, "no-store", status.Header().Get("Cache-Control"))
	assert.Equal(t, 1, bridge.getCalls)
	assert.NotContains(t, status.Body.String(), testBrowserSourceID)
}

func TestSmartAccountsBrowserCSVSchemaReviewRejectsCrossTenantConflictingAndDataBearingRequests(t *testing.T) {
	handlers, _, bridge, discoveryID := newSmartAccountsBrowserCSVSchemaHandlers()

	crossTenant := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(crossTenant, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, "other-tenant", discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID))
	assert.Equal(t, http.StatusNotFound, crossTenant.Code, crossTenant.Body.String())
	assert.Zero(t, bridge.registerCalls)

	dataBearing := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(dataBearing, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true,"header_names":["must-not-persist"]}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID))
	assert.Equal(t, http.StatusBadRequest, dataBearing.Code, dataBearing.Body.String())
	assert.NotContains(t, dataBearing.Body.String(), "must-not-persist")
	assert.Zero(t, bridge.registerCalls)

	invalidResource := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(invalidResource, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, "annual_report", testBrowserCSVSchemaID))
	assert.Equal(t, http.StatusBadRequest, invalidResource.Code, invalidResource.Body.String())
	assert.Zero(t, bridge.registerCalls)

	summaryGrid := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(summaryGrid, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserJournalEntriesSummaryResourceID, testBrowserCSVSchemaID))
	assert.Equal(t, http.StatusBadRequest, summaryGrid.Code, summaryGrid.Body.String())
	assert.Zero(t, bridge.registerCalls, "journal_entries is summary evidence, never a reviewed authoritative GL adapter")

	accepted := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(accepted, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID))
	require.Equal(t, http.StatusCreated, accepted.Code, accepted.Body.String())
	conflict := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(conflict, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, "another_csv_v1"))
	assert.Equal(t, http.StatusBadRequest, conflict.Code, conflict.Body.String())
	assert.Equal(t, 1, bridge.registerCalls)
}

func TestSmartAccountsBrowserCSVSchemaReviewRequiresOwnerAndRedactsBridgeFailure(t *testing.T) {
	handlers, _, bridge, discoveryID := newSmartAccountsBrowserCSVSchemaHandlers()
	nonOwner := ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID)
	nonOwner = withClaims(nonOwner, createTestClaims("admin-1", "admin@example.com", testBrowserPairingTenantID, "admin"))
	forbidden := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(forbidden, nonOwner)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)
	assert.Zero(t, bridge.registerCalls)

	bridge.err = errors.New("private details source=" + testBrowserSourceID + " cookie=must-not-leak")
	failure := httptest.NewRecorder()
	handlers.ReviewSmartAccountsBrowserCSVSchema(failure, ownerBrowserCSVSchemaRequest(http.MethodPost, `{"review_confirmed":true}`, testBrowserPairingTenantID, discoveryID, smartaccountssync.BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID))
	assert.Equal(t, http.StatusBadGateway, failure.Code, failure.Body.String())
	assert.NotContains(t, failure.Body.String(), testBrowserSourceID)
	assert.NotContains(t, failure.Body.String(), "must-not-leak")
}
