package models

import (
	"encoding/json"
	"time"
)

// Tenant represents a company/organization using the system (GORM model)
type Tenant struct {
	ID                  string          `gorm:"type:uuid;primaryKey" json:"id"`
	Name                string          `gorm:"size:255;not null" json:"name"`
	Slug                string          `gorm:"size:100;not null;uniqueIndex" json:"slug"`
	SchemaName          string          `gorm:"column:schema_name;size:100;not null;uniqueIndex" json:"schema_name"`
	Settings            json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	IsActive            bool            `gorm:"column:is_active;not null;default:true" json:"is_active"`
	OnboardingCompleted bool            `gorm:"column:onboarding_completed;not null;default:false" json:"onboarding_completed"`
	CreatedAt           time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time       `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	TenantUsers []TenantUserModel `gorm:"foreignKey:TenantID" json:"tenant_users,omitempty"`
}

// TableName returns the table name for GORM
func (Tenant) TableName() string {
	return "tenants"
}

// User represents a system user (GORM model)
type User struct {
	ID           string    `gorm:"type:uuid;primaryKey" json:"id"`
	Email        string    `gorm:"size:255;not null;uniqueIndex" json:"email"`
	PasswordHash string    `gorm:"column:password_hash;type:text;not null" json:"-"`
	Name         string    `gorm:"size:255;not null" json:"name"`
	IsActive     bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	TenantUsers []TenantUserModel `gorm:"foreignKey:UserID" json:"tenant_users,omitempty"`
}

// TableName returns the table name for GORM
func (User) TableName() string {
	return "users"
}

// TenantUserModel represents a user's membership in a tenant (GORM model)
// Named TenantUserModel to avoid conflict with domain type
type TenantUserModel struct {
	TenantID  string     `gorm:"column:tenant_id;type:uuid;primaryKey" json:"tenant_id"`
	UserID    string     `gorm:"column:user_id;type:uuid;primaryKey" json:"user_id"`
	Role      string     `gorm:"size:50;not null" json:"role"`
	IsDefault bool       `gorm:"column:is_default;not null;default:false" json:"is_default"`
	IsActive  bool       `gorm:"column:is_active;not null;default:true" json:"is_active"`
	InvitedBy *string    `gorm:"column:invited_by;type:uuid" json:"invited_by,omitempty"`
	InvitedAt *time.Time `gorm:"column:invited_at" json:"invited_at,omitempty"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for GORM
func (TenantUserModel) TableName() string {
	return "tenant_users"
}

// UserInvitation represents a pending invitation to join a tenant (GORM model)
type UserInvitation struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID   string     `gorm:"column:tenant_id;type:uuid;not null;index;uniqueIndex:idx_user_invitations_tenant_email" json:"tenant_id"`
	Email      string     `gorm:"size:255;not null;uniqueIndex:idx_user_invitations_tenant_email" json:"email"`
	Role       string     `gorm:"size:50;not null" json:"role"`
	InvitedBy  string     `gorm:"column:invited_by;type:uuid;not null" json:"invited_by"`
	Token      string     `gorm:"size:255;not null;uniqueIndex" json:"-"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	AcceptedAt *time.Time `gorm:"column:accepted_at" json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName returns the table name for GORM
func (UserInvitation) TableName() string {
	return "user_invitations"
}

// TenantAuditEvent records tenant administration and security-sensitive actions.
type TenantAuditEvent struct {
	ID          string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string          `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	ActorUserID *string         `gorm:"column:actor_user_id;type:uuid" json:"actor_user_id,omitempty"`
	Action      string          `gorm:"size:64;not null;index" json:"action"`
	TargetType  string          `gorm:"size:64;not null" json:"target_type"`
	TargetID    string          `gorm:"type:text;not null" json:"target_id"`
	TargetEmail *string         `gorm:"type:text" json:"target_email,omitempty"`
	Metadata    json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"metadata,omitempty"`
	CreatedAt   time.Time       `gorm:"not null;default:now();index" json:"created_at"`

	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Actor  *User   `gorm:"foreignKey:ActorUserID" json:"actor,omitempty"`
}

// TableName returns the table name for GORM.
func (TenantAuditEvent) TableName() string {
	return "tenant_audit_events"
}
