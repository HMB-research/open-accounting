package smartaccountssync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBridgeClientUsesExactBrowserCaptureWireContract(t *testing.T) {
	const tenantID = "b436c224-5df5-4b4d-a772-1897f9147400"
	const runID = browserCaptureRunID
	const token = "bridge-token-secret-for-test-123456"
	fixedNow := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	resourceID := BrowserGeneralLedgerResourceID
	request := validBrowserCaptureTestRequest()
	body := []byte("date,amount\n2024-01-01,1\n")
	digest := sha256.Sum256(body)
	digestHex := hex.EncodeToString(digest[:])
	packageSHA := strings.Repeat("a", 64)
	finalResponse := `{"run_id":"` + runID + `","status":"finalized_partial","manifest_version":"` + BrowserCaptureManifestVersion + `","scope":{"mode":"partial","from_inclusive":"2024-01-01","to_inclusive":"2024-12-31","cutoff_at":"2026-08-28T10:00:00Z","resource_ids":["` + resourceID + `"]},"resources":[{"resource_id":"` + resourceID + `","coverage":"export_csv","status":"completed","sha256":"` + digestHex + `","byte_count":25}],"receipt":{"status":"partial_coverage_recorded","ready":false,"completed_export_count":1,"required_export_count":1,"blocked_page_only_count":0,"finalized_at":"2026-08-28T10:01:00Z"},"staging":{"package_id":"sa-browser-synthetic","package_sha256":"` + packageSHA + `","status":"staged_review_required","record_chunks_acknowledged":1,"artifact_chunks_acknowledged":1,"finalized":true}}`

	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireBridgeAuthorization(t, r, tenantID, token, fixedNow)
		switch r.Method + " " + r.URL.Path {
		case http.MethodPost + " /v1/browser-captures/" + runID:
			var received BrowserCaptureStartRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
			assert.Equal(t, request, received)
			_, _ = w.Write([]byte(`{"run_id":"` + runID + `","status":"open","manifest_version":"` + BrowserCaptureManifestVersion + `","scope":{"mode":"partial","from_inclusive":"2024-01-01","to_inclusive":"2024-12-31","cutoff_at":"2026-08-28T10:00:00Z","resource_ids":["` + resourceID + `"]},"resources":[{"resource_id":"` + resourceID + `","coverage":"export_csv","status":"pending"}]}`))
		case http.MethodPut + " /v1/browser-captures/" + runID + "/resources/" + resourceID:
			assert.Equal(t, "text/csv", r.Header.Get("Content-Type"))
			assert.Equal(t, digestHex, r.Header.Get("X-SA-Browser-Resource-SHA256"))
			received := new(bytes.Buffer)
			_, err := received.ReadFrom(r.Body)
			require.NoError(t, err)
			assert.Equal(t, body, received.Bytes())
			writeBridgeJSON(w, BrowserCaptureResourceStatus{ResourceID: resourceID, Coverage: "export_csv", Status: "accepted", Created: true})
		case http.MethodPost + " /v1/browser-captures/" + runID + "/finalize":
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, int64(0), r.ContentLength)
			assert.Equal(t, "", readBridgeBody(t, r))
			_, _ = w.Write([]byte(finalResponse))
		case http.MethodGet + " /v1/browser-captures/" + runID:
			_, _ = w.Write([]byte(finalResponse))
		default:
			http.NotFound(w, r)
		}
	}))
	defer bridge.Close()

	client, err := NewHTTPBridgeClient(bridge.URL, token)
	require.NoError(t, err)
	client.httpClient = bridge.Client()
	client.now = func() time.Time { return fixedNow }

	started, err := client.StartBrowserCapture(context.Background(), tenantID, runID, request)
	require.NoError(t, err)
	assert.Equal(t, "open", started.Status)
	sealed, err := client.UploadBrowserCaptureResource(context.Background(), tenantID, runID, resourceID, digestHex, "text/csv", body)
	require.NoError(t, err)
	assert.Equal(t, "accepted", sealed.Status)
	finalized, err := client.FinalizeBrowserCapture(context.Background(), tenantID, runID)
	require.NoError(t, err)
	require.NotNil(t, finalized.Receipt)
	assert.Equal(t, "partial_coverage_recorded", finalized.Receipt.Status)
	require.NotNil(t, finalized.Staging)
	assert.Equal(t, "sa-browser-synthetic", finalized.Staging.PackageID)
	assert.True(t, finalized.Staging.Finalized)
	status, err := client.GetBrowserCapture(context.Background(), tenantID, runID)
	require.NoError(t, err)
	assert.Equal(t, finalized.Staging, status.Staging)
}

func TestBridgeBrowserCaptureStatusRejectsMalformedReceiptAndStaging(t *testing.T) {
	request := validBrowserCaptureTestRequest()
	base := bridgeBrowserCaptureResponse{
		RunID:           browserCaptureRunID,
		Status:          "finalized_partial",
		ManifestVersion: BrowserCaptureManifestVersion,
		Scope:           request.Scope,
		Resources: []bridgeBrowserCaptureResource{{
			ResourceID: BrowserGeneralLedgerResourceID, Coverage: "export_csv", Status: "completed", SHA256: strings.Repeat("b", 64), ByteCount: 1,
		}},
		Receipt: &BrowserCaptureCoverageReceipt{Status: "partial_coverage_recorded", CompletedExportCount: 1, RequiredExportCount: 1, FinalizedAt: "2026-08-28T10:01:00Z"},
		Staging: &BrowserCaptureStaging{PackageID: "sa-browser-synthetic", PackageSHA256: strings.Repeat("a", 64), Status: "staged_review_required", RecordChunksAcknowledged: 1, ArtifactChunksAcknowledged: 1, Finalized: true},
	}
	_, err := base.toBrowserCaptureStatus(browserCaptureRunID)
	require.NoError(t, err)
	compiledPrivate := base
	compiledPrivate.Staging = &BrowserCaptureStaging{PackageID: "sa-browser-synthetic", PackageSHA256: strings.Repeat("a", 64), Status: "compiled_private"}
	_, err = compiledPrivate.toBrowserCaptureStatus(browserCaptureRunID)
	require.NoError(t, err)

	malformedReceipt := base
	malformedReceipt.Receipt = &BrowserCaptureCoverageReceipt{Status: "partial_coverage_recorded", CompletedExportCount: 0, RequiredExportCount: 1, FinalizedAt: "2026-08-28T10:01:00Z"}
	_, err = malformedReceipt.toBrowserCaptureStatus(browserCaptureRunID)
	assert.ErrorIs(t, err, ErrBridgeRequestFailed)

	malformedStaging := base
	malformedStaging.Staging = &BrowserCaptureStaging{Status: "staged_review_required", PackageID: "sa-browser-synthetic", PackageSHA256: strings.Repeat("A", 64), Finalized: true}
	_, err = malformedStaging.toBrowserCaptureStatus(browserCaptureRunID)
	assert.ErrorIs(t, err, ErrBridgeRequestFailed)
}

func readBridgeBody(t *testing.T, request *http.Request) string {
	t.Helper()
	buffer := new(bytes.Buffer)
	_, err := buffer.ReadFrom(request.Body)
	require.NoError(t, err)
	return buffer.String()
}
