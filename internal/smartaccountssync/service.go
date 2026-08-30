package smartaccountssync

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

var (
	ErrControlNotConfigured       = errors.New("SmartAccounts sync control is not configured")
	ErrExplicitConfirmation       = errors.New("explicit confirmation is required before financial apply")
	ErrFinancialApplyUnavailable  = errors.New("financial apply is blocked until bridge capture, plan, and reconciliation review are complete")
	ErrBridgeDiscoveryUnavailable = errors.New("verified SmartAccounts bridge discovery is unavailable")
	ErrBrowserCaptureRequired     = errors.New("SmartAccounts browser relay capture is required for this browser-session binding")
)

// Store persists selected source-to-target tenant bindings and opaque
// references. It has no SmartAccounts client, accounting service, journal
// writer, invoice writer, or payment writer.
type Store interface {
	Get(ctx context.Context, tenantID, sourceCompanyID string) (*Control, error)
	Upsert(ctx context.Context, control Control) (*Control, error)
	MarkDryRunRequested(ctx context.Context, tenantID, sourceCompanyID string, requestedAt time.Time) (*Control, error)
	RecordCaptureRun(ctx context.Context, tenantID, sourceCompanyID, runID string, requestedAt time.Time) (*Control, error)
}

// captureHistoryStore is optional while deployments roll forward from the
// single-run control pointer. Production persistence implements it; the
// narrow test doubles can stay focused on connection policy.
type captureHistoryStore interface {
	UpsertCaptureProgress(ctx context.Context, tenantID, sourceCompanyID string, progress CaptureProgress, observedAt time.Time) error
	ListCaptureProgresses(ctx context.Context, tenantID, sourceCompanyID string) ([]CaptureProgress, error)
}

// BridgeCatalog is supplied by the private bridge. It returns verified,
// stable provider company IDs; Open Accounting never manufactures IDs based on
// display names. Its implementation must not return source accounting data.
type BridgeCatalog interface {
	Discover(ctx context.Context, tenantID string) (SourceDiscovery, error)
}

// UnavailableBridgeCatalog is the production-safe default before the private
// bridge catalog is configured. It performs no network operation.
type UnavailableBridgeCatalog struct{}

func (UnavailableBridgeCatalog) Discover(_ context.Context, _ string) (SourceDiscovery, error) {
	return SourceDiscovery{BridgeAvailable: false, LiveDataContacted: false}, ErrBridgeDiscoveryUnavailable
}

// StaticBridgeCatalog is an injectable catalog seam for tests and a private
// bridge adapter. It is not wired with a hard-coded production company ID.
type StaticBridgeCatalog struct {
	Discovery SourceDiscovery
	Err       error
}

func (c StaticBridgeCatalog) Discover(_ context.Context, _ string) (SourceDiscovery, error) {
	if c.Err != nil {
		return SourceDiscovery{}, c.Err
	}
	discovery := c.Discovery
	discovery.Sources = append([]SourceCandidate(nil), discovery.Sources...)
	sort.Slice(discovery.Sources, func(i, j int) bool {
		return discovery.Sources[i].SourceCompanyID < discovery.Sources[j].SourceCompanyID
	})
	return discovery, nil
}

// Service exposes a narrow preparatory control plane for a future bridge.
type Service struct {
	store   Store
	catalog BridgeCatalog
	now     func() time.Time
}

func NewService(store Store, catalog BridgeCatalog) *Service {
	if catalog == nil {
		catalog = UnavailableBridgeCatalog{}
	}
	return &Service{store: store, catalog: catalog, now: time.Now}
}

// DiscoverSources delegates discovery to the bridge catalog. It never calls
// SmartAccounts from Open Accounting and reports only catalog metadata.
func (s *Service) DiscoverSources(ctx context.Context, tenantID string) (SourceDiscovery, error) {
	if s == nil || s.catalog == nil {
		return SourceDiscovery{}, ErrBridgeDiscoveryUnavailable
	}
	discovery, err := s.catalog.Discover(ctx, strings.TrimSpace(tenantID))
	if err != nil {
		return SourceDiscovery{}, err
	}
	if !discovery.BridgeAvailable {
		return SourceDiscovery{}, ErrBridgeDiscoveryUnavailable
	}
	for _, source := range discovery.Sources {
		if !isVerifiedSourceCandidate(source) {
			return SourceDiscovery{}, errors.New("bridge catalog returned an invalid source candidate")
		}
	}
	return discovery, nil
}

// Configure verifies the selected stable source ID against the bridge catalog
// at configuration time, then stores an opaque secret-manager reference. The
// raw credential itself is rejected and no request value is included in a
// result.
func (s *Service) Configure(ctx context.Context, tenantID, actorID string, req ConfigureRequest) (*SyncStatus, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("SmartAccounts sync storage is not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("tenant id is required")
	}
	discovery, err := s.DiscoverSources(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("discover bridge sources: %w", err)
	}
	source, found := selectVerifiedSource(discovery.Sources, req.SourceCompanyID)
	if !found {
		return nil, errors.New("selected source is not verified by the bridge catalog")
	}
	if !req.SmartAccountsGLAuthoritative || !source.GeneralLedgerAuthoritative {
		return nil, errors.New("SmartAccounts GL authority must be explicitly confirmed")
	}
	if strings.TrimSpace(req.InvoicePaymentMode) != InvoicePaymentModeNonPosting || source.InvoicePaymentMode != InvoicePaymentModeNonPosting {
		return nil, errors.New("invoice and payment records must remain NON_POSTING for the GL-authoritative sync")
	}
	secretReference, err := validateOpaqueSecretReference(req.SecretReference)
	if err != nil {
		return nil, err
	}
	now := s.currentTime()
	control, err := s.store.Upsert(ctx, Control{
		TenantID:          tenantID,
		SourceCompanyID:   source.SourceCompanyID,
		SourceCompanyName: source.SourceCompanyName,
		SecretReference:   secretReference,
		CreatedBy:         strings.TrimSpace(actorID),
		UpdatedAt:         now,
	})
	if err != nil {
		return nil, fmt.Errorf("save SmartAccounts sync control: %w", err)
	}
	return statusForControl(control), nil
}

// ConfigureBridgeConnection persists the opaque bridge reference and source
// identity returned only after the bridge validates the credentials. The
// browser cannot supply a source company ID.
func (s *Service) ConfigureBridgeConnection(ctx context.Context, tenantID, actorID string, req ConnectRequest, connection BridgeConnection) (*SyncStatus, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("SmartAccounts sync storage is not configured")
	}
	if err := validateConnectionPolicy(tenantID, req); err != nil {
		return nil, err
	}
	if !safeBridgeID(connection.ConnectionID) || !safeBridgeID(connection.SourceCompanyID) || strings.TrimSpace(connection.SourceCompanyName) == "" || connection.SourceBindingStatus != "api_key_identity_and_snapshot_validated" || !validSHA256(connection.AccountSnapshotSHA256) {
		return nil, errors.New("bridge returned an invalid source identity")
	}
	secretReference, err := validateOpaqueSecretReference(connection.SecretReference)
	if err != nil || !isExpectedBridgeSecretReference(secretReference, connection.ConnectionID) {
		return nil, errors.New("bridge returned an invalid opaque secret reference")
	}
	now := s.currentTime()
	control, err := s.store.Upsert(ctx, Control{
		TenantID:          strings.TrimSpace(tenantID),
		SourceCompanyID:   connection.SourceCompanyID,
		SourceCompanyName: strings.TrimSpace(connection.SourceCompanyName),
		SecretReference:   secretReference,
		CreatedBy:         strings.TrimSpace(actorID),
		UpdatedAt:         now,
	})
	if err != nil {
		return nil, fmt.Errorf("save SmartAccounts sync control: %w", err)
	}
	return statusForControl(control), nil
}

// ConfigureBrowserSession stores a pairing-derived opaque browser-session
// reference and selected SmartAccounts UI company identifier. It does not
// receive a browser cookie, API key, source export, or financial data.
func (s *Service) ConfigureBrowserSession(ctx context.Context, tenantID, actorID, pairingID, sourceCompanyID string) (*SyncStatus, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("SmartAccounts sync storage is not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	sourceCompanyID = strings.TrimSpace(sourceCompanyID)
	if !safeBridgeID(tenantID) || !validBrowserPairingID(pairingID) || !validBrowserSourceCompanyID(sourceCompanyID) {
		return nil, errors.New("browser-session source binding is invalid")
	}
	now := s.currentTime()
	control, err := s.store.Upsert(ctx, Control{
		TenantID:          tenantID,
		SourceCompanyID:   sourceCompanyID,
		SourceCompanyName: "SmartAccounts browser session",
		SecretReference:   browserSessionReference(pairingID),
		CreatedBy:         strings.TrimSpace(actorID),
		UpdatedAt:         now,
	})
	if err != nil {
		return nil, fmt.Errorf("save SmartAccounts browser-session control: %w", err)
	}
	return statusForControl(control), nil
}

// Status returns a safe non-financial status for one explicit source-to-target
// tenant binding. No source means no implicit all-source selection.
func (s *Service) Status(ctx context.Context, tenantID, sourceCompanyID string) (*SyncStatus, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("SmartAccounts sync storage is not configured")
	}
	if strings.TrimSpace(sourceCompanyID) == "" {
		return nil, errors.New("source company id is required")
	}
	control, err := s.store.Get(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID))
	if errors.Is(err, ErrControlNotConfigured) {
		return &SyncStatus{
			Provider:                     Provider,
			SourceCompanyID:              strings.TrimSpace(sourceCompanyID),
			InvoicePaymentMode:           InvoicePaymentModeNonPosting,
			CaptureStatus:                CaptureStatusNotRequested,
			PlanStatus:                   PlanStatusNotRequested,
			ReconciliationStatus:         ReconciliationStatusPending,
			ExplicitConfirmationRequired: true,
			NextAction:                   "Connect and validate this verified source-to-target tenant binding through the private bridge.",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts sync control: %w", err)
	}
	return statusForControl(control), nil
}

// RequestDryRun records only operator intent for one explicit binding. It
// deliberately does not call the bridge, fetch source data, build a financial
// plan, or apply anything.
func (s *Service) RequestDryRun(ctx context.Context, tenantID, sourceCompanyID string) (*SyncStatus, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("SmartAccounts sync storage is not configured")
	}
	control, err := s.store.Get(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID))
	if err != nil {
		return nil, err
	}
	if isBrowserSessionReference(control.SecretReference) {
		return statusForControl(control), ErrBrowserCaptureRequired
	}
	if control.DryRunRequestedAt == nil {
		control, err = s.store.MarkDryRunRequested(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID), s.currentTime())
		if err != nil {
			return nil, fmt.Errorf("record SmartAccounts dry-run request: %w", err)
		}
	}
	return statusForControl(control), nil
}

// StartCapture invokes the bridge's read-only capture and then records only
// its safe run ID. Source records remain bridge-owned and are never persisted
// in Open Accounting by this control path.
func (s *Service) StartCapture(ctx context.Context, tenantID, sourceCompanyID string, request CaptureRequest, bridge BridgeClient) (*SyncStatus, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("SmartAccounts sync storage is not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	sourceCompanyID = strings.TrimSpace(sourceCompanyID)
	control, err := s.store.Get(ctx, tenantID, sourceCompanyID)
	if err != nil {
		return nil, err
	}
	if isBrowserSessionReference(control.SecretReference) {
		return statusForControl(control), ErrBrowserCaptureRequired
	}
	if bridge == nil {
		return nil, ErrBridgeClientUnavailable
	}
	connectionID, err := bridgeConnectionID(control.SecretReference)
	if err != nil {
		return nil, err
	}
	progress, err := bridge.StartCapture(ctx, tenantID, connectionID, request)
	if err != nil {
		return nil, err
	}
	if progress.RunID == "" {
		return nil, errors.New("bridge returned an invalid capture run")
	}
	control, err = s.store.RecordCaptureRun(ctx, tenantID, sourceCompanyID, progress.RunID, s.currentTime())
	if err != nil {
		return nil, fmt.Errorf("record SmartAccounts capture run: %w", err)
	}
	if err := s.recordCaptureProgress(ctx, tenantID, sourceCompanyID, progress); err != nil {
		return nil, err
	}
	status := statusForControl(control)
	status.CaptureStatus = progress.Status
	status.CaptureRunID = progress.RunID
	status.CaptureProgress = &progress
	if err := s.addCaptureHistory(ctx, tenantID, sourceCompanyID, status); err != nil {
		return nil, err
	}
	applyStagingStatus(status, progress.Staging)
	return status, nil
}

// StatusWithCapture returns the existing safe status and, when a run has
// started, proxies only its safe bridge progress summary.
func (s *Service) StatusWithCapture(ctx context.Context, tenantID, sourceCompanyID string, bridge BridgeClient) (*SyncStatus, error) {
	status, err := s.Status(ctx, tenantID, sourceCompanyID)
	if err != nil || !status.Configured {
		return status, err
	}
	control, err := s.store.Get(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID))
	if err != nil || strings.TrimSpace(control.CaptureRunID) == "" {
		if err == nil {
			err = s.addCaptureHistory(ctx, tenantID, sourceCompanyID, status)
		}
		return status, err
	}
	if isBrowserSessionReference(control.SecretReference) {
		return statusForControl(control), ErrBrowserCaptureRequired
	}
	if bridge == nil {
		return nil, ErrBridgeClientUnavailable
	}
	connectionID, err := bridgeConnectionID(control.SecretReference)
	if err != nil {
		return nil, err
	}
	progress, err := bridge.GetCapture(ctx, strings.TrimSpace(tenantID), connectionID, control.CaptureRunID)
	if err != nil {
		return nil, err
	}
	if err := s.recordCaptureProgress(ctx, tenantID, sourceCompanyID, progress); err != nil {
		return nil, err
	}
	status.CaptureStatus = progress.Status
	status.CaptureRunID = progress.RunID
	status.CaptureProgress = &progress
	if err := s.addCaptureHistory(ctx, tenantID, sourceCompanyID, status); err != nil {
		return nil, err
	}
	applyStagingStatus(status, progress.Staging)
	return status, nil
}

func (s *Service) recordCaptureProgress(ctx context.Context, tenantID, sourceCompanyID string, progress CaptureProgress) error {
	history, ok := s.store.(captureHistoryStore)
	if !ok {
		return nil
	}
	if err := history.UpsertCaptureProgress(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID), progress, s.currentTime()); err != nil {
		return fmt.Errorf("record SmartAccounts capture progress: %w", err)
	}
	return nil
}

func (s *Service) addCaptureHistory(ctx context.Context, tenantID, sourceCompanyID string, status *SyncStatus) error {
	if status == nil {
		return nil
	}
	history, ok := s.store.(captureHistoryStore)
	if !ok {
		return nil
	}
	progresses, err := history.ListCaptureProgresses(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID))
	if err != nil {
		return fmt.Errorf("list SmartAccounts capture progress: %w", err)
	}
	status.CaptureProgresses = progresses
	for _, progress := range progresses {
		applyStagingStatus(status, progress.Staging)
	}
	return nil
}

func applyStagingStatus(status *SyncStatus, staging *CaptureStaging) {
	if status == nil || staging == nil || !staging.Finalized || strings.ToLower(strings.TrimSpace(staging.Status)) != "staged_review_required" {
		return
	}
	status.PlanStatus = "STAGED_REVIEW_REQUIRED"
	status.ReconciliationStatus = "REVIEW_REQUIRED"
	status.NextAction = "Review the staged SmartAccounts chart and GL preview, then explicitly confirm apply."
}

// ConfirmFinancialApply is the only intended entry point for a future apply
// executor. This preparatory control plane always blocks after checking an
// explicit confirmation, so it cannot make financial writes.
func (s *Service) ConfirmFinancialApply(ctx context.Context, tenantID, sourceCompanyID string, req ConfirmApplyRequest) (*SyncStatus, error) {
	if !req.Confirm {
		return nil, ErrExplicitConfirmation
	}
	status, err := s.Status(ctx, tenantID, sourceCompanyID)
	if err != nil {
		return nil, err
	}
	return status, ErrFinancialApplyUnavailable
}

func isVerifiedSourceCandidate(source SourceCandidate) bool {
	return strings.TrimSpace(source.Provider) == Provider &&
		strings.TrimSpace(source.SourceCompanyID) != "" &&
		strings.TrimSpace(source.SourceCompanyName) != "" &&
		source.BridgeVerified
}

func selectVerifiedSource(sources []SourceCandidate, sourceCompanyID string) (SourceCandidate, bool) {
	for _, source := range sources {
		if strings.TrimSpace(source.SourceCompanyID) == strings.TrimSpace(sourceCompanyID) && isVerifiedSourceCandidate(source) {
			return source, true
		}
	}
	return SourceCandidate{}, false
}

func statusForControl(control *Control) *SyncStatus {
	if control == nil {
		return nil
	}
	status := &SyncStatus{
		Provider:                     Provider,
		SourceCompanyID:              control.SourceCompanyID,
		SourceCompanyName:            control.SourceCompanyName,
		Configured:                   true,
		SecretReferenceConfigured:    strings.TrimSpace(control.SecretReference) != "",
		SmartAccountsGLAuthoritative: true,
		InvoicePaymentMode:           InvoicePaymentModeNonPosting,
		CaptureStatus:                CaptureStatusNotRequested,
		PlanStatus:                   PlanStatusNotRequested,
		ReconciliationStatus:         ReconciliationStatusPending,
		ExplicitConfirmationRequired: true,
		FinancialApplyEligible:       false,
		FinancialWritesStarted:       false,
		NextAction:                   "Request a dry run to await bridge capture; no source data has been contacted.",
	}
	if isBrowserSessionReference(control.SecretReference) {
		status.CaptureStatus = "AWAITING_BRAVE_BROWSER_CAPTURE"
		status.PlanStatus = PlanStatusAwaitingCapturedInput
		status.NextAction = "Brave browser session paired. Start the browser-local read-only capture and review its complete staged package before any financial apply."
		return status
	}
	if control.DryRunRequestedAt != nil {
		status.DryRunRequestedAt = control.DryRunRequestedAt
		status.CaptureStatus = CaptureStatusAwaitingBridge
		status.PlanStatus = PlanStatusAwaitingCapturedInput
		status.NextAction = "Await bridge capture, then review the GL plan and reconciliation evidence before explicit financial apply confirmation."
	}
	if strings.TrimSpace(control.CaptureRunID) != "" {
		status.CaptureRunID = control.CaptureRunID
		status.CaptureStatus = CaptureStatusAwaitingBridge
		status.PlanStatus = PlanStatusAwaitingCapturedInput
		status.NextAction = "Capture is running in the private bridge; review safe progress and reconciliation evidence before any future financial apply."
	}
	return status
}

func validateOpaqueSecretReference(value string) (string, error) {
	reference := strings.TrimSpace(value)
	if reference == "" {
		return "", errors.New("opaque secret reference is required")
	}
	if len(reference) > 512 || strings.ContainsAny(reference, "\r\n\t ") {
		return "", errors.New("opaque secret reference must be a short URI without whitespace")
	}
	parsed, err := url.ParseRequestURI(reference)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("opaque secret reference must be a secret-manager URI, not a raw key")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "secret-ref", "vault", "op", "sops":
		return reference, nil
	default:
		return "", errors.New("opaque secret reference must use secret-ref, vault, op, or sops URI scheme")
	}
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
