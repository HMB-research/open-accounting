package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ReminderPostgresRepository implements ReminderRepository for PostgreSQL
type ReminderPostgresRepository struct {
	db *pgxpool.Pool
}

// NewReminderPostgresRepository creates a new PostgreSQL reminder repository
func NewReminderPostgresRepository(db *pgxpool.Pool) *ReminderPostgresRepository {
	return &ReminderPostgresRepository{db: db}
}

// GetOverdueInvoices retrieves all overdue sales invoices
func (r *ReminderPostgresRepository) GetOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]OverdueInvoice, error) {
	query := fmt.Sprintf(`
		SELECT
			i.id,
			i.invoice_number,
			i.contact_id,
			c.name as contact_name,
			COALESCE(c.email, '') as contact_email,
			i.issue_date,
			i.due_date,
			i.total,
			i.amount_paid,
			(i.total - i.amount_paid) as outstanding_amount,
			i.currency,
			GREATEST(0, ($2::date - i.due_date)::int) as days_overdue
		FROM %s.invoices i
		JOIN %s.contacts c ON i.contact_id = c.id
		WHERE i.tenant_id = $1
			AND i.invoice_type = 'SALES'
			AND i.status NOT IN ('PAID', 'VOIDED')
			AND i.due_date < $2
			AND (i.total - i.amount_paid) > 0
		ORDER BY days_overdue DESC, i.total DESC
	`, schemaName, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("query overdue invoices: %w", err)
	}
	defer rows.Close()

	invoices := []OverdueInvoice{}
	for rows.Next() {
		var inv OverdueInvoice
		var issueDate, dueDate time.Time

		if err := rows.Scan(
			&inv.ID,
			&inv.InvoiceNumber,
			&inv.ContactID,
			&inv.ContactName,
			&inv.ContactEmail,
			&issueDate,
			&dueDate,
			&inv.Total,
			&inv.AmountPaid,
			&inv.OutstandingAmount,
			&inv.Currency,
			&inv.DaysOverdue,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		inv.IssueDate = issueDate.Format("2006-01-02")
		inv.DueDate = dueDate.Format("2006-01-02")
		invoices = append(invoices, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return invoices, nil
}

// GetReminderCount gets the number of reminders sent for an invoice
func (r *ReminderPostgresRepository) GetReminderCount(ctx context.Context, schemaName, tenantID, invoiceID string) (int, *time.Time, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*), MAX(sent_at)
		FROM %s.payment_reminders
		WHERE tenant_id = $1 AND invoice_id = $2 AND status = 'SENT'
	`, schemaName)

	var count int
	var lastSentAt *time.Time

	err := r.db.QueryRow(ctx, query, tenantID, invoiceID).Scan(&count, &lastSentAt)
	if err != nil {
		// Table might not exist yet
		return 0, nil, nil
	}

	return count, lastSentAt, nil
}

// CreateReminder creates a new payment reminder record
func (r *ReminderPostgresRepository) CreateReminder(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	// Ensure table exists
	if err := r.ensureReminderTable(ctx, schemaName); err != nil {
		return err
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.payment_reminders (
			id, tenant_id, invoice_id, invoice_number, contact_id, contact_name,
			contact_email, reminder_number, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, schemaName)

	_, err := r.db.Exec(ctx, query,
		reminder.ID,
		reminder.TenantID,
		reminder.InvoiceID,
		reminder.InvoiceNumber,
		reminder.ContactID,
		reminder.ContactName,
		reminder.ContactEmail,
		reminder.ReminderNumber,
		reminder.Status,
		reminder.CreatedAt,
		reminder.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert reminder: %w", err)
	}

	return nil
}

// UpdateReminderStatus updates the status of a reminder
func (r *ReminderPostgresRepository) UpdateReminderStatus(ctx context.Context, schemaName, reminderID string, status ReminderStatus, sentAt *time.Time, errorMsg string) error {
	query := fmt.Sprintf(`
		UPDATE %s.payment_reminders
		SET status = $1, sent_at = $2, error_message = $3, updated_at = $4
		WHERE id = $5
	`, schemaName)

	_, err := r.db.Exec(ctx, query, status, sentAt, errorMsg, time.Now(), reminderID)
	if err != nil {
		return fmt.Errorf("update reminder status: %w", err)
	}

	return nil
}

// GetRemindersByInvoice gets all reminders for an invoice
func (r *ReminderPostgresRepository) GetRemindersByInvoice(ctx context.Context, schemaName, tenantID, invoiceID string) ([]PaymentReminder, error) {
	// Ensure table exists
	if err := r.ensureReminderTable(ctx, schemaName); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, invoice_id, invoice_number, contact_id, contact_name,
			   contact_email, reminder_number, status, sent_at, error_message,
			   created_at, updated_at
		FROM %s.payment_reminders
		WHERE tenant_id = $1 AND invoice_id = $2
		ORDER BY created_at DESC
	`, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("query reminders: %w", err)
	}
	defer rows.Close()

	reminders := []PaymentReminder{}
	for rows.Next() {
		var rem PaymentReminder
		if err := rows.Scan(
			&rem.ID,
			&rem.TenantID,
			&rem.InvoiceID,
			&rem.InvoiceNumber,
			&rem.ContactID,
			&rem.ContactName,
			&rem.ContactEmail,
			&rem.ReminderNumber,
			&rem.Status,
			&rem.SentAt,
			&rem.ErrorMessage,
			&rem.CreatedAt,
			&rem.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		reminders = append(reminders, rem)
	}

	return reminders, nil
}

// ensureReminderTable creates the payment_reminders table if it doesn't exist
func (r *ReminderPostgresRepository) ensureReminderTable(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.payment_reminders (
			id UUID PRIMARY KEY,
			tenant_id UUID NOT NULL REFERENCES %s.tenants(id),
			invoice_id UUID NOT NULL,
			invoice_number VARCHAR(50) NOT NULL,
			contact_id UUID NOT NULL,
			contact_name VARCHAR(255) NOT NULL,
			contact_email VARCHAR(255),
			reminder_number INTEGER NOT NULL DEFAULT 1,
			status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
			sent_at TIMESTAMP WITH TIME ZONE,
			error_message TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_payment_reminders_tenant_id
			ON %s.payment_reminders(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_payment_reminders_invoice_id
			ON %s.payment_reminders(invoice_id);
	`, schemaName, schemaName, schemaName, schemaName)

	_, err := r.db.Exec(ctx, query)
	return err
}

// MockReminderRepository for testing
type MockReminderRepository struct {
	OverdueInvoices []OverdueInvoice
	Reminders       map[string][]PaymentReminder
	GetOverdueErr   error
}

// NewMockReminderRepository creates a new mock reminder repository
func NewMockReminderRepository() *MockReminderRepository {
	return &MockReminderRepository{
		OverdueInvoices: make([]OverdueInvoice, 0),
		Reminders:       make(map[string][]PaymentReminder),
	}
}

// GetOverdueInvoices returns mock overdue invoices
func (m *MockReminderRepository) GetOverdueInvoices(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]OverdueInvoice, error) {
	if m.GetOverdueErr != nil {
		return nil, m.GetOverdueErr
	}
	return m.OverdueInvoices, nil
}

// GetReminderCount returns mock reminder count
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

// CreateReminder creates a mock reminder
func (m *MockReminderRepository) CreateReminder(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	m.Reminders[reminder.InvoiceID] = append(m.Reminders[reminder.InvoiceID], *reminder)
	return nil
}

// UpdateReminderStatus updates mock reminder status
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

// GetRemindersByInvoice returns mock reminders for an invoice
func (m *MockReminderRepository) GetRemindersByInvoice(ctx context.Context, schemaName, tenantID, invoiceID string) ([]PaymentReminder, error) {
	return m.Reminders[invoiceID], nil
}

// AddMockOverdueInvoice adds a mock overdue invoice for testing
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
