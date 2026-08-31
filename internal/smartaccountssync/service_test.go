package smartaccountssync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	hmbSourceID    = "sa-company-hmb-9881"
	secondSourceID = "sa-company-other-102"
)

type memoryStore struct {
	controls        map[string]Control
	captureProgress map[string]CaptureProgress
	upserts         int
	dryRuns         int
}

func controlKey(tenantID, sourceCompanyID string) string {
	return tenantID + "\x00" + sourceCompanyID
}

func (s *memoryStore) Get(_ context.Context, tenantID, sourceCompanyID string) (*Control, error) {
	control, ok := s.controls[controlKey(tenantID, sourceCompanyID)]
	if !ok {
		return nil, ErrControlNotConfigured
	}
	return &control, nil
}

func (s *memoryStore) Upsert(_ context.Context, control Control) (*Control, error) {
	if s.controls == nil {
		s.controls = map[string]Control{}
	}
	s.upserts++
	key := controlKey(control.TenantID, control.SourceCompanyID)
	if existing, ok := s.controls[key]; ok {
		control.CreatedAt = existing.CreatedAt
	} else if control.CreatedAt.IsZero() {
		control.CreatedAt = control.UpdatedAt
	}
	s.controls[key] = control
	return &control, nil
}

func (s *memoryStore) MarkDryRunRequested(_ context.Context, tenantID, sourceCompanyID string, requestedAt time.Time) (*Control, error) {
	key := controlKey(tenantID, sourceCompanyID)
	control, ok := s.controls[key]
	if !ok {
		return nil, ErrControlNotConfigured
	}
	s.dryRuns++
	control.DryRunRequestedAt = &requestedAt
	control.UpdatedAt = requestedAt
	s.controls[key] = control
	return &control, nil
}

func (s *memoryStore) RecordCaptureRun(_ context.Context, tenantID, sourceCompanyID, runID string, requestedAt time.Time) (*Control, error) {
	key := controlKey(tenantID, sourceCompanyID)
	control, ok := s.controls[key]
	if !ok {
		return nil, ErrControlNotConfigured
	}
	control.CaptureRunID = runID
	control.DryRunRequestedAt = &requestedAt
	control.UpdatedAt = requestedAt
	s.controls[key] = control
	return &control, nil
}

func (s *memoryStore) UpsertCaptureProgress(_ context.Context, tenantID, sourceCompanyID string, progress CaptureProgress, _ time.Time) error {
	if s.captureProgress == nil {
		s.captureProgress = map[string]CaptureProgress{}
	}
	s.captureProgress[controlKey(tenantID, sourceCompanyID)+"\x00"+progress.RunID] = progress
	return nil
}

func (s *memoryStore) ListCaptureProgresses(_ context.Context, tenantID, sourceCompanyID string) ([]CaptureProgress, error) {
	prefix := controlKey(tenantID, sourceCompanyID) + "\x00"
	result := make([]CaptureProgress, 0)
	for key, progress := range s.captureProgress {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, progress)
		}
	}
	return result, nil
}

func verifiedCatalog(sources ...SourceCandidate) StaticBridgeCatalog {
	return StaticBridgeCatalog{Discovery: SourceDiscovery{
		BridgeAvailable:   true,
		LiveDataContacted: false,
		Sources:           sources,
	}}
}

func holdMyBeerSource() SourceCandidate {
	return SourceCandidate{
		Provider:                   Provider,
		SourceCompanyID:            hmbSourceID,
		SourceCompanyName:          "Hold My Beer OÜ",
		Default:                    true,
		BridgeVerified:             true,
		GeneralLedgerAuthoritative: true,
		InvoicePaymentMode:         InvoicePaymentModeNonPosting,
	}
}

func anotherVerifiedSource() SourceCandidate {
	return SourceCandidate{
		Provider:                   Provider,
		SourceCompanyID:            secondSourceID,
		SourceCompanyName:          "Another Company OÜ",
		BridgeVerified:             true,
		GeneralLedgerAuthoritative: true,
		InvoicePaymentMode:         InvoicePaymentModeNonPosting,
	}
}

func newTestService(store *memoryStore, sources ...SourceCandidate) *Service {
	if len(sources) == 0 {
		sources = []SourceCandidate{holdMyBeerSource()}
	}
	return NewService(store, verifiedCatalog(sources...))
}

func configuredRequest(sourceCompanyID string) ConfigureRequest {
	return ConfigureRequest{
		SourceCompanyID:              sourceCompanyID,
		SecretReference:              "secret-ref://hmb-prod/smartaccounts",
		SmartAccountsGLAuthoritative: true,
		InvoicePaymentMode:           InvoicePaymentModeNonPosting,
	}
}

func TestDiscoverSourcesUsesBridgeVerifiedStableIDWithoutLiveContact(t *testing.T) {
	discovery, err := newTestService(nil).DiscoverSources(context.Background(), "tenant-1")

	require.NoError(t, err)
	assert.True(t, discovery.BridgeAvailable)
	assert.False(t, discovery.LiveDataContacted)
	require.Len(t, discovery.Sources, 1)
	assert.Equal(t, hmbSourceID, discovery.Sources[0].SourceCompanyID)
	assert.Equal(t, "Hold My Beer OÜ", discovery.Sources[0].SourceCompanyName)
	assert.True(t, discovery.Sources[0].Default)
	assert.True(t, discovery.Sources[0].BridgeVerified)
	assert.True(t, discovery.Sources[0].GeneralLedgerAuthoritative)
	assert.Equal(t, InvoicePaymentModeNonPosting, discovery.Sources[0].InvoicePaymentMode)
}

func TestConfigureStoresOnlyOpaqueReferenceAndReturnsSafeStatus(t *testing.T) {
	store := &memoryStore{}
	service := newTestService(store)
	fixedNow := time.Date(2026, 8, 27, 14, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	status, err := service.Configure(context.Background(), "tenant-1", "user-1", configuredRequest(hmbSourceID))

	require.NoError(t, err)
	assert.True(t, status.Configured)
	assert.True(t, status.SecretReferenceConfigured)
	assert.True(t, status.SmartAccountsGLAuthoritative)
	assert.Equal(t, InvoicePaymentModeNonPosting, status.InvoicePaymentMode)
	assert.False(t, status.FinancialApplyEligible)
	assert.False(t, status.FinancialWritesStarted)
	assert.NotContains(t, mustJSON(t, status), "secret-ref://hmb-prod/smartaccounts")
	assert.Equal(t, "secret-ref://hmb-prod/smartaccounts", store.controls[controlKey("tenant-1", hmbSourceID)].SecretReference)
	assert.Equal(t, 1, store.upserts)
}

func TestConfigureRejectsRawKeyUnsafePolicyAndUnknownSource(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ConfigureRequest)
		want   string
	}{
		{name: "raw key", mutate: func(req *ConfigureRequest) { req.SecretReference = "sa_live_raw_key" }, want: "not a raw key"},
		{name: "authority missing", mutate: func(req *ConfigureRequest) { req.SmartAccountsGLAuthoritative = false }, want: "authority"},
		{name: "posting invoice mode", mutate: func(req *ConfigureRequest) { req.InvoicePaymentMode = "POSTING" }, want: "NON_POSTING"},
		{name: "not bridge verified", mutate: func(req *ConfigureRequest) { req.SourceCompanyID = "looked-up-from-display-name" }, want: "not verified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := configuredRequest(hmbSourceID)
			tc.mutate(&req)
			_, err := newTestService(&memoryStore{}).Configure(context.Background(), "tenant-1", "user-1", req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateConnectionRequestRejectsUnsafePolicyBeforeCredentialReferenceReachesBridge(t *testing.T) {
	service := newTestService(&memoryStore{})
	valid := ConnectRequest{
		SourceCredentialReference:    "secret-ref://file/connection-1",
		SmartAccountsGLAuthoritative: true,
		InvoicePaymentMode:           InvoicePaymentModeNonPosting,
	}
	require.NoError(t, service.ValidateConnectionRequest(context.Background(), "tenant-1", valid))

	valid.SmartAccountsGLAuthoritative = false
	assert.ErrorContains(t, service.ValidateConnectionRequest(context.Background(), "tenant-1", valid), "authority")
	valid.SmartAccountsGLAuthoritative = true
	valid.InvoicePaymentMode = "POSTING"
	assert.ErrorContains(t, service.ValidateConnectionRequest(context.Background(), "tenant-1", valid), "NON_POSTING")
	valid.InvoicePaymentMode = InvoicePaymentModeNonPosting
	valid.SourceCredentialReference = "test-api-secret-must-not-be-accepted"
	assert.ErrorContains(t, service.ValidateConnectionRequest(context.Background(), "tenant-1", valid), "credential reference")
}

func TestControlBindingsAreExplicitAndTenantIsolated(t *testing.T) {
	store := &memoryStore{}
	service := newTestService(store, holdMyBeerSource(), anotherVerifiedSource())

	_, err := service.Configure(context.Background(), "tenant-1", "user-1", configuredRequest(hmbSourceID))
	require.NoError(t, err)
	second := configuredRequest(secondSourceID)
	second.SecretReference = "vault://sync/another-company"
	_, err = service.Configure(context.Background(), "tenant-1", "user-1", second)
	require.NoError(t, err)

	require.Len(t, store.controls, 2)
	status, err := service.Status(context.Background(), "tenant-1", hmbSourceID)
	require.NoError(t, err)
	assert.Equal(t, "Hold My Beer OÜ", status.SourceCompanyName)

	otherTenant, err := service.Status(context.Background(), "tenant-2", hmbSourceID)
	require.NoError(t, err)
	assert.False(t, otherTenant.Configured)
	assert.Empty(t, otherTenant.SourceCompanyName)
	_, err = service.Status(context.Background(), "tenant-1", "")
	assert.ErrorContains(t, err, "source company id is required")
}

func TestDryRunRecordsIntentButDoesNotStartFinancialWrites(t *testing.T) {
	store := &memoryStore{}
	service := newTestService(store)
	fixedNow := time.Date(2026, 8, 27, 14, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	_, err := service.Configure(context.Background(), "tenant-1", "user-1", configuredRequest(hmbSourceID))
	require.NoError(t, err)

	status, err := service.RequestDryRun(context.Background(), "tenant-1", hmbSourceID)

	require.NoError(t, err)
	assert.Equal(t, CaptureStatusAwaitingBridge, status.CaptureStatus)
	assert.Equal(t, PlanStatusAwaitingCapturedInput, status.PlanStatus)
	assert.Equal(t, ReconciliationStatusPending, status.ReconciliationStatus)
	assert.False(t, status.FinancialApplyEligible)
	assert.False(t, status.FinancialWritesStarted)
	assert.Equal(t, 1, store.dryRuns)
}

func TestConfirmFinancialApplyRequiresExplicitConfirmationAndRemainsBlocked(t *testing.T) {
	store := &memoryStore{}
	service := newTestService(store)
	_, err := service.Configure(context.Background(), "tenant-1", "user-1", configuredRequest(hmbSourceID))
	require.NoError(t, err)

	_, err = service.ConfirmFinancialApply(context.Background(), "tenant-1", hmbSourceID, ConfirmApplyRequest{})
	assert.ErrorIs(t, err, ErrExplicitConfirmation)

	status, err := service.ConfirmFinancialApply(context.Background(), "tenant-1", hmbSourceID, ConfirmApplyRequest{Confirm: true})
	assert.ErrorIs(t, err, ErrFinancialApplyUnavailable)
	require.NotNil(t, status)
	assert.False(t, status.FinancialWritesStarted)
}

func TestUnavailableCatalogBlocksDiscoveryAndConfiguration(t *testing.T) {
	service := NewService(&memoryStore{}, UnavailableBridgeCatalog{})
	_, err := service.DiscoverSources(context.Background(), "tenant-1")
	assert.ErrorIs(t, err, ErrBridgeDiscoveryUnavailable)

	_, err = service.Configure(context.Background(), "tenant-1", "user-1", configuredRequest(hmbSourceID))
	assert.ErrorIs(t, err, ErrBridgeDiscoveryUnavailable)
}

type captureHistoryBridge struct {
	progresses []CaptureProgress
	requests   []CaptureRequest
}

func (b *captureHistoryBridge) ConnectAndValidate(context.Context, string, string) (BridgeConnection, error) {
	return BridgeConnection{}, errors.New("not used")
}

func (b *captureHistoryBridge) StartCapture(_ context.Context, _ string, _ string, request CaptureRequest) (CaptureProgress, error) {
	b.requests = append(b.requests, request)
	if len(b.progresses) == 0 {
		return CaptureProgress{}, errors.New("no capture progress configured")
	}
	progress := b.progresses[0]
	b.progresses = b.progresses[1:]
	return progress, nil
}

func (b *captureHistoryBridge) GetCapture(_ context.Context, _ string, _ string, _ string) (CaptureProgress, error) {
	if len(b.progresses) == 0 {
		return CaptureProgress{RunID: "capture-window", Status: "completed_with_review"}, nil
	}
	return b.progresses[len(b.progresses)-1], nil
}

func TestSelectedDateWindowCaptureRetainsEarlierFullHistoryProgress(t *testing.T) {
	store := &memoryStore{controls: map[string]Control{
		controlKey("tenant-1", hmbSourceID): {TenantID: "tenant-1", SourceCompanyID: hmbSourceID, SourceCompanyName: "Hold My Beer OÜ", SecretReference: "secret-ref://sa-bridge/connection-hmb"},
	}}
	service := newTestService(store)
	bridge := &captureHistoryBridge{progresses: []CaptureProgress{
		{RunID: "capture-full", Status: "completed_with_review", ScopeMode: "full_history", ResourceIDs: []string{"general.entries.get", "inventory.warehouse_movements.get"}, Resources: []CaptureResourceStatus{{ResourceID: "inventory.warehouse_movements.get", Status: "review_required", ReasonCode: "source_date_window_required"}}, Summary: CaptureSummary{Total: 2, Completed: 1, ReviewRequired: 1}},
		{RunID: "capture-window", Status: "running", ScopeMode: "window", DateFrom: "2020-01-01", DateTo: "2026-08-27", ResourceIDs: []string{"inventory.warehouse_movements.get"}, Resources: []CaptureResourceStatus{{ResourceID: "inventory.warehouse_movements.get", Status: "running"}}, Summary: CaptureSummary{Total: 1, Running: 1}},
	}}
	_, err := service.StartCapture(context.Background(), "tenant-1", hmbSourceID, CaptureRequest{ScopeMode: "full_history"}, bridge)
	require.NoError(t, err)
	status, err := service.StartCapture(context.Background(), "tenant-1", hmbSourceID, CaptureRequest{ScopeMode: "window", DateFrom: "2020-01-01", DateTo: "2026-08-27", ResourceIDs: []string{"inventory.warehouse_movements.get"}}, bridge)
	require.NoError(t, err)
	assert.Equal(t, "capture-window", status.CaptureRunID)
	require.Len(t, status.CaptureProgresses, 2)
	assert.Equal(t, []CaptureRequest{
		{ScopeMode: "full_history"},
		{ScopeMode: "window", DateFrom: "2020-01-01", DateTo: "2026-08-27", ResourceIDs: []string{"inventory.warehouse_movements.get"}},
	}, bridge.requests)
	assert.Contains(t, store.captureProgress, controlKey("tenant-1", hmbSourceID)+"\x00capture-full")
	assert.Contains(t, store.captureProgress, controlKey("tenant-1", hmbSourceID)+"\x00capture-window")
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return string(payload)
}
