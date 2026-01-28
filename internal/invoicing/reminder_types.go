package invoicing

import (
	"time"

	"github.com/shopspring/decimal"
)

// ReminderStatus represents the status of a payment reminder
type ReminderStatus string

const (
	ReminderStatusPending  ReminderStatus = "PENDING"
	ReminderStatusSent     ReminderStatus = "SENT"
	ReminderStatusFailed   ReminderStatus = "FAILED"
	ReminderStatusCanceled ReminderStatus = "CANCELED"
)

// PaymentReminder represents a payment reminder for an overdue invoice
type PaymentReminder struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id"`
	InvoiceID      string         `json:"invoice_id"`
	InvoiceNumber  string         `json:"invoice_number"`
	ContactID      string         `json:"contact_id"`
	ContactName    string         `json:"contact_name"`
	ContactEmail   string         `json:"contact_email"`
	RuleID         *string        `json:"rule_id,omitempty"`      // Link to reminder rule
	TriggerType    string         `json:"trigger_type,omitempty"` // BEFORE_DUE, ON_DUE, AFTER_DUE
	DaysOffset     int            `json:"days_offset,omitempty"`  // Days from due date
	ReminderNumber int            `json:"reminder_number"`        // 1st, 2nd, 3rd reminder etc.
	Status         ReminderStatus `json:"status"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// OverdueInvoice represents an overdue invoice with reminder info
type OverdueInvoice struct {
	ID                string          `json:"id"`
	InvoiceNumber     string          `json:"invoice_number"`
	ContactID         string          `json:"contact_id"`
	ContactName       string          `json:"contact_name"`
	ContactEmail      string          `json:"contact_email,omitempty"`
	IssueDate         string          `json:"issue_date"`
	DueDate           string          `json:"due_date"`
	Total             decimal.Decimal `json:"total"`
	AmountPaid        decimal.Decimal `json:"amount_paid"`
	OutstandingAmount decimal.Decimal `json:"outstanding_amount"`
	Currency          string          `json:"currency"`
	DaysOverdue       int             `json:"days_overdue"`
	ReminderCount     int             `json:"reminder_count"`
	LastReminderAt    *time.Time      `json:"last_reminder_at,omitempty"`
}

// OverdueInvoicesSummary represents a summary of overdue invoices
type OverdueInvoicesSummary struct {
	TotalOverdue       decimal.Decimal  `json:"total_overdue"`
	InvoiceCount       int              `json:"invoice_count"`
	ContactCount       int              `json:"contact_count"`
	AverageDaysOverdue int              `json:"average_days_overdue"`
	Invoices           []OverdueInvoice `json:"invoices"`
	GeneratedAt        time.Time        `json:"generated_at"`
}

// SendReminderRequest represents a request to send a payment reminder
type SendReminderRequest struct {
	InvoiceID string `json:"invoice_id"`
	Message   string `json:"message,omitempty"` // Optional custom message
}

// SendBulkRemindersRequest represents a request to send multiple reminders
type SendBulkRemindersRequest struct {
	InvoiceIDs []string `json:"invoice_ids"`
	Message    string   `json:"message,omitempty"`
}

// ReminderResult represents the result of sending a reminder
type ReminderResult struct {
	InvoiceID     string `json:"invoice_id"`
	InvoiceNumber string `json:"invoice_number"`
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	ReminderID    string `json:"reminder_id,omitempty"`
}

// BulkReminderResult represents the results of sending multiple reminders
type BulkReminderResult struct {
	TotalRequested int              `json:"total_requested"`
	Successful     int              `json:"successful"`
	Failed         int              `json:"failed"`
	Results        []ReminderResult `json:"results"`
}
