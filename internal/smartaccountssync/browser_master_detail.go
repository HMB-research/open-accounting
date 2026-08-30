package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BrowserMasterDetailManifestVersion = "smartaccounts-browser-master-detail-v1"
	BrowserMasterDetailConsentVersion  = "smartaccounts-browser-master-detail-transfer-consent-v1"
	BrowserMasterDetailReviewVersion   = "open-accounting-browser-master-detail-review-v1"
	BrowserMasterDetailSnapshotPolicy  = "current_snapshot_only"

	BrowserMasterDetailClientsResource  = "clients"
	BrowserMasterDetailVendorsResource  = "vendors"
	BrowserMasterDetailArticlesResource = "articles"

	BrowserMasterDetailClientsSchema  = BrowserMasterDetailManifestVersion + "/clients_detail_v1"
	BrowserMasterDetailVendorsSchema  = BrowserMasterDetailManifestVersion + "/vendors_detail_v1"
	BrowserMasterDetailArticlesSchema = BrowserMasterDetailManifestVersion + "/articles_detail_v1"

	browserMasterDetailLifetime = 10 * time.Minute
	browserMasterDetailMaxBytes = 32 << 20
)

var (
	ErrBrowserMasterDetailUnauthorized = errors.New("SmartAccounts browser master-detail authorization is invalid, expired, or not scoped to this request")
	ErrBrowserMasterDetailUnavailable  = errors.New("SmartAccounts browser master-detail relay is unavailable")
	ErrBrowserMasterDetailConsent      = errors.New("SmartAccounts browser master-detail transfer consent is required")
)

// BrowserMasterDetailFieldRule and BrowserMasterDetailContract mirror the
// private bridge's closed reviewed contract. They contain routes and field
// names only; source rows, browser state, headers, and credentials are never
// accepted by this API.
type BrowserMasterDetailFieldRule struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Enums    []string `json:"enums,omitempty"`
}

type BrowserMasterDetailContract struct {
	Version              string                         `json:"version"`
	Resource             string                         `json:"resource"`
	Origin               string                         `json:"origin"`
	ListPagePath         string                         `json:"list_page_path"`
	DetailPathPrefix     string                         `json:"detail_path_prefix"`
	DetailResultPagePath string                         `json:"detail_result_page_path"`
	Fields               []BrowserMasterDetailFieldRule `json:"fields"`
}

// BrowserMasterDetailScope is a current master-data snapshot boundary. Its
// dates are not a ledger history range: the source list is captured only as it
// exists on SnapshotDate in the SmartAccounts business timezone.
type BrowserMasterDetailScope struct {
	FromInclusive string `json:"from_inclusive"`
	ToInclusive   string `json:"to_inclusive"`
	CutoffAt      string `json:"cutoff_at"`
}

type BrowserMasterDetailTransferConsent struct {
	Version     string    `json:"version"`
	Confirmed   bool      `json:"confirmed"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// BrowserMasterDetailAuthorizeRequest is one owner action that authorizes the
// three fixed master-detail resources serially. The caller cannot choose
// contracts, schemas, paths, dates, or a financial action.
type BrowserMasterDetailAuthorizeRequest struct {
	SourceCompanyID          string `json:"source_company_id"`
	TransferConsentConfirmed bool   `json:"transfer_consent_confirmed"`
	// BatchID is returned from the first owner action and must be supplied for
	// an exact retry/resume of the three immutable runs. Refresh requests omit
	// it and deliberately create a new same-day snapshot generation.
	BatchID string `json:"batch_id,omitempty"`
	Refresh bool   `json:"refresh,omitempty"`
}

type BrowserMasterDetailResumeRequest struct {
	TransferConsentConfirmed bool `json:"transfer_consent_confirmed"`
}

type BrowserMasterDetailStartRequest struct {
	SourceCompanyID string                             `json:"source_company_id"`
	ManifestVersion string                             `json:"manifest_version"`
	ResourceID      string                             `json:"resource_id"`
	SchemaID        string                             `json:"schema_id"`
	Contract        BrowserMasterDetailContract        `json:"contract"`
	Scope           BrowserMasterDetailScope           `json:"scope"`
	ApprovalSHA256  string                             `json:"approval_sha256"`
	Consent         BrowserMasterDetailTransferConsent `json:"transfer_consent"`
}

// BrowserMasterDetailAuthorization stores only the digest of the capability.
// The contract, scope, source, and resource are immutable once stored.
type BrowserMasterDetailAuthorization struct {
	RunID           string
	TenantID        string
	BatchID         string
	SourceCompanyID string
	SnapshotDate    string
	ManifestVersion string
	ResourceID      string
	SchemaID        string
	SourceSchema    string
	Contract        BrowserMasterDetailContract
	ContractSHA256  string
	ApprovalSHA256  string
	Scope           BrowserMasterDetailScope
	TokenSHA256     string
	CreatedBy       string
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

// BrowserMasterDetailIssue is returned only in the immediate owner action
// response. The browser relay keeps CaptureToken in memory and must process
// issues in Sequence order; owner status endpoints never return it.
type BrowserMasterDetailIssue struct {
	RunID           string                             `json:"run_id"`
	TenantID        string                             `json:"tenant_id"`
	SourceCompanyID string                             `json:"source_company_id"`
	ManifestVersion string                             `json:"manifest_version"`
	ResourceID      string                             `json:"resource_id"`
	SchemaID        string                             `json:"schema_id"`
	SourceSchema    string                             `json:"source_schema"`
	Contract        BrowserMasterDetailContract        `json:"contract"`
	ContractSHA256  string                             `json:"contract_sha256"`
	ApprovalSHA256  string                             `json:"approval_sha256"`
	Scope           BrowserMasterDetailScope           `json:"scope"`
	SnapshotPolicy  string                             `json:"snapshot_policy"`
	SnapshotDate    string                             `json:"snapshot_date"`
	ExpiresAt       time.Time                          `json:"expires_at"`
	TransferConsent BrowserMasterDetailTransferConsent `json:"transfer_consent"`
	CaptureToken    string                             `json:"capture_token"`
	Sequence        int                                `json:"sequence"`
}

type BrowserMasterDetailIssueSet struct {
	BatchID string                     `json:"batch_id"`
	Issues  []BrowserMasterDetailIssue `json:"issues"`
}

// BrowserMasterDetailStatus is safe for either the owner (without a token) or
// the extension (with a scoped token). It omits source paths/rows, the review
// contract, raw NDJSON, and capability material.
type BrowserMasterDetailStatus struct {
	RunID           string                   `json:"run_id"`
	TenantID        string                   `json:"tenant_id"`
	SourceCompanyID string                   `json:"source_company_id"`
	ManifestVersion string                   `json:"manifest_version"`
	ResourceID      string                   `json:"resource_id"`
	SchemaID        string                   `json:"schema_id"`
	SourceSchema    string                   `json:"source_schema"`
	ContractSHA256  string                   `json:"contract_sha256"`
	ApprovalSHA256  string                   `json:"approval_sha256"`
	Scope           BrowserMasterDetailScope `json:"scope"`
	SnapshotPolicy  string                   `json:"snapshot_policy"`
	SnapshotDate    string                   `json:"snapshot_date"`
	Status          string                   `json:"status"`
	NDJSONSHA256    string                   `json:"ndjson_sha256,omitempty"`
	RecordCount     int                      `json:"record_count,omitempty"`
	PackageID       string                   `json:"package_id,omitempty"`
	PackageSHA256   string                   `json:"package_sha256,omitempty"`
}

type BrowserMasterDetailUploadResult struct {
	RunID   string `json:"run_id"`
	Status  string `json:"status"`
	Created bool   `json:"created"`
}

type BrowserMasterDetailStore interface {
	FindOrCreateBrowserMasterDetailAuthorization(context.Context, BrowserMasterDetailAuthorization) (*BrowserMasterDetailAuthorization, bool, error)
	GetBrowserMasterDetailAuthorization(context.Context, string, string) (*BrowserMasterDetailAuthorization, error)
	RotateBrowserMasterDetailAuthorization(context.Context, BrowserMasterDetailAuthorization) error
}

type BrowserMasterDetailBridge interface {
	StartBrowserMasterDetail(context.Context, string, string, BrowserMasterDetailStartRequest) (BrowserMasterDetailStatus, error)
	GetBrowserMasterDetail(context.Context, string, string) (BrowserMasterDetailStatus, error)
	UploadBrowserMasterDetail(context.Context, string, string, string, []byte) (BrowserMasterDetailUploadResult, error)
	FinalizeBrowserMasterDetail(context.Context, string, string) (BrowserMasterDetailStatus, error)
}

// BrowserMasterDetailStagedPackageVerifier is a deliberately narrow,
// read-only boundary to OA's internal package receiver. The bridge sealing a
// package is not enough: only this verifier may promote safe status to
// STAGED_REVIEW_REQUIRED after checking the exact tenant/source/package/digest
// tuple retained by OA.
type BrowserMasterDetailStagedPackageVerifier interface {
	VerifyStagedPackage(context.Context, string, string, string, string) error
}

// BrowserMasterDetailStagedPackageVerifierFunc adapts application wiring
// without teaching this browser-only package about tenant schema resolution.
type BrowserMasterDetailStagedPackageVerifierFunc func(context.Context, string, string, string, string) error

func (f BrowserMasterDetailStagedPackageVerifierFunc) VerifyStagedPackage(ctx context.Context, tenantID, sourceCompanyID, packageID, packageSHA256 string) error {
	return f(ctx, tenantID, sourceCompanyID, packageID, packageSHA256)
}

// BrowserMasterDetailService is intentionally non-financial. Its public
// dependency surface contains no journal, invoice, payment, or product writer.
type BrowserMasterDetailService struct {
	store    BrowserMasterDetailStore
	controls BrowserCaptureControlReader
	bridge   BrowserMasterDetailBridge
	staging  BrowserMasterDetailStagedPackageVerifier
	now      func() time.Time
	newRun   func() string
	newToken func() (string, error)
	location *time.Location
}

func NewBrowserMasterDetailService(store BrowserMasterDetailStore, controls BrowserCaptureControlReader, bridge BrowserMasterDetailBridge) *BrowserMasterDetailService {
	location, err := time.LoadLocation("Europe/Tallinn")
	if err != nil {
		location = time.FixedZone("Europe/Tallinn", 3*60*60)
	}
	return &BrowserMasterDetailService{store: store, controls: controls, bridge: bridge, now: time.Now, newRun: uuid.NewString, newToken: newBrowserCaptureToken, location: location}
}

// SetStagedPackageVerifier connects the package-delivery receiver only after
// application bootstrap has created it. No browser token, source evidence, or
// financial writer is exposed through this seam.
func (s *BrowserMasterDetailService) SetStagedPackageVerifier(verifier BrowserMasterDetailStagedPackageVerifier) *BrowserMasterDetailService {
	if s != nil {
		s.staging = verifier
	}
	return s
}

func (s *BrowserMasterDetailService) Authorize(ctx context.Context, tenantID, actor string, request BrowserMasterDetailAuthorizeRequest) (*BrowserMasterDetailIssueSet, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || s.newRun == nil || s.newToken == nil || s.location == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !request.TransferConsentConfirmed || !validBrowserSourceCompanyID(strings.TrimSpace(request.SourceCompanyID)) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	tenantID, actor, request.SourceCompanyID = strings.TrimSpace(tenantID), strings.TrimSpace(actor), strings.TrimSpace(request.SourceCompanyID)
	control, err := s.controls.Get(ctx, tenantID, request.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	now := s.currentTime()
	batchID := strings.TrimSpace(request.BatchID)
	if request.Refresh && batchID != "" {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	if batchID == "" {
		batchID = s.newRun()
	}
	if !validBrowserPairingID(batchID) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	snapshotDate := now.In(s.location).Format(time.DateOnly)
	scope := BrowserMasterDetailScope{FromInclusive: snapshotDate, ToInclusive: snapshotDate, CutoffAt: now.Format(time.RFC3339)}
	issues := make([]BrowserMasterDetailIssue, 0, 3)
	for index, resourceID := range browserMasterDetailResourceIDs() {
		issue, err := s.issueResource(ctx, tenantID, actor, batchID, request.SourceCompanyID, resourceID, snapshotDate, scope, now, index+1)
		if err != nil {
			return nil, err
		}
		issues = append(issues, *issue)
	}
	return &BrowserMasterDetailIssueSet{BatchID: batchID, Issues: issues}, nil
}

func (s *BrowserMasterDetailService) Resume(ctx context.Context, tenantID, actor, runID string, request BrowserMasterDetailResumeRequest) (*BrowserMasterDetailIssue, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || s.newToken == nil || !request.TransferConsentConfirmed || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(strings.TrimSpace(runID)) {
		return nil, ErrBrowserMasterDetailConsent
	}
	auth, err := s.store.GetBrowserMasterDetailAuthorization(ctx, strings.TrimSpace(runID), strings.TrimSpace(tenantID))
	if err != nil || auth == nil || !validBrowserMasterDetailAuthorization(*auth) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	if control, err := s.controls.Get(ctx, auth.TenantID, auth.SourceCompanyID); err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	if status, err := s.bridge.GetBrowserMasterDetail(ctx, auth.TenantID, auth.RunID); err != nil || !sameBrowserMasterDetailStatus(status, auth) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	now := s.currentTime()
	auth.TokenSHA256, auth.CreatedBy, auth.ExpiresAt = browserCaptureTokenSHA256(token), strings.TrimSpace(actor), now.Add(browserMasterDetailLifetime)
	if err := s.store.RotateBrowserMasterDetailAuthorization(ctx, *auth); err != nil {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	return masterDetailIssue(auth, token, now, browserMasterDetailSequence(auth.ResourceID)), nil
}

func (s *BrowserMasterDetailService) OwnerStatus(ctx context.Context, tenantID, runID string) (BrowserMasterDetailStatus, error) {
	auth, err := s.ownerAuthorization(ctx, tenantID, runID)
	if err != nil {
		return BrowserMasterDetailStatus{}, err
	}
	status, err := s.bridge.GetBrowserMasterDetail(ctx, auth.TenantID, auth.RunID)
	if err != nil || !sameBrowserMasterDetailStatus(status, auth) {
		return BrowserMasterDetailStatus{}, ErrBrowserMasterDetailUnavailable
	}
	return s.ownerMasterDetailStatus(ctx, status, auth), nil
}

func (s *BrowserMasterDetailService) Status(ctx context.Context, tenantID, runID, token string) (BrowserMasterDetailStatus, error) {
	auth, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil {
		return BrowserMasterDetailStatus{}, err
	}
	status, err := s.bridge.GetBrowserMasterDetail(ctx, auth.TenantID, auth.RunID)
	if err != nil || !sameBrowserMasterDetailStatus(status, auth) {
		return BrowserMasterDetailStatus{}, ErrBrowserMasterDetailUnavailable
	}
	return s.ownerMasterDetailStatus(ctx, status, auth), nil
}

func (s *BrowserMasterDetailService) Upload(ctx context.Context, tenantID, runID, token, digest, contentType string, body []byte) (BrowserMasterDetailUploadResult, error) {
	auth, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil || contentType != "application/x-ndjson" || len(body) == 0 || len(body) > browserMasterDetailMaxBytes || !validSHA256(digest) {
		return BrowserMasterDetailUploadResult{}, ErrBrowserMasterDetailUnauthorized
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != strings.TrimSpace(digest) {
		return BrowserMasterDetailUploadResult{}, ErrBrowserMasterDetailUnauthorized
	}
	result, err := s.bridge.UploadBrowserMasterDetail(ctx, auth.TenantID, auth.RunID, digest, body)
	if err != nil || result.RunID != auth.RunID || result.Status != "accepted" {
		return BrowserMasterDetailUploadResult{}, ErrBrowserMasterDetailUnavailable
	}
	return result, nil
}

func (s *BrowserMasterDetailService) Finalize(ctx context.Context, tenantID, runID, token string) (BrowserMasterDetailStatus, error) {
	auth, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil {
		return BrowserMasterDetailStatus{}, err
	}
	status, err := s.bridge.FinalizeBrowserMasterDetail(ctx, auth.TenantID, auth.RunID)
	if err != nil || !sameBrowserMasterDetailStatus(status, auth) {
		return BrowserMasterDetailStatus{}, ErrBrowserMasterDetailUnavailable
	}
	return s.ownerMasterDetailStatus(ctx, status, auth), nil
}

func (s *BrowserMasterDetailService) issueResource(ctx context.Context, tenantID, actor, batchID, source, resourceID, snapshotDate string, scope BrowserMasterDetailScope, now time.Time, sequence int) (*BrowserMasterDetailIssue, error) {
	contract, schema, sourceSchema, ok := browserMasterDetailContractFor(resourceID)
	if !ok {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	contractSHA, err := browserMasterDetailSHA256(contract)
	if err != nil {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	candidate := BrowserMasterDetailAuthorization{RunID: s.newRun(), TenantID: tenantID, BatchID: batchID, SourceCompanyID: source, SnapshotDate: snapshotDate, ManifestVersion: BrowserMasterDetailManifestVersion, ResourceID: resourceID, SchemaID: schema, SourceSchema: sourceSchema, Contract: contract, ContractSHA256: contractSHA, ApprovalSHA256: browserMasterDetailApprovalSHA256(tenantID, source, resourceID, snapshotDate, contractSHA, actor, now), Scope: scope, TokenSHA256: browserCaptureTokenSHA256(token), CreatedBy: actor, ExpiresAt: now.Add(browserMasterDetailLifetime), CreatedAt: now}
	if !validBrowserMasterDetailAuthorization(candidate) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	persisted, _, err := s.store.FindOrCreateBrowserMasterDetailAuthorization(ctx, candidate)
	if err != nil || persisted == nil || !validBrowserMasterDetailAuthorization(*persisted) || !sameBrowserMasterDetailImmutable(*persisted, candidate) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	if persisted.RunID != candidate.RunID {
		persisted.TokenSHA256, persisted.CreatedBy, persisted.ExpiresAt = browserCaptureTokenSHA256(token), actor, now.Add(browserMasterDetailLifetime)
		if err := s.store.RotateBrowserMasterDetailAuthorization(ctx, *persisted); err != nil {
			return nil, ErrBrowserMasterDetailUnavailable
		}
	}
	request := BrowserMasterDetailStartRequest{SourceCompanyID: persisted.SourceCompanyID, ManifestVersion: persisted.ManifestVersion, ResourceID: persisted.ResourceID, SchemaID: persisted.SchemaID, Contract: persisted.Contract, Scope: persisted.Scope, ApprovalSHA256: persisted.ApprovalSHA256, Consent: BrowserMasterDetailTransferConsent{Version: BrowserMasterDetailConsentVersion, Confirmed: true, ConfirmedAt: now}}
	status, err := s.bridge.StartBrowserMasterDetail(ctx, persisted.TenantID, persisted.RunID, request)
	if err != nil || !sameBrowserMasterDetailStatus(status, persisted) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	return masterDetailIssue(persisted, token, now, sequence), nil
}

func (s *BrowserMasterDetailService) ownerAuthorization(ctx context.Context, tenantID, runID string) (*BrowserMasterDetailAuthorization, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(strings.TrimSpace(runID)) {
		return nil, ErrBrowserMasterDetailUnavailable
	}
	auth, err := s.store.GetBrowserMasterDetailAuthorization(ctx, strings.TrimSpace(runID), strings.TrimSpace(tenantID))
	if err != nil || auth == nil || !validBrowserMasterDetailAuthorization(*auth) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	if control, err := s.controls.Get(ctx, auth.TenantID, auth.SourceCompanyID); err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	return auth, nil
}

func (s *BrowserMasterDetailService) authorize(ctx context.Context, tenantID, runID, token string) (*BrowserMasterDetailAuthorization, error) {
	auth, err := s.ownerAuthorization(ctx, tenantID, runID)
	if err != nil || !validBrowserCaptureToken(token) || !auth.ExpiresAt.After(s.currentTime()) || auth.TokenSHA256 != browserCaptureTokenSHA256(token) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	return auth, nil
}

func (s *BrowserMasterDetailService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func browserMasterDetailResourceIDs() []string {
	return []string{BrowserMasterDetailClientsResource, BrowserMasterDetailVendorsResource, BrowserMasterDetailArticlesResource}
}

func browserMasterDetailSequence(resourceID string) int {
	for index, candidate := range browserMasterDetailResourceIDs() {
		if candidate == resourceID {
			return index + 1
		}
	}
	return 0
}

func browserMasterDetailContractFor(resource string) (BrowserMasterDetailContract, string, string, bool) {
	party := func() []BrowserMasterDetailFieldRule {
		// Exact private v1 reviewed order. The stable source identity is the
		// browser-observed detail link, rather than an edit-form field.
		return []BrowserMasterDetailFieldRule{{"name", "string", true, nil}, {"regCode", "string", false, nil}, {"vatNumber", "string", false, nil}, {"address", "object", false, nil}, {"countrySubmittedInputValue", "string", false, nil}}
	}
	switch resource {
	case BrowserMasterDetailClientsResource:
		return BrowserMasterDetailContract{Version: BrowserMasterDetailManifestVersion, Resource: resource, Origin: "https://sa.smartaccounts.eu", ListPagePath: "/et/clients", DetailPathPrefix: "/et/clients.change/", DetailResultPagePath: "/et/clients", Fields: party()}, "clients_detail_v1", BrowserMasterDetailClientsSchema, true
	case BrowserMasterDetailVendorsResource:
		return BrowserMasterDetailContract{Version: BrowserMasterDetailManifestVersion, Resource: resource, Origin: "https://sa.smartaccounts.eu", ListPagePath: "/et/vendors", DetailPathPrefix: "/et/vendors.change/", DetailResultPagePath: "/et/vendors", Fields: party()}, "vendors_detail_v1", BrowserMasterDetailVendorsSchema, true
	case BrowserMasterDetailArticlesResource:
		fields := []BrowserMasterDetailFieldRule{{"code", "string", true, nil}, {"description", "string", false, nil}, {"type", "string", true, []string{"PRODUCT", "SERVICE", "WH"}}, {"unit", "string", false, nil}, {"priceSales", "decimal", false, nil}}
		return BrowserMasterDetailContract{Version: BrowserMasterDetailManifestVersion, Resource: resource, Origin: "https://sa.smartaccounts.eu", ListPagePath: "/et/articles", DetailPathPrefix: "/et/articles.change/", DetailResultPagePath: "/et/articles", Fields: fields}, "articles_detail_v1", BrowserMasterDetailArticlesSchema, true
	}
	return BrowserMasterDetailContract{}, "", "", false
}

func browserMasterDetailSHA256(contract BrowserMasterDetailContract) (string, error) {
	encoded, err := json.Marshal(contract)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func browserMasterDetailApprovalSHA256(tenant, source, resource, snapshot, contract, actor string, now time.Time) string {
	encoded, _ := json.Marshal(struct {
		Version, TenantID, SourceCompanyID, ResourceID, SnapshotDate, ContractSHA256, Actor string
		ReviewedAt                                                                          string
	}{BrowserMasterDetailReviewVersion, tenant, source, resource, snapshot, contract, actor, now.UTC().Format(time.RFC3339)})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func validBrowserMasterDetailAuthorization(value BrowserMasterDetailAuthorization) bool {
	contract, schema, sourceSchema, ok := browserMasterDetailContractFor(value.ResourceID)
	if !ok || !validBrowserPairingID(value.RunID) || !safeBridgeID(value.TenantID) || !validBrowserPairingID(value.BatchID) || !validBrowserSourceCompanyID(value.SourceCompanyID) || value.ManifestVersion != BrowserMasterDetailManifestVersion || value.SchemaID != schema || value.SourceSchema != sourceSchema || !sameBrowserMasterDetailContract(value.Contract, contract) || !validSHA256(value.ContractSHA256) || !validSHA256(value.ApprovalSHA256) || !validSHA256(value.TokenSHA256) || !validBrowserMasterDetailScope(value.Scope) || value.SnapshotDate != value.Scope.FromInclusive || value.SnapshotDate != value.Scope.ToInclusive || !value.ExpiresAt.After(value.CreatedAt) || value.CreatedAt.IsZero() || strings.TrimSpace(value.CreatedBy) == "" {
		return false
	}
	contractSHA, err := browserMasterDetailSHA256(value.Contract)
	return err == nil && contractSHA == value.ContractSHA256
}

func validBrowserMasterDetailStartRequest(value BrowserMasterDetailStartRequest) bool {
	contract, schema, _, ok := browserMasterDetailContractFor(value.ResourceID)
	return ok && validBrowserSourceCompanyID(strings.TrimSpace(value.SourceCompanyID)) && value.ManifestVersion == BrowserMasterDetailManifestVersion && value.SchemaID == schema && sameBrowserMasterDetailContract(value.Contract, contract) && validBrowserMasterDetailScope(value.Scope) && validSHA256(value.ApprovalSHA256) && value.Consent.Version == BrowserMasterDetailConsentVersion && value.Consent.Confirmed && !value.Consent.ConfirmedAt.IsZero()
}

func validBrowserMasterDetailResourceSchema(resourceID, schemaID, sourceSchema string) bool {
	_, expectedSchemaID, expectedSourceSchema, found := browserMasterDetailContractFor(resourceID)
	return found && schemaID == expectedSchemaID && sourceSchema == expectedSourceSchema
}

func validBrowserMasterDetailScope(scope BrowserMasterDetailScope) bool {
	from, fromErr := time.Parse(time.DateOnly, strings.TrimSpace(scope.FromInclusive))
	to, toErr := time.Parse(time.DateOnly, strings.TrimSpace(scope.ToInclusive))
	_, cutoffErr := time.Parse(time.RFC3339, strings.TrimSpace(scope.CutoffAt))
	return fromErr == nil && toErr == nil && cutoffErr == nil && from.Equal(to)
}

func sameBrowserMasterDetailImmutable(left, right BrowserMasterDetailAuthorization) bool {
	// An exact batch retry occurs moments later, so its server clock cutoff may
	// differ. The persisted first scope remains authoritative and is sent to
	// the bridge; only the logical current-snapshot day must match here.
	return left.TenantID == right.TenantID && left.BatchID == right.BatchID && left.SourceCompanyID == right.SourceCompanyID && left.SnapshotDate == right.SnapshotDate && left.ManifestVersion == right.ManifestVersion && left.ResourceID == right.ResourceID && left.SchemaID == right.SchemaID && left.SourceSchema == right.SourceSchema && left.ContractSHA256 == right.ContractSHA256
}

func sameBrowserMasterDetailScope(left, right BrowserMasterDetailScope) bool {
	return left.FromInclusive == right.FromInclusive && left.ToInclusive == right.ToInclusive && left.CutoffAt == right.CutoffAt
}

func sameBrowserMasterDetailContract(left, right BrowserMasterDetailContract) bool {
	leftSHA, leftErr := browserMasterDetailSHA256(left)
	rightSHA, rightErr := browserMasterDetailSHA256(right)
	return leftErr == nil && rightErr == nil && leftSHA == rightSHA
}

func sameBrowserMasterDetailStatus(status BrowserMasterDetailStatus, auth *BrowserMasterDetailAuthorization) bool {
	// The private bridge intentionally returns only its safe run status. OA
	// validates the returned immutable identifiers and rehydrates tenant,
	// source, approval, scope, and snapshot from the durable authorization.
	if auth == nil || status.RunID != auth.RunID || status.ManifestVersion != auth.ManifestVersion || status.ResourceID != auth.ResourceID || status.SchemaID != auth.SchemaID || status.SourceSchema != auth.SourceSchema || status.ContractSHA256 != auth.ContractSHA256 || status.ApprovalSHA256 != "" || !sameBrowserMasterDetailScope(status.Scope, BrowserMasterDetailScope{}) || status.TenantID != "" || status.SourceCompanyID != "" || status.SnapshotPolicy != "" || status.SnapshotDate != "" {
		return false
	}
	if status.RecordCount < 0 || (status.NDJSONSHA256 != "" && !validSHA256(status.NDJSONSHA256)) || (status.PackageSHA256 != "" && !validSHA256(status.PackageSHA256)) || (status.PackageID != "" && !safeBridgeID(status.PackageID)) {
		return false
	}
	if status.Status == "open" {
		return status.PackageID == "" && status.PackageSHA256 == ""
	}
	return status.Status == "finalized" && status.NDJSONSHA256 != "" && status.RecordCount > 0 && status.PackageID != "" && status.PackageSHA256 != ""
}

func masterDetailIssue(auth *BrowserMasterDetailAuthorization, token string, now time.Time, sequence int) *BrowserMasterDetailIssue {
	if auth == nil {
		return nil
	}
	return &BrowserMasterDetailIssue{RunID: auth.RunID, TenantID: auth.TenantID, SourceCompanyID: auth.SourceCompanyID, ManifestVersion: auth.ManifestVersion, ResourceID: auth.ResourceID, SchemaID: auth.SchemaID, SourceSchema: auth.SourceSchema, Contract: auth.Contract, ContractSHA256: auth.ContractSHA256, ApprovalSHA256: auth.ApprovalSHA256, Scope: auth.Scope, SnapshotPolicy: BrowserMasterDetailSnapshotPolicy, SnapshotDate: auth.SnapshotDate, ExpiresAt: auth.ExpiresAt, TransferConsent: BrowserMasterDetailTransferConsent{Version: BrowserMasterDetailConsentVersion, Confirmed: true, ConfirmedAt: now.UTC().Truncate(time.Second)}, CaptureToken: token, Sequence: sequence}
}

func (s *BrowserMasterDetailService) ownerMasterDetailStatus(ctx context.Context, status BrowserMasterDetailStatus, auth *BrowserMasterDetailAuthorization) BrowserMasterDetailStatus {
	status.TenantID, status.SourceCompanyID = auth.TenantID, auth.SourceCompanyID
	status.SnapshotPolicy, status.SnapshotDate = BrowserMasterDetailSnapshotPolicy, auth.SnapshotDate
	status.ApprovalSHA256, status.Scope = auth.ApprovalSHA256, auth.Scope
	// Private bridge finalization seals an evidence/archive package. It becomes
	// staging-review-ready only after OA's receiver verifies the exact retained
	// tenant/source/package/digest tuple. A missing, stale, cross-tenant, or
	// hash-mismatched delivery remains evidence-only and never enables preview.
	if status.Status == "finalized" {
		if s != nil && s.staging != nil && s.staging.VerifyStagedPackage(ctx, auth.TenantID, auth.SourceCompanyID, status.PackageID, status.PackageSHA256) == nil {
			status.Status = "STAGED_REVIEW_REQUIRED"
		} else {
			status.Status = "finalized_archived_evidence"
		}
	}
	return status
}
