package tenant

import (
	"encoding/json"
	"time"
)

// Tenant represents a company/organization using the system
type Tenant struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Slug       string         `json:"slug"`
	SchemaName string         `json:"schema_name"`
	Settings   TenantSettings `json:"settings"`
	IsActive   bool           `json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// TenantSettings contains configurable settings for a tenant
type TenantSettings struct {
	DefaultCurrency string `json:"default_currency"`
	CountryCode     string `json:"country_code"`
	Timezone        string `json:"timezone"`
	DateFormat      string `json:"date_format"`
	DecimalSep      string `json:"decimal_sep"`
	ThousandsSep    string `json:"thousands_sep"`
	FiscalYearStart int    `json:"fiscal_year_start_month"` // 1-12
	VATNumber       string `json:"vat_number,omitempty"`
	RegCode         string `json:"reg_code,omitempty"`
	Address         string `json:"address,omitempty"`
	Email           string `json:"email,omitempty"`
	Phone           string `json:"phone,omitempty"`
	Logo            string `json:"logo,omitempty"`
}

// DefaultSettings returns default tenant settings for Estonia
func DefaultSettings() TenantSettings {
	return TenantSettings{
		DefaultCurrency: "EUR",
		CountryCode:     "EE",
		Timezone:        "Europe/Tallinn",
		DateFormat:      "DD.MM.YYYY",
		DecimalSep:      ",",
		ThousandsSep:    " ",
		FiscalYearStart: 1, // January
	}
}

// Scan implements the sql.Scanner interface for TenantSettings
func (s *TenantSettings) Scan(src interface{}) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	}
	*s = DefaultSettings()
	return nil
}

// User represents a system user
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TenantUser represents a user's membership in a tenant
type TenantUser struct {
	TenantID  string    `json:"tenant_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

// Role constants
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleUser   = "user"
	RoleViewer = "viewer"
)

// CreateTenantRequest is the request to create a new tenant
type CreateTenantRequest struct {
	Name     string          `json:"name"`
	Slug     string          `json:"slug"`
	Settings *TenantSettings `json:"settings,omitempty"`
	OwnerID  string          `json:"-"`
}

// CreateUserRequest is the request to create a new user
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// UserWithTenants represents a user with their tenant memberships
type UserWithTenants struct {
	User
	Tenants []TenantMembership `json:"tenants"`
}

// TenantMembership represents a user's membership in a tenant
type TenantMembership struct {
	Tenant    Tenant `json:"tenant"`
	Role      string `json:"role"`
	IsDefault bool   `json:"is_default"`
}
