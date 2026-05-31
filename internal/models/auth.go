package models

import "time"

// RefreshSession stores revocable refresh-token session metadata.
type RefreshSession struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     string     `gorm:"column:user_id;type:uuid;not null;index" json:"user_id"`
	TokenHash  string     `gorm:"column:token_hash;size:64;not null;uniqueIndex" json:"-"`
	CreatedAt  time.Time  `gorm:"not null;default:now()" json:"created_at"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null;index" json:"expires_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for GORM.
func (RefreshSession) TableName() string {
	return "refresh_sessions"
}
