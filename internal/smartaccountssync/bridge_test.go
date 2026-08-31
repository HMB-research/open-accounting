package smartaccountssync

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBridgeClientConnectsAndValidatesWithOpaqueCredentialReference(t *testing.T) {
	const (
		tenantID                  = "tenant-alpha"
		token                     = "bridge-token-secret-for-test-123456"
		sourceCredentialReference = "secret-ref://file/connection-alpha"
		connectionID              = "connection-alpha"
	)
	fixedNow := time.Date(2026, 8, 27, 15, 0, 0, 0, time.UTC)
	var requests []string

	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case http.MethodPut + " /v1/connections/" + connectionID:
			var input map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			assert.Equal(t, map[string]string{"source_credential_reference": sourceCredentialReference}, input)
			assert.Empty(t, r.Header.Get("X-SmartAccounts-Source"))
			writeBridgeJSON(w, bridgeConnectionResponse{
				ConnectionID:        connectionID,
				SecretReference:     "secret-ref://sa-bridge/" + connectionID,
				Configured:          true,
				SourceCompanyID:     "sa-key-v1-test-identity",
				SourceIdentityLabel: "SmartAccounts source",
			})
		case http.MethodPost + " /v1/connections/" + connectionID + "/validate":
			assert.Zero(t, r.ContentLength)
			writeBridgeJSON(w, bridgeValidationResponse{ConnectionID: connectionID, Status: "connected", AccountCount: 3, SourceCompanyID: "sa-key-v1-test-identity", SourceIdentityLabel: "SmartAccounts source", SourceBindingStatus: "api_key_identity_and_snapshot_validated", AccountSnapshotSHA256: strings.Repeat("a", 64)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	client.now = func() time.Time { return fixedNow }

	connection, err := client.ConnectAndValidate(context.Background(), tenantID, sourceCredentialReference)

	require.NoError(t, err)
	assert.Equal(t, connectionID, connection.ConnectionID)
	assert.Equal(t, "secret-ref://sa-bridge/"+connectionID, connection.SecretReference)
	assert.Equal(t, "connected", connection.ValidationStatus)
	assert.Equal(t, 3, connection.AccountCount)
	assert.Equal(t, "sa-key-v1-test-identity", connection.SourceCompanyID)
	assert.Equal(t, []string{
		http.MethodPut + " /v1/connections/" + connectionID,
		http.MethodPost + " /v1/connections/" + connectionID + "/validate",
	}, requests)
}

func TestCaptureProgressExposesValidatedStagingMetadataOnly(t *testing.T) {
	progress, err := (bridgeCaptureResponse{
		ConnectionID: "connection-1", RunID: "capture-1", Status: "complete", SourceCompanyID: "sa-key-v1-source", SourceIdentityLabel: "SmartAccounts source",
		Scope:   bridgeCaptureScope{Mode: "full_history", SourceAsOfDate: "2026-08-27", CutoffAt: "2026-08-27T15:00:00Z"},
		Staging: &bridgeCaptureStaging{PackageID: "package-1", PackageSHA256: strings.Repeat("a", 64), Status: "staged_review_required", RecordChunksAcknowledged: 2, ArtifactChunksAcknowledged: 1, Finalized: true},
	}).toCaptureProgress("connection-1")
	require.NoError(t, err)
	require.NotNil(t, progress.Staging)
	assert.Equal(t, "package-1", progress.Staging.PackageID)
	assert.Equal(t, 2, progress.Staging.RecordChunksAcknowledged)
	assert.True(t, progress.Staging.Finalized)
	assert.Equal(t, "full_history", progress.ScopeMode)
	assert.Empty(t, progress.DateFrom)
	assert.Equal(t, "2026-08-27", progress.SourceAsOfDate)
}

func TestBridgeBrowserCaptureAcceptsCompilingBeforeAPackageReceiptExists(t *testing.T) {
	scope := BrowserCaptureScope{Mode: "partial", FromInclusive: "2020-01-01", ToInclusive: "2026-08-28", CutoffAt: "2026-08-28T12:00:00Z", ResourceIDs: []string{"journal_entries"}}
	resources := []BrowserCaptureResourceStatus{{ResourceID: "journal_entries", Coverage: "export_csv", Status: "completed"}}
	receipt := &BrowserCaptureCoverageReceipt{Status: "partial_coverage_recorded", Ready: false, CompletedExportCount: 1, RequiredExportCount: 1, FinalizedAt: "2026-08-28T12:00:00Z"}
	assert.True(t, validBridgeBrowserCaptureFinalization("finalized_partial", scope, resources, receipt, &BrowserCaptureStaging{Status: "compiling"}))
	assert.False(t, validBridgeBrowserCaptureFinalization("finalized_partial", scope, resources, receipt, &BrowserCaptureStaging{Status: "compiling", PackageID: "must-not-exist"}))
}

func TestHTTPBridgeClientAllowsExplicitFullHistoryWithoutAnInventedDateRange(t *testing.T) {
	request := CaptureRequest{ScopeMode: "full_history"}
	assert.True(t, validCaptureRequest(request))
	assert.False(t, validCaptureRequest(CaptureRequest{ScopeMode: "full_history", DateFrom: "2020-01-01"}))
	assert.True(t, validCaptureRequest(CaptureRequest{ScopeMode: "window", DateFrom: "2020-01-01", DateTo: "2020-01-31"}))
	assert.False(t, validCaptureRequest(CaptureRequest{ScopeMode: "window"}))
}

func TestHTTPBridgeClientStartsFullHistoryWithoutDateParameters(t *testing.T) {
	const tenantID = "tenant-alpha"
	const token = "bridge-token-secret-for-test-123456"
	fixedNow := time.Date(2026, 8, 27, 15, 0, 0, 0, time.UTC)
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/connections/connection-1/captures", r.URL.Path)
		var input map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
		assert.Equal(t, map[string]any{"scope_mode": "full_history"}, input)
		writeBridgeJSON(w, bridgeCaptureResponse{
			ConnectionID: "connection-1", RunID: "capture-1", Status: "running", SourceCompanyID: "sa-key-v1-source", SourceIdentityLabel: "SmartAccounts source",
			Scope:   bridgeCaptureScope{Mode: "full_history", SourceAsOfDate: "2026-08-27", CutoffAt: "2026-08-27T15:00:00Z"},
			Summary: CaptureSummary{Total: 40, Running: 24, ReviewRequired: 2, BraveDiscoveryRequired: 2},
		})
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	client.now = func() time.Time { return fixedNow }
	progress, err := client.StartCapture(context.Background(), tenantID, "connection-1", CaptureRequest{ScopeMode: "full_history"})

	require.NoError(t, err)
	assert.Equal(t, "full_history", progress.ScopeMode)
	assert.Equal(t, "2026-08-27", progress.SourceAsOfDate)
	assert.Equal(t, 2, progress.Summary.ReviewRequired)
}

func TestHTTPBridgeClientForwardsExplicitWindowResourceSelection(t *testing.T) {
	const tenantID = "tenant-alpha"
	const token = "bridge-token-secret-for-test-123456"
	fixedNow := time.Date(2026, 8, 27, 15, 0, 0, 0, time.UTC)
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		var input map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
		assert.Equal(t, map[string]any{
			"scope_mode":   "window",
			"date_from":    "2020-01-01",
			"date_to":      "2020-01-31",
			"resource_ids": []any{"inventory.warehouse_movements.get", "payroll.worker_absences.get"},
		}, input)
		writeBridgeJSON(w, bridgeCaptureResponse{
			ConnectionID: "connection-1", RunID: "capture-window-1", Status: "running", SourceCompanyID: "sa-key-v1-source", SourceIdentityLabel: "SmartAccounts source",
			Scope:   bridgeCaptureScope{Mode: "window", DateFrom: "2020-01-01", DateTo: "2020-01-31", ResourceIDs: []string{"inventory.warehouse_movements.get", "payroll.worker_absences.get"}, SourceAsOfDate: "2026-08-27", CutoffAt: "2026-08-27T15:00:00Z"},
			Summary: CaptureSummary{Total: 2, Running: 2},
		})
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	client.now = func() time.Time { return fixedNow }
	progress, err := client.StartCapture(context.Background(), tenantID, "connection-1", CaptureRequest{ScopeMode: "window", DateFrom: "2020-01-01", DateTo: "2020-01-31", ResourceIDs: []string{"inventory.warehouse_movements.get", "payroll.worker_absences.get"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"inventory.warehouse_movements.get", "payroll.worker_absences.get"}, progress.ResourceIDs)
}

func TestHTTPBridgeClientRedactsBridgeFailuresAndDoesNotValidateAfterFailedConnect(t *testing.T) {
	const sourceCredentialReference = "secret-ref://file/connection-alpha"
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream rejected source credential reference"))
			return
		}
		t.Fatal("validate must not run after a failed credential PUT")
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, "bridge-token-secret-for-test-123456")
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	_, err = client.ConnectAndValidate(context.Background(), "tenant-alpha", sourceCredentialReference)

	assert.ErrorIs(t, err, ErrBridgeRequestFailed)
	assert.NotContains(t, err.Error(), sourceCredentialReference)
}

func TestHTTPBridgeClientHealthVerifiesDataFreeBridgeCapabilities(t *testing.T) {
	var requests []string
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.Header.Get("Authorization") != "" || request.URL.RawQuery != "" || request.ContentLength > 0 {
			t.Fatalf("unexpected readiness request: %s %s", request.Method, request.URL.String())
		}
		requests = append(requests, request.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/capabilities":
			w.Header().Set("Cache-Control", "no-store")
			writeBridgeJSON(w, validBridgeCapabilitiesResponse())
		default:
			http.NotFound(w, request)
		}
	}))
	defer bridge.Close()
	client, err := NewHTTPBridgeClient(bridge.URL, "bridge-token-secret-for-test-123456")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Health(context.Background()); err != nil || !assert.Equal(t, []string{"/health", "/capabilities"}, requests) {
		t.Fatalf("health failed: requests=%v err=%v", requests, err)
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", bridge.URL+"/health")
		w.WriteHeader(http.StatusFound)
	}))
	defer redirect.Close()
	client, err = NewHTTPBridgeClient(redirect.URL, "bridge-token-secret-for-test-123456")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Health(context.Background()); !errors.Is(err, ErrBridgeRequestFailed) {
		t.Fatalf("redirect health error = %v", err)
	}
}

func TestHTTPBridgeClientHealthFailsClosedForIncompatibleCapabilities(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload string
	}{
		{name: "missing required field", payload: `{"schema_version":"sa-bridge-capabilities-v1"}`},
		{name: "unknown field", payload: `{"schema_version":"sa-bridge-capabilities-v1","bridge_api_version":"sa-bridge-api-v1","bridge_version":"bridge-v1","bridge_commit":"abc123","browser_discovery_protocol_version":"smartaccounts-browser-discovery-protocol-v1","browser_discovery_contract_version":"smartaccounts-brave-discovery-contract-v1","browser_capture_manifest_version":"smartaccounts-brave-ui-v2","browser_capture_protocol_version":"smartaccounts-browser-capture-bridge-v1","browser_csv_schema_registry_version":"smartaccounts-browser-csv-schema-registry-v1","browser_csv_schema_review_version":"smartaccounts-browser-csv-schema-review-v1","oa_staging_protocol_version":"open-accounting-import-delivery-v1","source_company_id":"must-not-be-accepted"}`},
		{name: "wrong staging protocol", payload: `{"schema_version":"sa-bridge-capabilities-v1","bridge_api_version":"sa-bridge-api-v1","bridge_version":"bridge-v1","bridge_commit":"abc123","browser_discovery_protocol_version":"smartaccounts-browser-discovery-protocol-v1","browser_discovery_contract_version":"smartaccounts-brave-discovery-contract-v1","browser_capture_manifest_version":"smartaccounts-brave-ui-v2","browser_capture_protocol_version":"smartaccounts-browser-capture-bridge-v1","browser_csv_schema_registry_version":"smartaccounts-browser-csv-schema-registry-v1","browser_csv_schema_review_version":"smartaccounts-browser-csv-schema-review-v1","oa_staging_protocol_version":"unexpected-v1"}`},
		{name: "missing no store", payload: `{"schema_version":"sa-bridge-capabilities-v1","bridge_api_version":"sa-bridge-api-v1","bridge_version":"bridge-v1","bridge_commit":"abc123","browser_discovery_protocol_version":"smartaccounts-browser-discovery-protocol-v1","browser_discovery_contract_version":"smartaccounts-brave-discovery-contract-v1","browser_capture_manifest_version":"smartaccounts-brave-ui-v2","browser_capture_protocol_version":"smartaccounts-browser-capture-bridge-v1","browser_csv_schema_registry_version":"smartaccounts-browser-csv-schema-registry-v1","browser_csv_schema_review_version":"smartaccounts-browser-csv-schema-review-v1","oa_staging_protocol_version":"open-accounting-import-delivery-v1"}`},
		{name: "trailing JSON", payload: `{"schema_version":"sa-bridge-capabilities-v1","bridge_api_version":"sa-bridge-api-v1","bridge_version":"bridge-v1","bridge_commit":"abc123","browser_discovery_protocol_version":"smartaccounts-browser-discovery-protocol-v1","browser_discovery_contract_version":"smartaccounts-brave-discovery-contract-v1","browser_capture_manifest_version":"smartaccounts-brave-ui-v2","browser_capture_protocol_version":"smartaccounts-browser-capture-bridge-v1","browser_csv_schema_registry_version":"smartaccounts-browser-csv-schema-registry-v1","browser_csv_schema_review_version":"smartaccounts-browser-csv-schema-review-v1","oa_staging_protocol_version":"open-accounting-import-delivery-v1"}{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/health":
					writeBridgeJSON(w, map[string]string{"status": "ok"})
				case "/capabilities":
					w.Header().Set("Content-Type", "application/json")
					if test.name != "missing no store" {
						w.Header().Set("Cache-Control", "no-store")
					}
					_, _ = w.Write([]byte(test.payload))
				default:
					http.NotFound(w, request)
				}
			}))
			defer bridge.Close()

			client, err := NewHTTPBridgeClient(bridge.URL, "bridge-token-secret-for-test-123456")
			require.NoError(t, err)
			client.httpClient = bridge.Client()
			err = client.Health(context.Background())
			require.ErrorIs(t, err, ErrBridgeRequestFailed)
			assert.NotContains(t, err.Error(), "source_company_id")
		})
	}
}

func validBridgeCapabilitiesResponse() bridgeCapabilitiesResponse {
	return bridgeCapabilitiesResponse{
		SchemaVersion:                   bridgeCapabilitiesSchemaVersion,
		BridgeAPIVersion:                bridgeCapabilitiesAPIVersion,
		BridgeVersion:                   "bridge-v1",
		BridgeCommit:                    "abc123",
		BrowserDiscoveryProtocolVersion: bridgeDiscoveryProtocolVersion,
		BrowserDiscoveryContractVersion: bridgeDiscoveryContractVersion,
		BrowserCaptureManifestVersion:   bridgeCaptureManifestVersion,
		BrowserCaptureProtocolVersion:   bridgeCaptureProtocolVersion,
		BrowserCSVSchemaRegistryVersion: bridgeCSVSchemaRegistryVersion,
		BrowserCSVSchemaReviewVersion:   bridgeCSVSchemaReviewVersion,
		OAStagingProtocolVersion:        bridgeOAStagingProtocolVersion,
	}
}

func TestHTTPBridgeClientProxiesOnlyTheExactRedactedBrowserDiscoveryReceipt(t *testing.T) {
	const (
		tenantID = testBrowserDiscoveryTenantID
		token    = "bridge-discovery-token-secret-for-test-123456"
	)
	fixedNow := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	result := browserDiscoveryRelayResultWithHeader(testBrowserDiscoveryID)
	var postBody map[string]json.RawMessage
	requests := 0
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		require.Empty(t, r.URL.RawQuery)
		require.Equal(t, "/v1/browser-discovery-receipts/"+testBrowserDiscoverySourceID+"/"+testBrowserDiscoveryID, r.URL.Path)
		requests++
		switch r.Method {
		case http.MethodPost:
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&postBody))
			assert.Equal(t, []string{"contract_version", "manifest_version", "resources", "source_company_id", "status"}, sortedJSONKeys(postBody))
			assert.Equal(t, `"`+testBrowserDiscoverySourceID+`"`, string(postBody["source_company_id"]))
			assert.NotContains(t, string(postBody["resources"]), "cookies")
			assert.NotContains(t, string(postBody["resources"]), "source_row")
			writeBridgeJSON(w, browserDiscoveryReceipt(testBrowserDiscoveryID))
		case http.MethodGet:
			require.Zero(t, r.ContentLength)
			writeBridgeJSON(w, browserDiscoveryReceipt(testBrowserDiscoveryID))
		default:
			t.Fatalf("unexpected bridge request %s", r.Method)
		}
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	client.now = func() time.Time { return fixedNow }
	request := BrowserDiscoveryBridgeReceiptRequest{
		SourceCompanyID: testBrowserDiscoverySourceID, ManifestVersion: result.ManifestVersion,
		ContractVersion: result.ContractVersion, Status: result.Status, Resources: result.Resources,
	}
	receipt, err := client.RecordBrowserDiscoveryReceipt(context.Background(), tenantID, testBrowserDiscoverySourceID, testBrowserDiscoveryID, request)
	require.NoError(t, err)
	assert.Equal(t, browserDiscoveryReceipt(testBrowserDiscoveryID), receipt)
	receipt, err = client.GetBrowserDiscoveryReceipt(context.Background(), tenantID, testBrowserDiscoverySourceID, testBrowserDiscoveryID)
	require.NoError(t, err)
	assert.Equal(t, browserDiscoveryReceipt(testBrowserDiscoveryID), receipt)
	assert.Equal(t, 2, requests)
}

func TestHTTPBridgeClientProxiesOnlyReviewedBrowserCSVSchemaMetadata(t *testing.T) {
	const (
		tenantID = testBrowserDiscoveryTenantID
		token    = "bridge-schema-review-token-secret-for-test-123456"
	)
	fixedNow := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	reviewedAt := time.Date(2026, 8, 28, 11, 59, 59, 0, time.UTC)
	request := BrowserCSVSchemaApprovalBridgeRequest{
		DiscoveryID: testBrowserDiscoveryID, SchemaID: testBrowserCSVSchemaID,
		Review: BrowserCSVSchemaReview{Version: BrowserCSVSchemaReviewVersion, Confirmed: true, ReviewedAt: reviewedAt, AuditID: "c1a222aa-11aa-4e4e-8ee8-4a08de7310d3"},
	}
	bridgeResponse := browserCSVSchemaApprovalResponse()
	requests := 0
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		require.Empty(t, r.URL.RawQuery)
		require.Equal(t, "/v1/browser-csv-schema-approvals/"+testBrowserDiscoverySourceID+"/"+BrowserGeneralLedgerResourceID+"/"+testBrowserCSVSchemaID, r.URL.Path)
		requests++
		switch r.Method {
		case http.MethodPost:
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			var input map[string]json.RawMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			assert.Equal(t, []string{"discovery_id", "review", "schema_id"}, sortedJSONKeys(input))
			assert.Equal(t, `"`+testBrowserDiscoveryID+`"`, string(input["discovery_id"]))
			assert.Equal(t, `"`+testBrowserCSVSchemaID+`"`, string(input["schema_id"]))
			assert.NotContains(t, string(input["review"]), "header")
			assert.NotContains(t, string(input["review"]), "cookie")
			assert.NotContains(t, string(input["review"]), testBrowserDiscoverySourceID)
			writeBridgeJSON(w, bridgeResponse)
		case http.MethodGet:
			require.Zero(t, r.ContentLength)
			writeBridgeJSON(w, bridgeResponse)
		default:
			t.Fatalf("unexpected bridge request %s", r.Method)
		}
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	client.now = func() time.Time { return fixedNow }

	response, err := client.RegisterBrowserCSVSchemaApproval(context.Background(), tenantID, testBrowserDiscoverySourceID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID, request)
	require.NoError(t, err)
	assert.Equal(t, bridgeResponse, response)
	response, err = client.GetBrowserCSVSchemaApproval(context.Background(), tenantID, testBrowserDiscoverySourceID, BrowserGeneralLedgerResourceID, testBrowserCSVSchemaID)
	require.NoError(t, err)
	assert.Equal(t, bridgeResponse, response)
	assert.Equal(t, 2, requests)
}

func sortedJSONKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestNormalizeSourceCredentialReferenceUsesOnlyItsOpaqueIdentifier(t *testing.T) {
	reference, connectionID, err := normalizeSourceCredentialReference(" secret-ref://file/connection-alpha ")
	require.NoError(t, err)
	assert.Equal(t, "secret-ref://file/connection-alpha", reference)
	assert.Equal(t, "connection-alpha", connectionID)

	for _, value := range []string{
		"", "raw-api-key", "vault://secret/connection-alpha", "secret-ref://vault/connection-alpha", "secret-ref://file/connection-alpha?query=not-allowed", "secret-ref://file/connection-alpha?", "secret-ref://file/connection%2Dalpha", "secret-ref://file/connection alpha", "secret-ref://file/connection-alpha/extra",
	} {
		_, _, err := normalizeSourceCredentialReference(value)
		assert.Error(t, err, value)
	}
}

func TestConfiguredBridgeCatalogDoesNotGuessSourceIdentity(t *testing.T) {
	discovery, err := (ConfiguredBridgeCatalog{}).Discover(context.Background(), "tenant-a")

	require.NoError(t, err)
	assert.True(t, discovery.BridgeAvailable)
	assert.Empty(t, discovery.Sources)
}

func requireBridgeAuthorization(t *testing.T, request *http.Request, tenantID, secret string, now time.Time) {
	t.Helper()
	bearer := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(bearer, ".")
	require.Len(t, parts, 4)
	assert.Equal(t, "v1", parts[0])
	decodedTenant, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	assert.Equal(t, tenantID, string(decodedTenant))
	expiresAt, err := strconv.ParseInt(parts[2], 10, 64)
	require.NoError(t, err)
	assert.Equal(t, now.Add(bridgeTokenLifetime).Unix(), expiresAt)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join(parts[:3], ".")))
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), parts[3])
}

func writeBridgeJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
