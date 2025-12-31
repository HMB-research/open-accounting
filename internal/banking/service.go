package banking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides bank reconciliation operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new banking service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new banking service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// EnsureSchema creates the bank reconciliation tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	if s.db == nil {
		return fmt.Errorf("database connection not available")
	}
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

	// If this is default, unset other defaults
	if req.IsDefault {
		if err := s.repo.UnsetDefaultAccounts(ctx, schemaName, tenantID); err != nil {
			return nil, err
		}
	}

	if err := s.repo.CreateBankAccount(ctx, schemaName, account); err != nil {
		return nil, err
	}

	return account, nil
}

// GetBankAccount retrieves a bank account by ID
func (s *Service) GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error) {
	account, err := s.repo.GetBankAccount(ctx, schemaName, tenantID, accountID)
	if err != nil {
		return nil, err
	}

	// Calculate current balance from transactions
	balance, err := s.repo.CalculateAccountBalance(ctx, schemaName, accountID)
	if err == nil {
		account.Balance = balance
	}

	return account, nil
}

// ListBankAccounts lists all bank accounts for a tenant
func (s *Service) ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error) {
	accounts, err := s.repo.ListBankAccounts(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, err
	}

	// Calculate balance for each account
	for i := range accounts {
		balance, err := s.repo.CalculateAccountBalance(ctx, schemaName, accounts[i].ID)
		if err == nil {
			accounts[i].Balance = balance
		}
	}

	return accounts, nil
}

// UpdateBankAccount updates a bank account
func (s *Service) UpdateBankAccount(ctx context.Context, schemaName, tenantID, accountID string, req *UpdateBankAccountRequest) (*BankAccount, error) {
	account, err := s.repo.GetBankAccount(ctx, schemaName, tenantID, accountID)
	if err != nil {
		return nil, err
	}

	// Handle setting as default
	if req.IsDefault != nil && *req.IsDefault && !account.IsDefault {
		if err := s.repo.UnsetDefaultAccounts(ctx, schemaName, tenantID); err != nil {
			return nil, err
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

	if err := s.repo.UpdateBankAccount(ctx, schemaName, account); err != nil {
		return nil, err
	}

	return account, nil
}

// DeleteBankAccount deletes a bank account (only if no transactions)
func (s *Service) DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	// Check for transactions
	count, err := s.repo.CountTransactionsForAccount(ctx, schemaName, accountID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrAccountHasTransactions
	}

	return s.repo.DeleteBankAccount(ctx, schemaName, tenantID, accountID)
}

// =============================================================================
// Bank Transaction Operations
// =============================================================================

// ListTransactions lists bank transactions with filters
func (s *Service) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error) {
	return s.repo.ListTransactions(ctx, schemaName, tenantID, filter)
}

// GetTransaction retrieves a single bank transaction
func (s *Service) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error) {
	return s.repo.GetTransaction(ctx, schemaName, tenantID, transactionID)
}

// MatchTransaction matches a bank transaction to a payment
func (s *Service) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	return s.repo.MatchTransaction(ctx, schemaName, tenantID, transactionID, paymentID)
}

// UnmatchTransaction removes the match from a bank transaction
func (s *Service) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	return s.repo.UnmatchTransaction(ctx, schemaName, tenantID, transactionID)
}

// =============================================================================
// Reconciliation Operations
// =============================================================================

// CreateReconciliation starts a new reconciliation session
func (s *Service) CreateReconciliation(ctx context.Context, schemaName, tenantID, bankAccountID, userID string, req *CreateReconciliationRequest) (*BankReconciliation, error) {
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

	if err := s.repo.CreateReconciliation(ctx, schemaName, reconciliation); err != nil {
		return nil, err
	}

	return reconciliation, nil
}

// GetReconciliation retrieves a reconciliation by ID
func (s *Service) GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error) {
	return s.repo.GetReconciliation(ctx, schemaName, tenantID, reconciliationID)
}

// ListReconciliations lists reconciliations for a bank account
func (s *Service) ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error) {
	return s.repo.ListReconciliations(ctx, schemaName, tenantID, bankAccountID)
}

// CompleteReconciliation marks a reconciliation as complete
func (s *Service) CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	return s.repo.CompleteReconciliation(ctx, schemaName, tenantID, reconciliationID)
}

// AddTransactionToReconciliation adds a transaction to a reconciliation session
func (s *Service) AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error {
	return s.repo.AddTransactionToReconciliation(ctx, schemaName, tenantID, transactionID, reconciliationID)
}

// =============================================================================
// Import Operations
// =============================================================================

// GetImportHistory retrieves import history for a bank account
func (s *Service) GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error) {
	return s.repo.GetImportHistory(ctx, schemaName, tenantID, bankAccountID)
}

// calculateAccountBalance is kept for backward compatibility but delegates to repository
func (s *Service) calculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error) {
	return s.repo.CalculateAccountBalance(ctx, schemaName, accountID)
}
