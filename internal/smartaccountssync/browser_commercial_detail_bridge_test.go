package smartaccountssync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBridgeClientUsesExactPrivateCommercialDetailWireContract(t *testing.T) {
	const runID = "389f6fec-1994-4cfe-8ea6-bb7281d3050f"
	fixedNow := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	review := BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: fixedNow, AuditID: "4d8a3f84-5749-42cb-92a4-3fd2c56a702f"}
	contract, schema, sourceSchema, routeSHA, found := browserCommercialDetailContractFor(BrowserCommercialClientInvoicesResource, review)
	require.True(t, found)
	contractSHA, err := browserCommercialDetailSHA256(contract)
	require.NoError(t, err)
	consent := BrowserCommercialDetailTransferConsent{Version: BrowserCommercialDetailConsentVersion, Confirmed: true, ConfirmedAt: fixedNow}
	consentSHA, err := browserCommercialDetailConsentSHA256(consent)
	require.NoError(t, err)
	input := BrowserCommercialDetailStartRequest{SourceCompanyID: browserSourceID, ManifestVersion: BrowserCommercialDetailManifestVersion, Contract: contract, Scope: BrowserCommercialDetailScope{FromInclusive: "2026-01-01", ToInclusive: "2026-08-28", CutoffAt: "2026-08-28T10:00:00Z"}, Consent: consent}
	expected, err := json.Marshal(input)
	require.NoError(t, err)
	open := `{"run_id":"` + runID + `","status":"open","manifest_version":"smartaccounts-browser-commercial-detail-v1","resource_id":"client_invoices","schema_id":"` + schema + `","source_schema":"` + sourceSchema + `","route_sha256":"` + routeSHA + `","contract_sha256":"` + contractSHA + `","consent_sha256":"` + consentSHA + `"}`
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, browserPairingTenantID, "bridge-token-secret-for-test-123456", fixedNow)
		w.Header().Set("Cache-Control", "no-store")
		assert.Equal(t, "/v1/browser-commercial-captures/"+runID, r.URL.Path)
		switch r.Method {
		case http.MethodPost:
			body, readErr := io.ReadAll(r.Body)
			require.NoError(t, readErr)
			assert.JSONEq(t, string(expected), string(body))
		case http.MethodGet:
			assert.Zero(t, r.ContentLength)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
		_, _ = w.Write([]byte(open))
	}))
	defer bridge.Close()
	client, err := NewHTTPBridgeClient(bridge.URL, "bridge-token-secret-for-test-123456")
	require.NoError(t, err)
	client.httpClient, client.now = bridge.Client(), func() time.Time { return fixedNow }
	started, err := client.StartBrowserCommercialDetail(context.Background(), browserPairingTenantID, runID, input)
	require.NoError(t, err)
	assert.Equal(t, "open", started.Status)
	assert.Empty(t, started.WorkflowID)
	assert.Zero(t, started.Sequence)
	status, err := client.GetBrowserCommercialDetail(context.Background(), browserPairingTenantID, runID)
	require.NoError(t, err)
	assert.Equal(t, sourceSchema, status.SourceSchema)
	assert.Empty(t, status.PackageID)
}

func TestHTTPBridgeClientRejectsLeakedCommercialResponseFields(t *testing.T) {
	const runID = "389f6fec-1994-4cfe-8ea6-bb7281d3050f"
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"run_id":"` + runID + `","status":"open","manifest_version":"smartaccounts-browser-commercial-detail-v1","resource_id":"client_invoices","schema_id":"client_invoices_detail_v1","source_schema":"smartaccounts-browser-commercial-detail-v1/client_invoices_detail_v1","route_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","contract_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","consent_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","source_company_id":"must-not-leak"}`))
	}))
	defer bridge.Close()
	client, err := NewHTTPBridgeClient(bridge.URL, "bridge-token-secret-for-test-123456")
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	_, err = client.GetBrowserCommercialDetail(context.Background(), browserPairingTenantID, runID)
	assert.ErrorIs(t, err, ErrBridgeRequestFailed)
}
