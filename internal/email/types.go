package email

import (
	"errors"
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
	StatusPending EmailStatus = "PENDING"
	StatusSent    EmailStatus = "SENT"
	StatusFailed  EmailStatus = "FAILED"
)

// SMTPConfig holds SMTP server configuration
type SMTPConfig struct {
	Host              string `json:"smtp_host"`
	Port              int    `json:"smtp_port"`
	Username          string `json:"smtp_username"`
	Password          string `json:"smtp_password,omitempty"`
	PasswordEncrypted string `json:"-"`
	FromEmail         string `json:"smtp_from_email"`
	FromName          string `json:"smtp_from_name"`
	UseTLS            bool   `json:"smtp_use_tls"`
}

// Validate validates the SMTP configuration
func (c *SMTPConfig) Validate() error {
	if c.Host == "" {
		return errors.New("SMTP host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("invalid SMTP port")
	}
	if c.FromEmail == "" {
		return errors.New("from email is required")
	}
	return nil
}

// IsConfigured returns true if SMTP is configured
func (c *SMTPConfig) IsConfigured() bool {
	return c.Host != "" && c.Port > 0 && c.FromEmail != ""
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	ID           string       `json:"id"`
	TenantID     string       `json:"tenant_id"`
	TemplateType TemplateType `json:"template_type"`
	Subject      string       `json:"subject"`
	BodyHTML     string       `json:"body_html"`
	BodyText     string       `json:"body_text,omitempty"`
	IsActive     bool         `json:"is_active"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// EmailLog represents a sent email log entry
type EmailLog struct {
	ID             string      `json:"id"`
	TenantID       string      `json:"tenant_id"`
	EmailType      string      `json:"email_type"`
	RecipientEmail string      `json:"recipient_email"`
	RecipientName  string      `json:"recipient_name,omitempty"`
	Subject        string      `json:"subject"`
	Status         EmailStatus `json:"status"`
	SentAt         *time.Time  `json:"sent_at,omitempty"`
	ErrorMessage   string      `json:"error_message,omitempty"`
	RelatedID      string      `json:"related_id,omitempty"` // Invoice ID or Payment ID
	CreatedAt      time.Time   `json:"created_at"`
}

// SendInvoiceRequest represents a request to send an invoice via email
type SendInvoiceRequest struct {
	RecipientEmail string `json:"recipient_email"`
	RecipientName  string `json:"recipient_name,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Message        string `json:"message,omitempty"`
	AttachPDF      bool   `json:"attach_pdf"`
}

// Validate validates the send invoice request
func (r *SendInvoiceRequest) Validate() error {
	if r.RecipientEmail == "" {
		return errors.New("recipient email is required")
	}
	return nil
}

// SendPaymentReceiptRequest represents a request to send a payment receipt
type SendPaymentReceiptRequest struct {
	RecipientEmail string `json:"recipient_email"`
	RecipientName  string `json:"recipient_name,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Message        string `json:"message,omitempty"`
}

// Validate validates the send payment receipt request
func (r *SendPaymentReceiptRequest) Validate() error {
	if r.RecipientEmail == "" {
		return errors.New("recipient email is required")
	}
	return nil
}

// UpdateSMTPConfigRequest represents a request to update SMTP settings
type UpdateSMTPConfigRequest struct {
	Host      string `json:"smtp_host"`
	Port      int    `json:"smtp_port"`
	Username  string `json:"smtp_username"`
	Password  string `json:"smtp_password,omitempty"`
	FromEmail string `json:"smtp_from_email"`
	FromName  string `json:"smtp_from_name"`
	UseTLS    bool   `json:"smtp_use_tls"`
}

// UpdateTemplateRequest represents a request to update an email template
type UpdateTemplateRequest struct {
	Subject  string `json:"subject"`
	BodyHTML string `json:"body_html"`
	BodyText string `json:"body_text,omitempty"`
	IsActive bool   `json:"is_active"`
}

// TestSMTPRequest represents a request to test SMTP configuration
type TestSMTPRequest struct {
	RecipientEmail string `json:"recipient_email"`
}

// TestSMTPResponse represents the response from a SMTP test
type TestSMTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// EmailSentResponse represents the response after sending an email
type EmailSentResponse struct {
	Success bool   `json:"success"`
	LogID   string `json:"log_id"`
	Message string `json:"message"`
}

// DefaultTemplates returns the default email templates
func DefaultTemplates() map[TemplateType]EmailTemplate {
	return map[TemplateType]EmailTemplate{
		TemplateInvoiceSend: {
			TemplateType: TemplateInvoiceSend,
			Subject:      "Invoice {{.InvoiceNumber}} from {{.CompanyName}}",
			BodyHTML: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
<h2>Invoice {{.InvoiceNumber}}</h2>
<p>Dear {{.ContactName}},</p>
<p>Please find attached invoice {{.InvoiceNumber}} for {{.TotalAmount}} {{.Currency}}.</p>
<p><strong>Due Date:</strong> {{.DueDate}}</p>
{{if .Message}}
<p>{{.Message}}</p>
{{end}}
<p>Thank you for your business.</p>
<p>Best regards,<br>{{.CompanyName}}</p>
</div>
</body>
</html>`,
			IsActive: true,
		},
		TemplatePaymentReceipt: {
			TemplateType: TemplatePaymentReceipt,
			Subject:      "Payment Receipt from {{.CompanyName}}",
			BodyHTML: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
<h2>Payment Receipt</h2>
<p>Dear {{.ContactName}},</p>
<p>We have received your payment of {{.Amount}} {{.Currency}} on {{.PaymentDate}}.</p>
<p><strong>Payment Reference:</strong> {{.Reference}}</p>
{{if .Message}}
<p>{{.Message}}</p>
{{end}}
<p>Thank you for your payment.</p>
<p>Best regards,<br>{{.CompanyName}}</p>
</div>
</body>
</html>`,
			IsActive: true,
		},
		TemplateOverdueReminder: {
			TemplateType: TemplateOverdueReminder,
			Subject:      "Overdue Invoice Reminder - {{.InvoiceNumber}}",
			BodyHTML: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
<h2>Payment Reminder</h2>
<p>Dear {{.ContactName}},</p>
<p>This is a friendly reminder that invoice {{.InvoiceNumber}} for {{.TotalAmount}} {{.Currency}} is now overdue.</p>
<p><strong>Original Due Date:</strong> {{.DueDate}}</p>
<p><strong>Days Overdue:</strong> {{.DaysOverdue}}</p>
<p>Please arrange payment at your earliest convenience.</p>
<p>If you have already made payment, please disregard this reminder.</p>
<p>Best regards,<br>{{.CompanyName}}</p>
</div>
</body>
</html>`,
			IsActive: true,
		},
	}
}

// TemplateData holds data for rendering email templates
type TemplateData struct {
	// Common fields
	CompanyName string
	ContactName string
	Message     string

	// Invoice fields
	InvoiceNumber string
	TotalAmount   string
	Currency      string
	DueDate       string
	IssueDate     string
	DaysOverdue   int

	// Payment fields
	Amount      string
	PaymentDate string
	Reference   string
}
