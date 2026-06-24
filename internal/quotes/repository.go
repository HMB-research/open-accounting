package quotes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
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

var errQuotesRepositoryDatabaseNotConfigured = errors.New("quotes repository database is not configured")

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create quotes GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errQuotesRepositoryDatabaseNotConfigured
	}
	return r.db.WithContext(ctx), nil
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return database.TenantTable(db, schemaName, tableName)
}

// Create inserts a new quote with its lines
func (r *GORMRepository) Create(ctx context.Context, schemaName string, quote *Quote) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		quotesTable, err := database.TenantTable(tx, schemaName, "quotes")
		if err != nil {
			return fmt.Errorf("qualify quotes table: %w", err)
		}
		if err := quotesTable.Create(quoteToModel(quote)).Error; err != nil {
			return fmt.Errorf("insert quote: %w", err)
		}

		if len(quote.Lines) == 0 {
			return nil
		}

		linesTable, err := database.TenantTable(tx, schemaName, "quote_lines")
		if err != nil {
			return fmt.Errorf("qualify quote lines table: %w", err)
		}
		lineModels := make([]models.QuoteLine, len(quote.Lines))
		for i := range quote.Lines {
			quote.Lines[i].QuoteID = quote.ID
			lineModels[i] = *quoteLineToModel(&quote.Lines[i])
		}
		if err := linesTable.Create(&lineModels).Error; err != nil {
			return fmt.Errorf("insert quote line: %w", err)
		}
		return nil
	})
}

// GetByID retrieves a quote by ID with its lines
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, quoteID string) (*Quote, error) {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return nil, fmt.Errorf("qualify quotes table: %w", err)
	}

	var quoteModel models.Quote
	err = db.Where("id = ? AND tenant_id = ?", quoteID, tenantID).First(&quoteModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrQuoteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}

	quote := quoteFromModel(&quoteModel)
	lines, err := r.listQuoteLines(ctx, schemaName, tenantID, quoteID)
	if err != nil {
		return nil, err
	}
	quote.Lines = lines
	return quote, nil
}

// List retrieves quotes with optional filtering
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter *QuoteFilter) ([]Quote, error) {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return nil, fmt.Errorf("qualify quotes table: %w", err)
	}

	query := db.Where("tenant_id = ?", tenantID)
	if filter != nil {
		if filter.Status != "" {
			query = query.Where("status = ?", string(filter.Status))
		}
		if filter.ContactID != "" {
			query = query.Where("contact_id = ?", filter.ContactID)
		}
		if filter.FromDate != nil {
			query = query.Where("quote_date >= ?", filter.FromDate)
		}
		if filter.ToDate != nil {
			query = query.Where("quote_date <= ?", filter.ToDate)
		}
		if strings.TrimSpace(filter.Search) != "" {
			query = query.Where("quote_number ILIKE ?", "%"+strings.TrimSpace(filter.Search)+"%")
		}
	}

	var quoteModels []models.Quote
	if err := query.
		Order("quote_date DESC").
		Order("quote_number DESC").
		Find(&quoteModels).Error; err != nil {
		return nil, fmt.Errorf("list quotes: %w", err)
	}

	quotes := make([]Quote, len(quoteModels))
	for i := range quoteModels {
		quotes[i] = *quoteFromModel(&quoteModels[i])
	}
	return quotes, nil
}

// Update updates a quote and its lines
func (r *GORMRepository) Update(ctx context.Context, schemaName string, quote *Quote) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		quotesTable, err := database.TenantTable(tx, schemaName, "quotes")
		if err != nil {
			return fmt.Errorf("qualify quotes table: %w", err)
		}
		result := quotesTable.Where("id = ? AND tenant_id = ? AND status = ?", quote.ID, quote.TenantID, string(QuoteStatusDraft)).
			Updates(map[string]interface{}{
				"contact_id":    quote.ContactID,
				"quote_date":    quote.QuoteDate,
				"valid_until":   quote.ValidUntil,
				"currency":      quote.Currency,
				"exchange_rate": quote.ExchangeRate.String(),
				"subtotal":      quote.Subtotal.String(),
				"vat_amount":    quote.VATAmount.String(),
				"total":         quote.Total.String(),
				"notes":         quote.Notes,
				"updated_at":    time.Now(),
			})
		if result.Error != nil {
			return fmt.Errorf("update quote: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrQuoteNotFound
		}

		linesTable, err := database.TenantTable(tx, schemaName, "quote_lines")
		if err != nil {
			return fmt.Errorf("qualify quote lines table: %w", err)
		}
		if err := linesTable.Where("quote_id = ?", quote.ID).Delete(&models.QuoteLine{}).Error; err != nil {
			return fmt.Errorf("delete quote lines: %w", err)
		}
		if len(quote.Lines) == 0 {
			return nil
		}

		lineModels := make([]models.QuoteLine, len(quote.Lines))
		for i := range quote.Lines {
			quote.Lines[i].QuoteID = quote.ID
			lineModels[i] = *quoteLineToModel(&quote.Lines[i])
		}
		if err := linesTable.Create(&lineModels).Error; err != nil {
			return fmt.Errorf("insert quote line: %w", err)
		}
		return nil
	})
}

// UpdateStatus updates the status of a quote
func (r *GORMRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, quoteID string, status QuoteStatus) error {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return fmt.Errorf("qualify quotes table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", quoteID, tenantID).
		Updates(map[string]interface{}{
			"status":     string(status),
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

// Delete removes a quote (only drafts)
func (r *GORMRepository) Delete(ctx context.Context, schemaName, tenantID, quoteID string) error {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return fmt.Errorf("qualify quotes table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ? AND status = ?", quoteID, tenantID, string(QuoteStatusDraft)).
		Delete(&models.Quote{})
	if result.Error != nil {
		return fmt.Errorf("delete quote: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

// GenerateNumber generates a new quote number
func (r *GORMRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return "", fmt.Errorf("qualify quotes table: %w", err)
	}

	var seq int
	if err := db.
		Select(`
			COALESCE(MAX(
				CASE
					WHEN quote_number ~ ? THEN CAST(SUBSTRING(quote_number FROM ?) AS INTEGER)
					ELSE 0
				END
			), 0) + 1
		`, "Q-[0-9]+$", "Q-([0-9]+)$").
		Where("tenant_id = ?", tenantID).
		Scan(&seq).Error; err != nil {
		return "", fmt.Errorf("generate quote number: %w", err)
	}
	return fmt.Sprintf("Q-%05d", seq), nil
}

// SetConvertedToOrder marks a quote as converted to an order
func (r *GORMRepository) SetConvertedToOrder(ctx context.Context, schemaName, tenantID, quoteID, orderID string) error {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return fmt.Errorf("qualify quotes table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", quoteID, tenantID).
		Updates(map[string]interface{}{
			"status":                string(QuoteStatusConverted),
			"converted_to_order_id": orderID,
			"updated_at":            time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("set converted to order: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

// SetConvertedToInvoice marks a quote as converted to an invoice
func (r *GORMRepository) SetConvertedToInvoice(ctx context.Context, schemaName, tenantID, quoteID, invoiceID string) error {
	db, err := r.tenantTable(ctx, schemaName, "quotes")
	if err != nil {
		return fmt.Errorf("qualify quotes table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", quoteID, tenantID).
		Updates(map[string]interface{}{
			"status":                  string(QuoteStatusConverted),
			"converted_to_invoice_id": invoiceID,
			"updated_at":              time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("set converted to invoice: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrQuoteNotFound
	}
	return nil
}

func (r *GORMRepository) listQuoteLines(ctx context.Context, schemaName, tenantID, quoteID string) ([]QuoteLine, error) {
	db, err := r.tenantTable(ctx, schemaName, "quote_lines")
	if err != nil {
		return nil, fmt.Errorf("qualify quote lines table: %w", err)
	}

	var lineModels []models.QuoteLine
	if err := db.
		Where("quote_id = ? AND tenant_id = ?", quoteID, tenantID).
		Order("line_number ASC").
		Find(&lineModels).Error; err != nil {
		return nil, fmt.Errorf("get quote lines: %w", err)
	}

	lines := make([]QuoteLine, len(lineModels))
	for i := range lineModels {
		lines[i] = *quoteLineFromModel(&lineModels[i])
	}
	return lines, nil
}

func quoteToModel(quote *Quote) *models.Quote {
	return &models.Quote{
		ID:                   quote.ID,
		TenantID:             quote.TenantID,
		QuoteNumber:          quote.QuoteNumber,
		ContactID:            quote.ContactID,
		QuoteDate:            quote.QuoteDate,
		ValidUntil:           quote.ValidUntil,
		Status:               string(quote.Status),
		Currency:             quote.Currency,
		ExchangeRate:         models.Decimal{Decimal: quote.ExchangeRate},
		Subtotal:             models.Decimal{Decimal: quote.Subtotal},
		VATAmount:            models.Decimal{Decimal: quote.VATAmount},
		Total:                models.Decimal{Decimal: quote.Total},
		Notes:                quote.Notes,
		ConvertedToOrderID:   quote.ConvertedToOrderID,
		ConvertedToInvoiceID: quote.ConvertedToInvoiceID,
		CreatedAt:            quote.CreatedAt,
		CreatedBy:            quote.CreatedBy,
		UpdatedAt:            quote.UpdatedAt,
	}
}

func quoteFromModel(quote *models.Quote) *Quote {
	return &Quote{
		ID:                   quote.ID,
		TenantID:             quote.TenantID,
		QuoteNumber:          quote.QuoteNumber,
		ContactID:            quote.ContactID,
		QuoteDate:            quote.QuoteDate,
		ValidUntil:           quote.ValidUntil,
		Status:               QuoteStatus(quote.Status),
		Currency:             quote.Currency,
		ExchangeRate:         quote.ExchangeRate.Decimal,
		Subtotal:             quote.Subtotal.Decimal,
		VATAmount:            quote.VATAmount.Decimal,
		Total:                quote.Total.Decimal,
		Notes:                quote.Notes,
		ConvertedToOrderID:   quote.ConvertedToOrderID,
		ConvertedToInvoiceID: quote.ConvertedToInvoiceID,
		CreatedAt:            quote.CreatedAt,
		CreatedBy:            quote.CreatedBy,
		UpdatedAt:            quote.UpdatedAt,
	}
}

func quoteLineToModel(line *QuoteLine) *models.QuoteLine {
	return &models.QuoteLine{
		ID:              line.ID,
		TenantID:        line.TenantID,
		QuoteID:         line.QuoteID,
		LineNumber:      line.LineNumber,
		Description:     line.Description,
		Quantity:        models.Decimal{Decimal: line.Quantity},
		Unit:            line.Unit,
		UnitPrice:       models.Decimal{Decimal: line.UnitPrice},
		DiscountPercent: models.Decimal{Decimal: line.DiscountPercent},
		VATRate:         models.Decimal{Decimal: line.VATRate},
		LineSubtotal:    models.Decimal{Decimal: line.LineSubtotal},
		LineVAT:         models.Decimal{Decimal: line.LineVAT},
		LineTotal:       models.Decimal{Decimal: line.LineTotal},
		ProductID:       line.ProductID,
	}
}

func quoteLineFromModel(line *models.QuoteLine) *QuoteLine {
	return &QuoteLine{
		ID:              line.ID,
		TenantID:        line.TenantID,
		QuoteID:         line.QuoteID,
		LineNumber:      line.LineNumber,
		Description:     line.Description,
		Quantity:        line.Quantity.Decimal,
		Unit:            line.Unit,
		UnitPrice:       line.UnitPrice.Decimal,
		DiscountPercent: line.DiscountPercent.Decimal,
		VATRate:         line.VATRate.Decimal,
		LineSubtotal:    line.LineSubtotal.Decimal,
		LineVAT:         line.LineVAT.Decimal,
		LineTotal:       line.LineTotal.Decimal,
		ProductID:       line.ProductID,
	}
}
