package models

import "time"

// Document represents evidence or supporting files attached to tenant records.
type Document struct {
	ID             string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID       string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	EntityType     string     `gorm:"column:entity_type;size:50;not null;index" json:"entity_type"`
	EntityID       string     `gorm:"column:entity_id;type:uuid;not null;index" json:"entity_id"`
	DocumentType   string     `gorm:"column:document_type;size:50;not null" json:"document_type"`
	FileName       string     `gorm:"column:file_name;size:255;not null" json:"file_name"`
	ContentType    string     `gorm:"column:content_type;size:100;not null" json:"content_type"`
	FileSize       int64      `gorm:"column:file_size;not null" json:"file_size"`
	StorageKey     string     `gorm:"column:storage_key;type:text;not null;uniqueIndex" json:"-"`
	Notes          string     `gorm:"type:text" json:"notes,omitempty"`
	RetentionUntil *time.Time `gorm:"column:retention_until" json:"retention_until,omitempty"`
	ReviewStatus   string     `gorm:"column:review_status;size:20;not null;default:'PENDING'" json:"review_status"`
	ReviewNote     *string    `gorm:"column:review_note;type:text" json:"review_note,omitempty"`
	ReviewedBy     *string    `gorm:"column:reviewed_by;type:uuid" json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time `gorm:"column:reviewed_at" json:"reviewed_at,omitempty"`
	UploadedBy     string     `gorm:"column:uploaded_by;type:uuid;not null" json:"uploaded_by"`
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

// TableName returns the table name for GORM.
func (Document) TableName() string {
	return "documents"
}
