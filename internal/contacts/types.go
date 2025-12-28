package contacts

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContactType represents the type of contact
type ContactType string

const (
	ContactTypeCustomer ContactType = "CUSTOMER"
	ContactTypeSupplier ContactType = "SUPPLIER"
	ContactTypeBoth     ContactType = "BOTH"
)

// Contact represents a customer or supplier
type Contact struct {
	ID               string          `json:"id"`
	TenantID         string          `json:"tenant_id"`
	Code             string          `json:"code,omitempty"`
	Name             string          `json:"name"`
	ContactType      ContactType     `json:"contact_type"`
	RegCode          string          `json:"reg_code,omitempty"`
	VATNumber        string          `json:"vat_number,omitempty"`
	Email            string          `json:"email,omitempty"`
	Phone            string          `json:"phone,omitempty"`
	AddressLine1     string          `json:"address_line1,omitempty"`
	AddressLine2     string          `json:"address_line2,omitempty"`
	City             string          `json:"city,omitempty"`
	PostalCode       string          `json:"postal_code,omitempty"`
	CountryCode      string          `json:"country_code"`
	PaymentTermsDays int             `json:"payment_terms_days"`
	CreditLimit      decimal.Decimal `json:"credit_limit,omitempty"`
	DefaultAccountID *string         `json:"default_account_id,omitempty"`
	IsActive         bool            `json:"is_active"`
	Notes            string          `json:"notes,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// CreateContactRequest is the request to create a contact
type CreateContactRequest struct {
	Code             string          `json:"code,omitempty"`
	Name             string          `json:"name"`
	ContactType      ContactType     `json:"contact_type"`
	RegCode          string          `json:"reg_code,omitempty"`
	VATNumber        string          `json:"vat_number,omitempty"`
	Email            string          `json:"email,omitempty"`
	Phone            string          `json:"phone,omitempty"`
	AddressLine1     string          `json:"address_line1,omitempty"`
	AddressLine2     string          `json:"address_line2,omitempty"`
	City             string          `json:"city,omitempty"`
	PostalCode       string          `json:"postal_code,omitempty"`
	CountryCode      string          `json:"country_code,omitempty"`
	PaymentTermsDays int             `json:"payment_terms_days,omitempty"`
	CreditLimit      decimal.Decimal `json:"credit_limit,omitempty"`
	DefaultAccountID *string         `json:"default_account_id,omitempty"`
	Notes            string          `json:"notes,omitempty"`
}

// UpdateContactRequest is the request to update a contact
type UpdateContactRequest struct {
	Name             *string          `json:"name,omitempty"`
	RegCode          *string          `json:"reg_code,omitempty"`
	VATNumber        *string          `json:"vat_number,omitempty"`
	Email            *string          `json:"email,omitempty"`
	Phone            *string          `json:"phone,omitempty"`
	AddressLine1     *string          `json:"address_line1,omitempty"`
	AddressLine2     *string          `json:"address_line2,omitempty"`
	City             *string          `json:"city,omitempty"`
	PostalCode       *string          `json:"postal_code,omitempty"`
	CountryCode      *string          `json:"country_code,omitempty"`
	PaymentTermsDays *int             `json:"payment_terms_days,omitempty"`
	CreditLimit      *decimal.Decimal `json:"credit_limit,omitempty"`
	DefaultAccountID *string          `json:"default_account_id,omitempty"`
	Notes            *string          `json:"notes,omitempty"`
	IsActive         *bool            `json:"is_active,omitempty"`
}

// ContactFilter provides filtering options for listing contacts
type ContactFilter struct {
	ContactType ContactType
	ActiveOnly  bool
	Search      string
}
