package invoicing

import "time"

// TriggerType represents when a reminder should be triggered
type TriggerType string

const (
	TriggerBeforeDue TriggerType = "BEFORE_DUE"
	TriggerOnDue     TriggerType = "ON_DUE"
	TriggerAfterDue  TriggerType = "AFTER_DUE"
)

// ReminderRule defines when automated reminders should be sent
type ReminderRule struct {
	ID                string      `json:"id"`
	TenantID          string      `json:"tenant_id"`
	Name              string      `json:"name"`
	TriggerType       TriggerType `json:"trigger_type"`
	DaysOffset        int         `json:"days_offset"`
	EmailTemplateType string      `json:"email_template_type"`
	IsActive          bool        `json:"is_active"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// CreateReminderRuleRequest is the request to create a reminder rule
type CreateReminderRuleRequest struct {
	Name              string      `json:"name"`
	TriggerType       TriggerType `json:"trigger_type"`
	DaysOffset        int         `json:"days_offset"`
	EmailTemplateType string      `json:"email_template_type,omitempty"`
	IsActive          bool        `json:"is_active"`
}

// Validate validates the create rule request
func (r *CreateReminderRuleRequest) Validate() error {
	if r.Name == "" {
		return ErrRuleNameRequired
	}
	if r.TriggerType == "" {
		return ErrTriggerTypeRequired
	}
	if r.TriggerType != TriggerBeforeDue && r.TriggerType != TriggerOnDue && r.TriggerType != TriggerAfterDue {
		return ErrInvalidTriggerType
	}
	if r.DaysOffset < 0 {
		return ErrInvalidDaysOffset
	}
	return nil
}

// UpdateReminderRuleRequest is the request to update a reminder rule
type UpdateReminderRuleRequest struct {
	Name              *string `json:"name,omitempty"`
	EmailTemplateType *string `json:"email_template_type,omitempty"`
	IsActive          *bool   `json:"is_active,omitempty"`
}

// InvoiceForReminder represents an invoice that may need a reminder
type InvoiceForReminder struct {
	ID                string `json:"id"`
	InvoiceNumber     string `json:"invoice_number"`
	ContactID         string `json:"contact_id"`
	ContactName       string `json:"contact_name"`
	ContactEmail      string `json:"contact_email,omitempty"`
	IssueDate         string `json:"issue_date"`
	DueDate           string `json:"due_date"`
	Total             string `json:"total"`
	AmountPaid        string `json:"amount_paid"`
	OutstandingAmount string `json:"outstanding_amount"`
	Currency          string `json:"currency"`
	DaysUntilDue      int    `json:"days_until_due"`  // Negative if overdue
	DaysOverdue       int    `json:"days_overdue"`    // 0 if not overdue
}

// AutomatedReminderResult represents the result of an automated reminder run
type AutomatedReminderResult struct {
	TenantID      string    `json:"tenant_id"`
	RuleID        string    `json:"rule_id"`
	RuleName      string    `json:"rule_name"`
	InvoicesFound int       `json:"invoices_found"`
	RemindersSent int       `json:"reminders_sent"`
	Skipped       int       `json:"skipped"`
	Failed        int       `json:"failed"`
	Errors        []string  `json:"errors,omitempty"`
	RunAt         time.Time `json:"run_at"`
}

// Errors
var (
	ErrRuleNameRequired    = &ValidationError{Field: "name", Message: "rule name is required"}
	ErrTriggerTypeRequired = &ValidationError{Field: "trigger_type", Message: "trigger type is required"}
	ErrInvalidTriggerType  = &ValidationError{Field: "trigger_type", Message: "invalid trigger type"}
	ErrInvalidDaysOffset   = &ValidationError{Field: "days_offset", Message: "days offset cannot be negative"}
	ErrRuleNotFound        = &NotFoundError{Entity: "reminder rule"}
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NotFoundError represents a not found error
type NotFoundError struct {
	Entity string
}

func (e *NotFoundError) Error() string {
	return e.Entity + " not found"
}
