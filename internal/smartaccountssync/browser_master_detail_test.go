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

type browserMasterDetailMemoryStore struct {
	controls map[string]Control
	byRun    map[string]BrowserMasterDetailAuthorization
	byBatch  map[string]string
}

func (s *browserMasterDetailMemoryStore) Get(_ context.Context, tenantID, sourceID string) (*Control, error) {
	value, found := s.controls[controlKey(tenantID, sourceID)]
	if !found {
		return nil, ErrControlNotConfigured
	}
	return &value, nil
}

func (s *browserMasterDetailMemoryStore) FindOrCreateBrowserMasterDetailAuthorization(_ context.Context, value BrowserMasterDetailAuthorization) (*BrowserMasterDetailAuthorization, bool, error) {
	if s.byRun == nil {
		s.byRun, s.byBatch = map[string]BrowserMasterDetailAuthorization{}, map[string]string{}
	}
	key := value.TenantID + "\x00" + value.BatchID + "\x00" + value.ResourceID
	if runID, found := s.byBatch[key]; found {
		persisted := s.byRun[runID]
		return &persisted, false, nil
	}
	s.byRun[value.RunID] = value
	s.byBatch[key] = value.RunID
	return &value, true, nil
}

func (s *browserMasterDetailMemoryStore) GetBrowserMasterDetailAuthorization(_ context.Context, runID, tenantID string) (*BrowserMasterDetailAuthorization, error) {
	value, found := s.byRun[runID]
	if !found || value.TenantID != tenantID {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	return &value, nil
}

func (s *browserMasterDetailMemoryStore) RotateBrowserMasterDetailAuthorization(_ context.Context, value BrowserMasterDetailAuthorization) error {
	stored, found := s.byRun[value.RunID]
	if !found || stored.TenantID != value.TenantID {
		return ErrBrowserMasterDetailUnauthorized
	}
	s.byRun[value.RunID] = value
	return nil
}

type browserMasterDetailBridgeStub struct {
	starts    []BrowserMasterDetailStartRequest
	runs      []string
	finalized map[string]bool
}

type browserMasterDetailStagingVerifier struct {
	tenant, source, packageID, packageSHA string
	err                                   error
	calls                                 int
}

func (v *browserMasterDetailStagingVerifier) VerifyStagedPackage(_ context.Context, tenantID, sourceCompanyID, packageID, packageSHA256 string) error {
	v.calls++
	if v.err != nil {
		return v.err
	}
	if v.tenant != tenantID || v.source != sourceCompanyID || v.packageID != packageID || v.packageSHA != packageSHA256 {
		return errors.New("staged package binding mismatch")
	}
	return nil
}

func (s *browserMasterDetailBridgeStub) StartBrowserMasterDetail(_ context.Context, _ string, runID string, request BrowserMasterDetailStartRequest) (BrowserMasterDetailStatus, error) {
	s.starts, s.runs = append(s.starts, request), append(s.runs, runID)
	_, _, sourceSchema, _ := browserMasterDetailContractFor(request.ResourceID)
	contractSHA, _ := browserMasterDetailSHA256(request.Contract)
	return BrowserMasterDetailStatus{RunID: runID, Status: "open", ManifestVersion: request.ManifestVersion, ResourceID: request.ResourceID, SchemaID: request.SchemaID, SourceSchema: sourceSchema, ContractSHA256: contractSHA}, nil
}

func (s *browserMasterDetailBridgeStub) GetBrowserMasterDetail(_ context.Context, _ string, runID string) (BrowserMasterDetailStatus, error) {
	for index, candidate := range s.runs {
		if candidate == runID {
			request := s.starts[index]
			_, _, sourceSchema, _ := browserMasterDetailContractFor(request.ResourceID)
			contractSHA, _ := browserMasterDetailSHA256(request.Contract)
			status := BrowserMasterDetailStatus{RunID: runID, Status: "open", ManifestVersion: request.ManifestVersion, ResourceID: request.ResourceID, SchemaID: request.SchemaID, SourceSchema: sourceSchema, ContractSHA256: contractSHA}
			if s.finalized[runID] {
				status.Status, status.NDJSONSHA256, status.RecordCount = "finalized", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 2
				status.PackageID, status.PackageSHA256 = "master-detail-package", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			}
			return status, nil
		}
	}
	return BrowserMasterDetailStatus{}, errors.New("run not started")
}

func (s *browserMasterDetailBridgeStub) UploadBrowserMasterDetail(_ context.Context, _ string, runID, _ string, _ []byte) (BrowserMasterDetailUploadResult, error) {
	return BrowserMasterDetailUploadResult{RunID: runID, Status: "accepted", Created: true}, nil
}

func (s *browserMasterDetailBridgeStub) FinalizeBrowserMasterDetail(_ context.Context, _ string, runID string) (BrowserMasterDetailStatus, error) {
	if s.finalized == nil {
		s.finalized = map[string]bool{}
	}
	s.finalized[runID] = true
	status, err := s.GetBrowserMasterDetail(context.Background(), "", runID)
	if err != nil {
		return BrowserMasterDetailStatus{}, err
	}
	return status, nil
}

func newBrowserMasterDetailTestService(now time.Time) (*BrowserMasterDetailService, *browserMasterDetailMemoryStore, *browserMasterDetailBridgeStub) {
	store := &browserMasterDetailMemoryStore{controls: map[string]Control{controlKey(browserPairingTenantID, browserSourceID): {TenantID: browserPairingTenantID, SourceCompanyID: browserSourceID, SecretReference: browserSessionReference(browserPairingID)}}}
	bridge := &browserMasterDetailBridgeStub{}
	service := NewBrowserMasterDetailService(store, store, bridge)
	service.now = func() time.Time { return now }
	tokenNumber := 0
	service.newToken = func() (string, error) {
		tokenNumber++
		return fmt.Sprintf("%043d", tokenNumber), nil
	}
	return service, store, bridge
}

func TestBrowserMasterDetailAuthorizesExactThreeResourceWorkflowAndRehydratesPrivateStatus(t *testing.T) {
	now := time.Date(2026, 8, 28, 21, 30, 0, 0, time.UTC) // Tallinn is the following day.
	service, store, bridge := newBrowserMasterDetailTestService(now)
	issues, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserMasterDetailAuthorizeRequest{SourceCompanyID: browserSourceID, TransferConsentConfirmed: true})

	require.NoError(t, err)
	require.Len(t, issues.Issues, 3)
	assert.Equal(t, []string{"clients", "vendors", "articles"}, []string{issues.Issues[0].ResourceID, issues.Issues[1].ResourceID, issues.Issues[2].ResourceID})
	assert.Equal(t, "2026-08-29", issues.Issues[0].SnapshotDate)
	assert.Equal(t, "2026-08-29", issues.Issues[0].Scope.FromInclusive)
	assert.Equal(t, "2026-08-28T21:30:00Z", issues.Issues[0].Scope.CutoffAt)
	assert.Equal(t, BrowserMasterDetailSnapshotPolicy, issues.Issues[0].SnapshotPolicy)
	assert.Len(t, bridge.starts, 3)
	assert.Equal(t, "clients_detail_v1", bridge.starts[0].SchemaID)
	assert.Equal(t, "/et/clients", bridge.starts[0].Contract.DetailResultPagePath)
	assert.NotContains(t, mustJSON(t, store.byRun), issues.Issues[0].CaptureToken)

	status, err := service.Status(context.Background(), browserPairingTenantID, issues.Issues[0].RunID, issues.Issues[0].CaptureToken)
	require.NoError(t, err)
	assert.Equal(t, browserPairingTenantID, status.TenantID)
	assert.Equal(t, browserSourceID, status.SourceCompanyID)
	assert.Equal(t, issues.Issues[0].Scope, status.Scope)
	assert.Equal(t, issues.Issues[0].ApprovalSHA256, status.ApprovalSHA256)
}

func TestBrowserMasterDetailExactContractGoldenDigests(t *testing.T) {
	expected := map[string]string{
		BrowserMasterDetailClientsResource:  "e5fc0e94878e4728428b0a68a114e2853cb0b0b2973a407f16f7455b1155d46a",
		BrowserMasterDetailVendorsResource:  "0c1ffbc7e7e856b7331796e27a32fff60a9abfe41b0784f81373805d442e6e67",
		BrowserMasterDetailArticlesResource: "e129484661ce11a0f11f2299ca0120b1293753fa5b805fc3dfbbd7987a11a46c",
	}
	for resource, digest := range expected {
		contract, _, _, found := browserMasterDetailContractFor(resource)
		require.True(t, found)
		actual, err := browserMasterDetailSHA256(contract)
		require.NoError(t, err)
		assert.Equal(t, digest, actual, resource)
	}
}

func TestBrowserMasterDetailBatchRetryAndExplicitSameDayRefreshRemainIsolated(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	service, _, bridge := newBrowserMasterDetailTestService(now)
	first, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserMasterDetailAuthorizeRequest{SourceCompanyID: browserSourceID, TransferConsentConfirmed: true})
	require.NoError(t, err)
	service.now = func() time.Time { return now.Add(time.Minute) }
	retry, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserMasterDetailAuthorizeRequest{SourceCompanyID: browserSourceID, TransferConsentConfirmed: true, BatchID: first.BatchID})
	require.NoError(t, err)
	for index := range first.Issues {
		assert.Equal(t, first.Issues[index].RunID, retry.Issues[index].RunID)
		assert.Equal(t, first.Issues[index].Scope, retry.Issues[index].Scope)
		assert.NotEqual(t, first.Issues[index].CaptureToken, retry.Issues[index].CaptureToken)
	}
	refreshed, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserMasterDetailAuthorizeRequest{SourceCompanyID: browserSourceID, TransferConsentConfirmed: true, Refresh: true})
	require.NoError(t, err)
	assert.NotEqual(t, first.BatchID, refreshed.BatchID)
	assert.NotEqual(t, first.Issues[0].RunID, refreshed.Issues[0].RunID)
	assert.Len(t, bridge.starts, 9)
}

func TestBrowserMasterDetailRejectsCrossTenantAndExpiredCapability(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	service, _, _ := newBrowserMasterDetailTestService(now)
	issues, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserMasterDetailAuthorizeRequest{SourceCompanyID: browserSourceID, TransferConsentConfirmed: true})
	require.NoError(t, err)
	_, err = service.Status(context.Background(), "7c7e0e25-1aae-464e-aee0-5c8d9687a5d0", issues.Issues[0].RunID, issues.Issues[0].CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserMasterDetailUnauthorized)
	service.now = func() time.Time { return issues.Issues[0].ExpiresAt.Add(time.Second) }
	_, err = service.Status(context.Background(), browserPairingTenantID, issues.Issues[0].RunID, issues.Issues[0].CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserMasterDetailUnauthorized)
}

func TestBrowserMasterDetailPromotesOnlyExactOAStagedPackage(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	service, _, _ := newBrowserMasterDetailTestService(now)
	issues, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserMasterDetailAuthorizeRequest{SourceCompanyID: browserSourceID, TransferConsentConfirmed: true})
	require.NoError(t, err)
	first := issues.Issues[0]

	// A sealed bridge package is archive evidence until OA verifies its exact
	// tenant/source/package/digest staging record.
	status, err := service.Finalize(context.Background(), browserPairingTenantID, first.RunID, first.CaptureToken)
	require.NoError(t, err)
	assert.Equal(t, "finalized_archived_evidence", status.Status)

	verifier := &browserMasterDetailStagingVerifier{tenant: browserPairingTenantID, source: browserSourceID, packageID: "master-detail-package", packageSHA: strings.Repeat("b", 64)}
	service.SetStagedPackageVerifier(verifier)
	status, err = service.OwnerStatus(context.Background(), browserPairingTenantID, first.RunID)
	require.NoError(t, err)
	assert.Equal(t, "STAGED_REVIEW_REQUIRED", status.Status)
	assert.Equal(t, 1, verifier.calls)

	// A mismatched verifier result cannot turn an archive-only result into a
	// preview/apply signal, including after an owner status refresh.
	verifier.packageSHA = strings.Repeat("c", 64)
	status, err = service.OwnerStatus(context.Background(), browserPairingTenantID, first.RunID)
	require.NoError(t, err)
	assert.Equal(t, "finalized_archived_evidence", status.Status)
}
