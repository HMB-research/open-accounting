package smartaccountssync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrBridgeClientUnavailable = errors.New("SmartAccounts bridge client is unavailable")
	ErrBridgeRequestFailed     = errors.New("SmartAccounts bridge request failed")
)

const (
	bridgeTokenLifetime                = 5 * time.Minute
	maxBridgeResponseBytes             = 64 << 10
	maxSourceCredentialReferenceLength = 512
)

// BridgeClient is the narrow server-only integration to the private NUC
// bridge. It does not expose a source-data, package, planning, or financial
// apply operation.
type BridgeClient interface {
	ConnectAndValidate(ctx context.Context, tenantID, sourceCredentialReference string) (BridgeConnection, error)
	StartCapture(ctx context.Context, tenantID, connectionID string, request CaptureRequest) (CaptureProgress, error)
	GetCapture(ctx context.Context, tenantID, connectionID, runID string) (CaptureProgress, error)
}

// BridgeHealthChecker is an optional, data-free readiness seam. It checks the
// private bridge process health endpoint and its exact, public capability
// contract; it never mints a tenant token, reads credentials, or contacts
// SmartAccounts.
type BridgeHealthChecker interface {
	Health(context.Context) error
}

// BridgeConnection contains safe connection metadata returned by the bridge.
type BridgeConnection struct {
	ConnectionID          string
	SecretReference       string
	ValidationStatus      string
	AccountCount          int
	SourceCompanyID       string
	SourceCompanyName     string
	SourceBindingStatus   string
	AccountSnapshotSHA256 string
}

// UnavailableBridgeClient is the safe default when the private bridge is not
// configured. It performs no network operation.
type UnavailableBridgeClient struct{}

func (UnavailableBridgeClient) Health(_ context.Context) error { return ErrBridgeClientUnavailable }

func (UnavailableBridgeClient) ConnectAndValidate(_ context.Context, _ string, _ string) (BridgeConnection, error) {
	return BridgeConnection{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) StartCapture(_ context.Context, _ string, _ string, _ CaptureRequest) (CaptureProgress, error) {
	return CaptureProgress{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) GetCapture(_ context.Context, _ string, _ string, _ string) (CaptureProgress, error) {
	return CaptureProgress{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) RecordBrowserDiscoveryReceipt(_ context.Context, _ string, _ string, _ string, _ BrowserDiscoveryBridgeReceiptRequest) (BrowserDiscoveryReceipt, error) {
	return BrowserDiscoveryReceipt{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) GetBrowserDiscoveryReceipt(_ context.Context, _ string, _ string, _ string) (BrowserDiscoveryReceipt, error) {
	return BrowserDiscoveryReceipt{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) RegisterBrowserCSVSchemaApproval(_ context.Context, _ string, _ string, _ string, _ string, _ BrowserCSVSchemaApprovalBridgeRequest) (BrowserCSVSchemaApprovalResponse, error) {
	return BrowserCSVSchemaApprovalResponse{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) GetBrowserCSVSchemaApproval(_ context.Context, _ string, _ string, _ string, _ string) (BrowserCSVSchemaApprovalResponse, error) {
	return BrowserCSVSchemaApprovalResponse{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) StartBrowserCapture(_ context.Context, _ string, _ string, _ BrowserCaptureStartRequest) (BrowserCaptureStatus, error) {
	return BrowserCaptureStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) GetBrowserCapture(_ context.Context, _ string, _ string) (BrowserCaptureStatus, error) {
	return BrowserCaptureStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) UploadBrowserCaptureResource(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ []byte) (BrowserCaptureResourceStatus, error) {
	return BrowserCaptureResourceStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) FinalizeBrowserCapture(_ context.Context, _ string, _ string) (BrowserCaptureStatus, error) {
	return BrowserCaptureStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) StartBrowserMasterDetail(_ context.Context, _ string, _ string, _ BrowserMasterDetailStartRequest) (BrowserMasterDetailStatus, error) {
	return BrowserMasterDetailStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) GetBrowserMasterDetail(_ context.Context, _ string, _ string) (BrowserMasterDetailStatus, error) {
	return BrowserMasterDetailStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) UploadBrowserMasterDetail(_ context.Context, _ string, _ string, _ string, _ []byte) (BrowserMasterDetailUploadResult, error) {
	return BrowserMasterDetailUploadResult{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) FinalizeBrowserMasterDetail(_ context.Context, _ string, _ string) (BrowserMasterDetailStatus, error) {
	return BrowserMasterDetailStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) StartBrowserCommercialDetail(_ context.Context, _ string, _ string, _ BrowserCommercialDetailStartRequest) (BrowserCommercialDetailStatus, error) {
	return BrowserCommercialDetailStatus{}, ErrBridgeClientUnavailable
}

func (UnavailableBridgeClient) GetBrowserCommercialDetail(_ context.Context, _ string, _ string) (BrowserCommercialDetailStatus, error) {
	return BrowserCommercialDetailStatus{}, ErrBridgeClientUnavailable
}

// HTTPBridgeClient speaks only to the private bridge internal API. tokenSecret
// is the server-only HMAC secret; it is used only to mint short-lived,
// destination-tenant scoped tokens and is never forwarded as a raw value.
type HTTPBridgeClient struct {
	baseURL     *url.URL
	tokenSecret []byte
	httpClient  *http.Client
	now         func() time.Time
}

// NewHTTPBridgeClient creates the private bridge adapter. A caller should keep
// it unreachable from browser configuration and host-published ports.
func NewHTTPBridgeClient(baseURL, tokenSecret string) (*HTTPBridgeClient, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("SmartAccounts bridge URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("SmartAccounts bridge URL must use HTTP or HTTPS")
	}
	if len(tokenSecret) < 16 {
		return nil, errors.New("SmartAccounts bridge token configuration is invalid")
	}
	return &HTTPBridgeClient{
		baseURL:     parsed,
		tokenSecret: []byte(tokenSecret),
		httpClient: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}, nil
}

const (
	bridgeCapabilitiesSchemaVersion = "sa-bridge-capabilities-v1"
	bridgeCapabilitiesAPIVersion    = "sa-bridge-api-v1"
	bridgeDiscoveryProtocolVersion  = "smartaccounts-browser-discovery-protocol-v1"
	bridgeDiscoveryContractVersion  = "smartaccounts-brave-discovery-contract-v1"
	// Keep the bridge readiness check coupled to the generated browser contract.
	// A bridge serving an older capture manifest must fail readiness rather than
	// accepting a summary-grid capture as an authoritative GL source.
	bridgeCaptureManifestVersion   = BrowserCaptureManifestVersion
	bridgeCaptureProtocolVersion   = "smartaccounts-browser-capture-bridge-v1"
	bridgeCSVSchemaRegistryVersion = "smartaccounts-browser-csv-schema-registry-v1"
	bridgeCSVSchemaReviewVersion   = "smartaccounts-browser-csv-schema-review-v1"
	bridgeOAStagingProtocolVersion = "open-accounting-import-delivery-v1"
)

type bridgeCapabilitiesResponse struct {
	SchemaVersion                   string `json:"schema_version"`
	BridgeAPIVersion                string `json:"bridge_api_version"`
	BridgeVersion                   string `json:"bridge_version"`
	BridgeCommit                    string `json:"bridge_commit"`
	BrowserDiscoveryProtocolVersion string `json:"browser_discovery_protocol_version"`
	BrowserDiscoveryContractVersion string `json:"browser_discovery_contract_version"`
	BrowserCaptureManifestVersion   string `json:"browser_capture_manifest_version"`
	BrowserCaptureProtocolVersion   string `json:"browser_capture_protocol_version"`
	BrowserCSVSchemaRegistryVersion string `json:"browser_csv_schema_registry_version"`
	BrowserCSVSchemaReviewVersion   string `json:"browser_csv_schema_review_version"`
	OAStagingProtocolVersion        string `json:"oa_staging_protocol_version"`
}

// Health verifies the internal bridge process and the exact data-free bridge
// capability contract without credentials or source access. Redirects remain
// disabled by the configured client.
func (c *HTTPBridgeClient) Health(ctx context.Context) error {
	if c == nil || c.baseURL == nil || c.httpClient == nil {
		return ErrBridgeClientUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.baseURL.String(), "/")+"/health", nil)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrBridgeRequestFailed
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes)).Decode(&payload); err != nil || payload.Status != "ok" {
		return ErrBridgeRequestFailed
	}
	return c.verifyCapabilities(ctx)
}

// verifyCapabilities accepts only the versioned, source-free bridge contract.
// The endpoint is intentionally unauthenticated: readiness must neither mint a
// tenant token nor reveal bridge state. A changed or expanded response fails
// closed so a newer bridge cannot silently run an incompatible import flow.
func (c *HTTPBridgeClient) verifyCapabilities(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.baseURL.String(), "/")+"/capabilities", nil)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrBridgeRequestFailed
	}
	if !strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "application/json") || !cacheControlNoStore(response.Header.Get("Cache-Control")) {
		return ErrBridgeRequestFailed
	}

	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	decoder.DisallowUnknownFields()
	var capabilities bridgeCapabilitiesResponse
	if err := decoder.Decode(&capabilities); err != nil || decoder.Decode(&struct{}{}) != io.EOF || !validBridgeCapabilities(capabilities) {
		return ErrBridgeRequestFailed
	}
	return nil
}

func validBridgeCapabilities(value bridgeCapabilitiesResponse) bool {
	return value.SchemaVersion == bridgeCapabilitiesSchemaVersion &&
		value.BridgeAPIVersion == bridgeCapabilitiesAPIVersion &&
		validBridgeCapabilityStamp(value.BridgeVersion) &&
		validBridgeCapabilityStamp(value.BridgeCommit) &&
		value.BrowserDiscoveryProtocolVersion == bridgeDiscoveryProtocolVersion &&
		value.BrowserDiscoveryContractVersion == bridgeDiscoveryContractVersion &&
		value.BrowserCaptureManifestVersion == bridgeCaptureManifestVersion &&
		value.BrowserCaptureProtocolVersion == bridgeCaptureProtocolVersion &&
		value.BrowserCSVSchemaRegistryVersion == bridgeCSVSchemaRegistryVersion &&
		value.BrowserCSVSchemaReviewVersion == bridgeCSVSchemaReviewVersion &&
		value.OAStagingProtocolVersion == bridgeOAStagingProtocolVersion
}

func validBridgeCapabilityStamp(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 256
}

func cacheControlNoStore(value string) bool {
	for _, directive := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
			return true
		}
	}
	return false
}

// ConnectAndValidate submits only the opaque external source credential
// reference to the bridge PUT endpoint and performs exactly one bridge
// validation request before returning the bridge-owned opaque reference. It
// never receives, sends, or includes raw source API material in an error.
func (c *HTTPBridgeClient) ConnectAndValidate(ctx context.Context, tenantID, sourceCredentialReference string) (BridgeConnection, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || len(c.tokenSecret) < 16 {
		return BridgeConnection{}, ErrBridgeClientUnavailable
	}
	if !safeBridgeID(tenantID) {
		return BridgeConnection{}, ErrBridgeRequestFailed
	}
	credentialReference, connectionID, err := normalizeSourceCredentialReference(sourceCredentialReference)
	if err != nil {
		return BridgeConnection{}, ErrBridgeRequestFailed
	}

	payload, err := json.Marshal(struct {
		SourceCredentialReference string `json:"source_credential_reference"`
	}{
		SourceCredentialReference: credentialReference,
	})
	if err != nil {
		return BridgeConnection{}, ErrBridgeRequestFailed
	}

	var configured bridgeConnectionResponse
	if err := c.requestJSON(ctx, tenantID, http.MethodPut, "/v1/connections/"+connectionID, bytes.NewReader(payload), &configured); err != nil {
		return BridgeConnection{}, err
	}
	if configured.ConnectionID != connectionID || !configured.Configured || !isExpectedBridgeSecretReference(configured.SecretReference, connectionID) || !safeBridgeID(configured.SourceCompanyID) || strings.TrimSpace(configured.SourceIdentityLabel) == "" {
		return BridgeConnection{}, ErrBridgeRequestFailed
	}

	var validation bridgeValidationResponse
	if err := c.requestJSON(ctx, tenantID, http.MethodPost, "/v1/connections/"+connectionID+"/validate", nil, &validation); err != nil {
		return BridgeConnection{}, err
	}
	if validation.ConnectionID != connectionID || validation.Status != "connected" || validation.AccountCount < 0 || validation.SourceCompanyID != configured.SourceCompanyID || strings.TrimSpace(validation.SourceIdentityLabel) == "" || validation.SourceBindingStatus != "api_key_identity_and_snapshot_validated" || !validSHA256(validation.AccountSnapshotSHA256) {
		return BridgeConnection{}, ErrBridgeRequestFailed
	}

	return BridgeConnection{
		ConnectionID:          configured.ConnectionID,
		SecretReference:       configured.SecretReference,
		ValidationStatus:      validation.Status,
		AccountCount:          validation.AccountCount,
		SourceCompanyID:       validation.SourceCompanyID,
		SourceCompanyName:     validation.SourceIdentityLabel,
		SourceBindingStatus:   validation.SourceBindingStatus,
		AccountSnapshotSHA256: validation.AccountSnapshotSHA256,
	}, nil
}

// normalizeSourceCredentialReference accepts exactly the opaque reference
// grammar implemented by bridge 5a39445's production provider:
// secret-ref://file/<connection-id>. The connection ID deliberately comes from
// the opaque reference identifier; the bridge's file provider requires the
// same identifier to select the tenant-bound external mount. It is never an
// API key, source company ID, or source display name.
func normalizeSourceCredentialReference(value string) (reference, connectionID string, err error) {
	reference = strings.TrimSpace(value)
	if reference == "" || len(reference) > maxSourceCredentialReferenceLength || strings.ContainsAny(reference, "\r\n\t ?#@") {
		return "", "", errors.New("source credential reference is invalid")
	}
	parsed, parseErr := url.ParseRequestURI(reference)
	if parseErr != nil || parsed.Scheme != "secret-ref" || parsed.Host != "file" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", errors.New("source credential reference is invalid")
	}
	connectionID = strings.TrimPrefix(parsed.Path, "/")
	if parsed.Path != "/"+connectionID || !safeBridgeID(connectionID) || reference != "secret-ref://file/"+connectionID {
		return "", "", errors.New("source credential reference is invalid")
	}
	return reference, connectionID, nil
}

type bridgeConnectionResponse struct {
	ConnectionID        string `json:"connection_id"`
	SecretReference     string `json:"secret_reference"`
	Configured          bool   `json:"configured"`
	SourceCompanyID     string `json:"source_company_id"`
	SourceIdentityLabel string `json:"source_identity_label"`
}

type bridgeValidationResponse struct {
	ConnectionID          string `json:"connection_id"`
	Status                string `json:"status"`
	AccountCount          int    `json:"account_count"`
	SourceCompanyID       string `json:"source_company_id"`
	SourceIdentityLabel   string `json:"source_identity_label"`
	SourceBindingStatus   string `json:"source_binding_status"`
	AccountSnapshotSHA256 string `json:"account_snapshot_sha256"`
}

func (c *HTTPBridgeClient) requestJSON(ctx context.Context, tenantID, method, path string, body io.Reader, target interface{}) error {
	authorization, err := c.authorizationHeader(tenantID)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	endpoint := strings.TrimRight(c.baseURL.String(), "/") + path
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	request.Header.Set("Authorization", authorization)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ErrBridgeRequestFailed
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	if err := decoder.Decode(target); err != nil {
		return ErrBridgeRequestFailed
	}
	return nil
}

// RecordBrowserDiscoveryReceipt is a server-to-server, HMAC-authenticated
// proxy for a strictly redacted browser discovery result. The source selector
// is supplied only by OA's persisted tenant binding; no browser calls this
// private route and its response is reduced to aggregate safe receipt fields.
func (c *HTTPBridgeClient) RecordBrowserDiscoveryReceipt(ctx context.Context, tenantID, sourceCompanyID, discoveryID string, request BrowserDiscoveryBridgeReceiptRequest) (BrowserDiscoveryReceipt, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserSourceCompanyID(sourceCompanyID) || !validBrowserDiscoveryID(discoveryID) || strings.TrimSpace(request.SourceCompanyID) != strings.TrimSpace(sourceCompanyID) || request.ManifestVersion != BrowserDiscoveryManifestVersion || request.ContractVersion != BrowserDiscoveryContractVersion || !validBrowserDiscoveryStatus(request.Status) || !sameBrowserDiscoveryResources(request.Resources, browserDiscoveryResourceIDs(), true, request.Status == "completed") {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	return c.browserDiscoveryReceiptRequest(ctx, tenantID, http.MethodPost, sourceCompanyID, discoveryID, bytes.NewReader(payload))
}

func (c *HTTPBridgeClient) GetBrowserDiscoveryReceipt(ctx context.Context, tenantID, sourceCompanyID, discoveryID string) (BrowserDiscoveryReceipt, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserSourceCompanyID(sourceCompanyID) || !validBrowserDiscoveryID(discoveryID) {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	return c.browserDiscoveryReceiptRequest(ctx, tenantID, http.MethodGet, sourceCompanyID, discoveryID, nil)
}

func (c *HTTPBridgeClient) browserDiscoveryReceiptRequest(ctx context.Context, tenantID, method, sourceCompanyID, discoveryID string, body io.Reader) (BrowserDiscoveryReceipt, error) {
	authorization, err := c.authorizationHeader(strings.TrimSpace(tenantID))
	if err != nil {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	endpoint := strings.TrimRight(c.baseURL.String(), "/") + "/v1/browser-discovery-receipts/" + strings.TrimSpace(sourceCompanyID) + "/" + strings.TrimSpace(discoveryID)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// Both the first accepted receipt and an exact replay use the same
		// safe response shape.
	case http.StatusNotFound:
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryNotFound
	case http.StatusConflict:
		return BrowserDiscoveryReceipt{}, ErrBrowserDiscoveryConflict

	default:
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	var receipt BrowserDiscoveryReceipt
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil || decoder.Decode(&struct{}{}) != io.EOF || receipt.DiscoveryID != strings.TrimSpace(discoveryID) {
		return BrowserDiscoveryReceipt{}, ErrBridgeRequestFailed
	}
	return receipt, nil
}

// RegisterBrowserCSVSchemaApproval submits an already durable OA owner-review
// assertion to the private bridge. The source selector is resolved only from
// OA's persisted discovery authorization and never exposed to callers.
func (c *HTTPBridgeClient) RegisterBrowserCSVSchemaApproval(ctx context.Context, tenantID, sourceCompanyID, resourceID, schemaID string, input BrowserCSVSchemaApprovalBridgeRequest) (BrowserCSVSchemaApprovalResponse, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserSourceCompanyID(sourceCompanyID) || !validBrowserCSVSchemaResourceID(resourceID) || !validBrowserCSVSchemaID(schemaID) || !validBrowserDiscoveryID(input.DiscoveryID) || input.SchemaID != strings.TrimSpace(schemaID) || input.Review.Version != BrowserCSVSchemaReviewVersion || !input.Review.Confirmed || !validBrowserDiscoveryID(input.Review.AuditID) || input.Review.ReviewedAt.IsZero() {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	return c.browserCSVSchemaApprovalRequest(ctx, tenantID, http.MethodPost, sourceCompanyID, resourceID, schemaID, bytes.NewReader(payload))
}

func (c *HTTPBridgeClient) GetBrowserCSVSchemaApproval(ctx context.Context, tenantID, sourceCompanyID, resourceID, schemaID string) (BrowserCSVSchemaApprovalResponse, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserSourceCompanyID(sourceCompanyID) || !validBrowserCSVSchemaResourceID(resourceID) || !validBrowserCSVSchemaID(schemaID) {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	return c.browserCSVSchemaApprovalRequest(ctx, tenantID, http.MethodGet, sourceCompanyID, resourceID, schemaID, nil)
}

func (c *HTTPBridgeClient) browserCSVSchemaApprovalRequest(ctx context.Context, tenantID, method, sourceCompanyID, resourceID, schemaID string, body io.Reader) (BrowserCSVSchemaApprovalResponse, error) {
	authorization, err := c.authorizationHeader(strings.TrimSpace(tenantID))
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	endpoint := strings.TrimRight(c.baseURL.String(), "/") + "/v1/browser-csv-schema-approvals/" + strings.TrimSpace(sourceCompanyID) + "/" + strings.TrimSpace(resourceID) + "/" + strings.TrimSpace(schemaID)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
	case http.StatusNotFound:
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalNotFound
	case http.StatusConflict:
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalConflict
	default:
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	var result BrowserCSVSchemaApprovalResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil || decoder.Decode(&struct{}{}) != io.EOF || result.ResourceID != strings.TrimSpace(resourceID) || result.SchemaID != strings.TrimSpace(schemaID) || result.Status != "registered" || !validSHA256(result.ApprovalSHA256) {
		return BrowserCSVSchemaApprovalResponse{}, ErrBridgeRequestFailed
	}
	return result, nil
}

// StartCapture starts or resumes the bridge's read-only capture. Its payload
// contains scope only; source identity and credentials remain bridge-owned.
func (c *HTTPBridgeClient) StartCapture(ctx context.Context, tenantID, connectionID string, request CaptureRequest) (CaptureProgress, error) {
	if c == nil || !safeBridgeID(tenantID) || !safeBridgeID(connectionID) || !validCaptureRequest(request) {
		return CaptureProgress{}, ErrBridgeRequestFailed
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return CaptureProgress{}, ErrBridgeRequestFailed
	}
	var response bridgeCaptureResponse
	if err := c.requestJSON(ctx, tenantID, http.MethodPost, "/v1/connections/"+connectionID+"/captures", bytes.NewReader(payload), &response); err != nil {
		return CaptureProgress{}, err
	}
	return response.toCaptureProgress(connectionID)
}

// GetCapture returns bridge-owned safe capture progress only.
func (c *HTTPBridgeClient) GetCapture(ctx context.Context, tenantID, connectionID, runID string) (CaptureProgress, error) {
	if c == nil || !safeBridgeID(tenantID) || !safeBridgeID(connectionID) || !safeBridgeID(runID) {
		return CaptureProgress{}, ErrBridgeRequestFailed
	}
	var response bridgeCaptureResponse
	if err := c.requestJSON(ctx, tenantID, http.MethodGet, "/v1/connections/"+connectionID+"/captures/"+runID, nil, &response); err != nil {
		return CaptureProgress{}, err
	}
	return response.toCaptureProgress(connectionID)
}

func (c *HTTPBridgeClient) StartBrowserCapture(ctx context.Context, tenantID, runID string, request BrowserCaptureStartRequest) (BrowserCaptureStatus, error) {
	if c == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) || !validBrowserCaptureRequest(request) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	var response bridgeBrowserCaptureResponse
	if err := c.requestJSON(ctx, tenantID, http.MethodPost, "/v1/browser-captures/"+runID, bytes.NewReader(payload), &response); err != nil {
		return BrowserCaptureStatus{}, err
	}
	return response.toBrowserCaptureStatus(runID)
}
func (c *HTTPBridgeClient) GetBrowserCapture(ctx context.Context, tenantID, runID string) (BrowserCaptureStatus, error) {
	if c == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	var response bridgeBrowserCaptureResponse
	if err := c.requestJSON(ctx, tenantID, http.MethodGet, "/v1/browser-captures/"+runID, nil, &response); err != nil {
		return BrowserCaptureStatus{}, err
	}
	return response.toBrowserCaptureStatus(runID)
}
func (c *HTTPBridgeClient) FinalizeBrowserCapture(ctx context.Context, tenantID, runID string) (BrowserCaptureStatus, error) {
	if c == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	authorization, err := c.authorizationHeader(tenantID)
	if err != nil {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL.String(), "/")+"/v1/browser-captures/"+runID+"/finalize", http.NoBody)
	if err != nil {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	// The private bridge contract deliberately uses an empty request body while
	// still requiring the JSON media type. The extension-facing OA endpoint
	// separately accepts the exact `{}` envelope and does not forward it.
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	httpResponse, err := c.httpClient.Do(request)
	if err != nil {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	var result bridgeBrowserCaptureResponse
	if err := json.NewDecoder(io.LimitReader(httpResponse.Body, maxBridgeResponseBytes)).Decode(&result); err != nil {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	return result.toBrowserCaptureStatus(runID)
}
func (c *HTTPBridgeClient) UploadBrowserCaptureResource(ctx context.Context, tenantID, runID, resourceID, digest, contentType string, body []byte) (BrowserCaptureResourceStatus, error) {
	bodyDigest := sha256.Sum256(body)
	if c == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) || !safeBridgeID(resourceID) || !validSHA256(digest) || len(body) == 0 || len(body) > BrowserCaptureMaxResourceBytes || hex.EncodeToString(bodyDigest[:]) != digest {
		return BrowserCaptureResourceStatus{}, ErrBridgeRequestFailed
	}
	auth, err := c.authorizationHeader(tenantID)
	if err != nil {
		return BrowserCaptureResourceStatus{}, ErrBridgeRequestFailed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(c.baseURL.String(), "/")+"/v1/browser-captures/"+runID+"/resources/"+resourceID, bytes.NewReader(body))
	if err != nil {
		return BrowserCaptureResourceStatus{}, ErrBridgeRequestFailed
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-SA-Browser-Resource-SHA256", digest)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return BrowserCaptureResourceStatus{}, ErrBridgeRequestFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return BrowserCaptureResourceStatus{}, ErrBridgeRequestFailed
	}
	var result BrowserCaptureResourceStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBridgeResponseBytes)).Decode(&result); err != nil || !safeBridgeID(result.ResourceID) || strings.TrimSpace(result.Status) == "" {
		return BrowserCaptureResourceStatus{}, ErrBridgeRequestFailed
	}
	return result, nil
}

// StartBrowserMasterDetail uses the separate, reviewed master-detail bridge
// contract. It is deliberately not a variant of CSV/GL capture.
func (c *HTTPBridgeClient) StartBrowserMasterDetail(ctx context.Context, tenantID, runID string, input BrowserMasterDetailStartRequest) (BrowserMasterDetailStatus, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) || !validBrowserMasterDetailStartRequest(input) {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	return c.browserMasterDetailRequest(ctx, tenantID, http.MethodPost, runID, "", bytes.NewReader(payload))
}

func (c *HTTPBridgeClient) GetBrowserMasterDetail(ctx context.Context, tenantID, runID string) (BrowserMasterDetailStatus, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	return c.browserMasterDetailRequest(ctx, tenantID, http.MethodGet, runID, "", nil)
}

func (c *HTTPBridgeClient) UploadBrowserMasterDetail(ctx context.Context, tenantID, runID, digest string, body []byte) (BrowserMasterDetailUploadResult, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) || !validSHA256(digest) || len(body) == 0 || len(body) > browserMasterDetailMaxBytes {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != digest {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	auth, err := c.authorizationHeader(tenantID)
	if err != nil {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(c.baseURL.String(), "/")+"/v1/browser-master-detail-captures/"+runID+"/resource", bytes.NewReader(body))
	if err != nil {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	request.Header.Set("Authorization", auth)
	request.Header.Set("Content-Type", "application/x-ndjson")
	request.Header.Set("X-SA-Browser-Resource-SHA256", digest)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	decoder.DisallowUnknownFields()
	var result BrowserMasterDetailUploadResult
	if err := decoder.Decode(&result); err != nil || decoder.Decode(&struct{}{}) != io.EOF || result.RunID != runID || result.Status != "accepted" {
		return BrowserMasterDetailUploadResult{}, ErrBridgeRequestFailed
	}
	return result, nil
}

func (c *HTTPBridgeClient) FinalizeBrowserMasterDetail(ctx context.Context, tenantID, runID string) (BrowserMasterDetailStatus, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	return c.browserMasterDetailRequest(ctx, tenantID, http.MethodPost, runID, "finalize", http.NoBody)
}

func (c *HTTPBridgeClient) browserMasterDetailRequest(ctx context.Context, tenantID, method, runID, suffix string, body io.Reader) (BrowserMasterDetailStatus, error) {
	auth, err := c.authorizationHeader(tenantID)
	if err != nil {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	path := "/v1/browser-master-detail-captures/" + runID
	if suffix != "" {
		path += "/" + suffix
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL.String(), "/")+path, body)
	if err != nil {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	request.Header.Set("Authorization", auth)
	request.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !cacheControlNoStore(response.Header.Get("Cache-Control")) {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	decoder.DisallowUnknownFields()
	var responseBody bridgeBrowserMasterDetailResponse
	if err := decoder.Decode(&responseBody); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	return responseBody.toBrowserMasterDetailStatus(runID)
}

// bridgeBrowserMasterDetailResponse deliberately models only the documented
// private safe response. BrowserMasterDetailStatus contains OA's rehydrated
// tenant/source/scope/approval projection, so decoding directly into it would
// accidentally accept fields the private bridge must never return.
type bridgeBrowserMasterDetailResponse struct {
	RunID           string `json:"run_id"`
	Status          string `json:"status"`
	ManifestVersion string `json:"manifest_version"`
	ResourceID      string `json:"resource_id"`
	SchemaID        string `json:"schema_id"`
	SourceSchema    string `json:"source_schema"`
	ContractSHA256  string `json:"contract_sha256"`
	NDJSONSHA256    string `json:"ndjson_sha256,omitempty"`
	RecordCount     int    `json:"record_count,omitempty"`
	PackageID       string `json:"package_id,omitempty"`
	PackageSHA256   string `json:"package_sha256,omitempty"`
}

func (value bridgeBrowserMasterDetailResponse) toBrowserMasterDetailStatus(runID string) (BrowserMasterDetailStatus, error) {
	if value.RunID != runID || value.ManifestVersion != BrowserMasterDetailManifestVersion || !validBrowserMasterDetailResourceSchema(value.ResourceID, value.SchemaID, value.SourceSchema) || !validSHA256(value.ContractSHA256) || value.RecordCount < 0 {
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	switch value.Status {
	case "open":
		if value.NDJSONSHA256 != "" || value.RecordCount != 0 || value.PackageID != "" || value.PackageSHA256 != "" {
			return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
		}
	case "finalized":
		if !validSHA256(value.NDJSONSHA256) || value.RecordCount < 1 || !safeBridgeID(value.PackageID) || !validSHA256(value.PackageSHA256) {
			return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
		}
	default:
		return BrowserMasterDetailStatus{}, ErrBridgeRequestFailed
	}
	return BrowserMasterDetailStatus{RunID: value.RunID, Status: value.Status, ManifestVersion: value.ManifestVersion, ResourceID: value.ResourceID, SchemaID: value.SchemaID, SourceSchema: value.SourceSchema, ContractSHA256: value.ContractSHA256, NDJSONSHA256: value.NDJSONSHA256, RecordCount: value.RecordCount, PackageID: value.PackageID, PackageSHA256: value.PackageSHA256}, nil
}

// StartBrowserCommercialDetail implements the exact private commercial
// start envelope. The client only receives code-reviewed metadata generated
// by BrowserCommercialDetailService; browser callers cannot select it.
func (c *HTTPBridgeClient) StartBrowserCommercialDetail(ctx context.Context, tenantID, runID string, input BrowserCommercialDetailStartRequest) (BrowserCommercialDetailStatus, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) || !validBrowserCommercialDetailStartRequest(input) {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	return c.browserCommercialDetailRequest(ctx, tenantID, http.MethodPost, runID, bytes.NewReader(payload))
}

func (c *HTTPBridgeClient) GetBrowserCommercialDetail(ctx context.Context, tenantID, runID string) (BrowserCommercialDetailStatus, error) {
	if c == nil || c.baseURL == nil || c.httpClient == nil || !safeBridgeID(tenantID) || !validBrowserPairingID(runID) {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	return c.browserCommercialDetailRequest(ctx, tenantID, http.MethodGet, runID, nil)
}

func (c *HTTPBridgeClient) browserCommercialDetailRequest(ctx context.Context, tenantID, method, runID string, body io.Reader) (BrowserCommercialDetailStatus, error) {
	auth, err := c.authorizationHeader(tenantID)
	if err != nil {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL.String(), "/")+"/v1/browser-commercial-captures/"+runID, body)
	if err != nil {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	request.Header.Set("Authorization", auth)
	request.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !cacheControlNoStore(response.Header.Get("Cache-Control")) {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBridgeResponseBytes))
	decoder.DisallowUnknownFields()
	var responseBody bridgeBrowserCommercialDetailResponse
	if err := decoder.Decode(&responseBody); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	return responseBody.toBrowserCommercialDetailStatus(runID)
}

// bridgeBrowserCommercialDetailResponse is deliberately closed over the
// documented private safe receipt. It rejects leaked source IDs/contracts,
// route fields, browser state, rows, values, credentials, and delivery state.
type bridgeBrowserCommercialDetailResponse struct {
	RunID           string `json:"run_id"`
	Status          string `json:"status"`
	ManifestVersion string `json:"manifest_version"`
	ResourceID      string `json:"resource_id"`
	SchemaID        string `json:"schema_id"`
	SourceSchema    string `json:"source_schema"`
	RouteSHA256     string `json:"route_sha256"`
	ContractSHA256  string `json:"contract_sha256"`
	ConsentSHA256   string `json:"consent_sha256"`
	NDJSONSHA256    string `json:"ndjson_sha256,omitempty"`
	RecordCount     int    `json:"record_count,omitempty"`
	ReviewRequired  int    `json:"review_required,omitempty"`
	PackageID       string `json:"package_id,omitempty"`
	PackageSHA256   string `json:"package_sha256,omitempty"`
}

func (value bridgeBrowserCommercialDetailResponse) toBrowserCommercialDetailStatus(runID string) (BrowserCommercialDetailStatus, error) {
	if value.RunID != runID || value.ManifestVersion != BrowserCommercialDetailManifestVersion || !validBrowserCommercialDetailResourceSchema(value.ResourceID, value.SchemaID, value.SourceSchema) || !validSHA256(value.RouteSHA256) || !validSHA256(value.ContractSHA256) || !validSHA256(value.ConsentSHA256) || value.RecordCount < 0 || value.ReviewRequired < 0 || value.ReviewRequired > value.RecordCount {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	if value.Status == "open" {
		if value.NDJSONSHA256 != "" || value.RecordCount != 0 || value.ReviewRequired != 0 || value.PackageID != "" || value.PackageSHA256 != "" {
			return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
		}
	} else if value.Status == "finalized" {
		if !validSHA256(value.NDJSONSHA256) || value.RecordCount < 1 || value.ReviewRequired != value.RecordCount || !safeBridgeID(value.PackageID) || !validSHA256(value.PackageSHA256) {
			return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
		}
	} else {
		return BrowserCommercialDetailStatus{}, ErrBridgeRequestFailed
	}
	return BrowserCommercialDetailStatus{RunID: value.RunID, Status: value.Status, ManifestVersion: value.ManifestVersion, ResourceID: value.ResourceID, SchemaID: value.SchemaID, SourceSchema: value.SourceSchema, RouteSHA256: value.RouteSHA256, ContractSHA256: value.ContractSHA256, ConsentSHA256: value.ConsentSHA256, NDJSONSHA256: value.NDJSONSHA256, RecordCount: value.RecordCount, ReviewRequired: value.ReviewRequired, PackageID: value.PackageID, PackageSHA256: value.PackageSHA256}, nil
}

type bridgeBrowserCaptureResponse struct {
	RunID           string                         `json:"run_id"`
	Status          string                         `json:"status"`
	ManifestVersion string                         `json:"manifest_version"`
	Scope           BrowserCaptureScope            `json:"scope"`
	Resources       []bridgeBrowserCaptureResource `json:"resources"`
	Receipt         *BrowserCaptureCoverageReceipt `json:"receipt,omitempty"`
	Staging         *BrowserCaptureStaging         `json:"staging,omitempty"`
}

// bridgeBrowserCaptureResource accepts the private bridge's safe checksum and
// byte-count fields only to validate its contract. OA intentionally projects
// neither into the public relay/owner status response.
type bridgeBrowserCaptureResource struct {
	ResourceID string `json:"resource_id"`
	Coverage   string `json:"coverage"`
	Status     string `json:"status"`
	SHA256     string `json:"sha256,omitempty"`
	ByteCount  int64  `json:"byte_count,omitempty"`
}

func (r bridgeBrowserCaptureResponse) toBrowserCaptureStatus(runID string) (BrowserCaptureStatus, error) {
	if r.RunID != runID || r.ManifestVersion != BrowserCaptureManifestVersion || !validBrowserCaptureRequest(BrowserCaptureStartRequest{SourceCompanyID: "sa-browser-v1-1", ManifestVersion: r.ManifestVersion, Scope: r.Scope}) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	resources := make([]BrowserCaptureResourceStatus, 0, len(r.Resources))
	for _, v := range r.Resources {
		if !validBridgeBrowserCaptureResource(v) {
			return BrowserCaptureStatus{}, ErrBridgeRequestFailed
		}
		resources = append(resources, BrowserCaptureResourceStatus{ResourceID: v.ResourceID, Coverage: v.Coverage, Status: v.Status})
	}
	if !sameBrowserCaptureResources(resources, r.Scope.ResourceIDs) || !validBridgeBrowserCaptureFinalization(r.Status, r.Scope, resources, r.Receipt, r.Staging) {
		return BrowserCaptureStatus{}, ErrBridgeRequestFailed
	}
	return BrowserCaptureStatus{RunID: r.RunID, Status: r.Status, ManifestVersion: r.ManifestVersion, Scope: canonicalBrowserCaptureScope(r.Scope), Resources: resources, Receipt: r.Receipt, Staging: r.Staging}, nil
}

func validBridgeBrowserCaptureResource(resource bridgeBrowserCaptureResource) bool {
	if !safeBridgeID(resource.ResourceID) || resource.ByteCount < 0 {
		return false
	}
	switch resource.Coverage {
	case "export_csv":
		switch resource.Status {
		case "pending":
			return resource.SHA256 == "" && resource.ByteCount == 0
		case "completed":
			return validSHA256(resource.SHA256) && resource.ByteCount > 0
		}
	case "page_only":
		return resource.Status == "blocked" && resource.SHA256 == "" && resource.ByteCount == 0
	}
	return false
}

func validBridgeBrowserCaptureFinalization(status string, scope BrowserCaptureScope, resources []BrowserCaptureResourceStatus, receipt *BrowserCaptureCoverageReceipt, staging *BrowserCaptureStaging) bool {
	switch status {
	case "open":
		return receipt == nil && staging == nil
	case "finalized_partial":
		if scope.Mode != "partial" || !validBridgeBrowserCaptureReceipt(receipt, resources, false) {
			return false
		}
	case "finalized_full_blocked":
		if scope.Mode != "full" || !validBridgeBrowserCaptureReceipt(receipt, resources, true) || staging != nil {
			return false
		}
	default:
		return false
	}
	return validBridgeBrowserCaptureStaging(staging)
}

func validBridgeBrowserCaptureReceipt(receipt *BrowserCaptureCoverageReceipt, resources []BrowserCaptureResourceStatus, full bool) bool {
	if receipt == nil || !validRFC3339(receipt.FinalizedAt) || receipt.CompletedExportCount < 0 || receipt.RequiredExportCount < 0 || receipt.BlockedPageOnlyCount < 0 {
		return false
	}
	exports, completed, blocked := 0, 0, 0
	for _, resource := range resources {
		switch resource.Coverage {
		case "export_csv":
			exports++
			if resource.Status == "completed" {
				completed++
			}
		case "page_only":
			blocked++
		}
	}
	if receipt.CompletedExportCount != completed || receipt.RequiredExportCount != exports || receipt.BlockedPageOnlyCount != blocked {
		return false
	}
	if full {
		if receipt.Status != "full_coverage_blocked" || receipt.Ready || len(receipt.Issues) == 0 {
			return false
		}
		for _, issue := range receipt.Issues {
			if !safeBridgeID(issue.Code) || (issue.ResourceID != "" && !scopeAllowsResource(BrowserCaptureScope{ResourceIDs: resourceIDs(resources)}, issue.ResourceID)) {
				return false
			}
		}
		return true
	}
	return receipt.Status == "partial_coverage_recorded" && !receipt.Ready && len(receipt.Issues) == 0
}

func resourceIDs(resources []BrowserCaptureResourceStatus) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.ResourceID)
	}
	return ids
}

func validBridgeBrowserCaptureStaging(staging *BrowserCaptureStaging) bool {
	if staging == nil {
		return true
	}
	if staging.RecordChunksAcknowledged < 0 || staging.ArtifactChunksAcknowledged < 0 {
		return false
	}
	// The bridge can safely report compilation before it has created a
	// canonical package. This is progress only: no package receipt exists yet
	// and callers must keep the batch in CAPTURE_RUNNING.
	if staging.Status == "compiling" {
		return staging.PackageID == "" && staging.PackageSHA256 == "" && staging.IssueCode == "" && staging.RecordChunksAcknowledged == 0 && staging.ArtifactChunksAcknowledged == 0 && !staging.Finalized
	}
	if staging.Status == "review_required" {
		return staging.PackageID == "" && staging.PackageSHA256 == "" && staging.IssueCode == "browser_csv_schema_or_journal_review_required" && staging.RecordChunksAcknowledged == 0 && staging.ArtifactChunksAcknowledged == 0 && !staging.Finalized
	}
	if !safeBridgeID(staging.PackageID) || !validSHA256(staging.PackageSHA256) || staging.IssueCode != "" {
		return false
	}
	switch staging.Status {
	case "compiled_private", "pending_receiver_configuration", "staging", "staging_retry_required":
		return !staging.Finalized
	case "staged_review_required":
		return staging.Finalized
	default:
		return false
	}
}

type bridgeCaptureResponse struct {
	ConnectionID        string                  `json:"connection_id"`
	RunID               string                  `json:"run_id"`
	Status              string                  `json:"status"`
	SourceCompanyID     string                  `json:"source_company_id"`
	SourceIdentityLabel string                  `json:"source_identity_label"`
	Scope               bridgeCaptureScope      `json:"scope"`
	Resources           []bridgeCaptureResource `json:"resources"`
	Summary             CaptureSummary          `json:"summary"`
	Staging             *bridgeCaptureStaging   `json:"staging,omitempty"`
}
type bridgeCaptureStaging struct {
	PackageID                  string `json:"package_id"`
	PackageSHA256              string `json:"package_sha256"`
	Status                     string `json:"status"`
	RecordChunksAcknowledged   int    `json:"record_chunks_acknowledged"`
	ArtifactChunksAcknowledged int    `json:"artifact_chunks_acknowledged"`
	Finalized                  bool   `json:"finalized"`
}

type bridgeCaptureScope struct {
	Mode           string   `json:"mode"`
	DateFrom       string   `json:"date_from,omitempty"`
	DateTo         string   `json:"date_to,omitempty"`
	ResourceIDs    []string `json:"resource_ids"`
	SourceAsOfDate string   `json:"source_as_of_date"`
	CutoffAt       string   `json:"cutoff_at"`
}

// bridgeCaptureResource deliberately omits the bridge cursor when converted
// into CaptureResourceStatus. Cursor values are source-control state, not UI
// progress metadata.
type bridgeCaptureResource struct {
	ResourceID     string `json:"resource_id"`
	EndpointStatus string `json:"endpoint_status"`
	Status         string `json:"status"`
	ReasonCode     string `json:"reason_code"`
	PageCount      int    `json:"page_count"`
	DeletedCount   int    `json:"deleted_count"`
	ByteCount      int64  `json:"byte_count"`
	SHA256         string `json:"sha256"`
	ScopeSHA256    string `json:"scope_sha256"`
	NextEligibleAt string `json:"next_eligible_at"`
}

func (response bridgeCaptureResponse) toCaptureProgress(connectionID string) (CaptureProgress, error) {
	if response.ConnectionID != connectionID || !safeBridgeID(response.RunID) || strings.TrimSpace(response.Status) == "" || !safeBridgeID(response.SourceCompanyID) || strings.TrimSpace(response.SourceIdentityLabel) == "" || !validCaptureScope(response.Scope) || response.Summary.Total < 0 || response.Summary.Completed < 0 || response.Summary.Running < 0 || response.Summary.Interrupted < 0 || response.Summary.RateLimited < 0 || response.Summary.ReviewRequired < 0 || response.Summary.DependencyRequired < 0 || response.Summary.BraveDiscoveryRequired < 0 {
		return CaptureProgress{}, ErrBridgeRequestFailed
	}
	resources := make([]CaptureResourceStatus, 0, len(response.Resources))
	for _, resource := range response.Resources {
		if strings.TrimSpace(resource.ResourceID) == "" || strings.TrimSpace(resource.EndpointStatus) == "" || strings.TrimSpace(resource.Status) == "" || resource.PageCount < 0 || resource.DeletedCount < 0 || resource.ByteCount < 0 || (resource.SHA256 != "" && !validSHA256(resource.SHA256)) || (resource.ScopeSHA256 != "" && !validSHA256(resource.ScopeSHA256)) || (resource.NextEligibleAt != "" && !validRFC3339(resource.NextEligibleAt)) {
			return CaptureProgress{}, ErrBridgeRequestFailed
		}
		resources = append(resources, CaptureResourceStatus{
			ResourceID:     resource.ResourceID,
			EndpointStatus: resource.EndpointStatus,
			Status:         resource.Status,
			ReasonCode:     resource.ReasonCode,
			PageCount:      resource.PageCount,
			DeletedCount:   resource.DeletedCount,
			ByteCount:      resource.ByteCount,
			SHA256:         resource.SHA256,
			ScopeSHA256:    resource.ScopeSHA256,
			NextEligibleAt: resource.NextEligibleAt,
		})
	}
	resourceIDs := append([]string(nil), response.Scope.ResourceIDs...)
	if len(resourceIDs) == 0 {
		// Compatibility with an already-running bridge release. The response
		// still names every selected resource, so retain an exact safe scope
		// rather than losing a pending capture during a rolling update.
		for _, resource := range resources {
			resourceIDs = append(resourceIDs, resource.ResourceID)
		}
	}
	progress := CaptureProgress{RunID: response.RunID, Status: response.Status, ScopeMode: response.Scope.Mode, DateFrom: response.Scope.DateFrom, DateTo: response.Scope.DateTo, ResourceIDs: resourceIDs, SourceAsOfDate: response.Scope.SourceAsOfDate, CutoffAt: response.Scope.CutoffAt, Resources: resources, Summary: response.Summary}
	if response.Staging != nil {
		staging := response.Staging
		if !safeBridgeID(staging.PackageID) || !validSHA256(staging.PackageSHA256) || strings.TrimSpace(staging.Status) == "" || staging.RecordChunksAcknowledged < 0 || staging.ArtifactChunksAcknowledged < 0 {
			return CaptureProgress{}, ErrBridgeRequestFailed
		}
		progress.Staging = &CaptureStaging{PackageID: staging.PackageID, PackageSHA256: staging.PackageSHA256, Status: staging.Status, RecordChunksAcknowledged: staging.RecordChunksAcknowledged, ArtifactChunksAcknowledged: staging.ArtifactChunksAcknowledged, Finalized: staging.Finalized}
	}
	return progress, nil
}

func validCaptureRequest(request CaptureRequest) bool {
	scopeMode := strings.TrimSpace(request.ScopeMode)
	validScope := false
	switch scopeMode {
	case "", "window":
		validScope = validCaptureDateRange(request.DateFrom, request.DateTo)
	case "full_history":
		validScope = request.DateFrom == "" && request.DateTo == ""
	}
	return validScope && validCaptureResourceIDs(request.ResourceIDs) && (request.MaxPages == 0 || (request.MaxPages >= 1 && request.MaxPages <= 1000)) && (request.RateBudget == 0 || (request.RateBudget >= 1 && request.RateBudget <= 1000)) && (request.ResumeRunID == "" || safeBridgeID(request.ResumeRunID))
}

func validCaptureScope(scope bridgeCaptureScope) bool {
	if !validRFC3339(scope.CutoffAt) {
		return false
	}
	if _, err := time.Parse(time.DateOnly, scope.SourceAsOfDate); err != nil {
		return false
	}
	if !validCaptureResourceIDs(scope.ResourceIDs) {
		return false
	}
	switch scope.Mode {
	case "window":
		return validCaptureDateRange(scope.DateFrom, scope.DateTo)
	case "full_history":
		return scope.DateFrom == "" && scope.DateTo == ""
	default:
		return false
	}
}

func validCaptureResourceIDs(resourceIDs []string) bool {
	if len(resourceIDs) > 40 {
		return false
	}
	seen := make(map[string]struct{}, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		if !safeBridgeID(resourceID) {
			return false
		}
		if _, exists := seen[resourceID]; exists {
			return false
		}
		seen[resourceID] = struct{}{}
	}
	return true
}

func validRFC3339(value string) bool {
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func validCaptureDateRange(dateFrom, dateTo string) bool {
	from, err := time.Parse(time.DateOnly, dateFrom)
	if err != nil {
		return false
	}
	to, err := time.Parse(time.DateOnly, dateTo)
	return err == nil && !to.Before(from)
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func (c *HTTPBridgeClient) authorizationHeader(tenantID string) (string, error) {
	if !safeBridgeID(tenantID) || len(c.tokenSecret) < 16 {
		return "", errors.New("invalid bridge token claims")
	}
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	header := "v1." + base64.RawURLEncoding.EncodeToString([]byte(tenantID)) + "." + strconv.FormatInt(now().UTC().Add(bridgeTokenLifetime).Unix(), 10)
	signature := hmac.New(sha256.New, c.tokenSecret)
	_, _ = signature.Write([]byte(header))
	return "Bearer " + header + "." + hex.EncodeToString(signature.Sum(nil)), nil
}

func safeBridgeID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func isExpectedBridgeSecretReference(reference, connectionID string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(reference))
	if err != nil || parsed.Scheme != "secret-ref" || parsed.Host != "sa-bridge" || parsed.User != nil {
		return false
	}
	return parsed.Path == "/"+connectionID && parsed.RawQuery == "" && parsed.Fragment == ""
}

func bridgeConnectionID(reference string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(reference))
	if err != nil || parsed.Scheme != "secret-ref" || parsed.Host != "sa-bridge" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("bridge secret reference is invalid")
	}
	connectionID := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Path != "/"+connectionID || !safeBridgeID(connectionID) {
		return "", errors.New("bridge secret reference is invalid")
	}
	return connectionID, nil
}

// ConfiguredBridgeCatalog reports that a private bridge is configured. The
// v1.7 source has no company-discovery endpoint, so it deliberately returns no
// guessed source metadata. A validated connection supplies the source identity.
type ConfiguredBridgeCatalog struct{}

func (ConfiguredBridgeCatalog) Discover(_ context.Context, _ string) (SourceDiscovery, error) {
	return SourceDiscovery{
		BridgeAvailable:   true,
		LiveDataContacted: false,
		Sources:           []SourceCandidate{},
	}, nil
}

// ValidateConnectionRequest checks only request policy before a handler sends
// the opaque external credential reference to the bridge. Source identity
// cannot be supplied by the browser: the bridge derives it from the provider
// credential during validation.
func (s *Service) ValidateConnectionRequest(_ context.Context, tenantID string, req ConnectRequest) error {
	if s == nil || s.store == nil {
		return errors.New("SmartAccounts sync storage is not configured")
	}
	if err := validateConnectionPolicy(tenantID, req); err != nil {
		return err
	}
	if _, _, err := normalizeSourceCredentialReference(req.SourceCredentialReference); err != nil {
		return errors.New("SmartAccounts source credential reference is required")
	}
	return nil
}

func validateConnectionPolicy(tenantID string, req ConnectRequest) error {
	if !safeBridgeID(strings.TrimSpace(tenantID)) {
		return errors.New("tenant id is required")
	}
	if !req.SmartAccountsGLAuthoritative {
		return errors.New("SmartAccounts GL authority must be explicitly confirmed")
	}
	if strings.TrimSpace(req.InvoicePaymentMode) != InvoicePaymentModeNonPosting {
		return errors.New("invoice and payment records must remain NON_POSTING for the GL-authoritative sync")
	}
	return nil
}
