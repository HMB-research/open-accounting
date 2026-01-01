package models

import (
	"time"
)

// ContactType represents the type of contact
type ContactType string

const (
	ContactTypeCustomer ContactType = "CUSTOMER"
	ContactTypeSupplier ContactType = "SUPPLIER"
	ContactTypeBoth     ContactType = "BOTH"
)

// Contact represents a customer or supplier in GORM
type Contact struct {
	ID               string      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID         string      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Code             string      `gorm:"size:50" json:"code,omitempty"`
	Name             string      `gorm:"size:255;not null" json:"name"`
	ContactType      ContactType `gorm:"column:type;size:20;not null" json:"contact_type"`
	RegCode          string      `gorm:"size:50" json:"reg_code,omitempty"`
	VATNumber        string      `gorm:"size:50" json:"vat_number,omitempty"`
	Email            string      `gorm:"size:255" json:"email,omitempty"`
	Phone            string      `gorm:"size:50" json:"phone,omitempty"`
	AddressLine1     string      `gorm:"size:255" json:"address_line1,omitempty"`
	AddressLine2     string      `gorm:"size:255" json:"address_line2,omitempty"`
	City             string      `gorm:"size:100" json:"city,omitempty"`
	PostalCode       string      `gorm:"size:20" json:"postal_code,omitempty"`
	CountryCode      string      `gorm:"size:2;not null;default:'EE'" json:"country_code"`
	PaymentTermsDays int         `gorm:"not null;default:14" json:"payment_terms_days"`
	CreditLimit      Decimal     `gorm:"type:numeric(28,8);not null;default:0" json:"credit_limit"`
	DefaultAccountID *string     `gorm:"type:uuid" json:"default_account_id,omitempty"`
	IsActive         bool        `gorm:"not null;default:true" json:"is_active"`
	Notes            string      `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt        time.Time   `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time   `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM
func (Contact) TableName() string {
	return "contacts"
}
