package models

import (
	"time"
)

// TemplateType represents the type of email template
type TemplateType string

const (
	TemplateInvoiceSend     TemplateType = "INVOICE_SEND"
	TemplatePaymentReceipt  TemplateType = "PAYMENT_RECEIPT"
	TemplateOverdueReminder TemplateType = "OVERDUE_REMINDER"
)

// EmailStatus represents the status of an email
type EmailStatus string

const (
	EmailStatusPending EmailStatus = "PENDING"
	EmailStatusSent    EmailStatus = "SENT"
	EmailStatusFailed  EmailStatus = "FAILED"
)

// EmailTemplate represents an email template in GORM
type EmailTemplate struct {
	ID           string       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string       `gorm:"type:uuid;not null;index" json:"tenant_id"`
	TemplateType TemplateType `gorm:"column:template_type;size:50;not null" json:"template_type"`
	Subject      string       `gorm:"type:text;not null" json:"subject"`
	BodyHTML     string       `gorm:"column:body_html;type:text;not null" json:"body_html"`
	BodyText     string       `gorm:"column:body_text;type:text" json:"body_text,omitempty"`
	IsActive     bool         `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time    `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time    `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM
func (EmailTemplate) TableName() string {
	return "email_templates"
}

// EmailLog represents a sent email log entry in GORM
type EmailLog struct {
	ID             string      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EmailType      string      `gorm:"column:email_type;size:50;not null" json:"email_type"`
	RecipientEmail string      `gorm:"size:255;not null" json:"recipient_email"`
	RecipientName  string      `gorm:"size:255" json:"recipient_name,omitempty"`
	Subject        string      `gorm:"type:text;not null" json:"subject"`
	Status         EmailStatus `gorm:"size:20;not null;default:'PENDING'" json:"status"`
	SentAt         *time.Time  `gorm:"" json:"sent_at,omitempty"`
	ErrorMessage   string      `gorm:"type:text" json:"error_message,omitempty"`
	RelatedID      *string     `gorm:"type:uuid" json:"related_id,omitempty"`
	CreatedAt      time.Time   `gorm:"not null;default:now()" json:"created_at"`
}

// TableName returns the table name for GORM
func (EmailLog) TableName() string {
	return "email_log"
}
