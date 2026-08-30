package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	batchSourceOne   = "sa-browser-v1-111111"
	batchSourceTwo   = "sa-browser-v1-222222"
	batchSourceThree = "sa-browser-v1-333333"
	batchTenantOne   = "a436c224-5df5-4b4d-a772-1897f9147400"
	batchTenantTwo   = "b436c224-5df5-4b4d-a772-1897f9147400"
	batchPairingOne  = "11111111-1111-4111-8111-111111111111"
	batchPairingTwo  = "22222222-2222-4222-8222-222222222222"
	batchCatalogID   = "33333333-3333-4333-8333-333333333333"
)

type browserOnboardingBatchMemoryStore struct {
	mu           sync.Mutex
	byReceiptKey map[string]BrowserOnboardingBatch
	byID         map[string]BrowserOnboardingBatch
	outcomes     map[string]map[string]BrowserOnboardingBatchOutcome
	startLocks   map[string]*sync.Mutex
}

func batchStoreKey(ownerID, catalogReceiptID string) string { return ownerID + "/" + catalogReceiptID }

func (s *browserOnboardingBatchMemoryStore) WithBrowserOnboardingBatchStartLock(_ context.Context, ownerID, catalogReceiptID string, callback func() error) error {
	if callback == nil {
		return ErrBrowserOnboardingBatchUnavailable
	}
	key := batchStoreKey(ownerID, catalogReceiptID)
	s.mu.Lock()
	if s.startLocks == nil {
		s.startLocks = map[string]*sync.Mutex{}
	}
	lock := s.startLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		s.startLocks[key] = lock
	}
	s.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return callback()
}

func cloneBrowserOnboardingBatch(batch BrowserOnboardingBatch) BrowserOnboardingBatch {
	batch.SelectedSources = append([]BrowserOnboardingSource(nil), batch.SelectedSources...)
	batch.ObservedSourceIDs = append([]string(nil), batch.ObservedSourceIDs...)
	return batch
}

func (s *browserOnboardingBatchMemoryStore) FindBrowserOnboardingBatchByCatalogReceipt(_ context.Context, ownerID, catalogReceiptID string) (*BrowserOnboardingBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, found := s.byReceiptKey[batchStoreKey(ownerID, catalogReceiptID)]
	if !found {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	batch = cloneBrowserOnboardingBatch(batch)
	return &batch, nil
}

func (s *browserOnboardingBatchMemoryStore) GetBrowserOnboardingBatch(_ context.Context, ownerID, batchID string) (*BrowserOnboardingBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, found := s.byID[batchID]
	if !found || batch.OwnerID != ownerID {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	batch = cloneBrowserOnboardingBatch(batch)
	return &batch, nil
}

func (s *browserOnboardingBatchMemoryStore) CreateBrowserOnboardingBatch(_ context.Context, batch BrowserOnboardingBatch) (*BrowserOnboardingBatch, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byReceiptKey == nil {
		s.byReceiptKey = map[string]BrowserOnboardingBatch{}
		s.byID = map[string]BrowserOnboardingBatch{}
		s.outcomes = map[string]map[string]BrowserOnboardingBatchOutcome{}
	}
	key := batchStoreKey(batch.OwnerID, batch.CatalogReceiptID)
	if _, found := s.byReceiptKey[key]; found {
		return nil, false, nil
	}
	batch = cloneBrowserOnboardingBatch(batch)
	s.byReceiptKey[key] = batch
	s.byID[batch.ID] = batch
	return &batch, true, nil
}

func (s *browserOnboardingBatchMemoryStore) SaveBrowserOnboardingBatchProgress(_ context.Context, ownerID, batchID, status string, outcomes []BrowserOnboardingBatchOutcome, updatedAt time.Time) (*BrowserOnboardingBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, found := s.byID[batchID]
	if !found || batch.OwnerID != ownerID {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	batch.Status = status
	batch.UpdatedAt = updatedAt.UTC()
	s.byID[batchID] = batch
	s.byReceiptKey[batchStoreKey(ownerID, batch.CatalogReceiptID)] = batch
	if s.outcomes[batchID] == nil {
		s.outcomes[batchID] = map[string]BrowserOnboardingBatchOutcome{}
	}
	for _, outcome := range outcomes {
		if original, exists := s.outcomes[batchID][outcome.SourceCompanyID]; exists {
			outcome.CreatedAt = original.CreatedAt
		}
		s.outcomes[batchID][outcome.SourceCompanyID] = outcome
	}
	copy := cloneBrowserOnboardingBatch(batch)
	return &copy, nil
}

func (s *browserOnboardingBatchMemoryStore) ListBrowserOnboardingBatchOutcomes(_ context.Context, ownerID, batchID string) ([]BrowserOnboardingBatchOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, found := s.byID[batchID]
	if !found || batch.OwnerID != ownerID {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	output := make([]BrowserOnboardingBatchOutcome, 0, len(s.outcomes[batchID]))
	for _, outcome := range s.outcomes[batchID] {
		output = append(output, outcome)
	}
	return canonicalBrowserOnboardingBatchOutcomes(output), nil
}

type browserOnboardingBatchCatalog struct {
	mu       sync.Mutex
	receipts map[string]BrowserOnboardingCatalogReceipt
	err      error
}

func batchCatalogKey(ownerID, receiptID string) string { return ownerID + "/" + receiptID }

func (c *browserOnboardingBatchCatalog) GetBrowserOnboardingCatalogReceipt(_ context.Context, ownerID, receiptID string) (*BrowserOnboardingCatalogReceipt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	receipt, found := c.receipts[batchCatalogKey(ownerID, receiptID)]
	if !found {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	receipt.Sources = append([]BrowserOnboardingSource(nil), receipt.Sources...)
	return &receipt, nil
}

type browserOnboardingBatchRunner struct {
	mu            sync.Mutex
	startCalls    int
	statusCalls   int
	startByOwner  map[string]*BrowserOnboardingResponse
	statusByOwner map[string]map[string]*BrowserOnboardingResult
	startErr      error
}

func (r *browserOnboardingBatchRunner) Start(_ context.Context, ownerID string, request BrowserOnboardingRequest) (*BrowserOnboardingResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startCalls++
	if r.startErr != nil {
		return nil, r.startErr
	}
	response := r.startByOwner[ownerID]
	if response == nil {
		return &BrowserOnboardingResponse{}, nil
	}
	copy := &BrowserOnboardingResponse{Bindings: append([]BrowserOnboardingResult(nil), response.Bindings...)}
	return copy, nil
}

func (r *browserOnboardingBatchRunner) Status(_ context.Context, ownerID, sourceID string) (*BrowserOnboardingResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusCalls++
	result := r.statusByOwner[ownerID][sourceID]
	if result == nil {
		return nil, ErrBrowserOnboardingNotFound
	}
	copy := *result
	return &copy, nil
}

func batchResult(sourceID, sourceName, tenantID, pairingID, status, reason string) BrowserOnboardingResult {
	return BrowserOnboardingResult{BrowserOnboardingBinding: BrowserOnboardingBinding{SourceCompanyID: sourceID, SourceCompanyName: sourceName, TenantID: tenantID, TenantName: sourceName, PairingID: pairingID, Status: status}, ReasonCode: reason}
}

func newBrowserOnboardingBatchTestService() (*BrowserOnboardingBatchService, *browserOnboardingBatchMemoryStore, *browserOnboardingBatchCatalog, *browserOnboardingBatchRunner) {
	store := &browserOnboardingBatchMemoryStore{}
	sources := []BrowserOnboardingSource{{SourceCompanyID: batchSourceTwo, SourceCompanyName: "Same Visible Name"}, {SourceCompanyID: batchSourceOne, SourceCompanyName: "Same Visible Name"}}
	catalog := &browserOnboardingBatchCatalog{receipts: map[string]BrowserOnboardingCatalogReceipt{
		batchCatalogKey("owner-1", batchCatalogID):                         testBrowserOnboardingCatalogReceipt("owner-1", batchCatalogID, sources),
		batchCatalogKey("owner-2", "44444444-4444-4444-8444-444444444444"): testBrowserOnboardingCatalogReceipt("owner-2", "44444444-4444-4444-8444-444444444444", sources),
	}}
	runner := &browserOnboardingBatchRunner{startByOwner: map[string]*BrowserOnboardingResponse{}, statusByOwner: map[string]map[string]*BrowserOnboardingResult{}}
	service := NewBrowserOnboardingBatchService(store, catalog, runner)
	service.now = func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	service.newID = uuid.NewString
	return service, store, catalog, runner
}

func batchRequest(mode string, sourceIDs ...string) BrowserOnboardingBatchRequest {
	return BrowserOnboardingBatchRequest{CatalogReceiptID: batchCatalogID, Mode: mode, SelectedSourceIDs: sourceIDs, OwnerConfirmed: true}
}

func testBrowserOnboardingCatalogReceipt(ownerID, receiptID string, sources []BrowserOnboardingSource) BrowserOnboardingCatalogReceipt {
	companies := sourcesToBrowserOnboardingCatalogCompanies(sources)
	canonical, ok := canonicalBrowserOnboardingCatalogCompanies(companies)
	if !ok {
		panic("invalid test catalog")
	}
	encoded, err := jsonMarshalBrowserOnboardingCatalogDigest(BrowserOnboardingCatalogSchemaVersion, canonical)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(encoded)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	return BrowserOnboardingCatalogReceipt{ID: receiptID, WorkflowID: "55555555-5555-4555-8555-555555555555", OwnerID: ownerID, TokenSHA256: strings.Repeat("a", 64), NonceSHA256: strings.Repeat("b", 64), SchemaVersion: BrowserOnboardingCatalogSchemaVersion, IntentVersion: BrowserOnboardingCatalogIntentVersion, SourceIDVersion: BrowserOnboardingCatalogSourceIDVersion, DigestAlgorithm: BrowserOnboardingCatalogDigestAlgorithm, Status: BrowserOnboardingCatalogStatusAccepted, CatalogSHA256: hex.EncodeToString(digest[:]), CatalogCount: len(canonical), Sources: browserOnboardingCatalogCompaniesToSources(canonical), ObservedAt: now, ExpiresAt: now.Add(2 * time.Minute), ReceiptExpiresAt: now.Add(10 * time.Minute), AcceptedAt: now, CreatedAt: now, UpdatedAt: now}
}

func TestBrowserOnboardingBatchAllRequiresExactObservedSetAndExactRetryReusesManifest(t *testing.T) {
	service, _, _, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""),
		batchResult(batchSourceTwo, "Same Visible Name", batchTenantTwo, batchPairingTwo, BrowserOnboardingPaired, ""),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{
		batchSourceOne: ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, "")),
		batchSourceTwo: ptrBatchResult(batchResult(batchSourceTwo, "Same Visible Name", batchTenantTwo, batchPairingTwo, BrowserOnboardingPaired, "")),
	}

	request := batchRequest(BrowserOnboardingBatchModeAll, batchSourceTwo, batchSourceOne)
	first, err := service.Start(context.Background(), "owner-1", request)
	require.NoError(t, err)
	assert.False(t, first.Reused)
	assert.Equal(t, BrowserOnboardingBatchReady, first.Batch.Status)
	assert.Equal(t, []string{batchSourceOne, batchSourceTwo}, first.Batch.ObservedSourceIDs)
	require.Len(t, first.Outcomes, 2)
	assert.Equal(t, batchSourceOne, first.Outcomes[0].SourceCompanyID)
	assert.Equal(t, batchSourceTwo, first.Outcomes[1].SourceCompanyID)

	second, err := service.Start(context.Background(), "owner-1", request)
	require.NoError(t, err)
	assert.True(t, second.Reused)
	assert.Equal(t, first.Batch.ID, second.Batch.ID)
	assert.Equal(t, 1, runner.startCalls)

	_, err = service.Start(context.Background(), "owner-1", batchRequest(BrowserOnboardingBatchModeAll, batchSourceOne))
	assert.ErrorIs(t, err, ErrBrowserOnboardingBatchInvalid)
	assert.Equal(t, 1, runner.startCalls)
}

func TestBrowserOnboardingBatchSelectedRequiresNonemptyStrictSubset(t *testing.T) {
	service, _, _, runner := newBrowserOnboardingBatchTestService()
	for _, request := range []BrowserOnboardingBatchRequest{
		batchRequest(BrowserOnboardingBatchModeSelected),
		batchRequest(BrowserOnboardingBatchModeSelected, batchSourceOne, batchSourceTwo),
		batchRequest(BrowserOnboardingBatchModeSelected, batchSourceThree),
		batchRequest(BrowserOnboardingBatchModeAll, batchSourceOne, batchSourceOne),
		{CatalogReceiptID: batchCatalogID, Mode: BrowserOnboardingBatchModeSelected, SelectedSourceIDs: []string{batchSourceOne}},
	} {
		_, err := service.Start(context.Background(), "owner-1", request)
		assert.ErrorIs(t, err, ErrBrowserOnboardingBatchInvalid)
	}
	assert.Zero(t, runner.startCalls)
}

func TestBrowserOnboardingBatchRejectsChangedSelectionAndCatalogNameForSameObservedSet(t *testing.T) {
	service, _, catalog, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{batchSourceOne: ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""))}
	firstRequest := batchRequest(BrowserOnboardingBatchModeSelected, batchSourceOne)
	_, err := service.Start(context.Background(), "owner-1", firstRequest)
	require.NoError(t, err)

	_, err = service.Start(context.Background(), "owner-1", batchRequest(BrowserOnboardingBatchModeSelected, batchSourceTwo))
	assert.ErrorIs(t, err, ErrBrowserOnboardingBatchConflict)

	catalog.mu.Lock()
	receipt := catalog.receipts[batchCatalogKey("owner-1", batchCatalogID)]
	receipt = testBrowserOnboardingCatalogReceipt("owner-1", receipt.ID, []BrowserOnboardingSource{{SourceCompanyID: batchSourceOne, SourceCompanyName: "Renamed"}, {SourceCompanyID: batchSourceTwo, SourceCompanyName: "Same Visible Name"}})
	catalog.receipts[batchCatalogKey("owner-1", batchCatalogID)] = receipt
	catalog.mu.Unlock()
	_, err = service.Start(context.Background(), "owner-1", firstRequest)
	assert.ErrorIs(t, err, ErrBrowserOnboardingBatchConflict)
	assert.Equal(t, 1, runner.startCalls)
}

func TestBrowserOnboardingBatchAllowsFreshReceiptWithSameObservedCatalog(t *testing.T) {
	service, store, catalog, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{
		batchSourceOne: ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, "")),
	}
	first, err := service.Start(context.Background(), "owner-1", batchRequest(BrowserOnboardingBatchModeSelected, batchSourceOne))
	require.NoError(t, err)

	const freshReceiptID = "66666666-6666-4666-8666-666666666666"
	catalog.mu.Lock()
	catalog.receipts[batchCatalogKey("owner-1", freshReceiptID)] = testBrowserOnboardingCatalogReceipt("owner-1", freshReceiptID, []BrowserOnboardingSource{{SourceCompanyID: batchSourceOne, SourceCompanyName: "Same Visible Name"}, {SourceCompanyID: batchSourceTwo, SourceCompanyName: "Same Visible Name"}})
	catalog.mu.Unlock()
	second, err := service.Start(context.Background(), "owner-1", BrowserOnboardingBatchRequest{CatalogReceiptID: freshReceiptID, Mode: BrowserOnboardingBatchModeSelected, SelectedSourceIDs: []string{batchSourceOne}, OwnerConfirmed: true})
	require.NoError(t, err)
	assert.NotEqual(t, first.Batch.ID, second.Batch.ID)
	assert.Equal(t, first.Batch.ObservedSourcesSHA256, second.Batch.ObservedSourcesSHA256)
	store.mu.Lock()
	assert.Len(t, store.byID, 2)
	store.mu.Unlock()
}

func TestBrowserOnboardingBatchPersistsPartialFailureWithoutClaimingReady(t *testing.T) {
	service, _, _, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""),
		batchResult(batchSourceTwo, "Same Visible Name", "", "", BrowserOnboardingFailed, "tenant_create_failed"),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{
		batchSourceOne: ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, "")),
		batchSourceTwo: ptrBatchResult(batchResult(batchSourceTwo, "Same Visible Name", "", "", BrowserOnboardingFailed, "tenant_create_failed")),
	}

	response, err := service.Start(context.Background(), "owner-1", batchRequest(BrowserOnboardingBatchModeAll, batchSourceOne, batchSourceTwo))
	require.NoError(t, err)
	assert.Equal(t, BrowserOnboardingBatchReview, response.Batch.Status)
	require.Len(t, response.Outcomes, 2)
	assert.Equal(t, BrowserOnboardingPaired, response.Outcomes[0].Status)
	assert.Equal(t, BrowserOnboardingFailed, response.Outcomes[1].Status)
	assert.Equal(t, "tenant_create_failed", response.Outcomes[1].ReasonCode)
}

func TestBrowserOnboardingBatchNeedsClaimedExpectedSourcePairingBeforeReady(t *testing.T) {
	service, _, _, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPairingIssued, ""),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{batchSourceOne: ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPairingIssued, ""))}

	started, err := service.Start(context.Background(), "owner-1", batchRequest(BrowserOnboardingBatchModeSelected, batchSourceOne))
	require.NoError(t, err)
	assert.Equal(t, BrowserOnboardingBatchPending, started.Batch.Status)

	runner.mu.Lock()
	runner.statusByOwner["owner-1"][batchSourceOne] = ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""))
	runner.mu.Unlock()
	ready, err := service.Status(context.Background(), "owner-1", started.Batch.ID)
	require.NoError(t, err)
	assert.Equal(t, BrowserOnboardingBatchReady, ready.Batch.Status)

	// A nominal PAIRED status without the durable expected-source pairing ID
	// cannot make an immutable batch ready.
	runner.mu.Lock()
	runner.statusByOwner["owner-1"][batchSourceOne] = ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, "", BrowserOnboardingPaired, ""))
	runner.mu.Unlock()
	pending, err := service.Status(context.Background(), "owner-1", started.Batch.ID)
	require.NoError(t, err)
	assert.Equal(t, BrowserOnboardingBatchPending, pending.Batch.Status)
}

func TestBrowserOnboardingBatchIsOwnerScopedAndKeepsSameNamesDistinct(t *testing.T) {
	service, _, _, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""),
		batchResult(batchSourceTwo, "Same Visible Name", batchTenantTwo, batchPairingTwo, BrowserOnboardingPaired, ""),
	}}
	runner.startByOwner["owner-2"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", "", "", BrowserOnboardingReview, "source_already_bound"),
		batchResult(batchSourceTwo, "Same Visible Name", "", "", BrowserOnboardingReview, "source_already_bound"),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{}
	runner.statusByOwner["owner-2"] = map[string]*BrowserOnboardingResult{}
	request := batchRequest(BrowserOnboardingBatchModeAll, batchSourceOne, batchSourceTwo)

	ownerOne, err := service.Start(context.Background(), "owner-1", request)
	require.NoError(t, err)
	requestForOwnerTwo := request
	requestForOwnerTwo.CatalogReceiptID = "44444444-4444-4444-8444-444444444444"
	ownerTwo, err := service.Start(context.Background(), "owner-2", requestForOwnerTwo)
	require.NoError(t, err)
	assert.NotEqual(t, ownerOne.Batch.ID, ownerTwo.Batch.ID)
	assert.Equal(t, BrowserOnboardingBatchReady, ownerOne.Batch.Status)
	assert.Equal(t, BrowserOnboardingBatchReview, ownerTwo.Batch.Status)
	assert.Empty(t, ownerTwo.Outcomes[0].TenantID)
	assert.Empty(t, ownerTwo.Outcomes[0].TenantName)
	assert.NotEqual(t, ownerOne.Outcomes[0].TenantID, ownerOne.Outcomes[1].TenantID)

	_, err = service.Status(context.Background(), "owner-2", ownerOne.Batch.ID)
	assert.ErrorIs(t, err, ErrBrowserOnboardingBatchNotFound)
}

func TestBrowserOnboardingBatchConcurrentExactRetryCreatesOneManifestAndOneOnboardingStart(t *testing.T) {
	service, store, _, runner := newBrowserOnboardingBatchTestService()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{
		batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""),
	}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{batchSourceOne: ptrBatchResult(batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPaired, ""))}
	request := batchRequest(BrowserOnboardingBatchModeSelected, batchSourceOne)

	var group sync.WaitGroup
	errorsCh := make(chan error, 12)
	ids := make(chan string, 12)
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Start(context.Background(), "owner-1", request)
			if err == nil {
				ids <- result.Batch.ID
			}
			errorsCh <- err
		}()
	}
	group.Wait()
	close(errorsCh)
	close(ids)
	for err := range errorsCh {
		assert.NoError(t, err)
	}
	var firstID string
	for id := range ids {
		if firstID == "" {
			firstID = id
		}
		assert.Equal(t, firstID, id)
	}
	runner.mu.Lock()
	starts := runner.startCalls
	runner.mu.Unlock()
	assert.Equal(t, 1, starts)
	store.mu.Lock()
	assert.Len(t, store.byID, 1)
	store.mu.Unlock()
}

func TestBrowserOnboardingBatchReturnsFreshPairingsOnlyFromOwnerConfirmedStart(t *testing.T) {
	service, store, _, runner := newBrowserOnboardingBatchTestService()
	firstToken := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq"
	first := batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingOne, BrowserOnboardingPairingIssued, "")
	first.Pairing = &BrowserPairingIssue{PairingID: batchPairingOne, PairingToken: firstToken, ExpiresAt: time.Date(2026, 8, 28, 12, 10, 0, 0, time.UTC)}
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{first}}
	runner.statusByOwner["owner-1"] = map[string]*BrowserOnboardingResult{batchSourceOne: ptrBatchResult(first)}
	request := batchRequest(BrowserOnboardingBatchModeSelected, batchSourceOne)

	issued, err := service.Start(context.Background(), "owner-1", request)
	require.NoError(t, err)
	require.Len(t, issued.PairingIssues, 1)
	assert.Equal(t, issued.Batch.ID, issued.PairingIssues[0].BatchID)
	assert.Equal(t, batchSourceOne, issued.PairingIssues[0].SourceCompanyID)
	assert.Equal(t, batchTenantOne, issued.PairingIssues[0].TenantID)
	assert.Equal(t, firstToken, issued.PairingIssues[0].Pairing.PairingToken)

	store.mu.Lock()
	persisted, marshalErr := json.Marshal(struct {
		Batch    BrowserOnboardingBatch
		Outcomes map[string]BrowserOnboardingBatchOutcome
	}{Batch: store.byID[issued.Batch.ID], Outcomes: store.outcomes[issued.Batch.ID]})
	store.mu.Unlock()
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(persisted), firstToken)

	secondToken := "0123456789_abcdefghijklmnopqrstuvwxyzABCDEF"
	second := batchResult(batchSourceOne, "Same Visible Name", batchTenantOne, batchPairingTwo, BrowserOnboardingPairingIssued, "")
	second.Pairing = &BrowserPairingIssue{PairingID: batchPairingTwo, PairingToken: secondToken, ExpiresAt: time.Date(2026, 8, 28, 12, 10, 0, 0, time.UTC)}
	runner.mu.Lock()
	runner.startByOwner["owner-1"] = &BrowserOnboardingResponse{Bindings: []BrowserOnboardingResult{second}}
	runner.statusByOwner["owner-1"][batchSourceOne] = ptrBatchResult(second)
	runner.mu.Unlock()

	reissued, err := service.Start(context.Background(), "owner-1", request)
	require.NoError(t, err)
	assert.True(t, reissued.Reused)
	require.Len(t, reissued.PairingIssues, 1)
	assert.Equal(t, secondToken, reissued.PairingIssues[0].Pairing.PairingToken)
	status, err := service.Status(context.Background(), "owner-1", issued.Batch.ID)
	require.NoError(t, err)
	assert.Empty(t, status.PairingIssues)
}

func TestBrowserOnboardingSourceLimitIsSharedByBatchAndRunner(t *testing.T) {
	for _, count := range []int{25, 26, 100, 101, BrowserOnboardingMaxSources, BrowserOnboardingMaxSources + 1} {
		sources := make([]BrowserOnboardingSource, 0, count)
		for index := 1; index <= count; index++ {
			sources = append(sources, BrowserOnboardingSource{SourceCompanyID: fmt.Sprintf("sa-browser-v1-%d", index), SourceCompanyName: fmt.Sprintf("Company %d", index)})
		}
		_, runnerOK := canonicalBrowserOnboardingSources(sources)
		_, batchOK := canonicalBrowserOnboardingBatchSources(sources)
		assert.Equal(t, count <= BrowserOnboardingMaxSources, runnerOK, "runner count %d", count)
		assert.Equal(t, runnerOK, batchOK, "batch/runner count %d", count)
	}
}

func ptrBatchResult(result BrowserOnboardingResult) *BrowserOnboardingResult { return &result }
