package apitoken

import "time"

// APIToken is persisted metadata for a tenant-scoped API token.
type APIToken struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateRequest creates a new tenant-scoped API token.
type CreateRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateResult includes the raw token string, returned only once on creation.
type CreateResult struct {
	Token    string    `json:"token"`
	APIToken *APIToken `json:"api_token"`
}

// ValidationRecord is used internally to map a raw API token to auth claims.
type ValidationRecord struct {
	APIToken
	Email string
	Role  string
}
