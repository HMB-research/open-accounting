//go:build gorm

package recurring

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM recurring repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// EnsureSchema creates the recurring invoice tables if they don't exist
// Note: Uses raw SQL as GORM AutoMigrate is not suitable for dynamic schema names
func (r *GORMRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf(`
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
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			send_email_on_generation BOOLEAN DEFAULT false,
			email_template_type VARCHAR(50) DEFAULT 'INVOICE_SEND',
			recipient_email_override TEXT,
			attach_pdf_to_email BOOLEAN DEFAULT true,
			email_subject_override TEXT,
			email_message TEXT
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
	`, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName)

	return r.db.WithContext(ctx).Exec(query).Error
}

// Create inserts a new recurring invoice
func (r *GORMRepository) Create(ctx context.Context, schemaName string, ri *RecurringInvoice) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	riModel := recurringInvoiceToModel(ri)
	if err := db.Create(riModel).Error; err != nil {
		return fmt.Errorf("create recurring invoice: %w", err)
	}
	return nil
}

// CreateLine inserts a recurring invoice line
func (r *GORMRepository) CreateLine(ctx context.Context, schemaName string, line *RecurringInvoiceLine) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	lineModel := recurringInvoiceLineToModel(line)
	if err := db.Create(lineModel).Error; err != nil {
		return fmt.Errorf("create recurring invoice line: %w", err)
	}
	return nil
}

// GetByID retrieves a recurring invoice by ID (without lines)
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, id string) (*RecurringInvoice, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	// Use raw query to join with contacts for contact_name
	var result struct {
		models.RecurringInvoice
		ContactName string
	}

	err := db.Raw(`
		SELECT r.*, COALESCE(c.name, '') as contact_name
		FROM recurring_invoices r
		LEFT JOIN contacts c ON r.contact_id = c.id
		WHERE r.id = ? AND r.tenant_id = ?
	`, id, tenantID).Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("get recurring invoice: %w", err)
	}
	if result.ID == "" {
		return nil, ErrRecurringInvoiceNotFound
	}

	ri := modelToRecurringInvoice(&result.RecurringInvoice)
	ri.ContactName = result.ContactName
	return ri, nil
}

// GetLines retrieves lines for a recurring invoice
func (r *GORMRepository) GetLines(ctx context.Context, schemaName, recurringInvoiceID string) ([]RecurringInvoiceLine, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var lineModels []models.RecurringInvoiceLine
	if err := db.Where("recurring_invoice_id = ?", recurringInvoiceID).
		Order("line_number").
		Find(&lineModels).Error; err != nil {
		return nil, fmt.Errorf("get recurring invoice lines: %w", err)
	}

	lines := make([]RecurringInvoiceLine, len(lineModels))
	for i, lm := range lineModels {
		lines[i] = *modelToRecurringInvoiceLine(&lm)
	}

	return lines, nil
}

// List retrieves all recurring invoices for a tenant
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]RecurringInvoice, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	// Use raw query to join with contacts for contact_name
	query := `
		SELECT r.*, COALESCE(c.name, '') as contact_name
		FROM recurring_invoices r
		LEFT JOIN contacts c ON r.contact_id = c.id
		WHERE r.tenant_id = ?
	`
	if activeOnly {
		query += " AND r.is_active = true"
	}
	query += " ORDER BY r.next_generation_date, r.name"

	var results []struct {
		models.RecurringInvoice
		ContactName string
	}

	if err := db.Raw(query, tenantID).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("list recurring invoices: %w", err)
	}

	invoices := make([]RecurringInvoice, len(results))
	for i, res := range results {
		invoices[i] = *modelToRecurringInvoice(&res.RecurringInvoice)
		invoices[i].ContactName = res.ContactName
	}

	return invoices, nil
}

// Update updates a recurring invoice
func (r *GORMRepository) Update(ctx context.Context, schemaName string, ri *RecurringInvoice) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.RecurringInvoice{}).
		Where("id = ? AND tenant_id = ?", ri.ID, ri.TenantID).
		Updates(map[string]interface{}{
			"name":                     ri.Name,
			"contact_id":               ri.ContactID,
			"frequency":                ri.Frequency,
			"end_date":                 ri.EndDate,
			"payment_terms_days":       ri.PaymentTermsDays,
			"reference":                ri.Reference,
			"notes":                    ri.Notes,
			"updated_at":               ri.UpdatedAt,
			"send_email_on_generation": ri.SendEmailOnGeneration,
			"email_template_type":      ri.EmailTemplateType,
			"recipient_email_override": ri.RecipientEmailOverride,
			"attach_pdf_to_email":      ri.AttachPDFToEmail,
			"email_subject_override":   ri.EmailSubjectOverride,
			"email_message":            ri.EmailMessage,
		})
	if result.Error != nil {
		return fmt.Errorf("update recurring invoice: %w", result.Error)
	}
	return nil
}

// DeleteLines deletes all lines for a recurring invoice
func (r *GORMRepository) DeleteLines(ctx context.Context, schemaName, recurringInvoiceID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	if err := db.Where("recurring_invoice_id = ?", recurringInvoiceID).
		Delete(&models.RecurringInvoiceLine{}).Error; err != nil {
		return fmt.Errorf("delete recurring invoice lines: %w", err)
	}
	return nil
}

// Delete deletes a recurring invoice
func (r *GORMRepository) Delete(ctx context.Context, schemaName, tenantID, id string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.RecurringInvoice{})
	if result.Error != nil {
		return fmt.Errorf("delete recurring invoice: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRecurringInvoiceNotFound
	}
	return nil
}

// SetActive sets the active status of a recurring invoice
func (r *GORMRepository) SetActive(ctx context.Context, schemaName, tenantID, id string, active bool) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.RecurringInvoice{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{
			"is_active":  active,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("set active: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRecurringInvoiceNotFound
	}
	return nil
}

// GetDueRecurringInvoiceIDs returns IDs of recurring invoices due for generation
func (r *GORMRepository) GetDueRecurringInvoiceIDs(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]string, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var ids []string
	err := db.Model(&models.RecurringInvoice{}).
		Select("id").
		Where("tenant_id = ?", tenantID).
		Where("is_active = ?", true).
		Where("next_generation_date <= ?", asOfDate).
		Where("end_date IS NULL OR end_date >= ?", asOfDate).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("get due recurring invoice IDs: %w", err)
	}

	return ids, nil
}

// UpdateAfterGeneration updates generation tracking fields
func (r *GORMRepository) UpdateAfterGeneration(ctx context.Context, schemaName, tenantID, id string, nextDate time.Time, generatedAt time.Time) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	err := db.Model(&models.RecurringInvoice{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{
			"next_generation_date": nextDate,
			"last_generated_at":    generatedAt,
			"updated_at":           generatedAt,
		}).
		Update("generated_count", gorm.Expr("generated_count + 1")).Error
	if err != nil {
		return fmt.Errorf("update after generation: %w", err)
	}
	return nil
}

// UpdateInvoiceEmailStatus updates the invoice with email delivery status
func (r *GORMRepository) UpdateInvoiceEmailStatus(ctx context.Context, schemaName, invoiceID string, sentAt *time.Time, status, logID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	// Update the invoices table directly
	err := db.Table("invoices").
		Where("id = ?", invoiceID).
		Updates(map[string]interface{}{
			"last_email_sent_at": sentAt,
			"last_email_status":  status,
			"last_email_log_id":  logID,
		}).Error
	if err != nil {
		return fmt.Errorf("update invoice email status: %w", err)
	}
	return nil
}

// Conversion helpers

func recurringInvoiceToModel(ri *RecurringInvoice) *models.RecurringInvoice {
	return &models.RecurringInvoice{
		ID:                     ri.ID,
		TenantID:               ri.TenantID,
		Name:                   ri.Name,
		ContactID:              ri.ContactID,
		InvoiceType:            ri.InvoiceType,
		Currency:               ri.Currency,
		Frequency:              models.Frequency(ri.Frequency),
		StartDate:              ri.StartDate,
		EndDate:                ri.EndDate,
		NextGenerationDate:     ri.NextGenerationDate,
		PaymentTermsDays:       ri.PaymentTermsDays,
		Reference:              ri.Reference,
		Notes:                  ri.Notes,
		IsActive:               ri.IsActive,
		LastGeneratedAt:        ri.LastGeneratedAt,
		GeneratedCount:         ri.GeneratedCount,
		CreatedAt:              ri.CreatedAt,
		CreatedBy:              ri.CreatedBy,
		UpdatedAt:              ri.UpdatedAt,
		SendEmailOnGeneration:  ri.SendEmailOnGeneration,
		EmailTemplateType:      ri.EmailTemplateType,
		RecipientEmailOverride: ri.RecipientEmailOverride,
		AttachPDFToEmail:       ri.AttachPDFToEmail,
		EmailSubjectOverride:   ri.EmailSubjectOverride,
		EmailMessage:           ri.EmailMessage,
	}
}

func modelToRecurringInvoice(m *models.RecurringInvoice) *RecurringInvoice {
	return &RecurringInvoice{
		ID:                     m.ID,
		TenantID:               m.TenantID,
		Name:                   m.Name,
		ContactID:              m.ContactID,
		InvoiceType:            m.InvoiceType,
		Currency:               m.Currency,
		Frequency:              Frequency(m.Frequency),
		StartDate:              m.StartDate,
		EndDate:                m.EndDate,
		NextGenerationDate:     m.NextGenerationDate,
		PaymentTermsDays:       m.PaymentTermsDays,
		Reference:              m.Reference,
		Notes:                  m.Notes,
		IsActive:               m.IsActive,
		LastGeneratedAt:        m.LastGeneratedAt,
		GeneratedCount:         m.GeneratedCount,
		CreatedAt:              m.CreatedAt,
		CreatedBy:              m.CreatedBy,
		UpdatedAt:              m.UpdatedAt,
		SendEmailOnGeneration:  m.SendEmailOnGeneration,
		EmailTemplateType:      m.EmailTemplateType,
		RecipientEmailOverride: m.RecipientEmailOverride,
		AttachPDFToEmail:       m.AttachPDFToEmail,
		EmailSubjectOverride:   m.EmailSubjectOverride,
		EmailMessage:           m.EmailMessage,
	}
}

func recurringInvoiceLineToModel(line *RecurringInvoiceLine) *models.RecurringInvoiceLine {
	return &models.RecurringInvoiceLine{
		ID:                 line.ID,
		RecurringInvoiceID: line.RecurringInvoiceID,
		LineNumber:         line.LineNumber,
		Description:        line.Description,
		Quantity:           models.Decimal{Decimal: line.Quantity},
		Unit:               line.Unit,
		UnitPrice:          models.Decimal{Decimal: line.UnitPrice},
		DiscountPercent:    models.Decimal{Decimal: line.DiscountPercent},
		VATRate:            models.Decimal{Decimal: line.VATRate},
		AccountID:          line.AccountID,
		ProductID:          line.ProductID,
	}
}

func modelToRecurringInvoiceLine(m *models.RecurringInvoiceLine) *RecurringInvoiceLine {
	return &RecurringInvoiceLine{
		ID:                 m.ID,
		RecurringInvoiceID: m.RecurringInvoiceID,
		LineNumber:         m.LineNumber,
		Description:        m.Description,
		Quantity:           m.Quantity.Decimal,
		Unit:               m.Unit,
		UnitPrice:          m.UnitPrice.Decimal,
		DiscountPercent:    m.DiscountPercent.Decimal,
		VATRate:            m.VATRate.Decimal,
		AccountID:          m.AccountID,
		ProductID:          m.ProductID,
	}
}

// Ensure GORMRepository implements Repository interface
var _ Repository = (*GORMRepository)(nil)
