package invoicing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

// ReminderRuleRepository defines the interface for reminder rule data access.
type ReminderRuleRepository interface {
	ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error)
	ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error)
	GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error)
	CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error
	UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error
	DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error
	GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error)
	HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error)
	RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error
}

// ReminderRuleGORMRepository implements ReminderRuleRepository with the shared ORM layer.
type ReminderRuleGORMRepository struct {
	db *gorm.DB
}

func NewReminderRuleRepository(db *pgxpool.Pool) *ReminderRuleGORMRepository {
	if db == nil {
		return &ReminderRuleGORMRepository{}
	}
	gormDB, err := newGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create reminder rule GORM repository: %w", err))
	}
	return NewReminderRuleGORMRepository(gormDB)
}

func NewReminderRuleGORMRepository(db *gorm.DB) *ReminderRuleGORMRepository {
	return &ReminderRuleGORMRepository{db: db}
}

func (r *ReminderRuleGORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("reminder rule repository database is not configured")
	}
	return r.db.WithContext(ctx), nil
}

func (r *ReminderRuleGORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return database.TenantTable(db, schemaName, tableName)
}

func (r *ReminderRuleGORMRepository) ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
	db, err := r.tenantTable(ctx, schemaName, "reminder_rules")
	if err != nil {
		return nil, fmt.Errorf("qualify reminder rules table: %w", err)
	}

	var ruleModels []models.ReminderRule
	if err := db.
		Where("tenant_id = ?", tenantID).
		Order("trigger_type ASC, days_offset ASC").
		Find(&ruleModels).Error; err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	return reminderRulesFromModels(ruleModels), nil
}

func (r *ReminderRuleGORMRepository) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
	db, err := r.tenantTable(ctx, schemaName, "reminder_rules")
	if err != nil {
		return nil, fmt.Errorf("qualify reminder rules table: %w", err)
	}

	var ruleModels []models.ReminderRule
	if err := db.
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("trigger_type ASC, days_offset ASC").
		Find(&ruleModels).Error; err != nil {
		return nil, fmt.Errorf("query active rules: %w", err)
	}
	return reminderRulesFromModels(ruleModels), nil
}

func (r *ReminderRuleGORMRepository) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error) {
	db, err := r.tenantTable(ctx, schemaName, "reminder_rules")
	if err != nil {
		return nil, fmt.Errorf("qualify reminder rules table: %w", err)
	}

	var ruleModel models.ReminderRule
	err = db.Where("tenant_id = ? AND id = ?", tenantID, ruleID).First(&ruleModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query rule: %w", err)
	}
	return reminderRuleFromModel(&ruleModel), nil
}

func (r *ReminderRuleGORMRepository) CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
	db, err := r.tenantTable(ctx, schemaName, "reminder_rules")
	if err != nil {
		return fmt.Errorf("qualify reminder rules table: %w", err)
	}

	if err := db.Create(reminderRuleToModel(rule)).Error; err != nil {
		return fmt.Errorf("insert rule: %w", err)
	}
	return nil
}

func (r *ReminderRuleGORMRepository) UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
	db, err := r.tenantTable(ctx, schemaName, "reminder_rules")
	if err != nil {
		return fmt.Errorf("qualify reminder rules table: %w", err)
	}

	result := db.Model(&models.ReminderRule{}).
		Where("id = ? AND tenant_id = ?", rule.ID, rule.TenantID).
		Updates(map[string]interface{}{
			"name":                rule.Name,
			"email_template_type": rule.EmailTemplateType,
			"is_active":           rule.IsActive,
			"updated_at":          time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRuleNotFound
	}
	return nil
}

func (r *ReminderRuleGORMRepository) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
	db, err := r.tenantTable(ctx, schemaName, "reminder_rules")
	if err != nil {
		return fmt.Errorf("qualify reminder rules table: %w", err)
	}

	result := db.Where("tenant_id = ? AND id = ?", tenantID, ruleID).Delete(&models.ReminderRule{})
	if result.Error != nil {
		return fmt.Errorf("delete rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRuleNotFound
	}
	return nil
}

func (r *ReminderRuleGORMRepository) GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error) {
	targetDate := asOfDate
	statuses := []string{"SENT", "PARTIALLY_PAID"}
	switch rule.TriggerType {
	case TriggerBeforeDue:
		targetDate = asOfDate.AddDate(0, 0, rule.DaysOffset)
	case TriggerOnDue:
		targetDate = asOfDate
	case TriggerAfterDue:
		targetDate = asOfDate.AddDate(0, 0, -rule.DaysOffset)
		statuses = append(statuses, "OVERDUE")
	default:
		return nil, ErrInvalidTriggerType
	}

	invoicesTable, err := database.QualifiedTable(schemaName, "invoices")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}
	contactsTable, err := database.QualifiedTable(schemaName, "contacts")
	if err != nil {
		return nil, fmt.Errorf("qualify contacts table: %w", err)
	}
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("query invoices for rule: %w", err)
	}

	var rows []struct {
		ID                string
		InvoiceNumber     string
		ContactID         string
		ContactName       string
		ContactEmail      string
		IssueDate         string
		DueDate           string
		Total             string
		AmountPaid        string
		OutstandingAmount string
		Currency          string
	}
	if err := db.
		Table(invoicesTable+" AS i").
		Select(`
			i.id,
			i.invoice_number,
			i.contact_id,
			c.name AS contact_name,
			COALESCE(c.email, '') AS contact_email,
			i.issue_date::text AS issue_date,
			i.due_date::text AS due_date,
			i.total::text AS total,
			i.amount_paid::text AS amount_paid,
			(i.total - i.amount_paid)::text AS outstanding_amount,
			i.currency
		`).
		Joins("JOIN "+contactsTable+" AS c ON i.contact_id = c.id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.invoice_type = ?", "SALES").
		Where("i.status IN ?", statuses).
		Where("i.due_date::date = ?::date", targetDate.Format("2006-01-02")).
		Where("i.total > i.amount_paid").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query invoices for rule: %w", err)
	}

	invoices := make([]InvoiceForReminder, 0, len(rows))
	for _, row := range rows {
		dueDate, err := time.Parse("2006-01-02", row.DueDate)
		if err != nil {
			return nil, fmt.Errorf("parse due date: %w", err)
		}

		daysUntilDue := int(dueDate.Sub(asOfDate).Hours() / 24)
		inv := InvoiceForReminder{
			ID:                row.ID,
			InvoiceNumber:     row.InvoiceNumber,
			ContactID:         row.ContactID,
			ContactName:       row.ContactName,
			ContactEmail:      row.ContactEmail,
			IssueDate:         row.IssueDate,
			DueDate:           row.DueDate,
			Total:             row.Total,
			AmountPaid:        row.AmountPaid,
			OutstandingAmount: row.OutstandingAmount,
			Currency:          row.Currency,
			DaysUntilDue:      daysUntilDue,
		}
		if daysUntilDue < 0 {
			inv.DaysOverdue = -daysUntilDue
		}
		invoices = append(invoices, inv)
	}

	return invoices, nil
}

func (r *ReminderRuleGORMRepository) HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error) {
	db, err := r.tenantTable(ctx, schemaName, "payment_reminders")
	if err != nil {
		return false, fmt.Errorf("qualify payment reminders table: %w", err)
	}

	var count int64
	if err := db.
		Where("tenant_id = ? AND invoice_id = ? AND rule_id = ? AND status = ?", tenantID, invoiceID, ruleID, ReminderStatusSent).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check reminder sent: %w", err)
	}
	return count > 0, nil
}

func (r *ReminderRuleGORMRepository) RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	db, err := r.tenantTable(ctx, schemaName, "payment_reminders")
	if err != nil {
		return fmt.Errorf("qualify payment reminders table: %w", err)
	}

	if err := db.Create(paymentReminderToModel(reminder)).Error; err != nil {
		return fmt.Errorf("insert reminder: %w", err)
	}
	return nil
}

func reminderRuleToModel(rule *ReminderRule) *models.ReminderRule {
	return &models.ReminderRule{
		ID:                rule.ID,
		TenantID:          rule.TenantID,
		Name:              rule.Name,
		TriggerType:       string(rule.TriggerType),
		DaysOffset:        rule.DaysOffset,
		EmailTemplateType: rule.EmailTemplateType,
		IsActive:          rule.IsActive,
		CreatedAt:         rule.CreatedAt,
		UpdatedAt:         rule.UpdatedAt,
	}
}

func reminderRuleFromModel(rule *models.ReminderRule) *ReminderRule {
	return &ReminderRule{
		ID:                rule.ID,
		TenantID:          rule.TenantID,
		Name:              rule.Name,
		TriggerType:       TriggerType(rule.TriggerType),
		DaysOffset:        rule.DaysOffset,
		EmailTemplateType: rule.EmailTemplateType,
		IsActive:          rule.IsActive,
		CreatedAt:         rule.CreatedAt,
		UpdatedAt:         rule.UpdatedAt,
	}
}

func reminderRulesFromModels(ruleModels []models.ReminderRule) []ReminderRule {
	rules := make([]ReminderRule, len(ruleModels))
	for i := range ruleModels {
		rules[i] = *reminderRuleFromModel(&ruleModels[i])
	}
	return rules
}
