package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReminderRuleRepository defines the interface for reminder rule data access
type ReminderRuleRepository interface {
	// ListRules returns all reminder rules for a tenant
	ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error)

	// ListActiveRules returns only active rules
	ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error)

	// GetRule returns a single rule by ID
	GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error)

	// CreateRule creates a new reminder rule
	CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error

	// UpdateRule updates an existing rule
	UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error

	// DeleteRule deletes a rule
	DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error

	// GetInvoicesForRule returns invoices that match a rule's criteria
	GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error)

	// HasReminderBeenSent checks if a reminder was already sent for this invoice+rule combo
	HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error)

	// RecordReminderSent records that a reminder was sent
	RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error
}

// ReminderRulePostgresRepository implements ReminderRuleRepository
type ReminderRulePostgresRepository struct {
	db *pgxpool.Pool
}

// NewReminderRulePostgresRepository creates a new repository
func NewReminderRulePostgresRepository(db *pgxpool.Pool) *ReminderRulePostgresRepository {
	return &ReminderRulePostgresRepository{db: db}
}

func (r *ReminderRulePostgresRepository) ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at
		FROM %s.reminder_rules
		WHERE tenant_id = $1
		ORDER BY trigger_type, days_offset
	`, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	return scanRules(rows)
}

func (r *ReminderRulePostgresRepository) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at
		FROM %s.reminder_rules
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY trigger_type, days_offset
	`, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query active rules: %w", err)
	}
	defer rows.Close()

	return scanRules(rows)
}

func (r *ReminderRulePostgresRepository) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at
		FROM %s.reminder_rules
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var rule ReminderRule
	err := r.db.QueryRow(ctx, query, tenantID, ruleID).Scan(
		&rule.ID, &rule.TenantID, &rule.Name, &rule.TriggerType, &rule.DaysOffset,
		&rule.EmailTemplateType, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query rule: %w", err)
	}

	return &rule, nil
}

func (r *ReminderRulePostgresRepository) CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.reminder_rules (id, tenant_id, name, trigger_type, days_offset, email_template_type, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, schemaName)

	_, err := r.db.Exec(ctx, query,
		rule.ID, rule.TenantID, rule.Name, rule.TriggerType, rule.DaysOffset,
		rule.EmailTemplateType, rule.IsActive, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert rule: %w", err)
	}

	return nil
}

func (r *ReminderRulePostgresRepository) UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
	query := fmt.Sprintf(`
		UPDATE %s.reminder_rules
		SET name = $1, email_template_type = $2, is_active = $3, updated_at = $4
		WHERE id = $5 AND tenant_id = $6
	`, schemaName)

	result, err := r.db.Exec(ctx, query,
		rule.Name, rule.EmailTemplateType, rule.IsActive, time.Now(), rule.ID, rule.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrRuleNotFound
	}

	return nil
}

func (r *ReminderRulePostgresRepository) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
	query := fmt.Sprintf(`DELETE FROM %s.reminder_rules WHERE tenant_id = $1 AND id = $2`, schemaName)

	result, err := r.db.Exec(ctx, query, tenantID, ruleID)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrRuleNotFound
	}

	return nil
}

func (r *ReminderRulePostgresRepository) GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error) {
	var query string
	var targetDate time.Time

	switch rule.TriggerType {
	case TriggerBeforeDue:
		// Invoices due in exactly N days
		targetDate = asOfDate.AddDate(0, 0, rule.DaysOffset)
		query = fmt.Sprintf(`
			SELECT i.id, i.invoice_number, i.contact_id, c.name, COALESCE(c.email, ''),
			       i.issue_date::text, i.due_date::text, i.total::text, i.amount_paid::text,
			       (i.total - i.amount_paid)::text, i.currency,
			       (i.due_date::date - $2::date) as days_until_due
			FROM %s.invoices i
			JOIN %s.contacts c ON i.contact_id = c.id
			WHERE i.tenant_id = $1
			  AND i.invoice_type = 'SALES'
			  AND i.status IN ('SENT', 'PARTIALLY_PAID')
			  AND i.due_date::date = $2::date
			  AND i.total > i.amount_paid
		`, schemaName, schemaName)
	case TriggerOnDue:
		// Invoices due today
		targetDate = asOfDate
		query = fmt.Sprintf(`
			SELECT i.id, i.invoice_number, i.contact_id, c.name, COALESCE(c.email, ''),
			       i.issue_date::text, i.due_date::text, i.total::text, i.amount_paid::text,
			       (i.total - i.amount_paid)::text, i.currency,
			       0 as days_until_due
			FROM %s.invoices i
			JOIN %s.contacts c ON i.contact_id = c.id
			WHERE i.tenant_id = $1
			  AND i.invoice_type = 'SALES'
			  AND i.status IN ('SENT', 'PARTIALLY_PAID')
			  AND i.due_date::date = $2::date
			  AND i.total > i.amount_paid
		`, schemaName, schemaName)
	case TriggerAfterDue:
		// Invoices overdue by exactly N days
		targetDate = asOfDate.AddDate(0, 0, -rule.DaysOffset)
		query = fmt.Sprintf(`
			SELECT i.id, i.invoice_number, i.contact_id, c.name, COALESCE(c.email, ''),
			       i.issue_date::text, i.due_date::text, i.total::text, i.amount_paid::text,
			       (i.total - i.amount_paid)::text, i.currency,
			       ($2::date - i.due_date::date) as days_overdue
			FROM %s.invoices i
			JOIN %s.contacts c ON i.contact_id = c.id
			WHERE i.tenant_id = $1
			  AND i.invoice_type = 'SALES'
			  AND i.status IN ('SENT', 'PARTIALLY_PAID', 'OVERDUE')
			  AND i.due_date::date = $2::date
			  AND i.total > i.amount_paid
		`, schemaName, schemaName)
	}

	rows, err := r.db.Query(ctx, query, tenantID, targetDate)
	if err != nil {
		return nil, fmt.Errorf("query invoices for rule: %w", err)
	}
	defer rows.Close()

	var invoices []InvoiceForReminder
	for rows.Next() {
		var inv InvoiceForReminder
		var daysValue int
		err := rows.Scan(
			&inv.ID, &inv.InvoiceNumber, &inv.ContactID, &inv.ContactName, &inv.ContactEmail,
			&inv.IssueDate, &inv.DueDate, &inv.Total, &inv.AmountPaid,
			&inv.OutstandingAmount, &inv.Currency, &daysValue,
		)
		if err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}

		if rule.TriggerType == TriggerAfterDue {
			inv.DaysOverdue = daysValue
			inv.DaysUntilDue = -daysValue
		} else {
			inv.DaysUntilDue = daysValue
			inv.DaysOverdue = 0
		}

		invoices = append(invoices, inv)
	}

	return invoices, nil
}

func (r *ReminderRulePostgresRepository) HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.payment_reminders
		WHERE tenant_id = $1 AND invoice_id = $2 AND rule_id = $3 AND status = 'SENT'
	`, schemaName)

	var count int
	err := r.db.QueryRow(ctx, query, tenantID, invoiceID, ruleID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check reminder sent: %w", err)
	}

	return count > 0, nil
}

func (r *ReminderRulePostgresRepository) RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.payment_reminders
		(id, tenant_id, invoice_id, invoice_number, contact_id, contact_name, contact_email,
		 rule_id, trigger_type, days_offset, reminder_number, status, sent_at, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, schemaName)

	_, err := r.db.Exec(ctx, query,
		reminder.ID, reminder.TenantID, reminder.InvoiceID, reminder.InvoiceNumber,
		reminder.ContactID, reminder.ContactName, reminder.ContactEmail,
		reminder.RuleID, reminder.TriggerType, reminder.DaysOffset,
		reminder.ReminderNumber, reminder.Status, reminder.SentAt,
		reminder.ErrorMessage, reminder.CreatedAt, reminder.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert reminder: %w", err)
	}

	return nil
}

func scanRules(rows pgx.Rows) ([]ReminderRule, error) {
	var rules []ReminderRule
	for rows.Next() {
		var rule ReminderRule
		err := rows.Scan(
			&rule.ID, &rule.TenantID, &rule.Name, &rule.TriggerType, &rule.DaysOffset,
			&rule.EmailTemplateType, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
