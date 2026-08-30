package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type manifestOnlyDeliveryStore struct{ manifest importdelivery.Manifest }

func (s *manifestOnlyDeliveryStore) CreateManifest(_ context.Context, _ string, tenantID string, manifest importdelivery.Manifest) (importdelivery.Status, error) {
	s.manifest = manifest
	return importdelivery.Status{PackageID: manifest.PackageID, TenantID: tenantID, SourceCompanyID: manifest.SourceCompanyID, Status: importdelivery.StatusReceiving, ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordCount: manifest.RecordCount, ArtifactCount: len(manifest.Artifacts), Created: true}, nil
}
func (*manifestOnlyDeliveryStore) GetStatus(context.Context, string, string, string) (importdelivery.Status, error) {
	return importdelivery.Status{}, importdelivery.ErrNotFound
}
func (*manifestOnlyDeliveryStore) PutRecordChunk(context.Context, string, string, string, importdelivery.StoredRecordChunk) (importdelivery.ChunkResult, error) {
	return importdelivery.ChunkResult{}, importdelivery.ErrNotFound
}
func (*manifestOnlyDeliveryStore) PutArtifactChunk(context.Context, string, string, string, string, importdelivery.StoredArtifactChunk) (importdelivery.ChunkResult, error) {
	return importdelivery.ChunkResult{}, importdelivery.ErrNotFound
}
func (*manifestOnlyDeliveryStore) ListRecordChunks(context.Context, string, string, string) ([]importdelivery.StoredRecordChunk, error) {
	return nil, nil
}
func (*manifestOnlyDeliveryStore) ListArtifactChunks(context.Context, string, string, string, string) ([]importdelivery.StoredArtifactChunk, error) {
	return nil, nil
}
func (*manifestOnlyDeliveryStore) Finalize(context.Context, string, string, string, string, time.Time) (importdelivery.Status, error) {
	return importdelivery.Status{}, importdelivery.ErrNotFound
}
func (s *manifestOnlyDeliveryStore) GetManifest(context.Context, string, string, string) (importdelivery.Manifest, error) {
	return s.manifest, nil
}

type manifestOnlyBinder struct{}

func (manifestOnlyBinder) EnsureSourceCompanyBinding(context.Context, string, string, string) error {
	return nil
}

type testDeliveryNonceStore struct{ used map[string]bool }

func (s *testDeliveryNonceStore) ConsumeNonce(_ context.Context, tenantID, nonce string, _ time.Time) error {
	if s.used == nil {
		s.used = map[string]bool{}
	}
	key := tenantID + "/" + nonce
	if s.used[key] {
		return importdelivery.ErrNonceReplayed
	}
	s.used[key] = true
	return nil
}

func TestBridgeManifestHandlerUsesExactHMACAndDoesNotNeedBrowserAuth(t *testing.T) {
	const secret = "package-delivery-secret-for-handler-123456"
	store := &manifestOnlyDeliveryStore{}
	service := importdelivery.NewService(store, manifestOnlyBinder{})
	authenticator, err := importdelivery.NewHMACAuthenticator(secret, &testDeliveryNonceStore{})
	require.NoError(t, err)
	fixedNow := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	// The verifier's clock is intentionally exercised via a current timestamp;
	// this handler test verifies the exact signed path/body/header contract.
	_ = fixedNow
	h := &Handlers{importDeliveryService: service, importDeliveryAuthenticator: authenticator}
	manifest := importdelivery.Manifest{SchemaVersion: "v1", PackageID: "package-1", ManifestSHA256: testDigest("manifest"), PackageSHA256: testDigest("package"), RecordsSHA256: testDigest("{\"entity_type\":\"contact\"}\n"), Provider: "smartaccounts", SourceCompanyID: "sa-key-v1-source", SourceIdentity: importdelivery.SourceIdentity{ID: "sa-key-v1-source", ValidationSnapshotSHA256: testDigest("snapshot")}, Authority: importdelivery.Authority{GeneralLedgerAuthority: "smartaccounts", SmartAccountsGLAuthoritative: true}, Scope: importdelivery.Scope{Mode: "full", CutoffAt: time.Now().UTC().Format(time.RFC3339)}, RecordCount: 1}
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	path := "/api/v1/internal/bridge/tenants/tenant-1/packages/package-1/manifest"
	timestamp := time.Now().UTC().Format(time.RFC3339)
	digest := testDigest(string(body))
	nonce := "nonce-handler-1"
	signature := testBridgeSignature(secret, http.MethodPut, path, "tenant-1", timestamp, nonce, digest)
	req := withURLParams(httptest.NewRequest(http.MethodPut, path, strings.NewReader(string(body))), map[string]string{"tenantID": "tenant-1", "packageID": "package-1"})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OA-Bridge-Tenant", "tenant-1")
	req.Header.Set("X-OA-Bridge-Timestamp", timestamp)
	req.Header.Set("X-OA-Bridge-Nonce", nonce)
	req.Header.Set("X-OA-Bridge-Content-SHA256", digest)
	req.Header.Set("X-OA-Bridge-Signature", signature)
	w := httptest.NewRecorder()
	h.PutBridgePackageManifest(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "package-1", store.manifest.PackageID)
	assert.Equal(t, "sa-key-v1-source", store.manifest.SourceCompanyID)
	assert.NotContains(t, w.Body.String(), "api_secret")
}

func testDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func testBridgeSignature(secret, method, path, tenant, timestamp, nonce, digest string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join([]string{"v1", method, path, tenant, timestamp, nonce, digest}, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}
