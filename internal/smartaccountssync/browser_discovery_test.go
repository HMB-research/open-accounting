package smartaccountssync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBrowserDiscoveryTenantID = "tenant-discovery"
	testBrowserDiscoverySourceID = "sa-browser-v1-424242"
	testBrowserDiscoveryID       = "417f6fec-1994-4cfe-8ea6-bb7281d3050f"
)

type memoryBrowserDiscoveryStore struct {
	authorizations map[string]BrowserDiscoveryAuthorization
	receipts       map[string]BrowserDiscoveryReceipt
}

func (s *memoryBrowserDiscoveryStore) CreateBrowserDiscoveryAuthorization(_ context.Context, authorization BrowserDiscoveryAuthorization) error {
	if s.authorizations == nil {
		s.authorizations = map[string]BrowserDiscoveryAuthorization{}
	}
	key := authorization.TenantID + "/" + authorization.DiscoveryID
	if _, found := s.authorizations[key]; found {
		return errors.New("duplicate discovery authorization")
	}
	s.authorizations[key] = authorization
	return nil
}

func (s *memoryBrowserDiscoveryStore) GetBrowserDiscoveryAuthorization(_ context.Context, tenantID, discoveryID string) (*BrowserDiscoveryAuthorization, error) {
	authorization, found := s.authorizations[tenantID+"/"+discoveryID]
	if !found {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	return &authorization, nil
}

func (s *memoryBrowserDiscoveryStore) SaveBrowserDiscoveryReceipt(_ context.Context, tenantID, discoveryID string, receipt BrowserDiscoveryReceipt, _ time.Time) error {
	if _, found := s.authorizations[tenantID+"/"+discoveryID]; !found {
		return ErrBrowserDiscoveryUnauthorized
	}
	if s.receipts == nil {
		s.receipts = map[string]BrowserDiscoveryReceipt{}
	}
	s.receipts[tenantID+"/"+discoveryID] = receipt
	return nil
}

type memoryBrowserDiscoveryControls struct {
	controls map[string]Control
}

func (s *memoryBrowserDiscoveryControls) Get(_ context.Context, tenantID, sourceCompanyID string) (*Control, error) {
	control, found := s.controls[tenantID+"/"+sourceCompanyID]
	if !found {
		return nil, ErrControlNotConfigured
	}
	return &control, nil
}

type memoryBrowserDiscoveryBridge struct {
	recordCalls int
	getCalls    int
	tenantID    string
	sourceID    string
	discoveryID string
	request     BrowserDiscoveryBridgeReceiptRequest
	receipt     BrowserDiscoveryReceipt
	err         error
}

func (b *memoryBrowserDiscoveryBridge) RecordBrowserDiscoveryReceipt(_ context.Context, tenantID, sourceCompanyID, discoveryID string, request BrowserDiscoveryBridgeReceiptRequest) (BrowserDiscoveryReceipt, error) {
	b.recordCalls++
	b.tenantID, b.sourceID, b.discoveryID, b.request = tenantID, sourceCompanyID, discoveryID, request
	if b.err != nil {
		return BrowserDiscoveryReceipt{}, b.err
	}
	return b.receipt, nil
}

func (b *memoryBrowserDiscoveryBridge) GetBrowserDiscoveryReceipt(_ context.Context, tenantID, sourceCompanyID, discoveryID string) (BrowserDiscoveryReceipt, error) {
	b.getCalls++
	b.tenantID, b.sourceID, b.discoveryID = tenantID, sourceCompanyID, discoveryID
	if b.err != nil {
		return BrowserDiscoveryReceipt{}, b.err
	}
	return b.receipt, nil
}

func newBrowserDiscoveryService(now time.Time) (*BrowserDiscoveryService, *memoryBrowserDiscoveryStore, *memoryBrowserDiscoveryBridge) {
	store := &memoryBrowserDiscoveryStore{}
	controls := &memoryBrowserDiscoveryControls{controls: map[string]Control{
		testBrowserDiscoveryTenantID + "/" + testBrowserDiscoverySourceID: {
			TenantID: testBrowserDiscoveryTenantID, SourceCompanyID: testBrowserDiscoverySourceID,
			SecretReference: browserSessionReference("0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3"),
		},
	}}
	bridge := &memoryBrowserDiscoveryBridge{receipt: browserDiscoveryReceipt(testBrowserDiscoveryID)}
	service := NewBrowserDiscoveryService(store, controls, bridge)
	service.now = func() time.Time { return now }
	service.newID = func() string { return testBrowserDiscoveryID }
	return service, store, bridge
}

func browserDiscoveryReceipt(discoveryID string) BrowserDiscoveryReceipt {
	return BrowserDiscoveryReceipt{
		DiscoveryID: discoveryID, Status: "completed", ManifestVersion: BrowserDiscoveryManifestVersion,
		ContractVersion: BrowserDiscoveryContractVersion, ContractSHA256: strings.Repeat("a", 64),
		ResourceCount: 31, CaptureReadyCount: 1, FilterRequiredCount: 23, PageOnlyRequiredCount: 7,
	}
}

func browserDiscoveryRelayResult(discoveryID string) BrowserDiscoveryRelayResult {
	resources := make([]BrowserDiscoveryResource, 0, len(browserDiscoveryResourceIDs()))
	for _, resourceID := range browserDiscoveryResourceIDs() {
		resources = append(resources, BrowserDiscoveryResource{
			ResourceID: resourceID, CaptureStatus: "filter_contract_required",
			Binding: BrowserDiscoveryBinding{Session: "verified", Company: "verified", Page: "verified"},
			Contract: BrowserDiscoveryResourceContract{
				Version:    BrowserDiscoveryContractVersion,
				PagePath:   "/discovery/" + resourceID,
				Pagination: BrowserDiscoveryPagination{Kind: "unobserved", ControlIDs: []string{}},
				Response:   BrowserDiscoveryResponseContract{Observation: "unobserved", ContentType: "unobserved", HeaderNames: []string{}},
			},
		})
	}
	return BrowserDiscoveryRelayResult{
		Source: browserDiscoveryRelaySource, Type: browserDiscoveryRelayEvent, Version: 1,
		DiscoveryID: discoveryID, ManifestVersion: BrowserDiscoveryManifestVersion,
		ContractVersion: BrowserDiscoveryContractVersion, Status: "completed",
		Resources: resources,
	}
}

func browserDiscoveryRelayResultWithHeader(discoveryID string) BrowserDiscoveryRelayResult {
	result := browserDiscoveryRelayResult(discoveryID)
	for index := range result.Resources {
		if result.Resources[index].ResourceID != "journal_entries" {
			continue
		}
		result.Resources[index].CaptureStatus = "capture_ready"
		result.Resources[index].Contract = BrowserDiscoveryResourceContract{
			Version: BrowserDiscoveryContractVersion, PagePath: "/et/entries",
			Request:    &BrowserDiscoveryRequestContract{Method: "GET", Path: "/et/entries.exportgriddata:exportgrid/csv"},
			Filter:     &BrowserDiscoveryFilterContract{Method: "POST", Path: "/et/entries.filterformcomp.form", ControlIDs: []string{"filterCreatedBy", "filterResults", "fromDateComp", "toDateComp"}},
			Pagination: BrowserDiscoveryPagination{Kind: "unobserved", ControlIDs: []string{}},
			Response:   BrowserDiscoveryResponseContract{Observation: "range_header", ContentType: "text/csv", HeaderNames: []string{"Entry number", "Posting date"}},
		}
		break
	}
	return result
}

func TestBrowserDiscoveryIssueRecordsOnlyActionTimeConsentAndBinding(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, store, _ := newBrowserDiscoveryService(now)

	_, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{SourceCompanyID: testBrowserDiscoverySourceID})
	require.ErrorIs(t, err, ErrBrowserDiscoveryConsent)

	issue, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{
		SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true,
	})
	require.NoError(t, err)
	assert.Equal(t, testBrowserDiscoveryID, issue.DiscoveryID)
	assert.Equal(t, testBrowserDiscoveryTenantID, issue.TenantID)
	assert.Equal(t, testBrowserDiscoverySourceID, issue.SourceCompanyID)
	assert.Equal(t, browserDiscoveryResourceIDs(), issue.ResourceIDs)
	assert.Equal(t, BrowserDiscoveryConsent{Version: 1, Confirmed: true, ConfirmedAt: now, Scope: "metadata_only"}, issue.DiscoveryConsent)
	assert.Equal(t, now.Add(browserDiscoveryLifetime), issue.ExpiresAt)
	stored := store.authorizations[testBrowserDiscoveryTenantID+"/"+testBrowserDiscoveryID]
	assert.True(t, stored.MetadataOnlyConsentConfirmed)
	assert.False(t, stored.HeaderProbeConsentConfirmed)
	assert.Equal(t, browserDiscoveryResourceIDs(), stored.ResourceIDs)
	assert.Len(t, issue.ResourceIDs, 31)
	for _, blocker := range []string{"annual_report", "other_reports", "warehouse_inventory", "warehouse_movements_report", "warehouses", "worker_absences", "wage_reports"} {
		assert.Contains(t, issue.ResourceIDs, blocker)
	}
	assert.NotEqual(t, []string{"journal_entries"}, issue.ResourceIDs, "journal discovery alone cannot claim full sync coverage")

	service.newID = func() string { return "40f16858-83ed-4934-8ac1-d642652b7f53" }
	headerIssue, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{
		SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "metadata_and_header_probe", headerIssue.DiscoveryConsent.Scope)
	assert.True(t, headerIssue.DiscoveryConsent.ResponseHeaderProbeConfirmed)
}

func TestBrowserDiscoveryReceiveIsTenantBoundRedactedAndHeaderConsentGated(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, store, bridge := newBrowserDiscoveryService(now)
	_, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{
		SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true,
	})
	require.NoError(t, err)

	receipt, err := service.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, browserDiscoveryRelayResultWithHeader(testBrowserDiscoveryID))
	require.NoError(t, err)
	assert.Equal(t, browserDiscoveryReceipt(testBrowserDiscoveryID), receipt)
	assert.Equal(t, 1, bridge.recordCalls)
	assert.Equal(t, testBrowserDiscoveryTenantID, bridge.tenantID)
	assert.Equal(t, testBrowserDiscoverySourceID, bridge.sourceID)
	assert.Equal(t, testBrowserDiscoverySourceID, bridge.request.SourceCompanyID)
	assert.Len(t, bridge.request.Resources, 31)
	assert.Equal(t, browserDiscoveryResourceIDs(), resourceIDsFromBrowserDiscovery(bridge.request.Resources))
	assert.Equal(t, receipt, store.receipts[testBrowserDiscoveryTenantID+"/"+testBrowserDiscoveryID])

	_, err = service.Receive(context.Background(), "other-tenant", testBrowserDiscoveryID, browserDiscoveryRelayResultWithHeader(testBrowserDiscoveryID))
	require.ErrorIs(t, err, ErrBrowserDiscoveryUnauthorized)
	assert.Equal(t, 1, bridge.recordCalls, "wrong tenant must not reach bridge")

	withoutHeaderConsent, _, noHeaderBridge := newBrowserDiscoveryService(now)
	_, err = withoutHeaderConsent.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true})
	require.NoError(t, err)
	_, err = withoutHeaderConsent.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, browserDiscoveryRelayResultWithHeader(testBrowserDiscoveryID))
	require.ErrorIs(t, err, ErrBrowserDiscoveryInvalid)
	assert.Zero(t, noHeaderBridge.recordCalls, "range headers require separately recorded consent")
}

func TestBrowserDiscoveryRejectsExpiredOrMalformedRelayAndAllowsSafePostExpiryStatus(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, _, bridge := newBrowserDiscoveryService(now)
	_, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true})
	require.NoError(t, err)

	malformed := browserDiscoveryRelayResult(testBrowserDiscoveryID)
	malformed.Type = "unexpected"
	_, err = service.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, malformed)
	require.ErrorIs(t, err, ErrBrowserDiscoveryInvalid)
	assert.Zero(t, bridge.recordCalls)

	service.now = func() time.Time { return now.Add(browserDiscoveryLifetime) }
	_, err = service.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, browserDiscoveryRelayResult(testBrowserDiscoveryID))
	require.ErrorIs(t, err, ErrBrowserDiscoveryUnauthorized)
	assert.Zero(t, bridge.recordCalls)

	receipt, err := service.Status(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID)
	require.NoError(t, err)
	assert.Equal(t, browserDiscoveryReceipt(testBrowserDiscoveryID), receipt)
	assert.Equal(t, 1, bridge.getCalls)
}

func TestBrowserDiscoveryPreservesBridgeConflictForImmutableReplay(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, _, bridge := newBrowserDiscoveryService(now)
	_, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true, ResponseHeaderProbeConfirmed: true})
	require.NoError(t, err)
	bridge.err = ErrBrowserDiscoveryConflict
	_, err = service.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, browserDiscoveryRelayResult(testBrowserDiscoveryID))
	require.ErrorIs(t, err, ErrBrowserDiscoveryConflict)
}

func TestBrowserDiscoveryPersistsAnExpiredPartialReceiptButRejectsJournalOnlyCompleted(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	service, store, bridge := newBrowserDiscoveryService(now)
	_, err := service.Issue(context.Background(), testBrowserDiscoveryTenantID, "owner-1", BrowserDiscoveryStartRequest{SourceCompanyID: testBrowserDiscoverySourceID, MetadataOnlyConsentConfirmed: true})
	require.NoError(t, err)

	journalOnly := browserDiscoveryRelayResult(testBrowserDiscoveryID)
	journalOnly.Resources = journalOnly.Resources[:1]
	_, err = service.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, journalOnly)
	require.ErrorIs(t, err, ErrBrowserDiscoveryInvalid)
	assert.Zero(t, bridge.recordCalls, "journal-only metadata is never complete discovery")

	partial := browserDiscoveryRelayResult(testBrowserDiscoveryID)
	partial.Status = "expired"
	partial.Resources = partial.Resources[:3]
	bridge.receipt = BrowserDiscoveryReceipt{
		DiscoveryID: testBrowserDiscoveryID, Status: "expired", ManifestVersion: BrowserDiscoveryManifestVersion,
		ContractVersion: BrowserDiscoveryContractVersion, ContractSHA256: strings.Repeat("b", 64),
		ResourceCount: 3, FilterRequiredCount: 3,
	}
	receipt, err := service.Receive(context.Background(), testBrowserDiscoveryTenantID, testBrowserDiscoveryID, partial)
	require.NoError(t, err)
	assert.Equal(t, "expired", receipt.Status)
	assert.Equal(t, 3, receipt.ResourceCount)
	assert.Equal(t, receipt, store.receipts[testBrowserDiscoveryTenantID+"/"+testBrowserDiscoveryID])
}

func TestBrowserDiscoveryContractValidatorMatchesBridgeHeaderAndIdentifierBounds(t *testing.T) {
	headerNames := make([]string, 128)
	for index := range headerNames {
		headerNames[index] = fmt.Sprintf("Header-%03d", index)
	}
	assert.True(t, validBrowserDiscoveryHeaderNames(headerNames), "the bridge accepts 128 exact, ordered header names")
	assert.False(t, validBrowserDiscoveryHeaderNames(append(headerNames, "Header-128")), "129 header names exceed the bridge limit")

	assert.True(t, validBrowserDiscoveryHeaderNames([]string{"Name", "name"}), "case variants are distinct metadata labels")
	assert.False(t, validBrowserDiscoveryHeaderNames([]string{"Name", "Name"}), "only exact duplicates are rejected")
	assert.True(t, validBrowserDiscoveryHeaderNames([]string{"Column: detail"}), "colon is allowed in a header label")
	assert.True(t, validBrowserDiscoveryHeaderNames([]string{strings.Repeat("é", 60)}), "120 UTF-8 bytes is accepted")
	assert.False(t, validBrowserDiscoveryHeaderNames([]string{strings.Repeat("é", 61)}), "more than 120 UTF-8 bytes is rejected")
	assert.False(t, validBrowserDiscoveryHeaderNames([]string{" leading"}), "header labels are validated without trimming")
	assert.False(t, validBrowserDiscoveryHeaderNames([]string{"name\x7f"}), "DEL is rejected")

	ordered := []string{"Zed", "alpha", "Name", "name"}
	assert.Equal(t, ordered, canonicalBrowserDiscoveryHeaderNames(ordered), "header order is contract metadata")

	controlIDs := make([]string, 26)
	for index := range controlIDs {
		controlIDs[index] = fmt.Sprintf("Control%d", index)
	}
	assert.True(t, validBrowserDiscoveryIdentifiers(controlIDs), "pagination/control IDs are bounded by the request size, not an arbitrary count")
	assert.True(t, validBrowserDiscoveryPagination(BrowserDiscoveryPagination{Kind: "visible_control_ids", ControlIDs: controlIDs}))
	assert.True(t, validBrowserDiscoveryIdentifier("A"+strings.Repeat("a", 79)))
	assert.False(t, validBrowserDiscoveryIdentifier("A"+strings.Repeat("a", 80)))
	assert.False(t, validBrowserDiscoveryIdentifier("1control"))
	assert.False(t, validBrowserDiscoveryIdentifier("control:id"))
}
