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

// MockRepository for testing
type MockRepository struct {
	JournalEntries    []JournalEntryWithLines
	CashBalance       decimal.Decimal
	GetEntriesErr     error
	GetCashBalanceErr error
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		JournalEntries: make([]JournalEntryWithLines, 0),
		CashBalance:    decimal.Zero,
	}
}

// GetJournalEntriesForPeriod returns mock journal entries
func (m *MockRepository) GetJournalEntriesForPeriod(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]JournalEntryWithLines, error) {
	if m.GetEntriesErr != nil {
		return nil, m.GetEntriesErr
	}

	// Filter by date range
	var result []JournalEntryWithLines
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
