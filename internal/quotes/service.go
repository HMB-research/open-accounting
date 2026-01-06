package quotes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides quote operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new quotes service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new quotes service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// Create creates a new quote
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreateQuoteRequest) (*Quote, error) {
	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		ContactID:    req.ContactID,
		QuoteDate:    req.QuoteDate,
		ValidUntil:   req.ValidUntil,
		Currency:     req.Currency,
		ExchangeRate: req.ExchangeRate,
		Status:       QuoteStatusDraft,
		Notes:        req.Notes,
		CreatedAt:    time.Now(),
		CreatedBy:    req.UserID,
		UpdatedAt:    time.Now(),
	}

	if quote.Currency == "" {
		quote.Currency = "EUR"
	}
	if quote.ExchangeRate.IsZero() {
		quote.ExchangeRate = decimal.NewFromInt(1)
	}
	if quote.QuoteDate.IsZero() {
		quote.QuoteDate = time.Now()
	}

	// Convert request lines to quote lines
	for i, reqLine := range req.Lines {
		line := QuoteLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			LineNumber:      i + 1,
			Description:     reqLine.Description,
			Quantity:        reqLine.Quantity,
			Unit:            reqLine.Unit,
			UnitPrice:       reqLine.UnitPrice,
			DiscountPercent: reqLine.DiscountPercent,
			VATRate:         reqLine.VATRate,
			ProductID:       reqLine.ProductID,
		}
		line.Calculate()
		quote.Lines = append(quote.Lines, line)
	}

	// Calculate totals
	quote.Calculate()

	// Validate
	if err := quote.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate quote number
	quoteNumber, err := s.repo.GenerateNumber(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("generate quote number: %w", err)
	}
	quote.QuoteNumber = quoteNumber

	// Create quote via repository
	if err := s.repo.Create(ctx, schemaName, quote); err != nil {
		return nil, fmt.Errorf("create quote: %w", err)
	}

	return quote, nil
}

// GetByID retrieves a quote by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, quoteID string) (*Quote, error) {
	quote, err := s.repo.GetByID(ctx, schemaName, tenantID, quoteID)
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}
	return quote, nil
}

// List retrieves quotes with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *QuoteFilter) ([]Quote, error) {
	quotes, err := s.repo.List(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list quotes: %w", err)
	}
	return quotes, nil
}

// Update updates a quote (only drafts)
func (s *Service) Update(ctx context.Context, tenantID, schemaName, quoteID string, req *UpdateQuoteRequest) (*Quote, error) {
	// Get existing quote
	existing, err := s.repo.GetByID(ctx, schemaName, tenantID, quoteID)
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}

	if existing.Status != QuoteStatusDraft {
		return nil, fmt.Errorf("only draft quotes can be updated")
	}

	// Update fields
	existing.ContactID = req.ContactID
	existing.QuoteDate = req.QuoteDate
	existing.ValidUntil = req.ValidUntil
	existing.Currency = req.Currency
	existing.ExchangeRate = req.ExchangeRate
	existing.Notes = req.Notes
	existing.UpdatedAt = time.Now()

	if existing.Currency == "" {
		existing.Currency = "EUR"
	}
	if existing.ExchangeRate.IsZero() {
		existing.ExchangeRate = decimal.NewFromInt(1)
	}

	// Replace lines
	existing.Lines = nil
	for i, reqLine := range req.Lines {
		line := QuoteLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			QuoteID:         quoteID,
			LineNumber:      i + 1,
			Description:     reqLine.Description,
			Quantity:        reqLine.Quantity,
			Unit:            reqLine.Unit,
			UnitPrice:       reqLine.UnitPrice,
			DiscountPercent: reqLine.DiscountPercent,
			VATRate:         reqLine.VATRate,
			ProductID:       reqLine.ProductID,
		}
		line.Calculate()
		existing.Lines = append(existing.Lines, line)
	}

	// Calculate totals
	existing.Calculate()

	// Validate
	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update via repository
	if err := s.repo.Update(ctx, schemaName, existing); err != nil {
		return nil, fmt.Errorf("update quote: %w", err)
	}

	return existing, nil
}

// Send marks a quote as sent
func (s *Service) Send(ctx context.Context, tenantID, schemaName, quoteID string) error {
	quote, err := s.repo.GetByID(ctx, schemaName, tenantID, quoteID)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}
	if quote.Status != QuoteStatusDraft {
		return fmt.Errorf("quote not in draft status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, quoteID, QuoteStatusSent); err != nil {
		return fmt.Errorf("send quote: %w", err)
	}
	return nil
}

// Accept marks a quote as accepted by the customer
func (s *Service) Accept(ctx context.Context, tenantID, schemaName, quoteID string) error {
	quote, err := s.repo.GetByID(ctx, schemaName, tenantID, quoteID)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}
	if quote.Status != QuoteStatusSent && quote.Status != QuoteStatusDraft {
		return fmt.Errorf("quote cannot be accepted in current status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, quoteID, QuoteStatusAccepted); err != nil {
		return fmt.Errorf("accept quote: %w", err)
	}
	return nil
}

// Reject marks a quote as rejected by the customer
func (s *Service) Reject(ctx context.Context, tenantID, schemaName, quoteID string) error {
	quote, err := s.repo.GetByID(ctx, schemaName, tenantID, quoteID)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}
	if quote.Status != QuoteStatusSent && quote.Status != QuoteStatusDraft {
		return fmt.Errorf("quote cannot be rejected in current status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, quoteID, QuoteStatusRejected); err != nil {
		return fmt.Errorf("reject quote: %w", err)
	}
	return nil
}

// Delete deletes a draft quote
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, quoteID string) error {
	if err := s.repo.Delete(ctx, schemaName, tenantID, quoteID); err != nil {
		return fmt.Errorf("delete quote: %w", err)
	}
	return nil
}

// ConvertToOrder marks a quote as converted to an order
func (s *Service) ConvertToOrder(ctx context.Context, tenantID, schemaName, quoteID, orderID string) error {
	if err := s.repo.SetConvertedToOrder(ctx, schemaName, tenantID, quoteID, orderID); err != nil {
		return fmt.Errorf("convert to order: %w", err)
	}
	return nil
}

// ConvertToInvoice marks a quote as converted to an invoice
func (s *Service) ConvertToInvoice(ctx context.Context, tenantID, schemaName, quoteID, invoiceID string) error {
	if err := s.repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quoteID, invoiceID); err != nil {
		return fmt.Errorf("convert to invoice: %w", err)
	}
	return nil
}
