package banking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides bank reconciliation operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new banking service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// EnsureSchema creates the bank reconciliation tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf(`
		-- Bank Reconciliations table
		CREATE TABLE IF NOT EXISTS %s.bank_reconciliations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL,
			bank_account_id UUID NOT NULL REFERENCES %s.bank_accounts(id),
			statement_date DATE NOT NULL,
			opening_balance NUMERIC(28,8) NOT NULL,
			closing_balance NUMERIC(28,8) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'IN_PROGRESS' CHECK (status IN ('IN_PROGRESS', 'COMPLETED')),
			completed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_by UUID NOT NULL
		);

		-- Bank Statement Imports table
		CREATE TABLE IF NOT EXISTS %s.bank_statement_imports (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL,
			bank_account_id UUID NOT NULL REFERENCES %s.bank_accounts(id),
			file_name VARCHAR(255) NOT NULL,
			transactions_imported INTEGER NOT NULL DEFAULT 0,
			transactions_matched INTEGER NOT NULL DEFAULT 0,
			duplicates_skipped INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Add reconciliation_id to bank_transactions if not exists
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = '%s'
				AND table_name = 'bank_transactions'
				AND column_name = 'reconciliation_id'
			) THEN
				ALTER TABLE %s.bank_transactions
				ADD COLUMN reconciliation_id UUID REFERENCES %s.bank_reconciliations(id);
			END IF;
		END $$;

		-- Create indexes if not exists
		CREATE INDEX IF NOT EXISTS idx_bank_reconciliations_account ON %s.bank_reconciliations(bank_account_id);
		CREATE INDEX IF NOT EXISTS idx_bank_reconciliations_status ON %s.bank_reconciliations(status);
		CREATE INDEX IF NOT EXISTS idx_bank_imports_account ON %s.bank_statement_imports(bank_account_id);
	`, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName, schemaName)

	_, err := s.db.Exec(ctx, query)
	return err
}

// =============================================================================
// Bank Account Operations
// =============================================================================

// CreateBankAccount creates a new bank account
func (s *Service) CreateBankAccount(ctx context.Context, schemaName, tenantID string, req *CreateBankAccountRequest) (*BankAccount, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	currency := req.Currency
	if currency == "" {
		currency = "EUR"
	}

	account := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		Name:          req.Name,
		AccountNumber: req.AccountNumber,
		BankName:      req.BankName,
		SwiftCode:     req.SwiftCode,
		Currency:      currency,
		GLAccountID:   req.GLAccountID,
		IsDefault:     req.IsDefault,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// If this is default, unset other defaults
	if req.IsDefault {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.bank_accounts SET is_default = false WHERE tenant_id = $1
		`, schemaName), tenantID)
		if err != nil {
			return nil, fmt.Errorf("unset default: %w", err)
		}
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_accounts (id, tenant_id, name, account_number, bank_name, swift_code, currency, gl_account_id, is_default, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, schemaName), account.ID, account.TenantID, account.Name, account.AccountNumber, account.BankName,
		account.SwiftCode, account.Currency, account.GLAccountID, account.IsDefault, account.IsActive, account.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert bank account: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return account, nil
}

// GetBankAccount retrieves a bank account by ID
func (s *Service) GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error) {
	var account BankAccount
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, account_number, bank_name, swift_code, currency, gl_account_id, is_default, is_active, created_at
		FROM %s.bank_accounts
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), accountID, tenantID).Scan(
		&account.ID, &account.TenantID, &account.Name, &account.AccountNumber, &account.BankName,
		&account.SwiftCode, &account.Currency, &account.GLAccountID, &account.IsDefault, &account.IsActive, &account.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("bank account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}

	// Calculate current balance from transactions
	balance, err := s.calculateAccountBalance(ctx, schemaName, accountID)
	if err == nil {
		account.Balance = balance
	}

	return &account, nil
}

// ListBankAccounts lists all bank accounts for a tenant
func (s *Service) ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error) {
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

	rows, err := s.db.Query(ctx, query, args...)
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

		// Calculate balance for each account
		balance, err := s.calculateAccountBalance(ctx, schemaName, account.ID)
		if err == nil {
			account.Balance = balance
		}

		accounts = append(accounts, account)
	}

	if accounts == nil {
		accounts = []BankAccount{}
	}

	return accounts, nil
}

// UpdateBankAccount updates a bank account
func (s *Service) UpdateBankAccount(ctx context.Context, schemaName, tenantID, accountID string, req *UpdateBankAccountRequest) (*BankAccount, error) {
	account, err := s.GetBankAccount(ctx, schemaName, tenantID, accountID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Handle setting as default
	if req.IsDefault != nil && *req.IsDefault && !account.IsDefault {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.bank_accounts SET is_default = false WHERE tenant_id = $1
		`, schemaName), tenantID)
		if err != nil {
			return nil, fmt.Errorf("unset default: %w", err)
		}
	}

	if req.Name != "" {
		account.Name = req.Name
	}
	if req.BankName != "" {
		account.BankName = req.BankName
	}
	if req.SwiftCode != "" {
		account.SwiftCode = req.SwiftCode
	}
	if req.GLAccountID != nil {
		account.GLAccountID = req.GLAccountID
	}
	if req.IsActive != nil {
		account.IsActive = *req.IsActive
	}
	if req.IsDefault != nil {
		account.IsDefault = *req.IsDefault
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_accounts
		SET name = $1, bank_name = $2, swift_code = $3, gl_account_id = $4, is_active = $5, is_default = $6
		WHERE id = $7 AND tenant_id = $8
	`, schemaName), account.Name, account.BankName, account.SwiftCode, account.GLAccountID,
		account.IsActive, account.IsDefault, accountID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("update bank account: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return account, nil
}

// DeleteBankAccount deletes a bank account (only if no transactions)
func (s *Service) DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	// Check for transactions
	var count int
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.bank_transactions WHERE bank_account_id = $1
	`, schemaName), accountID).Scan(&count)
	if err != nil {
		return fmt.Errorf("check transactions: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete bank account with transactions")
	}

	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.bank_accounts WHERE id = $1 AND tenant_id = $2
	`, schemaName), accountID, tenantID)
	if err != nil {
		return fmt.Errorf("delete bank account: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("bank account not found")
	}

	return nil
}

func (s *Service) calculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(amount), 0) FROM %s.bank_transactions WHERE bank_account_id = $1
	`, schemaName), accountID).Scan(&balance)
	return balance, err
}

// =============================================================================
// Bank Transaction Operations
// =============================================================================

// ListTransactions lists bank transactions with filters
func (s *Service) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error) {
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

	rows, err := s.db.Query(ctx, query, args...)
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
func (s *Service) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error) {
	var t BankTransaction
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
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
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	return &t, nil
}

// MatchTransaction matches a bank transaction to a payment
func (s *Service) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET matched_payment_id = $1, status = 'MATCHED'
		WHERE id = $2 AND tenant_id = $3 AND status = 'UNMATCHED'
	`, schemaName), paymentID, transactionID, tenantID)
	if err != nil {
		return fmt.Errorf("match transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found or already matched")
	}
	return nil
}

// UnmatchTransaction removes the match from a bank transaction
func (s *Service) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET matched_payment_id = NULL, status = 'UNMATCHED'
		WHERE id = $1 AND tenant_id = $2 AND status = 'MATCHED'
	`, schemaName), transactionID, tenantID)
	if err != nil {
		return fmt.Errorf("unmatch transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found or not matched")
	}
	return nil
}

// =============================================================================
// Reconciliation Operations
// =============================================================================

// CreateReconciliation starts a new reconciliation session
func (s *Service) CreateReconciliation(ctx context.Context, schemaName, tenantID, bankAccountID, userID string, req *CreateReconciliationRequest) (*BankReconciliation, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	statementDate, err := time.Parse("2006-01-02", req.StatementDate)
	if err != nil {
		return nil, fmt.Errorf("invalid statement date: %w", err)
	}

	reconciliation := &BankReconciliation{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		BankAccountID:  bankAccountID,
		StatementDate:  statementDate,
		OpeningBalance: req.OpeningBalance,
		ClosingBalance: req.ClosingBalance,
		Status:         ReconciliationInProgress,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	_, err = s.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_reconciliations (id, tenant_id, bank_account_id, statement_date, opening_balance, closing_balance, status, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, schemaName), reconciliation.ID, reconciliation.TenantID, reconciliation.BankAccountID,
		reconciliation.StatementDate, reconciliation.OpeningBalance, reconciliation.ClosingBalance,
		reconciliation.Status, reconciliation.CreatedAt, reconciliation.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("create reconciliation: %w", err)
	}

	return reconciliation, nil
}

// GetReconciliation retrieves a reconciliation by ID
func (s *Service) GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error) {
	var r BankReconciliation
	err := s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, bank_account_id, statement_date, opening_balance, closing_balance, status, completed_at, created_at, created_by
		FROM %s.bank_reconciliations
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), reconciliationID, tenantID).Scan(
		&r.ID, &r.TenantID, &r.BankAccountID, &r.StatementDate, &r.OpeningBalance, &r.ClosingBalance,
		&r.Status, &r.CompletedAt, &r.CreatedAt, &r.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("reconciliation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get reconciliation: %w", err)
	}
	return &r, nil
}

// ListReconciliations lists reconciliations for a bank account
func (s *Service) ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error) {
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
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
		var r BankReconciliation
		if err := rows.Scan(
			&r.ID, &r.TenantID, &r.BankAccountID, &r.StatementDate, &r.OpeningBalance, &r.ClosingBalance,
			&r.Status, &r.CompletedAt, &r.CreatedAt, &r.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan reconciliation: %w", err)
		}
		reconciliations = append(reconciliations, r)
	}

	if reconciliations == nil {
		reconciliations = []BankReconciliation{}
	}

	return reconciliations, nil
}

// CompleteReconciliation marks a reconciliation as complete
func (s *Service) CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	now := time.Now()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

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
		return fmt.Errorf("reconciliation not found or already completed")
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
func (s *Service) AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error {
	result, err := s.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET reconciliation_id = $1
		WHERE id = $2 AND tenant_id = $3
	`, schemaName), reconciliationID, transactionID, tenantID)
	if err != nil {
		return fmt.Errorf("add to reconciliation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}
	return nil
}

// =============================================================================
// Import Operations
// =============================================================================

// GetImportHistory retrieves import history for a bank account
func (s *Service) GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error) {
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
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
