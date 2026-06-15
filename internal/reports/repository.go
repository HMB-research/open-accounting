package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Repository defines the interface for report data access
type Repository interface {
	// GetJournalEntriesForPeriod retrieves journal entries within a date range
	GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error)

	// GetCashAccountBalance gets balance of cash accounts at a specific date
	GetCashAccountBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (decimal.Decimal, error)

	// GetOutstandingInvoicesByContact retrieves unpaid invoices grouped by contact
	GetOutstandingInvoicesByContact(ctx context.Context, schemaName, tenantID string, invoiceType string, asOfDate time.Time) ([]ContactBalance, error)

	// GetContactInvoices retrieves outstanding invoices for a specific contact
	GetContactInvoices(ctx context.Context, schemaName, tenantID, contactID string, invoiceType string, asOfDate time.Time) ([]BalanceInvoice, error)

	// GetContact retrieves contact details
	GetContact(ctx context.Context, schemaName, tenantID, contactID string) (ContactInfo, error)

	// GetContactStatementOpeningBalance retrieves the opening balance before a statement period
	GetContactStatementOpeningBalance(ctx context.Context, schemaName, tenantID, contactID, invoiceType, paymentType string, startDate time.Time) (decimal.Decimal, error)

	// GetContactStatementEntries retrieves invoice and payment activity for a statement period
	GetContactStatementEntries(ctx context.Context, schemaName, tenantID, contactID, invoiceType, paymentType string, startDate, endDate time.Time) ([]ContactStatementEntry, error)

	// GetSalesMarginLines retrieves sales invoice lines with estimated product costs
	GetSalesMarginLines(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]SalesMarginLine, error)

	// GetCashFlowMappingOverrides retrieves tenant-level cash-flow account mappings.
	GetCashFlowMappingOverrides(ctx context.Context, tenantID string) (CashFlowMappingOverrides, error)

	// UpdateCashFlowMappingOverrides replaces tenant-level cash-flow account mappings.
	UpdateCashFlowMappingOverrides(ctx context.Context, tenantID string, mapping CashFlowMappingOverrides) (CashFlowMappingOverrides, error)
}

// ContactInfo holds basic contact information for reports
type ContactInfo struct {
	ID    string
	Name  string
	Code  string
	Email string
}

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates an ORM-backed reports repository.
func NewGORMRepository(pool *pgxpool.Pool) *GORMRepository {
	if pool == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create reports GORM repository: %w", err))
	}
	return &GORMRepository{db: gormDB}
}

func (r *GORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("reports repository database is not configured")
	}
	return r.db.WithContext(ctx), nil
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName, alias string) (*gorm.DB, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	qualifiedTable, err := qualifiedTenantTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	if alias != "" {
		qualifiedTable += " AS " + alias
	}
	return db.Table(qualifiedTable), nil
}

func qualifiedTenantTable(schemaName, tableName string) (string, error) {
	return database.QualifiedTable(schemaName, tableName)
}

// GetJournalEntriesForPeriod retrieves journal entries within a date range
func (r *GORMRepository) GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error) {
	journalEntries, err := r.tenantTable(ctx, schemaName, "journal_entries", "je")
	if err != nil {
		return nil, fmt.Errorf("qualify journal entries table: %w", err)
	}
	journalLinesTable, err := qualifiedTenantTable(schemaName, "journal_entry_lines")
	if err != nil {
		return nil, fmt.Errorf("qualify journal entry lines table: %w", err)
	}
	accountsTable, err := qualifiedTenantTable(schemaName, "accounts")
	if err != nil {
		return nil, fmt.Errorf("qualify accounts table: %w", err)
	}

	var rows []journalEntryLineRow
	if err := journalEntries.
		Select(`
			je.id,
			je.entry_date,
			je.description,
			a.code AS account_code,
			a.name AS account_name,
			a.account_type AS account_type,
			jl.debit_amount AS debit,
			jl.credit_amount AS credit
		`).
		Joins("JOIN "+journalLinesTable+" AS jl ON je.id = jl.journal_entry_id").
		Joins("JOIN "+accountsTable+" AS a ON jl.account_id = a.id").
		Where("je.tenant_id = ?", tenantID).
		Where("je.entry_date >= ? AND je.entry_date <= ?", startDate, endDate).
		Where("je.status = ?", models.JournalStatusPosted).
		Order("je.entry_date ASC, je.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query journal entries: %w", err)
	}

	entriesMap := make(map[string]*JournalEntryWithLines)
	entryOrder := make([]string, 0)
	for _, row := range rows {
		entry, ok := entriesMap[row.ID]
		if !ok {
			entry = &JournalEntryWithLines{
				ID:          row.ID,
				EntryDate:   row.EntryDate,
				Description: row.Description,
				Lines:       []JournalLine{},
			}
			entriesMap[row.ID] = entry
			entryOrder = append(entryOrder, row.ID)
		}

		entry.Lines = append(entry.Lines, JournalLine{
			AccountCode: row.AccountCode,
			AccountName: row.AccountName,
			AccountType: row.AccountType,
			Debit:       row.Debit.Decimal,
			Credit:      row.Credit.Decimal,
		})
	}

	result := make([]JournalEntryWithLines, 0, len(entryOrder))
	for _, id := range entryOrder {
		result = append(result, *entriesMap[id])
	}
	return result, nil
}

// GetCashAccountBalance gets balance of cash accounts at a specific date
func (r *GORMRepository) GetCashAccountBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (decimal.Decimal, error) {
	journalEntries, err := r.tenantTable(ctx, schemaName, "journal_entries", "je")
	if err != nil {
		return decimal.Zero, fmt.Errorf("qualify journal entries table: %w", err)
	}
	journalLinesTable, err := qualifiedTenantTable(schemaName, "journal_entry_lines")
	if err != nil {
		return decimal.Zero, fmt.Errorf("qualify journal entry lines table: %w", err)
	}
	accountsTable, err := qualifiedTenantTable(schemaName, "accounts")
	if err != nil {
		return decimal.Zero, fmt.Errorf("qualify accounts table: %w", err)
	}

	var row decimalRow
	if err := journalEntries.
		Select("COALESCE(SUM(jl.debit_amount - jl.credit_amount), 0) AS total").
		Joins("JOIN "+journalLinesTable+" AS jl ON je.id = jl.journal_entry_id").
		Joins("JOIN "+accountsTable+" AS a ON jl.account_id = a.id").
		Where("je.tenant_id = ?", tenantID).
		Where("je.entry_date <= ?", asOfDate).
		Where("je.status = ?", models.JournalStatusPosted).
		Where("a.code LIKE ?", "10%").
		Scan(&row).Error; err != nil {
		return decimal.Zero, fmt.Errorf("query cash balance: %w", err)
	}
	return row.Total.Decimal, nil
}

// GetOutstandingInvoicesByContact retrieves unpaid invoices grouped by contact
func (r *GORMRepository) GetOutstandingInvoicesByContact(ctx context.Context, schemaName, tenantID string, invoiceType string, asOfDate time.Time) ([]ContactBalance, error) {
	invoices, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}
	contactsTable, err := qualifiedTenantTable(schemaName, "contacts")
	if err != nil {
		return nil, fmt.Errorf("qualify contacts table: %w", err)
	}

	var rows []contactBalanceRow
	if err := invoices.
		Select(`
			c.id AS contact_id,
			c.name AS contact_name,
			COALESCE(c.code, '') AS contact_code,
			COALESCE(c.email, '') AS contact_email,
			COALESCE(SUM(i.total - i.amount_paid), 0) AS balance,
			COUNT(i.id) AS invoice_count,
			MIN(i.issue_date) AS oldest_invoice
		`).
		Joins("JOIN "+contactsTable+" AS c ON i.contact_id = c.id AND i.tenant_id = c.tenant_id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.invoice_type = ?", invoiceType).
		Where("i.status IN ?", balanceConfirmationInvoiceStatuses()).
		Where("i.issue_date <= ?", asOfDate).
		Where("(i.total - i.amount_paid) > 0").
		Group("c.id, c.name, c.code, c.email").
		Order("balance DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query outstanding invoices: %w", err)
	}

	contacts := make([]ContactBalance, 0, len(rows))
	for _, row := range rows {
		cb := ContactBalance{
			ContactID:    row.ContactID,
			ContactName:  row.ContactName,
			ContactCode:  row.ContactCode,
			ContactEmail: row.ContactEmail,
			Balance:      row.Balance.Decimal,
			InvoiceCount: row.InvoiceCount,
		}
		if row.OldestInvoice != nil {
			cb.OldestInvoice = row.OldestInvoice.Format("2006-01-02")
		}
		contacts = append(contacts, cb)
	}
	return contacts, nil
}

// GetContactInvoices retrieves outstanding invoices for a specific contact
func (r *GORMRepository) GetContactInvoices(ctx context.Context, schemaName, tenantID, contactID string, invoiceType string, asOfDate time.Time) ([]BalanceInvoice, error) {
	invoicesTable, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}

	var rows []balanceInvoiceRow
	if err := invoicesTable.
		Select(`
			i.id AS invoice_id,
			i.invoice_number,
			i.issue_date AS invoice_date,
			i.due_date,
			i.total AS total_amount,
			i.amount_paid,
			i.currency,
			GREATEST(0, (?::date - i.due_date)) AS days_overdue
		`, asOfDate).
		Where("i.tenant_id = ?", tenantID).
		Where("i.contact_id = ?", contactID).
		Where("i.invoice_type = ?", invoiceType).
		Where("i.status IN ?", balanceConfirmationInvoiceStatuses()).
		Where("i.issue_date <= ?", asOfDate).
		Where("(i.total - i.amount_paid) > 0").
		Order("i.issue_date ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query contact invoices: %w", err)
	}

	invoices := make([]BalanceInvoice, 0, len(rows))
	for _, row := range rows {
		invoices = append(invoices, BalanceInvoice{
			InvoiceID:         row.InvoiceID,
			InvoiceNumber:     row.InvoiceNumber,
			InvoiceDate:       row.InvoiceDate.Format("2006-01-02"),
			DueDate:           row.DueDate.Format("2006-01-02"),
			TotalAmount:       row.TotalAmount.Decimal,
			AmountPaid:        row.AmountPaid.Decimal,
			OutstandingAmount: row.TotalAmount.Sub(row.AmountPaid.Decimal),
			Currency:          row.Currency,
			DaysOverdue:       row.DaysOverdue,
		})
	}
	return invoices, nil
}

func balanceConfirmationInvoiceStatuses() []string {
	return []string{
		string(models.InvoiceStatusSent),
		string(models.InvoiceStatusPartiallyPaid),
		string(models.InvoiceStatusOverdue),
	}
}

func contactStatementInvoiceStatuses() []string {
	return []string{
		string(models.InvoiceStatusSent),
		string(models.InvoiceStatusPartiallyPaid),
		string(models.InvoiceStatusPaid),
		string(models.InvoiceStatusOverdue),
	}
}

// GetContact retrieves contact details
func (r *GORMRepository) GetContact(ctx context.Context, schemaName, tenantID, contactID string) (ContactInfo, error) {
	contactsTable, err := r.tenantTable(ctx, schemaName, "contacts", "c")
	if err != nil {
		return ContactInfo{}, fmt.Errorf("qualify contacts table: %w", err)
	}

	var contact ContactInfo
	err = contactsTable.
		Select("c.id, c.name, COALESCE(c.code, '') AS code, COALESCE(c.email, '') AS email").
		Where("c.id = ? AND c.tenant_id = ?", contactID, tenantID).
		Take(&contact).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ContactInfo{}, fmt.Errorf("contact not found")
	}
	if err != nil {
		return ContactInfo{}, fmt.Errorf("query contact: %w", err)
	}
	return contact, nil
}

// GetContactStatementOpeningBalance retrieves the opening balance before a statement period.
func (r *GORMRepository) GetContactStatementOpeningBalance(ctx context.Context, schemaName, tenantID, contactID, invoiceType, paymentType string, startDate time.Time) (decimal.Decimal, error) {
	invoiceTotal, err := r.sumInvoiceStatementAmountBefore(ctx, schemaName, tenantID, contactID, invoiceType, startDate)
	if err != nil {
		return decimal.Zero, err
	}
	paymentTotal, err := r.sumPaymentStatementAmountBefore(ctx, schemaName, tenantID, contactID, paymentType, startDate)
	if err != nil {
		return decimal.Zero, err
	}
	return invoiceTotal.Sub(paymentTotal), nil
}

func (r *GORMRepository) sumInvoiceStatementAmountBefore(ctx context.Context, schemaName, tenantID, contactID, invoiceType string, startDate time.Time) (decimal.Decimal, error) {
	invoicesTable, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return decimal.Zero, fmt.Errorf("qualify invoices table: %w", err)
	}

	var row decimalRow
	if err := invoicesTable.
		Select("COALESCE(SUM(i.base_total), 0) AS total").
		Where("i.tenant_id = ?", tenantID).
		Where("i.contact_id = ?", contactID).
		Where("i.invoice_type = ?", invoiceType).
		Where("i.status IN ?", contactStatementInvoiceStatuses()).
		Where("i.issue_date < ?", startDate).
		Scan(&row).Error; err != nil {
		return decimal.Zero, fmt.Errorf("query contact statement invoice opening balance: %w", err)
	}
	return row.Total.Decimal, nil
}

func (r *GORMRepository) sumPaymentStatementAmountBefore(ctx context.Context, schemaName, tenantID, contactID, paymentType string, startDate time.Time) (decimal.Decimal, error) {
	paymentsTable, err := r.tenantTable(ctx, schemaName, "payments", "p")
	if err != nil {
		return decimal.Zero, fmt.Errorf("qualify payments table: %w", err)
	}

	var row decimalRow
	if err := paymentsTable.
		Select("COALESCE(SUM(p.base_amount), 0) AS total").
		Where("p.tenant_id = ?", tenantID).
		Where("p.contact_id = ?", contactID).
		Where("p.payment_type = ?", paymentType).
		Where("p.payment_date < ?", startDate).
		Scan(&row).Error; err != nil {
		return decimal.Zero, fmt.Errorf("query contact statement payment opening balance: %w", err)
	}
	return row.Total.Decimal, nil
}

// GetContactStatementEntries retrieves invoice and payment activity for a statement period.
func (r *GORMRepository) GetContactStatementEntries(ctx context.Context, schemaName, tenantID, contactID, invoiceType, paymentType string, startDate, endDate time.Time) ([]ContactStatementEntry, error) {
	invoiceEntries, err := r.getContactStatementInvoiceEntries(ctx, schemaName, tenantID, contactID, invoiceType, startDate, endDate)
	if err != nil {
		return nil, err
	}
	paymentEntries, err := r.getContactStatementPaymentEntries(ctx, schemaName, tenantID, contactID, paymentType, startDate, endDate)
	if err != nil {
		return nil, err
	}

	entries := append(invoiceEntries, paymentEntries...)
	sortContactStatementEntries(entries)
	return entries, nil
}

func (r *GORMRepository) getContactStatementInvoiceEntries(ctx context.Context, schemaName, tenantID, contactID, invoiceType string, startDate, endDate time.Time) ([]ContactStatementEntry, error) {
	invoicesTable, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}

	var rows []statementInvoiceRow
	if err := invoicesTable.
		Select(`
			i.id AS document_id,
			i.invoice_number AS document_number,
			i.issue_date AS document_date,
			i.due_date,
			i.reference,
			i.notes,
			i.currency,
			i.total AS document_amount,
			i.base_total AS statement_amount
		`).
		Where("i.tenant_id = ?", tenantID).
		Where("i.contact_id = ?", contactID).
		Where("i.invoice_type = ?", invoiceType).
		Where("i.status IN ?", contactStatementInvoiceStatuses()).
		Where("i.issue_date >= ? AND i.issue_date <= ?", startDate, endDate).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query contact statement invoices: %w", err)
	}

	entries := make([]ContactStatementEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ContactStatementEntry{
			Date:            row.DocumentDate.Format("2006-01-02"),
			DocumentType:    "INVOICE",
			DocumentID:      row.DocumentID,
			DocumentNumber:  row.DocumentNumber,
			DueDate:         row.DueDate.Format("2006-01-02"),
			Description:     firstNonEmpty(row.Reference, row.Notes, row.DocumentNumber),
			Reference:       row.Reference,
			Currency:        row.Currency,
			DocumentAmount:  row.DocumentAmount.Decimal,
			StatementAmount: row.StatementAmount.Decimal,
		})
	}
	return entries, nil
}

func (r *GORMRepository) getContactStatementPaymentEntries(ctx context.Context, schemaName, tenantID, contactID, paymentType string, startDate, endDate time.Time) ([]ContactStatementEntry, error) {
	paymentsTable, err := r.tenantTable(ctx, schemaName, "payments", "p")
	if err != nil {
		return nil, fmt.Errorf("qualify payments table: %w", err)
	}

	var rows []statementPaymentRow
	if err := paymentsTable.
		Select(`
			p.id AS document_id,
			p.payment_number AS document_number,
			p.payment_date AS document_date,
			p.reference,
			p.notes,
			p.currency,
			p.amount AS document_amount,
			-p.base_amount AS statement_amount
		`).
		Where("p.tenant_id = ?", tenantID).
		Where("p.contact_id = ?", contactID).
		Where("p.payment_type = ?", paymentType).
		Where("p.payment_date >= ? AND p.payment_date <= ?", startDate, endDate).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query contact statement payments: %w", err)
	}

	entries := make([]ContactStatementEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ContactStatementEntry{
			Date:            row.DocumentDate.Format("2006-01-02"),
			DocumentType:    "PAYMENT",
			DocumentID:      row.DocumentID,
			DocumentNumber:  row.DocumentNumber,
			Description:     firstNonEmpty(row.Reference, row.Notes, row.DocumentNumber),
			Reference:       row.Reference,
			Currency:        row.Currency,
			DocumentAmount:  row.DocumentAmount.Decimal,
			StatementAmount: row.StatementAmount.Decimal,
		})
	}
	return entries, nil
}

// GetSalesMarginLines retrieves sales invoice lines with estimated product costs.
func (r *GORMRepository) GetSalesMarginLines(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]SalesMarginLine, error) {
	invoicesTable, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}
	invoiceLinesTable, err := qualifiedTenantTable(schemaName, "invoice_lines")
	if err != nil {
		return nil, fmt.Errorf("qualify invoice lines table: %w", err)
	}
	contactsTable, err := qualifiedTenantTable(schemaName, "contacts")
	if err != nil {
		return nil, fmt.Errorf("qualify contacts table: %w", err)
	}
	productsTable, err := qualifiedTenantTable(schemaName, "products")
	if err != nil {
		return nil, fmt.Errorf("qualify products table: %w", err)
	}

	var rows []salesMarginRow
	if err := invoicesTable.
		Select(`
			i.id AS invoice_id,
			i.invoice_number,
			i.issue_date AS invoice_date,
			i.contact_id,
			c.name AS contact_name,
			COALESCE(p.id::text, '') AS product_id,
			COALESCE(p.code, '') AS product_code,
			COALESCE(p.name, '') AS product_name,
			COALESCE(il.description, '') AS description,
			il.quantity,
			(il.line_subtotal * i.exchange_rate) AS revenue,
			COALESCE(p.purchase_price, 0) AS unit_cost,
			(il.quantity * COALESCE(p.purchase_price, 0)) AS cost
		`).
		Joins("JOIN "+invoiceLinesTable+" AS il ON il.invoice_id = i.id AND il.tenant_id = i.tenant_id").
		Joins("JOIN "+contactsTable+" AS c ON c.id = i.contact_id AND c.tenant_id = i.tenant_id").
		Joins("LEFT JOIN "+productsTable+" AS p ON p.id = il.product_id AND p.tenant_id = i.tenant_id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.invoice_type = ?", models.InvoiceTypeSales).
		Where("i.status <> ?", models.InvoiceStatusVoided).
		Where("i.issue_date >= ? AND i.issue_date <= ?", startDate, endDate).
		Order("i.issue_date ASC, i.invoice_number ASC, il.line_number ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query sales margin lines: %w", err)
	}

	lines := make([]SalesMarginLine, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, SalesMarginLine{
			InvoiceID:     row.InvoiceID,
			InvoiceNumber: row.InvoiceNumber,
			InvoiceDate:   row.InvoiceDate.Format("2006-01-02"),
			ContactID:     row.ContactID,
			ContactName:   row.ContactName,
			ProductID:     row.ProductID,
			ProductCode:   row.ProductCode,
			ProductName:   row.ProductName,
			Description:   row.Description,
			Quantity:      row.Quantity.Decimal,
			Revenue:       row.Revenue.Decimal,
			UnitCost:      row.UnitCost.Decimal,
			Cost:          row.Cost.Decimal,
		})
	}
	return lines, nil
}

// GetCashFlowMappingOverrides retrieves tenant-level cash-flow account mappings from tenant settings.
func (r *GORMRepository) GetCashFlowMappingOverrides(ctx context.Context, tenantID string) (CashFlowMappingOverrides, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return CashFlowMappingOverrides{}, err
	}

	var tenant models.Tenant
	err = db.Select("id", "settings").Where("id = ?", tenantID).Take(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CashFlowMappingOverrides{}, fmt.Errorf("tenant not found")
	}
	if err != nil {
		return CashFlowMappingOverrides{}, fmt.Errorf("query cash flow mapping: %w", err)
	}
	return cashFlowMappingFromSettings(tenant.Settings)
}

// UpdateCashFlowMappingOverrides replaces tenant-level cash-flow account mappings in tenant settings.
func (r *GORMRepository) UpdateCashFlowMappingOverrides(ctx context.Context, tenantID string, mapping CashFlowMappingOverrides) (CashFlowMappingOverrides, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return CashFlowMappingOverrides{}, err
	}

	var tenant models.Tenant
	err = db.Select("id", "settings").Where("id = ?", tenantID).Take(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CashFlowMappingOverrides{}, fmt.Errorf("tenant not found")
	}
	if err != nil {
		return CashFlowMappingOverrides{}, fmt.Errorf("query cash flow mapping: %w", err)
	}

	updatedSettings, err := settingsWithCashFlowMapping(tenant.Settings, mapping)
	if err != nil {
		return CashFlowMappingOverrides{}, err
	}
	result := db.Model(&models.Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{
			"settings":   updatedSettings,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return CashFlowMappingOverrides{}, fmt.Errorf("update cash flow mapping: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return CashFlowMappingOverrides{}, fmt.Errorf("tenant not found")
	}
	return mapping, nil
}

type journalEntryLineRow struct {
	ID          string         `gorm:"column:id"`
	EntryDate   time.Time      `gorm:"column:entry_date"`
	Description string         `gorm:"column:description"`
	AccountCode string         `gorm:"column:account_code"`
	AccountName string         `gorm:"column:account_name"`
	AccountType string         `gorm:"column:account_type"`
	Debit       models.Decimal `gorm:"column:debit"`
	Credit      models.Decimal `gorm:"column:credit"`
}

type decimalRow struct {
	Total models.Decimal `gorm:"column:total"`
}

type contactBalanceRow struct {
	ContactID     string         `gorm:"column:contact_id"`
	ContactName   string         `gorm:"column:contact_name"`
	ContactCode   string         `gorm:"column:contact_code"`
	ContactEmail  string         `gorm:"column:contact_email"`
	Balance       models.Decimal `gorm:"column:balance"`
	InvoiceCount  int            `gorm:"column:invoice_count"`
	OldestInvoice *time.Time     `gorm:"column:oldest_invoice"`
}

type balanceInvoiceRow struct {
	InvoiceID     string         `gorm:"column:invoice_id"`
	InvoiceNumber string         `gorm:"column:invoice_number"`
	InvoiceDate   time.Time      `gorm:"column:invoice_date"`
	DueDate       time.Time      `gorm:"column:due_date"`
	TotalAmount   models.Decimal `gorm:"column:total_amount"`
	AmountPaid    models.Decimal `gorm:"column:amount_paid"`
	Currency      string         `gorm:"column:currency"`
	DaysOverdue   int            `gorm:"column:days_overdue"`
}

type statementInvoiceRow struct {
	DocumentID      string         `gorm:"column:document_id"`
	DocumentNumber  string         `gorm:"column:document_number"`
	DocumentDate    time.Time      `gorm:"column:document_date"`
	DueDate         time.Time      `gorm:"column:due_date"`
	Reference       string         `gorm:"column:reference"`
	Notes           string         `gorm:"column:notes"`
	Currency        string         `gorm:"column:currency"`
	DocumentAmount  models.Decimal `gorm:"column:document_amount"`
	StatementAmount models.Decimal `gorm:"column:statement_amount"`
}

type statementPaymentRow struct {
	DocumentID      string         `gorm:"column:document_id"`
	DocumentNumber  string         `gorm:"column:document_number"`
	DocumentDate    time.Time      `gorm:"column:document_date"`
	Reference       string         `gorm:"column:reference"`
	Notes           string         `gorm:"column:notes"`
	Currency        string         `gorm:"column:currency"`
	DocumentAmount  models.Decimal `gorm:"column:document_amount"`
	StatementAmount models.Decimal `gorm:"column:statement_amount"`
}

type salesMarginRow struct {
	InvoiceID     string         `gorm:"column:invoice_id"`
	InvoiceNumber string         `gorm:"column:invoice_number"`
	InvoiceDate   time.Time      `gorm:"column:invoice_date"`
	ContactID     string         `gorm:"column:contact_id"`
	ContactName   string         `gorm:"column:contact_name"`
	ProductID     string         `gorm:"column:product_id"`
	ProductCode   string         `gorm:"column:product_code"`
	ProductName   string         `gorm:"column:product_name"`
	Description   string         `gorm:"column:description"`
	Quantity      models.Decimal `gorm:"column:quantity"`
	Revenue       models.Decimal `gorm:"column:revenue"`
	UnitCost      models.Decimal `gorm:"column:unit_cost"`
	Cost          models.Decimal `gorm:"column:cost"`
}

func cashFlowMappingFromSettings(settings json.RawMessage) (CashFlowMappingOverrides, error) {
	settingsMap, err := settingsMapFromRaw(settings)
	if err != nil {
		return CashFlowMappingOverrides{}, err
	}
	rawMapping, ok := settingsMap["cash_flow_mapping"]
	if !ok || string(rawMapping) == "null" {
		return CashFlowMappingOverrides{}, nil
	}

	var mapping CashFlowMappingOverrides
	if err := json.Unmarshal(rawMapping, &mapping); err != nil {
		return CashFlowMappingOverrides{}, fmt.Errorf("parse cash flow mapping: %w", err)
	}
	return mapping, nil
}

func settingsWithCashFlowMapping(settings json.RawMessage, mapping CashFlowMappingOverrides) (json.RawMessage, error) {
	settingsMap, err := settingsMapFromRaw(settings)
	if err != nil {
		return nil, err
	}
	rawMapping, err := json.Marshal(mapping)
	if err != nil {
		return nil, fmt.Errorf("marshal cash flow mapping: %w", err)
	}
	settingsMap["cash_flow_mapping"] = rawMapping

	updatedSettings, err := json.Marshal(settingsMap)
	if err != nil {
		return nil, fmt.Errorf("marshal tenant settings: %w", err)
	}
	return updatedSettings, nil
}

func settingsMapFromRaw(settings json.RawMessage) (map[string]json.RawMessage, error) {
	if len(settings) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	settingsMap := make(map[string]json.RawMessage)
	if err := json.Unmarshal(settings, &settingsMap); err != nil {
		return nil, fmt.Errorf("parse tenant settings: %w", err)
	}
	if settingsMap == nil {
		return map[string]json.RawMessage{}, nil
	}
	return settingsMap, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func sortContactStatementEntries(entries []ContactStatementEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.Date != right.Date {
			return left.Date < right.Date
		}
		if documentSortOrder(left.DocumentType) != documentSortOrder(right.DocumentType) {
			return documentSortOrder(left.DocumentType) < documentSortOrder(right.DocumentType)
		}
		if left.DocumentNumber != right.DocumentNumber {
			return left.DocumentNumber < right.DocumentNumber
		}
		return left.DocumentID < right.DocumentID
	})
}

func documentSortOrder(documentType string) int {
	if documentType == "INVOICE" {
		return 1
	}
	return 2
}

// MockRepository for testing
type MockRepository struct {
	JournalEntries                []JournalEntryWithLines
	CashBalance                   decimal.Decimal
	CashFlowMapping               CashFlowMappingOverrides
	ContactBalances               []ContactBalance
	ContactInvoices               []BalanceInvoice
	Contact                       ContactInfo
	ContactStatementOpening       decimal.Decimal
	ContactStatementEntries       []ContactStatementEntry
	SalesMarginLines              []SalesMarginLine
	GetEntriesErr                 error
	GetCashBalanceErr             error
	GetCashFlowMappingErr         error
	UpdateCashFlowMappingErr      error
	GetContactBalancesErr         error
	GetContactInvoicesErr         error
	GetContactErr                 error
	GetContactStatementOpeningErr error
	GetContactStatementEntriesErr error
	GetSalesMarginLinesErr        error
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		JournalEntries:          make([]JournalEntryWithLines, 0),
		CashBalance:             decimal.Zero,
		ContactBalances:         make([]ContactBalance, 0),
		ContactInvoices:         make([]BalanceInvoice, 0),
		ContactStatementEntries: make([]ContactStatementEntry, 0),
		SalesMarginLines:        make([]SalesMarginLine, 0),
	}
}

// GetJournalEntriesForPeriod returns mock journal entries
func (m *MockRepository) GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error) {
	if m.GetEntriesErr != nil {
		return nil, m.GetEntriesErr
	}

	// Filter by date range
	result := []JournalEntryWithLines{}
	for _, entry := range m.JournalEntries {
		if (entry.EntryDate.Equal(startDate) || entry.EntryDate.After(startDate)) &&
			(entry.EntryDate.Equal(endDate) || entry.EntryDate.Before(endDate)) {
			result = append(result, entry)
		}
	}
	return result, nil
}

// GetCashAccountBalance returns mock cash balance
func (m *MockRepository) GetCashAccountBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (decimal.Decimal, error) {
	if m.GetCashBalanceErr != nil {
		return decimal.Zero, m.GetCashBalanceErr
	}
	return m.CashBalance, nil
}

// GetCashFlowMappingOverrides returns mock tenant-level cash-flow mappings.
func (m *MockRepository) GetCashFlowMappingOverrides(ctx context.Context, tenantID string) (CashFlowMappingOverrides, error) {
	if m.GetCashFlowMappingErr != nil {
		return CashFlowMappingOverrides{}, m.GetCashFlowMappingErr
	}
	return m.CashFlowMapping, nil
}

// UpdateCashFlowMappingOverrides updates mock tenant-level cash-flow mappings.
func (m *MockRepository) UpdateCashFlowMappingOverrides(ctx context.Context, tenantID string, mapping CashFlowMappingOverrides) (CashFlowMappingOverrides, error) {
	if m.UpdateCashFlowMappingErr != nil {
		return CashFlowMappingOverrides{}, m.UpdateCashFlowMappingErr
	}
	m.CashFlowMapping = mapping
	return m.CashFlowMapping, nil
}

// GetOutstandingInvoicesByContact returns mock contact balances
func (m *MockRepository) GetOutstandingInvoicesByContact(ctx context.Context, schemaName, tenantID string, invoiceType string, asOfDate time.Time) ([]ContactBalance, error) {
	if m.GetContactBalancesErr != nil {
		return nil, m.GetContactBalancesErr
	}
	return m.ContactBalances, nil
}

// GetContactInvoices returns mock contact invoices
func (m *MockRepository) GetContactInvoices(ctx context.Context, schemaName, tenantID, contactID string, invoiceType string, asOfDate time.Time) ([]BalanceInvoice, error) {
	if m.GetContactInvoicesErr != nil {
		return nil, m.GetContactInvoicesErr
	}
	return m.ContactInvoices, nil
}

// GetContact returns mock contact info
func (m *MockRepository) GetContact(ctx context.Context, schemaName, tenantID, contactID string) (ContactInfo, error) {
	if m.GetContactErr != nil {
		return ContactInfo{}, m.GetContactErr
	}
	return m.Contact, nil
}

// GetContactStatementOpeningBalance returns mock contact statement opening balance.
func (m *MockRepository) GetContactStatementOpeningBalance(ctx context.Context, schemaName, tenantID, contactID, invoiceType, paymentType string, startDate time.Time) (decimal.Decimal, error) {
	if m.GetContactStatementOpeningErr != nil {
		return decimal.Zero, m.GetContactStatementOpeningErr
	}
	return m.ContactStatementOpening, nil
}

// GetContactStatementEntries returns mock contact statement entries.
func (m *MockRepository) GetContactStatementEntries(ctx context.Context, schemaName, tenantID, contactID, invoiceType, paymentType string, startDate, endDate time.Time) ([]ContactStatementEntry, error) {
	if m.GetContactStatementEntriesErr != nil {
		return nil, m.GetContactStatementEntriesErr
	}
	return m.ContactStatementEntries, nil
}

// GetSalesMarginLines returns mock sales margin lines.
func (m *MockRepository) GetSalesMarginLines(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]SalesMarginLine, error) {
	if m.GetSalesMarginLinesErr != nil {
		return nil, m.GetSalesMarginLinesErr
	}
	return m.SalesMarginLines, nil
}
