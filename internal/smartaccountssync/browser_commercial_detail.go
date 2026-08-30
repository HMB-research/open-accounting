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
	BrowserCommercialDetailManifestVersion = "smartaccounts-browser-commercial-detail-v1"
	BrowserCommercialDetailWorkflowVersion = "smartaccounts-browser-commercial-detail-workflow-v1"
	BrowserCommercialDetailConsentVersion  = "smartaccounts-browser-commercial-transfer-consent-v1"
	BrowserCommercialDetailReviewVersion   = "smartaccounts-browser-commercial-schema-review-v1"
	BrowserCommercialDetailListSelector    = "list_selector_required"

	BrowserCommercialClientInvoicesResource = "client_invoices"
	BrowserCommercialBankPaymentsResource   = "bank_payments"

	browserCommercialDetailLifetime = 10 * time.Minute
)

var (
	ErrBrowserCommercialDetailUnauthorized = errors.New("SmartAccounts browser commercial-detail authorization is invalid, expired, or not scoped to this request")
	ErrBrowserCommercialDetailUnavailable  = errors.New("SmartAccounts browser commercial-detail relay is unavailable")
	ErrBrowserCommercialDetailConsent      = errors.New("SmartAccounts browser commercial-detail transfer consent is required")
	ErrBrowserCommercialDetailBlocked      = errors.New("SmartAccounts commercial detail requires a reviewed visible list selector")
	ErrBrowserCommercialDetailRunNotFound  = errors.New("SmartAccounts browser commercial-detail bridge run has not been started")
)

// BrowserCommercialDetailFieldRule and BrowserCommercialDetailContract mirror
// the closed e4b1524 private contract. They are static reviewed metadata; no
// caller can select an arbitrary source endpoint or field projection.
type BrowserCommercialDetailFieldRule struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Enums    []string `json:"enums,omitempty"`
}

type BrowserCommercialDetailReview struct {
	Version    string    `json:"version"`
	Confirmed  bool      `json:"confirmed"`
	ReviewedAt time.Time `json:"reviewed_at"`
	AuditID    string    `json:"audit_id"`
}

type BrowserCommercialDetailContract struct {
	Version              string                             `json:"version"`
	Resource             string                             `json:"resource"`
	Origin               string                             `json:"origin"`
	ListPagePath         string                             `json:"list_page_path"`
	DetailPathPrefix     string                             `json:"detail_path_prefix"`
	StableIDField        string                             `json:"stable_id_field"`
	Fields               []BrowserCommercialDetailFieldRule `json:"fields"`
	RowStableIDField     string                             `json:"row_stable_id_field,omitempty"`
	RowFields            []BrowserCommercialDetailFieldRule `json:"row_fields,omitempty"`
	CommentPathPrefix    string                             `json:"comment_path_prefix,omitempty"`
	CommentStableIDField string                             `json:"comment_stable_id_field,omitempty"`
	CommentFields        []BrowserCommercialDetailFieldRule `json:"comment_fields,omitempty"`
	AttachmentPathPrefix string                             `json:"attachment_path_prefix,omitempty"`
	Review               BrowserCommercialDetailReview      `json:"review"`
}

type BrowserCommercialDetailScope struct {
	FromInclusive string `json:"from_inclusive"`
	ToInclusive   string `json:"to_inclusive"`
	CutoffAt      string `json:"cutoff_at"`
}

type BrowserCommercialDetailTransferConsent struct {
	Version     string    `json:"version"`
	Confirmed   bool      `json:"confirmed"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// BrowserCommercialDetailAuthorizeRequest deliberately requires a durable
// 081 batch and selected source. The server verifies both against the owner
// and source workflow; a caller cannot attach an arbitrary source to tenant.
type BrowserCommercialDetailAuthorizeRequest struct {
	BatchID                  string `json:"batch_id"`
	SourceCompanyID          string `json:"source_company_id"`
	TransferConsentConfirmed bool   `json:"transfer_consent_confirmed"`
}

type BrowserCommercialDetailResumeRequest struct {
	TransferConsentConfirmed bool `json:"transfer_consent_confirmed"`
}

type BrowserCommercialDetailStartRequest struct {
	SourceCompanyID string                                 `json:"source_company_id"`
	ManifestVersion string                                 `json:"manifest_version"`
	Contract        BrowserCommercialDetailContract        `json:"contract"`
	Scope           BrowserCommercialDetailScope           `json:"scope"`
	Consent         BrowserCommercialDetailTransferConsent `json:"transfer_consent"`
}

// BrowserCommercialDetailAuthorization persists only safe immutable bindings
// and digest/count control metadata. TokenSHA256 is the capability digest;
// raw tokens, contracts, source rows, names and amounts are never persisted.
type BrowserCommercialDetailAuthorization struct {
	RunID           string
	TenantID        string
	BatchID         string
	WorkflowID      string
	SourceCompanyID string
	ManifestVersion string
	ResourceID      string
	Sequence        int
	SchemaID        string
	SourceSchema    string
	ReviewAuditID   string
	ReviewedAt      time.Time
	ContractSHA256  string
	RouteSHA256     string
	ConsentSHA256   string
	Scope           BrowserCommercialDetailScope
	TokenSHA256     string
	Status          string
	NDJSONSHA256    string
	RecordCount     int
	ReviewRequired  int
	PackageID       string
	PackageSHA256   string
	// BridgeStartedAt is control metadata only. A nil value means the
	// extension has not yet made the documented status-first start request.
	BridgeStartedAt *time.Time
	CreatedBy       string
	ExpiresAt       time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// BrowserCommercialDetailIssue is action-response-only. Callers must forward
// it directly to the same-window extension and immediately drop CaptureToken.
type BrowserCommercialDetailIssue struct {
	RunID              string                                 `json:"run_id"`
	TenantID           string                                 `json:"tenant_id"`
	SourceCompanyID    string                                 `json:"source_company_id"`
	ManifestVersion    string                                 `json:"manifest_version"`
	ResourceID         string                                 `json:"resource_id"`
	SchemaID           string                                 `json:"schema_id"`
	SourceSchema       string                                 `json:"source_schema"`
	Contract           BrowserCommercialDetailContract        `json:"contract"`
	ContractSHA256     string                                 `json:"contract_sha256"`
	RouteSHA256        string                                 `json:"route_sha256"`
	ConsentSHA256      string                                 `json:"consent_sha256"`
	Scope              BrowserCommercialDetailScope           `json:"scope"`
	TransferConsent    BrowserCommercialDetailTransferConsent `json:"transfer_consent"`
	Workflow           BrowserCommercialDetailWorkflow        `json:"workflow"`
	ListSelectorStatus string                                 `json:"list_selector_status"`
	ExpiresAt          time.Time                              `json:"expires_at"`
	CaptureToken       string                                 `json:"capture_token"`
}

type BrowserCommercialDetailWorkflow struct {
	Version    string   `json:"version"`
	WorkflowID string   `json:"workflow_id"`
	Sequence   int      `json:"sequence"`
	Resources  []string `json:"resources"`
}

type BrowserCommercialDetailIssueSet struct {
	WorkflowID string                         `json:"workflow_id"`
	Issues     []BrowserCommercialDetailIssue `json:"issues"`
}

// BrowserCommercialDetailStatus is the sole durable/public status. It is
// deliberately count/digest-only and cannot imply stage, preview, apply, or
// full-claim eligibility.
type BrowserCommercialDetailStatus struct {
	RunID              string `json:"run_id"`
	WorkflowID         string `json:"workflow_id"`
	ManifestVersion    string `json:"manifest_version"`
	ResourceID         string `json:"resource_id"`
	Sequence           int    `json:"sequence"`
	SchemaID           string `json:"schema_id"`
	SourceSchema       string `json:"source_schema"`
	Status             string `json:"status"`
	ListSelectorStatus string `json:"list_selector_status"`
	RouteSHA256        string `json:"route_sha256"`
	ContractSHA256     string `json:"contract_sha256"`
	ConsentSHA256      string `json:"consent_sha256"`
	NDJSONSHA256       string `json:"ndjson_sha256,omitempty"`
	RecordCount        int    `json:"record_count"`
	ReviewRequired     int    `json:"review_required"`
	PackageID          string `json:"package_id,omitempty"`
	PackageSHA256      string `json:"package_sha256,omitempty"`
}

type BrowserCommercialDetailStore interface {
	FindOrCreateBrowserCommercialDetailAuthorization(context.Context, BrowserCommercialDetailAuthorization) (*BrowserCommercialDetailAuthorization, bool, error)
	GetBrowserCommercialDetailAuthorization(context.Context, string, string) (*BrowserCommercialDetailAuthorization, error)
	RotateBrowserCommercialDetailAuthorization(context.Context, BrowserCommercialDetailAuthorization) error
	SaveBrowserCommercialDetailStatus(context.Context, BrowserCommercialDetailAuthorization) error
}

type BrowserCommercialDetailBatchReader interface {
	GetBrowserBatchWorkflow(context.Context, string, string) (*BrowserBatchWorkflow, error)
	ListBrowserBatchSourceWorkflows(context.Context, string, string) ([]BrowserBatchSourceWorkflow, error)
}

type BrowserCommercialDetailBridge interface {
	StartBrowserCommercialDetail(context.Context, string, string, BrowserCommercialDetailStartRequest) (BrowserCommercialDetailStatus, error)
	GetBrowserCommercialDetail(context.Context, string, string) (BrowserCommercialDetailStatus, error)
}

type BrowserCommercialDetailService struct {
	store    BrowserCommercialDetailStore
	batches  BrowserCommercialDetailBatchReader
	bridge   BrowserCommercialDetailBridge
	now      func() time.Time
	newRun   func() string
	newToken func() (string, error)
}

func NewBrowserCommercialDetailService(store BrowserCommercialDetailStore, batches BrowserCommercialDetailBatchReader, bridge BrowserCommercialDetailBridge) *BrowserCommercialDetailService {
	return &BrowserCommercialDetailService{store: store, batches: batches, bridge: bridge, now: time.Now, newRun: uuid.NewString, newToken: newBrowserCaptureToken}
}

func (s *BrowserCommercialDetailService) Authorize(ctx context.Context, tenantID, actor string, request BrowserCommercialDetailAuthorizeRequest) (*BrowserCommercialDetailIssueSet, error) {
	if s == nil || s.store == nil || s.batches == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || strings.TrimSpace(actor) == "" || !validBrowserPairingID(strings.TrimSpace(request.BatchID)) || !validBrowserSourceCompanyID(strings.TrimSpace(request.SourceCompanyID)) {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	if !request.TransferConsentConfirmed {
		return nil, ErrBrowserCommercialDetailConsent
	}
	tenantID, actor, request.BatchID, request.SourceCompanyID = strings.TrimSpace(tenantID), strings.TrimSpace(actor), strings.TrimSpace(request.BatchID), strings.TrimSpace(request.SourceCompanyID)
	workflow, err := s.batches.GetBrowserBatchWorkflow(ctx, actor, request.BatchID)
	if err != nil || workflow == nil || !validBrowserBatchWorkflow(*workflow) || workflow.TransferConfirmedAt == nil || !validBrowserBatchTransferScope(workflow.TransferScope) {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	sources, err := s.batches.ListBrowserBatchSourceWorkflows(ctx, actor, request.BatchID)
	if err != nil {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	if !commercialSourceBoundToTenant(sources, request.BatchID, request.SourceCompanyID, tenantID) {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	now := s.currentTime().Truncate(time.Second)
	workflowID := browserCommercialWorkflowID(tenantID, request.BatchID, request.SourceCompanyID)
	scope := BrowserCommercialDetailScope{FromInclusive: workflow.TransferScope.FromInclusive, ToInclusive: workflow.TransferScope.ToInclusive, CutoffAt: workflow.TransferScope.CutoffAt}
	issues := make([]BrowserCommercialDetailIssue, 0, 2)
	for sequence, resource := range browserCommercialDetailResources() {
		issue, err := s.issue(ctx, tenantID, actor, request.BatchID, workflowID, request.SourceCompanyID, resource, sequence+1, scope, now)
		if err != nil {
			return nil, err
		}
		issues = append(issues, *issue)
	}
	return &BrowserCommercialDetailIssueSet{WorkflowID: workflowID, Issues: issues}, nil
}

func (s *BrowserCommercialDetailService) Resume(ctx context.Context, tenantID, actor, runID string, request BrowserCommercialDetailResumeRequest) (*BrowserCommercialDetailIssue, error) {
	if s == nil || !request.TransferConsentConfirmed {
		return nil, ErrBrowserCommercialDetailConsent
	}
	auth, err := s.ownerAuthorization(ctx, tenantID, actor, runID)
	if err != nil {
		return nil, err
	}
	// Sequence two is intentionally not opened until the first reviewed
	// resource has a future selector-complete terminal receipt. A capability
	// rotation for its dormant immutable run is still safe and resumable.
	if auth.Sequence == 1 && auth.BridgeStartedAt != nil {
		bridgeStatus, err := s.bridge.GetBrowserCommercialDetail(ctx, auth.TenantID, auth.RunID)
		if err != nil || !sameBrowserCommercialDetailBridgeStatus(bridgeStatus, auth) {
			return nil, ErrBrowserCommercialDetailUnavailable
		}
		if err := s.persistBridgeStatus(ctx, auth, bridgeStatus); err != nil {
			return nil, err
		}
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	now := s.currentTime().Truncate(time.Second)
	auth.TokenSHA256, auth.CreatedBy, auth.ExpiresAt, auth.UpdatedAt = browserCaptureTokenSHA256(token), strings.TrimSpace(actor), now.Add(browserCommercialDetailLifetime), now
	if err := s.store.RotateBrowserCommercialDetailAuthorization(ctx, *auth); err != nil {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	return browserCommercialDetailIssue(auth, token, now), nil
}

func (s *BrowserCommercialDetailService) OwnerStatus(ctx context.Context, tenantID, actor, runID string) (BrowserCommercialDetailStatus, error) {
	auth, err := s.ownerAuthorization(ctx, tenantID, actor, runID)
	if err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	if auth.Sequence != 1 {
		return browserCommercialDetailSafeStatus(auth), nil
	}
	if auth.BridgeStartedAt == nil {
		return browserCommercialDetailSafeStatus(auth), nil
	}
	status, err := s.bridge.GetBrowserCommercialDetail(ctx, auth.TenantID, auth.RunID)
	if err != nil || !sameBrowserCommercialDetailBridgeStatus(status, auth) {
		return BrowserCommercialDetailStatus{}, ErrBrowserCommercialDetailUnavailable
	}
	if err := s.persistBridgeStatus(ctx, auth, status); err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	return browserCommercialDetailSafeStatus(auth), nil
}

func (s *BrowserCommercialDetailService) Status(ctx context.Context, tenantID, runID, token string) (BrowserCommercialDetailStatus, error) {
	auth, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	if auth.Sequence != 1 {
		return browserCommercialDetailSafeStatus(auth), nil
	}
	if auth.BridgeStartedAt == nil {
		return BrowserCommercialDetailStatus{}, ErrBrowserCommercialDetailRunNotFound
	}
	status, err := s.bridge.GetBrowserCommercialDetail(ctx, auth.TenantID, auth.RunID)
	if err != nil || !sameBrowserCommercialDetailBridgeStatus(status, auth) {
		return BrowserCommercialDetailStatus{}, ErrBrowserCommercialDetailUnavailable
	}
	if err := s.persistBridgeStatus(ctx, auth, status); err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	return browserCommercialDetailSafeStatus(auth), nil
}

// Start is an extension-only, capability-bound replay of the immutable start
// envelope. It never accepts caller-selected fields and cannot reach source
// bytes; the returned state is still list_selector_required.
func (s *BrowserCommercialDetailService) Start(ctx context.Context, tenantID, runID, token string, provided BrowserCommercialDetailStartRequest) (BrowserCommercialDetailStatus, error) {
	auth, err := s.authorize(ctx, tenantID, runID, token)
	if err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	if auth.Sequence != 1 {
		return BrowserCommercialDetailStatus{}, ErrBrowserCommercialDetailBlocked
	}
	request, err := browserCommercialDetailStartRequest(auth)
	if err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	if !sameBrowserCommercialDetailStartRequest(provided, request) {
		return BrowserCommercialDetailStatus{}, ErrBrowserCommercialDetailUnauthorized
	}
	status, err := s.bridge.StartBrowserCommercialDetail(ctx, auth.TenantID, auth.RunID, request)
	if err != nil || !sameBrowserCommercialDetailBridgeStatus(status, auth) {
		return BrowserCommercialDetailStatus{}, ErrBrowserCommercialDetailUnavailable
	}
	if err := s.persistBridgeStatus(ctx, auth, status); err != nil {
		return BrowserCommercialDetailStatus{}, err
	}
	return browserCommercialDetailSafeStatus(auth), nil
}

// Upload and Finalize are intentionally blocked before any request body or
// private bridge call. The approved contract has no visible selector/pager,
// so no capability can reach source capture or package finalization yet.
func (s *BrowserCommercialDetailService) Upload(context.Context, string, string, string) error {
	return ErrBrowserCommercialDetailBlocked
}
func (s *BrowserCommercialDetailService) Finalize(context.Context, string, string, string) error {
	return ErrBrowserCommercialDetailBlocked
}

func (s *BrowserCommercialDetailService) issue(ctx context.Context, tenantID, actor, batchID, workflowID, source, resource string, sequence int, scope BrowserCommercialDetailScope, now time.Time) (*BrowserCommercialDetailIssue, error) {
	contract, schema, sourceSchema, routeSHA, ok := browserCommercialDetailContractFor(resource, BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: now, AuditID: browserCommercialReviewAuditID(workflowID, resource)})
	if !ok {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	contractSHA, err := browserCommercialDetailSHA256(contract)
	if err != nil {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	consent := BrowserCommercialDetailTransferConsent{Version: BrowserCommercialDetailConsentVersion, Confirmed: true, ConfirmedAt: now}
	consentSHA, err := browserCommercialDetailConsentSHA256(consent)
	if err != nil {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	candidate := BrowserCommercialDetailAuthorization{RunID: s.newRun(), TenantID: tenantID, BatchID: batchID, WorkflowID: workflowID, SourceCompanyID: source, ManifestVersion: BrowserCommercialDetailManifestVersion, ResourceID: resource, Sequence: sequence, SchemaID: schema, SourceSchema: sourceSchema, ReviewAuditID: contract.Review.AuditID, ReviewedAt: contract.Review.ReviewedAt, ContractSHA256: contractSHA, RouteSHA256: routeSHA, ConsentSHA256: consentSHA, Scope: scope, TokenSHA256: browserCaptureTokenSHA256(token), Status: BrowserCommercialDetailListSelector, CreatedBy: actor, ExpiresAt: now.Add(browserCommercialDetailLifetime), CreatedAt: now, UpdatedAt: now}
	if !validBrowserCommercialDetailAuthorization(candidate) {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	persisted, created, err := s.store.FindOrCreateBrowserCommercialDetailAuthorization(ctx, candidate)
	if err != nil || persisted == nil || !validBrowserCommercialDetailAuthorization(*persisted) || !sameBrowserCommercialDetailImmutable(*persisted, candidate) {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	if !created {
		persisted.TokenSHA256, persisted.CreatedBy, persisted.ExpiresAt, persisted.UpdatedAt = browserCaptureTokenSHA256(token), actor, now.Add(browserCommercialDetailLifetime), now
		if err := s.store.RotateBrowserCommercialDetailAuthorization(ctx, *persisted); err != nil {
			return nil, ErrBrowserCommercialDetailUnavailable
		}
	}
	return browserCommercialDetailIssue(persisted, token, now), nil
}

func (s *BrowserCommercialDetailService) ownerAuthorization(ctx context.Context, tenantID, actor, runID string) (*BrowserCommercialDetailAuthorization, error) {
	if s == nil || s.store == nil || s.batches == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || strings.TrimSpace(actor) == "" || !validBrowserPairingID(strings.TrimSpace(runID)) {
		return nil, ErrBrowserCommercialDetailUnavailable
	}
	auth, err := s.store.GetBrowserCommercialDetailAuthorization(ctx, strings.TrimSpace(runID), strings.TrimSpace(tenantID))
	if err != nil || auth == nil || !validBrowserCommercialDetailAuthorization(*auth) {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	if _, err := s.batches.GetBrowserBatchWorkflow(ctx, strings.TrimSpace(actor), auth.BatchID); err != nil {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	return auth, nil
}

func (s *BrowserCommercialDetailService) authorize(ctx context.Context, tenantID, runID, token string) (*BrowserCommercialDetailAuthorization, error) {
	if s == nil || s.store == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(strings.TrimSpace(runID)) || !validBrowserCaptureToken(token) {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	auth, err := s.store.GetBrowserCommercialDetailAuthorization(ctx, strings.TrimSpace(runID), strings.TrimSpace(tenantID))
	if err != nil || auth == nil || !validBrowserCommercialDetailAuthorization(*auth) || !auth.ExpiresAt.After(s.currentTime()) || auth.TokenSHA256 != browserCaptureTokenSHA256(token) {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	return auth, nil
}

func (s *BrowserCommercialDetailService) persistBridgeStatus(ctx context.Context, auth *BrowserCommercialDetailAuthorization, status BrowserCommercialDetailStatus) error {
	if auth == nil || !sameBrowserCommercialDetailBridgeStatus(status, auth) {
		return ErrBrowserCommercialDetailUnavailable
	}
	if status.Status == "finalized" {
		auth.Status = "finalized_archived_evidence"
	} else {
		auth.Status = BrowserCommercialDetailListSelector
	}
	auth.NDJSONSHA256, auth.RecordCount, auth.ReviewRequired, auth.PackageID, auth.PackageSHA256 = status.NDJSONSHA256, status.RecordCount, status.ReviewRequired, status.PackageID, status.PackageSHA256
	auth.UpdatedAt = s.currentTime()
	if auth.BridgeStartedAt == nil {
		startedAt := auth.UpdatedAt
		auth.BridgeStartedAt = &startedAt
	}
	if err := s.store.SaveBrowserCommercialDetailStatus(ctx, *auth); err != nil {
		return ErrBrowserCommercialDetailUnavailable
	}
	return nil
}

func (s *BrowserCommercialDetailService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func browserCommercialDetailResources() []string {
	return []string{BrowserCommercialClientInvoicesResource, BrowserCommercialBankPaymentsResource}
}
func browserCommercialDetailSequence(resource string) int {
	for index, value := range browserCommercialDetailResources() {
		if value == resource {
			return index + 1
		}
	}
	return 0
}

func browserCommercialDetailContractFor(resource string, review BrowserCommercialDetailReview) (BrowserCommercialDetailContract, string, string, string, bool) {
	var contract BrowserCommercialDetailContract
	var schema, sourceSchema, detailForm string
	switch resource {
	case BrowserCommercialClientInvoicesResource:
		schema, detailForm = "client_invoices_detail_v1", "/et/clientinvoices.clientinvoiceaddeditcomp.addeditform"
		contract = BrowserCommercialDetailContract{Version: BrowserCommercialDetailManifestVersion, Resource: resource, Origin: "https://sa.smartaccounts.eu", ListPagePath: "/et/clientinvoices", DetailPathPrefix: "/et/clientinvoices.change/", StableIDField: "invoiceNumber", Fields: []BrowserCommercialDetailFieldRule{{"clients", "string", false, nil}, {"contactName", "string", false, nil}, {"invoiceDate", "date", false, nil}, {"invoiceNumber", "string", true, nil}, {"invoiceDueDate", "date", false, nil}, {"invoiceInterest", "string", false, nil}, {"invoiceEntryDate", "date", false, nil}, {"invReferenceNumber", "string", false, nil}, {"invoiceCurrency", "string", false, nil}, {"branches2", "string", false, nil}, {"articles", "string", false, nil}, {"invoiceEntryDescription", "string", false, nil}, {"invoiceEntryQuantity", "string", false, nil}, {"invoiceEntryPrice", "string", false, nil}, {"invoiceEntryDiscountPc", "string", false, nil}, {"invoiceRoundAmount", "string", false, nil}, {"invoicePaymentMethod", "string", false, nil}, {"invoicePaymentAmountD", "string", false, nil}, {"selectedTemplate", "string", false, nil}, {"invoiceNote", "string", false, nil}, {"internalNote", "string", false, nil}}, Review: review}
	case BrowserCommercialBankPaymentsResource:
		schema, detailForm = "bank_payments_detail_v1", "/et/payments/bank.paymentslistcomp.paymentaddeditcomp.addeditform"
		contract = BrowserCommercialDetailContract{Version: BrowserCommercialDetailManifestVersion, Resource: resource, Origin: "https://sa.smartaccounts.eu", ListPagePath: "/et/payments/bank", DetailPathPrefix: "/et/payments/bank.paymentslistcomp.change/", StableIDField: "paymentDocument", Fields: []BrowserCommercialDetailFieldRule{{"vendors", "string", false, nil}, {"paymentBankAccount", "string", false, nil}, {"paymentDate", "date", false, nil}, {"paymentCurrency", "string", false, nil}, {"paymentDocument", "string", true, nil}, {"paymentExtraDescription", "string", false, nil}, {"paymentExtraQuantity", "string", false, nil}, {"paymentExtraPrice", "string", false, nil}}, Review: review}
	default:
		return BrowserCommercialDetailContract{}, "", "", "", false
	}
	sourceSchema = BrowserCommercialDetailManifestVersion + "/" + schema
	routeBytes, err := json.Marshal(struct{ ListPagePath, DetailPathPrefix, DetailFormPath string }{contract.ListPagePath, contract.DetailPathPrefix, detailForm})
	if err != nil {
		return BrowserCommercialDetailContract{}, "", "", "", false
	}
	sum := sha256.Sum256(routeBytes)
	return contract, schema, sourceSchema, hex.EncodeToString(sum[:]), true
}

func browserCommercialDetailSHA256(contract BrowserCommercialDetailContract) (string, error) {
	payload, err := json.Marshal(contract)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
func browserCommercialDetailConsentSHA256(consent BrowserCommercialDetailTransferConsent) (string, error) {
	payload, err := json.Marshal(consent)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
func browserCommercialWorkflowID(tenantID, batchID, sourceID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("smartaccounts-commercial-v1\x00"+tenantID+"\x00"+batchID+"\x00"+sourceID)).String()
}
func browserCommercialReviewAuditID(workflowID, resource string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("smartaccounts-commercial-review-v1\x00"+workflowID+"\x00"+resource)).String()
}

func validBrowserCommercialDetailAuthorization(value BrowserCommercialDetailAuthorization) bool {
	contract, schema, sourceSchema, routeSHA, ok := browserCommercialDetailContractFor(value.ResourceID, BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: value.ReviewedAt, AuditID: value.ReviewAuditID})
	if !ok || !validBrowserPairingID(value.RunID) || !safeBridgeID(value.TenantID) || !validBrowserPairingID(value.BatchID) || !validBrowserPairingID(value.WorkflowID) || !validBrowserSourceCompanyID(value.SourceCompanyID) || value.ManifestVersion != BrowserCommercialDetailManifestVersion || value.Sequence != browserCommercialDetailSequence(value.ResourceID) || value.SchemaID != schema || value.SourceSchema != sourceSchema || value.RouteSHA256 != routeSHA || !validBrowserCommercialDetailScope(value.Scope) || !validSHA256(value.ContractSHA256) || !validSHA256(value.ConsentSHA256) || !validSHA256(value.TokenSHA256) || !validBrowserPairingID(value.ReviewAuditID) || value.ReviewedAt.IsZero() || value.CreatedAt.IsZero() || !value.ExpiresAt.After(value.CreatedAt) || strings.TrimSpace(value.CreatedBy) == "" || !validBrowserCommercialDetailStoredStatus(value.Status) || value.RecordCount < 0 || value.ReviewRequired < 0 || value.ReviewRequired > value.RecordCount {
		return false
	}
	contractSHA, err := browserCommercialDetailSHA256(contract)
	return err == nil && contractSHA == value.ContractSHA256 && (value.NDJSONSHA256 == "" || validSHA256(value.NDJSONSHA256)) && (value.PackageSHA256 == "" || validSHA256(value.PackageSHA256)) && (value.PackageID == "" || safeBridgeID(value.PackageID))
}

func validBrowserCommercialDetailStoredStatus(status string) bool {
	return status == BrowserCommercialDetailListSelector || status == "open" || status == "finalized_archived_evidence"
}

func validBrowserCommercialDetailStartRequest(value BrowserCommercialDetailStartRequest) bool {
	contract, schema, _, _, found := browserCommercialDetailContractFor(value.Contract.Resource, value.Contract.Review)
	if !found || !validBrowserSourceCompanyID(strings.TrimSpace(value.SourceCompanyID)) || value.ManifestVersion != BrowserCommercialDetailManifestVersion || value.Contract.Version != BrowserCommercialDetailManifestVersion || value.Contract.Resource == "" || value.Contract.Review.Version != BrowserCommercialDetailReviewVersion || !value.Contract.Review.Confirmed || !validBrowserPairingID(value.Contract.Review.AuditID) || value.Contract.Review.ReviewedAt.IsZero() || !validBrowserCommercialDetailScope(value.Scope) || value.Consent.Version != BrowserCommercialDetailConsentVersion || !value.Consent.Confirmed || value.Consent.ConfirmedAt.IsZero() {
		return false
	}
	_, expectedSchema, _, _, _ := browserCommercialDetailContractFor(value.Contract.Resource, value.Contract.Review)
	return schema == expectedSchema && sameBrowserCommercialDetailContract(value.Contract, contract)
}

func validBrowserCommercialDetailResourceSchema(resourceID, schemaID, sourceSchema string) bool {
	_, expectedSchema, expectedSourceSchema, _, found := browserCommercialDetailContractFor(resourceID, BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: time.Unix(0, 0).UTC(), AuditID: uuid.NewString()})
	return found && schemaID == expectedSchema && sourceSchema == expectedSourceSchema
}

func sameBrowserCommercialDetailContract(left, right BrowserCommercialDetailContract) bool {
	leftSHA, leftErr := browserCommercialDetailSHA256(left)
	rightSHA, rightErr := browserCommercialDetailSHA256(right)
	return leftErr == nil && rightErr == nil && leftSHA == rightSHA
}

func sameBrowserCommercialDetailStartRequest(left, right BrowserCommercialDetailStartRequest) bool {
	leftContract, leftErr := browserCommercialDetailSHA256(left.Contract)
	rightContract, rightErr := browserCommercialDetailSHA256(right.Contract)
	leftConsent, leftConsentErr := browserCommercialDetailConsentSHA256(left.Consent)
	rightConsent, rightConsentErr := browserCommercialDetailConsentSHA256(right.Consent)
	return leftErr == nil && rightErr == nil && leftConsentErr == nil && rightConsentErr == nil && left.SourceCompanyID == right.SourceCompanyID && left.ManifestVersion == right.ManifestVersion && leftContract == rightContract && leftConsent == rightConsent && sameBrowserCommercialDetailScope(left.Scope, right.Scope)
}
func validBrowserCommercialDetailScope(scope BrowserCommercialDetailScope) bool {
	from, e1 := time.Parse(time.DateOnly, strings.TrimSpace(scope.FromInclusive))
	to, e2 := time.Parse(time.DateOnly, strings.TrimSpace(scope.ToInclusive))
	cutoff, e3 := time.Parse(time.RFC3339, strings.TrimSpace(scope.CutoffAt))
	return e1 == nil && e2 == nil && e3 == nil && !from.After(to) && cutoff.UTC().Format(time.DateOnly) == scope.ToInclusive
}
func sameBrowserCommercialDetailImmutable(left, right BrowserCommercialDetailAuthorization) bool {
	return left.TenantID == right.TenantID && left.BatchID == right.BatchID && left.WorkflowID == right.WorkflowID && left.SourceCompanyID == right.SourceCompanyID && left.ManifestVersion == right.ManifestVersion && left.ResourceID == right.ResourceID && left.Sequence == right.Sequence && left.SchemaID == right.SchemaID && left.SourceSchema == right.SourceSchema && left.ReviewAuditID == right.ReviewAuditID && left.ContractSHA256 == right.ContractSHA256 && left.RouteSHA256 == right.RouteSHA256 && sameBrowserCommercialDetailScope(left.Scope, right.Scope)
}
func sameBrowserCommercialDetailScope(left, right BrowserCommercialDetailScope) bool {
	return left.FromInclusive == right.FromInclusive && left.ToInclusive == right.ToInclusive && left.CutoffAt == right.CutoffAt
}

// Bridge response has no source, contract, scope, review, or raw token. OA
// validates only the exact safe documented fields then rehydrates safe status
// from the durable owner authorization.
func sameBrowserCommercialDetailBridgeStatus(status BrowserCommercialDetailStatus, auth *BrowserCommercialDetailAuthorization) bool {
	if auth == nil || status.RunID != auth.RunID || status.ManifestVersion != auth.ManifestVersion || status.ResourceID != auth.ResourceID || status.Sequence != 0 || status.WorkflowID != "" || status.SchemaID != auth.SchemaID || status.SourceSchema != auth.SourceSchema || status.RouteSHA256 != auth.RouteSHA256 || status.ContractSHA256 != auth.ContractSHA256 || status.ConsentSHA256 != auth.ConsentSHA256 || status.ListSelectorStatus != "" || status.RecordCount < 0 || status.ReviewRequired < 0 || status.ReviewRequired > status.RecordCount {
		return false
	}
	if status.Status == "open" {
		return status.NDJSONSHA256 == "" && status.RecordCount == 0 && status.ReviewRequired == 0 && status.PackageID == "" && status.PackageSHA256 == ""
	}
	return status.Status == "finalized" && validSHA256(status.NDJSONSHA256) && status.RecordCount > 0 && status.ReviewRequired == status.RecordCount && safeBridgeID(status.PackageID) && validSHA256(status.PackageSHA256)
}

func browserCommercialDetailIssue(auth *BrowserCommercialDetailAuthorization, token string, now time.Time) *BrowserCommercialDetailIssue {
	if auth == nil {
		return nil
	}
	contract, _, _, _, ok := browserCommercialDetailContractFor(auth.ResourceID, BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: auth.ReviewedAt, AuditID: auth.ReviewAuditID})
	if !ok {
		return nil
	}
	consent := BrowserCommercialDetailTransferConsent{Version: BrowserCommercialDetailConsentVersion, Confirmed: true, ConfirmedAt: auth.CreatedAt}
	return &BrowserCommercialDetailIssue{RunID: auth.RunID, TenantID: auth.TenantID, SourceCompanyID: auth.SourceCompanyID, ManifestVersion: auth.ManifestVersion, ResourceID: auth.ResourceID, SchemaID: auth.SchemaID, SourceSchema: auth.SourceSchema, Contract: contract, ContractSHA256: auth.ContractSHA256, RouteSHA256: auth.RouteSHA256, ConsentSHA256: auth.ConsentSHA256, Scope: auth.Scope, TransferConsent: consent, Workflow: BrowserCommercialDetailWorkflow{Version: BrowserCommercialDetailWorkflowVersion, WorkflowID: auth.WorkflowID, Sequence: auth.Sequence, Resources: browserCommercialDetailResources()}, ListSelectorStatus: BrowserCommercialDetailListSelector, ExpiresAt: auth.ExpiresAt, CaptureToken: token}
}

func browserCommercialDetailStartRequest(auth *BrowserCommercialDetailAuthorization) (BrowserCommercialDetailStartRequest, error) {
	if auth == nil {
		return BrowserCommercialDetailStartRequest{}, ErrBrowserCommercialDetailUnavailable
	}
	contract, _, _, _, ok := browserCommercialDetailContractFor(auth.ResourceID, BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: auth.ReviewedAt, AuditID: auth.ReviewAuditID})
	if !ok {
		return BrowserCommercialDetailStartRequest{}, ErrBrowserCommercialDetailUnavailable
	}
	return BrowserCommercialDetailStartRequest{SourceCompanyID: auth.SourceCompanyID, ManifestVersion: auth.ManifestVersion, Contract: contract, Scope: auth.Scope, Consent: BrowserCommercialDetailTransferConsent{Version: BrowserCommercialDetailConsentVersion, Confirmed: true, ConfirmedAt: auth.CreatedAt}}, nil
}

func browserCommercialDetailSafeStatus(auth *BrowserCommercialDetailAuthorization) BrowserCommercialDetailStatus {
	if auth == nil {
		return BrowserCommercialDetailStatus{}
	}
	return BrowserCommercialDetailStatus{RunID: auth.RunID, WorkflowID: auth.WorkflowID, ManifestVersion: auth.ManifestVersion, ResourceID: auth.ResourceID, Sequence: auth.Sequence, SchemaID: auth.SchemaID, SourceSchema: auth.SourceSchema, Status: auth.Status, ListSelectorStatus: BrowserCommercialDetailListSelector, RouteSHA256: auth.RouteSHA256, ContractSHA256: auth.ContractSHA256, ConsentSHA256: auth.ConsentSHA256, NDJSONSHA256: auth.NDJSONSHA256, RecordCount: auth.RecordCount, ReviewRequired: auth.ReviewRequired, PackageID: auth.PackageID, PackageSHA256: auth.PackageSHA256}
}

func commercialSourceBoundToTenant(sources []BrowserBatchSourceWorkflow, batchID, sourceID, tenantID string) bool {
	for _, source := range sources {
		if source.BatchID == batchID && source.SourceCompanyID == sourceID && source.TenantID == tenantID {
			return true
		}
	}
	return false
}
