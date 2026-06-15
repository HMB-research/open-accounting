package models

import (
	"encoding/json"
	"time"
)

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

// PasswordResetToken stores a one-time account recovery token.
type PasswordResetToken struct {
	ID             string     `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         string     `gorm:"column:user_id;type:uuid;not null;index" json:"user_id"`
	TokenHash      string     `gorm:"column:token_hash;type:char(64);not null;uniqueIndex" json:"-"`
	RequestedEmail string     `gorm:"column:requested_email;size:255;not null;index" json:"requested_email"`
	RequestIP      string     `gorm:"column:request_ip;type:text" json:"request_ip,omitempty"`
	UserAgent      string     `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	CreatedAt      time.Time  `gorm:"not null;default:now();index" json:"created_at"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null;index" json:"expires_at"`
	UsedAt         *time.Time `gorm:"column:used_at;index" json:"used_at,omitempty"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for GORM.
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// SecurityAuditEvent stores auth-sensitive account activity.
type SecurityAuditEvent struct {
	ID           string          `gorm:"type:uuid;primaryKey" json:"id"`
	ActorUserID  *string         `gorm:"column:actor_user_id;type:uuid;index" json:"actor_user_id,omitempty"`
	ActorEmail   *string         `gorm:"column:actor_email;size:255" json:"actor_email,omitempty"`
	Action       string          `gorm:"size:64;not null;index" json:"action"`
	TargetUserID *string         `gorm:"column:target_user_id;type:uuid;index" json:"target_user_id,omitempty"`
	TargetEmail  *string         `gorm:"column:target_email;size:255" json:"target_email,omitempty"`
	RequestIP    *string         `gorm:"column:request_ip;type:text" json:"request_ip,omitempty"`
	UserAgent    *string         `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	Metadata     json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"metadata,omitempty"`
	CreatedAt    time.Time       `gorm:"not null;default:now();index" json:"created_at"`
}

// TableName returns the table name for GORM.
func (SecurityAuditEvent) TableName() string {
	return "security_audit_events"
}
