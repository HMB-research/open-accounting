//go:build gorm

package invoicing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM invoicing repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// Create inserts a new invoice with its lines
func (r *GORMRepository) Create(ctx context.Context, schemaName string, invoice *Invoice) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Insert invoice
		invModel := invoiceToModel(invoice)
		if err := tx.Create(invModel).Error; err != nil {
			return fmt.Errorf("insert invoice: %w", err)
		}

		// Insert lines
		for i := range invoice.Lines {
			line := &invoice.Lines[i]
			line.InvoiceID = invoice.ID

			lineModel := invoiceLineToModel(line)
			if err := tx.Create(lineModel).Error; err != nil {
				return fmt.Errorf("insert invoice line: %w", err)
			}
		}

		return nil
	})
}

// GetByID retrieves an invoice by ID with its lines
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, invoiceID string) (*Invoice, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var invModel models.Invoice
	err := db.Where("id = ? AND tenant_id = ?", invoiceID, tenantID).First(&invModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvoiceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get invoice: %w", err)
	}

	// Load lines
	var lineModels []models.InvoiceLine
	if err := db.Where("invoice_id = ? AND tenant_id = ?", invoiceID, tenantID).
		Order("line_number").
		Find(&lineModels).Error; err != nil {
		return nil, fmt.Errorf("get invoice lines: %w", err)
	}

	inv := modelToInvoice(&invModel)
	inv.Lines = make([]InvoiceLine, len(lineModels))
	for i, lm := range lineModels {
		inv.Lines[i] = *modelToInvoiceLine(&lm)
	}

	return inv, nil
}

// List retrieves invoices with optional filtering
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter *InvoiceFilter) ([]Invoice, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	query := db.Where("tenant_id = ?", tenantID)

	if filter != nil {
		if filter.InvoiceType != "" {
			query = query.Where("invoice_type = ?", filter.InvoiceType)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.ContactID != "" {
			query = query.Where("contact_id = ?", filter.ContactID)
		}
		if filter.FromDate != nil {
			query = query.Where("issue_date >= ?", filter.FromDate)
		}
		if filter.ToDate != nil {
			query = query.Where("issue_date <= ?", filter.ToDate)
		}
		if filter.Search != "" {
			searchPattern := "%" + filter.Search + "%"
			query = query.Where("invoice_number ILIKE ? OR reference ILIKE ?", searchPattern, searchPattern)
		}
	}

	query = query.Order("issue_date DESC, invoice_number DESC")

	var invModels []models.Invoice
	if err := query.Find(&invModels).Error; err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}

	invoices := make([]Invoice, len(invModels))
	for i, im := range invModels {
		invoices[i] = *modelToInvoice(&im)
	}

	return invoices, nil
}

// UpdateStatus updates the status of an invoice
func (r *GORMRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, invoiceID string, status InvoiceStatus) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.Invoice{}).
		Where("id = ? AND tenant_id = ?", invoiceID, tenantID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrInvoiceNotFound
	}
	return nil
}

// UpdatePayment updates the amount paid and status of an invoice
func (r *GORMRepository) UpdatePayment(ctx context.Context, schemaName, tenantID, invoiceID string, amountPaid decimal.Decimal, status InvoiceStatus) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.Invoice{}).
		Where("id = ? AND tenant_id = ?", invoiceID, tenantID).
		Updates(map[string]interface{}{
			"amount_paid": amountPaid.String(),
			"status":      status,
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update payment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrInvoiceNotFound
	}
	return nil
}

// GenerateNumber generates a new invoice number
func (r *GORMRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string, invoiceType InvoiceType) (string, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	prefix := "INV"
	if invoiceType == InvoiceTypePurchase {
		prefix = "BILL"
	} else if invoiceType == InvoiceTypeCreditNote {
		prefix = "CN"
	}

	var seq int
	err := db.Raw(fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(invoice_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM invoices WHERE tenant_id = ? AND invoice_type = ?
	`, prefix), tenantID, invoiceType).Scan(&seq).Error
	if err != nil {
		return "", fmt.Errorf("generate invoice number: %w", err)
	}

	return fmt.Sprintf("%s-%05d", prefix, seq), nil
}

// UpdateOverdueStatus updates the status of overdue invoices
func (r *GORMRepository) UpdateOverdueStatus(ctx context.Context, schemaName, tenantID string) (int, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.Invoice{}).
		Where("tenant_id = ?", tenantID).
		Where("status IN ?", []string{string(StatusSent), string(StatusPartiallyPaid)}).
		Where("due_date < ?", time.Now()).
		Where("amount_paid < total").
		Updates(map[string]interface{}{
			"status":     StatusOverdue,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("update overdue status: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}

// Conversion helpers between domain types and GORM models

func modelToInvoice(m *models.Invoice) *Invoice {
	return &Invoice{
		ID:             m.ID,
		TenantID:       m.TenantID,
		InvoiceNumber:  m.InvoiceNumber,
		InvoiceType:    InvoiceType(m.InvoiceType),
		ContactID:      m.ContactID,
		IssueDate:      m.IssueDate,
		DueDate:        m.DueDate,
		Currency:       m.Currency,
		ExchangeRate:   m.ExchangeRate.Decimal,
		Subtotal:       m.Subtotal.Decimal,
		VATAmount:      m.VATAmount.Decimal,
		Total:          m.Total.Decimal,
		BaseSubtotal:   m.BaseSubtotal.Decimal,
		BaseVATAmount:  m.BaseVATAmount.Decimal,
		BaseTotal:      m.BaseTotal.Decimal,
		AmountPaid:     m.AmountPaid.Decimal,
		Status:         InvoiceStatus(m.Status),
		Reference:      m.Reference,
		Notes:          m.Notes,
		JournalEntryID: m.JournalEntryID,
		EInvoiceSentAt: m.EInvoiceSentAt,
		EInvoiceID:     m.EInvoiceID,
		CreatedAt:      m.CreatedAt,
		CreatedBy:      m.CreatedBy,
		UpdatedAt:      m.UpdatedAt,
	}
}

func invoiceToModel(inv *Invoice) *models.Invoice {
	return &models.Invoice{
		ID:             inv.ID,
		TenantID:       inv.TenantID,
		InvoiceNumber:  inv.InvoiceNumber,
		InvoiceType:    models.InvoiceType(inv.InvoiceType),
		ContactID:      inv.ContactID,
		IssueDate:      inv.IssueDate,
		DueDate:        inv.DueDate,
		Currency:       inv.Currency,
		ExchangeRate:   models.Decimal{Decimal: inv.ExchangeRate},
		Subtotal:       models.Decimal{Decimal: inv.Subtotal},
		VATAmount:      models.Decimal{Decimal: inv.VATAmount},
		Total:          models.Decimal{Decimal: inv.Total},
		BaseSubtotal:   models.Decimal{Decimal: inv.BaseSubtotal},
		BaseVATAmount:  models.Decimal{Decimal: inv.BaseVATAmount},
		BaseTotal:      models.Decimal{Decimal: inv.BaseTotal},
		AmountPaid:     models.Decimal{Decimal: inv.AmountPaid},
		Status:         models.InvoiceStatus(inv.Status),
		Reference:      inv.Reference,
		Notes:          inv.Notes,
		JournalEntryID: inv.JournalEntryID,
		EInvoiceSentAt: inv.EInvoiceSentAt,
		EInvoiceID:     inv.EInvoiceID,
		CreatedAt:      inv.CreatedAt,
		CreatedBy:      inv.CreatedBy,
		UpdatedAt:      inv.UpdatedAt,
	}
}

func modelToInvoiceLine(m *models.InvoiceLine) *InvoiceLine {
	return &InvoiceLine{
		ID:              m.ID,
		TenantID:        m.TenantID,
		InvoiceID:       m.InvoiceID,
		LineNumber:      m.LineNumber,
		Description:     m.Description,
		Quantity:        m.Quantity.Decimal,
		Unit:            m.Unit,
		UnitPrice:       m.UnitPrice.Decimal,
		DiscountPercent: m.DiscountPercent.Decimal,
		VATRate:         m.VATRate.Decimal,
		LineSubtotal:    m.LineSubtotal.Decimal,
		LineVAT:         m.LineVAT.Decimal,
		LineTotal:       m.LineTotal.Decimal,
		AccountID:       m.AccountID,
		ProductID:       m.ProductID,
	}
}

func invoiceLineToModel(l *InvoiceLine) *models.InvoiceLine {
	return &models.InvoiceLine{
		ID:              l.ID,
		TenantID:        l.TenantID,
		InvoiceID:       l.InvoiceID,
		LineNumber:      l.LineNumber,
		Description:     l.Description,
		Quantity:        models.Decimal{Decimal: l.Quantity},
		Unit:            l.Unit,
		UnitPrice:       models.Decimal{Decimal: l.UnitPrice},
		DiscountPercent: models.Decimal{Decimal: l.DiscountPercent},
		VATRate:         models.Decimal{Decimal: l.VATRate},
		LineSubtotal:    models.Decimal{Decimal: l.LineSubtotal},
		LineVAT:         models.Decimal{Decimal: l.LineVAT},
		LineTotal:       models.Decimal{Decimal: l.LineTotal},
		AccountID:       l.AccountID,
		ProductID:       l.ProductID,
	}
}
