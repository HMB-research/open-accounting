package models

import "time"

// APIToken represents a tenant-scoped API token (GORM model).
type APIToken struct {
	ID          string     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID    string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	UserID      string     `gorm:"column:user_id;type:uuid;not null;index" json:"user_id"`
	Name        string     `gorm:"size:255;not null" json:"name"`
	TokenHash   string     `gorm:"column:token_hash;size:64;not null;uniqueIndex" json:"-"`
	TokenPrefix string     `gorm:"column:token_prefix;size:24;not null" json:"token_prefix"`
	LastUsedAt  *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	RevokedAt   *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

// TableName returns the table name for GORM.
func (APIToken) TableName() string {
	return "api_tokens"
}
