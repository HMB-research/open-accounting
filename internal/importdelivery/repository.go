package importdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORMRepository persists only server-to-server archive staging data. Every
// tenant archive table is dynamically qualified through database.TenantTable.
type GORMRepository struct{ db *gorm.DB }

var newGormDBFromPool = database.NewGormDBFromPool

func NewRepository(pool *pgxpool.Pool) *GORMRepository {
	if pool == nil {
		return &GORMRepository{}
	}
	db, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create import delivery repository: %w", err))
	}
	return &GORMRepository{db: db}
}

func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

func (r *GORMRepository) table(ctx context.Context, schemaName, name string) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import delivery database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schemaName, name)
}

func (r *GORMRepository) CreateManifest(ctx context.Context, schemaName, tenantID string, manifest Manifest) (Status, error) {
	table, err := r.table(ctx, schemaName, "external_import_deliveries")
	if err != nil {
		return Status{}, err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return Status{}, fmt.Errorf("encode archive manifest: %w", err)
	}
	record := &models.ImportDeliveryRecord{ID: uuid.NewString(), TenantID: tenantID, PackageID: manifest.PackageID, Provider: manifest.Provider, SourceCompanyID: manifest.SourceCompanyID, ManifestSHA256: manifest.ManifestSHA256, PackageSHA256: manifest.PackageSHA256, RecordsSHA256: manifest.RecordsSHA256, RecordCount: manifest.RecordCount, ArtifactCount: len(manifest.Artifacts), Status: StatusReceiving, Manifest: encoded}
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "package_id"}}, DoNothing: true}).Create(record)
	if result.Error != nil {
		return Status{}, fmt.Errorf("create archive manifest: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var current models.ImportDeliveryRecord
		if err := table.Where("tenant_id = ? AND package_id = ?", tenantID, manifest.PackageID).First(&current).Error; err != nil {
			return Status{}, fmt.Errorf("load idempotent archive manifest: %w", err)
		}
		if current.ManifestSHA256 != manifest.ManifestSHA256 || current.PackageSHA256 != manifest.PackageSHA256 || current.SourceCompanyID != manifest.SourceCompanyID {
			return Status{}, ErrChunkConflict
		}
		return r.statusFromRecord(ctx, schemaName, &current, false)
	}
	return r.statusFromRecord(ctx, schemaName, record, true)
}

func (r *GORMRepository) GetStatus(ctx context.Context, schemaName, tenantID, packageID string) (Status, error) {
	table, err := r.table(ctx, schemaName, "external_import_deliveries")
	if err != nil {
		return Status{}, err
	}
	var record models.ImportDeliveryRecord
	err = table.Where("tenant_id = ? AND package_id = ?", tenantID, packageID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, fmt.Errorf("get archive delivery: %w", err)
	}
	return r.statusFromRecord(ctx, schemaName, &record, false)
}

func (r *GORMRepository) GetManifest(ctx context.Context, schemaName, tenantID, packageID string) (Manifest, error) {
	table, err := r.table(ctx, schemaName, "external_import_deliveries")
	if err != nil {
		return Manifest{}, err
	}
	var record models.ImportDeliveryRecord
	err = table.Select("manifest").Where("tenant_id = ? AND package_id = ?", tenantID, packageID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Manifest{}, ErrNotFound
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("get archive manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(record.Manifest, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode archive manifest: %w", err)
	}
	return manifest, nil
}

// IterateRecords reads the completed tenant archive one bounded NDJSON record
// at a time. It is server-only plumbing for a reviewed normalizer; it never
// exposes archive content through an HTTP response and never buffers a whole
// package in memory.
func (r *GORMRepository) IterateRecords(ctx context.Context, schemaName, tenantID, packageID string, visit func(json.RawMessage) error) error {
	if visit == nil {
		return errors.New("archive record visitor is required")
	}
	delivery, err := r.delivery(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return err
	}
	if delivery.Status != StatusStagedReview {
		return ErrFinalizeIncomplete
	}
	table, err := r.table(ctx, schemaName, "external_import_record_chunks")
	if err != nil {
		return err
	}
	var rows []models.ImportDeliveryRecordChunk
	if err := table.Select("sequence, data").Where("delivery_id = ?", delivery.ID).Order("sequence ASC").Find(&rows).Error; err != nil {
		return fmt.Errorf("list archive record chunks: %w", err)
	}
	for _, row := range rows {
		for _, line := range bytes.Split(bytes.TrimSpace(row.Data), []byte{'\n'}) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			if err := visit(append(json.RawMessage(nil), line...)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *GORMRepository) PutRecordChunk(ctx context.Context, schemaName, tenantID, packageID string, chunk StoredRecordChunk) (ChunkResult, error) {
	return r.putChunk(ctx, schemaName, tenantID, packageID, "", chunk, StoredArtifactChunk{})
}

func (r *GORMRepository) PutArtifactChunk(ctx context.Context, schemaName, tenantID, packageID, artifactID string, chunk StoredArtifactChunk) (ChunkResult, error) {
	return r.putChunk(ctx, schemaName, tenantID, packageID, artifactID, StoredRecordChunk{}, chunk)
}

func (r *GORMRepository) putChunk(ctx context.Context, schemaName, tenantID, packageID, artifactID string, recordChunk StoredRecordChunk, artifactChunk StoredArtifactChunk) (ChunkResult, error) {
	if r == nil || r.db == nil {
		return ChunkResult{}, errors.New("import delivery database is not configured")
	}
	returnResult := ChunkResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deliveries, err := database.TenantTable(tx, schemaName, "external_import_deliveries")
		if err != nil {
			return err
		}
		var delivery models.ImportDeliveryRecord
		err = deliveries.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND package_id = ?", tenantID, packageID).First(&delivery).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if delivery.Status != StatusReceiving {
			return ErrAlreadyFinalized
		}
		if artifactID == "" {
			chunks, err := database.TenantTable(tx, schemaName, "external_import_record_chunks")
			if err != nil {
				return err
			}
			var existing models.ImportDeliveryRecordChunk
			err = chunks.Where("delivery_id = ? AND sequence = ?", delivery.ID, recordChunk.Sequence).First(&existing).Error
			if err == nil {
				if existing.SHA256 != recordChunk.SHA256 || existing.RecordCount != recordChunk.RecordCount {
					return ErrChunkConflict
				}
				returnResult = ChunkResult{Status: "records_accepted", NextRecordSequence: recordChunk.Sequence + 1, Created: false}
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			var expected int64
			if err := chunks.Model(&models.ImportDeliveryRecordChunk{}).Where("delivery_id = ?", delivery.ID).Count(&expected).Error; err != nil {
				return err
			}
			if recordChunk.Sequence != int(expected) {
				return ErrChunkOutOfOrder
			}
			if err := chunks.Create(&models.ImportDeliveryRecordChunk{DeliveryID: delivery.ID, Sequence: recordChunk.Sequence, RecordCount: recordChunk.RecordCount, SHA256: recordChunk.SHA256, Data: append([]byte(nil), recordChunk.Data...)}).Error; err != nil {
				return err
			}
			returnResult = ChunkResult{Status: "records_accepted", NextRecordSequence: recordChunk.Sequence + 1, Created: true}
			return nil
		}
		manifest, err := decodeManifest(delivery.Manifest)
		if err != nil {
			return err
		}
		if !manifestHasArtifact(manifest, artifactID) {
			return ErrChunkInvalid
		}
		chunks, err := database.TenantTable(tx, schemaName, "external_import_artifact_chunks")
		if err != nil {
			return err
		}
		var existing models.ImportDeliveryArtifactChunk
		err = chunks.Where("delivery_id = ? AND artifact_id = ? AND sequence = ?", delivery.ID, artifactID, artifactChunk.Sequence).First(&existing).Error
		if err == nil {
			if existing.SHA256 != artifactChunk.SHA256 || existing.ChunkCount != artifactChunk.ChunkCount {
				return ErrChunkConflict
			}
			returnResult = ChunkResult{Status: "artifact_accepted", Created: false}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var expected int64
		if err := chunks.Model(&models.ImportDeliveryArtifactChunk{}).Where("delivery_id = ? AND artifact_id = ?", delivery.ID, artifactID).Count(&expected).Error; err != nil {
			return err
		}
		if artifactChunk.Sequence != int(expected) {
			return ErrChunkOutOfOrder
		}
		if err := chunks.Create(&models.ImportDeliveryArtifactChunk{DeliveryID: delivery.ID, ArtifactID: artifactID, Sequence: artifactChunk.Sequence, ChunkCount: artifactChunk.ChunkCount, SHA256: artifactChunk.SHA256, Data: append([]byte(nil), artifactChunk.Data...)}).Error; err != nil {
			return err
		}
		returnResult = ChunkResult{Status: "artifact_accepted", Created: true}
		return nil
	})
	if err != nil {
		return ChunkResult{}, err
	}
	return returnResult, nil
}

func (r *GORMRepository) ListRecordChunks(ctx context.Context, schemaName, tenantID, packageID string) ([]StoredRecordChunk, error) {
	delivery, err := r.delivery(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return nil, err
	}
	table, err := r.table(ctx, schemaName, "external_import_record_chunks")
	if err != nil {
		return nil, err
	}
	var rows []models.ImportDeliveryRecordChunk
	if err := table.Where("delivery_id = ?", delivery.ID).Order("sequence ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]StoredRecordChunk, 0, len(rows))
	for _, row := range rows {
		result = append(result, StoredRecordChunk{Sequence: row.Sequence, RecordCount: row.RecordCount, SHA256: row.SHA256, Data: append([]byte(nil), row.Data...)})
	}
	return result, nil
}

func (r *GORMRepository) ListArtifactChunks(ctx context.Context, schemaName, tenantID, packageID, artifactID string) ([]StoredArtifactChunk, error) {
	delivery, err := r.delivery(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return nil, err
	}
	table, err := r.table(ctx, schemaName, "external_import_artifact_chunks")
	if err != nil {
		return nil, err
	}
	var rows []models.ImportDeliveryArtifactChunk
	if err := table.Where("delivery_id = ? AND artifact_id = ?", delivery.ID, artifactID).Order("sequence ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]StoredArtifactChunk, 0, len(rows))
	for _, row := range rows {
		result = append(result, StoredArtifactChunk{Sequence: row.Sequence, ChunkCount: row.ChunkCount, SHA256: row.SHA256, Data: append([]byte(nil), row.Data...)})
	}
	return result, nil
}

func (r *GORMRepository) Finalize(ctx context.Context, schemaName, tenantID, packageID, stagedSessionID string, finalizedAt time.Time) (Status, error) {
	if r == nil || r.db == nil {
		return Status{}, errors.New("import delivery database is not configured")
	}
	var result models.ImportDeliveryRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		table, err := database.TenantTable(tx, schemaName, "external_import_deliveries")
		if err != nil {
			return err
		}
		var record models.ImportDeliveryRecord
		err = table.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND package_id = ?", tenantID, packageID).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if record.Status == StatusStagedReview {
			result = record
			return nil
		}
		if record.Status != StatusReceiving {
			return ErrFinalizeIncomplete
		}
		if err := table.Model(&record).Updates(map[string]interface{}{"status": StatusStagedReview, "staged_session_id": stagedSessionID, "updated_at": finalizedAt}).Error; err != nil {
			return err
		}
		record.Status, record.StagedSessionID, record.UpdatedAt = StatusStagedReview, stagedSessionID, finalizedAt
		result = record
		return nil
	})
	if err != nil {
		return Status{}, err
	}
	return r.statusFromRecord(ctx, schemaName, &result, true)
}

func (r *GORMRepository) delivery(ctx context.Context, schemaName, tenantID, packageID string) (*models.ImportDeliveryRecord, error) {
	table, err := r.table(ctx, schemaName, "external_import_deliveries")
	if err != nil {
		return nil, err
	}
	var record models.ImportDeliveryRecord
	err = table.Where("tenant_id = ? AND package_id = ?", tenantID, packageID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GORMRepository) statusFromRecord(ctx context.Context, schemaName string, record *models.ImportDeliveryRecord, created bool) (Status, error) {
	if record == nil {
		return Status{}, ErrNotFound
	}
	var recordChunks, artifactRows int64
	recordTable, err := r.table(ctx, schemaName, "external_import_record_chunks")
	if err != nil {
		return Status{}, err
	}
	if err := recordTable.Model(&models.ImportDeliveryRecordChunk{}).Where("delivery_id = ?", record.ID).Count(&recordChunks).Error; err != nil {
		return Status{}, err
	}
	artifactTable, err := r.table(ctx, schemaName, "external_import_artifact_chunks")
	if err != nil {
		return Status{}, err
	}
	if err := artifactTable.Model(&models.ImportDeliveryArtifactChunk{}).Distinct("artifact_id").Where("delivery_id = ?", record.ID).Count(&artifactRows).Error; err != nil {
		return Status{}, err
	}
	return Status{PackageID: record.PackageID, TenantID: record.TenantID, SourceCompanyID: record.SourceCompanyID, Status: record.Status, ManifestSHA256: record.ManifestSHA256, PackageSHA256: record.PackageSHA256, RecordCount: record.RecordCount, RecordChunks: int(recordChunks), NextRecordSequence: int(recordChunks), ArtifactCount: record.ArtifactCount, ArtifactsComplete: int(artifactRows), StagedSessionID: record.StagedSessionID, Created: created, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, nil
}

func decodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
func manifestHasArtifact(m Manifest, id string) bool {
	for _, artifact := range m.Artifacts {
		if artifact.ArtifactID == id {
			return true
		}
	}
	return false
}

// ConsumeNonce establishes replay protection in public metadata, separate from
// tenant source archive tables and without retaining the signed request body.
func (r *GORMRepository) ConsumeNonce(ctx context.Context, tenantID, nonce string, expiresAt time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("import delivery database is not configured")
	}
	result := r.db.WithContext(ctx).Table("public.import_delivery_nonces").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "nonce"}}, DoNothing: true}).Create(&models.ImportDeliveryNonce{TenantID: strings.TrimSpace(tenantID), Nonce: strings.TrimSpace(nonce), ExpiresAt: expiresAt})
	if result.Error != nil {
		return fmt.Errorf("consume bridge delivery nonce: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrChunkConflict
	}
	return nil
}
