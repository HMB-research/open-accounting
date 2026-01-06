package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
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
}

// ContactInfo holds basic contact information for reports
type ContactInfo struct {
	ID    string
	Name  string
	Code  string
	Email string
}

// PostgresRepository implements Repository for PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// GetJournalEntriesForPeriod retrieves journal entries within a date range
func (r *PostgresRepository) GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error) {
	query := fmt.Sprintf(`
		SELECT
			je.id,
			je.entry_date,
			je.description,
			jl.account_code,
			a.name as account_name,
			a.type as account_type,
			jl.debit,
			jl.credit
		FROM %s.journal_entries je
		JOIN %s.journal_lines jl ON je.id = jl.journal_entry_id
		JOIN %s.accounts a ON jl.account_code = a.code
		WHERE je.tenant_id = $1
			AND je.entry_date >= $2
			AND je.entry_date <= $3
			AND je.status = 'POSTED'
		ORDER BY je.entry_date, je.id
	`, schemaName, schemaName, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query journal entries: %w", err)
	}
	defer rows.Close()

	entriesMap := make(map[string]*JournalEntryWithLines)

	for rows.Next() {
		var (
			id          string
			entryDate   time.Time
			description string
			accountCode string
			accountName string
			accountType string
			debit       decimal.Decimal
			credit      decimal.Decimal
		)

		if err := rows.Scan(&id, &entryDate, &description, &accountCode, &accountName, &accountType, &debit, &credit); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		entry, ok := entriesMap[id]
		if !ok {
			entry = &JournalEntryWithLines{
				ID:          id,
				EntryDate:   entryDate,
				Description: description,
				Lines:       []JournalLine{},
			}
			entriesMap[id] = entry
		}

		entry.Lines = append(entry.Lines, JournalLine{
			AccountCode: accountCode,
			AccountName: accountName,
			AccountType: accountType,
			Debit:       debit,
			Credit:      credit,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	result := make([]JournalEntryWithLines, 0, len(entriesMap))
	for _, entry := range entriesMap {
		result = append(result, *entry)
	}

	return result, nil
}

// GetCashAccountBalance gets balance of cash accounts at a specific date
func (r *PostgresRepository) GetCashAccountBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) (decimal.Decimal, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(jl.debit - jl.credit), 0) as balance
		FROM %s.journal_entries je
		JOIN %s.journal_lines jl ON je.id = jl.journal_entry_id
		WHERE je.tenant_id = $1
			AND je.entry_date <= $2
			AND je.status = 'POSTED'
			AND jl.account_code LIKE '10%%'
	`, schemaName, schemaName)

	var balance decimal.Decimal
	err := r.db.QueryRow(ctx, query, tenantID, asOfDate).Scan(&balance)
	if err != nil {
		return decimal.Zero, fmt.Errorf("query cash balance: %w", err)
	}

	return balance, nil
}

// GetOutstandingInvoicesByContact retrieves unpaid invoices grouped by contact
func (r *PostgresRepository) GetOutstandingInvoicesByContact(ctx context.Context, schemaName, tenantID string, invoiceType string, asOfDate time.Time) ([]ContactBalance, error) {
	query := fmt.Sprintf(`
		SELECT
			c.id as contact_id,
			c.name as contact_name,
			COALESCE(c.code, '') as contact_code,
			COALESCE(c.email, '') as contact_email,
			COALESCE(SUM(i.total - i.amount_paid), 0) as balance,
			COUNT(i.id) as invoice_count,
			MIN(i.issue_date) as oldest_invoice
		FROM %s.invoices i
		JOIN %s.contacts c ON i.contact_id = c.id
		WHERE i.tenant_id = $1
			AND i.invoice_type = $2
			AND i.status NOT IN ('PAID', 'VOIDED')
			AND i.issue_date <= $3
			AND (i.total - i.amount_paid) > 0
		GROUP BY c.id, c.name, c.code, c.email
		ORDER BY balance DESC
	`, schemaName, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID, invoiceType, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("query outstanding invoices: %w", err)
	}
	defer rows.Close()

	contacts := []ContactBalance{}
	for rows.Next() {
		var cb ContactBalance
		var oldestInvoice *time.Time
		if err := rows.Scan(&cb.ContactID, &cb.ContactName, &cb.ContactCode, &cb.ContactEmail, &cb.Balance, &cb.InvoiceCount, &oldestInvoice); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		if oldestInvoice != nil {
			cb.OldestInvoice = oldestInvoice.Format("2006-01-02")
		}
		contacts = append(contacts, cb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return contacts, nil
}

// GetContactInvoices retrieves outstanding invoices for a specific contact
func (r *PostgresRepository) GetContactInvoices(ctx context.Context, schemaName, tenantID, contactID string, invoiceType string, asOfDate time.Time) ([]BalanceInvoice, error) {
	query := fmt.Sprintf(`
		SELECT
			i.id,
			i.invoice_number,
			i.issue_date,
			i.due_date,
			i.total,
			i.amount_paid,
			i.currency,
			GREATEST(0, ($4::date - i.due_date)) as days_overdue
		FROM %s.invoices i
		WHERE i.tenant_id = $1
			AND i.contact_id = $2
			AND i.invoice_type = $3
			AND i.status NOT IN ('PAID', 'VOIDED')
			AND i.issue_date <= $4
			AND (i.total - i.amount_paid) > 0
		ORDER BY i.issue_date ASC
	`, schemaName)

	rows, err := r.db.Query(ctx, query, tenantID, contactID, invoiceType, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("query contact invoices: %w", err)
	}
	defer rows.Close()

	invoices := []BalanceInvoice{}
	for rows.Next() {
		var inv BalanceInvoice
		var invoiceDate, dueDate time.Time
		if err := rows.Scan(&inv.InvoiceID, &inv.InvoiceNumber, &invoiceDate, &dueDate, &inv.TotalAmount, &inv.AmountPaid, &inv.Currency, &inv.DaysOverdue); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		inv.InvoiceDate = invoiceDate.Format("2006-01-02")
		inv.DueDate = dueDate.Format("2006-01-02")
		inv.OutstandingAmount = inv.TotalAmount.Sub(inv.AmountPaid)
		invoices = append(invoices, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return invoices, nil
}

// GetContact retrieves contact details
func (r *PostgresRepository) GetContact(ctx context.Context, schemaName, tenantID, contactID string) (ContactInfo, error) {
	query := fmt.Sprintf(`
		SELECT id, name, COALESCE(code, ''), COALESCE(email, '')
		FROM %s.contacts
		WHERE id = $1 AND tenant_id = $2
	`, schemaName)

	var c ContactInfo
	err := r.db.QueryRow(ctx, query, contactID, tenantID).Scan(&c.ID, &c.Name, &c.Code, &c.Email)
	if err != nil {
		return ContactInfo{}, fmt.Errorf("query contact: %w", err)
	}

	return c, nil
}

// MockRepository for testing
type MockRepository struct {
	JournalEntries        []JournalEntryWithLines
	CashBalance           decimal.Decimal
	ContactBalances       []ContactBalance
	ContactInvoices       []BalanceInvoice
	Contact               ContactInfo
	GetEntriesErr         error
	GetCashBalanceErr     error
	GetContactBalancesErr error
	GetContactInvoicesErr error
	GetContactErr         error
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		JournalEntries:  make([]JournalEntryWithLines, 0),
		CashBalance:     decimal.Zero,
		ContactBalances: make([]ContactBalance, 0),
		ContactInvoices: make([]BalanceInvoice, 0),
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
