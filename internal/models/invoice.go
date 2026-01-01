package models

import (
	"time"
)

// InvoiceType represents the type of invoice
type InvoiceType string

const (
	InvoiceTypeSales      InvoiceType = "SALES"
	InvoiceTypePurchase   InvoiceType = "PURCHASE"
	InvoiceTypeCreditNote InvoiceType = "CREDIT_NOTE"
)

// InvoiceStatus represents the status of an invoice
type InvoiceStatus string

const (
	InvoiceStatusDraft         InvoiceStatus = "DRAFT"
	InvoiceStatusSent          InvoiceStatus = "SENT"
	InvoiceStatusPartiallyPaid InvoiceStatus = "PARTIALLY_PAID"
	InvoiceStatusPaid          InvoiceStatus = "PAID"
	InvoiceStatusOverdue       InvoiceStatus = "OVERDUE"
	InvoiceStatusVoided        InvoiceStatus = "VOIDED"
)

// Invoice represents a sales or purchase invoice (GORM model)
type Invoice struct {
	ID             string        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string        `gorm:"type:uuid;not null;index" json:"tenant_id"`
	InvoiceNumber  string        `gorm:"size:50;not null" json:"invoice_number"`
	InvoiceType    InvoiceType   `gorm:"column:invoice_type;size:20;not null" json:"invoice_type"`
	ContactID      string        `gorm:"type:uuid;not null;index" json:"contact_id"`
	IssueDate      time.Time     `gorm:"type:date;not null" json:"issue_date"`
	DueDate        time.Time     `gorm:"type:date;not null" json:"due_date"`
	Currency       string        `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	ExchangeRate   Decimal       `gorm:"type:numeric(18,10);not null;default:1" json:"exchange_rate"`
	Subtotal       Decimal       `gorm:"type:numeric(28,8);not null;default:0" json:"subtotal"`
	VATAmount      Decimal       `gorm:"column:vat_amount;type:numeric(28,8);not null;default:0" json:"vat_amount"`
	Total          Decimal       `gorm:"type:numeric(28,8);not null;default:0" json:"total"`
	BaseSubtotal   Decimal       `gorm:"column:base_subtotal;type:numeric(28,8);not null;default:0" json:"base_subtotal"`
	BaseVATAmount  Decimal       `gorm:"column:base_vat_amount;type:numeric(28,8);not null;default:0" json:"base_vat_amount"`
	BaseTotal      Decimal       `gorm:"column:base_total;type:numeric(28,8);not null;default:0" json:"base_total"`
	AmountPaid     Decimal       `gorm:"column:amount_paid;type:numeric(28,8);not null;default:0" json:"amount_paid"`
	Status         InvoiceStatus `gorm:"size:20;not null;default:'DRAFT'" json:"status"`
	Reference      string        `gorm:"size:255" json:"reference,omitempty"`
	Notes          string        `gorm:"type:text" json:"notes,omitempty"`
	JournalEntryID *string       `gorm:"column:journal_entry_id;type:uuid" json:"journal_entry_id,omitempty"`
	EInvoiceSentAt *time.Time    `gorm:"column:einvoice_sent_at" json:"einvoice_sent_at,omitempty"`
	EInvoiceID     *string       `gorm:"column:einvoice_id;size:255" json:"einvoice_id,omitempty"`
	CreatedAt      time.Time     `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy      string        `gorm:"type:uuid;not null" json:"created_by"`
	UpdatedAt      time.Time     `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	Lines   []InvoiceLine `gorm:"foreignKey:InvoiceID" json:"lines,omitempty"`
	Contact *Contact      `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
}

// TableName returns the table name for GORM
func (Invoice) TableName() string {
	return "invoices"
}

// InvoiceLine represents a line item on an invoice (GORM model)
type InvoiceLine struct {
	ID              string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	InvoiceID       string  `gorm:"column:invoice_id;type:uuid;not null;index" json:"invoice_id"`
	LineNumber      int     `gorm:"column:line_number;not null" json:"line_number"`
	Description     string  `gorm:"type:text;not null" json:"description"`
	Quantity        Decimal `gorm:"type:numeric(28,8);not null;default:1" json:"quantity"`
	Unit            string  `gorm:"size:20" json:"unit,omitempty"`
	UnitPrice       Decimal `gorm:"column:unit_price;type:numeric(28,8);not null;default:0" json:"unit_price"`
	DiscountPercent Decimal `gorm:"column:discount_percent;type:numeric(5,2);not null;default:0" json:"discount_percent"`
	VATRate         Decimal `gorm:"column:vat_rate;type:numeric(5,2);not null;default:0" json:"vat_rate"`
	LineSubtotal    Decimal `gorm:"column:line_subtotal;type:numeric(28,8);not null;default:0" json:"line_subtotal"`
	LineVAT         Decimal `gorm:"column:line_vat;type:numeric(28,8);not null;default:0" json:"line_vat"`
	LineTotal       Decimal `gorm:"column:line_total;type:numeric(28,8);not null;default:0" json:"line_total"`
	AccountID       *string `gorm:"column:account_id;type:uuid" json:"account_id,omitempty"`
	ProductID       *string `gorm:"column:product_id;type:uuid" json:"product_id,omitempty"`

	// Relations
	Invoice *Invoice `gorm:"foreignKey:InvoiceID" json:"invoice,omitempty"`
}

// TableName returns the table name for GORM
func (InvoiceLine) TableName() string {
	return "invoice_lines"
}
