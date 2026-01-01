package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Repository defines the contract for invoice data access
type Repository interface {
	Create(ctx context.Context, schemaName string, invoice *Invoice) error
	GetByID(ctx context.Context, schemaName, tenantID, invoiceID string) (*Invoice, error)
	List(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error)
	UpdateStatus(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error
	UpdatePayment(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error)
	UpdateOverdueStatus(ctx context.Context, schemaName, tenantID string) (int, error)
}

// ErrInvoiceNotFound is returned when an invoice is not found
var ErrInvoiceNotFound = fmt.Errorf("invoice not found")

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new invoice with its lines
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, invoice *Invoice) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.invoices (
			id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date,
			currency, exchange_rate, subtotal, vat_amount, total,
			base_subtotal, base_vat_amount, base_total, amount_paid, status,
			reference, notes, created_at, created_by, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`, schemaName),
		invoice.ID, invoice.TenantID, invoice.InvoiceNumber, invoice.InvoiceType,
		invoice.ContactID, invoice.IssueDate, invoice.DueDate,
		invoice.Currency, invoice.ExchangeRate, invoice.Subtotal, invoice.VATAmount, invoice.Total,
		invoice.BaseSubtotal, invoice.BaseVATAmount, invoice.BaseTotal, invoice.AmountPaid, invoice.Status,
		invoice.Reference, invoice.Notes, invoice.CreatedAt, invoice.CreatedBy, invoice.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}

	for i := range invoice.Lines {
		line := &invoice.Lines[i]
		line.InvoiceID = invoice.ID

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.invoice_lines (
				id, tenant_id, invoice_id, line_number, description, quantity, unit,
				unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
				account_id, product_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`, schemaName),
			line.ID, line.TenantID, line.InvoiceID, line.LineNumber, line.Description,
			line.Quantity, line.Unit, line.UnitPrice, line.DiscountPercent, line.VATRate,
			line.LineSubtotal, line.LineVAT, line.LineTotal, line.AccountID, line.ProductID,
		)
		if err != nil {
			return fmt.Errorf("insert invoice line: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves an invoice by ID with its lines
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, invoiceID string) (*Invoice, error) {
	var inv Invoice
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date,
		       currency, exchange_rate, subtotal, vat_amount, total,
		       base_subtotal, base_vat_amount, base_total, amount_paid, status,
		       reference, notes, journal_entry_id, einvoice_sent_at, einvoice_id,
		       created_at, created_by, updated_at
		FROM %s.invoices
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), invoiceID, tenantID).Scan(
		&inv.ID, &inv.TenantID, &inv.InvoiceNumber, &inv.InvoiceType, &inv.ContactID,
		&inv.IssueDate, &inv.DueDate, &inv.Currency, &inv.ExchangeRate,
		&inv.Subtotal, &inv.VATAmount, &inv.Total,
		&inv.BaseSubtotal, &inv.BaseVATAmount, &inv.BaseTotal, &inv.AmountPaid, &inv.Status,
		&inv.Reference, &inv.Notes, &inv.JournalEntryID, &inv.EInvoiceSentAt, &inv.EInvoiceID,
		&inv.CreatedAt, &inv.CreatedBy, &inv.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrInvoiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, invoice_id, line_number, description, quantity, unit,
		       unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
		       account_id, product_id
		FROM %s.invoice_lines
		WHERE invoice_id = $1 AND tenant_id = $2
		ORDER BY line_number
	`, schemaName), invoiceID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get invoice lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line InvoiceLine
		if err := rows.Scan(
			&line.ID, &line.TenantID, &line.InvoiceID, &line.LineNumber, &line.Description,
			&line.Quantity, &line.Unit, &line.UnitPrice, &line.DiscountPercent, &line.VATRate,
			&line.LineSubtotal, &line.LineVAT, &line.LineTotal, &line.AccountID, &line.ProductID,
		); err != nil {
			return nil, fmt.Errorf("scan invoice line: %w", err)
		}
		inv.Lines = append(inv.Lines, line)
	}

	return &inv, nil
}

// List retrieves invoices with optional filtering
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date,
		       currency, exchange_rate, subtotal, vat_amount, total,
		       base_subtotal, base_vat_amount, base_total, amount_paid, status,
		       reference, notes, journal_entry_id, einvoice_sent_at, einvoice_id,
		       created_at, created_by, updated_at
		FROM %s.invoices
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.InvoiceType != "" {
			query += fmt.Sprintf(" AND invoice_type = $%d", argNum)
			args = append(args, filter.InvoiceType)
			argNum++
		}
		if filter.Status != "" {
			query += fmt.Sprintf(" AND status = $%d", argNum)
			args = append(args, filter.Status)
			argNum++
		}
		if filter.ContactID != "" {
			query += fmt.Sprintf(" AND contact_id = $%d", argNum)
			args = append(args, filter.ContactID)
			argNum++
		}
		if filter.FromDate != nil {
			query += fmt.Sprintf(" AND issue_date >= $%d", argNum)
			args = append(args, filter.FromDate)
			argNum++
		}
		if filter.ToDate != nil {
			query += fmt.Sprintf(" AND issue_date <= $%d", argNum)
			args = append(args, filter.ToDate)
			argNum++
		}
		if filter.Search != "" {
			query += fmt.Sprintf(" AND (invoice_number ILIKE $%d OR reference ILIKE $%d)", argNum, argNum)
			args = append(args, "%"+filter.Search+"%")
		}
	}

	query += " ORDER BY issue_date DESC, invoice_number DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(
			&inv.ID, &inv.TenantID, &inv.InvoiceNumber, &inv.InvoiceType, &inv.ContactID,
			&inv.IssueDate, &inv.DueDate, &inv.Currency, &inv.ExchangeRate,
			&inv.Subtotal, &inv.VATAmount, &inv.Total,
			&inv.BaseSubtotal, &inv.BaseVATAmount, &inv.BaseTotal, &inv.AmountPaid, &inv.Status,
			&inv.Reference, &inv.Notes, &inv.JournalEntryID, &inv.EInvoiceSentAt, &inv.EInvoiceID,
			&inv.CreatedAt, &inv.CreatedBy, &inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}

	return invoices, nil
}

// UpdateStatus updates the status of an invoice
func (r *PostgresRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), status, time.Now(), invoiceID, tenantID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvoiceNotFound
	}
	return nil
}

// UpdatePayment updates the amount paid and status of an invoice
func (r *PostgresRepository) UpdatePayment(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices
		SET amount_paid = $1, status = $2, updated_at = $3
		WHERE id = $4 AND tenant_id = $5
	`, schemaName), amountPaid, status, time.Now(), invoiceID, tenantID)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvoiceNotFound
	}
	return nil
}

// GenerateNumber generates a new invoice number
func (r *PostgresRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error) {
	prefix := "INV"
	if invoiceType == InvoiceTypePurchase {
		prefix = "BILL"
	} else if invoiceType == InvoiceTypeCreditNote {
		prefix = "CN"
	}

	var seq int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(invoice_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.invoices WHERE tenant_id = $1 AND invoice_type = $2
	`, prefix, schemaName), tenantID, invoiceType).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate invoice number: %w", err)
	}

	return fmt.Sprintf("%s-%05d", prefix, seq), nil
}

// UpdateOverdueStatus updates the status of overdue invoices
func (r *PostgresRepository) UpdateOverdueStatus(ctx context.Context, schemaName, tenantID string) (int, error) {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices
		SET status = $1, updated_at = $2
		WHERE tenant_id = $3
		  AND status IN ($4, $5)
		  AND due_date < $6
		  AND amount_paid < total
	`, schemaName), StatusOverdue, time.Now(), tenantID, StatusSent, StatusPartiallyPaid, time.Now())
	if err != nil {
		return 0, fmt.Errorf("update overdue status: %w", err)
	}

	return int(result.RowsAffected()), nil
}
