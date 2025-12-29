package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wneessen/go-mail"
)

// Service handles email operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new email service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// EnsureSchema creates email tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.email_templates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			template_type VARCHAR(50) NOT NULL,
			subject TEXT NOT NULL,
			body_html TEXT NOT NULL,
			body_text TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE (tenant_id, template_type)
		);

		CREATE TABLE IF NOT EXISTS %s.email_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			email_type VARCHAR(50) NOT NULL,
			recipient_email VARCHAR(255) NOT NULL,
			recipient_name VARCHAR(255),
			subject TEXT NOT NULL,
			status VARCHAR(20) DEFAULT 'PENDING',
			sent_at TIMESTAMPTZ,
			error_message TEXT,
			related_id UUID,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_email_log_tenant ON %s.email_log(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_email_log_status ON %s.email_log(status);
	`, schemaName, schemaName, schemaName, schemaName)

	_, err := s.db.Exec(ctx, query)
	return err
}

// GetSMTPConfig retrieves SMTP configuration for a tenant
func (s *Service) GetSMTPConfig(ctx context.Context, tenantID string) (*SMTPConfig, error) {
	var settingsJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT settings FROM tenants WHERE id = $1
	`, tenantID).Scan(&settingsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant settings: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(settingsJSON, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	config := &SMTPConfig{
		Port:   587,
		UseTLS: true,
	}

	if host, ok := settings["smtp_host"].(string); ok {
		config.Host = host
	}
	if port, ok := settings["smtp_port"].(float64); ok {
		config.Port = int(port)
	}
	if username, ok := settings["smtp_username"].(string); ok {
		config.Username = username
	}
	if password, ok := settings["smtp_password"].(string); ok {
		config.Password = password
	}
	if fromEmail, ok := settings["smtp_from_email"].(string); ok {
		config.FromEmail = fromEmail
	}
	if fromName, ok := settings["smtp_from_name"].(string); ok {
		config.FromName = fromName
	}
	if useTLS, ok := settings["smtp_use_tls"].(bool); ok {
		config.UseTLS = useTLS
	}

	return config, nil
}

// UpdateSMTPConfig updates SMTP configuration for a tenant
func (s *Service) UpdateSMTPConfig(ctx context.Context, tenantID string, req *UpdateSMTPConfigRequest) error {
	// Get current settings
	var settingsJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT settings FROM tenants WHERE id = $1
	`, tenantID).Scan(&settingsJSON)
	if err != nil {
		return fmt.Errorf("failed to get tenant settings: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(settingsJSON, &settings); err != nil {
		settings = make(map[string]interface{})
	}

	// Update SMTP settings
	settings["smtp_host"] = req.Host
	settings["smtp_port"] = req.Port
	settings["smtp_username"] = req.Username
	if req.Password != "" {
		settings["smtp_password"] = req.Password
	}
	settings["smtp_from_email"] = req.FromEmail
	settings["smtp_from_name"] = req.FromName
	settings["smtp_use_tls"] = req.UseTLS

	// Save updated settings
	newSettingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE tenants SET settings = $2, updated_at = NOW() WHERE id = $1
	`, tenantID, newSettingsJSON)
	if err != nil {
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
	if err := s.sendMail(config, m); err != nil {
		return &TestSMTPResponse{Success: false, Message: fmt.Sprintf("failed to send: %v", err)}, nil
	}

	return &TestSMTPResponse{Success: true, Message: "Test email sent successfully"}, nil
}

// GetTemplate retrieves an email template
func (s *Service) GetTemplate(ctx context.Context, schemaName string, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
	var tmpl EmailTemplate
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, template_type, subject, body_html, COALESCE(body_text, ''), is_active, created_at, updated_at
		FROM %s.email_templates
		WHERE tenant_id = $1 AND template_type = $2
	`, schemaName), tenantID, templateType).Scan(
		&tmpl.ID, &tmpl.TenantID, &tmpl.TemplateType, &tmpl.Subject, &tmpl.BodyHTML, &tmpl.BodyText, &tmpl.IsActive, &tmpl.CreatedAt, &tmpl.UpdatedAt,
	)
	if err != nil {
		// Return default template if not found
		defaults := DefaultTemplates()
		if defaultTmpl, ok := defaults[templateType]; ok {
			defaultTmpl.TenantID = tenantID
			return &defaultTmpl, nil
		}
		return nil, fmt.Errorf("template not found: %w", err)
	}
	return &tmpl, nil
}

// ListTemplates lists all email templates for a tenant
func (s *Service) ListTemplates(ctx context.Context, schemaName string, tenantID string) ([]EmailTemplate, error) {
	// First ensure schema exists
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, template_type, subject, body_html, COALESCE(body_text, ''), is_active, created_at, updated_at
		FROM %s.email_templates
		WHERE tenant_id = $1
		ORDER BY template_type
	`, schemaName), tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	templates := []EmailTemplate{}
	existingTypes := make(map[TemplateType]bool)

	for rows.Next() {
		var tmpl EmailTemplate
		if err := rows.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.TemplateType, &tmpl.Subject, &tmpl.BodyHTML, &tmpl.BodyText, &tmpl.IsActive, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, tmpl)
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

	id := uuid.New().String()
	var tmpl EmailTemplate

	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.email_templates (id, tenant_id, template_type, subject, body_html, body_text, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, template_type) DO UPDATE SET
			subject = EXCLUDED.subject,
			body_html = EXCLUDED.body_html,
			body_text = EXCLUDED.body_text,
			is_active = EXCLUDED.is_active,
			updated_at = NOW()
		RETURNING id, tenant_id, template_type, subject, body_html, COALESCE(body_text, ''), is_active, created_at, updated_at
	`, schemaName), id, tenantID, templateType, req.Subject, req.BodyHTML, req.BodyText, req.IsActive).Scan(
		&tmpl.ID, &tmpl.TenantID, &tmpl.TemplateType, &tmpl.Subject, &tmpl.BodyHTML, &tmpl.BodyText, &tmpl.IsActive, &tmpl.CreatedAt, &tmpl.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	return &tmpl, nil
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
	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.email_log (id, tenant_id, email_type, recipient_email, recipient_name, subject, status, related_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'PENDING', $7)
	`, schemaName), logID, tenantID, emailType, recipient, recipientName, subject, relatedID)
	if err != nil {
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
	if err := s.sendMail(config, m); err != nil {
		return s.logEmailError(ctx, schemaName, logID, err)
	}

	// Update log as sent
	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.email_log SET status = 'SENT', sent_at = NOW() WHERE id = $1
	`, schemaName), logID)
	if err != nil {
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
	_, err := s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.email_log SET status = 'FAILED', error_message = $2 WHERE id = $1
	`, schemaName), logID, sendErr.Error())
	if err != nil {
		fmt.Printf("failed to update email log: %v\n", err)
	}
	return nil, fmt.Errorf("failed to send email: %w", sendErr)
}

// sendMail sends an email using go-mail
func (s *Service) sendMail(config *SMTPConfig, m *mail.Msg) error {
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
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, email_type, recipient_email, COALESCE(recipient_name, ''), subject, status, sent_at, COALESCE(error_message, ''), related_id, created_at
		FROM %s.email_log
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, schemaName), tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get email log: %w", err)
	}
	defer rows.Close()

	logs := []EmailLog{}
	for rows.Next() {
		var log EmailLog
		var relatedID *string
		if err := rows.Scan(&log.ID, &log.TenantID, &log.EmailType, &log.RecipientEmail, &log.RecipientName, &log.Subject, &log.Status, &log.SentAt, &log.ErrorMessage, &relatedID, &log.CreatedAt); err != nil {
			return nil, err
		}
		if relatedID != nil {
			log.RelatedID = *relatedID
		}
		logs = append(logs, log)
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
