package importdelivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	manifests map[string]Manifest
	statuses  map[string]Status
	records   map[string][]StoredRecordChunk
	artifacts map[string]map[string][]StoredArtifactChunk
}

func key(tenantID, packageID string) string { return tenantID + "\x00" + packageID }

func (s *memoryStore) CreateManifest(_ context.Context, _ string, tenantID string, manifest Manifest) (Status, error) {
	if s.manifests == nil {
		s.manifests, s.statuses, s.records, s.artifacts = map[string]Manifest{}, map[string]Status{}, map[string][]StoredRecordChunk{}, map[string]map[string][]StoredArtifactChunk{}
	}
	k := key(tenantID, manifest.PackageID)
	if existing, ok := s.manifests[k]; ok {
		if existing.ManifestSHA256 != manifest.ManifestSHA256 {
			return Status{}, ErrChunkConflict
		}
		status := s.statuses[k]
		status.Created = false
		return status, nil
	}
	s.manifests[k] = manifest
	status := Status{PackageID: manifest.PackageID, TenantID: tenantID, SourceCompanyID: manifest.SourceCompanyID, Status: StatusReceiving, ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordCount: manifest.RecordCount, ArtifactCount: len(manifest.Artifacts), Created: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.statuses[k] = status
	return status, nil
}
func (s *memoryStore) GetStatus(_ context.Context, _ string, tenantID, packageID string) (Status, error) {
	status, ok := s.statuses[key(tenantID, packageID)]
	if !ok {
		return Status{}, ErrNotFound
	}
	status.RecordChunks = len(s.records[key(tenantID, packageID)])
	status.NextRecordSequence = status.RecordChunks
	status.ArtifactsComplete = len(s.artifacts[key(tenantID, packageID)])
	return status, nil
}
func (s *memoryStore) GetManifest(_ context.Context, _ string, tenantID, packageID string) (Manifest, error) {
	manifest, ok := s.manifests[key(tenantID, packageID)]
	if !ok {
		return Manifest{}, ErrNotFound
	}
	return manifest, nil
}
func (s *memoryStore) IterateRecords(_ context.Context, _ string, tenantID, packageID string, visit func(json.RawMessage) error) error {
	if visit == nil {
		return errors.New("archive record visitor is required")
	}
	if _, ok := s.statuses[key(tenantID, packageID)]; !ok {
		return ErrNotFound
	}
	for _, chunk := range s.records[key(tenantID, packageID)] {
		for _, line := range bytes.Split(bytes.TrimSpace(chunk.Data), []byte{'\n'}) {
			if line = bytes.TrimSpace(line); len(line) == 0 {
				continue
			}
			if err := visit(append(json.RawMessage(nil), line...)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *memoryStore) PutRecordChunk(_ context.Context, _ string, tenantID, packageID string, chunk StoredRecordChunk) (ChunkResult, error) {
	k := key(tenantID, packageID)
	if _, ok := s.statuses[k]; !ok {
		return ChunkResult{}, ErrNotFound
	}
	chunks := s.records[k]
	if s.statuses[k].Status != StatusReceiving {
		return ChunkResult{}, ErrAlreadyFinalized
	}
	if chunk.Sequence < len(chunks) {
		existing := chunks[chunk.Sequence]
		if existing.SHA256 == chunk.SHA256 && existing.RecordCount == chunk.RecordCount {
			return ChunkResult{Status: "records_accepted", NextRecordSequence: chunk.Sequence + 1}, nil
		}
		return ChunkResult{}, ErrChunkConflict
	}
	if chunk.Sequence != len(chunks) {
		return ChunkResult{}, ErrChunkOutOfOrder
	}
	s.records[k] = append(chunks, chunk)
	return ChunkResult{Status: "records_accepted", NextRecordSequence: chunk.Sequence + 1, Created: true}, nil
}
func (s *memoryStore) PutArtifactChunk(_ context.Context, _ string, tenantID, packageID, artifactID string, chunk StoredArtifactChunk) (ChunkResult, error) {
	k := key(tenantID, packageID)
	manifest, ok := s.manifests[k]
	if !ok {
		return ChunkResult{}, ErrNotFound
	}
	if !manifestHasArtifact(manifest, artifactID) {
		return ChunkResult{}, ErrChunkInvalid
	}
	if s.statuses[k].Status != StatusReceiving {
		return ChunkResult{}, ErrAlreadyFinalized
	}
	if s.artifacts[k] == nil {
		s.artifacts[k] = map[string][]StoredArtifactChunk{}
	}
	chunks := s.artifacts[k][artifactID]
	if chunk.Sequence < len(chunks) {
		existing := chunks[chunk.Sequence]
		if existing.SHA256 == chunk.SHA256 && existing.ChunkCount == chunk.ChunkCount {
			return ChunkResult{Status: "artifact_accepted"}, nil
		}
		return ChunkResult{}, ErrChunkConflict
	}
	if chunk.Sequence != len(chunks) {
		return ChunkResult{}, ErrChunkOutOfOrder
	}
	s.artifacts[k][artifactID] = append(chunks, chunk)
	return ChunkResult{Status: "artifact_accepted", Created: true}, nil
}
func (s *memoryStore) ListRecordChunks(_ context.Context, _ string, tenantID, packageID string) ([]StoredRecordChunk, error) {
	chunks, ok := s.records[key(tenantID, packageID)]
	if !ok {
		return nil, nil
	}
	return append([]StoredRecordChunk(nil), chunks...), nil
}
func (s *memoryStore) ListArtifactChunks(_ context.Context, _ string, tenantID, packageID, artifactID string) ([]StoredArtifactChunk, error) {
	chunks := s.artifacts[key(tenantID, packageID)][artifactID]
	return append([]StoredArtifactChunk(nil), chunks...), nil
}
func (s *memoryStore) Finalize(_ context.Context, _ string, tenantID, packageID, stagedID string, at time.Time) (Status, error) {
	k := key(tenantID, packageID)
	status, ok := s.statuses[k]
	if !ok {
		return Status{}, ErrNotFound
	}
	status.Status, status.StagedSessionID, status.UpdatedAt, status.Created = StatusStagedReview, stagedID, at, true
	s.statuses[k] = status
	return s.GetStatus(context.Background(), "", tenantID, packageID)
}

type memoryBinder struct{ owners map[string]string }

func (b *memoryBinder) EnsureSourceCompanyBinding(_ context.Context, tenantID, provider, sourceID string) error {
	k := provider + "\x00" + sourceID
	if b.owners == nil {
		b.owners = map[string]string{}
	}
	if existing, ok := b.owners[k]; ok && existing != tenantID {
		return ErrSourceBindingConflict
	}
	b.owners[k] = tenantID
	return nil
}

func TestDeliveryStagesCompleteTenantArchiveWithoutFinancialWrites(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store, &memoryBinder{})
	service.now = func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }
	recordBytes := []byte(`{"entity_type":"contact"}` + "\n")
	artifactBytes := []byte("PDF evidence")
	manifest := testManifest(recordBytes, artifactBytes)

	status, err := service.AcceptManifest(context.Background(), "tenant_a", "tenant-a", manifest)
	require.NoError(t, err)
	assert.Equal(t, StatusReceiving, status.Status)
	assert.True(t, status.Created)
	record := encodedRecordChunk(0, 1, recordBytes)
	result, err := service.AcceptRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, record)
	require.NoError(t, err)
	assert.True(t, result.Created)
	assert.Equal(t, 1, result.NextRecordSequence)
	artifact := encodedArtifactChunk(0, 1, artifactBytes)
	_, err = service.AcceptArtifactChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, "invoice-pdf-1", artifact)
	require.NoError(t, err)

	status, err = service.Finalize(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, FinalizeRequest{ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordsSHA256: manifest.RecordsSHA256, RecordCount: 1, ArtifactCount: 1})
	require.NoError(t, err)
	assert.Equal(t, StatusStagedReview, status.Status)
	assert.Equal(t, "archive-"+manifest.PackageID, status.StagedSessionID)
	assert.Equal(t, StatusStagedReview, store.statuses[key("tenant-a", manifest.PackageID)].Status) // No financial writer exists in this service.
	assert.Len(t, store.records[key("tenant-a", manifest.PackageID)], 1)
}

// This is the bridge receiver's raw-wire contract: records are ordered raw
// NDJSON chunks (not base64 JSON), artifacts use raw bytes, and a declared
// zero-byte artifact is complete without a synthetic empty chunk.
func TestRawBridgeWireContractStagesZeroBasedChunksAndZeroByteArtifacts(t *testing.T) {
	store, binder := &memoryStore{}, &memoryBinder{}
	service := NewService(store, binder)
	first := []byte(`{"entity_type":"account","external_id":"a-1"}` + "\n")
	second := []byte(`{"entity_type":"general_ledger_journal","external_id":"j-1"}` + "\n")
	records := append(append([]byte(nil), first...), second...)
	artifact := []byte("safe attachment bytes")
	manifest := testManifest(records, artifact)
	manifest.RecordCount = 2
	manifest.Artifacts = append(manifest.Artifacts, ArtifactManifest{ArtifactID: "empty-proof", SHA256: sha256Hex(nil), ByteCount: 0, MediaType: "application/pdf", ContentEncoding: "identity"})

	_, err := service.AcceptManifest(context.Background(), "tenant_a", "tenant-a", manifest)
	require.NoError(t, err)
	// Raw headers carry SHA-256 of exactly these bytes. The first sequence is 0.
	result, err := service.AcceptRawRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, 0, 1, sha256Hex(first), first)
	require.NoError(t, err)
	assert.Equal(t, 1, result.NextRecordSequence)
	_, err = service.AcceptRawRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, 1, 1, sha256Hex(second), second)
	require.NoError(t, err)
	_, err = service.AcceptRawArtifactChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, "invoice-pdf-1", 0, 1, sha256Hex(artifact), artifact)
	require.NoError(t, err)

	status, err := service.Finalize(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, FinalizeRequest{ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordsSHA256: sha256Hex(records), RecordCount: 2, ArtifactCount: 2})
	require.NoError(t, err)
	assert.Equal(t, StatusStagedReview, status.Status)
	storedRecords, err := store.ListRecordChunks(context.Background(), "tenant_a", "tenant-a", manifest.PackageID)
	require.NoError(t, err)
	assert.Len(t, storedRecords, 2)
	assert.Empty(t, store.artifacts[key("tenant-a", manifest.PackageID)]["empty-proof"], "zero-byte artifact must not need a fake raw chunk")
	// Same source cannot be accepted by another tenant, even with a new package.
	other := manifest
	other.PackageID = "package-2"
	other.ManifestSHA256 = sha256Hex([]byte("other"))
	_, err = service.AcceptManifest(context.Background(), "tenant_b", "tenant-b", other)
	assert.ErrorIs(t, err, ErrSourceBindingConflict)
}

func TestDeliveryRejectsReplayConflictIncompleteFinalizeAndCrossTenantSource(t *testing.T) {
	store, binder := &memoryStore{}, &memoryBinder{}
	service := NewService(store, binder)
	recordBytes, artifactBytes := []byte(`{"entity_type":"contact"}`+"\n"), []byte("PDF evidence")
	manifest := testManifest(recordBytes, artifactBytes)
	_, err := service.AcceptManifest(context.Background(), "tenant_a", "tenant-a", manifest)
	require.NoError(t, err)
	_, err = service.Finalize(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, FinalizeRequest{ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordsSHA256: manifest.RecordsSHA256, RecordCount: 1, ArtifactCount: 1})
	assert.ErrorIs(t, err, ErrFinalizeIncomplete)
	_, err = service.AcceptRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, encodedRecordChunk(1, 1, recordBytes))
	assert.ErrorIs(t, err, ErrChunkOutOfOrder)
	_, err = service.AcceptRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, encodedRecordChunk(0, 1, recordBytes))
	require.NoError(t, err)
	conflict := encodedRecordChunk(0, 1, []byte(`{"entity_type":"employee"}`+"\n"))
	_, err = service.AcceptRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, conflict)
	assert.ErrorIs(t, err, ErrChunkConflict)
	other := manifest
	other.PackageID = "package-other"
	other.ManifestSHA256 = sha256Hex([]byte("other-manifest"))
	_, err = service.AcceptManifest(context.Background(), "tenant_b", "tenant-b", other)
	assert.ErrorIs(t, err, ErrSourceBindingConflict)
}

func TestVerifyStagedPackageRequiresExactTenantSourceAndDigest(t *testing.T) {
	store, binder := &memoryStore{}, &memoryBinder{}
	service := NewService(store, binder)
	records, artifact := []byte(`{"entity_type":"customer"}`+"\n"), []byte("evidence")
	manifest := testManifest(records, artifact)
	_, err := service.AcceptManifest(context.Background(), "tenant_a", "tenant-a", manifest)
	require.NoError(t, err)
	_, err = service.AcceptRawRecordChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, 0, 1, sha256Hex(records), records)
	require.NoError(t, err)
	_, err = service.AcceptRawArtifactChunk(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, "invoice-pdf-1", 0, 1, sha256Hex(artifact), artifact)
	require.NoError(t, err)
	_, err = service.Finalize(context.Background(), "tenant_a", "tenant-a", manifest.PackageID, FinalizeRequest{ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordsSHA256: manifest.RecordsSHA256, RecordCount: 1, ArtifactCount: 1})
	require.NoError(t, err)

	require.NoError(t, service.VerifyStagedPackage(context.Background(), "tenant_a", "tenant-a", manifest.SourceCompanyID, manifest.PackageID, manifest.PackageSHA256))
	assert.ErrorIs(t, service.VerifyStagedPackage(context.Background(), "tenant_a", "tenant-b", manifest.SourceCompanyID, manifest.PackageID, manifest.PackageSHA256), ErrStagedPackageMismatch)
	assert.ErrorIs(t, service.VerifyStagedPackage(context.Background(), "tenant_a", "tenant-a", "other-source", manifest.PackageID, manifest.PackageSHA256), ErrStagedPackageMismatch)
	assert.ErrorIs(t, service.VerifyStagedPackage(context.Background(), "tenant_a", "tenant-a", manifest.SourceCompanyID, manifest.PackageID, sha256Hex([]byte("other-package"))), ErrStagedPackageMismatch)
}

func TestHMACAuthenticatorChecksDigestAndRejectsReplay(t *testing.T) {
	nonces := &memoryNonceStore{}
	authenticator, err := NewHMACAuthenticator("package-delivery-secret-for-test-123456", nonces)
	require.NoError(t, err)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	authenticator.now = func() time.Time { return now }
	req := signedRequest("package-delivery-secret-for-test-123456", "PUT", "/api/v1/internal/bridge/tenants/tenant-a/packages/package-1/manifest", "tenant-a", now, "nonce-1", []byte(`{}`))
	require.NoError(t, authenticator.Authenticate(context.Background(), req))
	assert.ErrorIs(t, authenticator.Authenticate(context.Background(), req), ErrNonceReplayed)
	req.Nonce = "nonce-2"
	req.ContentSHA256 = sha256Hex([]byte("tampered"))
	assert.ErrorIs(t, authenticator.Authenticate(context.Background(), req), ErrAuthenticationFailed)
}

func TestControlledSourceBinderRequiresExistingTenantConnection(t *testing.T) {
	registry := &memoryBinder{}
	binder := NewControlledSourceBinder(fakeControlStore{}, registry)
	require.NoError(t, binder.EnsureSourceCompanyBinding(context.Background(), "tenant-a", ProviderSmartAccounts, "sa-key-v1-source"))
	assert.Equal(t, "tenant-a", registry.owners[ProviderSmartAccounts+"\x00sa-key-v1-source"])

	err := NewControlledSourceBinder(fakeControlStore{}, registry).EnsureSourceCompanyBinding(context.Background(), "tenant-a", ProviderSmartAccounts, "sa-key-v1-other")
	assert.ErrorIs(t, err, ErrSourceNotConfiguredForTenant)
}

type fakeControlStore struct{}

func (fakeControlStore) Get(_ context.Context, tenantID, sourceID string) (*smartaccountssync.Control, error) {
	if tenantID == "tenant-a" && sourceID == "sa-key-v1-source" {
		return &smartaccountssync.Control{TenantID: tenantID, SourceCompanyID: sourceID, SecretReference: "secret-ref://sa-bridge/connection-a"}, nil
	}
	return nil, smartaccountssync.ErrControlNotConfigured
}

type memoryNonceStore struct{ used map[string]struct{} }

func (s *memoryNonceStore) ConsumeNonce(_ context.Context, tenantID, nonce string, _ time.Time) error {
	if s.used == nil {
		s.used = map[string]struct{}{}
	}
	k := tenantID + "\x00" + nonce
	if _, ok := s.used[k]; ok {
		return ErrNonceReplayed
	}
	s.used[k] = struct{}{}
	return nil
}

func testManifest(recordBytes, artifactBytes []byte) Manifest {
	return Manifest{SchemaVersion: SchemaVersionV1, PackageID: "package-1", ManifestSHA256: sha256Hex([]byte("manifest")), PackageSHA256: sha256Hex([]byte("package")), RecordsSHA256: sha256Hex(recordBytes), Provider: ProviderSmartAccounts, SourceCompanyID: "sa-key-v1-source", SourceIdentity: SourceIdentity{ID: "sa-key-v1-source", ValidationSnapshotSHA256: sha256Hex([]byte("snapshot"))}, Authority: Authority{GeneralLedgerAuthority: ProviderSmartAccounts, SmartAccountsGLAuthoritative: true}, Scope: Scope{Mode: "full", CutoffAt: "2026-08-27T12:00:00Z"}, RecordCount: 1, Artifacts: []ArtifactManifest{{ArtifactID: "invoice-pdf-1", SHA256: sha256Hex(artifactBytes), ByteCount: int64(len(artifactBytes)), MediaType: "application/pdf", ContentEncoding: "base64"}}}
}
func encodedRecordChunk(sequence, count int, data []byte) RecordChunk {
	return RecordChunk{Sequence: sequence, RecordCount: count, SHA256: sha256Hex(data), ContentEncoding: "base64-ndjson", DataBase64: base64.StdEncoding.EncodeToString(data)}
}
func encodedArtifactChunk(sequence, count int, data []byte) ArtifactChunk {
	return ArtifactChunk{Sequence: sequence, ChunkCount: count, SHA256: sha256Hex(data), ContentEncoding: "base64", DataBase64: base64.StdEncoding.EncodeToString(data)}
}
func signedRequest(secret, method, path, tenantID string, now time.Time, nonce string, body []byte) SignedRequest {
	timestamp := now.Format(time.RFC3339)
	digest := sha256Hex(body)
	parts := []string{"v1", method, path, tenantID, timestamp, nonce, digest}
	signature := hmacSignature(secret, parts)
	return SignedRequest{Method: method, Path: path, TenantID: tenantID, Timestamp: timestamp, Nonce: nonce, ContentSHA256: digest, Signature: signature, Body: body}
}
func hmacSignature(secret string, parts []string) string { // Use the production verifier's canonical content without exposing any key values.
	message := ""
	for index, part := range parts {
		if index > 0 {
			message += "\n"
		}
		message += part
	}
	return hmacHex([]byte(secret), []byte(message))
}
func hmacHex(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

var _ = sort.Slice
