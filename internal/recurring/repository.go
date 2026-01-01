package recurring

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for recurring invoice data access
type Repository interface {
	// Schema management
	EnsureSchema(ctx context.Context, schemaName string) error

	// Recurring Invoice CRUD
	Create(ctx context.Context, schemaName string, ri *RecurringInvoice) error
	CreateLine(ctx context.Context, schemaName string, line *RecurringInvoiceLine) error
	GetByID(ctx context.Context, schemaName, tenantID, id string) (*RecurringInvoice, error)
	GetLines(ctx context.Context, schemaName, recurringInvoiceID string) ([]RecurringInvoiceLine, error)
	List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]RecurringInvoice, error)
	Update(ctx context.Context, schemaName string, ri *RecurringInvoice) error
	DeleteLines(ctx context.Context, schemaName, recurringInvoiceID string) error
	Delete(ctx context.Context, schemaName, tenantID, id string) error

	// Status operations
	SetActive(ctx context.Context, schemaName, tenantID, id string, active bool) error

	// Generation tracking
	GetDueRecurringInvoiceIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error)
	UpdateAfterGeneration(ctx context.Context, schemaName, tenantID, id string, nextDate time.Time, generatedAt time.Time) error
	UpdateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, sentAt *time.Time, status, logID string) error
}

// Common errors
var (
	ErrRecurringInvoiceNotFound = fmt.Errorf("recurring invoice not found")
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// EnsureSchema creates the recurring invoice tables if they don't exist
func (r *PostgresRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.recurring_invoices (
			id UUID PRIMARY KEY,
			tenant_id UUID NOT NULL,
			name VARCHAR(100) NOT NULL,
			contact_id UUID NOT NULL,
			invoice_type VARCHAR(20) NOT NULL DEFAULT 'SALES',
			currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
			frequency VARCHAR(20) NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE,
			next_generation_date DATE NOT NULL,
			payment_terms_days INTEGER NOT NULL DEFAULT 14,
			reference TEXT,
			notes TEXT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			last_generated_at TIMESTAMPTZ,
			generated_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_by UUID NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			send_email_on_generation BOOLEAN DEFAULT false,
			email_template_type VARCHAR(50) DEFAULT 'INVOICE_SEND',
			recipient_email_override TEXT,
			attach_pdf_to_email BOOLEAN DEFAULT true,
			email_subject_override TEXT,
			email_message TEXT
		);

		CREATE TABLE IF NOT EXISTS %s.recurring_invoice_lines (
			id UUID PRIMARY KEY,
			recurring_invoice_id UUID NOT NULL REFERENCES %s.recurring_invoices(id) ON DELETE CASCADE,
			line_number INTEGER NOT NULL,
			description TEXT NOT NULL,
			quantity NUMERIC(18,6) NOT NULL DEFAULT 1,
			unit VARCHAR(20),
			unit_price NUMERIC(28,8) NOT NULL,
			discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
			vat_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
			account_id UUID,
			product_id UUID
		);

		CREATE INDEX IF NOT EXISTS idx_recurring_invoices_tenant ON %s.recurring_invoices(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_recurring_invoices_next_gen ON %s.recurring_invoices(next_generation_date) WHERE is_active = true;
		CREATE INDEX IF NOT EXISTS idx_recurring_invoice_lines_recurring ON %s.recurring_invoice_lines(recurring_invoice_id);
	`, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName))
	return err
}

// Create inserts a new recurring invoice
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, ri *RecurringInvoice) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.recurring_invoices (
			id, tenant_id, name, contact_id, invoice_type, currency, frequency,
			start_date, end_date, next_generation_date, payment_terms_days,
			reference, notes, is_active, generated_count, created_at, created_by, updated_at,
			send_email_on_generation, email_template_type, recipient_email_override,
			attach_pdf_to_email, email_subject_override, email_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`, schemaName),
		ri.ID, ri.TenantID, ri.Name, ri.ContactID, ri.InvoiceType, ri.Currency, ri.Frequency,
		ri.StartDate, ri.EndDate, ri.NextGenerationDate, ri.PaymentTermsDays,
		ri.Reference, ri.Notes, ri.IsActive, ri.GeneratedCount, ri.CreatedAt, ri.CreatedBy, ri.UpdatedAt,
		ri.SendEmailOnGeneration, ri.EmailTemplateType, ri.RecipientEmailOverride,
		ri.AttachPDFToEmail, ri.EmailSubjectOverride, ri.EmailMessage,
	)
	return err
}

// CreateLine inserts a recurring invoice line
func (r *PostgresRepository) CreateLine(ctx context.Context, schemaName string, line *RecurringInvoiceLine) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.recurring_invoice_lines (
			id, recurring_invoice_id, line_number, description, quantity, unit,
			unit_price, discount_percent, vat_rate, account_id, product_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, schemaName),
		line.ID, line.RecurringInvoiceID, line.LineNumber, line.Description, line.Quantity, line.Unit,
		line.UnitPrice, line.DiscountPercent, line.VATRate, line.AccountID, line.ProductID,
	)
	return err
}

// GetByID retrieves a recurring invoice by ID (without lines)
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, id string) (*RecurringInvoice, error) {
	var ri RecurringInvoice
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT r.id, r.tenant_id, r.name, r.contact_id, COALESCE(c.name, ''),
		       r.invoice_type, r.currency, r.frequency, r.start_date, r.end_date,
		       r.next_generation_date, r.payment_terms_days, r.reference, r.notes,
		       r.is_active, r.last_generated_at, r.generated_count, r.created_at, r.created_by, r.updated_at,
		       COALESCE(r.send_email_on_generation, false), COALESCE(r.email_template_type, 'INVOICE_SEND'),
		       COALESCE(r.recipient_email_override, ''), COALESCE(r.attach_pdf_to_email, true),
		       COALESCE(r.email_subject_override, ''), COALESCE(r.email_message, '')
		FROM %s.recurring_invoices r
		LEFT JOIN %s.contacts c ON r.contact_id = c.id
		WHERE r.id = $1 AND r.tenant_id = $2
	`, schemaName, schemaName), id, tenantID).Scan(
		&ri.ID, &ri.TenantID, &ri.Name, &ri.ContactID, &ri.ContactName,
		&ri.InvoiceType, &ri.Currency, &ri.Frequency, &ri.StartDate, &ri.EndDate,
		&ri.NextGenerationDate, &ri.PaymentTermsDays, &ri.Reference, &ri.Notes,
		&ri.IsActive, &ri.LastGeneratedAt, &ri.GeneratedCount, &ri.CreatedAt, &ri.CreatedBy, &ri.UpdatedAt,
		&ri.SendEmailOnGeneration, &ri.EmailTemplateType, &ri.RecipientEmailOverride,
		&ri.AttachPDFToEmail, &ri.EmailSubjectOverride, &ri.EmailMessage,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrRecurringInvoiceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ri, nil
}

// GetLines retrieves lines for a recurring invoice
func (r *PostgresRepository) GetLines(ctx context.Context, schemaName, recurringInvoiceID string) ([]RecurringInvoiceLine, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, recurring_invoice_id, line_number, description, quantity, unit,
		       unit_price, discount_percent, vat_rate, account_id, product_id
		FROM %s.recurring_invoice_lines
		WHERE recurring_invoice_id = $1
		ORDER BY line_number
	`, schemaName), recurringInvoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []RecurringInvoiceLine
	for rows.Next() {
		var line RecurringInvoiceLine
		if err := rows.Scan(
			&line.ID, &line.RecurringInvoiceID, &line.LineNumber, &line.Description,
			&line.Quantity, &line.Unit, &line.UnitPrice, &line.DiscountPercent,
			&line.VATRate, &line.AccountID, &line.ProductID,
		); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// List retrieves all recurring invoices for a tenant
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]RecurringInvoice, error) {
	query := fmt.Sprintf(`
		SELECT r.id, r.tenant_id, r.name, r.contact_id, COALESCE(c.name, ''),
		       r.invoice_type, r.currency, r.frequency, r.start_date, r.end_date,
		       r.next_generation_date, r.payment_terms_days, r.reference, r.notes,
		       r.is_active, r.last_generated_at, r.generated_count, r.created_at, r.created_by, r.updated_at,
		       COALESCE(r.send_email_on_generation, false), COALESCE(r.email_template_type, 'INVOICE_SEND'),
		       COALESCE(r.recipient_email_override, ''), COALESCE(r.attach_pdf_to_email, true),
		       COALESCE(r.email_subject_override, ''), COALESCE(r.email_message, '')
		FROM %s.recurring_invoices r
		LEFT JOIN %s.contacts c ON r.contact_id = c.id
		WHERE r.tenant_id = $1
	`, schemaName, schemaName)

	if activeOnly {
		query += " AND r.is_active = true"
	}
	query += " ORDER BY r.next_generation_date, r.name"

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RecurringInvoice
	for rows.Next() {
		var ri RecurringInvoice
		if err := rows.Scan(
			&ri.ID, &ri.TenantID, &ri.Name, &ri.ContactID, &ri.ContactName,
			&ri.InvoiceType, &ri.Currency, &ri.Frequency, &ri.StartDate, &ri.EndDate,
			&ri.NextGenerationDate, &ri.PaymentTermsDays, &ri.Reference, &ri.Notes,
			&ri.IsActive, &ri.LastGeneratedAt, &ri.GeneratedCount, &ri.CreatedAt, &ri.CreatedBy, &ri.UpdatedAt,
			&ri.SendEmailOnGeneration, &ri.EmailTemplateType, &ri.RecipientEmailOverride,
			&ri.AttachPDFToEmail, &ri.EmailSubjectOverride, &ri.EmailMessage,
		); err != nil {
			return nil, err
		}
		results = append(results, ri)
	}
	return results, nil
}

// Update updates a recurring invoice
func (r *PostgresRepository) Update(ctx context.Context, schemaName string, ri *RecurringInvoice) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.recurring_invoices SET
			name = $1, contact_id = $2, frequency = $3, end_date = $4,
			payment_terms_days = $5, reference = $6, notes = $7, updated_at = $8,
			send_email_on_generation = $9, email_template_type = $10, recipient_email_override = $11,
			attach_pdf_to_email = $12, email_subject_override = $13, email_message = $14
		WHERE id = $15 AND tenant_id = $16
	`, schemaName),
		ri.Name, ri.ContactID, ri.Frequency, ri.EndDate, ri.PaymentTermsDays,
		ri.Reference, ri.Notes, ri.UpdatedAt,
		ri.SendEmailOnGeneration, ri.EmailTemplateType, ri.RecipientEmailOverride,
		ri.AttachPDFToEmail, ri.EmailSubjectOverride, ri.EmailMessage,
		ri.ID, ri.TenantID,
	)
	return err
}

// DeleteLines deletes all lines for a recurring invoice
func (r *PostgresRepository) DeleteLines(ctx context.Context, schemaName, recurringInvoiceID string) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.recurring_invoice_lines WHERE recurring_invoice_id = $1
	`, schemaName), recurringInvoiceID)
	return err
}

// Delete deletes a recurring invoice
func (r *PostgresRepository) Delete(ctx context.Context, schemaName, tenantID, id string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.recurring_invoices WHERE id = $1 AND tenant_id = $2
	`, schemaName), id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRecurringInvoiceNotFound
	}
	return nil
}

// SetActive sets the active status of a recurring invoice
func (r *PostgresRepository) SetActive(ctx context.Context, schemaName, tenantID, id string, active bool) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.recurring_invoices SET is_active = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), active, time.Now(), id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRecurringInvoiceNotFound
	}
	return nil
}

// GetDueRecurringInvoiceIDs returns IDs of recurring invoices due for generation
func (r *PostgresRepository) GetDueRecurringInvoiceIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id FROM %s.recurring_invoices
		WHERE tenant_id = $1
		  AND is_active = true
		  AND next_generation_date <= $2
		  AND (end_date IS NULL OR end_date >= $2)
	`, schemaName), tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// UpdateAfterGeneration updates generation tracking fields
func (r *PostgresRepository) UpdateAfterGeneration(ctx context.Context, schemaName, tenantID, id string, nextDate time.Time, generatedAt time.Time) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.recurring_invoices SET
			next_generation_date = $1,
			last_generated_at = $2,
			generated_count = generated_count + 1,
			updated_at = $3
		WHERE id = $4 AND tenant_id = $5
	`, schemaName), nextDate, generatedAt, generatedAt, id, tenantID)
	return err
}

// UpdateInvoiceEmailStatus updates the invoice with email delivery status
func (r *PostgresRepository) UpdateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, sentAt *time.Time, status, logID string) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices SET
			last_email_sent_at = $1,
			last_email_status = $2,
			last_email_log_id = $3
		WHERE id = $4
	`, schemaName), sentAt, status, logID, invoiceID)
	return err
}
