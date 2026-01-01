package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wneessen/go-mail"
)

// MailSender is an interface for sending mail (for testing)
type MailSender interface {
	SendMail(config *SMTPConfig, m *mail.Msg) error
}

// DefaultMailSender implements MailSender using go-mail
type DefaultMailSender struct{}

// SendMail sends an email using go-mail
func (d *DefaultMailSender) SendMail(config *SMTPConfig, m *mail.Msg) error {
	var opts []mail.Option
	opts = append(opts, mail.WithPort(config.Port))

	if config.Username != "" {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthPlain))
		opts = append(opts, mail.WithUsername(config.Username))
		opts = append(opts, mail.WithPassword(config.Password))
	}

	if config.UseTLS {
		opts = append(opts, mail.WithTLSPortPolicy(mail.TLSMandatory))
		opts = append(opts, mail.WithTLSConfig(&tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		}))
	}

	client, err := mail.NewClient(config.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	if err := client.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// Service handles email operations
type Service struct {
	db     *pgxpool.Pool
	repo   Repository
	mailer MailSender
}

// NewService creates a new email service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:     db,
		repo:   NewPostgresRepository(db),
		mailer: &DefaultMailSender{},
	}
}

// NewServiceWithRepository creates a new email service with a custom repository (for testing)
func NewServiceWithRepository(repo Repository, mailer MailSender) *Service {
	return &Service{
		repo:   repo,
		mailer: mailer,
	}
}

// EnsureSchema creates email tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	return s.repo.EnsureSchema(ctx, schemaName)
}

// GetSMTPConfig retrieves SMTP configuration for a tenant
func (s *Service) GetSMTPConfig(ctx context.Context, tenantID string) (*SMTPConfig, error) {
	settingsJSON, err := s.repo.GetTenantSettings(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant settings: %w", err)
	}

	return ParseSMTPConfig(settingsJSON)
}

// UpdateSMTPConfig updates SMTP configuration for a tenant
func (s *Service) UpdateSMTPConfig(ctx context.Context, tenantID string, req *UpdateSMTPConfigRequest) error {
	// Get current settings
	settingsJSON, err := s.repo.GetTenantSettings(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant settings: %w", err)
	}

	// Merge new SMTP settings
	newSettingsJSON, err := MergeSMTPConfig(settingsJSON, req)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := s.repo.UpdateTenantSettings(ctx, tenantID, newSettingsJSON); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	return nil
}

// TestSMTP tests the SMTP configuration
func (s *Service) TestSMTP(ctx context.Context, tenantID string, recipientEmail string) (*TestSMTPResponse, error) {
	config, err := s.GetSMTPConfig(ctx, tenantID)
	if err != nil {
		return &TestSMTPResponse{Success: false, Message: err.Error()}, nil
	}

	if !config.IsConfigured() {
		return &TestSMTPResponse{Success: false, Message: "SMTP is not configured"}, nil
	}

	// Create test message
	m := mail.NewMsg()
	if err := m.From(config.FromEmail); err != nil {
		return &TestSMTPResponse{Success: false, Message: fmt.Sprintf("invalid from address: %v", err)}, nil
	}
	if err := m.To(recipientEmail); err != nil {
		return &TestSMTPResponse{Success: false, Message: fmt.Sprintf("invalid recipient address: %v", err)}, nil
	}
	m.Subject("Test Email from Open Accounting")
	m.SetBodyString(mail.TypeTextPlain, "This is a test email to verify your SMTP configuration is working correctly.")

	// Send test email
	if err := s.mailer.SendMail(config, m); err != nil {
		return &TestSMTPResponse{Success: false, Message: fmt.Sprintf("failed to send: %v", err)}, nil
	}

	return &TestSMTPResponse{Success: true, Message: "Test email sent successfully"}, nil
}

// GetTemplate retrieves an email template
func (s *Service) GetTemplate(ctx context.Context, schemaName string, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
	tmpl, err := s.repo.GetTemplate(ctx, schemaName, tenantID, templateType)
	if err == ErrTemplateNotFound {
		// Return default template if not found
		defaults := DefaultTemplates()
		if defaultTmpl, ok := defaults[templateType]; ok {
			defaultTmpl.TenantID = tenantID
			return &defaultTmpl, nil
		}
		return nil, fmt.Errorf("template not found: %w", err)
	}
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

// ListTemplates lists all email templates for a tenant
func (s *Service) ListTemplates(ctx context.Context, schemaName string, tenantID string) ([]EmailTemplate, error) {
	// First ensure schema exists
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	templates, err := s.repo.ListTemplates(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	existingTypes := make(map[TemplateType]bool)
	for _, tmpl := range templates {
		existingTypes[tmpl.TemplateType] = true
	}

	// Add default templates for missing types
	defaults := DefaultTemplates()
	for templateType, defaultTmpl := range defaults {
		if !existingTypes[templateType] {
			defaultTmpl.TenantID = tenantID
			templates = append(templates, defaultTmpl)
		}
	}

	return templates, nil
}

// UpdateTemplate updates an email template
func (s *Service) UpdateTemplate(ctx context.Context, schemaName string, tenantID string, templateType TemplateType, req *UpdateTemplateRequest) (*EmailTemplate, error) {
	// Ensure schema exists
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	tmpl := &EmailTemplate{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		TemplateType: templateType,
		Subject:      req.Subject,
		BodyHTML:     req.BodyHTML,
		BodyText:     req.BodyText,
		IsActive:     req.IsActive,
	}

	if err := s.repo.UpsertTemplate(ctx, schemaName, tmpl); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	return tmpl, nil
}

// SendEmail sends an email using the tenant's SMTP configuration
func (s *Service) SendEmail(ctx context.Context, schemaName string, tenantID string, emailType string, recipient string, recipientName string, subject string, bodyHTML string, bodyText string, attachments []Attachment, relatedID string) (*EmailSentResponse, error) {
	// Get SMTP config
	config, err := s.GetSMTPConfig(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMTP config: %w", err)
	}

	if !config.IsConfigured() {
		return nil, fmt.Errorf("SMTP is not configured for this organization")
	}

	// Ensure schema exists
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	// Create email log entry
	logID := uuid.New().String()
	emailLog := &EmailLog{
		ID:             logID,
		TenantID:       tenantID,
		EmailType:      emailType,
		RecipientEmail: recipient,
		RecipientName:  recipientName,
		Subject:        subject,
		Status:         StatusPending,
		RelatedID:      relatedID,
	}
	if err := s.repo.CreateEmailLog(ctx, schemaName, emailLog); err != nil {
		return nil, fmt.Errorf("failed to create email log: %w", err)
	}

	// Create message
	m := mail.NewMsg()
	if config.FromName != "" {
		if err := m.FromFormat(config.FromName, config.FromEmail); err != nil {
			return s.logEmailError(ctx, schemaName, logID, err)
		}
	} else {
		if err := m.From(config.FromEmail); err != nil {
			return s.logEmailError(ctx, schemaName, logID, err)
		}
	}

	if recipientName != "" {
		if err := m.AddToFormat(recipientName, recipient); err != nil {
			return s.logEmailError(ctx, schemaName, logID, err)
		}
	} else {
		if err := m.To(recipient); err != nil {
			return s.logEmailError(ctx, schemaName, logID, err)
		}
	}

	m.Subject(subject)
	m.SetBodyString(mail.TypeTextHTML, bodyHTML)
	if bodyText != "" {
		m.AddAlternativeString(mail.TypeTextPlain, bodyText)
	}

	// Add attachments
	for _, att := range attachments {
		if err := m.AttachReader(att.Filename, bytes.NewReader(att.Content)); err != nil {
			return s.logEmailError(ctx, schemaName, logID, fmt.Errorf("attach file %s: %w", att.Filename, err))
		}
	}

	// Send email
	if err := s.mailer.SendMail(config, m); err != nil {
		return s.logEmailError(ctx, schemaName, logID, err)
	}

	// Update log as sent
	now := time.Now()
	if err := s.repo.UpdateEmailLogStatus(ctx, schemaName, logID, StatusSent, &now, ""); err != nil {
		// Email was sent, just log the error
		fmt.Printf("failed to update email log: %v\n", err)
	}

	return &EmailSentResponse{
		Success: true,
		LogID:   logID,
		Message: "Email sent successfully",
	}, nil
}

// logEmailError logs an email error and returns the response
func (s *Service) logEmailError(ctx context.Context, schemaName string, logID string, sendErr error) (*EmailSentResponse, error) {
	if err := s.repo.UpdateEmailLogStatus(ctx, schemaName, logID, StatusFailed, nil, sendErr.Error()); err != nil {
		fmt.Printf("failed to update email log: %v\n", err)
	}
	return nil, fmt.Errorf("failed to send email: %w", sendErr)
}

// RenderTemplate renders an email template with data
func (s *Service) RenderTemplate(tmpl *EmailTemplate, data *TemplateData) (subject string, bodyHTML string, bodyText string, err error) {
	// Parse and render subject
	subjectTmpl, err := template.New("subject").Parse(tmpl.Subject)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render subject: %w", err)
	}
	subject = subjectBuf.String()

	// Parse and render HTML body
	htmlTmpl, err := template.New("body_html").Parse(tmpl.BodyHTML)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse HTML template: %w", err)
	}
	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render HTML: %w", err)
	}
	bodyHTML = htmlBuf.String()

	// Parse and render text body if present
	if tmpl.BodyText != "" {
		textTmpl, err := template.New("body_text").Parse(tmpl.BodyText)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse text template: %w", err)
		}
		var textBuf bytes.Buffer
		if err := textTmpl.Execute(&textBuf, data); err != nil {
			return "", "", "", fmt.Errorf("failed to render text: %w", err)
		}
		bodyText = textBuf.String()
	}

	return subject, bodyHTML, bodyText, nil
}

// GetEmailLog retrieves email logs for a tenant
func (s *Service) GetEmailLog(ctx context.Context, schemaName string, tenantID string, limit int) ([]EmailLog, error) {
	logs, err := s.repo.GetEmailLog(ctx, schemaName, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get email log: %w", err)
	}
	return logs, nil
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	Content     []byte
	ContentType string
}

// InvoiceEmailData holds data for invoice-related emails
type InvoiceEmailData struct {
	InvoiceID     string
	InvoiceNumber string
	ContactName   string
	ContactEmail  string
	TotalAmount   string
	Currency      string
	IssueDate     time.Time
	DueDate       time.Time
	CompanyName   string
}

// PaymentEmailData holds data for payment-related emails
type PaymentEmailData struct {
	PaymentID    string
	ContactName  string
	ContactEmail string
	Amount       string
	Currency     string
	PaymentDate  time.Time
	Reference    string
	CompanyName  string
}
