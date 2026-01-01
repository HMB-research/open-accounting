package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// RepositoryInterface defines the contract for accounting data access
type RepositoryInterface interface {
	GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*Account, error)
	ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Account, error)
	CreateAccount(ctx context.Context, schemaName string, a *Account) error
	GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error)
	CreateJournalEntry(ctx context.Context, schemaName string, je *JournalEntry) error
	CreateJournalEntryTx(ctx context.Context, schemaName string, tx pgx.Tx, je *JournalEntry) error
	UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status JournalEntryStatus, userID string) error
	GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error)
	GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error)
	GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]AccountBalance, error)
	VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *JournalEntry) error
}

// Repository provides access to accounting data
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new accounting repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetAccountByID retrieves an account by ID
func (r *Repository) GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*Account, error) {
	var a Account
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, code, name, account_type, parent_id, is_active, is_system, COALESCE(description, ''), created_at
		FROM %s.accounts
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), accountID, tenantID).Scan(
		&a.ID, &a.TenantID, &a.Code, &a.Name, &a.AccountType,
		&a.ParentID, &a.IsActive, &a.IsSystem, &a.Description, &a.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("account not found: %s", accountID)
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return &a, nil
}

// ListAccounts retrieves all accounts for a tenant
func (r *Repository) ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Account, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, code, name, account_type, parent_id, is_active, is_system, COALESCE(description, ''), created_at
		FROM %s.accounts
		WHERE tenant_id = $1
	`, schemaName)
	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY code"

	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.Code, &a.Name, &a.AccountType,
			&a.ParentID, &a.IsActive, &a.IsSystem, &a.Description, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// CreateAccount creates a new account
func (r *Repository) CreateAccount(ctx context.Context, schemaName string, a *Account) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}

	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.accounts (id, tenant_id, code, name, account_type, parent_id, is_active, is_system, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, schemaName), a.ID, a.TenantID, a.Code, a.Name, a.AccountType, a.ParentID, a.IsActive, a.IsSystem, a.Description, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

// GetJournalEntryByID retrieves a journal entry with its lines
func (r *Repository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*JournalEntry, error) {
	var je JournalEntry
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, entry_number, entry_date, description, reference, source_type, source_id,
		       status, posted_at, posted_by, voided_at, voided_by, COALESCE(void_reason, ''), created_at, created_by
		FROM %s.journal_entries
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), entryID, tenantID).Scan(
		&je.ID, &je.TenantID, &je.EntryNumber, &je.EntryDate, &je.Description, &je.Reference,
		&je.SourceType, &je.SourceID, &je.Status, &je.PostedAt, &je.PostedBy,
		&je.VoidedAt, &je.VoidedBy, &je.VoidReason, &je.CreatedAt, &je.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("journal entry not found: %s", entryID)
	}
	if err != nil {
		return nil, fmt.Errorf("get journal entry: %w", err)
	}

	// Load lines
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, journal_entry_id, account_id, COALESCE(description, ''),
		       debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit
		FROM %s.journal_entry_lines
		WHERE journal_entry_id = $1 AND tenant_id = $2
		ORDER BY id
	`, schemaName), entryID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get journal entry lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line JournalEntryLine
		if err := rows.Scan(
			&line.ID, &line.TenantID, &line.JournalEntryID, &line.AccountID, &line.Description,
			&line.DebitAmount, &line.CreditAmount, &line.Currency, &line.ExchangeRate,
			&line.BaseDebit, &line.BaseCredit,
		); err != nil {
			return nil, fmt.Errorf("scan journal entry line: %w", err)
		}
		je.Lines = append(je.Lines, line)
	}

	return &je, nil
}

// CreateJournalEntry creates a new journal entry with lines
func (r *Repository) CreateJournalEntry(ctx context.Context, schemaName string, je *JournalEntry) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.CreateJournalEntryTx(ctx, schemaName, tx, je); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CreateJournalEntryTx creates a journal entry within a transaction
func (r *Repository) CreateJournalEntryTx(ctx context.Context, schemaName string, tx pgx.Tx, je *JournalEntry) error {
	if je.ID == "" {
		je.ID = uuid.New().String()
	}
	if je.CreatedAt.IsZero() {
		je.CreatedAt = time.Now()
	}

	// Generate entry number
	var seq int
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(entry_number FROM 4) AS INTEGER)), 0) + 1
		FROM %s.journal_entries WHERE tenant_id = $1
	`, schemaName), je.TenantID).Scan(&seq)
	if err != nil {
		return fmt.Errorf("generate entry number: %w", err)
	}
	je.EntryNumber = fmt.Sprintf("JE-%05d", seq)

	// Insert entry
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.journal_entries (id, tenant_id, entry_number, entry_date, description, reference,
		                             source_type, source_id, status, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, schemaName), je.ID, je.TenantID, je.EntryNumber, je.EntryDate, je.Description, je.Reference,
		je.SourceType, je.SourceID, je.Status, je.CreatedAt, je.CreatedBy)
	if err != nil {
		return fmt.Errorf("insert journal entry: %w", err)
	}

	// Insert lines
	for i := range je.Lines {
		line := &je.Lines[i]
		if line.ID == "" {
			line.ID = uuid.New().String()
		}
		line.TenantID = je.TenantID
		line.JournalEntryID = je.ID

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, description,
			                                 debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, schemaName), line.ID, line.TenantID, line.JournalEntryID, line.AccountID, line.Description,
			line.DebitAmount, line.CreditAmount, line.Currency, line.ExchangeRate, line.BaseDebit, line.BaseCredit)
		if err != nil {
			return fmt.Errorf("insert journal entry line: %w", err)
		}
	}

	return nil
}

// UpdateJournalEntryStatus updates the status of a journal entry
func (r *Repository) UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status JournalEntryStatus, userID string) error {
	now := time.Now()
	var query string
	var args []interface{}

	switch status {
	case StatusPosted:
		query = fmt.Sprintf(`
			UPDATE %s.journal_entries
			SET status = $1, posted_at = $2, posted_by = $3
			WHERE id = $4 AND tenant_id = $5 AND status = $6
		`, schemaName)
		args = []interface{}{status, now, userID, entryID, tenantID, StatusDraft}
	case StatusVoided:
		return fmt.Errorf("use VoidJournalEntry method to void entries")
	default:
		return fmt.Errorf("invalid status transition to: %s", status)
	}

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update journal entry status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("entry not found or invalid status transition")
	}
	return nil
}

// GetAccountBalance retrieves the balance of an account as of a date
func (r *Repository) GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	var debitSum, creditSum decimal.Decimal
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(jel.debit_amount), 0), COALESCE(SUM(jel.credit_amount), 0)
		FROM %s.journal_entry_lines jel
		JOIN %s.journal_entries je ON je.id = jel.journal_entry_id
		WHERE jel.account_id = $1 AND jel.tenant_id = $2
		  AND je.entry_date <= $3 AND je.status = 'POSTED'
	`, schemaName, schemaName), accountID, tenantID, asOfDate).Scan(&debitSum, &creditSum)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get account balance: %w", err)
	}

	// Get account type to determine normal balance
	var accountType AccountType
	err = r.db.QueryRow(ctx, fmt.Sprintf(`SELECT account_type FROM %s.accounts WHERE id = $1 AND tenant_id = $2`, schemaName), accountID, tenantID).Scan(&accountType)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get account type: %w", err)
	}

	if accountType.IsDebitNormal() {
		return debitSum.Sub(creditSum), nil
	}
	return creditSum.Sub(debitSum), nil
}

// GetTrialBalance retrieves all account balances as of a date
func (r *Repository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]AccountBalance, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		WITH account_totals AS (
			SELECT
				a.id AS account_id,
				a.code AS account_code,
				a.name AS account_name,
				a.account_type,
				COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
				COALESCE(SUM(jel.credit_amount), 0) AS total_credits
			FROM %s.accounts a
			LEFT JOIN %s.journal_entry_lines jel ON jel.account_id = a.id AND jel.tenant_id = a.tenant_id
			LEFT JOIN %s.journal_entries je ON je.id = jel.journal_entry_id
			WHERE a.tenant_id = $1
			  AND (je.id IS NULL OR (je.entry_date <= $2 AND je.status = 'POSTED'))
			GROUP BY a.id, a.code, a.name, a.account_type
		)
		SELECT
			account_id,
			account_code,
			account_name,
			account_type,
			total_debits,
			total_credits,
			CASE
				WHEN account_type IN ('ASSET', 'EXPENSE') THEN total_debits - total_credits
				ELSE total_credits - total_debits
			END AS net_balance
		FROM account_totals
		WHERE total_debits != 0 OR total_credits != 0
		ORDER BY account_code
	`, schemaName, schemaName, schemaName), tenantID, asOfDate)
	if err != nil {
		return nil, fmt.Errorf("get trial balance: %w", err)
	}
	defer rows.Close()

	var balances []AccountBalance
	for rows.Next() {
		var ab AccountBalance
		if err := rows.Scan(
			&ab.AccountID, &ab.AccountCode, &ab.AccountName, &ab.AccountType,
			&ab.DebitBalance, &ab.CreditBalance, &ab.NetBalance,
		); err != nil {
			return nil, fmt.Errorf("scan account balance: %w", err)
		}
		balances = append(balances, ab)
	}
	return balances, nil
}

// GetPeriodBalances retrieves account activity for a specific period (not cumulative)
func (r *Repository) GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]AccountBalance, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		WITH period_totals AS (
			SELECT
				a.id AS account_id,
				a.code AS account_code,
				a.name AS account_name,
				a.account_type,
				COALESCE(SUM(jel.debit_amount), 0) AS total_debits,
				COALESCE(SUM(jel.credit_amount), 0) AS total_credits
			FROM %s.accounts a
			LEFT JOIN %s.journal_entry_lines jel ON jel.account_id = a.id AND jel.tenant_id = a.tenant_id
			LEFT JOIN %s.journal_entries je ON je.id = jel.journal_entry_id
			WHERE a.tenant_id = $1
			  AND (je.id IS NULL OR (je.entry_date >= $2 AND je.entry_date <= $3 AND je.status = 'POSTED'))
			  AND a.account_type IN ('REVENUE', 'EXPENSE')
			GROUP BY a.id, a.code, a.name, a.account_type
		)
		SELECT
			account_id,
			account_code,
			account_name,
			account_type,
			total_debits,
			total_credits,
			CASE
				WHEN account_type = 'EXPENSE' THEN total_debits - total_credits
				ELSE total_credits - total_debits
			END AS net_balance
		FROM period_totals
		WHERE total_debits != 0 OR total_credits != 0
		ORDER BY account_type DESC, account_code
	`, schemaName, schemaName, schemaName), tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get period balances: %w", err)
	}
	defer rows.Close()

	var balances []AccountBalance
	for rows.Next() {
		var ab AccountBalance
		if err := rows.Scan(
			&ab.AccountID, &ab.AccountCode, &ab.AccountName, &ab.AccountType,
			&ab.DebitBalance, &ab.CreditBalance, &ab.NetBalance,
		); err != nil {
			return nil, fmt.Errorf("scan period balance: %w", err)
		}
		balances = append(balances, ab)
	}
	return balances, nil
}

// VoidJournalEntry voids a journal entry and creates a reversal entry within a transaction
func (r *Repository) VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *JournalEntry) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Mark original as voided
	now := time.Now()
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.journal_entries
		SET status = $1, voided_at = $2, voided_by = $3, void_reason = $4
		WHERE id = $5 AND tenant_id = $6 AND status = $7
	`, schemaName), StatusVoided, now, userID, reason, entryID, tenantID, StatusPosted)
	if err != nil {
		return fmt.Errorf("mark entry as voided: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("entry not found or not in posted status")
	}

	// Create the reversal entry
	if err := r.CreateJournalEntryTx(ctx, schemaName, tx, reversal); err != nil {
		return fmt.Errorf("create reversal entry: %w", err)
	}

	return tx.Commit(ctx)
}
