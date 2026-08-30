package importdelivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrNotFound              = errors.New("bridge package delivery not found")
	ErrTenantIsolation       = errors.New("bridge package tenant isolation failure")
	ErrManifestInvalid       = errors.New("bridge package manifest is invalid")
	ErrChunkInvalid          = errors.New("bridge package chunk is invalid")
	ErrChunkOutOfOrder       = errors.New("bridge package chunk is out of order")
	ErrChunkConflict         = errors.New("bridge package chunk conflicts with an existing chunk")
	ErrFinalizeIncomplete    = errors.New("bridge package delivery is incomplete")
	ErrAlreadyFinalized      = errors.New("bridge package delivery is already finalized")
	ErrSourceBindingConflict = errors.New("bridge package source is bound to another tenant")
	// ErrStagedPackageMismatch deliberately has one opaque failure surface for
	// a missing, receiving, cross-source, or digest-mismatched package. Callers
	// must not infer another tenant's package state from this verification.
	ErrStagedPackageMismatch = errors.New("bridge package is not the expected staged review package")
)

// Store is implemented by archive persistence. It stores source content only
// in tenant-isolated tables. No method can call a financial write service.
type Store interface {
	CreateManifest(ctx context.Context, schemaName, tenantID string, manifest Manifest) (Status, error)
	GetStatus(ctx context.Context, schemaName, tenantID, packageID string) (Status, error)
	PutRecordChunk(ctx context.Context, schemaName, tenantID, packageID string, chunk StoredRecordChunk) (ChunkResult, error)
	PutArtifactChunk(ctx context.Context, schemaName, tenantID, packageID, artifactID string, chunk StoredArtifactChunk) (ChunkResult, error)
	ListRecordChunks(ctx context.Context, schemaName, tenantID, packageID string) ([]StoredRecordChunk, error)
	ListArtifactChunks(ctx context.Context, schemaName, tenantID, packageID, artifactID string) ([]StoredArtifactChunk, error)
	Finalize(ctx context.Context, schemaName, tenantID, packageID, stagedSessionID string, finalizedAt time.Time) (Status, error)
}

// ArchiveReader is the minimal server-only read seam consumed by reviewed
// planners. Its implementation retains records within tenant archive storage;
// it never provides an HTTP/raw-record response surface.
type ArchiveReader interface {
	GetStatus(context.Context, string, string, string) (Status, error)
	GetManifest(context.Context, string, string, string) (Manifest, error)
	IterateRecords(context.Context, string, string, string, func(json.RawMessage) error) error
}

// SourceBinder is shared with the existing import-session receiver. It makes
// one provider/source identity exclusively belong to its selected OA tenant.
type SourceBinder interface {
	EnsureSourceCompanyBinding(ctx context.Context, tenantID, provider, sourceCompanyID string) error
}

type StoredRecordChunk struct {
	Sequence    int
	RecordCount int
	SHA256      string
	Data        []byte
}

type StoredArtifactChunk struct {
	Sequence   int
	ChunkCount int
	SHA256     string
	Data       []byte
}

// Service stages bounded source archive content. It intentionally returns
// review-required rather than a financial application result at finalization.
type Service struct {
	store  Store
	binder SourceBinder
	now    func() time.Time
}

func NewService(store Store, binder SourceBinder) *Service {
	return &Service{store: store, binder: binder, now: time.Now}
}

func (s *Service) AcceptManifest(ctx context.Context, schemaName, tenantID string, manifest Manifest) (Status, error) {
	if s == nil || s.store == nil || s.binder == nil {
		return Status{}, errors.New("bridge package delivery storage is not configured")
	}
	if err := validateManifest(manifest); err != nil {
		return Status{}, err
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(schemaName) == "" {
		return Status{}, ErrTenantIsolation
	}
	if err := s.binder.EnsureSourceCompanyBinding(ctx, strings.TrimSpace(tenantID), manifest.Provider, manifest.SourceCompanyID); err != nil {
		return Status{}, mapSourceBindingError(err)
	}
	return s.store.CreateManifest(ctx, schemaName, strings.TrimSpace(tenantID), manifest)
}

func (s *Service) Status(ctx context.Context, schemaName, tenantID, packageID string) (Status, error) {
	if s == nil || s.store == nil {
		return Status{}, errors.New("bridge package delivery storage is not configured")
	}
	if !safeID(packageID) || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(schemaName) == "" {
		return Status{}, ErrTenantIsolation
	}
	return s.store.GetStatus(ctx, schemaName, strings.TrimSpace(tenantID), strings.TrimSpace(packageID))
}

// GetStatus makes Service an ArchiveReader while preserving Status as the
// concise public receiver API name.
func (s *Service) GetStatus(ctx context.Context, schemaName, tenantID, packageID string) (Status, error) {
	return s.Status(ctx, schemaName, tenantID, packageID)
}

// GetManifest and IterateRecords are server-only archive plumbing for
// review-required planners. Both require a store that explicitly implements
// the read capability; finalization itself still only needs GetManifest.
func (s *Service) GetManifest(ctx context.Context, schemaName, tenantID, packageID string) (Manifest, error) {
	if s == nil {
		return Manifest{}, errors.New("bridge package archive reader is not configured")
	}
	archive, ok := s.store.(ArchiveReader)
	if !ok {
		return Manifest{}, errors.New("bridge package archive reader is not configured")
	}
	return archive.GetManifest(ctx, schemaName, tenantID, packageID)
}

func (s *Service) IterateRecords(ctx context.Context, schemaName, tenantID, packageID string, visit func(json.RawMessage) error) error {
	if s == nil {
		return errors.New("bridge package archive reader is not configured")
	}
	archive, ok := s.store.(ArchiveReader)
	if !ok {
		return errors.New("bridge package archive reader is not configured")
	}
	return archive.IterateRecords(ctx, schemaName, tenantID, packageID, visit)
}

// VerifyStagedPackage proves an already delivered package is the exact
// tenant/source/package/digest tuple that reached STAGED_REVIEW_REQUIRED.
// It is intentionally read-only and exposes no archive records. Browser relay
// finalization can use this narrow seam without trusting extra bridge fields.
func (s *Service) VerifyStagedPackage(ctx context.Context, schemaName, tenantID, sourceCompanyID, packageID, packageSHA256 string) error {
	if s == nil || s.store == nil || strings.TrimSpace(schemaName) == "" || strings.TrimSpace(tenantID) == "" || !safeID(packageID) || !safeID(sourceCompanyID) || !isSHA256(packageSHA256) {
		return ErrStagedPackageMismatch
	}
	status, err := s.Status(ctx, schemaName, tenantID, packageID)
	if err != nil || status.TenantID != strings.TrimSpace(tenantID) || status.SourceCompanyID != strings.TrimSpace(sourceCompanyID) || status.PackageID != strings.TrimSpace(packageID) || status.PackageSHA256 != strings.TrimSpace(packageSHA256) || status.Status != StatusStagedReview {
		return ErrStagedPackageMismatch
	}
	return nil
}

func (s *Service) AcceptRecordChunk(ctx context.Context, schemaName, tenantID, packageID string, input RecordChunk) (ChunkResult, error) {
	if s == nil || s.store == nil {
		return ChunkResult{}, errors.New("bridge package delivery storage is not configured")
	}
	data, err := decodeChunk(input.DataBase64, input.SHA256)
	if err != nil || input.ContentEncoding != "base64-ndjson" {
		return ChunkResult{}, ErrChunkInvalid
	}
	return s.AcceptRawRecordChunk(ctx, schemaName, tenantID, packageID, input.Sequence, input.RecordCount, input.SHA256, data)
}

// AcceptRawRecordChunk is the production transport for one raw NDJSON chunk.
// The JSON/base64 method remains only as an isolated service-test seam; the
// internal HTTP receiver deliberately never inflates base64 bodies.
func (s *Service) AcceptRawRecordChunk(ctx context.Context, schemaName, tenantID, packageID string, sequence, recordCount int, digest string, data []byte) (ChunkResult, error) {
	if s == nil || s.store == nil {
		return ChunkResult{}, errors.New("bridge package delivery storage is not configured")
	}
	if !safeID(packageID) || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(schemaName) == "" {
		return ChunkResult{}, ErrTenantIsolation
	}
	if sequence < 0 || recordCount < 1 || len(data) == 0 || len(data) > maxChunkBytes || !isSHA256(digest) || sha256Hex(data) != digest || countNDJSONRecords(data) != recordCount {
		return ChunkResult{}, ErrChunkInvalid
	}
	return s.store.PutRecordChunk(ctx, schemaName, strings.TrimSpace(tenantID), strings.TrimSpace(packageID), StoredRecordChunk{Sequence: sequence, RecordCount: recordCount, SHA256: digest, Data: append([]byte(nil), data...)})
}

func (s *Service) AcceptArtifactChunk(ctx context.Context, schemaName, tenantID, packageID, artifactID string, input ArtifactChunk) (ChunkResult, error) {
	if s == nil || s.store == nil {
		return ChunkResult{}, errors.New("bridge package delivery storage is not configured")
	}
	data, err := decodeChunk(input.DataBase64, input.SHA256)
	if err != nil || input.ContentEncoding != "base64" {
		return ChunkResult{}, ErrChunkInvalid
	}
	return s.AcceptRawArtifactChunk(ctx, schemaName, tenantID, packageID, artifactID, input.Sequence, input.ChunkCount, input.SHA256, data)
}

func (s *Service) AcceptRawArtifactChunk(ctx context.Context, schemaName, tenantID, packageID, artifactID string, sequence, chunkCount int, digest string, data []byte) (ChunkResult, error) {
	if s == nil || s.store == nil {
		return ChunkResult{}, errors.New("bridge package delivery storage is not configured")
	}
	if !safeID(packageID) || !safeID(artifactID) || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(schemaName) == "" {
		return ChunkResult{}, ErrTenantIsolation
	}
	if sequence < 0 || chunkCount < 1 || sequence >= chunkCount || len(data) == 0 || len(data) > maxChunkBytes || !isSHA256(digest) || sha256Hex(data) != digest {
		return ChunkResult{}, ErrChunkInvalid
	}
	return s.store.PutArtifactChunk(ctx, schemaName, strings.TrimSpace(tenantID), strings.TrimSpace(packageID), strings.TrimSpace(artifactID), StoredArtifactChunk{Sequence: sequence, ChunkCount: chunkCount, SHA256: digest, Data: append([]byte(nil), data...)})
}

// Finalize verifies complete ordered content and advances only to a staged
// review state. It never parses source JSON into OA business records or posts
// a journal, invoice, vendor invoice, or payment.
func (s *Service) Finalize(ctx context.Context, schemaName, tenantID, packageID string, req FinalizeRequest) (Status, error) {
	if s == nil || s.store == nil {
		return Status{}, errors.New("bridge package delivery storage is not configured")
	}
	status, err := s.Status(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return Status{}, err
	}
	if status.Status == StatusStagedReview {
		if status.ManifestSHA256 == req.ManifestSHA256 && status.PackageSHA256 == req.PackageSHA256 {
			status.Created = false
			return status, nil
		}
		return Status{}, ErrAlreadyFinalized
	}
	if !isSHA256(req.ManifestSHA256) || !isSHA256(req.PackageSHA256) || !isSHA256(req.RecordsSHA256) || req.RecordCount < 0 || req.ArtifactCount < 0 ||
		status.ManifestSHA256 != req.ManifestSHA256 || status.PackageSHA256 != req.PackageSHA256 || status.RecordCount != req.RecordCount || status.ArtifactCount != req.ArtifactCount {
		return Status{}, ErrFinalizeIncomplete
	}
	records, err := s.store.ListRecordChunks(ctx, schemaName, tenantID, packageID)
	if err != nil || !orderedRecordChunksComplete(records, req.RecordCount, req.RecordsSHA256) {
		return Status{}, ErrFinalizeIncomplete
	}
	manifest, err := s.manifest(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return Status{}, err
	}
	for _, artifact := range manifest.Artifacts {
		chunks, err := s.store.ListArtifactChunks(ctx, schemaName, tenantID, packageID, artifact.ArtifactID)
		if err != nil || !orderedArtifactChunksComplete(chunks, artifact) {
			return Status{}, ErrFinalizeIncomplete
		}
	}
	stagedID := "archive-" + status.PackageID
	return s.store.Finalize(ctx, schemaName, tenantID, packageID, stagedID, s.currentTime())
}

// manifest is intentionally a narrow optional store capability. It avoids
// returning raw archive data while finalization needs declared artifact hashes.
type manifestStore interface {
	GetManifest(ctx context.Context, schemaName, tenantID, packageID string) (Manifest, error)
}

func (s *Service) manifest(ctx context.Context, schemaName, tenantID, packageID string) (Manifest, error) {
	store, ok := s.store.(manifestStore)
	if !ok {
		return Manifest{}, errors.New("bridge package manifest store is not configured")
	}
	return store.GetManifest(ctx, schemaName, tenantID, packageID)
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func validateManifest(m Manifest) error {
	if m.SchemaVersion != SchemaVersionV1 || m.Provider != ProviderSmartAccounts || !safeID(m.PackageID) || !safeID(m.SourceCompanyID) || !safeID(m.SourceIdentity.ID) ||
		!isSHA256(m.ManifestSHA256) || !isSHA256(m.PackageSHA256) || !isSHA256(m.RecordsSHA256) || !isSHA256(m.SourceIdentity.ValidationSnapshotSHA256) ||
		m.Authority.GeneralLedgerAuthority != ProviderSmartAccounts || !m.Authority.SmartAccountsGLAuthoritative || m.RecordCount < 0 || strings.TrimSpace(m.Scope.Mode) == "" || strings.TrimSpace(m.Scope.CutoffAt) == "" {
		return ErrManifestInvalid
	}
	if _, err := time.Parse(time.RFC3339, m.Scope.CutoffAt); err != nil {
		return ErrManifestInvalid
	}
	seen := map[string]struct{}{}
	for _, artifact := range m.Artifacts {
		if !safeID(artifact.ArtifactID) || !isSHA256(artifact.SHA256) || artifact.ByteCount < 0 || strings.TrimSpace(artifact.MediaType) == "" || strings.TrimSpace(artifact.ContentEncoding) == "" {
			return ErrManifestInvalid
		}
		if _, duplicate := seen[artifact.ArtifactID]; duplicate {
			return ErrManifestInvalid
		}
		seen[artifact.ArtifactID] = struct{}{}
	}
	return nil
}

func decodeChunk(encoded, expectedSHA string) ([]byte, error) {
	if !isSHA256(expectedSHA) || len(encoded) == 0 || len(encoded) > base64.StdEncoding.EncodedLen(maxChunkBytes)+4 {
		return nil, ErrChunkInvalid
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > maxChunkBytes || sha256Hex(data) != expectedSHA {
		return nil, ErrChunkInvalid
	}
	return data, nil
}

func countNDJSONRecords(data []byte) int {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return 0
	}
	return len(bytes.Split(trimmed, []byte{'\n'}))
}

func orderedRecordChunksComplete(chunks []StoredRecordChunk, expectedCount int, expectedHash string) bool {
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Sequence < chunks[j].Sequence })
	var data []byte
	count := 0
	for index, chunk := range chunks {
		if chunk.Sequence != index || sha256Hex(chunk.Data) != chunk.SHA256 {
			return false
		}
		data = append(data, chunk.Data...)
		count += chunk.RecordCount
	}
	return count == expectedCount && sha256Hex(data) == expectedHash
}

func orderedArtifactChunksComplete(chunks []StoredArtifactChunk, artifact ArtifactManifest) bool {
	if len(chunks) == 0 && artifact.ByteCount == 0 {
		return true
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Sequence < chunks[j].Sequence })
	var data []byte
	for index, chunk := range chunks {
		if chunk.ChunkCount != len(chunks) || chunk.Sequence != index || sha256Hex(chunk.Data) != chunk.SHA256 {
			return false
		}
		data = append(data, chunk.Data...)
	}
	return int64(len(data)) == artifact.ByteCount && sha256Hex(data) == artifact.SHA256
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
func safeID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > 255 {
		return false
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func mapSourceBindingError(err error) error {
	if errors.Is(err, ErrSourceBindingConflict) {
		return err
	}
	return fmt.Errorf("ensure bridge source binding: %w", err)
}
