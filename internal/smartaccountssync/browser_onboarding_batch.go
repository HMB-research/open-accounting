package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BrowserOnboardingBatchModeSelected = "selected"
	BrowserOnboardingBatchModeAll      = "all"

	BrowserOnboardingBatchPending = "PENDING"
	BrowserOnboardingBatchReview  = "REVIEW_REQUIRED"
	BrowserOnboardingBatchReady   = "READY"
	// BrowserOnboardingBatchComplete is reserved for a later, separately
	// confirmed workflow. This batch service never promotes a batch to COMPLETE
	// merely because it created targets or issued pairings.
	BrowserOnboardingBatchComplete = "COMPLETE"

	browserOnboardingBatchManifestVersion = "smartaccounts-browser-onboarding-batch-v1"
	browserOnboardingCatalogMaxLifetime   = 10 * time.Minute
)

var (
	ErrBrowserOnboardingBatchInvalid     = errors.New("SmartAccounts browser onboarding batch is invalid")
	ErrBrowserOnboardingBatchConflict    = errors.New("SmartAccounts browser onboarding batch conflicts with its immutable manifest")
	ErrBrowserOnboardingBatchNotFound    = errors.New("SmartAccounts browser onboarding batch was not found for this owner")
	ErrBrowserOnboardingBatchUnavailable = errors.New("SmartAccounts browser onboarding batch is unavailable")
)

// BrowserOnboardingCatalogReceipt is the server-issued, short-lived record of
// the relay's current visible company picker. It contains opaque selectors and
// display names only. It is not a SmartAccounts API catalog certification; the
// selected source still has to claim its expected-source pairing before any
// target binding can be ready.
type BrowserOnboardingCatalogReceipt struct {
	ID               string
	WorkflowID       string
	OwnerID          string
	TokenSHA256      string
	NonceSHA256      string
	SchemaVersion    string
	IntentVersion    string
	SourceIDVersion  string
	DigestAlgorithm  string
	Status           string
	CatalogSHA256    string
	CatalogCount     int
	Sources          []BrowserOnboardingSource
	ObservedAt       time.Time
	ExpiresAt        time.Time
	ReceiptExpiresAt time.Time
	AcceptedAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// BrowserOnboardingSourceCatalog resolves only a fresh server-issued relay
// receipt. It is deliberately not a browser request payload, so an operator
// cannot turn "all" into an implicit or caller-supplied source set.
type BrowserOnboardingSourceCatalog interface {
	GetBrowserOnboardingCatalogReceipt(context.Context, string, string) (*BrowserOnboardingCatalogReceipt, error)
}

// BrowserOnboardingBatchRunner is implemented by BrowserOnboardingService.
// Splitting it out keeps immutable batch accounting free of HTTP and lets the
// existing source→tenant/pairing safeguards remain the sole target creator.
type BrowserOnboardingBatchRunner interface {
	Start(context.Context, string, BrowserOnboardingRequest) (*BrowserOnboardingResponse, error)
	Status(context.Context, string, string) (*BrowserOnboardingResult, error)
}

// BrowserOnboardingBatchRequest contains source selectors only. An owner has
// to explicitly submit the same selected set even for mode=all; the service
// compares it with the trusted observed catalog before saving anything.
type BrowserOnboardingBatchRequest struct {
	CatalogReceiptID  string   `json:"catalog_receipt_id"`
	Mode              string   `json:"mode"`
	SelectedSourceIDs []string `json:"selected_source_ids"`
	OwnerConfirmed    bool     `json:"owner_confirmed"`
}

// BrowserOnboardingBatchResumeRequest requires a fresh explicit owner action
// before the server returns another action-response-only pairing token for an
// existing immutable batch. It has no source selection or catalog fields.
type BrowserOnboardingBatchResumeRequest struct {
	OwnerConfirmed bool `json:"owner_confirmed"`
}

// BrowserOnboardingBatch is an immutable selection manifest plus aggregate
// safe progress. SelectedSources is server-derived display metadata; the
// observed catalog field contains opaque source IDs only.
type BrowserOnboardingBatch struct {
	ID                    string                    `json:"batch_id"`
	OwnerID               string                    `json:"-"`
	CatalogReceiptID      string                    `json:"catalog_receipt_id"`
	RelayObservedAt       time.Time                 `json:"relay_observed_at"`
	Mode                  string                    `json:"mode"`
	SelectedSources       []BrowserOnboardingSource `json:"selected_sources"`
	ObservedSourceIDs     []string                  `json:"observed_source_ids"`
	ObservedSourcesSHA256 string                    `json:"observed_sources_sha256"`
	ManifestSHA256        string                    `json:"manifest_sha256"`
	Status                string                    `json:"status"`
	CreatedAt             time.Time                 `json:"created_at"`
	UpdatedAt             time.Time                 `json:"updated_at"`
}

// BrowserOnboardingBatchOutcome persists each selected source independently.
// A failure remains visible without preventing other selected sources from
// reaching a claimed pairing, but it keeps the aggregate batch review-only.
type BrowserOnboardingBatchOutcome struct {
	SourceCompanyID   string    `json:"source_company_id"`
	SourceCompanyName string    `json:"source_company_name"`
	TenantID          string    `json:"tenant_id,omitempty"`
	TenantName        string    `json:"tenant_name,omitempty"`
	PairingID         string    `json:"pairing_id,omitempty"`
	Status            string    `json:"status"`
	TenantCreated     bool      `json:"tenant_created"`
	TenantReused      bool      `json:"tenant_reused"`
	ReasonCode        string    `json:"reason_code,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type BrowserOnboardingBatchResponse struct {
	Batch    BrowserOnboardingBatch          `json:"batch"`
	Outcomes []BrowserOnboardingBatchOutcome `json:"outcomes"`
	// PairingIssues is action-response-only. It is deliberately omitted from
	// every store, status call, log, and retry record: the raw pairing token is
	// passed straight to relay memory and can be reissued only by a new owner
	// confirmed Start request for the exact same immutable batch.
	PairingIssues []BrowserOnboardingBatchPairingIssue `json:"pairing_issues,omitempty"`
	Reused        bool                                 `json:"reused"`
}

// BrowserOnboardingBatchPairingIssue binds an otherwise short-lived pairing
// token to the immutable batch, its one opaque selected source, and exactly
// one OA tenant. The relay must enforce all three bindings before claiming.
type BrowserOnboardingBatchPairingIssue struct {
	BatchID         string              `json:"batch_id"`
	SourceCompanyID string              `json:"source_company_id"`
	TenantID        string              `json:"tenant_id"`
	Pairing         BrowserPairingIssue `json:"pairing"`
}

// BrowserOnboardingBatchStore uses an owner+fresh-catalog-receipt key. This
// means an exact retry returns the same immutable manifest, while an attempt
// to alter membership, a display name, or the manifest digest for that same
// receipt conflicts. A later fresh receipt may create a later batch even when
// the visible opaque source set is unchanged.
type BrowserOnboardingBatchStore interface {
	// WithBrowserOnboardingBatchStartLock serializes the entire initial
	// create/retry/start/progress sequence for one owner and one immutable
	// catalog receipt. Implementations must not persist capabilities inside the
	// callback. A transaction-scoped lock is sufficient because the callback
	// saves the durable outcomes before releasing it.
	WithBrowserOnboardingBatchStartLock(context.Context, string, string, func() error) error
	FindBrowserOnboardingBatchByCatalogReceipt(context.Context, string, string) (*BrowserOnboardingBatch, error)
	GetBrowserOnboardingBatch(context.Context, string, string) (*BrowserOnboardingBatch, error)
	CreateBrowserOnboardingBatch(context.Context, BrowserOnboardingBatch) (*BrowserOnboardingBatch, bool, error)
	SaveBrowserOnboardingBatchProgress(context.Context, string, string, string, []BrowserOnboardingBatchOutcome, time.Time) (*BrowserOnboardingBatch, error)
	ListBrowserOnboardingBatchOutcomes(context.Context, string, string) ([]BrowserOnboardingBatchOutcome, error)
}

// BrowserOnboardingBatchService creates or resumes a strictly bounded group
// of selected opaque sources. It has no accounting service, bridge client, or
// financial apply dependency.
type BrowserOnboardingBatchService struct {
	store   BrowserOnboardingBatchStore
	catalog BrowserOnboardingSourceCatalog
	runner  BrowserOnboardingBatchRunner
	now     func() time.Time
	newID   func() string
}

func NewBrowserOnboardingBatchService(store BrowserOnboardingBatchStore, catalog BrowserOnboardingSourceCatalog, runner BrowserOnboardingBatchRunner) *BrowserOnboardingBatchService {
	return &BrowserOnboardingBatchService{store: store, catalog: catalog, runner: runner, now: time.Now, newID: uuid.NewString}
}

// Start creates a manifest only after explicit owner confirmation. A newly
// created batch invokes the existing per-source onboarding service once; an
// exact retry refreshes durable pairing state without creating a second batch.
func (s *BrowserOnboardingBatchService) Start(ctx context.Context, actorID string, request BrowserOnboardingBatchRequest) (*BrowserOnboardingBatchResponse, error) {
	if s == nil || s.store == nil || s.catalog == nil || s.runner == nil || s.newID == nil || !request.OwnerConfirmed || strings.TrimSpace(actorID) == "" {
		return nil, ErrBrowserOnboardingBatchInvalid
	}
	actorID = strings.TrimSpace(actorID)
	receipt, err := s.catalog.GetBrowserOnboardingCatalogReceipt(ctx, actorID, strings.TrimSpace(request.CatalogReceiptID))
	if err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	if !validBrowserOnboardingCatalogReceipt(receipt, actorID, s.currentTime()) {
		return nil, ErrBrowserOnboardingBatchInvalid
	}
	batch, err := newBrowserOnboardingBatch(s.newID(), actorID, request, *receipt, s.currentTime())
	if err != nil {
		return nil, err
	}

	var response *BrowserOnboardingBatchResponse
	err = s.store.WithBrowserOnboardingBatchStartLock(ctx, actorID, batch.CatalogReceiptID, func() error {
		result, startErr := s.startLocked(ctx, actorID, batch)
		if startErr != nil {
			return startErr
		}
		response = result
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrBrowserOnboardingBatchInvalid), errors.Is(err, ErrBrowserOnboardingBatchConflict), errors.Is(err, ErrBrowserOnboardingBatchNotFound), errors.Is(err, ErrBrowserOnboardingBatchUnavailable):
			return nil, err
		default:
			return nil, ErrBrowserOnboardingBatchUnavailable
		}
	}
	if response == nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	return response, nil
}

// startLocked contains every branch which can call the per-source onboarding
// runner. It is deliberately invoked under the store's owner+catalog lock:
// once a manifest exists, an exact concurrent retry must observe the first
// durable outcomes rather than treating their short creation window as a
// second new batch start.
func (s *BrowserOnboardingBatchService) startLocked(ctx context.Context, actorID string, batch BrowserOnboardingBatch) (*BrowserOnboardingBatchResponse, error) {
	existing, err := s.store.FindBrowserOnboardingBatchByCatalogReceipt(ctx, actorID, batch.CatalogReceiptID)
	if err == nil && existing != nil {
		if !sameBrowserOnboardingBatchManifest(*existing, batch) {
			return nil, ErrBrowserOnboardingBatchConflict
		}
		return s.retry(ctx, *existing)
	}
	if err != nil && !errors.Is(err, ErrBrowserOnboardingBatchNotFound) {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}

	persisted, created, err := s.store.CreateBrowserOnboardingBatch(ctx, batch)
	if err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	if !created || persisted == nil {
		existing, err = s.store.FindBrowserOnboardingBatchByCatalogReceipt(ctx, actorID, batch.CatalogReceiptID)
		if err != nil || existing == nil {
			return nil, ErrBrowserOnboardingBatchUnavailable
		}
		if !sameBrowserOnboardingBatchManifest(*existing, batch) {
			return nil, ErrBrowserOnboardingBatchConflict
		}
		return s.retry(ctx, *existing)
	}
	if !sameBrowserOnboardingBatchManifest(*persisted, batch) {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}

	response, err := s.runner.Start(ctx, actorID, BrowserOnboardingRequest{Sources: batch.SelectedSources, CreateMissingTenantsConfirmed: true})
	outcomes := browserOnboardingBatchOutcomes(batch.SelectedSources, response, err, s.currentTime())
	pairings, pairingErr := browserOnboardingBatchPairingIssues(*persisted, response, s.currentTime())
	if pairingErr != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	return s.persistProgress(ctx, *persisted, outcomes, pairings, false)
}

// Resume returns fresh action-response pairing capabilities for the immutable
// existing batch after its short-lived catalog receipt has expired. It does
// not accept a replacement source set, change selection, or create a batch;
// exact source membership remains the already persisted manifest.
func (s *BrowserOnboardingBatchService) Resume(ctx context.Context, actorID, batchID string, request BrowserOnboardingBatchResumeRequest) (*BrowserOnboardingBatchResponse, error) {
	if s == nil || s.store == nil || s.runner == nil || !request.OwnerConfirmed || strings.TrimSpace(actorID) == "" || !validBrowserPairingID(strings.TrimSpace(batchID)) {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	batch, err := s.store.GetBrowserOnboardingBatch(ctx, strings.TrimSpace(actorID), strings.TrimSpace(batchID))
	if err != nil || batch == nil {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	return s.retry(ctx, *batch)
}

// Status is owner-scoped. It refreshes claimed expected-source pairings but
// never issues a token, starts capture, or invokes a financial action.
func (s *BrowserOnboardingBatchService) Status(ctx context.Context, actorID, batchID string) (*BrowserOnboardingBatchResponse, error) {
	if s == nil || s.store == nil || s.runner == nil || strings.TrimSpace(actorID) == "" || !validBrowserPairingID(strings.TrimSpace(batchID)) {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	batch, err := s.store.GetBrowserOnboardingBatch(ctx, strings.TrimSpace(actorID), strings.TrimSpace(batchID))
	if err != nil || batch == nil {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	return s.refresh(ctx, *batch, true)
}

// retry is deliberately capability-aware: a completed/claimed batch only
// refreshes safe status, while a missing, failed, target-ready, or unclaimed
// pairing gets one fresh action-response envelope from the existing runner.
// It never creates a second immutable batch or persists the raw token.
func (s *BrowserOnboardingBatchService) retry(ctx context.Context, batch BrowserOnboardingBatch) (*BrowserOnboardingBatchResponse, error) {
	current, err := s.store.ListBrowserOnboardingBatchOutcomes(ctx, batch.OwnerID, batch.ID)
	if err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	shouldIssue := len(current) == 0
	for _, outcome := range current {
		if outcome.Status == BrowserOnboardingTargetReady || outcome.Status == BrowserOnboardingPairingIssued || outcome.Status == BrowserOnboardingFailed {
			shouldIssue = true
			break
		}
	}
	if !shouldIssue {
		return s.refresh(ctx, batch, true)
	}
	response, err := s.runner.Start(ctx, batch.OwnerID, BrowserOnboardingRequest{Sources: batch.SelectedSources, CreateMissingTenantsConfirmed: true})
	if err != nil {
		return s.refresh(ctx, batch, true)
	}
	outcomes := browserOnboardingBatchOutcomes(batch.SelectedSources, response, nil, s.currentTime())
	pairings, err := browserOnboardingBatchPairingIssues(batch, response, s.currentTime())
	if err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	return s.persistProgress(ctx, batch, outcomes, pairings, true)
}

func (s *BrowserOnboardingBatchService) refresh(ctx context.Context, batch BrowserOnboardingBatch, reused bool) (*BrowserOnboardingBatchResponse, error) {
	current, err := s.store.ListBrowserOnboardingBatchOutcomes(ctx, batch.OwnerID, batch.ID)
	if err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	bySource := make(map[string]BrowserOnboardingBatchOutcome, len(current))
	for _, outcome := range current {
		bySource[outcome.SourceCompanyID] = outcome
	}
	updated := make([]BrowserOnboardingBatchOutcome, 0, len(batch.SelectedSources))
	now := s.currentTime()
	for _, source := range batch.SelectedSources {
		result, statusErr := s.runner.Status(ctx, batch.OwnerID, source.SourceCompanyID)
		if statusErr == nil && result != nil {
			updated = append(updated, batchOutcomeFromResult(source, *result, now))
			continue
		}
		if prior, found := bySource[source.SourceCompanyID]; found {
			updated = append(updated, prior)
			continue
		}
		updated = append(updated, BrowserOnboardingBatchOutcome{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: source.SourceCompanyName, Status: BrowserOnboardingFailed, ReasonCode: "onboarding_status_unavailable", CreatedAt: now, UpdatedAt: now})
	}
	return s.persistProgress(ctx, batch, updated, nil, reused)
}

func (s *BrowserOnboardingBatchService) persistProgress(ctx context.Context, batch BrowserOnboardingBatch, outcomes []BrowserOnboardingBatchOutcome, pairingIssues []BrowserOnboardingBatchPairingIssue, reused bool) (*BrowserOnboardingBatchResponse, error) {
	if !validBrowserOnboardingBatchOutcomes(batch, outcomes) {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	status := browserOnboardingBatchStatus(batch, outcomes)
	persisted, err := s.store.SaveBrowserOnboardingBatchProgress(ctx, batch.OwnerID, batch.ID, status, outcomes, s.currentTime())
	if err != nil || persisted == nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	return &BrowserOnboardingBatchResponse{Batch: *persisted, Outcomes: canonicalBrowserOnboardingBatchOutcomes(outcomes), PairingIssues: pairingIssues, Reused: reused}, nil
}

func newBrowserOnboardingBatch(id, actorID string, request BrowserOnboardingBatchRequest, receipt BrowserOnboardingCatalogReceipt, now time.Time) (BrowserOnboardingBatch, error) {
	if !validBrowserPairingID(strings.TrimSpace(id)) || strings.TrimSpace(actorID) == "" || !request.OwnerConfirmed || !validBrowserOnboardingCatalogReceipt(&receipt, actorID, now) {
		return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchInvalid
	}
	available, ok := canonicalBrowserOnboardingBatchSources(receipt.Sources)
	if !ok {
		return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchInvalid
	}
	selectedIDs, ok := canonicalBrowserOnboardingBatchSourceIDs(request.SelectedSourceIDs)
	if !ok {
		return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchInvalid
	}
	byID := make(map[string]BrowserOnboardingSource, len(available))
	observedIDs := make([]string, 0, len(available))
	for _, source := range available {
		byID[source.SourceCompanyID] = source
		observedIDs = append(observedIDs, source.SourceCompanyID)
	}
	selected := make([]BrowserOnboardingSource, 0, len(selectedIDs))
	for _, sourceID := range selectedIDs {
		source, found := byID[sourceID]
		if !found {
			return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchInvalid
		}
		selected = append(selected, source)
	}
	mode := strings.TrimSpace(request.Mode)
	if (mode == BrowserOnboardingBatchModeAll && !sameBrowserOnboardingSourceIDs(selectedIDs, observedIDs)) || (mode == BrowserOnboardingBatchModeSelected && (len(selectedIDs) == 0 || len(selectedIDs) >= len(observedIDs))) || (mode != BrowserOnboardingBatchModeSelected && mode != BrowserOnboardingBatchModeAll) {
		return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchInvalid
	}
	observedDigest := receipt.CatalogSHA256
	if !validSHA256(observedDigest) {
		return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchInvalid
	}
	manifestDigest, err := browserOnboardingBatchDigest(struct {
		Version           string                    `json:"version"`
		CatalogReceiptID  string                    `json:"catalog_receipt_id"`
		CatalogSHA256     string                    `json:"catalog_sha256"`
		RelayObservedAt   time.Time                 `json:"relay_observed_at"`
		Mode              string                    `json:"mode"`
		SelectedSources   []BrowserOnboardingSource `json:"selected_sources"`
		ObservedSourceIDs []string                  `json:"observed_source_ids"`
	}{Version: browserOnboardingBatchManifestVersion, CatalogReceiptID: receipt.ID, CatalogSHA256: observedDigest, RelayObservedAt: receipt.ObservedAt.UTC(), Mode: mode, SelectedSources: selected, ObservedSourceIDs: observedIDs})
	if err != nil {
		return BrowserOnboardingBatch{}, ErrBrowserOnboardingBatchUnavailable
	}
	return BrowserOnboardingBatch{ID: strings.TrimSpace(id), OwnerID: strings.TrimSpace(actorID), CatalogReceiptID: strings.TrimSpace(receipt.ID), RelayObservedAt: receipt.ObservedAt.UTC(), Mode: mode, SelectedSources: selected, ObservedSourceIDs: observedIDs, ObservedSourcesSHA256: observedDigest, ManifestSHA256: manifestDigest, Status: BrowserOnboardingBatchPending, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func browserOnboardingBatchOutcomes(sources []BrowserOnboardingSource, response *BrowserOnboardingResponse, startErr error, now time.Time) []BrowserOnboardingBatchOutcome {
	bySource := make(map[string]BrowserOnboardingResult, len(sources))
	if startErr == nil && response != nil {
		for _, result := range response.Bindings {
			if _, exists := bySource[result.SourceCompanyID]; !exists {
				bySource[result.SourceCompanyID] = result
			}
		}
	}
	outcomes := make([]BrowserOnboardingBatchOutcome, 0, len(sources))
	for _, source := range sources {
		result, found := bySource[source.SourceCompanyID]
		if !found {
			reason := "onboarding_start_unavailable"
			if startErr == nil {
				reason = "onboarding_result_missing"
			}
			outcomes = append(outcomes, BrowserOnboardingBatchOutcome{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: source.SourceCompanyName, Status: BrowserOnboardingFailed, ReasonCode: reason, CreatedAt: now, UpdatedAt: now})
			continue
		}
		outcomes = append(outcomes, batchOutcomeFromResult(source, result, now))
	}
	return outcomes
}

func browserOnboardingBatchPairingIssues(batch BrowserOnboardingBatch, response *BrowserOnboardingResponse, now time.Time) ([]BrowserOnboardingBatchPairingIssue, error) {
	if response == nil {
		return nil, nil
	}
	expected := make(map[string]struct{}, len(batch.SelectedSources))
	for _, source := range batch.SelectedSources {
		expected[source.SourceCompanyID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(response.Bindings))
	issues := make([]BrowserOnboardingBatchPairingIssue, 0)
	for _, result := range response.Bindings {
		if result.Pairing == nil {
			continue
		}
		if _, selected := expected[result.SourceCompanyID]; !selected || result.Status != BrowserOnboardingPairingIssued || result.TenantID == "" || !safeBridgeID(result.TenantID) || result.PairingID != result.Pairing.PairingID || !validBrowserPairingID(result.Pairing.PairingID) || !validBrowserPairingToken(result.Pairing.PairingToken) || !result.Pairing.ExpiresAt.After(now) {
			return nil, errors.New("invalid SmartAccounts browser onboarding pairing issue")
		}
		if _, duplicate := seen[result.SourceCompanyID]; duplicate {
			return nil, errors.New("duplicate SmartAccounts browser onboarding pairing issue")
		}
		seen[result.SourceCompanyID] = struct{}{}
		issues = append(issues, BrowserOnboardingBatchPairingIssue{BatchID: batch.ID, SourceCompanyID: result.SourceCompanyID, TenantID: result.TenantID, Pairing: *result.Pairing})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].SourceCompanyID < issues[j].SourceCompanyID })
	return issues, nil
}

func batchOutcomeFromResult(source BrowserOnboardingSource, result BrowserOnboardingResult, now time.Time) BrowserOnboardingBatchOutcome {
	status := strings.TrimSpace(result.Status)
	if !validBrowserOnboardingOutcomeStatus(status) {
		status = BrowserOnboardingFailed
	}
	name := strings.TrimSpace(result.SourceCompanyName)
	if name == "" || result.SourceCompanyID != source.SourceCompanyID {
		name = source.SourceCompanyName
	}
	tenantID := strings.TrimSpace(result.TenantID)
	tenantName := strings.TrimSpace(result.TenantName)
	pairingID := strings.TrimSpace(result.PairingID)
	// A runner must not be able to leak a target name/opaque pairing through a
	// source that is unresolved or bound to a different owner. Target metadata
	// is meaningful only together with a target ID.
	if tenantID == "" {
		tenantName, pairingID = "", ""
	}
	return BrowserOnboardingBatchOutcome{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: name, TenantID: tenantID, TenantName: tenantName, PairingID: pairingID, Status: status, TenantCreated: result.TenantCreated, TenantReused: result.TenantReused, ReasonCode: strings.TrimSpace(result.ReasonCode), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
}

func browserOnboardingBatchStatus(batch BrowserOnboardingBatch, outcomes []BrowserOnboardingBatchOutcome) string {
	if len(outcomes) != len(batch.SelectedSources) {
		return BrowserOnboardingBatchPending
	}
	allClaimed := true
	for _, outcome := range outcomes {
		if outcome.Status != BrowserOnboardingPaired || outcome.TenantID == "" || outcome.PairingID == "" {
			if outcome.Status == BrowserOnboardingFailed || outcome.Status == BrowserOnboardingReview {
				return BrowserOnboardingBatchReview
			}
			allClaimed = false
		}
	}
	if allClaimed {
		return BrowserOnboardingBatchReady
	}
	return BrowserOnboardingBatchPending
}

func canonicalBrowserOnboardingBatchSources(input []BrowserOnboardingSource) ([]BrowserOnboardingSource, bool) {
	if len(input) == 0 || len(input) > BrowserOnboardingMaxSources {
		return nil, false
	}
	seen := make(map[string]struct{}, len(input))
	output := make([]BrowserOnboardingSource, 0, len(input))
	for _, source := range input {
		id := strings.TrimSpace(source.SourceCompanyID)
		name := strings.Join(strings.Fields(strings.TrimSpace(source.SourceCompanyName)), " ")
		if !validBrowserSourceCompanyID(id) || name == "" || len(name) > 120 {
			return nil, false
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, false
		}
		seen[id] = struct{}{}
		output = append(output, BrowserOnboardingSource{SourceCompanyID: id, SourceCompanyName: name})
	}
	sort.Slice(output, func(i, j int) bool { return output[i].SourceCompanyID < output[j].SourceCompanyID })
	return output, true
}

func canonicalBrowserOnboardingBatchSourceIDs(input []string) ([]string, bool) {
	if len(input) == 0 || len(input) > BrowserOnboardingMaxSources {
		return nil, false
	}
	seen := make(map[string]struct{}, len(input))
	output := make([]string, 0, len(input))
	for _, candidate := range input {
		candidate = strings.TrimSpace(candidate)
		if !validBrowserSourceCompanyID(candidate) {
			return nil, false
		}
		if _, duplicate := seen[candidate]; duplicate {
			return nil, false
		}
		seen[candidate] = struct{}{}
		output = append(output, candidate)
	}
	sort.Strings(output)
	return output, true
}

func sameBrowserOnboardingBatchManifest(existing, proposed BrowserOnboardingBatch) bool {
	return strings.TrimSpace(existing.OwnerID) == strings.TrimSpace(proposed.OwnerID) && existing.Mode == proposed.Mode && existing.ObservedSourcesSHA256 == proposed.ObservedSourcesSHA256 && existing.ManifestSHA256 == proposed.ManifestSHA256 && sameBrowserOnboardingSourceIDs(existing.ObservedSourceIDs, proposed.ObservedSourceIDs) && sameBrowserOnboardingSources(existing.SelectedSources, proposed.SelectedSources)
}

func validBrowserOnboardingCatalogReceipt(receipt *BrowserOnboardingCatalogReceipt, actorID string, now time.Time) bool {
	if receipt == nil || receipt.Status != BrowserOnboardingCatalogStatusAccepted || !validBrowserPairingID(strings.TrimSpace(receipt.ID)) || !validBrowserPairingID(strings.TrimSpace(receipt.WorkflowID)) || strings.TrimSpace(receipt.OwnerID) != strings.TrimSpace(actorID) || receipt.ObservedAt.IsZero() || receipt.ReceiptExpiresAt.IsZero() || !receipt.ReceiptExpiresAt.After(now) || receipt.ObservedAt.After(now.Add(30*time.Second)) || receipt.ReceiptExpiresAt.Sub(receipt.ObservedAt) > browserOnboardingCatalogMaxLifetime || !validSHA256(receipt.CatalogSHA256) || receipt.CatalogCount != len(receipt.Sources) {
		return false
	}
	companies := sourcesToBrowserOnboardingCatalogCompanies(receipt.Sources)
	canonical, ok := canonicalBrowserOnboardingCatalogCompanies(companies)
	if !ok || !sameBrowserOnboardingCatalogCompanies(companies, canonical) {
		return false
	}
	encoded, err := jsonMarshalBrowserOnboardingCatalogDigest(receipt.SchemaVersion, companies)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]) == receipt.CatalogSHA256
}

func sameBrowserOnboardingSourceIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameBrowserOnboardingSources(left, right []BrowserOnboardingSource) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func canonicalBrowserOnboardingBatchOutcomes(input []BrowserOnboardingBatchOutcome) []BrowserOnboardingBatchOutcome {
	output := append([]BrowserOnboardingBatchOutcome(nil), input...)
	sort.Slice(output, func(i, j int) bool { return output[i].SourceCompanyID < output[j].SourceCompanyID })
	return output
}

func validBrowserOnboardingBatchOutcomes(batch BrowserOnboardingBatch, outcomes []BrowserOnboardingBatchOutcome) bool {
	if len(outcomes) != len(batch.SelectedSources) {
		return false
	}
	expected := make(map[string]BrowserOnboardingSource, len(batch.SelectedSources))
	for _, source := range batch.SelectedSources {
		expected[source.SourceCompanyID] = source
	}
	seen := make(map[string]struct{}, len(outcomes))
	for _, outcome := range outcomes {
		source, found := expected[outcome.SourceCompanyID]
		if !found || source.SourceCompanyName != outcome.SourceCompanyName || !validBrowserOnboardingOutcomeStatus(outcome.Status) || strings.TrimSpace(outcome.ReasonCode) != outcome.ReasonCode || len(outcome.ReasonCode) > 120 || outcome.CreatedAt.IsZero() || outcome.UpdatedAt.IsZero() {
			return false
		}
		if _, duplicate := seen[outcome.SourceCompanyID]; duplicate {
			return false
		}
		seen[outcome.SourceCompanyID] = struct{}{}
		if (outcome.TenantID == "") != (outcome.TenantName == "") || (outcome.TenantID != "" && !safeBridgeID(outcome.TenantID)) || (outcome.PairingID != "" && !validBrowserPairingID(outcome.PairingID)) {
			return false
		}
	}
	return true
}

func validBrowserOnboardingOutcomeStatus(status string) bool {
	switch status {
	case BrowserOnboardingTargetReady, BrowserOnboardingPairingIssued, BrowserOnboardingPaired, BrowserOnboardingReview, BrowserOnboardingFailed:
		return true
	default:
		return false
	}
}

func browserOnboardingBatchDigest(value interface{}) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (s *BrowserOnboardingBatchService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
