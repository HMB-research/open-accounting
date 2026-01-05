package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for order data access
type Repository interface {
	Create(ctx context.Context, schemaName string, order *Order) error
	GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*Order, error)
	List(ctx context.Context, schemaName, tenantID string, filter *OrderFilter) ([]Order, error)
	Update(ctx context.Context, schemaName string, order *Order) error
	UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status OrderStatus) error
	Delete(ctx context.Context, schemaName, tenantID, orderID string) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error)
	SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error
}

// ErrOrderNotFound is returned when an order is not found
var ErrOrderNotFound = fmt.Errorf("order not found")

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new order with its lines
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, order *Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders (
			id, tenant_id, order_number, contact_id, order_date, expected_delivery,
			status, currency, exchange_rate, subtotal, vat_amount, total,
			notes, quote_id, created_at, created_by, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, schemaName),
		order.ID, order.TenantID, order.OrderNumber, order.ContactID,
		order.OrderDate, order.ExpectedDelivery, order.Status, order.Currency,
		order.ExchangeRate, order.Subtotal, order.VATAmount, order.Total,
		order.Notes, order.QuoteID, order.CreatedAt, order.CreatedBy, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	for i := range order.Lines {
		line := &order.Lines[i]
		line.OrderID = order.ID

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_lines (
				id, tenant_id, order_id, line_number, description, quantity, unit,
				unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
				product_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, schemaName),
			line.ID, line.TenantID, line.OrderID, line.LineNumber, line.Description,
			line.Quantity, line.Unit, line.UnitPrice, line.DiscountPercent, line.VATRate,
			line.LineSubtotal, line.LineVAT, line.LineTotal, line.ProductID,
		)
		if err != nil {
			return fmt.Errorf("insert order line: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves an order by ID with its lines
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, orderID string) (*Order, error) {
	var o Order
	var expectedDelivery *time.Time
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, order_number, contact_id, order_date, expected_delivery,
		       status, currency, exchange_rate, subtotal, vat_amount, total,
		       COALESCE(notes, ''), quote_id, converted_to_invoice_id,
		       created_at, created_by, updated_at
		FROM %s.orders
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), orderID, tenantID).Scan(
		&o.ID, &o.TenantID, &o.OrderNumber, &o.ContactID, &o.OrderDate, &expectedDelivery,
		&o.Status, &o.Currency, &o.ExchangeRate, &o.Subtotal, &o.VATAmount, &o.Total,
		&o.Notes, &o.QuoteID, &o.ConvertedToInvoiceID,
		&o.CreatedAt, &o.CreatedBy, &o.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	o.ExpectedDelivery = expectedDelivery

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, order_id, line_number, COALESCE(description, ''), quantity, COALESCE(unit, ''),
		       unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
		       product_id
		FROM %s.order_lines
		WHERE order_id = $1 AND tenant_id = $2
		ORDER BY line_number
	`, schemaName), orderID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get order lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line OrderLine
		if err := rows.Scan(
			&line.ID, &line.TenantID, &line.OrderID, &line.LineNumber, &line.Description,
			&line.Quantity, &line.Unit, &line.UnitPrice, &line.DiscountPercent, &line.VATRate,
			&line.LineSubtotal, &line.LineVAT, &line.LineTotal, &line.ProductID,
		); err != nil {
			return nil, fmt.Errorf("scan order line: %w", err)
		}
		o.Lines = append(o.Lines, line)
	}

	return &o, nil
}

// List retrieves orders with optional filtering
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter *OrderFilter) ([]Order, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, order_number, contact_id, order_date, expected_delivery,
		       status, currency, exchange_rate, subtotal, vat_amount, total,
		       COALESCE(notes, ''), quote_id, converted_to_invoice_id,
		       created_at, created_by, updated_at
		FROM %s.orders
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
			query += fmt.Sprintf(" AND order_date >= $%d", argNum)
			args = append(args, filter.FromDate)
			argNum++
		}
		if filter.ToDate != nil {
			query += fmt.Sprintf(" AND order_date <= $%d", argNum)
			args = append(args, filter.ToDate)
			argNum++
		}
		if filter.Search != "" {
			query += fmt.Sprintf(" AND (order_number ILIKE $%d)", argNum)
			args = append(args, "%"+filter.Search+"%")
		}
	}

	query += " ORDER BY order_date DESC, order_number DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		var expectedDelivery *time.Time
		if err := rows.Scan(
			&o.ID, &o.TenantID, &o.OrderNumber, &o.ContactID, &o.OrderDate, &expectedDelivery,
			&o.Status, &o.Currency, &o.ExchangeRate, &o.Subtotal, &o.VATAmount, &o.Total,
			&o.Notes, &o.QuoteID, &o.ConvertedToInvoiceID,
			&o.CreatedAt, &o.CreatedBy, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		o.ExpectedDelivery = expectedDelivery
		orders = append(orders, o)
	}

	return orders, nil
}

// Update updates an order and its lines
func (r *PostgresRepository) Update(ctx context.Context, schemaName string, order *Order) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.orders
		SET contact_id = $1, order_date = $2, expected_delivery = $3, currency = $4,
		    exchange_rate = $5, subtotal = $6, vat_amount = $7, total = $8,
		    notes = $9, updated_at = $10
		WHERE id = $11 AND tenant_id = $12 AND status IN ('PENDING', 'CONFIRMED')
	`, schemaName),
		order.ContactID, order.OrderDate, order.ExpectedDelivery, order.Currency,
		order.ExchangeRate, order.Subtotal, order.VATAmount, order.Total,
		order.Notes, time.Now(), order.ID, order.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrderNotFound
	}

	// Delete existing lines and insert new ones
	_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_lines WHERE order_id = $1`, schemaName), order.ID)
	if err != nil {
		return fmt.Errorf("delete order lines: %w", err)
	}

	for i := range order.Lines {
		line := &order.Lines[i]
		line.OrderID = order.ID

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_lines (
				id, tenant_id, order_id, line_number, description, quantity, unit,
				unit_price, discount_percent, vat_rate, line_subtotal, line_vat, line_total,
				product_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, schemaName),
			line.ID, line.TenantID, line.OrderID, line.LineNumber, line.Description,
			line.Quantity, line.Unit, line.UnitPrice, line.DiscountPercent, line.VATRate,
			line.LineSubtotal, line.LineVAT, line.LineTotal, line.ProductID,
		)
		if err != nil {
			return fmt.Errorf("insert order line: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateStatus updates the status of an order
func (r *PostgresRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, orderID string, status OrderStatus) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.orders
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), status, time.Now(), orderID, tenantID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// Delete removes an order (only pending)
func (r *PostgresRepository) Delete(ctx context.Context, schemaName, tenantID, orderID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.orders
		WHERE id = $1 AND tenant_id = $2 AND status = 'PENDING'
	`, schemaName), orderID, tenantID)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// GenerateNumber generates a new order number
func (r *PostgresRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	var seq int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(order_number FROM 'ORD-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.orders WHERE tenant_id = $1
	`, schemaName), tenantID).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate order number: %w", err)
	}

	return fmt.Sprintf("ORD-%05d", seq), nil
}

// SetConvertedToInvoice marks an order as converted to an invoice
func (r *PostgresRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, orderID, invoiceID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.orders
		SET converted_to_invoice_id = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), invoiceID, time.Now(), orderID, tenantID)
	if err != nil {
		return fmt.Errorf("set converted to invoice: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}
