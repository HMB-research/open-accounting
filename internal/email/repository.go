package email

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for email data access
type Repository interface {
	// Schema management
	EnsureSchema(ctx context.Context, schemaName string) error

	// SMTP Config operations
	GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error)
	UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error

	// Template operations
	GetTemplate(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error)
	ListTemplates(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error)
	UpsertTemplate(ctx context.Context, schemaName string, template *EmailTemplate) error

	// Email log operations
	CreateEmailLog(ctx context.Context, schemaName string, log *EmailLog) error
	UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error
	GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error)
}

// Common errors
var (
	ErrTemplateNotFound = fmt.Errorf("template not found")
	ErrSettingsNotFound = fmt.Errorf("settings not found")
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// EnsureSchema creates email tables if they don't exist
func (r *PostgresRepository) EnsureSchema(ctx context.Context, schemaName string) error {
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

	_, err := r.db.Exec(ctx, query)
	return err
}

// GetTenantSettings retrieves tenant settings JSON
func (r *PostgresRepository) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	var settingsJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT settings FROM tenants WHERE id = $1
	`, tenantID).Scan(&settingsJSON)
	if err == pgx.ErrNoRows {
		return nil, ErrSettingsNotFound
	}
	if err != nil {
		return nil, err
	}
	return settingsJSON, nil
}

// UpdateTenantSettings updates tenant settings
func (r *PostgresRepository) UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error {
	_, err := r.db.Exec(ctx, `
		UPDATE tenants SET settings = $2, updated_at = NOW() WHERE id = $1
	`, tenantID, settingsJSON)
	return err
}

// GetTemplate retrieves an email template
func (r *PostgresRepository) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
	var tmpl EmailTemplate
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, template_type, subject, body_html, COALESCE(body_text, ''), is_active, created_at, updated_at
		FROM %s.email_templates
		WHERE tenant_id = $1 AND template_type = $2
	`, schemaName), tenantID, templateType).Scan(
		&tmpl.ID, &tmpl.TenantID, &tmpl.TemplateType, &tmpl.Subject, &tmpl.BodyHTML, &tmpl.BodyText, &tmpl.IsActive, &tmpl.CreatedAt, &tmpl.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// ListTemplates lists all email templates for a tenant
func (r *PostgresRepository) ListTemplates(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, template_type, subject, body_html, COALESCE(body_text, ''), is_active, created_at, updated_at
		FROM %s.email_templates
		WHERE tenant_id = $1
		ORDER BY template_type
	`, schemaName), tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []EmailTemplate
	for rows.Next() {
		var tmpl EmailTemplate
		if err := rows.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.TemplateType, &tmpl.Subject, &tmpl.BodyHTML, &tmpl.BodyText, &tmpl.IsActive, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

// UpsertTemplate inserts or updates an email template
func (r *PostgresRepository) UpsertTemplate(ctx context.Context, schemaName string, template *EmailTemplate) error {
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.email_templates (id, tenant_id, template_type, subject, body_html, body_text, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, template_type) DO UPDATE SET
			subject = EXCLUDED.subject,
			body_html = EXCLUDED.body_html,
			body_text = EXCLUDED.body_text,
			is_active = EXCLUDED.is_active,
			updated_at = NOW()
		RETURNING id, tenant_id, template_type, subject, body_html, COALESCE(body_text, ''), is_active, created_at, updated_at
	`, schemaName), template.ID, template.TenantID, template.TemplateType, template.Subject, template.BodyHTML, template.BodyText, template.IsActive).Scan(
		&template.ID, &template.TenantID, &template.TemplateType, &template.Subject, &template.BodyHTML, &template.BodyText, &template.IsActive, &template.CreatedAt, &template.UpdatedAt,
	)
	return err
}

// CreateEmailLog creates a new email log entry
func (r *PostgresRepository) CreateEmailLog(ctx context.Context, schemaName string, log *EmailLog) error {
	var relatedID *string
	if log.RelatedID != "" {
		relatedID = &log.RelatedID
	}
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.email_log (id, tenant_id, email_type, recipient_email, recipient_name, subject, status, related_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, schemaName), log.ID, log.TenantID, log.EmailType, log.RecipientEmail, log.RecipientName, log.Subject, log.Status, relatedID)
	return err
}

// UpdateEmailLogStatus updates email log status
func (r *PostgresRepository) UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.email_log SET status = $2, sent_at = $3, error_message = $4 WHERE id = $1
	`, schemaName), logID, status, sentAt, errorMessage)
	return err
}

// GetEmailLog retrieves email logs for a tenant
func (r *PostgresRepository) GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, email_type, recipient_email, COALESCE(recipient_name, ''), subject, status, sent_at, COALESCE(error_message, ''), related_id, created_at
		FROM %s.email_log
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, schemaName), tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []EmailLog
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

// ParseSMTPConfig parses SMTP config from settings JSON
func ParseSMTPConfig(settingsJSON []byte) (*SMTPConfig, error) {
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

// MergeSMTPConfig merges SMTP config into existing settings JSON
func MergeSMTPConfig(existingJSON []byte, req *UpdateSMTPConfigRequest) ([]byte, error) {
	var settings map[string]interface{}
	if err := json.Unmarshal(existingJSON, &settings); err != nil {
		settings = make(map[string]interface{})
	}

	settings["smtp_host"] = req.Host
	settings["smtp_port"] = req.Port
	settings["smtp_username"] = req.Username
	if req.Password != "" {
		settings["smtp_password"] = req.Password
	}
	settings["smtp_from_email"] = req.FromEmail
	settings["smtp_from_name"] = req.FromName
	settings["smtp_use_tls"] = req.UseTLS

	return json.Marshal(settings)
}
