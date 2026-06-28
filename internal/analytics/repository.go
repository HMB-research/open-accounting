package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var migrationSettlementPaymentMethods = []string{"CUTOVER_SETTLEMENT", "MIGRATION_SETTLEMENT"}

// Repository defines the contract for analytics data access
type Repository interface {
	// Summary queries
	GetRevenueExpenses(ctx context.Context, schemaName string, start, end time.Time) (revenue, expenses decimal.Decimal, err error)
	GetReceivablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error)
	GetPayablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error)
	GetInvoiceCounts(ctx context.Context, schemaName string) (draft, pending, overdue int, err error)

	// Chart queries
	GetMonthlyRevenueExpenses(ctx context.Context, schemaName string, months int) ([]MonthlyData, error)
	GetMonthlyCashFlow(ctx context.Context, schemaName string, months int) ([]MonthlyCashFlowData, error)

	// Aging queries
	GetAgingByContact(ctx context.Context, schemaName, invoiceType string) ([]ContactAging, error)

	// Top items
	GetTopCustomers(ctx context.Context, schemaName string, limit int) ([]TopItem, error)

	// Activity feed
	GetRecentActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error)
}

// MonthlyData represents monthly financial data
type MonthlyData struct {
	Label    string
	Revenue  decimal.Decimal
	Expenses decimal.Decimal
}

// MonthlyCashFlowData represents monthly cash flow data
type MonthlyCashFlowData struct {
	Label    string
	Inflows  decimal.Decimal
	Outflows decimal.Decimal
}

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

var newAnalyticsGormDBFromPool = database.NewGormDBFromPool

func NewRepository(pool *pgxpool.Pool) *GORMRepository {
	if pool == nil {
		return &GORMRepository{}
	}
	gormDB, err := newAnalyticsGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create analytics GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName, alias string) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("analytics repository database is not configured")
	}
	qualifiedTable, err := database.QualifiedTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	if alias != "" {
		qualifiedTable += " AS " + alias
	}
	return r.db.WithContext(ctx).Table(qualifiedTable), nil
}

func qualifiedTenantTable(schemaName, tableName string) string {
	quotedSchema, _ := database.QuoteIdentifier(schemaName)
	quotedTable, _ := database.QuoteIdentifier(tableName)
	return quotedSchema + "." + quotedTable
}

// GetRevenueExpenses retrieves revenue and expenses for a period
func (r *GORMRepository) GetRevenueExpenses(ctx context.Context, schemaName string, start, end time.Time) (decimal.Decimal, decimal.Decimal, error) {
	db, err := r.tenantTable(ctx, schemaName, "journal_entry_lines", "jel")
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("qualify journal entry lines table: %w", err)
	}
	journalEntriesTable := qualifiedTenantTable(schemaName, "journal_entries")
	accountsTable := qualifiedTenantTable(schemaName, "accounts")

	var row struct {
		Revenue  decimal.Decimal
		Expenses decimal.Decimal
	}
	if err := db.
		Select(`
			COALESCE(SUM(CASE WHEN a.account_type = ? THEN jel.base_credit - jel.base_debit ELSE 0 END), 0) AS revenue,
			COALESCE(SUM(CASE WHEN a.account_type = ? THEN jel.base_debit - jel.base_credit ELSE 0 END), 0) AS expenses
		`, models.AccountTypeRevenue, models.AccountTypeExpense).
		Joins("JOIN "+journalEntriesTable+" AS je ON jel.journal_entry_id = je.id").
		Joins("JOIN "+accountsTable+" AS a ON jel.account_id = a.id").
		Where("je.status = ?", models.JournalStatusPosted).
		Where("je.entry_date >= ? AND je.entry_date <= ?", start, end).
		Scan(&row).Error; err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("get revenue expenses: %w", err)
	}
	return row.Revenue, row.Expenses, nil
}

// GetReceivablesSummary retrieves receivables totals
func (r *GORMRepository) GetReceivablesSummary(ctx context.Context, schemaName string) (decimal.Decimal, decimal.Decimal, error) {
	return r.invoiceBalanceSummary(ctx, schemaName, models.InvoiceTypeSales)
}

// GetPayablesSummary retrieves payables totals
func (r *GORMRepository) GetPayablesSummary(ctx context.Context, schemaName string) (decimal.Decimal, decimal.Decimal, error) {
	return r.invoiceBalanceSummary(ctx, schemaName, models.InvoiceTypePurchase)
}

func (r *GORMRepository) invoiceBalanceSummary(ctx context.Context, schemaName string, invoiceType models.InvoiceType) (decimal.Decimal, decimal.Decimal, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoices", "")
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("qualify invoices table: %w", err)
	}

	var row struct {
		Total   decimal.Decimal
		Overdue decimal.Decimal
	}
	if err := db.
		Select(`
			COALESCE(SUM(total - amount_paid), 0) AS total,
			COALESCE(SUM(CASE WHEN due_date < CURRENT_DATE THEN total - amount_paid ELSE 0 END), 0) AS overdue
		`).
		Where("invoice_type = ?", invoiceType).
		Where("status NOT IN ?", []models.InvoiceStatus{models.InvoiceStatusPaid, models.InvoiceStatusVoided}).
		Scan(&row).Error; err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("get invoice balance summary: %w", err)
	}
	return row.Total, row.Overdue, nil
}

// GetInvoiceCounts retrieves invoice counts by status
func (r *GORMRepository) GetInvoiceCounts(ctx context.Context, schemaName string) (int, int, int, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoices", "")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("qualify invoices table: %w", err)
	}

	var row struct {
		Draft   int
		Pending int
		Overdue int
	}
	if err := db.
		Select(`
			COUNT(*) FILTER (WHERE status = ?) AS draft,
			COUNT(*) FILTER (WHERE status IN (?, ?)) AS pending,
			COUNT(*) FILTER (WHERE status NOT IN (?, ?) AND due_date < CURRENT_DATE) AS overdue
		`,
			models.InvoiceStatusDraft,
			models.InvoiceStatusSent,
			models.InvoiceStatusPartiallyPaid,
			models.InvoiceStatusPaid,
			models.InvoiceStatusVoided,
		).
		Where("invoice_type = ?", models.InvoiceTypeSales).
		Scan(&row).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("get invoice counts: %w", err)
	}
	return row.Draft, row.Pending, row.Overdue, nil
}

// GetMonthlyRevenueExpenses retrieves monthly revenue and expense data
func (r *GORMRepository) GetMonthlyRevenueExpenses(ctx context.Context, schemaName string, months int) ([]MonthlyData, error) {
	monthStarts := recentMonthStarts(months)
	if len(monthStarts) == 0 {
		return []MonthlyData{}, nil
	}

	db, err := r.tenantTable(ctx, schemaName, "journal_entry_lines", "jel")
	if err != nil {
		return nil, fmt.Errorf("qualify journal entry lines table: %w", err)
	}
	journalEntriesTable := qualifiedTenantTable(schemaName, "journal_entries")
	accountsTable := qualifiedTenantTable(schemaName, "accounts")

	var rows []struct {
		Month    time.Time
		Revenue  decimal.Decimal
		Expenses decimal.Decimal
	}
	if err := db.
		Select(`
			date_trunc('month', je.entry_date)::date AS month,
			COALESCE(SUM(CASE WHEN a.account_type = ? THEN jel.base_credit - jel.base_debit ELSE 0 END), 0) AS revenue,
			COALESCE(SUM(CASE WHEN a.account_type = ? THEN jel.base_debit - jel.base_credit ELSE 0 END), 0) AS expenses
		`, models.AccountTypeRevenue, models.AccountTypeExpense).
		Joins("JOIN "+journalEntriesTable+" AS je ON jel.journal_entry_id = je.id").
		Joins("JOIN "+accountsTable+" AS a ON jel.account_id = a.id").
		Where("je.status = ?", models.JournalStatusPosted).
		Where("je.entry_date >= ? AND je.entry_date < ?", monthStarts[0], monthStarts[len(monthStarts)-1].AddDate(0, 1, 0)).
		Group("date_trunc('month', je.entry_date)").
		Order("month ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get monthly revenue expenses: %w", err)
	}

	byMonth := make(map[string]MonthlyData, len(rows))
	for _, row := range rows {
		byMonth[monthKey(row.Month)] = MonthlyData{
			Label:    monthLabel(row.Month),
			Revenue:  row.Revenue,
			Expenses: row.Expenses,
		}
	}

	results := make([]MonthlyData, 0, len(monthStarts))
	for _, month := range monthStarts {
		data, ok := byMonth[monthKey(month)]
		if !ok {
			data = MonthlyData{Label: monthLabel(month), Revenue: decimal.Zero, Expenses: decimal.Zero}
		}
		results = append(results, data)
	}
	return results, nil
}

// GetMonthlyCashFlow retrieves monthly cash flow data
func (r *GORMRepository) GetMonthlyCashFlow(ctx context.Context, schemaName string, months int) ([]MonthlyCashFlowData, error) {
	monthStarts := recentMonthStarts(months)
	if len(monthStarts) == 0 {
		return []MonthlyCashFlowData{}, nil
	}

	bankDB, err := r.tenantTable(ctx, schemaName, "bank_transactions", "bt")
	if err != nil {
		return nil, fmt.Errorf("qualify bank transactions table: %w", err)
	}

	type monthlyCashFlowRow struct {
		Month    time.Time
		Inflows  decimal.Decimal
		Outflows decimal.Decimal
	}
	var bankRows []monthlyCashFlowRow
	if err := bankDB.
		Select(`
			date_trunc('month', bt.transaction_date)::date AS month,
			COALESCE(SUM(CASE WHEN bt.amount > 0 THEN bt.amount ELSE 0 END), 0) AS inflows,
			COALESCE(SUM(CASE WHEN bt.amount < 0 THEN -bt.amount ELSE 0 END), 0) AS outflows
		`).
		Where("bt.transaction_date >= ? AND bt.transaction_date < ?", monthStarts[0], monthStarts[len(monthStarts)-1].AddDate(0, 1, 0)).
		Group("date_trunc('month', bt.transaction_date)").
		Order("month ASC").
		Scan(&bankRows).Error; err != nil {
		return nil, fmt.Errorf("get monthly bank cash flow: %w", err)
	}

	paymentsDB, err := r.tenantTable(ctx, schemaName, "payments", "p")
	if err != nil {
		return nil, fmt.Errorf("qualify payments table: %w", err)
	}

	var paymentRows []monthlyCashFlowRow
	if err := paymentsDB.
		Select(`
			date_trunc('month', p.payment_date)::date AS month,
			COALESCE(SUM(CASE WHEN p.payment_type = ? THEN p.base_amount ELSE 0 END), 0) AS inflows,
			COALESCE(SUM(CASE WHEN p.payment_type = ? THEN p.base_amount ELSE 0 END), 0) AS outflows
		`, models.PaymentTypeReceived, models.PaymentTypeMade).
		Where("p.payment_date >= ? AND p.payment_date < ?", monthStarts[0], monthStarts[len(monthStarts)-1].AddDate(0, 1, 0)).
		Where("COALESCE(NULLIF(UPPER(TRIM(p.payment_method)), ''), '') NOT IN ?", migrationSettlementPaymentMethods).
		Group("date_trunc('month', p.payment_date)").
		Order("month ASC").
		Scan(&paymentRows).Error; err != nil {
		return nil, fmt.Errorf("get monthly cash flow: %w", err)
	}

	byMonth := make(map[string]MonthlyCashFlowData, len(bankRows)+len(paymentRows))
	for _, row := range paymentRows {
		byMonth[monthKey(row.Month)] = MonthlyCashFlowData{
			Label:    monthLabel(row.Month),
			Inflows:  row.Inflows,
			Outflows: row.Outflows,
		}
	}
	for _, row := range bankRows {
		byMonth[monthKey(row.Month)] = MonthlyCashFlowData{
			Label:    monthLabel(row.Month),
			Inflows:  row.Inflows,
			Outflows: row.Outflows,
		}
	}

	results := make([]MonthlyCashFlowData, 0, len(monthStarts))
	for _, month := range monthStarts {
		data, ok := byMonth[monthKey(month)]
		if !ok {
			data = MonthlyCashFlowData{Label: monthLabel(month), Inflows: decimal.Zero, Outflows: decimal.Zero}
		}
		results = append(results, data)
	}
	return results, nil
}

// GetAgingByContact retrieves aging data grouped by contact
func (r *GORMRepository) GetAgingByContact(ctx context.Context, schemaName, invoiceType string) ([]ContactAging, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}
	contactsTable := qualifiedTenantTable(schemaName, "contacts")

	var rows []struct {
		ContactID   string
		ContactName string
		Current     decimal.Decimal
		Days1to30   decimal.Decimal `gorm:"column:days_1_30"`
		Days31to60  decimal.Decimal `gorm:"column:days_31_60"`
		Days61to90  decimal.Decimal `gorm:"column:days_61_90"`
		Days90Plus  decimal.Decimal `gorm:"column:days_90_plus"`
	}
	if err := db.
		Select(`
			c.id AS contact_id,
			c.name AS contact_name,
			COALESCE(SUM(CASE WHEN i.due_date >= CURRENT_DATE THEN i.total - i.amount_paid ELSE 0 END), 0) AS current,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE AND i.due_date >= CURRENT_DATE - 30 THEN i.total - i.amount_paid ELSE 0 END), 0) AS days_1_30,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE - 30 AND i.due_date >= CURRENT_DATE - 60 THEN i.total - i.amount_paid ELSE 0 END), 0) AS days_31_60,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE - 60 AND i.due_date >= CURRENT_DATE - 90 THEN i.total - i.amount_paid ELSE 0 END), 0) AS days_61_90,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE - 90 THEN i.total - i.amount_paid ELSE 0 END), 0) AS days_90_plus
		`).
		Joins("JOIN "+contactsTable+" AS c ON i.contact_id = c.id").
		Where("i.invoice_type = ?", invoiceType).
		Where("i.status NOT IN ?", []models.InvoiceStatus{models.InvoiceStatusPaid, models.InvoiceStatusVoided}).
		Group("c.id, c.name").
		Having("SUM(i.total - i.amount_paid) > 0").
		Order("SUM(i.total - i.amount_paid) DESC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get aging by contact: %w", err)
	}

	results := make([]ContactAging, 0, len(rows))
	for _, row := range rows {
		item := ContactAging{
			ContactID:   row.ContactID,
			ContactName: row.ContactName,
			Current:     row.Current,
			Days1to30:   row.Days1to30,
			Days31to60:  row.Days31to60,
			Days61to90:  row.Days61to90,
			Days90Plus:  row.Days90Plus,
		}
		item.Total = item.Current.Add(item.Days1to30).Add(item.Days31to60).Add(item.Days61to90).Add(item.Days90Plus)
		results = append(results, item)
	}
	return results, nil
}

// GetTopCustomers retrieves top customers by revenue
func (r *GORMRepository) GetTopCustomers(ctx context.Context, schemaName string, limit int) ([]TopItem, error) {
	if limit <= 0 {
		return []TopItem{}, nil
	}

	db, err := r.tenantTable(ctx, schemaName, "contacts", "c")
	if err != nil {
		return nil, fmt.Errorf("qualify contacts table: %w", err)
	}
	invoicesTable := qualifiedTenantTable(schemaName, "invoices")

	var items []TopItem
	if err := db.
		Select(`
			c.id AS id,
			c.name AS name,
			COALESCE(SUM(i.total), 0) AS amount,
			COUNT(i.id) AS count
		`).
		Joins("LEFT JOIN "+invoicesTable+" AS i ON i.contact_id = c.id AND i.invoice_type = ? AND i.status != ?", models.InvoiceTypeSales, models.InvoiceStatusVoided).
		Where("c.contact_type IN ?", []string{"CUSTOMER", "BOTH"}).
		Group("c.id, c.name").
		Order("amount DESC").
		Limit(limit).
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get top customers: %w", err)
	}
	return items, nil
}

// GetRecentActivity retrieves recent activity from invoices, payments, journal entries, and contacts
func (r *GORMRepository) GetRecentActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		return []ActivityItem{}, nil
	}

	items := make([]ActivityItem, 0, limit*4)
	invoiceItems, err := r.recentInvoiceActivity(ctx, schemaName, limit)
	if err != nil {
		return nil, err
	}
	items = append(items, invoiceItems...)

	paymentItems, err := r.recentPaymentActivity(ctx, schemaName, limit)
	if err != nil {
		return nil, err
	}
	items = append(items, paymentItems...)

	entryItems, err := r.recentJournalEntryActivity(ctx, schemaName, limit)
	if err != nil {
		return nil, err
	}
	items = append(items, entryItems...)

	contactItems, err := r.recentContactActivity(ctx, schemaName, limit)
	if err != nil {
		return nil, err
	}
	items = append(items, contactItems...)

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *GORMRepository) recentInvoiceActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	db, err := r.tenantTable(ctx, schemaName, "invoices", "i")
	if err != nil {
		return nil, fmt.Errorf("qualify invoices table: %w", err)
	}
	contactsTable := qualifiedTenantTable(schemaName, "contacts")

	var rows []struct {
		ID            string
		InvoiceType   models.InvoiceType
		InvoiceNumber string
		Status        models.InvoiceStatus
		ContactName   string
		CreatedAt     time.Time
		Amount        decimal.Decimal
	}
	if err := db.
		Select(`
			i.id::text AS id,
			i.invoice_type AS invoice_type,
			i.invoice_number AS invoice_number,
			i.status AS status,
			COALESCE(c.name, 'Unknown') AS contact_name,
			i.created_at AS created_at,
			i.total AS amount
		`).
		Joins("LEFT JOIN " + contactsTable + " AS c ON i.contact_id = c.id").
		Order("i.created_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get recent invoice activity: %w", err)
	}

	items := make([]ActivityItem, 0, len(rows))
	for _, row := range rows {
		amount := row.Amount
		prefix := "Bill"
		direction := "from"
		if row.InvoiceType == models.InvoiceTypeSales {
			prefix = "Invoice"
			direction = "to"
		}
		items = append(items, ActivityItem{
			ID:          row.ID,
			Type:        "INVOICE",
			Action:      invoiceActivityAction(row.Status),
			Description: fmt.Sprintf("%s %s %s %s", prefix, row.InvoiceNumber, direction, row.ContactName),
			CreatedAt:   row.CreatedAt,
			Amount:      &amount,
		})
	}
	return items, nil
}

func (r *GORMRepository) recentPaymentActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	db, err := r.tenantTable(ctx, schemaName, "payments", "p")
	if err != nil {
		return nil, fmt.Errorf("qualify payments table: %w", err)
	}
	contactsTable := qualifiedTenantTable(schemaName, "contacts")

	var rows []struct {
		ID          string
		PaymentType models.PaymentType
		ContactName string
		CreatedAt   time.Time
		Amount      decimal.Decimal
	}
	if err := db.
		Select(`
			p.id::text AS id,
			p.payment_type AS payment_type,
			COALESCE(c.name, 'Unknown') AS contact_name,
			p.payment_date::timestamptz AS created_at,
			p.amount AS amount
		`).
		Joins("LEFT JOIN " + contactsTable + " AS c ON p.contact_id = c.id").
		Order("p.payment_date DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get recent payment activity: %w", err)
	}

	items := make([]ActivityItem, 0, len(rows))
	for _, row := range rows {
		amount := row.Amount
		action := "made"
		description := "Payment made to " + row.ContactName
		if row.PaymentType == models.PaymentTypeReceived {
			action = "received"
			description = "Payment received from " + row.ContactName
		}
		items = append(items, ActivityItem{
			ID:          row.ID,
			Type:        "PAYMENT",
			Action:      action,
			Description: description,
			CreatedAt:   row.CreatedAt,
			Amount:      &amount,
		})
	}
	return items, nil
}

func (r *GORMRepository) recentJournalEntryActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	db, err := r.tenantTable(ctx, schemaName, "journal_entries", "je")
	if err != nil {
		return nil, fmt.Errorf("qualify journal entries table: %w", err)
	}
	linesTable := qualifiedTenantTable(schemaName, "journal_entry_lines")

	var rows []struct {
		ID        string
		Status    models.JournalEntryStatus
		Label     string
		CreatedAt time.Time
		Amount    decimal.Decimal
	}
	if err := db.
		Select(`
			je.id::text AS id,
			je.status AS status,
			COALESCE(je.description, je.reference) AS label,
			je.created_at AS created_at,
			COALESCE(SUM(jel.base_debit), 0) AS amount
		`).
		Joins("LEFT JOIN " + linesTable + " AS jel ON jel.journal_entry_id = je.id").
		Where("je.description IS NOT NULL OR je.reference IS NOT NULL").
		Group("je.id, je.status, je.description, je.reference, je.created_at").
		Order("je.created_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get recent journal entry activity: %w", err)
	}

	items := make([]ActivityItem, 0, len(rows))
	for _, row := range rows {
		amount := row.Amount
		action := "created"
		if row.Status == models.JournalStatusPosted {
			action = "posted"
		}
		items = append(items, ActivityItem{
			ID:          row.ID,
			Type:        "ENTRY",
			Action:      action,
			Description: "Journal entry: " + row.Label,
			CreatedAt:   row.CreatedAt,
			Amount:      &amount,
		})
	}
	return items, nil
}

func (r *GORMRepository) recentContactActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	db, err := r.tenantTable(ctx, schemaName, "contacts", "c")
	if err != nil {
		return nil, fmt.Errorf("qualify contacts table: %w", err)
	}

	var rows []struct {
		ID        string
		Name      string
		CreatedAt time.Time
	}
	if err := db.
		Select("c.id::text AS id, c.name AS name, c.created_at AS created_at").
		Order("c.created_at DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get recent contact activity: %w", err)
	}

	items := make([]ActivityItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ActivityItem{
			ID:          row.ID,
			Type:        "CONTACT",
			Action:      "created",
			Description: "New contact: " + row.Name,
			CreatedAt:   row.CreatedAt,
		})
	}
	return items, nil
}

func invoiceActivityAction(status models.InvoiceStatus) string {
	switch status {
	case models.InvoiceStatusDraft:
		return "created"
	case models.InvoiceStatusSent:
		return "sent"
	case models.InvoiceStatusPaid:
		return "paid"
	case models.InvoiceStatusVoided:
		return "voided"
	default:
		return "updated"
	}
}

func recentMonthStarts(months int) []time.Time {
	if months <= 0 {
		return []time.Time{}
	}
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	firstMonth := currentMonth.AddDate(0, -(months - 1), 0)

	result := make([]time.Time, 0, months)
	for i := 0; i < months; i++ {
		result = append(result, firstMonth.AddDate(0, i, 0))
	}
	return result
}

func monthKey(month time.Time) string {
	return month.Format("2006-01")
}

func monthLabel(month time.Time) string {
	return month.Format("Jan 2006")
}

func IsMigrationSettlementPaymentMethod(method string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	for _, candidate := range migrationSettlementPaymentMethods {
		if normalized == candidate {
			return true
		}
	}
	return false
}
