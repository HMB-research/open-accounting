package models

import "time"

// ReminderRule stores a tenant invoice reminder rule.
type ReminderRule struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID          string    `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Name              string    `gorm:"size:100;not null" json:"name"`
	TriggerType       string    `gorm:"column:trigger_type;size:20;not null" json:"trigger_type"`
	DaysOffset        int       `gorm:"column:days_offset;not null;default:0" json:"days_offset"`
	EmailTemplateType string    `gorm:"column:email_template_type;size:50;not null;default:'OVERDUE_REMINDER'" json:"email_template_type"`
	IsActive          bool      `gorm:"column:is_active;not null" json:"is_active"`
	CreatedAt         time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (ReminderRule) TableName() string {
	return "reminder_rules"
}

// PaymentReminder stores one invoice reminder attempt.
type PaymentReminder struct {
	ID             string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string     `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	InvoiceID      string     `gorm:"column:invoice_id;type:uuid;not null;index" json:"invoice_id"`
	InvoiceNumber  string     `gorm:"column:invoice_number;size:50;not null" json:"invoice_number"`
	ContactID      string     `gorm:"column:contact_id;type:uuid;not null" json:"contact_id"`
	ContactName    string     `gorm:"column:contact_name;size:255;not null" json:"contact_name"`
	ContactEmail   *string    `gorm:"column:contact_email;size:255" json:"contact_email,omitempty"`
	RuleID         *string    `gorm:"column:rule_id;type:uuid" json:"rule_id,omitempty"`
	TriggerType    string     `gorm:"column:trigger_type;size:20;not null" json:"trigger_type"`
	DaysOffset     int        `gorm:"column:days_offset;not null;default:0" json:"days_offset"`
	ReminderNumber int        `gorm:"column:reminder_number;not null;default:1" json:"reminder_number"`
	Status         string     `gorm:"size:20;not null;default:'PENDING'" json:"status"`
	SentAt         *time.Time `gorm:"column:sent_at" json:"sent_at,omitempty"`
	ErrorMessage   *string    `gorm:"column:error_message;type:text" json:"error_message,omitempty"`
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (PaymentReminder) TableName() string {
	return "payment_reminders"
}
