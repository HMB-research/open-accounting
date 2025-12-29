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

	// PDF/Invoice settings
	PDFPrimaryColor string `json:"pdf_primary_color,omitempty"`
	PDFFooterText   string `json:"pdf_footer_text,omitempty"`
	BankDetails     string `json:"bank_details,omitempty"`
	InvoiceTerms    string `json:"invoice_terms,omitempty"`
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
	RoleOwner      = "owner"      // Full access - can manage everything including organization
	RoleAdmin      = "admin"      // Administrative access - can manage users and settings
	RoleAccountant = "accountant" // Accounting access - can manage all accounting functions
	RoleViewer     = "viewer"     // Read-only access - can only view reports
)

// RolePermissions defines what each role can do
type RolePermissions struct {
	CanManageUsers    bool
	CanManageSettings bool
	CanManageAccounts bool
	CanCreateEntries  bool
	CanApproveEntries bool
	CanViewReports    bool
	CanManageInvoices bool
	CanManagePayments bool
	CanManageContacts bool
	CanManageBanking  bool
	CanExportData     bool
}

// GetRolePermissions returns permissions for a given role
func GetRolePermissions(role string) RolePermissions {
	switch role {
	case RoleOwner, RoleAdmin:
		return RolePermissions{
			CanManageUsers:    true,
			CanManageSettings: true,
			CanManageAccounts: true,
			CanCreateEntries:  true,
			CanApproveEntries: true,
			CanViewReports:    true,
			CanManageInvoices: true,
			CanManagePayments: true,
			CanManageContacts: true,
			CanManageBanking:  true,
			CanExportData:     true,
		}
	case RoleAccountant:
		return RolePermissions{
			CanManageUsers:    false,
			CanManageSettings: false,
			CanManageAccounts: true,
			CanCreateEntries:  true,
			CanApproveEntries: true,
			CanViewReports:    true,
			CanManageInvoices: true,
			CanManagePayments: true,
			CanManageContacts: true,
			CanManageBanking:  true,
			CanExportData:     true,
		}
	case RoleViewer:
		return RolePermissions{
			CanManageUsers:    false,
			CanManageSettings: false,
			CanManageAccounts: false,
			CanCreateEntries:  false,
			CanApproveEntries: false,
			CanViewReports:    true,
			CanManageInvoices: false,
			CanManagePayments: false,
			CanManageContacts: false,
			CanManageBanking:  false,
			CanExportData:     false,
		}
	default:
		return RolePermissions{} // No permissions for unknown roles
	}
}

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

// UserInvitation represents a pending invitation to join a tenant
type UserInvitation struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name,omitempty"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	InvitedBy  string     `json:"invited_by"`
	Token      string     `json:"-"` // Never expose token in JSON
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateInvitationRequest is the request to invite a user
type CreateInvitationRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AcceptInvitationRequest is the request to accept an invitation
type AcceptInvitationRequest struct {
	Token    string `json:"token"`
	Password string `json:"password,omitempty"` // Required if user doesn't exist
	Name     string `json:"name,omitempty"`     // Required if user doesn't exist
}

// ValidRoles returns the list of valid roles for invitations
func ValidRoles() []string {
	return []string{RoleAdmin, RoleAccountant, RoleViewer}
}

// IsValidRole checks if a role is valid for invitation
func IsValidRole(role string) bool {
	for _, r := range ValidRoles() {
		if r == role {
			return true
		}
	}
	return false
}
