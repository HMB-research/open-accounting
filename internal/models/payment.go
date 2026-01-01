package models

import (
	"time"
)

// PaymentType represents the type of payment
type PaymentType string

const (
	PaymentTypeReceived PaymentType = "RECEIVED"
	PaymentTypeMade     PaymentType = "MADE"
)

// Payment represents a payment received or made (GORM model)
type Payment struct {
	ID             string      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	PaymentNumber  string      `gorm:"column:payment_number;size:50;not null" json:"payment_number"`
	PaymentType    PaymentType `gorm:"column:payment_type;size:20;not null" json:"payment_type"`
	ContactID      *string     `gorm:"column:contact_id;type:uuid;index" json:"contact_id,omitempty"`
	PaymentDate    time.Time   `gorm:"column:payment_date;type:date;not null" json:"payment_date"`
	Amount         Decimal     `gorm:"type:numeric(28,8);not null;default:0" json:"amount"`
	Currency       string      `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	ExchangeRate   Decimal     `gorm:"column:exchange_rate;type:numeric(18,10);not null;default:1" json:"exchange_rate"`
	BaseAmount     Decimal     `gorm:"column:base_amount;type:numeric(28,8);not null;default:0" json:"base_amount"`
	PaymentMethod  string      `gorm:"column:payment_method;size:50" json:"payment_method,omitempty"`
	BankAccount    string      `gorm:"column:bank_account;size:100" json:"bank_account,omitempty"`
	Reference      string      `gorm:"size:255" json:"reference,omitempty"`
	Notes          string      `gorm:"type:text" json:"notes,omitempty"`
	JournalEntryID *string     `gorm:"column:journal_entry_id;type:uuid" json:"journal_entry_id,omitempty"`
	CreatedAt      time.Time   `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy      string      `gorm:"type:uuid;not null" json:"created_by"`

	// Relations
	Allocations []PaymentAllocation `gorm:"foreignKey:PaymentID" json:"allocations,omitempty"`
	Contact     *Contact            `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
}

// TableName returns the table name for GORM
func (Payment) TableName() string {
	return "payments"
}

// PaymentAllocation represents how a payment is allocated to invoices (GORM model)
type PaymentAllocation struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	PaymentID string    `gorm:"column:payment_id;type:uuid;not null;index" json:"payment_id"`
	InvoiceID string    `gorm:"column:invoice_id;type:uuid;not null;index" json:"invoice_id"`
	Amount    Decimal   `gorm:"type:numeric(28,8);not null;default:0" json:"amount"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	Payment *Payment `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
	Invoice *Invoice `gorm:"foreignKey:InvoiceID" json:"invoice,omitempty"`
}

// TableName returns the table name for GORM
func (PaymentAllocation) TableName() string {
	return "payment_allocations"
}
