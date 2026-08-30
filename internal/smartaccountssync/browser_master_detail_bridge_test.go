package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBridgeClientUsesExactPrivateMasterDetailWireContract(t *testing.T) {
	const (
		tenantID = browserPairingTenantID
		runID    = "389f6fec-1994-4cfe-8ea6-bb7281d3050f"
		token    = "bridge-token-secret-for-test-123456"
	)
	fixedNow := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	contract, schemaID, sourceSchema, found := browserMasterDetailContractFor(BrowserMasterDetailClientsResource)
	require.True(t, found)
	contractSHA, err := browserMasterDetailSHA256(contract)
	require.NoError(t, err)
	request := BrowserMasterDetailStartRequest{
		SourceCompanyID: browserSourceID, ManifestVersion: BrowserMasterDetailManifestVersion,
		ResourceID: BrowserMasterDetailClientsResource, SchemaID: schemaID, Contract: contract,
		Scope:          BrowserMasterDetailScope{FromInclusive: "2026-08-28", ToInclusive: "2026-08-28", CutoffAt: "2026-08-28T10:00:00Z"},
		ApprovalSHA256: strings.Repeat("a", 64),
		Consent:        BrowserMasterDetailTransferConsent{Version: BrowserMasterDetailConsentVersion, Confirmed: true, ConfirmedAt: fixedNow},
	}
	body := []byte(`{"external_id":"1"}` + "\n")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	privateOpen := `{"run_id":"` + runID + `","status":"open","manifest_version":"smartaccounts-browser-master-detail-v1","resource_id":"clients","schema_id":"clients_detail_v1","source_schema":"smartaccounts-browser-master-detail-v1/clients_detail_v1","contract_sha256":"` + contractSHA + `"}`
	privateFinal := `{"run_id":"` + runID + `","status":"finalized","manifest_version":"smartaccounts-browser-master-detail-v1","resource_id":"clients","schema_id":"clients_detail_v1","source_schema":"smartaccounts-browser-master-detail-v1/clients_detail_v1","contract_sha256":"` + contractSHA + `","ndjson_sha256":"` + digest + `","record_count":2,"package_id":"master-detail-package","package_sha256":"` + strings.Repeat("b", 64) + `"}`

	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		w.Header().Set("Cache-Control", "no-store")
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /v1/browser-master-detail-captures/" + runID:
			received, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			// The contract fixture intentionally has the bridge's response-only
			// fields absent. Start payload must contain only private start fields.
			assert.JSONEq(t, `{"source_company_id":"sa-browser-v1-123456","manifest_version":"smartaccounts-browser-master-detail-v1","resource_id":"clients","schema_id":"clients_detail_v1","contract":{"version":"smartaccounts-browser-master-detail-v1","resource":"clients","origin":"https://sa.smartaccounts.eu","list_page_path":"/et/clients","detail_path_prefix":"/et/clients.change/","detail_result_page_path":"/et/clients","fields":[{"name":"name","kind":"string","required":true},{"name":"regCode","kind":"string"},{"name":"vatNumber","kind":"string"},{"name":"address","kind":"object"},{"name":"countrySubmittedInputValue","kind":"string"}]},"scope":{"from_inclusive":"2026-08-28","to_inclusive":"2026-08-28","cutoff_at":"2026-08-28T10:00:00Z"},"approval_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","transfer_consent":{"version":"smartaccounts-browser-master-detail-transfer-consent-v1","confirmed":true,"confirmed_at":"2026-08-28T10:00:00Z"}}`, string(received))
			_, _ = w.Write([]byte(privateOpen))
		case http.MethodGet + " /v1/browser-master-detail-captures/" + runID:
			_, _ = w.Write([]byte(privateOpen))
		case http.MethodPut + " /v1/browser-master-detail-captures/" + runID + "/resource":
			assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
			assert.Equal(t, digest, r.Header.Get("X-SA-Browser-Resource-SHA256"))
			received, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, body, received)
			_, _ = w.Write([]byte(`{"run_id":"` + runID + `","status":"accepted","created":true}`))
		case http.MethodPost + " /v1/browser-master-detail-captures/" + runID + "/finalize":
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Zero(t, r.ContentLength)
			received, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Empty(t, received)
			_, _ = w.Write([]byte(privateFinal))
		default:
			http.NotFound(w, r)
		}
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient, client.now = bridge.Client(), func() time.Time { return fixedNow }

	started, err := client.StartBrowserMasterDetail(context.Background(), tenantID, runID, request)
	require.NoError(t, err)
	assert.Equal(t, "open", started.Status)
	assert.Empty(t, started.Scope)
	assert.Empty(t, started.ApprovalSHA256)
	assert.Empty(t, started.TenantID)
	accepted, err := client.UploadBrowserMasterDetail(context.Background(), tenantID, runID, digest, body)
	require.NoError(t, err)
	assert.True(t, accepted.Created)
	finalized, err := client.FinalizeBrowserMasterDetail(context.Background(), tenantID, runID)
	require.NoError(t, err)
	assert.Equal(t, "master-detail-package", finalized.PackageID)
	status, err := client.GetBrowserMasterDetail(context.Background(), tenantID, runID)
	require.NoError(t, err)
	assert.Equal(t, sourceSchema, status.SourceSchema)
	assert.Empty(t, status.Scope)
	assert.Empty(t, status.ApprovalSHA256)
}

func TestHTTPBridgeClientRejectsUnexpectedMasterDetailStatusFields(t *testing.T) {
	const runID = "389f6fec-1994-4cfe-8ea6-bb7281d3050f"
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"run_id":"` + runID + `","status":"open","manifest_version":"smartaccounts-browser-master-detail-v1","resource_id":"clients","schema_id":"clients_detail_v1","source_schema":"smartaccounts-browser-master-detail-v1/clients_detail_v1","contract_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","approval_sha256":"must-not-be-visible"}`))
	}))
	defer bridge.Close()
	client, err := NewHTTPBridgeClient(bridge.URL, "bridge-token-secret-for-test-123456")
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	_, err = client.GetBrowserMasterDetail(context.Background(), browserPairingTenantID, runID)
	require.ErrorIs(t, err, ErrBridgeRequestFailed)
	auth := &BrowserMasterDetailAuthorization{RunID: runID, TenantID: browserPairingTenantID, BatchID: "8e2f475d-1d3d-4d6d-8639-97ae56083cd1", SourceCompanyID: browserSourceID, SnapshotDate: "2026-08-28", ManifestVersion: BrowserMasterDetailManifestVersion, ResourceID: BrowserMasterDetailClientsResource, SchemaID: "clients_detail_v1", SourceSchema: BrowserMasterDetailClientsSchema, Contract: func() BrowserMasterDetailContract {
		value, _, _, _ := browserMasterDetailContractFor(BrowserMasterDetailClientsResource)
		return value
	}(), ContractSHA256: strings.Repeat("a", 64), ApprovalSHA256: strings.Repeat("b", 64), Scope: BrowserMasterDetailScope{FromInclusive: "2026-08-28", ToInclusive: "2026-08-28", CutoffAt: "2026-08-28T10:00:00Z"}, TokenSHA256: strings.Repeat("c", 64), CreatedBy: "owner", CreatedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 8, 28, 10, 10, 0, 0, time.UTC)}
	contractSHA, _ := browserMasterDetailSHA256(auth.Contract)
	auth.ContractSHA256 = contractSHA
	unsafe := BrowserMasterDetailStatus{RunID: runID, Status: "open", ManifestVersion: BrowserMasterDetailManifestVersion, ResourceID: BrowserMasterDetailClientsResource, SchemaID: "clients_detail_v1", SourceSchema: BrowserMasterDetailClientsSchema, ContractSHA256: contractSHA, ApprovalSHA256: "must-not-be-visible"}
	assert.False(t, sameBrowserMasterDetailStatus(unsafe, auth))
}
