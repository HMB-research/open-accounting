package banking

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Repository defines the contract for banking data access
type Repository interface {
	// Bank Account operations
	CreateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error
	GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error)
	ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error)
	UpdateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error
	DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error
	UnsetDefaultAccounts(ctx context.Context, schemaName, tenantID string) error
	CountTransactionsForAccount(ctx context.Context, schemaName, accountID string) (int, error)
	CalculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error)

	// Transaction operations
	ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error)
	GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error)
	MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error
	UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error
	CreateTransaction(ctx context.Context, schemaName string, t *BankTransaction) error
	IsTransactionDuplicate(ctx context.Context, schemaName, tenantID, bankAccountID string, date time.Time, amount decimal.Decimal, externalID string) (bool, error)

	// Reconciliation operations
	CreateReconciliation(ctx context.Context, schemaName string, r *BankReconciliation) error
	GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error)
	ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error)
	CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error
	AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error

	// Import operations
	CreateImportRecord(ctx context.Context, schemaName string, imp *BankStatementImport) error
	GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error)
}

// Error definitions
var (
	ErrBankAccountNotFound         = fmt.Errorf("bank account not found")
	ErrTransactionNotFound         = fmt.Errorf("transaction not found")
	ErrReconciliationNotFound      = fmt.Errorf("reconciliation not found")
	ErrAccountHasTransactions      = fmt.Errorf("cannot delete bank account with transactions")
	ErrTransactionAlreadyMatched   = fmt.Errorf("transaction not found or already matched")
	ErrTransactionNotMatched       = fmt.Errorf("transaction not found or not matched")
	ErrReconciliationAlreadyDone   = fmt.Errorf("reconciliation not found or already completed")
)

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateBankAccount inserts a new bank account
func (r *PostgresRepository) CreateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_accounts (id, tenant_id, name, account_number, bank_name, swift_code, currency, gl_account_id, is_default, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, schemaName), account.ID, account.TenantID, account.Name, account.AccountNumber, account.BankName,
		account.SwiftCode, account.Currency, account.GLAccountID, account.IsDefault, account.IsActive, account.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert bank account: %w", err)
	}
	return nil
}

// GetBankAccount retrieves a bank account by ID
func (r *PostgresRepository) GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error) {
	var account BankAccount
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, account_number, bank_name, swift_code, currency, gl_account_id, is_default, is_active, created_at
		FROM %s.bank_accounts
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), accountID, tenantID).Scan(
		&account.ID, &account.TenantID, &account.Name, &account.AccountNumber, &account.BankName,
		&account.SwiftCode, &account.Currency, &account.GLAccountID, &account.IsDefault, &account.IsActive, &account.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrBankAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}
	return &account, nil
}

// ListBankAccounts lists all bank accounts for a tenant
func (r *PostgresRepository) ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, account_number, bank_name, swift_code, currency, gl_account_id, is_default, is_active, created_at
		FROM %s.bank_accounts
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.IsActive != nil {
			query += fmt.Sprintf(" AND is_active = $%d", argNum)
			args = append(args, *filter.IsActive)
			argNum++
		}
		if filter.Currency != "" {
			query += fmt.Sprintf(" AND currency = $%d", argNum)
			args = append(args, filter.Currency)
		}
	}

	query += " ORDER BY is_default DESC, name"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list bank accounts: %w", err)
	}
	defer rows.Close()

	var accounts []BankAccount
	for rows.Next() {
		var account BankAccount
		if err := rows.Scan(
			&account.ID, &account.TenantID, &account.Name, &account.AccountNumber, &account.BankName,
			&account.SwiftCode, &account.Currency, &account.GLAccountID, &account.IsDefault, &account.IsActive, &account.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bank account: %w", err)
		}
		accounts = append(accounts, account)
	}

	if accounts == nil {
		accounts = []BankAccount{}
	}

	return accounts, nil
}

// UpdateBankAccount updates a bank account
func (r *PostgresRepository) UpdateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_accounts
		SET name = $1, bank_name = $2, swift_code = $3, gl_account_id = $4, is_active = $5, is_default = $6
		WHERE id = $7 AND tenant_id = $8
	`, schemaName), account.Name, account.BankName, account.SwiftCode, account.GLAccountID,
		account.IsActive, account.IsDefault, account.ID, account.TenantID)
	if err != nil {
		return fmt.Errorf("update bank account: %w", err)
	}
	return nil
}

// DeleteBankAccount deletes a bank account
func (r *PostgresRepository) DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.bank_accounts WHERE id = $1 AND tenant_id = $2
	`, schemaName), accountID, tenantID)
	if err != nil {
		return fmt.Errorf("delete bank account: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrBankAccountNotFound
	}
	return nil
}

// UnsetDefaultAccounts unsets all default accounts for a tenant
func (r *PostgresRepository) UnsetDefaultAccounts(ctx context.Context, schemaName, tenantID string) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_accounts SET is_default = false WHERE tenant_id = $1
	`, schemaName), tenantID)
	if err != nil {
		return fmt.Errorf("unset default: %w", err)
	}
	return nil
}

// CountTransactionsForAccount counts transactions for an account
func (r *PostgresRepository) CountTransactionsForAccount(ctx context.Context, schemaName, accountID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.bank_transactions WHERE bank_account_id = $1
	`, schemaName), accountID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count transactions: %w", err)
	}
	return count, nil
}

// CalculateAccountBalance calculates the balance of an account
func (r *PostgresRepository) CalculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(amount), 0) FROM %s.bank_transactions WHERE bank_account_id = $1
	`, schemaName), accountID).Scan(&balance)
	return balance, err
}

// ListTransactions lists bank transactions with filters
func (r *PostgresRepository) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, bank_account_id, transaction_date, value_date, amount, currency,
			   description, reference, counterparty_name, counterparty_account, status,
			   matched_payment_id, journal_entry_id, reconciliation_id, imported_at, external_id
		FROM %s.bank_transactions
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.BankAccountID != "" {
			query += fmt.Sprintf(" AND bank_account_id = $%d", argNum)
			args = append(args, filter.BankAccountID)
			argNum++
		}
		if filter.Status != "" {
			query += fmt.Sprintf(" AND status = $%d", argNum)
			args = append(args, filter.Status)
			argNum++
		}
		if filter.FromDate != nil {
			query += fmt.Sprintf(" AND transaction_date >= $%d", argNum)
			args = append(args, *filter.FromDate)
			argNum++
		}
		if filter.ToDate != nil {
			query += fmt.Sprintf(" AND transaction_date <= $%d", argNum)
			args = append(args, *filter.ToDate)
			argNum++
		}
		if filter.MinAmount != nil {
			query += fmt.Sprintf(" AND amount >= $%d", argNum)
			args = append(args, *filter.MinAmount)
			argNum++
		}
		if filter.MaxAmount != nil {
			query += fmt.Sprintf(" AND amount <= $%d", argNum)
			args = append(args, *filter.MaxAmount)
		}
	}

	query += " ORDER BY transaction_date DESC, imported_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var transactions []BankTransaction
	for rows.Next() {
		var t BankTransaction
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.BankAccountID, &t.TransactionDate, &t.ValueDate, &t.Amount, &t.Currency,
			&t.Description, &t.Reference, &t.CounterpartyName, &t.CounterpartyAccount, &t.Status,
			&t.MatchedPaymentID, &t.JournalEntryID, &t.ReconciliationID, &t.ImportedAt, &t.ExternalID,
		); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, t)
	}

	if transactions == nil {
		transactions = []BankTransaction{}
	}

	return transactions, nil
}

// GetTransaction retrieves a single bank transaction
func (r *PostgresRepository) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error) {
	var t BankTransaction
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, bank_account_id, transaction_date, value_date, amount, currency,
			   description, reference, counterparty_name, counterparty_account, status,
			   matched_payment_id, journal_entry_id, reconciliation_id, imported_at, external_id
		FROM %s.bank_transactions
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), transactionID, tenantID).Scan(
		&t.ID, &t.TenantID, &t.BankAccountID, &t.TransactionDate, &t.ValueDate, &t.Amount, &t.Currency,
		&t.Description, &t.Reference, &t.CounterpartyName, &t.CounterpartyAccount, &t.Status,
		&t.MatchedPaymentID, &t.JournalEntryID, &t.ReconciliationID, &t.ImportedAt, &t.ExternalID,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrTransactionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	return &t, nil
}

// MatchTransaction matches a bank transaction to a payment
func (r *PostgresRepository) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET matched_payment_id = $1, status = 'MATCHED'
		WHERE id = $2 AND tenant_id = $3 AND status = 'UNMATCHED'
	`, schemaName), paymentID, transactionID, tenantID)
	if err != nil {
		return fmt.Errorf("match transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTransactionAlreadyMatched
	}
	return nil
}

// UnmatchTransaction removes the match from a bank transaction
func (r *PostgresRepository) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET matched_payment_id = NULL, status = 'UNMATCHED'
		WHERE id = $1 AND tenant_id = $2 AND status = 'MATCHED'
	`, schemaName), transactionID, tenantID)
	if err != nil {
		return fmt.Errorf("unmatch transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTransactionNotMatched
	}
	return nil
}

// CreateTransaction inserts a new bank transaction
func (r *PostgresRepository) CreateTransaction(ctx context.Context, schemaName string, t *BankTransaction) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_transactions (
			id, tenant_id, bank_account_id, transaction_date, value_date, amount, currency,
			description, reference, counterparty_name, counterparty_account, status,
			matched_payment_id, journal_entry_id, reconciliation_id, imported_at, external_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, schemaName), t.ID, t.TenantID, t.BankAccountID, t.TransactionDate, t.ValueDate, t.Amount, t.Currency,
		t.Description, t.Reference, t.CounterpartyName, t.CounterpartyAccount, t.Status,
		t.MatchedPaymentID, t.JournalEntryID, t.ReconciliationID, t.ImportedAt, t.ExternalID)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

// IsTransactionDuplicate checks if a transaction is a duplicate
func (r *PostgresRepository) IsTransactionDuplicate(ctx context.Context, schemaName, tenantID, bankAccountID string, date time.Time, amount decimal.Decimal, externalID string) (bool, error) {
	// First check by external ID if provided
	if externalID != "" {
		var count int
		err := r.db.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s.bank_transactions
			WHERE tenant_id = $1 AND bank_account_id = $2 AND external_id = $3
		`, schemaName), tenantID, bankAccountID, externalID).Scan(&count)
		if err != nil {
			return false, fmt.Errorf("check duplicate: %w", err)
		}
		if count > 0 {
			return true, nil
		}
	}

	// Check by date and amount
	var count int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.bank_transactions
		WHERE tenant_id = $1 AND bank_account_id = $2 AND transaction_date = $3 AND amount = $4
	`, schemaName), tenantID, bankAccountID, date, amount).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check duplicate: %w", err)
	}
	return count > 0, nil
}

// CreateReconciliation inserts a new reconciliation
func (r *PostgresRepository) CreateReconciliation(ctx context.Context, schemaName string, rec *BankReconciliation) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_reconciliations (id, tenant_id, bank_account_id, statement_date, opening_balance, closing_balance, status, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, schemaName), rec.ID, rec.TenantID, rec.BankAccountID,
		rec.StatementDate, rec.OpeningBalance, rec.ClosingBalance,
		rec.Status, rec.CreatedAt, rec.CreatedBy)
	if err != nil {
		return fmt.Errorf("create reconciliation: %w", err)
	}
	return nil
}

// GetReconciliation retrieves a reconciliation by ID
func (r *PostgresRepository) GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error) {
	var rec BankReconciliation
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, bank_account_id, statement_date, opening_balance, closing_balance, status, completed_at, created_at, created_by
		FROM %s.bank_reconciliations
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), reconciliationID, tenantID).Scan(
		&rec.ID, &rec.TenantID, &rec.BankAccountID, &rec.StatementDate, &rec.OpeningBalance, &rec.ClosingBalance,
		&rec.Status, &rec.CompletedAt, &rec.CreatedAt, &rec.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrReconciliationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get reconciliation: %w", err)
	}
	return &rec, nil
}

// ListReconciliations lists reconciliations for a bank account
func (r *PostgresRepository) ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, bank_account_id, statement_date, opening_balance, closing_balance, status, completed_at, created_at, created_by
		FROM %s.bank_reconciliations
		WHERE tenant_id = $1 AND bank_account_id = $2
		ORDER BY statement_date DESC
	`, schemaName), tenantID, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("list reconciliations: %w", err)
	}
	defer rows.Close()

	var reconciliations []BankReconciliation
	for rows.Next() {
		var rec BankReconciliation
		if err := rows.Scan(
			&rec.ID, &rec.TenantID, &rec.BankAccountID, &rec.StatementDate, &rec.OpeningBalance, &rec.ClosingBalance,
			&rec.Status, &rec.CompletedAt, &rec.CreatedAt, &rec.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan reconciliation: %w", err)
		}
		reconciliations = append(reconciliations, rec)
	}

	if reconciliations == nil {
		reconciliations = []BankReconciliation{}
	}

	return reconciliations, nil
}

// CompleteReconciliation marks a reconciliation as complete
func (r *PostgresRepository) CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now()
	// Update reconciliation status
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_reconciliations
		SET status = 'COMPLETED', completed_at = $1
		WHERE id = $2 AND tenant_id = $3 AND status = 'IN_PROGRESS'
	`, schemaName), now, reconciliationID, tenantID)
	if err != nil {
		return fmt.Errorf("complete reconciliation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrReconciliationAlreadyDone
	}

	// Mark all matched transactions in this reconciliation as reconciled
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET status = 'RECONCILED', reconciliation_id = $1
		WHERE tenant_id = $2 AND reconciliation_id = $1 AND status = 'MATCHED'
	`, schemaName), reconciliationID, tenantID)
	if err != nil {
		return fmt.Errorf("update transactions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// AddTransactionToReconciliation adds a transaction to a reconciliation session
func (r *PostgresRepository) AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET reconciliation_id = $1
		WHERE id = $2 AND tenant_id = $3
	`, schemaName), reconciliationID, transactionID, tenantID)
	if err != nil {
		return fmt.Errorf("add to reconciliation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

// CreateImportRecord creates an import record
func (r *PostgresRepository) CreateImportRecord(ctx context.Context, schemaName string, imp *BankStatementImport) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_statement_imports (id, tenant_id, bank_account_id, file_name, transactions_imported, transactions_matched, duplicates_skipped, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, schemaName), imp.ID, imp.TenantID, imp.BankAccountID, imp.FileName, imp.TransactionsImported,
		imp.TransactionsMatched, imp.DuplicatesSkipped, imp.CreatedAt)
	if err != nil {
		return fmt.Errorf("create import record: %w", err)
	}
	return nil
}

// GetImportHistory retrieves import history for a bank account
func (r *PostgresRepository) GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, bank_account_id, file_name, transactions_imported, transactions_matched, duplicates_skipped, created_at
		FROM %s.bank_statement_imports
		WHERE tenant_id = $1 AND bank_account_id = $2
		ORDER BY created_at DESC
		LIMIT 50
	`, schemaName), tenantID, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("get import history: %w", err)
	}
	defer rows.Close()

	var imports []BankStatementImport
	for rows.Next() {
		var i BankStatementImport
		if err := rows.Scan(
			&i.ID, &i.TenantID, &i.BankAccountID, &i.FileName, &i.TransactionsImported,
			&i.TransactionsMatched, &i.DuplicatesSkipped, &i.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan import: %w", err)
		}
		imports = append(imports, i)
	}

	if imports == nil {
		imports = []BankStatementImport{}
	}

	return imports, nil
}
