package quotes

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for quote data access
type Repository interface {
	Create(ctx context.Context, schemaName string, quote *Quote) error
	GetByID(ctx context.Context, schemaName, tenantID, quoteID string) (*Quote, error)
	List(ctx context.Context, schemaName, tenantID string, filter *QuoteFilter) ([]Quote, error)
	Update(ctx context.Context, schemaName string, quote *Quote) error
	UpdateStatus(ctx context.Context, schemaName, tenantID, quoteID string, status QuoteStatus) error
	Delete(ctx context.Context, schemaName, tenantID, quoteID string) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error)
	SetConvertedToOrder(ctx context.Context, schemaName, tenantID, quoteID, orderID string) error
	SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error
}

// ErrQuoteNotFound is returned when a quote is not found
var ErrQuoteNotFound = fmt.Errorf("quote not found")

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new quote with its lines
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, quote *Quote) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.quotes (
			id, tenant_id, quote_number, contact_id, quote_date, valid_until,
			status, currency, exchange_rate, subtotal, vat_amount, total,
			notes, created_at, created_by, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, schemaName),
		quote.ID, quote.TenantID, quote.QuoteNumber, quote.ContactID,
		quote.QuoteDate, quote.ValidUntil, quote.Status, quote.Currency,
		quote.ExchangeRate, quote.Subtotal, quote.VATAmount, quote.Total,
		quote.Notes, quote.CreatedAt, quote.CreatedBy, quote.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert quote: %w", err)
	}

	for i := range quote.Lines {
		line := &quote.Lines[i]
		line.QuoteID = quote.ID

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.quote_lines (
				id, tenant_id, quote_id, line_number, description, quantity, unit,
				unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
				product_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, schemaName),
			line.ID, line.TenantID, line.QuoteID, line.LineNumber, line.Description,
			line.Quantity, line.Unit, line.UnitPrice, line.DiscountPercent, line.VATRate,
			line.LineSubtotal, line.LineVAT, line.LineTotal, line.ProductID,
		)
		if err != nil {
			return fmt.Errorf("insert quote line: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a quote by ID with its lines
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, quoteID string) (*Quote, error) {
	var q Quote
	var validUntil *time.Time
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, quote_number, contact_id, quote_date, valid_until,
		       status, currency, exchange_rate, subtotal, vat_amount, total,
		       COALESCE(notes, ''), converted_to_order_id, converted_to_invoice_id,
		       created_at, created_by, updated_at
		FROM %s.quotes
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), quoteID, tenantID).Scan(
		&q.ID, &q.TenantID, &q.QuoteNumber, &q.ContactID, &q.QuoteDate, &validUntil,
		&q.Status, &q.Currency, &q.ExchangeRate, &q.Subtotal, &q.VATAmount, &q.Total,
		&q.Notes, &q.ConvertedToOrderID, &q.ConvertedToInvoiceID,
		&q.CreatedAt, &q.CreatedBy, &q.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrQuoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}
	q.ValidUntil = validUntil

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, quote_id, line_number, COALESCE(description, ''), quantity, COALESCE(unit, ''),
		       unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
		       product_id
		FROM %s.quote_lines
		WHERE quote_id = $1 AND tenant_id = $2
		ORDER BY line_number
	`, schemaName), quoteID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get quote lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line QuoteLine
		if err := rows.Scan(
			&line.ID, &line.TenantID, &line.QuoteID, &line.LineNumber, &line.Description,
			&line.Quantity, &line.Unit, &line.UnitPrice, &line.DiscountPercent, &line.VATRate,
			&line.LineSubtotal, &line.LineVAT, &line.LineTotal, &line.ProductID,
		); err != nil {
			return nil, fmt.Errorf("scan quote line: %w", err)
		}
		q.Lines = append(q.Lines, line)
	}

	return &q, nil
}

// List retrieves quotes with optional filtering
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter *QuoteFilter) ([]Quote, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, quote_number, contact_id, quote_date, valid_until,
		       status, currency, exchange_rate, subtotal, vat_amount, total,
		       COALESCE(notes, ''), converted_to_order_id, converted_to_invoice_id,
		       created_at, created_by, updated_at
		FROM %s.quotes
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
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
			query += fmt.Sprintf(" AND quote_date >= $%d", argNum)
			args = append(args, filter.FromDate)
			argNum++
		}
		if filter.ToDate != nil {
			query += fmt.Sprintf(" AND quote_date <= $%d", argNum)
			args = append(args, filter.ToDate)
			argNum++
		}
		if filter.Search != "" {
			query += fmt.Sprintf(" AND (quote_number ILIKE $%d)", argNum)
			args = append(args, "%"+filter.Search+"%")
		}
	}

	query += " ORDER BY quote_date DESC, quote_number DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list quotes: %w", err)
	}
	defer rows.Close()

	var quotes []Quote
	for rows.Next() {
		var q Quote
		var validUntil *time.Time
		if err := rows.Scan(
			&q.ID, &q.TenantID, &q.QuoteNumber, &q.ContactID, &q.QuoteDate, &validUntil,
			&q.Status, &q.Currency, &q.ExchangeRate, &q.Subtotal, &q.VATAmount, &q.Total,
			&q.Notes, &q.ConvertedToOrderID, &q.ConvertedToInvoiceID,
			&q.CreatedAt, &q.CreatedBy, &q.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quote: %w", err)
		}
		q.ValidUntil = validUntil
		quotes = append(quotes, q)
	}

	return quotes, nil
}

// Update updates a quote and its lines
func (r *PostgresRepository) Update(ctx context.Context, schemaName string, quote *Quote) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.quotes
		SET contact_id = $1, quote_date = $2, valid_until = $3, currency = $4,
		    exchange_rate = $5, subtotal = $6, vat_amount = $7, total = $8,
		    notes = $9, updated_at = $10
		WHERE id = $11 AND tenant_id = $12 AND status = 'DRAFT'
	`, schemaName),
		quote.ContactID, quote.QuoteDate, quote.ValidUntil, quote.Currency,
		quote.ExchangeRate, quote.Subtotal, quote.VATAmount, quote.Total,
		quote.Notes, time.Now(), quote.ID, quote.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update quote: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrQuoteNotFound
	}

	// Delete existing lines and insert new ones
	_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.quote_lines WHERE quote_id = $1`, schemaName), quote.ID)
	if err != nil {
		return fmt.Errorf("delete quote lines: %w", err)
	}

	for i := range quote.Lines {
		line := &quote.Lines[i]
		line.QuoteID = quote.ID

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.quote_lines (
				id, tenant_id, quote_id, line_number, description, quantity, unit,
				unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
				product_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, schemaName),
			line.ID, line.TenantID, line.QuoteID, line.LineNumber, line.Description,
			line.Quantity, line.Unit, line.UnitPrice, line.DiscountPercent, line.VATRate,
			line.LineSubtotal, line.LineVAT, line.LineTotal, line.ProductID,
		)
		if err != nil {
			return fmt.Errorf("insert quote line: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateStatus updates the status of a quote
func (r *PostgresRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, quoteID string, status QuoteStatus) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.quotes
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), status, time.Now(), quoteID, tenantID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

// Delete removes a quote (only drafts)
func (r *PostgresRepository) Delete(ctx context.Context, schemaName, tenantID, quoteID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.quotes
		WHERE id = $1 AND tenant_id = $2 AND status = 'DRAFT'
	`, schemaName), quoteID, tenantID)
	if err != nil {
		return fmt.Errorf("delete quote: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

// GenerateNumber generates a new quote number
func (r *PostgresRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	var seq int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(quote_number FROM 'Q-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.quotes WHERE tenant_id = $1
	`, schemaName), tenantID).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate quote number: %w", err)
	}

	return fmt.Sprintf("Q-%05d", seq), nil
}

// SetConvertedToOrder marks a quote as converted to an order
func (r *PostgresRepository) SetConvertedToOrder(ctx context.Context, schemaName, tenantID, quoteID, orderID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.quotes
		SET status = $1, converted_to_order_id = $2, updated_at = $3
		WHERE id = $4 AND tenant_id = $5
	`, schemaName), QuoteStatusConverted, orderID, time.Now(), quoteID, tenantID)
	if err != nil {
		return fmt.Errorf("set converted to order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

// SetConvertedToInvoice marks a quote as converted to an invoice
func (r *PostgresRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.quotes
		SET status = $1, converted_to_invoice_id = $2, updated_at = $3
		WHERE id = $4 AND tenant_id = $5
	`, schemaName), QuoteStatusConverted, invoiceID, time.Now(), quoteID, tenantID)
	if err != nil {
		return fmt.Errorf("set converted to invoice: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrQuoteNotFound
	}
	return nil
}
