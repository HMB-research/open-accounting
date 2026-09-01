package smartaccountssync

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// The manifest currently contains 31 observed pages. The locally installed
	// relay visits them sequentially and may wait for a guarded source-page
	// transition per resource, so two minutes would cause normal, safe runs to
	// expire before completion. Ten minutes is the extension's independently
	// enforced maximum and is still bounded by fresh, action-time owner consent.
	browserDiscoveryLifetime    = 10 * time.Minute
	browserDiscoveryRelaySource = "smartaccounts-browser-relay"
	browserDiscoveryRelayEvent  = "smartaccounts-browser-relay.discovery-result.v1"
)

var (
	ErrBrowserDiscoveryUnauthorized = errors.New("SmartAccounts browser discovery authorization is invalid or not scoped to this request")
	ErrBrowserDiscoveryConsent      = errors.New("SmartAccounts browser discovery requires explicit owner consent")
	ErrBrowserDiscoveryInvalid      = errors.New("SmartAccounts browser discovery result is invalid")
	ErrBrowserDiscoveryUnavailable  = errors.New("SmartAccounts browser discovery is unavailable")
	ErrBrowserDiscoveryNotFound     = errors.New("SmartAccounts browser discovery receipt was not found")
	ErrBrowserDiscoveryConflict     = errors.New("SmartAccounts browser discovery receipt conflicts with its immutable authorization")
)

// BrowserDiscoveryAuthorization is metadata-only action-time authorization.
// It records no relay token, cookie, browser state, source record, export
// body, header value, URL query, or private discovery contract.
type BrowserDiscoveryAuthorization struct {
	DiscoveryID                  string
	TenantID                     string
	SourceCompanyID              string
	ManifestVersion              string
	ContractVersion              string
	ResourceIDs                  []string
	MetadataOnlyConsentConfirmed bool
	HeaderProbeConsentConfirmed  bool
	ConsentedAt                  time.Time
	CreatedBy                    string
	ExpiresAt                    time.Time
	CreatedAt                    time.Time
}

// BrowserDiscoveryReceipt is the only bridge response OA persists. It is a
// digest and aggregate counts, never the per-resource discovery contract.
type BrowserDiscoveryReceipt struct {
	DiscoveryID           string `json:"discovery_id"`
	Status                string `json:"status"`
	ManifestVersion       string `json:"manifest_version"`
	ContractVersion       string `json:"contract_version"`
	ContractSHA256        string `json:"contract_sha256"`
	ResourceCount         int    `json:"resource_count"`
	CaptureReadyCount     int    `json:"capture_ready_count"`
	FilterRequiredCount   int    `json:"filter_contract_required_count"`
	PageOnlyRequiredCount int    `json:"page_only_contract_required_count"`
	PrivateEndpointCount  int    `json:"private_endpoint_required_count"`
	BindingBlockedCount   int    `json:"binding_blocked_count"`
}

// BrowserDiscoveryIssue is sent once from the owner page directly to the
// locally installed relay with window.postMessage. It contains only the
// tenant/source/discovery binding and action-time consent, not a capability.
type BrowserDiscoveryIssue struct {
	DiscoveryID      string                  `json:"discovery_id"`
	TenantID         string                  `json:"tenant_id"`
	SourceCompanyID  string                  `json:"source_company_id"`
	ManifestVersion  string                  `json:"manifest_version"`
	ResourceIDs      []string                `json:"resource_ids"`
	ExpiresAt        time.Time               `json:"expires_at"`
	DiscoveryConsent BrowserDiscoveryConsent `json:"discovery_consent"`
}

// BrowserDiscoveryConsent distinguishes metadata-only discovery from the
// separately approved bounded response-header probe. The latter never allows
// a response body, header value, source row, cookie, or credential.
type BrowserDiscoveryConsent struct {
	Version                      int       `json:"version"`
	Confirmed                    bool      `json:"confirmed"`
	ConfirmedAt                  time.Time `json:"confirmed_at"`
	Scope                        string    `json:"scope"`
	ResponseHeaderProbeConfirmed bool      `json:"response_header_probe_confirmed,omitempty"`
}

// BrowserDiscoveryStartRequest intentionally has no caller-chosen discovery
// UUID, target tenant, raw source data, URL, resource contract, or token. OA
// derives the one currently proven resource surface from its static manifest.
type BrowserDiscoveryStartRequest struct {
	SourceCompanyID              string `json:"source_company_id"`
	MetadataOnlyConsentConfirmed bool   `json:"metadata_only_consent_confirmed"`
	ResponseHeaderProbeConfirmed bool   `json:"response_header_probe_confirmed"`
}

// BrowserDiscoveryRelayResult is the strict same-window relay event consumed
// by the OA page and proxied with the owner's normal authenticated request.
// The source selector is deliberately absent; OA gets it only from the
// persisted authorization before it contacts the private bridge.
type BrowserDiscoveryRelayResult struct {
	Source          string                     `json:"source"`
	Type            string                     `json:"type"`
	Version         int                        `json:"version"`
	DiscoveryID     string                     `json:"discovery_id"`
	ManifestVersion string                     `json:"manifest_version"`
	ContractVersion string                     `json:"contract_version"`
	Status          string                     `json:"status"`
	Resources       []BrowserDiscoveryResource `json:"resources"`
}

// BrowserDiscoveryBridgeReceiptRequest is sent only over the internal bridge
// HMAC connection. It is the exact redacted relay result plus the source
// binding resolved by OA; it is never emitted to a browser response.
type BrowserDiscoveryBridgeReceiptRequest struct {
	SourceCompanyID string                     `json:"source_company_id"`
	ManifestVersion string                     `json:"manifest_version"`
	ContractVersion string                     `json:"contract_version"`
	Status          string                     `json:"status"`
	Resources       []BrowserDiscoveryResource `json:"resources"`
}

type BrowserDiscoveryResource struct {
	ResourceID    string                           `json:"resource_id"`
	CaptureStatus string                           `json:"capture_status"`
	Binding       BrowserDiscoveryBinding          `json:"binding"`
	Contract      BrowserDiscoveryResourceContract `json:"contract"`
}

type BrowserDiscoveryBinding struct {
	Session string `json:"session"`
	Company string `json:"company"`
	Page    string `json:"page"`
}

type BrowserDiscoveryResourceContract struct {
	Version    string                           `json:"version"`
	PagePath   string                           `json:"page_path"`
	Request    *BrowserDiscoveryRequestContract `json:"request,omitempty"`
	Filter     *BrowserDiscoveryFilterContract  `json:"filter,omitempty"`
	Pagination BrowserDiscoveryPagination       `json:"pagination"`
	Response   BrowserDiscoveryResponseContract `json:"response"`
}

type BrowserDiscoveryRequestContract struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type BrowserDiscoveryFilterContract struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	ControlIDs []string `json:"control_ids"`
}

type BrowserDiscoveryPagination struct {
	Kind       string   `json:"kind"`
	ControlIDs []string `json:"control_ids"`
}

type BrowserDiscoveryResponseContract struct {
	Observation string   `json:"observation"`
	ContentType string   `json:"content_type"`
	HeaderNames []string `json:"header_names"`
}

type BrowserDiscoveryStore interface {
	CreateBrowserDiscoveryAuthorization(context.Context, BrowserDiscoveryAuthorization) error
	GetBrowserDiscoveryAuthorization(context.Context, string, string) (*BrowserDiscoveryAuthorization, error)
	SaveBrowserDiscoveryReceipt(context.Context, string, string, BrowserDiscoveryReceipt, time.Time) error
}

// BrowserDiscoveryControlReader narrows the existing sync controls to the
// only lookup discovery needs: an already claimed tenant/browser source.
type BrowserDiscoveryControlReader interface {
	Get(context.Context, string, string) (*Control, error)
}

type BrowserDiscoveryBridge interface {
	RecordBrowserDiscoveryReceipt(context.Context, string, string, string, BrowserDiscoveryBridgeReceiptRequest) (BrowserDiscoveryReceipt, error)
	GetBrowserDiscoveryReceipt(context.Context, string, string, string) (BrowserDiscoveryReceipt, error)
}

// BrowserDiscoveryService owns action-time consent and safe bridge receipt
// projection. It deliberately has no capture, package, executor, or ledger
// dependency, so a discovery receipt cannot start a transfer or financial
// apply operation.
type BrowserDiscoveryService struct {
	store    BrowserDiscoveryStore
	controls BrowserDiscoveryControlReader
	bridge   BrowserDiscoveryBridge
	now      func() time.Time
	newID    func() string
}

func NewBrowserDiscoveryService(store BrowserDiscoveryStore, controls BrowserDiscoveryControlReader, bridge BrowserDiscoveryBridge) *BrowserDiscoveryService {
	return &BrowserDiscoveryService{store: store, controls: controls, bridge: bridge, now: time.Now, newID: uuid.NewString}
}

func (s *BrowserDiscoveryService) Issue(ctx context.Context, tenantID, actor string, request BrowserDiscoveryStartRequest) (*BrowserDiscoveryIssue, error) {
	if s == nil || s.store == nil || s.controls == nil || s.newID == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserDiscoveryStartRequest(request) {
		return nil, ErrBrowserDiscoveryUnavailable
	}
	if !request.MetadataOnlyConsentConfirmed {
		return nil, ErrBrowserDiscoveryConsent
	}
	tenantID = strings.TrimSpace(tenantID)
	request.SourceCompanyID = strings.TrimSpace(request.SourceCompanyID)
	control, err := s.controls.Get(ctx, tenantID, request.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	discoveryID := s.newID()
	if !validBrowserDiscoveryID(discoveryID) {
		return nil, ErrBrowserDiscoveryUnavailable
	}
	now := s.currentTime()
	authorization := BrowserDiscoveryAuthorization{
		DiscoveryID:                  discoveryID,
		TenantID:                     tenantID,
		SourceCompanyID:              request.SourceCompanyID,
		ManifestVersion:              BrowserDiscoveryManifestVersion,
		ContractVersion:              BrowserDiscoveryContractVersion,
		ResourceIDs:                  browserDiscoveryResourceIDs(),
		MetadataOnlyConsentConfirmed: true,
		HeaderProbeConsentConfirmed:  request.ResponseHeaderProbeConfirmed,
		ConsentedAt:                  now,
		CreatedBy:                    strings.TrimSpace(actor),
		ExpiresAt:                    now.Add(browserDiscoveryLifetime),
		CreatedAt:                    now,
	}
	if err := s.store.CreateBrowserDiscoveryAuthorization(ctx, authorization); err != nil {
		return nil, ErrBrowserDiscoveryUnavailable
	}
	return &BrowserDiscoveryIssue{
		DiscoveryID:      authorization.DiscoveryID,
		TenantID:         authorization.TenantID,
		SourceCompanyID:  authorization.SourceCompanyID,
		ManifestVersion:  authorization.ManifestVersion,
		ResourceIDs:      append([]string(nil), authorization.ResourceIDs...),
		ExpiresAt:        authorization.ExpiresAt,
		DiscoveryConsent: browserDiscoveryConsent(authorization),
	}, nil
}

func browserDiscoveryConsent(authorization BrowserDiscoveryAuthorization) BrowserDiscoveryConsent {
	consent := BrowserDiscoveryConsent{Version: 1, Confirmed: true, ConfirmedAt: authorization.ConsentedAt, Scope: "metadata_only"}
	if authorization.HeaderProbeConsentConfirmed {
		consent.Scope = "metadata_and_header_probe"
		consent.ResponseHeaderProbeConfirmed = true
	}
	return consent
}

func (s *BrowserDiscoveryService) Receive(ctx context.Context, tenantID, discoveryID string, result BrowserDiscoveryRelayResult) (BrowserDiscoveryReceipt, error) {
	authorization, err := s.authorization(ctx, tenantID, discoveryID, true)
	if err != nil {
		return BrowserDiscoveryReceipt{}, err
	}
	if !validBrowserDiscoveryRelayResult(result, authorization) {
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryInvalid
	}
	receipt, err := s.bridge.RecordBrowserDiscoveryReceipt(ctx, authorization.TenantID, authorization.SourceCompanyID, authorization.DiscoveryID, BrowserDiscoveryBridgeReceiptRequest{
		SourceCompanyID: authorization.SourceCompanyID,
		ManifestVersion: result.ManifestVersion,
		ContractVersion: result.ContractVersion,
		Status:          result.Status,
		Resources:       canonicalBrowserDiscoveryResources(result.Resources),
	})
	if err != nil {
		return BrowserDiscoveryReceipt{}, err
	}
	if !validBrowserDiscoveryReceipt(receipt, authorization) {
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryUnavailable
	}
	if err := s.store.SaveBrowserDiscoveryReceipt(ctx, authorization.TenantID, authorization.DiscoveryID, receipt, s.currentTime()); err != nil {
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryUnavailable
	}
	return receipt, nil
}

// Status allows an owner to see durable bridge aggregate progress after the
// same-window authorization expires. It never emits a source selector,
// discovery contract, control IDs, header names, consent record, or token.
func (s *BrowserDiscoveryService) Status(ctx context.Context, tenantID, discoveryID string) (BrowserDiscoveryReceipt, error) {
	authorization, err := s.authorization(ctx, tenantID, discoveryID, false)
	if err != nil {
		return BrowserDiscoveryReceipt{}, err
	}
	receipt, err := s.bridge.GetBrowserDiscoveryReceipt(ctx, authorization.TenantID, authorization.SourceCompanyID, authorization.DiscoveryID)
	if err != nil {
		return BrowserDiscoveryReceipt{}, err
	}
	if !validBrowserDiscoveryReceipt(receipt, authorization) {
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryUnavailable
	}
	if err := s.store.SaveBrowserDiscoveryReceipt(ctx, authorization.TenantID, authorization.DiscoveryID, receipt, s.currentTime()); err != nil {
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryUnavailable
	}
	return receipt, nil
}

func (s *BrowserDiscoveryService) authorization(ctx context.Context, tenantID, discoveryID string, requireUnexpired bool) (*BrowserDiscoveryAuthorization, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserDiscoveryID(discoveryID) {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	authorization, err := s.store.GetBrowserDiscoveryAuthorization(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(discoveryID))
	if err != nil || authorization == nil || !validBrowserDiscoveryAuthorization(*authorization) {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	if requireUnexpired && !authorization.ExpiresAt.After(s.currentTime()) {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	control, err := s.controls.Get(ctx, authorization.TenantID, authorization.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	return authorization, nil
}

func (s *BrowserDiscoveryService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func validBrowserDiscoveryStartRequest(request BrowserDiscoveryStartRequest) bool {
	return validBrowserSourceCompanyID(strings.TrimSpace(request.SourceCompanyID))
}

func validBrowserDiscoveryID(value string) bool {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil && strings.ToLower(parsed.String()) == strings.TrimSpace(value)
}

func validBrowserDiscoveryAuthorization(authorization BrowserDiscoveryAuthorization) bool {
	return validBrowserDiscoveryID(authorization.DiscoveryID) && safeBridgeID(authorization.TenantID) && validBrowserSourceCompanyID(authorization.SourceCompanyID) && authorization.ManifestVersion == BrowserDiscoveryManifestVersion && authorization.ContractVersion == BrowserDiscoveryContractVersion && sameStringSet(authorization.ResourceIDs, browserDiscoveryResourceIDs()) && authorization.MetadataOnlyConsentConfirmed && !authorization.ConsentedAt.IsZero() && !authorization.ExpiresAt.IsZero() && authorization.ExpiresAt.After(authorization.ConsentedAt) && !authorization.CreatedAt.IsZero()
}

func validBrowserDiscoveryRelayResult(result BrowserDiscoveryRelayResult, authorization *BrowserDiscoveryAuthorization) bool {
	if authorization == nil || result.Source != browserDiscoveryRelaySource || result.Type != browserDiscoveryRelayEvent || result.Version != 1 || result.DiscoveryID != authorization.DiscoveryID || result.ManifestVersion != authorization.ManifestVersion || result.ContractVersion != authorization.ContractVersion || !validBrowserDiscoveryStatus(result.Status) || !sameBrowserDiscoveryResources(result.Resources, authorization.ResourceIDs, authorization.HeaderProbeConsentConfirmed, result.Status == "completed") {
		return false
	}
	return true
}

func validBrowserDiscoveryStatus(value string) bool {
	switch value {
	case "completed", "awaiting_browser", "company_binding_blocked", "expired", "discovery_failed":
		return true
	default:
		return false
	}
}

func sameBrowserDiscoveryResources(resources []BrowserDiscoveryResource, expected []string, headerProbeAllowed, requireFullSet bool) bool {
	if len(resources) > len(expected) || len(resources) > 31 || (requireFullSet && len(resources) != len(expected)) {
		return false
	}
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if !validBrowserDiscoveryResource(resource, headerProbeAllowed) {
			return false
		}
		if _, duplicate := seen[resource.ResourceID]; duplicate {
			return false
		}
		seen[resource.ResourceID] = struct{}{}
	}
	if requireFullSet {
		return sameStringSet(resourceIDsFromBrowserDiscovery(resources), expected)
	}
	for _, resourceID := range resourceIDsFromBrowserDiscovery(resources) {
		if !containsBrowserDiscoveryResourceID(expected, resourceID) {
			return false
		}
	}
	return true
}

func containsBrowserDiscoveryResourceID(expected []string, resourceID string) bool {
	for _, candidate := range expected {
		if candidate == resourceID {
			return true
		}
	}
	return false
}

func validBrowserDiscoveryResource(resource BrowserDiscoveryResource, headerProbeAllowed bool) bool {
	if !safeBridgeID(resource.ResourceID) || !validBrowserDiscoveryCaptureStatus(resource.CaptureStatus) || !validBrowserDiscoveryBinding(resource.Binding) || !validBrowserDiscoveryContract(resource.Contract, headerProbeAllowed) {
		return false
	}
	return true
}

func validBrowserDiscoveryCaptureStatus(value string) bool {
	switch value {
	case "capture_ready", "filter_contract_required", "page_only_contract_required", "private_endpoint_required", "session_blocked", "company_binding_blocked", "page_binding_blocked":
		return true
	default:
		return false
	}
}

func validBrowserDiscoveryBinding(binding BrowserDiscoveryBinding) bool {
	return validBrowserDiscoveryBindingValue(binding.Session) && validBrowserDiscoveryBindingValue(binding.Company) && validBrowserDiscoveryBindingValue(binding.Page)
}

func validBrowserDiscoveryBindingValue(value string) bool {
	switch value {
	case "verified", "blocked":
		return true
	default:
		return false
	}
}

func validBrowserDiscoveryContract(contract BrowserDiscoveryResourceContract, headerProbeAllowed bool) bool {
	if contract.Version != BrowserDiscoveryContractVersion || !validBrowserDiscoveryPath(contract.PagePath) || !validBrowserDiscoveryRequest(contract.Request) || !validBrowserDiscoveryFilter(contract.Filter) || !validBrowserDiscoveryPagination(contract.Pagination) || !validBrowserDiscoveryResponse(contract.Response, headerProbeAllowed) {
		return false
	}
	return true
}

func validBrowserDiscoveryRequest(request *BrowserDiscoveryRequestContract) bool {
	return request == nil || (request.Method == "GET" && validBrowserDiscoveryPath(request.Path))
}

func validBrowserDiscoveryFilter(filter *BrowserDiscoveryFilterContract) bool {
	return filter == nil || (filter.Method == "POST" && validBrowserDiscoveryPath(filter.Path) && validBrowserDiscoveryIdentifiers(filter.ControlIDs))
}

func validBrowserDiscoveryPagination(pagination BrowserDiscoveryPagination) bool {
	switch pagination.Kind {
	case "unobserved":
		return len(pagination.ControlIDs) == 0
	case "visible_control_ids":
		return len(pagination.ControlIDs) > 0 && validBrowserDiscoveryIdentifiers(pagination.ControlIDs)
	default:
		return false
	}
}

func validBrowserDiscoveryResponse(response BrowserDiscoveryResponseContract, headerProbeAllowed bool) bool {
	switch response.Observation {
	case "unobserved":
		return response.ContentType == "unobserved" && len(response.HeaderNames) == 0
	case "head":
		return response.ContentType == "text/csv" && len(response.HeaderNames) == 0
	case "range_header":
		return response.ContentType == "text/csv" && headerProbeAllowed && validBrowserDiscoveryHeaderNames(response.HeaderNames)
	default:
		return false
	}
}

func validBrowserDiscoveryPath(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "/") && len(trimmed) <= BrowserDiscoveryMaxPathBytes && !strings.ContainsAny(trimmed, "?#\r\n") && !strings.Contains(trimmed, "//")
}

func validBrowserDiscoveryIdentifiers(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validBrowserDiscoveryIdentifier(value) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

// validBrowserDiscoveryIdentifier is generated from the same private protocol
// artifact as the relay and bridge. The aggregate JSON request has its own
// bounded limit; there is no separate count limit for observed control IDs.
func validBrowserDiscoveryIdentifier(value string) bool {
	if len(value) < 1 || len(value) > BrowserDiscoveryMaxControlIDBytes || value[0] < 'A' || value[0] > 'z' || (value[0] > 'Z' && value[0] < 'a') {
		return false
	}
	for _, character := range value {
		alphaNumeric := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
		if !alphaNumeric && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func validBrowserDiscoveryHeaderNames(values []string) bool {
	if len(values) == 0 || len(values) > BrowserDiscoveryMaxHeaderNames {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		// Do not normalize header metadata. Its exact spelling and order are part
		// of the discovery contract that the bridge digests. The trim check only
		// rejects surrounding whitespace; it does not alter the original value.
		if len(value) < 1 || len(value) > BrowserDiscoveryMaxHeaderNameUTF8Bytes || strings.TrimSpace(value) != value {
			return false
		}
		for _, character := range value {
			if character < 0x20 || character == 0x7f {
				return false
			}
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validBrowserDiscoveryReceipt(receipt BrowserDiscoveryReceipt, authorization *BrowserDiscoveryAuthorization) bool {
	if authorization == nil || receipt.DiscoveryID != authorization.DiscoveryID || receipt.ManifestVersion != authorization.ManifestVersion || receipt.ContractVersion != authorization.ContractVersion || !validBrowserDiscoveryStatus(receipt.Status) || !validSHA256(receipt.ContractSHA256) || receipt.ResourceCount < 0 || receipt.ResourceCount > len(authorization.ResourceIDs) || (receipt.Status == "completed" && receipt.ResourceCount != len(authorization.ResourceIDs)) || receipt.CaptureReadyCount < 0 || receipt.FilterRequiredCount < 0 || receipt.PageOnlyRequiredCount < 0 || receipt.PrivateEndpointCount < 0 || receipt.BindingBlockedCount < 0 {
		return false
	}
	return receipt.CaptureReadyCount+receipt.FilterRequiredCount+receipt.PageOnlyRequiredCount+receipt.PrivateEndpointCount+receipt.BindingBlockedCount == receipt.ResourceCount
}

func canonicalBrowserDiscoveryResources(resources []BrowserDiscoveryResource) []BrowserDiscoveryResource {
	result := append([]BrowserDiscoveryResource(nil), resources...)
	sort.Slice(result, func(i, j int) bool { return result[i].ResourceID < result[j].ResourceID })
	for index := range result {
		if result[index].Contract.Filter != nil {
			result[index].Contract.Filter.ControlIDs = canonicalBrowserDiscoveryIdentifiers(result[index].Contract.Filter.ControlIDs)
		}
		result[index].Contract.Pagination.ControlIDs = canonicalBrowserDiscoveryIdentifiers(result[index].Contract.Pagination.ControlIDs)
		result[index].Contract.Response.HeaderNames = canonicalBrowserDiscoveryHeaderNames(result[index].Contract.Response.HeaderNames)
	}
	return result
}

func canonicalBrowserDiscoveryIdentifiers(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func canonicalBrowserDiscoveryHeaderNames(values []string) []string {
	// Header order is observed schema metadata. Preserve it exactly for the
	// bridge's immutable digest, unlike control IDs whose order is irrelevant.
	return append([]string(nil), values...)
}

func resourceIDsFromBrowserDiscovery(resources []BrowserDiscoveryResource) []string {
	result := make([]string, 0, len(resources))
	for _, resource := range resources {
		result = append(result, resource.ResourceID)
	}
	return result
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}
