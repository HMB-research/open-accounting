package models

import (
	"encoding/json"
	"time"
)

// ImportDeliveryRecord is a tenant-schema archive manifest/status. Raw source
// data is separated into chunk tables and has no browser-facing model.
type ImportDeliveryRecord struct {
	ID              string          `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID        string          `gorm:"column:tenant_id;type:uuid;not null;index"`
	PackageID       string          `gorm:"column:package_id;size:255;not null"`
	Provider        string          `gorm:"column:provider;size:64;not null"`
	SourceCompanyID string          `gorm:"column:source_company_id;size:255;not null"`
	ManifestSHA256  string          `gorm:"column:manifest_sha256;size:64;not null"`
	PackageSHA256   string          `gorm:"column:package_sha256;size:64;not null"`
	RecordsSHA256   string          `gorm:"column:records_sha256;size:64;not null"`
	RecordCount     int             `gorm:"column:record_count;not null"`
	ArtifactCount   int             `gorm:"column:artifact_count;not null"`
	Status          string          `gorm:"column:status;size:64;not null"`
	Manifest        json.RawMessage `gorm:"column:manifest;type:jsonb;not null"`
	StagedSessionID string          `gorm:"column:staged_session_id;type:text"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null;default:now()"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;not null;default:now()"`
}

func (ImportDeliveryRecord) TableName() string { return "external_import_deliveries" }

type ImportDeliveryRecordChunk struct {
	DeliveryID  string    `gorm:"column:delivery_id;type:uuid;primaryKey"`
	Sequence    int       `gorm:"column:sequence;primaryKey"`
	RecordCount int       `gorm:"column:record_count;not null"`
	SHA256      string    `gorm:"column:sha256;size:64;not null"`
	Data        []byte    `gorm:"column:data;type:bytea;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;default:now()"`
}

func (ImportDeliveryRecordChunk) TableName() string { return "external_import_record_chunks" }

type ImportDeliveryArtifactChunk struct {
	DeliveryID string    `gorm:"column:delivery_id;type:uuid;primaryKey"`
	ArtifactID string    `gorm:"column:artifact_id;size:255;primaryKey"`
	Sequence   int       `gorm:"column:sequence;primaryKey"`
	ChunkCount int       `gorm:"column:chunk_count;not null"`
	SHA256     string    `gorm:"column:sha256;size:64;not null"`
	Data       []byte    `gorm:"column:data;type:bytea;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;default:now()"`
}

func (ImportDeliveryArtifactChunk) TableName() string { return "external_import_artifact_chunks" }

// ImportDeliveryNonce is public auth replay metadata only. It stores neither
// source material nor HMAC secret values.
type ImportDeliveryNonce struct {
	TenantID  string    `gorm:"column:tenant_id;type:uuid;primaryKey"`
	Nonce     string    `gorm:"column:nonce;size:255;primaryKey"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:now()"`
}

func (ImportDeliveryNonce) TableName() string { return "import_delivery_nonces" }
