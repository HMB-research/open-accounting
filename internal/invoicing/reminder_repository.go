package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ReminderGORMRepository implements ReminderRepository with the shared ORM layer.
type ReminderGORMRepository struct {
	db *gorm.DB
}

func NewReminderRepository(db *pgxpool.Pool) *ReminderGORMRepository {
	if db == nil {
		return &ReminderGORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create reminder GORM repository: %w", err))
	}
	return NewReminderGORMRepository(gormDB)
}

func NewReminderGORMRepository(db *gorm.DB) *ReminderGORMRepository {
	return &ReminderGORMRepository{db: db}
}

func (r *ReminderGORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

// GetOverdueInvoices retrieves all overdue sales invoices.
func (r *ReminderGORMRepository) GetOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]OverdueInvoice, error) {
	invoicesTable, err := database.QualifiedTable(schemaName, "invoices")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}
	contactsTable, err := database.QualifiedTable(schemaName, "contacts")
	if err != nil {
		return nil, fmt.Errorf("qualify contacts table: %w", err)
	}

	var rows []struct {
		ID                string
		InvoiceNumber     string
		ContactID         string
		ContactName       string
		ContactEmail      string
		IssueDate         time.Time
		DueDate           time.Time
		Total             decimal.Decimal
		AmountPaid        decimal.Decimal
		OutstandingAmount decimal.Decimal
		Currency          string
		DaysOverdue       int
	}
	if err := r.db.WithContext(ctx).
		Table(invoicesTable+" AS i").
		Select(`
			i.id,
			i.invoice_number,
			i.contact_id,
			c.name AS contact_name,
			COALESCE(c.email, '') AS contact_email,
			i.issue_date,
			i.due_date,
			i.total,
			i.amount_paid,
			(i.total - i.amount_paid) AS outstanding_amount,
			i.currency,
			GREATEST(0, (?::date - i.due_date)::int) AS days_overdue
		`, asOfDate).
		Joins("JOIN "+contactsTable+" AS c ON i.contact_id = c.id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.invoice_type = ?", "SALES").
		Where("i.status NOT IN ?", []string{"PAID", "VOIDED"}).
		Where("i.due_date < ?", asOfDate).
		Where("(i.total - i.amount_paid) > ?", 0).
		Order("days_overdue DESC, i.total DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query overdue invoices: %w", err)
	}

	invoices := make([]OverdueInvoice, 0, len(rows))
	for _, row := range rows {
		invoices = append(invoices, OverdueInvoice{
			ID:                row.ID,
			InvoiceNumber:     row.InvoiceNumber,
			ContactID:         row.ContactID,
			ContactName:       row.ContactName,
			ContactEmail:      row.ContactEmail,
			IssueDate:         row.IssueDate.Format("2006-01-02"),
			DueDate:           row.DueDate.Format("2006-01-02"),
			Total:             row.Total,
			AmountPaid:        row.AmountPaid,
			OutstandingAmount: row.OutstandingAmount,
			Currency:          row.Currency,
			DaysOverdue:       row.DaysOverdue,
		})
	}

	return invoices, nil
}

// GetReminderCount gets the number of reminders sent for an invoice.
func (r *ReminderGORMRepository) GetReminderCount(ctx context.Context, schemaName, tenantID, invoiceID string) (int, *time.Time, error) {
	db, err := r.tenantTable(ctx, schemaName, "payment_reminders")
	if err != nil {
		return 0, nil, fmt.Errorf("qualify payment reminders table: %w", err)
	}

	var row struct {
		Count      int
		LastSentAt *time.Time
	}
	if err := db.
		Select("COUNT(*)::int AS count, MAX(sent_at) AS last_sent_at").
		Where("tenant_id = ? AND invoice_id = ? AND status = ?", tenantID, invoiceID, ReminderStatusSent).
		Scan(&row).Error; err != nil {
		return 0, nil, fmt.Errorf("query reminder count: %w", err)
	}

	return row.Count, row.LastSentAt, nil
}

// CreateReminder creates a new payment reminder record.
func (r *ReminderGORMRepository) CreateReminder(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	db, err := r.tenantTable(ctx, schemaName, "payment_reminders")
	if err != nil {
		return fmt.Errorf("qualify payment reminders table: %w", err)
	}

	if err := db.Create(paymentReminderToModel(reminder)).Error; err != nil {
		return fmt.Errorf("insert reminder: %w", err)
	}
	return nil
}

// UpdateReminderStatus updates the status of a reminder.
func (r *ReminderGORMRepository) UpdateReminderStatus(ctx context.Context, schemaName, reminderID string, status ReminderStatus, sentAt *time.Time, errorMsg string) error {
	db, err := r.tenantTable(ctx, schemaName, "payment_reminders")
	if err != nil {
		return fmt.Errorf("qualify payment reminders table: %w", err)
	}

	if err := db.Model(&models.PaymentReminder{}).
		Where("id = ?", reminderID).
		Updates(map[string]interface{}{
			"status":        string(status),
			"sent_at":       sentAt,
			"error_message": nilIfEmpty(errorMsg),
			"updated_at":    time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("update reminder status: %w", err)
	}

	return nil
}

// GetRemindersByInvoice gets all reminders for an invoice.
func (r *ReminderGORMRepository) GetRemindersByInvoice(ctx context.Context, schemaName, tenantID, invoiceID string) ([]PaymentReminder, error) {
	db, err := r.tenantTable(ctx, schemaName, "payment_reminders")
	if err != nil {
		return nil, fmt.Errorf("qualify payment reminders table: %w", err)
	}

	var reminderModels []models.PaymentReminder
	if err := db.
		Where("tenant_id = ? AND invoice_id = ?", tenantID, invoiceID).
		Order("created_at DESC").
		Find(&reminderModels).Error; err != nil {
		return nil, fmt.Errorf("query reminders: %w", err)
	}

	reminders := make([]PaymentReminder, len(reminderModels))
	for i := range reminderModels {
		reminders[i] = *paymentReminderFromModel(&reminderModels[i])
	}
	return reminders, nil
}

func paymentReminderToModel(reminder *PaymentReminder) *models.PaymentReminder {
	return &models.PaymentReminder{
		ID:             reminder.ID,
		TenantID:       reminder.TenantID,
		InvoiceID:      reminder.InvoiceID,
		InvoiceNumber:  reminder.InvoiceNumber,
		ContactID:      reminder.ContactID,
		ContactName:    reminder.ContactName,
		ContactEmail:   nilIfEmpty(reminder.ContactEmail),
		RuleID:         reminder.RuleID,
		TriggerType:    reminder.TriggerType,
		DaysOffset:     reminder.DaysOffset,
		ReminderNumber: reminder.ReminderNumber,
		Status:         string(reminder.Status),
		SentAt:         reminder.SentAt,
		ErrorMessage:   nilIfEmpty(reminder.ErrorMessage),
		CreatedAt:      reminder.CreatedAt,
		UpdatedAt:      reminder.UpdatedAt,
	}
}

func paymentReminderFromModel(reminder *models.PaymentReminder) *PaymentReminder {
	return &PaymentReminder{
		ID:             reminder.ID,
		TenantID:       reminder.TenantID,
		InvoiceID:      reminder.InvoiceID,
		InvoiceNumber:  reminder.InvoiceNumber,
		ContactID:      reminder.ContactID,
		ContactName:    reminder.ContactName,
		ContactEmail:   valueOrEmpty(reminder.ContactEmail),
		RuleID:         reminder.RuleID,
		TriggerType:    reminder.TriggerType,
		DaysOffset:     reminder.DaysOffset,
		ReminderNumber: reminder.ReminderNumber,
		Status:         ReminderStatus(reminder.Status),
		SentAt:         reminder.SentAt,
		ErrorMessage:   valueOrEmpty(reminder.ErrorMessage),
		CreatedAt:      reminder.CreatedAt,
		UpdatedAt:      reminder.UpdatedAt,
	}
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// MockReminderRepository for testing.
type MockReminderRepository struct {
	OverdueInvoices []OverdueInvoice
	Reminders       map[string][]PaymentReminder
	GetOverdueErr   error
}

// NewMockReminderRepository creates a new mock reminder repository.
func NewMockReminderRepository() *MockReminderRepository {
	return &MockReminderRepository{
		OverdueInvoices: make([]OverdueInvoice, 0),
		Reminders:       make(map[string][]PaymentReminder),
	}
}

// GetOverdueInvoices returns mock overdue invoices.
func (m *MockReminderRepository) GetOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]OverdueInvoice, error) {
	if m.GetOverdueErr != nil {
		return nil, m.GetOverdueErr
	}
	return m.OverdueInvoices, nil
}

// GetReminderCount returns mock reminder count.
func (m *MockReminderRepository) GetReminderCount(ctx context.Context, schemaName, tenantID, invoiceID string) (int, *time.Time, error) {
	reminders := m.Reminders[invoiceID]
	count := 0
	var lastSent *time.Time
	for _, r := range reminders {
		if r.Status == ReminderStatusSent {
			count++
			if r.SentAt != nil && (lastSent == nil || r.SentAt.After(*lastSent)) {
				lastSent = r.SentAt
			}
		}
	}
	return count, lastSent, nil
}

// CreateReminder creates a mock reminder.
func (m *MockReminderRepository) CreateReminder(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	m.Reminders[reminder.InvoiceID] = append(m.Reminders[reminder.InvoiceID], *reminder)
	return nil
}

// UpdateReminderStatus updates mock reminder status.
func (m *MockReminderRepository) UpdateReminderStatus(ctx context.Context, schemaName, reminderID string, status ReminderStatus, sentAt *time.Time, errorMsg string) error {
	for invoiceID, reminders := range m.Reminders {
		for i, r := range reminders {
			if r.ID == reminderID {
				m.Reminders[invoiceID][i].Status = status
				m.Reminders[invoiceID][i].SentAt = sentAt
				m.Reminders[invoiceID][i].ErrorMessage = errorMsg
				return nil
			}
		}
	}
	return nil
}

// GetRemindersByInvoice returns mock reminders for an invoice.
func (m *MockReminderRepository) GetRemindersByInvoice(ctx context.Context, schemaName, tenantID, invoiceID string) ([]PaymentReminder, error) {
	return m.Reminders[invoiceID], nil
}

// AddMockOverdueInvoice adds a mock overdue invoice for testing.
func (m *MockReminderRepository) AddMockOverdueInvoice(id, invoiceNumber, contactID, contactName, contactEmail, currency string, total, amountPaid decimal.Decimal, daysOverdue int) {
	m.OverdueInvoices = append(m.OverdueInvoices, OverdueInvoice{
		ID:                id,
		InvoiceNumber:     invoiceNumber,
		ContactID:         contactID,
		ContactName:       contactName,
		ContactEmail:      contactEmail,
		IssueDate:         time.Now().AddDate(0, 0, -daysOverdue-14).Format("2006-01-02"),
		DueDate:           time.Now().AddDate(0, 0, -daysOverdue).Format("2006-01-02"),
		Total:             total,
		AmountPaid:        amountPaid,
		OutstandingAmount: total.Sub(amountPaid),
		Currency:          currency,
		DaysOverdue:       daysOverdue,
	})
}
