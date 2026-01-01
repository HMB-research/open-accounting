//go:build gorm

package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"gorm.io/gorm"
)

// tenantSettings is a model for reading tenant settings from public schema
type tenantSettings struct {
	ID       string `gorm:"column:id;primaryKey"`
	Settings []byte `gorm:"column:settings;type:jsonb"`
}

func (tenantSettings) TableName() string {
	return "tenants"
}

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// EnsureSchema creates email tables if they don't exist
// Note: This uses raw SQL as GORM AutoMigrate is not suitable for dynamic schema names
func (r *GORMRepository) EnsureSchema(ctx context.Context, schemaName string) error {
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

	return r.db.WithContext(ctx).Exec(query).Error
}

// GetTenantSettings retrieves tenant settings JSON from public schema
func (r *GORMRepository) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	var tenant tenantSettings
	err := r.db.WithContext(ctx).
		Select("id", "settings").
		Where("id = ?", tenantID).
		First(&tenant).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSettingsNotFound
	}
	if err != nil {
		return nil, err
	}

	return tenant.Settings, nil
}

// UpdateTenantSettings updates tenant settings in public schema
func (r *GORMRepository) UpdateTenantSettings(ctx context.Context, tenantID string, settingsJSON []byte) error {
	return r.db.WithContext(ctx).
		Model(&tenantSettings{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"settings":   settingsJSON,
			"updated_at": time.Now(),
		}).Error
}

// GetTemplate retrieves an email template
func (r *GORMRepository) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var tmpl EmailTemplate
	err := db.Where("tenant_id = ? AND template_type = ?", tenantID, templateType).
		First(&tmpl).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	return &tmpl, nil
}

// ListTemplates lists all email templates for a tenant
func (r *GORMRepository) ListTemplates(ctx context.Context, schemaName, tenantID string) ([]EmailTemplate, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var templates []EmailTemplate
	err := db.Where("tenant_id = ?", tenantID).
		Order("template_type").
		Find(&templates).Error
	if err != nil {
		return nil, err
	}

	return templates, nil
}

// UpsertTemplate inserts or updates an email template
func (r *GORMRepository) UpsertTemplate(ctx context.Context, schemaName string, template *EmailTemplate) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	// Use raw SQL for ON CONFLICT upsert since GORM's Clauses approach can be tricky with composite keys
	err := db.Exec(`
		INSERT INTO email_templates (id, tenant_id, template_type, subject, body_html, body_text, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, template_type) DO UPDATE SET
			subject = EXCLUDED.subject,
			body_html = EXCLUDED.body_html,
			body_text = EXCLUDED.body_text,
			is_active = EXCLUDED.is_active,
			updated_at = NOW()
	`, template.ID, template.TenantID, template.TemplateType, template.Subject, template.BodyHTML, template.BodyText, template.IsActive).Error

	if err != nil {
		return err
	}

	// Fetch the updated/inserted template to populate all fields
	return db.Where("tenant_id = ? AND template_type = ?", template.TenantID, template.TemplateType).
		First(template).Error
}

// CreateEmailLog creates a new email log entry
func (r *GORMRepository) CreateEmailLog(ctx context.Context, schemaName string, log *EmailLog) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)
	return db.Create(log).Error
}

// UpdateEmailLogStatus updates email log status
func (r *GORMRepository) UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Model(&EmailLog{}).
		Where("id = ?", logID).
		Updates(map[string]interface{}{
			"status":        status,
			"sent_at":       sentAt,
			"error_message": errorMessage,
		}).Error
}

// GetEmailLog retrieves email logs for a tenant
func (r *GORMRepository) GetEmailLog(ctx context.Context, schemaName, tenantID string, limit int) ([]EmailLog, error) {
	if limit <= 0 {
		limit = 50
	}

	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var logs []EmailLog
	err := db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	return logs, nil
}
