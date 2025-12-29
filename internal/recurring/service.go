package recurring

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

// Service provides recurring invoice operations
type Service struct {
	db         *pgxpool.Pool
	invoicing  *invoicing.Service
}

// NewService creates a new recurring invoice service
func NewService(db *pgxpool.Pool, invoicingService *invoicing.Service) *Service {
	return &Service{
		db:        db,
		invoicing: invoicingService,
	}
}

// EnsureSchema ensures the recurring invoice tables exist in the tenant schema
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	_, err := s.db.Exec(ctx, fmt.Sprintf(`
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
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
	if err != nil {
		return fmt.Errorf("ensure recurring schema: %w", err)
	}
	return nil
}

// Create creates a new recurring invoice
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreateRecurringInvoiceRequest) (*RecurringInvoice, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	ri := &RecurringInvoice{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		Name:               req.Name,
		ContactID:          req.ContactID,
		InvoiceType:        req.InvoiceType,
		Currency:           req.Currency,
		Frequency:          req.Frequency,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		NextGenerationDate: req.StartDate,
		PaymentTermsDays:   req.PaymentTermsDays,
		Reference:          req.Reference,
		Notes:              req.Notes,
		IsActive:           true,
		GeneratedCount:     0,
		CreatedAt:          time.Now(),
		CreatedBy:          req.UserID,
		UpdatedAt:          time.Now(),
	}

	if ri.Currency == "" {
		ri.Currency = "EUR"
	}
	if ri.InvoiceType == "" {
		ri.InvoiceType = "SALES"
	}
	if ri.PaymentTermsDays == 0 {
		ri.PaymentTermsDays = 14
	}

	// Convert lines
	for i, reqLine := range req.Lines {
		line := RecurringInvoiceLine{
			ID:                 uuid.New().String(),
			RecurringInvoiceID: ri.ID,
			LineNumber:         i + 1,
			Description:        reqLine.Description,
			Quantity:           reqLine.Quantity,
			Unit:               reqLine.Unit,
			UnitPrice:          reqLine.UnitPrice,
			DiscountPercent:    reqLine.DiscountPercent,
			VATRate:            reqLine.VATRate,
			AccountID:          reqLine.AccountID,
			ProductID:          reqLine.ProductID,
		}
		if line.Quantity.IsZero() {
			line.Quantity = decimal.NewFromInt(1)
		}
		ri.Lines = append(ri.Lines, line)
	}

	if err := ri.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert recurring invoice
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.recurring_invoices (
			id, tenant_id, name, contact_id, invoice_type, currency, frequency,
			start_date, end_date, next_generation_date, payment_terms_days,
			reference, notes, is_active, generated_count, created_at, created_by, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`, schemaName),
		ri.ID, ri.TenantID, ri.Name, ri.ContactID, ri.InvoiceType, ri.Currency, ri.Frequency,
		ri.StartDate, ri.EndDate, ri.NextGenerationDate, ri.PaymentTermsDays,
		ri.Reference, ri.Notes, ri.IsActive, ri.GeneratedCount, ri.CreatedAt, ri.CreatedBy, ri.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert recurring invoice: %w", err)
	}

	// Insert lines
	for _, line := range ri.Lines {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.recurring_invoice_lines (
				id, recurring_invoice_id, line_number, description, quantity, unit,
				unit_price, discount_percent, vat_rate, account_id, product_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, schemaName),
			line.ID, line.RecurringInvoiceID, line.LineNumber, line.Description, line.Quantity, line.Unit,
			line.UnitPrice, line.DiscountPercent, line.VATRate, line.AccountID, line.ProductID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert recurring invoice line: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return ri, nil
}

// CreateFromInvoice creates a recurring invoice from an existing invoice
func (s *Service) CreateFromInvoice(ctx context.Context, tenantID, schemaName string, req *CreateFromInvoiceRequest) (*RecurringInvoice, error) {
	invoice, err := s.invoicing.GetByID(ctx, tenantID, schemaName, req.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}

	// Convert invoice lines to recurring invoice lines
	var lines []CreateRecurringInvoiceLineRequest
	for _, invLine := range invoice.Lines {
		lines = append(lines, CreateRecurringInvoiceLineRequest{
			Description:     invLine.Description,
			Quantity:        invLine.Quantity,
			Unit:            invLine.Unit,
			UnitPrice:       invLine.UnitPrice,
			DiscountPercent: invLine.DiscountPercent,
			VATRate:         invLine.VATRate,
			AccountID:       invLine.AccountID,
			ProductID:       invLine.ProductID,
		})
	}

	createReq := &CreateRecurringInvoiceRequest{
		Name:             req.Name,
		ContactID:        invoice.ContactID,
		InvoiceType:      string(invoice.InvoiceType),
		Currency:         invoice.Currency,
		Frequency:        req.Frequency,
		StartDate:        req.StartDate,
		EndDate:          req.EndDate,
		PaymentTermsDays: req.PaymentTermsDays,
		Reference:        invoice.Reference,
		Notes:            invoice.Notes,
		Lines:            lines,
		UserID:           req.UserID,
	}

	return s.Create(ctx, tenantID, schemaName, createReq)
}

// GetByID retrieves a recurring invoice by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, id string) (*RecurringInvoice, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	var ri RecurringInvoice
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT r.id, r.tenant_id, r.name, r.contact_id, c.name as contact_name,
		       r.invoice_type, r.currency, r.frequency, r.start_date, r.end_date,
		       r.next_generation_date, r.payment_terms_days, r.reference, r.notes,
		       r.is_active, r.last_generated_at, r.generated_count, r.created_at, r.created_by, r.updated_at
		FROM %s.recurring_invoices r
		LEFT JOIN %s.contacts c ON r.contact_id = c.id
		WHERE r.id = $1 AND r.tenant_id = $2
	`, schemaName, schemaName), id, tenantID).Scan(
		&ri.ID, &ri.TenantID, &ri.Name, &ri.ContactID, &ri.ContactName,
		&ri.InvoiceType, &ri.Currency, &ri.Frequency, &ri.StartDate, &ri.EndDate,
		&ri.NextGenerationDate, &ri.PaymentTermsDays, &ri.Reference, &ri.Notes,
		&ri.IsActive, &ri.LastGeneratedAt, &ri.GeneratedCount, &ri.CreatedAt, &ri.CreatedBy, &ri.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("recurring invoice not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get recurring invoice: %w", err)
	}

	// Load lines
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id, recurring_invoice_id, line_number, description, quantity, unit,
		       unit_price, discount_percent, vat_rate, account_id, product_id
		FROM %s.recurring_invoice_lines
		WHERE recurring_invoice_id = $1
		ORDER BY line_number
	`, schemaName), id)
	if err != nil {
		return nil, fmt.Errorf("get recurring invoice lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line RecurringInvoiceLine
		if err := rows.Scan(
			&line.ID, &line.RecurringInvoiceID, &line.LineNumber, &line.Description,
			&line.Quantity, &line.Unit, &line.UnitPrice, &line.DiscountPercent,
			&line.VATRate, &line.AccountID, &line.ProductID,
		); err != nil {
			return nil, fmt.Errorf("scan recurring invoice line: %w", err)
		}
		ri.Lines = append(ri.Lines, line)
	}

	return &ri, nil
}

// List retrieves all recurring invoices for a tenant
func (s *Service) List(ctx context.Context, tenantID, schemaName string, activeOnly bool) ([]RecurringInvoice, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.tenant_id, r.name, r.contact_id, c.name as contact_name,
		       r.invoice_type, r.currency, r.frequency, r.start_date, r.end_date,
		       r.next_generation_date, r.payment_terms_days, r.reference, r.notes,
		       r.is_active, r.last_generated_at, r.generated_count, r.created_at, r.created_by, r.updated_at
		FROM %s.recurring_invoices r
		LEFT JOIN %s.contacts c ON r.contact_id = c.id
		WHERE r.tenant_id = $1
	`, schemaName, schemaName)

	if activeOnly {
		query += " AND r.is_active = true"
	}
	query += " ORDER BY r.next_generation_date, r.name"

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list recurring invoices: %w", err)
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
		); err != nil {
			return nil, fmt.Errorf("scan recurring invoice: %w", err)
		}
		results = append(results, ri)
	}

	return results, nil
}

// Update updates a recurring invoice
func (s *Service) Update(ctx context.Context, tenantID, schemaName, id string, req *UpdateRecurringInvoiceRequest) (*RecurringInvoice, error) {
	ri, err := s.GetByID(ctx, tenantID, schemaName, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		ri.Name = *req.Name
	}
	if req.ContactID != nil {
		ri.ContactID = *req.ContactID
	}
	if req.Frequency != nil {
		ri.Frequency = *req.Frequency
	}
	if req.EndDate != nil {
		ri.EndDate = req.EndDate
	}
	if req.PaymentTermsDays != nil {
		ri.PaymentTermsDays = *req.PaymentTermsDays
	}
	if req.Reference != nil {
		ri.Reference = *req.Reference
	}
	if req.Notes != nil {
		ri.Notes = *req.Notes
	}
	ri.UpdatedAt = time.Now()

	if err := ri.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.recurring_invoices SET
			name = $1, contact_id = $2, frequency = $3, end_date = $4,
			payment_terms_days = $5, reference = $6, notes = $7, updated_at = $8
		WHERE id = $9 AND tenant_id = $10
	`, schemaName),
		ri.Name, ri.ContactID, ri.Frequency, ri.EndDate, ri.PaymentTermsDays,
		ri.Reference, ri.Notes, ri.UpdatedAt, ri.ID, ri.TenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("update recurring invoice: %w", err)
	}

	// Update lines if provided
	if len(req.Lines) > 0 {
		// Delete existing lines
		_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.recurring_invoice_lines WHERE recurring_invoice_id = $1`, schemaName), id)
		if err != nil {
			return nil, fmt.Errorf("delete recurring invoice lines: %w", err)
		}

		// Insert new lines
		ri.Lines = nil
		for i, reqLine := range req.Lines {
			line := RecurringInvoiceLine{
				ID:                 uuid.New().String(),
				RecurringInvoiceID: ri.ID,
				LineNumber:         i + 1,
				Description:        reqLine.Description,
				Quantity:           reqLine.Quantity,
				Unit:               reqLine.Unit,
				UnitPrice:          reqLine.UnitPrice,
				DiscountPercent:    reqLine.DiscountPercent,
				VATRate:            reqLine.VATRate,
				AccountID:          reqLine.AccountID,
				ProductID:          reqLine.ProductID,
			}
			if line.Quantity.IsZero() {
				line.Quantity = decimal.NewFromInt(1)
			}

			_, err = tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.recurring_invoice_lines (
					id, recurring_invoice_id, line_number, description, quantity, unit,
					unit_price, discount_percent, vat_rate, account_id, product_id
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`, schemaName),
				line.ID, line.RecurringInvoiceID, line.LineNumber, line.Description, line.Quantity, line.Unit,
				line.UnitPrice, line.DiscountPercent, line.VATRate, line.AccountID, line.ProductID,
			)
			if err != nil {
				return nil, fmt.Errorf("insert recurring invoice line: %w", err)
			}
			ri.Lines = append(ri.Lines, line)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return ri, nil
}

// Delete deletes a recurring invoice
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, id string) error {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return err
	}

	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.recurring_invoices WHERE id = $1 AND tenant_id = $2
	`, schemaName), id, tenantID)
	if err != nil {
		return fmt.Errorf("delete recurring invoice: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("recurring invoice not found: %s", id)
	}
	return nil
}

// Pause pauses a recurring invoice
func (s *Service) Pause(ctx context.Context, tenantID, schemaName, id string) error {
	return s.setActive(ctx, tenantID, schemaName, id, false)
}

// Resume resumes a paused recurring invoice
func (s *Service) Resume(ctx context.Context, tenantID, schemaName, id string) error {
	return s.setActive(ctx, tenantID, schemaName, id, true)
}

func (s *Service) setActive(ctx context.Context, tenantID, schemaName, id string, active bool) error {
	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.recurring_invoices SET is_active = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), active, time.Now(), id, tenantID)
	if err != nil {
		return fmt.Errorf("update recurring invoice: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("recurring invoice not found: %s", id)
	}
	return nil
}

// GenerateDueInvoices generates invoices for all due recurring invoices
func (s *Service) GenerateDueInvoices(ctx context.Context, tenantID, schemaName, userID string) ([]GenerationResult, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, err
	}

	// Find all due recurring invoices
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id FROM %s.recurring_invoices
		WHERE tenant_id = $1
		  AND is_active = true
		  AND next_generation_date <= $2
		  AND (end_date IS NULL OR end_date >= $2)
	`, schemaName), tenantID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("list due recurring invoices: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan recurring invoice id: %w", err)
		}
		ids = append(ids, id)
	}

	var results []GenerationResult
	for _, id := range ids {
		result, err := s.GenerateInvoice(ctx, tenantID, schemaName, id, userID)
		if err != nil {
			// Log error but continue with other recurring invoices
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

// GenerateInvoice generates a single invoice from a recurring invoice
func (s *Service) GenerateInvoice(ctx context.Context, tenantID, schemaName, recurringID, userID string) (*GenerationResult, error) {
	ri, err := s.GetByID(ctx, tenantID, schemaName, recurringID)
	if err != nil {
		return nil, err
	}

	if !ri.IsActive {
		return nil, fmt.Errorf("recurring invoice is not active")
	}

	// Create invoice request from recurring invoice
	issueDate := time.Now()
	dueDate := issueDate.AddDate(0, 0, ri.PaymentTermsDays)

	var lines []invoicing.CreateInvoiceLineRequest
	for _, riLine := range ri.Lines {
		lines = append(lines, invoicing.CreateInvoiceLineRequest{
			Description:     riLine.Description,
			Quantity:        riLine.Quantity,
			Unit:            riLine.Unit,
			UnitPrice:       riLine.UnitPrice,
			DiscountPercent: riLine.DiscountPercent,
			VATRate:         riLine.VATRate,
			AccountID:       riLine.AccountID,
			ProductID:       riLine.ProductID,
		})
	}

	invoiceReq := &invoicing.CreateInvoiceRequest{
		InvoiceType:  invoicing.InvoiceType(ri.InvoiceType),
		ContactID:    ri.ContactID,
		IssueDate:    issueDate,
		DueDate:      dueDate,
		Currency:     ri.Currency,
		ExchangeRate: decimal.NewFromInt(1),
		Reference:    ri.Reference,
		Notes:        ri.Notes,
		Lines:        lines,
		UserID:       userID,
	}

	// Create the invoice
	invoice, err := s.invoicing.Create(ctx, tenantID, schemaName, invoiceReq)
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	// Update recurring invoice
	nextDate := ri.CalculateNextDate(ri.NextGenerationDate)
	now := time.Now()
	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.recurring_invoices SET
			next_generation_date = $1,
			last_generated_at = $2,
			generated_count = generated_count + 1,
			updated_at = $3
		WHERE id = $4 AND tenant_id = $5
	`, schemaName), nextDate, now, now, recurringID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("update recurring invoice: %w", err)
	}

	return &GenerationResult{
		RecurringInvoiceID:     recurringID,
		GeneratedInvoiceID:     invoice.ID,
		GeneratedInvoiceNumber: invoice.InvoiceNumber,
	}, nil
}
