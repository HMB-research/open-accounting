package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
)

// Service provides invoicing operations
type Service struct {
	db         *pgxpool.Pool
	accounting *accounting.Service
}

// NewService creates a new invoicing service
func NewService(db *pgxpool.Pool, accountingService *accounting.Service) *Service {
	return &Service{
		db:         db,
		accounting: accountingService,
	}
}

// Create creates a new invoice
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreateInvoiceRequest) (*Invoice, error) {
	invoice := &Invoice{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		InvoiceType:  req.InvoiceType,
		ContactID:    req.ContactID,
		IssueDate:    req.IssueDate,
		DueDate:      req.DueDate,
		Currency:     req.Currency,
		ExchangeRate: req.ExchangeRate,
		Status:       StatusDraft,
		Reference:    req.Reference,
		Notes:        req.Notes,
		AmountPaid:   decimal.Zero,
		CreatedAt:    time.Now(),
		CreatedBy:    req.UserID,
		UpdatedAt:    time.Now(),
	}

	if invoice.Currency == "" {
		invoice.Currency = "EUR"
	}
	if invoice.ExchangeRate.IsZero() {
		invoice.ExchangeRate = decimal.NewFromInt(1)
	}
	if invoice.IssueDate.IsZero() {
		invoice.IssueDate = time.Now()
	}
	if invoice.DueDate.IsZero() {
		invoice.DueDate = invoice.IssueDate.AddDate(0, 0, 14) // Default 14 days
	}

	// Convert request lines to invoice lines
	for i, reqLine := range req.Lines {
		line := InvoiceLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			LineNumber:      i + 1,
			Description:     reqLine.Description,
			Quantity:        reqLine.Quantity,
			Unit:            reqLine.Unit,
			UnitPrice:       reqLine.UnitPrice,
			DiscountPercent: reqLine.DiscountPercent,
			VATRate:         reqLine.VATRate,
			AccountID:       reqLine.AccountID,
			ProductID:       reqLine.ProductID,
		}
		line.Calculate()
		invoice.Lines = append(invoice.Lines, line)
	}

	// Calculate totals
	invoice.Calculate()

	// Validate
	if err := invoice.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Generate invoice number
	var seq int
	prefix := "INV"
	if invoice.InvoiceType == InvoiceTypePurchase {
		prefix = "BILL"
	} else if invoice.InvoiceType == InvoiceTypeCreditNote {
		prefix = "CN"
	}

	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(invoice_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.invoices WHERE tenant_id = $1 AND invoice_type = $2
	`, prefix, schemaName), tenantID, invoice.InvoiceType).Scan(&seq)
	if err != nil {
		return nil, fmt.Errorf("generate invoice number: %w", err)
	}
	invoice.InvoiceNumber = fmt.Sprintf("%s-%05d", prefix, seq)

	// Insert invoice
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
		return nil, fmt.Errorf("insert invoice: %w", err)
	}

	// Insert lines
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
			return nil, fmt.Errorf("insert invoice line: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return invoice, nil
}

// GetByID retrieves an invoice by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, invoiceID string) (*Invoice, error) {
	var inv Invoice
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
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
		return nil, fmt.Errorf("invoice not found: %s", invoiceID)
	}
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}

	// Load lines
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
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
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *InvoiceFilter) ([]Invoice, error) {
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

	rows, err := s.db.Query(ctx, query, args...)
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

// Send marks an invoice as sent and updates status
func (s *Service) Send(ctx context.Context, tenantID, schemaName, invoiceID string) error {
	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4 AND status = $5
	`, schemaName), StatusSent, time.Now(), invoiceID, tenantID, StatusDraft)
	if err != nil {
		return fmt.Errorf("send invoice: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("invoice not found or not in draft status")
	}
	return nil
}

// RecordPayment records a payment against an invoice
func (s *Service) RecordPayment(ctx context.Context, tenantID, schemaName, invoiceID string, amount decimal.Decimal) error {
	invoice, err := s.GetByID(ctx, tenantID, schemaName, invoiceID)
	if err != nil {
		return err
	}

	if invoice.Status == StatusVoided {
		return fmt.Errorf("cannot record payment on voided invoice")
	}

	newAmountPaid := invoice.AmountPaid.Add(amount)
	var newStatus InvoiceStatus

	if newAmountPaid.GreaterThanOrEqual(invoice.Total) {
		newStatus = StatusPaid
		newAmountPaid = invoice.Total // Don't allow overpayment
	} else if newAmountPaid.GreaterThan(decimal.Zero) {
		newStatus = StatusPartiallyPaid
	} else {
		newStatus = invoice.Status
	}

	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices
		SET amount_paid = $1, status = $2, updated_at = $3
		WHERE id = $4 AND tenant_id = $5
	`, schemaName), newAmountPaid, newStatus, time.Now(), invoiceID, tenantID)
	if err != nil {
		return fmt.Errorf("record payment: %w", err)
	}

	return nil
}

// Void voids an invoice
func (s *Service) Void(ctx context.Context, tenantID, schemaName, invoiceID string) error {
	invoice, err := s.GetByID(ctx, tenantID, schemaName, invoiceID)
	if err != nil {
		return err
	}

	if invoice.Status == StatusVoided {
		return fmt.Errorf("invoice already voided")
	}

	if !invoice.AmountPaid.IsZero() {
		return fmt.Errorf("cannot void invoice with payments")
	}

	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.invoices
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), StatusVoided, time.Now(), invoiceID, tenantID)
	if err != nil {
		return fmt.Errorf("void invoice: %w", err)
	}

	return nil
}

// UpdateOverdueStatus updates status of overdue invoices
func (s *Service) UpdateOverdueStatus(ctx context.Context, tenantID, schemaName string) (int, error) {
	result, err := s.db.Exec(ctx, fmt.Sprintf(`
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
