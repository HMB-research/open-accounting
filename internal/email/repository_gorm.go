package email

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// tenantSettings is a model for reading tenant settings from public schema
type tenantSettings struct {
	ID       string `gorm:"column:id;primaryKey"`
	Settings []byte `gorm:"column:settings;type:jsonb"`
}

func (tenantSettings) TableName() string {
	return "tenants"
}

type emailLogRecord struct {
	ID             string `gorm:"column:id;primaryKey"`
	TenantID       string `gorm:"column:tenant_id"`
	EmailType      string `gorm:"column:email_type"`
	RecipientEmail string `gorm:"column:recipient_email"`
	RecipientName  string `gorm:"column:recipient_name"`
	Subject        string `gorm:"column:subject"`
	Status         EmailStatus
	SentAt         *time.Time `gorm:"column:sent_at"`
	ErrorMessage   string     `gorm:"column:error_message"`
	RelatedID      *string    `gorm:"column:related_id"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

// GetTenantSettings retrieves tenant settings JSON from public schema
func (r *GORMRepository) GetTenantSettings(ctx context.Context, tenantID string) ([]byte, error) {
	db, err := database.TenantTable(r.db.WithContext(ctx), "public", "tenants")
	if err != nil {
		return nil, err
	}

	var tenant tenantSettings
	err = db.
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
	db, err := database.TenantTable(r.db.WithContext(ctx), "public", "tenants")
	if err != nil {
		return err
	}

	return db.
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"settings":   settingsJSON,
			"updated_at": time.Now(),
		}).Error
}

// GetTemplate retrieves an email template
func (r *GORMRepository) GetTemplate(ctx context.Context, schemaName, tenantID string, templateType TemplateType) (*EmailTemplate, error) {
	db, err := r.tenantTable(ctx, schemaName, "email_templates")
	if err != nil {
		return nil, err
	}

	var tmpl EmailTemplate
	err = db.Where("tenant_id = ? AND template_type = ?", tenantID, templateType).
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
	db, err := r.tenantTable(ctx, schemaName, "email_templates")
	if err != nil {
		return nil, err
	}

	var templates []EmailTemplate
	err = db.Where("tenant_id = ?", tenantID).
		Order("template_type").
		Find(&templates).Error
	if err != nil {
		return nil, err
	}

	return templates, nil
}

// UpsertTemplate inserts or updates an email template
func (r *GORMRepository) UpsertTemplate(ctx context.Context, schemaName string, template *EmailTemplate) error {
	db, err := r.tenantTable(ctx, schemaName, "email_templates")
	if err != nil {
		return err
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "template_type"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"subject":    gorm.Expr("EXCLUDED.subject"),
			"body_html":  gorm.Expr("EXCLUDED.body_html"),
			"body_text":  gorm.Expr("EXCLUDED.body_text"),
			"is_active":  gorm.Expr("EXCLUDED.is_active"),
			"updated_at": time.Now(),
		}),
	}).Create(template).Error; err != nil {
		return err
	}

	var persisted EmailTemplate
	if err := db.Where("tenant_id = ? AND template_type = ?", template.TenantID, template.TemplateType).
		First(&persisted).Error; err != nil {
		return err
	}
	*template = persisted
	return nil
}

// CreateEmailLog creates a new email log entry
func (r *GORMRepository) CreateEmailLog(ctx context.Context, schemaName string, log *EmailLog) error {
	db, err := r.tenantTable(ctx, schemaName, "email_log")
	if err != nil {
		return err
	}
	return db.Create(emailLogToRecord(log)).Error
}

// UpdateEmailLogStatus updates email log status
func (r *GORMRepository) UpdateEmailLogStatus(ctx context.Context, schemaName, logID string, status EmailStatus, sentAt *time.Time, errorMessage string) error {
	db, err := r.tenantTable(ctx, schemaName, "email_log")
	if err != nil {
		return err
	}

	return db.Where("id = ?", logID).
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

	db, err := r.tenantTable(ctx, schemaName, "email_log")
	if err != nil {
		return nil, err
	}

	var records []emailLogRecord
	err = db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	logs := make([]EmailLog, 0, len(records))
	for _, record := range records {
		logs = append(logs, record.toEmailLog())
	}
	return logs, nil
}

func emailLogToRecord(log *EmailLog) *emailLogRecord {
	var relatedID *string
	if strings.TrimSpace(log.RelatedID) != "" {
		relatedID = &log.RelatedID
	}

	return &emailLogRecord{
		ID:             log.ID,
		TenantID:       log.TenantID,
		EmailType:      log.EmailType,
		RecipientEmail: log.RecipientEmail,
		RecipientName:  log.RecipientName,
		Subject:        log.Subject,
		Status:         log.Status,
		SentAt:         log.SentAt,
		ErrorMessage:   log.ErrorMessage,
		RelatedID:      relatedID,
		CreatedAt:      log.CreatedAt,
	}
}

func (r emailLogRecord) toEmailLog() EmailLog {
	log := EmailLog{
		ID:             r.ID,
		TenantID:       r.TenantID,
		EmailType:      r.EmailType,
		RecipientEmail: r.RecipientEmail,
		RecipientName:  r.RecipientName,
		Subject:        r.Subject,
		Status:         r.Status,
		SentAt:         r.SentAt,
		ErrorMessage:   r.ErrorMessage,
		CreatedAt:      r.CreatedAt,
	}
	if r.RelatedID != nil {
		log.RelatedID = *r.RelatedID
	}
	return log
}
