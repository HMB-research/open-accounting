package smartaccountssync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const browserCaptureLifetime = 10 * time.Minute

var (
	ErrBrowserCaptureUnauthorized = errors.New("SmartAccounts browser capture authorization is invalid, expired, or not scoped to this request")
	ErrBrowserCaptureInvalid      = errors.New("SmartAccounts browser capture request is not the reviewed general ledger scope")
	ErrBrowserCaptureUnavailable  = errors.New("SmartAccounts browser capture is unavailable")
	ErrBrowserCaptureConsent      = errors.New("SmartAccounts browser capture resume requires renewed owner transfer consent")
)

// BrowserCaptureScope is immutable after owner approval. It is intentionally
// small metadata, never an export, browser cookie, API key, or source record.
type BrowserCaptureScope struct {
	Mode          string   `json:"mode"`
	FromInclusive string   `json:"from_inclusive,omitempty"`
	ToInclusive   string   `json:"to_inclusive,omitempty"`
	CutoffAt      string   `json:"cutoff_at"`
	ResourceIDs   []string `json:"resource_ids"`
}

type BrowserCaptureStartRequest struct {
	SourceCompanyID string              `json:"source_company_id"`
	ManifestVersion string              `json:"manifest_version"`
	Scope           BrowserCaptureScope `json:"scope"`
}

// BrowserCaptureResumeRequest never changes scope. Renewed owner confirmation
// is required after an expiry/restart before OA returns another raw capability.
type BrowserCaptureResumeRequest struct {
	TransferConsentConfirmed bool `json:"transfer_consent_confirmed"`
}

// BrowserCaptureAuthorization persists only the short-lived capability digest
// and immutable scope. The raw capability is returned to the owner exactly
// once, then is reusable only for status, retries, approved resources, and
// finalization of that same run until expiry.
type BrowserCaptureAuthorization struct {
	RunID           string
	TenantID        string
	SourceCompanyID string
	ManifestVersion string
	Scope           BrowserCaptureScope
	TokenSHA256     string
	CreatedBy       string
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type BrowserCaptureTransferConsent struct {
	Version     int       `json:"version"`
	Confirmed   bool      `json:"confirmed"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

type BrowserCaptureIssue struct {
	RunID           string                        `json:"run_id"`
	TenantID        string                        `json:"tenant_id"`
	CaptureToken    string                        `json:"capture_token"`
	ExpiresAt       time.Time                     `json:"expires_at"`
	SourceCompanyID string                        `json:"source_company_id"`
	ManifestVersion string                        `json:"manifest_version"`
	Scope           BrowserCaptureScope           `json:"scope"`
	Status          string                        `json:"status"`
	TransferConsent BrowserCaptureTransferConsent `json:"transfer_consent"`
}

type BrowserCaptureResourceStatus struct {
	ResourceID string `json:"resource_id"`
	Coverage   string `json:"coverage"`
	Status     string `json:"status"`
	Created    bool   `json:"created,omitempty"`
}

// BrowserCaptureCoverageIssue is a fixed, redacted bridge coverage category.
// It deliberately has no source row, header, browser URL, or private path.
type BrowserCaptureCoverageIssue struct {
	ResourceID string `json:"resource_id,omitempty"`
	Code       string `json:"code"`
}

// BrowserCaptureCoverageReceipt is immutable finalization evidence. A partial
// receipt is evidence only; it does not mean complete history or financial
// approval.
type BrowserCaptureCoverageReceipt struct {
	Status               string                        `json:"status"`
	Ready                bool                          `json:"ready"`
	CompletedExportCount int                           `json:"completed_export_count"`
	RequiredExportCount  int                           `json:"required_export_count"`
	BlockedPageOnlyCount int                           `json:"blocked_page_only_count"`
	Issues               []BrowserCaptureCoverageIssue `json:"issues,omitempty"`
	FinalizedAt          string                        `json:"finalized_at"`
}

// BrowserCaptureStaging contains the bridge-to-OA package handoff progress.
// It contains only immutable package digests and counts, never source data.
type BrowserCaptureStaging struct {
	PackageID                  string `json:"package_id"`
	PackageSHA256              string `json:"package_sha256"`
	Status                     string `json:"status"`
	IssueCode                  string `json:"issue_code,omitempty"`
	RecordChunksAcknowledged   int    `json:"record_chunks_acknowledged"`
	ArtifactChunksAcknowledged int    `json:"artifact_chunks_acknowledged"`
	Finalized                  bool   `json:"finalized"`
}

// BrowserCaptureStatus is safe browser-facing progress. It has no source
// rows, bytes, headers, credentials, source identifier, or raw token.
type BrowserCaptureStatus struct {
	RunID           string                         `json:"run_id"`
	TenantID        string                         `json:"tenant_id"`
	SourceCompanyID string                         `json:"source_company_id"`
	Status          string                         `json:"status"`
	ManifestVersion string                         `json:"manifest_version"`
	Scope           BrowserCaptureScope            `json:"scope"`
	Resources       []BrowserCaptureResourceStatus `json:"resources"`
	Receipt         *BrowserCaptureCoverageReceipt `json:"receipt,omitempty"`
	Staging         *BrowserCaptureStaging         `json:"staging,omitempty"`
}

type BrowserCaptureStore interface {
	CreateBrowserCaptureAuthorization(context.Context, BrowserCaptureAuthorization) error
	GetBrowserCaptureAuthorization(context.Context, string, string) (*BrowserCaptureAuthorization, error)
	RotateBrowserCaptureAuthorization(context.Context, BrowserCaptureAuthorization) error
}

// BrowserCaptureControlReader deliberately avoids the broader Store contract;
// capture authorization only needs a single tenant/source binding lookup.
type BrowserCaptureControlReader interface {
	Get(context.Context, string, string) (*Control, error)
}

type BrowserCaptureBridge interface {
	StartBrowserCapture(context.Context, string, string, BrowserCaptureStartRequest) (BrowserCaptureStatus, error)
	GetBrowserCapture(context.Context, string, string) (BrowserCaptureStatus, error)
	UploadBrowserCaptureResource(context.Context, string, string, string, string, string, []byte) (BrowserCaptureResourceStatus, error)
	FinalizeBrowserCapture(context.Context, string, string) (BrowserCaptureStatus, error)
}

type BrowserCaptureService struct {
	store    BrowserCaptureStore
	controls BrowserCaptureControlReader
	bridge   BrowserCaptureBridge
	now      func() time.Time
	newRun   func() string
	newToken func() (string, error)
}

func NewBrowserCaptureService(store BrowserCaptureStore, controls BrowserCaptureControlReader, bridge BrowserCaptureBridge) *BrowserCaptureService {
	return &BrowserCaptureService{store: store, controls: controls, bridge: bridge, now: time.Now, newRun: uuid.NewString, newToken: newBrowserCaptureToken}
}

// Issue requires an existing tenant-scoped Brave source binding. OA records
// the token hash before contacting the bridge, so no bridge run can be used
// without an immutable OA authorization. A failed bridge start returns no raw
// token; the harmless expired digest row cannot authorize a guessed token.
func (s *BrowserCaptureService) Issue(ctx context.Context, tenantID, actor string, request BrowserCaptureStartRequest) (*BrowserCaptureIssue, error) {
	if s == nil || s.newRun == nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	if !validBrowserCaptureRequest(request) {
		return nil, ErrBrowserCaptureInvalid
	}
	runID := s.newRun()
	if !validBrowserPairingID(runID) {
		return nil, ErrBrowserCaptureUnavailable
	}
	return s.IssueForRun(ctx, tenantID, actor, runID, request)
}

// IssueForRun creates or rotates a raw relay capability for one already
// chosen immutable run. It is used by the workflow control plane so an owner
// retry (or a browser restart) keeps the same tenant/source/scope/run rather
// than creating duplicate bridge captures. The raw token is never persisted.
func (s *BrowserCaptureService) IssueForRun(ctx context.Context, tenantID, actor, runID string, request BrowserCaptureStartRequest) (*BrowserCaptureIssue, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || s.newToken == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(strings.TrimSpace(runID)) {
		return nil, ErrBrowserCaptureUnavailable
	}
	if !validBrowserCaptureRequest(request) {
		return nil, ErrBrowserCaptureInvalid
	}
	tenantID, runID = strings.TrimSpace(tenantID), strings.TrimSpace(runID)
	request.SourceCompanyID = strings.TrimSpace(request.SourceCompanyID)
	control, err := s.controls.Get(ctx, tenantID, request.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserCaptureUnauthorized
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	now := s.currentTime()
	request.Scope = canonicalBrowserCaptureScope(request.Scope)
	authorization := BrowserCaptureAuthorization{RunID: runID, TenantID: tenantID, SourceCompanyID: request.SourceCompanyID, ManifestVersion: request.ManifestVersion, Scope: request.Scope, TokenSHA256: browserCaptureTokenSHA256(token), CreatedBy: strings.TrimSpace(actor), ExpiresAt: now.Add(browserCaptureLifetime), CreatedAt: now}
	existing, lookupErr := s.store.GetBrowserCaptureAuthorization(ctx, runID, tenantID)
	switch {
	case lookupErr == nil && existing != nil:
		if existing.SourceCompanyID != authorization.SourceCompanyID || existing.ManifestVersion != authorization.ManifestVersion || !sameBrowserCaptureScope(existing.Scope, authorization.Scope) {
			return nil, ErrBrowserCaptureUnauthorized
		}
		// Preserve the original record time for audit continuity; rotate only
		// the short-lived opaque capability digest and expiry.
		authorization.CreatedAt = existing.CreatedAt
		if err := s.store.RotateBrowserCaptureAuthorization(ctx, authorization); err != nil {
			return nil, ErrBrowserCaptureUnavailable
		}
	case errors.Is(lookupErr, ErrBrowserCaptureUnauthorized):
		if err := s.store.CreateBrowserCaptureAuthorization(ctx, authorization); err != nil {
			return nil, ErrBrowserCaptureUnavailable
		}
	default:
		return nil, ErrBrowserCaptureUnavailable
	}
	status, err := s.bridge.StartBrowserCapture(ctx, tenantID, runID, request)
	if err != nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	if _, err := bindBrowserCaptureStatus(status, &authorization); err != nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	return &BrowserCaptureIssue{RunID: runID, TenantID: tenantID, CaptureToken: token, ExpiresAt: authorization.ExpiresAt, SourceCompanyID: request.SourceCompanyID, ManifestVersion: request.ManifestVersion, Scope: request.Scope, Status: status.Status, TransferConsent: BrowserCaptureTransferConsent{Version: 1, Confirmed: true, ConfirmedAt: now}}, nil
}

func (s *BrowserCaptureService) Status(ctx context.Context, tenantID, runID, token string) (BrowserCaptureStatus, error) {
	authorization, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil {
		return BrowserCaptureStatus{}, err
	}
	status, err := s.bridge.GetBrowserCapture(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(runID))
	if err != nil {
		return BrowserCaptureStatus{}, err
	}
	return bindBrowserCaptureStatus(status, authorization)
}

func (s *BrowserCaptureService) Upload(ctx context.Context, tenantID, runID, resourceID, token, digest string, body []byte) (BrowserCaptureResourceStatus, error) {
	authorization, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil || !scopeAllowsResource(authorization.Scope, resourceID) || len(body) == 0 || len(body) > BrowserCaptureMaxResourceBytes || !validSHA256(digest) || browserCaptureTokenSHA256(string(body)) != strings.TrimSpace(digest) {
		return BrowserCaptureResourceStatus{}, ErrBrowserCaptureUnauthorized
	}
	return s.bridge.UploadBrowserCaptureResource(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(runID), strings.TrimSpace(resourceID), strings.TrimSpace(digest), "text/csv", body)
}

func (s *BrowserCaptureService) Finalize(ctx context.Context, tenantID, runID, token string) (BrowserCaptureStatus, error) {
	authorization, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil {
		return BrowserCaptureStatus{}, err
	}
	status, err := s.bridge.FinalizeBrowserCapture(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(runID))
	if err != nil {
		return BrowserCaptureStatus{}, err
	}
	return bindBrowserCaptureStatus(status, authorization)
}

// OwnerStatus reads safe bridge progress for an existing immutable run. It is
// deliberately separate from extension relay status: ownership authenticates
// the caller at the HTTP layer and this method never returns a raw capability
// or its stored digest.
func (s *BrowserCaptureService) OwnerStatus(ctx context.Context, tenantID, runID string) (BrowserCaptureStatus, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(runID) {
		return BrowserCaptureStatus{}, ErrBrowserCaptureUnavailable
	}
	tenantID, runID = strings.TrimSpace(tenantID), strings.TrimSpace(runID)
	authorization, err := s.store.GetBrowserCaptureAuthorization(ctx, runID, tenantID)
	if err != nil || authorization == nil || !validBrowserCaptureRequest(BrowserCaptureStartRequest{SourceCompanyID: authorization.SourceCompanyID, ManifestVersion: authorization.ManifestVersion, Scope: authorization.Scope}) {
		return BrowserCaptureStatus{}, ErrBrowserCaptureUnauthorized
	}
	control, err := s.controls.Get(ctx, tenantID, authorization.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return BrowserCaptureStatus{}, ErrBrowserCaptureUnauthorized
	}
	status, err := s.bridge.GetBrowserCapture(ctx, tenantID, runID)
	if err != nil {
		return BrowserCaptureStatus{}, ErrBrowserCaptureUnavailable
	}
	return bindBrowserCaptureStatus(status, authorization)
}

// Resume rotates an expired/lost browser capability for the same immutable
// run. It never starts a new bridge run or changes tenant, source, manifest,
// scope, or resources. The caller must newly affirm transfer consent after a
// browser/extension restart because the original raw capability is gone.
func (s *BrowserCaptureService) Resume(ctx context.Context, tenantID, actor, runID string, request BrowserCaptureResumeRequest) (*BrowserCaptureIssue, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(runID) {
		return nil, ErrBrowserCaptureUnavailable
	}
	if !request.TransferConsentConfirmed {
		return nil, ErrBrowserCaptureConsent
	}
	tenantID, runID = strings.TrimSpace(tenantID), strings.TrimSpace(runID)
	authorization, err := s.store.GetBrowserCaptureAuthorization(ctx, runID, tenantID)
	if err != nil || authorization == nil || !validBrowserCaptureRequest(BrowserCaptureStartRequest{SourceCompanyID: authorization.SourceCompanyID, ManifestVersion: authorization.ManifestVersion, Scope: authorization.Scope}) {
		return nil, ErrBrowserCaptureUnauthorized
	}
	control, err := s.controls.Get(ctx, tenantID, authorization.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserCaptureUnauthorized
	}
	status, err := s.bridge.GetBrowserCapture(ctx, tenantID, runID)
	if err != nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	now := s.currentTime()
	authorization.TokenSHA256 = browserCaptureTokenSHA256(token)
	authorization.CreatedBy = strings.TrimSpace(actor)
	authorization.ExpiresAt = now.Add(browserCaptureLifetime)
	if err := s.store.RotateBrowserCaptureAuthorization(ctx, *authorization); err != nil {
		return nil, ErrBrowserCaptureUnavailable
	}
	return &BrowserCaptureIssue{RunID: runID, TenantID: tenantID, CaptureToken: token, ExpiresAt: authorization.ExpiresAt, SourceCompanyID: authorization.SourceCompanyID, ManifestVersion: authorization.ManifestVersion, Scope: canonicalBrowserCaptureScope(authorization.Scope), Status: status.Status, TransferConsent: BrowserCaptureTransferConsent{Version: 1, Confirmed: true, ConfirmedAt: now}}, nil
}

func bindBrowserCaptureStatus(status BrowserCaptureStatus, authorization *BrowserCaptureAuthorization) (BrowserCaptureStatus, error) {
	if authorization == nil || status.RunID != authorization.RunID || status.ManifestVersion != authorization.ManifestVersion || !sameBrowserCaptureScope(status.Scope, authorization.Scope) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	if !sameBrowserCaptureResources(status.Resources, authorization.Scope.ResourceIDs) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	status.TenantID = authorization.TenantID
	status.SourceCompanyID = authorization.SourceCompanyID
	return status, nil
}

func (s *BrowserCaptureService) authorize(ctx context.Context, tenantID, runID, token string) (*BrowserCaptureAuthorization, error) {
	if s == nil || s.store == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(runID) || !validBrowserCaptureToken(token) {
		return nil, ErrBrowserCaptureUnauthorized
	}
	authorization, err := s.store.GetBrowserCaptureAuthorization(ctx, strings.TrimSpace(runID), strings.TrimSpace(tenantID))
	if err != nil || authorization == nil || !authorization.ExpiresAt.After(s.currentTime()) || authorization.TokenSHA256 != browserCaptureTokenSHA256(token) {
		return nil, ErrBrowserCaptureUnauthorized
	}
	return authorization, nil
}

func (s *BrowserCaptureService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func newBrowserCaptureToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func browserCaptureTokenSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func validBrowserCaptureToken(value string) bool { return validBrowserPairingToken(value) }

func validBrowserCaptureRequest(request BrowserCaptureStartRequest) bool {
	if !validBrowserSourceCompanyID(request.SourceCompanyID) || request.ManifestVersion != BrowserCaptureManifestVersion {
		return false
	}
	scope := canonicalBrowserCaptureScope(request.Scope)
	// The generic owner capture route deliberately shares this validator with
	// the server-derived workflow.  It is *not* a broad browser-export escape
	// hatch: v2 has exactly one reviewed capture surface.  The
	// journal_entries grid is summary evidence and every other resource needs
	// its own reviewed, server-issued workflow before it can be relayed.
	if scope.Mode != "partial" || len(scope.ResourceIDs) != 1 || scope.ResourceIDs[0] != BrowserGeneralLedgerResourceID || scope.CutoffAt == "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339, scope.CutoffAt); err != nil {
		return false
	}
	if scope.FromInclusive == "" || scope.ToInclusive == "" {
		return false
	}
	from, fromErr := time.Parse(time.DateOnly, scope.FromInclusive)
	to, toErr := time.Parse(time.DateOnly, scope.ToInclusive)
	if fromErr != nil || toErr != nil || to.Before(from) {
		return false
	}
	for index, resourceID := range scope.ResourceIDs {
		if !safeBridgeID(resourceID) || (index > 0 && resourceID == scope.ResourceIDs[index-1]) {
			return false
		}
	}
	return true
}

func canonicalBrowserCaptureScope(scope BrowserCaptureScope) BrowserCaptureScope {
	scope.Mode = strings.TrimSpace(scope.Mode)
	scope.FromInclusive = strings.TrimSpace(scope.FromInclusive)
	scope.ToInclusive = strings.TrimSpace(scope.ToInclusive)
	scope.CutoffAt = strings.TrimSpace(scope.CutoffAt)
	scope.ResourceIDs = append([]string(nil), scope.ResourceIDs...)
	for index := range scope.ResourceIDs {
		scope.ResourceIDs[index] = strings.TrimSpace(scope.ResourceIDs[index])
	}
	sort.Strings(scope.ResourceIDs)
	return scope
}

func scopeAllowsResource(scope BrowserCaptureScope, resourceID string) bool {
	for _, candidate := range scope.ResourceIDs {
		if candidate == strings.TrimSpace(resourceID) {
			return true
		}
	}
	return false
}

func sameBrowserCaptureScope(left, right BrowserCaptureScope) bool {
	left, right = canonicalBrowserCaptureScope(left), canonicalBrowserCaptureScope(right)
	if left.Mode != right.Mode || left.FromInclusive != right.FromInclusive || left.ToInclusive != right.ToInclusive || left.CutoffAt != right.CutoffAt || len(left.ResourceIDs) != len(right.ResourceIDs) {
		return false
	}
	for index := range left.ResourceIDs {
		if left.ResourceIDs[index] != right.ResourceIDs[index] {
			return false
		}
	}
	return true
}

func sameBrowserCaptureResources(resources []BrowserCaptureResourceStatus, expectedIDs []string) bool {
	if len(resources) != len(expectedIDs) {
		return false
	}
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if _, found := seen[resource.ResourceID]; found || !scopeAllowsResource(BrowserCaptureScope{ResourceIDs: expectedIDs}, resource.ResourceID) {
			return false
		}
		seen[resource.ResourceID] = struct{}{}
	}
	return true
}
