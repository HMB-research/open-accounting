package payments

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/openaccounting/openaccounting/internal/invoicing"
)

// Service provides payment operations
type Service struct {
	db        *pgxpool.Pool
	invoicing *invoicing.Service
}

// NewService creates a new payments service
func NewService(db *pgxpool.Pool, invoicingService *invoicing.Service) *Service {
	return &Service{
		db:        db,
		invoicing: invoicingService,
	}
}

// Create creates a new payment
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreatePaymentRequest) (*Payment, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("payment amount must be positive")
	}

	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		PaymentType:   req.PaymentType,
		ContactID:     req.ContactID,
		PaymentDate:   req.PaymentDate,
		Amount:        req.Amount,
		Currency:      req.Currency,
		ExchangeRate:  req.ExchangeRate,
		PaymentMethod: req.PaymentMethod,
		BankAccount:   req.BankAccount,
		Reference:     req.Reference,
		Notes:         req.Notes,
		CreatedAt:     time.Now(),
		CreatedBy:     req.UserID,
	}

	if payment.Currency == "" {
		payment.Currency = "EUR"
	}
	if payment.ExchangeRate.IsZero() {
		payment.ExchangeRate = decimal.NewFromInt(1)
	}
	if payment.PaymentDate.IsZero() {
		payment.PaymentDate = time.Now()
	}

	payment.BaseAmount = payment.Amount.Mul(payment.ExchangeRate).Round(2)

	// Validate allocations don't exceed payment amount
	totalAllocated := decimal.Zero
	for _, alloc := range req.Allocations {
		if alloc.Amount.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("allocation amount must be positive")
		}
		totalAllocated = totalAllocated.Add(alloc.Amount)
	}
	if totalAllocated.GreaterThan(payment.Amount) {
		return nil, fmt.Errorf("total allocations exceed payment amount")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Generate payment number
	var seq int
	prefix := "PMT"
	if payment.PaymentType == PaymentTypeMade {
		prefix = "OUT"
	}

	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(payment_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.payments WHERE tenant_id = $1 AND payment_type = $2
	`, prefix, schemaName), tenantID, payment.PaymentType).Scan(&seq)
	if err != nil {
		return nil, fmt.Errorf("generate payment number: %w", err)
	}
	payment.PaymentNumber = fmt.Sprintf("%s-%05d", prefix, seq)

	// Insert payment
	_, err = tx.Exec(ctx, fmt.Sprintf(`
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
	if err != nil {
		return nil, fmt.Errorf("insert payment: %w", err)
	}

	// Create allocations and update invoices
	for _, allocReq := range req.Allocations {
		allocation := PaymentAllocation{
			ID:        uuid.New().String(),
			TenantID:  tenantID,
			PaymentID: payment.ID,
			InvoiceID: allocReq.InvoiceID,
			Amount:    allocReq.Amount,
			CreatedAt: time.Now(),
		}

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.payment_allocations (id, tenant_id, payment_id, invoice_id, amount, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, schemaName),
			allocation.ID, allocation.TenantID, allocation.PaymentID,
			allocation.InvoiceID, allocation.Amount, allocation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert allocation: %w", err)
		}

		payment.Allocations = append(payment.Allocations, allocation)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Update invoice payment amounts (outside transaction for simplicity)
	for _, alloc := range payment.Allocations {
		if err := s.invoicing.RecordPayment(ctx, tenantID, schemaName, alloc.InvoiceID, alloc.Amount); err != nil {
			// Log error but don't fail - payment is recorded
			fmt.Printf("warning: failed to update invoice %s payment amount: %v\n", alloc.InvoiceID, err)
		}
	}

	return payment, nil
}

// GetByID retrieves a payment by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, paymentID string) (*Payment, error) {
	var p Payment
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, payment_number, payment_type, contact_id, payment_date,
		       amount, currency, exchange_rate, base_amount, payment_method, bank_account,
		       reference, notes, journal_entry_id, created_at, created_by
		FROM %s.payments
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), paymentID, tenantID).Scan(
		&p.ID, &p.TenantID, &p.PaymentNumber, &p.PaymentType, &p.ContactID,
		&p.PaymentDate, &p.Amount, &p.Currency, &p.ExchangeRate, &p.BaseAmount,
		&p.PaymentMethod, &p.BankAccount, &p.Reference, &p.Notes,
		&p.JournalEntryID, &p.CreatedAt, &p.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("payment not found: %s", paymentID)
	}
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}

	// Load allocations
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, payment_id, invoice_id, amount, created_at
		FROM %s.payment_allocations
		WHERE payment_id = $1 AND tenant_id = $2
	`, schemaName), paymentID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get allocations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a PaymentAllocation
		if err := rows.Scan(&a.ID, &a.TenantID, &a.PaymentID, &a.InvoiceID, &a.Amount, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan allocation: %w", err)
		}
		p.Allocations = append(p.Allocations, a)
	}

	return &p, nil
}

// List retrieves payments with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *PaymentFilter) ([]Payment, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, payment_number, payment_type, contact_id, payment_date,
		       amount, currency, exchange_rate, base_amount, payment_method, bank_account,
		       reference, notes, journal_entry_id, created_at, created_by
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

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
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

// AllocateToInvoice allocates part of an existing payment to an invoice
func (s *Service) AllocateToInvoice(ctx context.Context, tenantID, schemaName, paymentID, invoiceID string, amount decimal.Decimal) error {
	payment, err := s.GetByID(ctx, tenantID, schemaName, paymentID)
	if err != nil {
		return err
	}

	unallocated := payment.UnallocatedAmount()
	if amount.GreaterThan(unallocated) {
		return fmt.Errorf("amount exceeds unallocated balance of %s", unallocated.String())
	}

	allocation := PaymentAllocation{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Amount:    amount,
		CreatedAt: time.Now(),
	}

	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.payment_allocations (id, tenant_id, payment_id, invoice_id, amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, schemaName),
		allocation.ID, allocation.TenantID, allocation.PaymentID,
		allocation.InvoiceID, allocation.Amount, allocation.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert allocation: %w", err)
	}

	// Update invoice
	if err := s.invoicing.RecordPayment(ctx, tenantID, schemaName, invoiceID, amount); err != nil {
		return fmt.Errorf("update invoice payment: %w", err)
	}

	return nil
}

// GetUnallocatedPayments returns payments with unallocated amounts
func (s *Service) GetUnallocatedPayments(ctx context.Context, tenantID, schemaName string, paymentType PaymentType) ([]Payment, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.tenant_id, p.payment_number, p.payment_type, p.contact_id, p.payment_date,
		       p.amount, p.currency, p.exchange_rate, p.base_amount, p.payment_method, p.bank_account,
		       p.reference, p.notes, p.journal_entry_id, p.created_at, p.created_by
		FROM %s.payments p
		WHERE p.tenant_id = $1 AND p.payment_type = $2
		  AND p.amount > COALESCE((
		      SELECT SUM(pa.amount) FROM %s.payment_allocations pa WHERE pa.payment_id = p.id
		  ), 0)
		ORDER BY p.payment_date
	`, schemaName, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID, paymentType)
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
