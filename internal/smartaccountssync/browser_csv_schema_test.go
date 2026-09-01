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

const testBrowserCSVSchemaID = BrowserGeneralLedgerCSVSchemaID

type memoryBrowserCSVSchemaStore struct {
	authorizations map[string]BrowserDiscoveryAuthorization
	approvals      map[string]BrowserCSVSchemaApproval
}

func (s *memoryBrowserCSVSchemaStore) GetBrowserDiscoveryAuthorization(_ context.Context, tenantID, discoveryID string) (*BrowserDiscoveryAuthorization, error) {
	authorization, found := s.authorizations[tenantID+"/"+discoveryID]
	if !found {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	return &authorization, nil
}

func (s *memoryBrowserCSVSchemaStore) FindOrCreateBrowserCSVSchemaApproval(_ context.Context, approval BrowserCSVSchemaApproval) (*BrowserCSVSchemaApproval, bool, error) {
	if s.approvals == nil {
		s.approvals = map[string]BrowserCSVSchemaApproval{}
	}
	key := approval.TenantID + "/" + approval.DiscoveryID + "/" + approval.ResourceID
	if existing, found := s.approvals[key]; found {
		return &existing, false, nil
	}
	s.approvals[key] = approval
	return &approval, true, nil
}

func (s *memoryBrowserCSVSchemaStore) GetBrowserCSVSchemaApproval(_ context.Context, tenantID, discoveryID, resourceID string) (*BrowserCSVSchemaApproval, error) {
	approval, found := s.approvals[tenantID+"/"+discoveryID+"/"+resourceID]
	if !found {
		return nil, ErrBrowserCSVSchemaApprovalNotFound
	}
	return &approval, nil
}

func (s *memoryBrowserCSVSchemaStore) MarkBrowserCSVSchemaApprovalRegistered(_ context.Context, approval BrowserCSVSchemaApproval, observedAt time.Time) (*BrowserCSVSchemaApproval, error) {
	key := approval.TenantID + "/" + approval.DiscoveryID + "/" + approval.ResourceID
	stored, found := s.approvals[key]
	if !found || stored.SchemaID != approval.SchemaID || stored.Review.AuditID != approval.Review.AuditID {
		return nil, ErrBrowserCSVSchemaApprovalConflict
	}
	approval.UpdatedAt = observedAt.UTC()
	s.approvals[key] = approval
	return &approval, nil
}

type memoryBrowserCSVSchemaBridge struct {
	registerCalls int
	getCalls      int
	tenantID      string
	sourceID      string
	resourceID    string
	schemaID      string
	request       BrowserCSVSchemaApprovalBridgeRequest
	response      BrowserCSVSchemaApprovalResponse
	err           error
}

func (b *memoryBrowserCSVSchemaBridge) RegisterBrowserCSVSchemaApproval(_ context.Context, tenantID, sourceID, resourceID, schemaID string, request BrowserCSVSchemaApprovalBridgeRequest) (BrowserCSVSchemaApprovalResponse, error) {
	b.registerCalls++
	b.tenantID, b.sourceID, b.resourceID, b.schemaID, b.request = tenantID, sourceID, resourceID, schemaID, request
	if b.err != nil {
		return BrowserCSVSchemaApprovalResponse{}, b.err
	}
	return b.response, nil
}

func (b *memoryBrowserCSVSchemaBridge) GetBrowserCSVSchemaApproval(_ context.Context, tenantID, sourceID, resourceID, schemaID string) (BrowserCSVSchemaApprovalResponse, error) {
	b.getCalls++
	b.tenantID, b.sourceID, b.resourceID, b.schemaID = tenantID, sourceID, resourceID, schemaID
	if b.err != nil {
		return BrowserCSVSchemaApprovalResponse{}, b.err
	}
	return b.response, nil
}

func newBrowserCSVSchemaApprovalService(now time.Time) (*BrowserCSVSchemaApprovalService, *memoryBrowserCSVSchemaStore, *memoryBrowserCSVSchemaBridge) {
	store := &memoryBrowserCSVSchemaStore{authorizations: map[string]BrowserDiscoveryAuthorization{
		testBrowserDiscoveryTenantID + "/" + testBrowserDiscoveryID: {
			TenantID: testBrowserDiscoveryTenantID, DiscoveryID: testBrowserDiscoveryID, SourceCompanyID: testBrowserDiscoverySourceID,
			ManifestVersion: BrowserDiscoveryManifestVersion, ContractVersion: BrowserDiscoveryContractVersion, ResourceIDs: browserDiscoveryResourceIDs(),
			MetadataOnlyConsentConfirmed: true, ConsentedAt: now, ExpiresAt: now.Add(time.Hour), CreatedAt: now,
		},
	}}
	controls := &memoryBrowserDiscoveryControls{controls: map[string]Control{
		testBrowserDiscoveryTenantID + "/" + testBrowserDiscoverySourceID: {
			TenantID: testBrowserDiscoveryTenantID, SourceCompanyID: testBrowserDiscoverySourceID,
			SecretReference: browserSessionReference("0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3"),
		},
	}}
	bridge := &memoryBrowserCSVSchemaBridge{response: browserCSVSchemaApprovalResponse()}
	service := NewBrowserCSVSchemaApprovalService(store, controls, bridge)
	service.now = func() time.Time { return now }
	service.newID = func() string { return "c1a222aa-11aa-4e4e-8ee8-4a08de7310d3" }
	return service, store, bridge
}

func browserCSVSchemaApprovalResponse() BrowserCSVSchemaApprovalResponse {
	return BrowserCSVSchemaApprovalResponse{
		ResourceID: BrowserGeneralLedgerResourceID, SchemaID: testBrowserCSVSchemaID, Status: "registered",
		ApprovalSHA256: strings.Repeat("a", 64),
	}
}

func TestBrowserCSVSchemaApprovalBindsReviewToDiscoveryAndReplaysSafely(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, store, bridge := newBrowserCSVSchemaApprovalService(now)

	response, created, err := service.Review(context.Background(), testBrowserDiscoveryTenantID, "owner-1", testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, browserCSVSchemaApprovalResponse(), response)
	assert.Equal(t, 1, bridge.registerCalls)
	assert.Equal(t, testBrowserDiscoveryTenantID, bridge.tenantID)
	assert.Equal(t, testBrowserDiscoverySourceID, bridge.sourceID, "source binding is resolved only after tenant/discovery authorization")
	assert.Equal(t, testBrowserDiscoveryID, bridge.request.DiscoveryID)
	assert.Equal(t, testBrowserCSVSchemaID, bridge.request.SchemaID)
	assert.Equal(t, BrowserCSVSchemaReviewVersion, bridge.request.Review.Version)
	assert.True(t, bridge.request.Review.Confirmed)
	assert.NotEmpty(t, bridge.request.Review.AuditID)
	assert.Equal(t, now, bridge.request.Review.ReviewedAt)

	stored := store.approvals[testBrowserDiscoveryTenantID+"/"+testBrowserDiscoveryID+"/"+BrowserGeneralLedgerResourceID]
	assert.Equal(t, BrowserCSVSchemaStatusRegistered, stored.Status)
	assert.Equal(t, strings.Repeat("a", 64), stored.ApprovalSHA256)
	assert.Equal(t, "owner-1", stored.ReviewedBy)

	_, created, err = service.Review(context.Background(), testBrowserDiscoveryTenantID, "owner-2", testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, 2, bridge.registerCalls, "retry uses the persisted immutable review assertion")
	assert.Equal(t, stored.Review.AuditID, bridge.request.Review.AuditID)
}

func TestBrowserCSVSchemaApprovalFailsClosedForCrossTenantUnapprovedResourceAndDigestConflict(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, _, bridge := newBrowserCSVSchemaApprovalService(now)

	_, _, err := service.Review(context.Background(), "other-tenant", "owner-1", testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	require.ErrorIs(t, err, ErrBrowserCSVSchemaApprovalUnauthorized)
	assert.Zero(t, bridge.registerCalls)

	_, _, err = service.Review(context.Background(), testBrowserDiscoveryTenantID, "owner-1", testBrowserDiscoveryID, BrowserJournalEntriesSummaryResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	require.ErrorIs(t, err, ErrBrowserCSVSchemaApprovalInvalid)
	assert.Zero(t, bridge.registerCalls, "the journal summary cannot receive the authoritative CSV adapter")

	_, _, err = service.Review(context.Background(), testBrowserDiscoveryTenantID, "owner-1", testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{})
	require.ErrorIs(t, err, ErrBrowserCSVSchemaApprovalInvalid)
	assert.Zero(t, bridge.registerCalls)

	_, _, err = service.Review(context.Background(), testBrowserDiscoveryTenantID, "owner-1", testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	require.NoError(t, err)
	bridge.response.ApprovalSHA256 = strings.Repeat("b", 64)
	_, err = service.Status(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID)
	require.ErrorIs(t, err, ErrBrowserCSVSchemaApprovalConflict)
	assert.Equal(t, 1, bridge.getCalls)
}

func TestBrowserCSVSchemaApprovalKeepsBridgeErrorsAndNoSourceInPublicResponse(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, _, bridge := newBrowserCSVSchemaApprovalService(now)
	bridge.err = errors.New("private failure carrying source/header/cookie")

	_, _, err := service.Review(context.Background(), testBrowserDiscoveryTenantID, "owner-1", testBrowserDiscoveryID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, BrowserCSVSchemaApprovalRequest{ReviewConfirmed: true})
	require.Error(t, err)
	assert.Equal(t, 1, bridge.registerCalls)

	encoded := mustJSON(t, browserCSVSchemaApprovalResponse())
	assert.NotContains(t, encoded, testBrowserDiscoverySourceID)
	assert.NotContains(t, encoded, "header")
	assert.NotContains(t, encoded, "cookie")
	assert.NotContains(t, encoded, "audit_id")
}
