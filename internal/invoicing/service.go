package invoicing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
)

// Service provides invoicing operations
type Service struct {
	db         *pgxpool.Pool
	repo       Repository
	accounting *accounting.Service
}

// NewService creates a new invoicing service with a PostgreSQL repository
func NewService(db *pgxpool.Pool, accountingService *accounting.Service) *Service {
	return &Service{
		db:         db,
		repo:       NewPostgresRepository(db),
		accounting: accountingService,
	}
}

// NewServiceWithRepository creates a new invoicing service with a custom repository
func NewServiceWithRepository(repo Repository, accountingService *accounting.Service) *Service {
	return &Service{
		repo:       repo,
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

	// Generate invoice number
	invoiceNumber, err := s.repo.GenerateNumber(ctx, schemaName, tenantID, invoice.InvoiceType)
	if err != nil {
		return nil, fmt.Errorf("generate invoice number: %w", err)
	}
	invoice.InvoiceNumber = invoiceNumber

	// Create invoice via repository
	if err := s.repo.Create(ctx, schemaName, invoice); err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	return invoice, nil
}

// GetByID retrieves an invoice by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, invoiceID string) (*Invoice, error) {
	invoice, err := s.repo.GetByID(ctx, schemaName, tenantID, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}
	return invoice, nil
}

// List retrieves invoices with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *InvoiceFilter) ([]Invoice, error) {
	invoices, err := s.repo.List(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	return invoices, nil
}

// Send marks an invoice as sent and updates status
func (s *Service) Send(ctx context.Context, tenantID, schemaName, invoiceID string) error {
	// Verify invoice exists and is in draft status
	invoice, err := s.repo.GetByID(ctx, schemaName, tenantID, invoiceID)
	if err != nil {
		return fmt.Errorf("get invoice: %w", err)
	}
	if invoice.Status != StatusDraft {
		return fmt.Errorf("invoice not in draft status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, invoiceID, StatusSent); err != nil {
		return fmt.Errorf("send invoice: %w", err)
	}
	return nil
}

// RecordPayment records a payment against an invoice
func (s *Service) RecordPayment(ctx context.Context, tenantID, schemaName, invoiceID string, amount decimal.Decimal) error {
	invoice, err := s.repo.GetByID(ctx, schemaName, tenantID, invoiceID)
	if err != nil {
		return fmt.Errorf("get invoice: %w", err)
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

	if err := s.repo.UpdatePayment(ctx, schemaName, tenantID, invoiceID, newAmountPaid, newStatus); err != nil {
		return fmt.Errorf("record payment: %w", err)
	}

	return nil
}

// Void voids an invoice
func (s *Service) Void(ctx context.Context, tenantID, schemaName, invoiceID string) error {
	invoice, err := s.repo.GetByID(ctx, schemaName, tenantID, invoiceID)
	if err != nil {
		return fmt.Errorf("get invoice: %w", err)
	}

	if invoice.Status == StatusVoided {
		return fmt.Errorf("invoice already voided")
	}

	if !invoice.AmountPaid.IsZero() {
		return fmt.Errorf("cannot void invoice with payments")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, invoiceID, StatusVoided); err != nil {
		return fmt.Errorf("void invoice: %w", err)
	}

	return nil
}

// UpdateOverdueStatus updates status of overdue invoices
func (s *Service) UpdateOverdueStatus(ctx context.Context, tenantID, schemaName string) (int, error) {
	return s.repo.UpdateOverdueStatus(ctx, schemaName, tenantID)
}
