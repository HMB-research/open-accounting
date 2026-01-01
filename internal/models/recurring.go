package models

import (
	"time"
)

// Frequency represents how often a recurring invoice is generated
type Frequency string

const (
	FrequencyWeekly    Frequency = "WEEKLY"
	FrequencyBiweekly  Frequency = "BIWEEKLY"
	FrequencyMonthly   Frequency = "MONTHLY"
	FrequencyQuarterly Frequency = "QUARTERLY"
	FrequencyYearly    Frequency = "YEARLY"
)

// RecurringInvoice represents a recurring invoice template (GORM model)
type RecurringInvoice struct {
	ID                 string     `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID           string     `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name               string     `gorm:"size:100;not null" json:"name"`
	ContactID          string     `gorm:"column:contact_id;type:uuid;not null" json:"contact_id"`
	InvoiceType        string     `gorm:"column:invoice_type;size:20;not null;default:'SALES'" json:"invoice_type"`
	Currency           string     `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	Frequency          Frequency  `gorm:"size:20;not null" json:"frequency"`
	StartDate          time.Time  `gorm:"column:start_date;type:date;not null" json:"start_date"`
	EndDate            *time.Time `gorm:"column:end_date;type:date" json:"end_date,omitempty"`
	NextGenerationDate time.Time  `gorm:"column:next_generation_date;type:date;not null" json:"next_generation_date"`
	PaymentTermsDays   int        `gorm:"column:payment_terms_days;not null;default:14" json:"payment_terms_days"`
	Reference          string     `gorm:"type:text" json:"reference,omitempty"`
	Notes              string     `gorm:"type:text" json:"notes,omitempty"`
	IsActive           bool       `gorm:"column:is_active;not null;default:true" json:"is_active"`
	LastGeneratedAt    *time.Time `gorm:"column:last_generated_at" json:"last_generated_at,omitempty"`
	GeneratedCount     int        `gorm:"column:generated_count;not null;default:0" json:"generated_count"`
	CreatedAt          time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy          string     `gorm:"column:created_by;type:uuid;not null" json:"created_by"`
	UpdatedAt          time.Time  `gorm:"not null;default:now()" json:"updated_at"`

	// Email configuration
	SendEmailOnGeneration  bool   `gorm:"column:send_email_on_generation;default:false" json:"send_email_on_generation"`
	EmailTemplateType      string `gorm:"column:email_template_type;size:50;default:'INVOICE_SEND'" json:"email_template_type,omitempty"`
	RecipientEmailOverride string `gorm:"column:recipient_email_override;type:text" json:"recipient_email_override,omitempty"`
	AttachPDFToEmail       bool   `gorm:"column:attach_pdf_to_email;default:true" json:"attach_pdf_to_email"`
	EmailSubjectOverride   string `gorm:"column:email_subject_override;type:text" json:"email_subject_override,omitempty"`
	EmailMessage           string `gorm:"column:email_message;type:text" json:"email_message,omitempty"`

	// Relations
	Lines []RecurringInvoiceLine `gorm:"foreignKey:RecurringInvoiceID" json:"lines,omitempty"`
}

// TableName returns the table name for GORM
func (RecurringInvoice) TableName() string {
	return "recurring_invoices"
}

// RecurringInvoiceLine represents a line item on a recurring invoice (GORM model)
type RecurringInvoiceLine struct {
	ID                 string  `gorm:"type:uuid;primaryKey" json:"id"`
	RecurringInvoiceID string  `gorm:"column:recurring_invoice_id;type:uuid;not null;index" json:"recurring_invoice_id"`
	LineNumber         int     `gorm:"column:line_number;not null" json:"line_number"`
	Description        string  `gorm:"type:text;not null" json:"description"`
	Quantity           Decimal `gorm:"type:numeric(18,6);not null;default:1" json:"quantity"`
	Unit               string  `gorm:"size:20" json:"unit,omitempty"`
	UnitPrice          Decimal `gorm:"column:unit_price;type:numeric(28,8);not null" json:"unit_price"`
	DiscountPercent    Decimal `gorm:"column:discount_percent;type:numeric(5,2);not null;default:0" json:"discount_percent"`
	VATRate            Decimal `gorm:"column:vat_rate;type:numeric(5,2);not null;default:0" json:"vat_rate"`
	AccountID          *string `gorm:"column:account_id;type:uuid" json:"account_id,omitempty"`
	ProductID          *string `gorm:"column:product_id;type:uuid" json:"product_id,omitempty"`

	// Relations
	RecurringInvoice *RecurringInvoice `gorm:"foreignKey:RecurringInvoiceID" json:"recurring_invoice,omitempty"`
}

// TableName returns the table name for GORM
func (RecurringInvoiceLine) TableName() string {
	return "recurring_invoice_lines"
}
