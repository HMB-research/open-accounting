package smartaccountssync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	batchWorkflowID        = "9c52d95f-4294-4f98-90cc-a572d9f5bc33"
	batchWorkflowCatalogID = "0c52d95f-4294-4f98-90cc-a572d9f5bc33"
)

type browserBatchWorkflowMemoryStore struct {
	mu        sync.Mutex
	workflows map[string]BrowserBatchWorkflow
	sources   map[string]map[string]BrowserBatchSourceWorkflow
}

type browserBatchWorkflowOnboardingReader struct {
	batch    BrowserOnboardingBatch
	outcomes []BrowserOnboardingBatchOutcome
}

func (r browserBatchWorkflowOnboardingReader) GetBrowserOnboardingBatch(_ context.Context, ownerID, batchID string) (*BrowserOnboardingBatch, error) {
	if ownerID != r.batch.OwnerID || batchID != r.batch.ID {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	copy := cloneBrowserOnboardingBatch(r.batch)
	return &copy, nil
}

func (r browserBatchWorkflowOnboardingReader) ListBrowserOnboardingBatchOutcomes(_ context.Context, ownerID, batchID string) ([]BrowserOnboardingBatchOutcome, error) {
	if ownerID != r.batch.OwnerID || batchID != r.batch.ID {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	return append([]BrowserOnboardingBatchOutcome(nil), r.outcomes...), nil
}

func cloneBrowserBatchWorkflow(workflow BrowserBatchWorkflow) BrowserBatchWorkflow {
	workflow.TransferScope.ResourceIDs = append([]string(nil), workflow.TransferScope.ResourceIDs...)
	workflow.TransferConfirmedAt = copyTime(workflow.TransferConfirmedAt)
	return workflow
}

func cloneBrowserBatchSourceWorkflow(source BrowserBatchSourceWorkflow) BrowserBatchSourceWorkflow {
	source.LeaseExpiresAt = copyTime(source.LeaseExpiresAt)
	return source
}

func TestBrowserBatchWorkflowRecordKeepsUnconfirmedTransferColumnsNull(t *testing.T) {
	now := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	workflow := BrowserBatchWorkflow{
		BatchID:                   batchWorkflowID,
		OwnerID:                   "owner-1",
		SchemaVersion:             BrowserBatchWorkflowSchemaVersion,
		HistoryFrom:               "1900-01-01",
		PreparatoryManifestSHA256: strings.Repeat("a", 64),
		PreparatoryConsentedAt:    now,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	record, err := browserBatchWorkflowToRecord(workflow)
	require.NoError(t, err)
	require.Nil(t, record.TransferManifestSHA256)
	require.Nil(t, record.TransferScope)
	require.Nil(t, record.TransferConfirmedAt)

	restored, err := browserBatchWorkflowFromRecord(record)
	require.NoError(t, err)
	require.Empty(t, restored.TransferManifestSHA256)
}

func TestBrowserBatchSourceRecordKeepsUnavailableProofColumnsNull(t *testing.T) {
	now := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	source := BrowserBatchSourceWorkflow{
		BatchID:         batchWorkflowID,
		SourceCompanyID: "sa-browser-v1-123",
		TenantID:        "1d16e71f-3f79-432e-a3a3-069f461dce2e",
		Phase:           BrowserBatchPhasePaired,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	record, err := browserBatchSourceWorkflowToRecord(source)
	require.NoError(t, err)
	require.Nil(t, record.DiscoveryContractSHA256)
	require.Nil(t, record.DiscoveryReceiptSHA256)
	require.Nil(t, record.SchemaID)
	require.Nil(t, record.SchemaApprovalSHA256)
	require.Nil(t, record.PackageID)
	require.Nil(t, record.PackageSHA256)
	require.Nil(t, record.PreviewSHA256)

	restored, err := browserBatchSourceWorkflowFromRecord(record)
	require.NoError(t, err)
	require.Empty(t, restored.DiscoveryContractSHA256)
	require.Empty(t, restored.SchemaID)
	require.Empty(t, restored.PackageID)
	require.Empty(t, restored.PreviewSHA256)
}

func (s *browserBatchWorkflowMemoryStore) CreateBrowserBatchWorkflow(_ context.Context, workflow BrowserBatchWorkflow, sources []BrowserBatchSourceWorkflow) (*BrowserBatchWorkflow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workflows == nil {
		s.workflows, s.sources = map[string]BrowserBatchWorkflow{}, map[string]map[string]BrowserBatchSourceWorkflow{}
	}
	if existing, found := s.workflows[workflow.BatchID]; found {
		copy := cloneBrowserBatchWorkflow(existing)
		return &copy, false, nil
	}
	s.workflows[workflow.BatchID] = cloneBrowserBatchWorkflow(workflow)
	s.sources[workflow.BatchID] = map[string]BrowserBatchSourceWorkflow{}
	for _, source := range sources {
		s.sources[workflow.BatchID][source.SourceCompanyID] = cloneBrowserBatchSourceWorkflow(source)
	}
	copy := cloneBrowserBatchWorkflow(workflow)
	return &copy, true, nil
}

func (s *browserBatchWorkflowMemoryStore) GetBrowserBatchWorkflow(_ context.Context, ownerID, batchID string) (*BrowserBatchWorkflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	copy := cloneBrowserBatchWorkflow(workflow)
	return &copy, nil
}

func (s *browserBatchWorkflowMemoryStore) ListBrowserBatchSourceWorkflows(_ context.Context, ownerID, batchID string) ([]BrowserBatchSourceWorkflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	result := make([]BrowserBatchSourceWorkflow, 0, len(s.sources[batchID]))
	for _, source := range s.sources[batchID] {
		result = append(result, cloneBrowserBatchSourceWorkflow(source))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ordinal < result[j].Ordinal })
	return result, nil
}

func (s *browserBatchWorkflowMemoryStore) CompareAndSwapBrowserBatchSource(_ context.Context, ownerID, batchID, sourceID, expectedPhase string, expectedGeneration int64, expectedLeaseID string, next BrowserBatchSourceWorkflow) (*BrowserBatchSourceWorkflow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return nil, false, ErrBrowserBatchWorkflowNotFound
	}
	current, found := s.sources[batchID][sourceID]
	if !found || current.Phase != expectedPhase || current.PhaseGeneration != expectedGeneration || current.LeaseID != expectedLeaseID {
		return nil, false, nil
	}
	if next.PhaseGeneration != current.PhaseGeneration+1 {
		return nil, false, ErrBrowserBatchWorkflowInvalid
	}
	s.sources[batchID][sourceID] = cloneBrowserBatchSourceWorkflow(next)
	copy := cloneBrowserBatchSourceWorkflow(next)
	return &copy, true, nil
}

func (s *browserBatchWorkflowMemoryStore) AcquireNextBrowserBatchLease(_ context.Context, ownerID, batchID, requiredPhase, leaseID string, now, expiresAt time.Time) (*BrowserBatchSourceWorkflow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return nil, false, ErrBrowserBatchWorkflowNotFound
	}
	if requiredPhase == BrowserBatchPhaseTransferConfirmationRequired && workflow.TransferManifestSHA256 == "" {
		return nil, false, ErrBrowserBatchWorkflowNotReady
	}
	all := orderedBrowserBatchSources(s.sources[batchID])
	for _, source := range all {
		if (source.Phase == BrowserBatchPhaseDiscoveryRunning || source.Phase == BrowserBatchPhaseCaptureRunning) && source.LeaseExpiresAt != nil && source.LeaseExpiresAt.After(now) {
			return nil, false, nil
		}
	}
	for _, source := range all {
		if source.Phase != requiredPhase || source.LeaseID != "" {
			continue
		}
		next := source
		if requiredPhase == BrowserBatchPhaseDiscoveryRequired {
			next.Phase = BrowserBatchPhaseDiscoveryRunning
		} else if requiredPhase == BrowserBatchPhaseTransferConfirmationRequired {
			next.Phase = BrowserBatchPhaseCaptureRunning
			if next.CaptureRunID == "" {
				next.CaptureRunID = leaseID
			}
		} else {
			return nil, false, ErrBrowserBatchWorkflowInvalid
		}
		next.PhaseGeneration++
		next.AttemptCount++
		next.LeaseID = leaseID
		expires := expiresAt.UTC()
		next.LeaseExpiresAt = &expires
		next.UpdatedAt = now.UTC()
		s.sources[batchID][source.SourceCompanyID] = next
		copy := cloneBrowserBatchSourceWorkflow(next)
		return &copy, true, nil
	}
	return nil, false, nil
}

func (s *browserBatchWorkflowMemoryStore) RecoverExpiredBrowserBatchLeases(_ context.Context, ownerID, batchID string, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return 0, ErrBrowserBatchWorkflowNotFound
	}
	recovered := 0
	for sourceID, source := range s.sources[batchID] {
		if (source.Phase != BrowserBatchPhaseDiscoveryRunning && source.Phase != BrowserBatchPhaseCaptureRunning) || source.LeaseExpiresAt == nil || source.LeaseExpiresAt.After(now) {
			continue
		}
		if source.Phase == BrowserBatchPhaseDiscoveryRunning {
			source.Phase = BrowserBatchPhaseDiscoveryRequired
		} else {
			source.Phase = BrowserBatchPhaseTransferConfirmationRequired
		}
		source.PhaseGeneration++
		source.LeaseID, source.LeaseExpiresAt = "", nil
		source.ReasonCode, source.UpdatedAt = "lease_expired", now.UTC()
		s.sources[batchID][sourceID] = source
		recovered++
	}
	return recovered, nil
}

func (s *browserBatchWorkflowMemoryStore) OpenBrowserBatchTransferConfirmation(_ context.Context, ownerID, batchID, expectedSchemaSHA256 string, updatedAt time.Time) (*BrowserBatchWorkflow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return nil, false, ErrBrowserBatchWorkflowNotFound
	}
	sources := orderedBrowserBatchSources(s.sources[batchID])
	actual, ok := browserBatchSchemaReadinessSHA256(sources)
	if !ok || actual != expectedSchemaSHA256 {
		return nil, false, ErrBrowserBatchWorkflowNotReady
	}
	alreadyOpen := true
	for _, source := range sources {
		if source.Phase != BrowserBatchPhaseTransferConfirmationRequired || source.LeaseID != "" {
			alreadyOpen = false
			break
		}
	}
	if alreadyOpen {
		copy := cloneBrowserBatchWorkflow(workflow)
		return &copy, false, nil
	}
	for _, source := range sources {
		if source.Phase != BrowserBatchPhaseSchemaApproved {
			return nil, false, ErrBrowserBatchWorkflowNotReady
		}
		source.Phase, source.PhaseGeneration, source.UpdatedAt = BrowserBatchPhaseTransferConfirmationRequired, source.PhaseGeneration+1, updatedAt.UTC()
		s.sources[batchID][source.SourceCompanyID] = source
	}
	copy := cloneBrowserBatchWorkflow(workflow)
	return &copy, true, nil
}

func (s *browserBatchWorkflowMemoryStore) ConfirmBrowserBatchTransfer(_ context.Context, ownerID, batchID, manifestSHA256 string, scope BrowserBatchTransferScope, confirmedAt time.Time) (*BrowserBatchWorkflow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	workflow, found := s.workflows[batchID]
	if !found || workflow.OwnerID != ownerID {
		return nil, false, ErrBrowserBatchWorkflowNotFound
	}
	if workflow.TransferManifestSHA256 != "" {
		if workflow.TransferManifestSHA256 != manifestSHA256 {
			return nil, false, ErrBrowserBatchWorkflowConflict
		}
		copy := cloneBrowserBatchWorkflow(workflow)
		return &copy, false, nil
	}
	for _, source := range s.sources[batchID] {
		if source.Phase != BrowserBatchPhaseTransferConfirmationRequired || source.LeaseID != "" {
			return nil, false, ErrBrowserBatchWorkflowNotReady
		}
	}
	at := confirmedAt.UTC()
	workflow.TransferManifestSHA256, workflow.TransferScope, workflow.TransferConfirmedAt, workflow.UpdatedAt = manifestSHA256, scope, &at, at
	s.workflows[batchID] = workflow
	copy := cloneBrowserBatchWorkflow(workflow)
	return &copy, true, nil
}

func orderedBrowserBatchSources(input map[string]BrowserBatchSourceWorkflow) []BrowserBatchSourceWorkflow {
	result := make([]BrowserBatchSourceWorkflow, 0, len(input))
	for _, source := range input {
		result = append(result, cloneBrowserBatchSourceWorkflow(source))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ordinal < result[j].Ordinal })
	return result
}

func newBrowserBatchWorkflowFixture(t *testing.T) (*BrowserBatchWorkflowService, *browserBatchWorkflowMemoryStore, time.Time) {
	t.Helper()
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	sources := []BrowserOnboardingSource{{SourceCompanyID: batchSourceOne, SourceCompanyName: "Alpha OÜ"}, {SourceCompanyID: batchSourceTwo, SourceCompanyName: "Beta OÜ"}}
	observed := []string{batchSourceOne, batchSourceTwo}
	observedDigest := strings.Repeat("a", 64)
	manifest, err := browserOnboardingBatchDigest(struct {
		Version           string                    `json:"version"`
		CatalogReceiptID  string                    `json:"catalog_receipt_id"`
		CatalogSHA256     string                    `json:"catalog_sha256"`
		RelayObservedAt   time.Time                 `json:"relay_observed_at"`
		Mode              string                    `json:"mode"`
		SelectedSources   []BrowserOnboardingSource `json:"selected_sources"`
		ObservedSourceIDs []string                  `json:"observed_source_ids"`
	}{browserOnboardingBatchManifestVersion, batchWorkflowCatalogID, observedDigest, now, BrowserOnboardingBatchModeAll, sources, observed})
	require.NoError(t, err)
	batch := BrowserOnboardingBatch{ID: batchWorkflowID, OwnerID: "owner-1", CatalogReceiptID: batchWorkflowCatalogID, RelayObservedAt: now, Mode: BrowserOnboardingBatchModeAll, SelectedSources: sources, ObservedSourceIDs: observed, ObservedSourcesSHA256: observedDigest, ManifestSHA256: manifest, Status: BrowserOnboardingBatchReady, CreatedAt: now, UpdatedAt: now}
	outcomes := []BrowserOnboardingBatchOutcome{
		{SourceCompanyID: batchSourceOne, SourceCompanyName: "Alpha OÜ", TenantID: batchTenantOne, TenantName: "Alpha OÜ", PairingID: batchPairingOne, Status: BrowserOnboardingPaired, CreatedAt: now, UpdatedAt: now},
		{SourceCompanyID: batchSourceTwo, SourceCompanyName: "Beta OÜ", TenantID: batchTenantTwo, TenantName: "Beta OÜ", PairingID: batchPairingTwo, Status: BrowserOnboardingPaired, CreatedAt: now, UpdatedAt: now},
	}
	store := &browserBatchWorkflowMemoryStore{}
	service := NewBrowserBatchWorkflowService(store, browserBatchWorkflowOnboardingReader{batch: batch, outcomes: outcomes})
	service.now = func() time.Time { return now }
	return service, store, now
}

func prepareBrowserBatchWorkflow(t *testing.T, service *BrowserBatchWorkflowService) *BrowserBatchWorkflowStatus {
	t.Helper()
	status, err := service.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true, HeaderProbeConsentConfirmed: true})
	require.NoError(t, err)
	return status
}

func approveAllBrowserBatchSchemas(t *testing.T, service *BrowserBatchWorkflowService) *BrowserBatchWorkflowStatus {
	t.Helper()
	for index := 0; ; index++ {
		claimed, err := service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
		if errors.Is(err, ErrBrowserBatchWorkflowNotReady) {
			break
		}
		require.NoError(t, err)
		completion := BrowserBatchDiscoveryCompletion{LeaseID: claimed.LeaseID, PhaseGeneration: claimed.PhaseGeneration, DiscoveryID: uuid.NewString(), DiscoveryContractSHA256: strings.Repeat(string(rune('a'+index)), 64), DiscoveryReceiptSHA256: strings.Repeat(string(rune('c'+index)), 64)}
		completed, err := service.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, completion)
		require.NoError(t, err)
		review, err := service.RequireSchemaReview(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, completed.PhaseGeneration)
		require.NoError(t, err)
		_, err = service.RecordSchemaApproval(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, BrowserBatchSchemaApproval{PhaseGeneration: review.PhaseGeneration, SchemaID: BrowserGeneralLedgerCSVSchemaID, SchemaApprovalSHA256: strings.Repeat(string(rune('e'+index)), 64)})
		require.NoError(t, err)
	}
	status, err := service.Status(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	return status
}

func TestBrowserBatchWorkflowStrictSerialSchemaAndTransferFlow(t *testing.T) {
	service, _, _ := newBrowserBatchWorkflowFixture(t)
	prepared := prepareBrowserBatchWorkflow(t, service)
	require.Equal(t, BrowserBatchPhaseDiscoveryRequired, prepared.Status)
	require.Empty(t, prepared.SchemaReadinessSHA256)

	first, err := service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	_, err = service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowNotReady, "the second source cannot run while the first lease is live")

	completed, err := service.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, first.SourceCompanyID, BrowserBatchDiscoveryCompletion{LeaseID: first.LeaseID, PhaseGeneration: first.PhaseGeneration, DiscoveryID: uuid.NewString(), DiscoveryContractSHA256: strings.Repeat("a", 64), DiscoveryReceiptSHA256: strings.Repeat("b", 64)})
	require.NoError(t, err)
	review, err := service.RequireSchemaReview(context.Background(), "owner-1", batchWorkflowID, first.SourceCompanyID, completed.PhaseGeneration)
	require.NoError(t, err)
	_, err = service.RecordSchemaApproval(context.Background(), "owner-1", batchWorkflowID, first.SourceCompanyID, BrowserBatchSchemaApproval{PhaseGeneration: review.PhaseGeneration, SchemaID: BrowserGeneralLedgerCSVSchemaID, SchemaApprovalSHA256: strings.Repeat("c", 64)})
	require.NoError(t, err)

	_, err = service.OpenTransferConfirmation(context.Background(), "owner-1", batchWorkflowID)
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowNotReady, "all original selected sources must be approved")
	ready := approveAllBrowserBatchSchemas(t, service)
	require.Equal(t, BrowserBatchPhaseSchemaApproved, ready.Status)
	require.True(t, validSHA256(ready.SchemaReadinessSHA256))

	opened, err := service.OpenTransferConfirmation(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, BrowserBatchPhaseTransferConfirmationRequired, opened.Status)
	require.Equal(t, ready.SchemaReadinessSHA256, opened.SchemaReadinessSHA256)
	reopened, err := service.OpenTransferConfirmation(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, opened.SchemaReadinessSHA256, reopened.SchemaReadinessSHA256)

	confirmed, err := service.ConfirmTransfer(context.Background(), "owner-1", batchWorkflowID, BrowserBatchTransferConfirmationRequest{OwnerConfirmed: true, ExpectedSchemaSHA256: opened.SchemaReadinessSHA256})
	require.NoError(t, err)
	require.True(t, validSHA256(confirmed.Workflow.TransferManifestSHA256))
	replay, err := service.ConfirmTransfer(context.Background(), "owner-1", batchWorkflowID, BrowserBatchTransferConfirmationRequest{OwnerConfirmed: true, ExpectedSchemaSHA256: opened.SchemaReadinessSHA256})
	require.NoError(t, err)
	require.Equal(t, confirmed.Workflow.TransferManifestSHA256, replay.Workflow.TransferManifestSHA256)
	require.Equal(t, confirmed.Workflow.TransferScope, replay.Workflow.TransferScope)
	_, err = service.ConfirmTransfer(context.Background(), "owner-1", batchWorkflowID, BrowserBatchTransferConfirmationRequest{OwnerConfirmed: true, ExpectedSchemaSHA256: strings.Repeat("d", 64)})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowConflict)
}

func TestBrowserBatchWorkflowRejectsSummarySchemaAndCannotPromoteHistoricalBinding(t *testing.T) {
	service, _, _ := newBrowserBatchWorkflowFixture(t)
	prepareBrowserBatchWorkflow(t, service)
	claimed, err := service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	completed, err := service.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, BrowserBatchDiscoveryCompletion{LeaseID: claimed.LeaseID, PhaseGeneration: claimed.PhaseGeneration, DiscoveryID: uuid.NewString(), DiscoveryContractSHA256: strings.Repeat("a", 64), DiscoveryReceiptSHA256: strings.Repeat("b", 64)})
	require.NoError(t, err)
	review, err := service.RequireSchemaReview(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, completed.PhaseGeneration)
	require.NoError(t, err)
	_, err = service.RecordSchemaApproval(context.Background(), "owner-1", batchWorkflowID, claimed.SourceCompanyID, BrowserBatchSchemaApproval{PhaseGeneration: review.PhaseGeneration, SchemaID: "journal_entries_csv_v1", SchemaApprovalSHA256: strings.Repeat("c", 64)})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowInvalid)

	// A historical v1 row may remain readable as evidence, but its old schema
	// cannot generate the readiness digest that opens a v2 transfer.
	historical := BrowserBatchSourceWorkflow{
		BatchID: batchWorkflowID, SourceCompanyID: batchSourceOne, TenantID: batchTenantOne, Ordinal: 0,
		Phase: BrowserBatchPhaseSchemaApproved, PhaseGeneration: 1, DiscoveryID: uuid.NewString(),
		DiscoveryContractSHA256: strings.Repeat("a", 64), DiscoveryReceiptSHA256: strings.Repeat("b", 64),
		SchemaID: "journal_entries_csv_v1", SchemaApprovalSHA256: strings.Repeat("c", 64),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	assert.True(t, validBrowserBatchSourceWorkflow(historical), "status remains readable as historical evidence")
	_, ok := browserBatchSchemaReadinessSHA256([]BrowserBatchSourceWorkflow{historical})
	assert.False(t, ok, "summary-grid history cannot be promoted to a v2 transfer")
}

func TestBrowserBatchWorkflowExpiredLeaseReclaimsAndRejectsStaleCompletion(t *testing.T) {
	service, _, now := newBrowserBatchWorkflowFixture(t)
	prepareBrowserBatchWorkflow(t, service)
	first, err := service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	service.now = func() time.Time { return now.Add(browserBatchWorkflowLeaseLifetime + time.Second) }
	reclaimed, err := service.ClaimNextDiscovery(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, first.SourceCompanyID, reclaimed.SourceCompanyID)
	require.NotEqual(t, first.LeaseID, reclaimed.LeaseID)
	require.Greater(t, reclaimed.PhaseGeneration, first.PhaseGeneration)

	_, err = service.CompleteDiscovery(context.Background(), "owner-1", batchWorkflowID, first.SourceCompanyID, BrowserBatchDiscoveryCompletion{LeaseID: first.LeaseID, PhaseGeneration: first.PhaseGeneration, DiscoveryID: uuid.NewString(), DiscoveryContractSHA256: strings.Repeat("a", 64), DiscoveryReceiptSHA256: strings.Repeat("b", 64)})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowConflict)
}

func TestBrowserBatchWorkflowExpiredCaptureLeaseReclaimsAndRejectsStaleStage(t *testing.T) {
	service, _, now := newBrowserBatchWorkflowFixture(t)
	prepareBrowserBatchWorkflow(t, service)
	ready := approveAllBrowserBatchSchemas(t, service)
	_, err := service.OpenTransferConfirmation(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	_, err = service.ConfirmTransfer(context.Background(), "owner-1", batchWorkflowID, BrowserBatchTransferConfirmationRequest{OwnerConfirmed: true, ExpectedSchemaSHA256: ready.SchemaReadinessSHA256})
	require.NoError(t, err)
	first, err := service.ClaimNextCapture(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	service.now = func() time.Time { return now.Add(browserBatchWorkflowLeaseLifetime + time.Second) }
	reclaimed, err := service.ClaimNextCapture(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	require.Equal(t, first.SourceCompanyID, reclaimed.SourceCompanyID)
	require.NotEqual(t, first.LeaseID, reclaimed.LeaseID)

	_, err = service.RecordStagedPackage(context.Background(), "owner-1", batchWorkflowID, first.SourceCompanyID, BrowserBatchStagedPackage{LeaseID: first.LeaseID, PhaseGeneration: first.PhaseGeneration, PackageID: "sa-browser-package", PackageSHA256: strings.Repeat("a", 64)})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowConflict)
}

func TestBrowserBatchWorkflowAggregateStatusIsMonotonic(t *testing.T) {
	base := BrowserBatchSourceWorkflow{BatchID: batchWorkflowID, TenantID: batchTenantOne, SourceCompanyID: batchSourceOne}
	second := base
	second.SourceCompanyID, second.TenantID, second.Ordinal = batchSourceTwo, batchTenantTwo, 1
	tests := []struct {
		name   string
		phases []string
		want   string
	}{
		{"paired", []string{BrowserBatchPhasePaired, BrowserBatchPhasePaired}, BrowserBatchPhasePaired},
		{"discovery complete", []string{BrowserBatchPhaseDiscoveryComplete, BrowserBatchPhaseDiscoveryComplete}, BrowserBatchPhaseDiscoveryComplete},
		{"schema approved", []string{BrowserBatchPhaseSchemaApproved, BrowserBatchPhaseSchemaApproved}, BrowserBatchPhaseSchemaApproved},
		{"mixed discovery uses least progress", []string{BrowserBatchPhaseDiscoveryRequired, BrowserBatchPhaseDiscoveryComplete}, BrowserBatchPhaseDiscoveryRequired},
		{"capture active", []string{BrowserBatchPhaseCaptureRunning, BrowserBatchPhaseTransferConfirmationRequired}, BrowserBatchPhaseCaptureRunning},
		{"mixed staged preview is staged", []string{BrowserBatchPhaseStaged, BrowserBatchPhasePreviewReady}, BrowserBatchPhaseStaged},
		{"retryable failure is visible", []string{BrowserBatchPhaseFailedRetryable, BrowserBatchPhaseSchemaApproved}, BrowserBatchPhaseFailedRetryable},
		{"review masks progress", []string{BrowserBatchPhaseReviewRequired, BrowserBatchPhasePreviewReady}, BrowserBatchPhaseReviewRequired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, right := base, second
			left.Phase, right.Phase = test.phases[0], test.phases[1]
			assert.Equal(t, test.want, browserBatchWorkflowStatus([]BrowserBatchSourceWorkflow{left, right}))
		})
	}
}

func TestBrowserBatchWorkflowMigrationKeepsOnlyControlState(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "082_smartaccounts_browser_batch_workflows.up.sql")
	text, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(text), "CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_batch_workflows")
	assert.Contains(t, string(text), "CREATE TABLE IF NOT EXISTS public.smartaccounts_browser_batch_source_workflows")
	for _, expected := range []string{"smartaccounts_browser_batch_workflows", "smartaccounts_browser_batch_source_workflows", "phase_generation", "lease_id", "transfer_manifest_sha256", "PREVIEW_READY", "phase_proof_check"} {
		assert.Contains(t, string(text), expected)
	}
	for _, forbidden := range []string{"pairing_token", "capture_token", "browser_cookie", "csv_body", "header_names", "journal_apply"} {
		assert.NotContains(t, string(text), forbidden)
	}
	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "082_smartaccounts_browser_batch_workflows.down.sql"))
	require.NoError(t, err)
	assert.Contains(t, string(down), "DROP TABLE IF EXISTS public.smartaccounts_browser_batch_source_workflows")
	assert.Contains(t, string(down), "DROP TABLE IF EXISTS public.smartaccounts_browser_batch_workflows")
}

func TestBrowserBatchSourceWorkflowRejectsMissingPhaseProofs(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	fixture := func(phase string) BrowserBatchSourceWorkflow {
		source := BrowserBatchSourceWorkflow{BatchID: batchWorkflowID, SourceCompanyID: batchSourceOne, TenantID: batchTenantOne, Phase: phase, CreatedAt: now, UpdatedAt: now}
		if browserBatchPhaseRequiresDiscovery(phase) {
			source.DiscoveryID, source.DiscoveryContractSHA256, source.DiscoveryReceiptSHA256 = uuid.NewString(), strings.Repeat("a", 64), strings.Repeat("b", 64)
		}
		if browserBatchPhaseRequiresSchema(phase) {
			source.SchemaID, source.SchemaApprovalSHA256 = BrowserGeneralLedgerCSVSchemaID, strings.Repeat("c", 64)
		}
		if browserBatchPhaseRequiresPackage(phase) {
			source.PackageID, source.PackageSHA256 = "sa-browser-package", strings.Repeat("d", 64)
		}
		if phase == BrowserBatchPhasePreviewReady {
			source.PreviewID, source.PreviewSHA256 = uuid.NewString(), strings.Repeat("e", 64)
		}
		if phase == BrowserBatchPhaseDiscoveryRunning || phase == BrowserBatchPhaseCaptureRunning {
			expires := now.Add(time.Minute)
			source.LeaseID, source.LeaseExpiresAt = uuid.NewString(), &expires
		}
		return source
	}
	tests := []struct {
		name   string
		source BrowserBatchSourceWorkflow
	}{
		{"discovery running without lease", func() BrowserBatchSourceWorkflow {
			source := fixture(BrowserBatchPhaseDiscoveryRunning)
			source.LeaseID, source.LeaseExpiresAt = "", nil
			return source
		}()},
		{"discovery complete without receipt", func() BrowserBatchSourceWorkflow {
			source := fixture(BrowserBatchPhaseDiscoveryComplete)
			source.DiscoveryID, source.DiscoveryContractSHA256, source.DiscoveryReceiptSHA256 = "", "", ""
			return source
		}()},
		{"schema approved without approval", func() BrowserBatchSourceWorkflow {
			source := fixture(BrowserBatchPhaseSchemaApproved)
			source.SchemaID, source.SchemaApprovalSHA256 = "", ""
			return source
		}()},
		{"staged without package", func() BrowserBatchSourceWorkflow {
			source := fixture(BrowserBatchPhaseStaged)
			source.PackageID, source.PackageSHA256 = "", ""
			return source
		}()},
		{"preview without preview receipt", func() BrowserBatchSourceWorkflow {
			source := fixture(BrowserBatchPhasePreviewReady)
			source.PreviewID, source.PreviewSHA256 = "", ""
			return source
		}()},
		{"review retains active lease", func() BrowserBatchSourceWorkflow {
			source := fixture(BrowserBatchPhaseReviewRequired)
			expires := now.Add(time.Minute)
			source.LeaseID, source.LeaseExpiresAt = uuid.NewString(), &expires
			return source
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { assert.False(t, validBrowserBatchSourceWorkflow(test.source)) })
	}
}

func TestBrowserBatchSchemaReadinessIsStableAfterLaterPhases(t *testing.T) {
	service, _, _ := newBrowserBatchWorkflowFixture(t)
	prepareBrowserBatchWorkflow(t, service)
	ready := approveAllBrowserBatchSchemas(t, service)
	first := ready.SchemaReadinessSHA256
	_, err := service.OpenTransferConfirmation(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	after, err := service.Status(context.Background(), "owner-1", batchWorkflowID)
	require.NoError(t, err)
	assert.Equal(t, first, after.SchemaReadinessSHA256)
}

func TestBrowserBatchWorkflowPrepareRejectsUnpairedSource(t *testing.T) {
	service, _, _ := newBrowserBatchWorkflowFixture(t)
	reader := service.onboarding.(browserBatchWorkflowOnboardingReader)
	reader.outcomes[1].Status = BrowserOnboardingPairingIssued
	service.onboarding = reader
	_, err := service.Prepare(context.Background(), "owner-1", batchWorkflowID, BrowserBatchPreparationRequest{HistoryFrom: "2020-01-01", OwnerConfirmed: true, MetadataDiscoveryConsentConfirmed: true, HeaderProbeConsentConfirmed: true})
	require.ErrorIs(t, err, ErrBrowserBatchWorkflowNotReady)
}

func TestBrowserBatchWorkflowNoCapabilitiesInStatus(t *testing.T) {
	service, _, _ := newBrowserBatchWorkflowFixture(t)
	status := prepareBrowserBatchWorkflow(t, service)
	encoded := []byte(status.Workflow.PreparatoryManifestSHA256 + status.SchemaReadinessSHA256)
	assert.NotContains(t, string(encoded), "Bearer")
	assert.False(t, errors.Is(ErrBrowserBatchWorkflowNotReady, ErrBrowserBatchWorkflowConflict))
}
