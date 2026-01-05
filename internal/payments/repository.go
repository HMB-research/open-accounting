package payments

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the interface for payment data access
type Repository interface {
	Create(ctx context.Context, schemaName string, payment *Payment) error
	GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error)
	List(ctx context.Context, schemaName, tenantID string, filter *PaymentFilter) ([]Payment, error)
	CreateAllocation(ctx context.Context, schemaName string, allocation *PaymentAllocation) error
	GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]PaymentAllocation, error)
	GetNextPaymentNumber(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) (int, error)
	GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) ([]Payment, error)
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create inserts a new payment
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, payment *Payment) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.payments (
			id, tenant_id, payment_number, payment_type, contact_id, payment_date,
			amount, currency, exchange_rate, base_amount, payment_method, bank_account,
			reference, notes, created_at, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, schemaName),
		payment.ID, payment.TenantID, payment.PaymentNumber, payment.PaymentType,
		payment.ContactID, payment.PaymentDate, payment.Amount, payment.Currency,
		payment.ExchangeRate, payment.BaseAmount, payment.PaymentMethod, payment.BankAccount,
		payment.Reference, payment.Notes, payment.CreatedAt, payment.CreatedBy,
	)
	return err
}

// GetByID retrieves a payment by ID
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, paymentID string) (*Payment, error) {
	var p Payment
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, payment_number, payment_type, contact_id, payment_date,
		       amount, currency, exchange_rate, base_amount, COALESCE(payment_method, ''), COALESCE(bank_account, ''),
		       COALESCE(reference, ''), COALESCE(notes, ''), journal_entry_id, created_at, created_by
		FROM %s.payments
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), paymentID, tenantID).Scan(
		&p.ID, &p.TenantID, &p.PaymentNumber, &p.PaymentType, &p.ContactID,
		&p.PaymentDate, &p.Amount, &p.Currency, &p.ExchangeRate, &p.BaseAmount,
		&p.PaymentMethod, &p.BankAccount, &p.Reference, &p.Notes,
		&p.JournalEntryID, &p.CreatedAt, &p.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List retrieves payments with optional filtering
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter *PaymentFilter) ([]Payment, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, payment_number, payment_type, contact_id, payment_date,
		       amount, currency, exchange_rate, base_amount, COALESCE(payment_method, ''), COALESCE(bank_account, ''),
		       COALESCE(reference, ''), COALESCE(notes, ''), journal_entry_id, created_at, created_by
		FROM %s.payments
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.PaymentType != "" {
			query += fmt.Sprintf(" AND payment_type = $%d", argNum)
			args = append(args, filter.PaymentType)
			argNum++
		}
		if filter.ContactID != "" {
			query += fmt.Sprintf(" AND contact_id = $%d", argNum)
			args = append(args, filter.ContactID)
			argNum++
		}
		if filter.FromDate != nil {
			query += fmt.Sprintf(" AND payment_date >= $%d", argNum)
			args = append(args, filter.FromDate)
			argNum++
		}
		if filter.ToDate != nil {
			query += fmt.Sprintf(" AND payment_date <= $%d", argNum)
			args = append(args, filter.ToDate)
		}
	}

	query += " ORDER BY payment_date DESC, payment_number DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.PaymentNumber, &p.PaymentType, &p.ContactID,
			&p.PaymentDate, &p.Amount, &p.Currency, &p.ExchangeRate, &p.BaseAmount,
			&p.PaymentMethod, &p.BankAccount, &p.Reference, &p.Notes,
			&p.JournalEntryID, &p.CreatedAt, &p.CreatedBy,
		); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	return payments, nil
}

// CreateAllocation inserts a payment allocation
func (r *PostgresRepository) CreateAllocation(ctx context.Context, schemaName string, allocation *PaymentAllocation) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.payment_allocations (id, tenant_id, payment_id, invoice_id, amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, schemaName),
		allocation.ID, allocation.TenantID, allocation.PaymentID,
		allocation.InvoiceID, allocation.Amount, allocation.CreatedAt,
	)
	return err
}

// GetAllocations retrieves allocations for a payment
func (r *PostgresRepository) GetAllocations(ctx context.Context, schemaName, tenantID, paymentID string) ([]PaymentAllocation, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, payment_id, invoice_id, amount, created_at
		FROM %s.payment_allocations
		WHERE payment_id = $1 AND tenant_id = $2
	`, schemaName), paymentID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocations []PaymentAllocation
	for rows.Next() {
		var a PaymentAllocation
		if err := rows.Scan(&a.ID, &a.TenantID, &a.PaymentID, &a.InvoiceID, &a.Amount, &a.CreatedAt); err != nil {
			return nil, err
		}
		allocations = append(allocations, a)
	}

	return allocations, nil
}

// GetNextPaymentNumber returns the next payment number sequence
func (r *PostgresRepository) GetNextPaymentNumber(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) (int, error) {
	prefix := "PMT"
	if paymentType == PaymentTypeMade {
		prefix = "OUT"
	}

	var seq int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(payment_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.payments WHERE tenant_id = $1 AND payment_type = $2
	`, prefix, schemaName), tenantID, paymentType).Scan(&seq)
	if err != nil {
		return 0, err
	}

	return seq, nil
}

// GetUnallocatedPayments returns payments with unallocated amounts
func (r *PostgresRepository) GetUnallocatedPayments(ctx context.Context, schemaName, tenantID string, paymentType PaymentType) ([]Payment, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.tenant_id, p.payment_number, p.payment_type, p.contact_id, p.payment_date,
		       p.amount, p.currency, p.exchange_rate, p.base_amount, COALESCE(p.payment_method, ''), COALESCE(p.bank_account, ''),
		       COALESCE(p.reference, ''), COALESCE(p.notes, ''), p.journal_entry_id, p.created_at, p.created_by
		FROM %s.payments p
		WHERE p.tenant_id = $1 AND p.payment_type = $2
		  AND p.amount > COALESCE((
		      SELECT SUM(pa.amount) FROM %s.payment_allocations pa WHERE pa.payment_id = p.id
		  ), 0)
		ORDER BY p.payment_date
	`, schemaName, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID, paymentType)
	if err != nil {
		return nil, fmt.Errorf("get unallocated payments: %w", err)
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.PaymentNumber, &p.PaymentType, &p.ContactID,
			&p.PaymentDate, &p.Amount, &p.Currency, &p.ExchangeRate, &p.BaseAmount,
			&p.PaymentMethod, &p.BankAccount, &p.Reference, &p.Notes,
			&p.JournalEntryID, &p.CreatedAt, &p.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, p)
	}

	return payments, nil
}
