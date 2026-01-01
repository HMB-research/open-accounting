//go:build gorm

package banking

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

// NewGORMRepository creates a new GORM banking repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// CreateBankAccount inserts a new bank account
func (r *GORMRepository) CreateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	accountModel := bankAccountToModel(account)
	if err := db.Create(accountModel).Error; err != nil {
		return fmt.Errorf("insert bank account: %w", err)
	}
	return nil
}

// GetBankAccount retrieves a bank account by ID
func (r *GORMRepository) GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var accountModel models.BankAccount
	err := db.Where("id = ? AND tenant_id = ?", accountID, tenantID).First(&accountModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBankAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}

	return modelToBankAccount(&accountModel), nil
}

// ListBankAccounts lists all bank accounts for a tenant
func (r *GORMRepository) ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	query := db.Where("tenant_id = ?", tenantID)

	if filter != nil {
		if filter.IsActive != nil {
			query = query.Where("is_active = ?", *filter.IsActive)
		}
		if filter.Currency != "" {
			query = query.Where("currency = ?", filter.Currency)
		}
	}

	query = query.Order("is_default DESC, name")

	var accountModels []models.BankAccount
	if err := query.Find(&accountModels).Error; err != nil {
		return nil, fmt.Errorf("list bank accounts: %w", err)
	}

	accounts := make([]BankAccount, len(accountModels))
	for i, am := range accountModels {
		accounts[i] = *modelToBankAccount(&am)
	}

	return accounts, nil
}

// UpdateBankAccount updates a bank account
func (r *GORMRepository) UpdateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.BankAccount{}).
		Where("id = ? AND tenant_id = ?", account.ID, account.TenantID).
		Updates(map[string]interface{}{
			"name":          account.Name,
			"bank_name":     account.BankName,
			"swift_code":    account.SwiftCode,
			"gl_account_id": account.GLAccountID,
			"is_active":     account.IsActive,
			"is_default":    account.IsDefault,
		})
	if result.Error != nil {
		return fmt.Errorf("update bank account: %w", result.Error)
	}
	return nil
}

// DeleteBankAccount deletes a bank account
func (r *GORMRepository) DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Where("id = ? AND tenant_id = ?", accountID, tenantID).Delete(&models.BankAccount{})
	if result.Error != nil {
		return fmt.Errorf("delete bank account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrBankAccountNotFound
	}
	return nil
}

// UnsetDefaultAccounts unsets all default accounts for a tenant
func (r *GORMRepository) UnsetDefaultAccounts(ctx context.Context, schemaName, tenantID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	if err := db.Model(&models.BankAccount{}).
		Where("tenant_id = ?", tenantID).
		Update("is_default", false).Error; err != nil {
		return fmt.Errorf("unset default: %w", err)
	}
	return nil
}

// CountTransactionsForAccount counts transactions for an account
func (r *GORMRepository) CountTransactionsForAccount(ctx context.Context, schemaName, accountID string) (int, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var count int64
	if err := db.Model(&models.BankTransaction{}).
		Where("bank_account_id = ?", accountID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count transactions: %w", err)
	}
	return int(count), nil
}

// CalculateAccountBalance calculates the balance of an account
func (r *GORMRepository) CalculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var result struct {
		Balance models.Decimal
	}
	err := db.Model(&models.BankTransaction{}).
		Select("COALESCE(SUM(amount), 0) as balance").
		Where("bank_account_id = ?", accountID).
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("calculate balance: %w", err)
	}
	return result.Balance.Decimal, nil
}

// ListTransactions lists bank transactions with filters
func (r *GORMRepository) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	query := db.Where("tenant_id = ?", tenantID)

	if filter != nil {
		if filter.BankAccountID != "" {
			query = query.Where("bank_account_id = ?", filter.BankAccountID)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.FromDate != nil {
			query = query.Where("transaction_date >= ?", *filter.FromDate)
		}
		if filter.ToDate != nil {
			query = query.Where("transaction_date <= ?", *filter.ToDate)
		}
		if filter.MinAmount != nil {
			query = query.Where("amount >= ?", filter.MinAmount.String())
		}
		if filter.MaxAmount != nil {
			query = query.Where("amount <= ?", filter.MaxAmount.String())
		}
	}

	query = query.Order("transaction_date DESC, imported_at DESC")

	var transactionModels []models.BankTransaction
	if err := query.Find(&transactionModels).Error; err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	transactions := make([]BankTransaction, len(transactionModels))
	for i, tm := range transactionModels {
		transactions[i] = *modelToBankTransaction(&tm)
	}

	return transactions, nil
}

// GetTransaction retrieves a single bank transaction
func (r *GORMRepository) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var transactionModel models.BankTransaction
	err := db.Where("id = ? AND tenant_id = ?", transactionID, tenantID).First(&transactionModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTransactionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	return modelToBankTransaction(&transactionModel), nil
}

// MatchTransaction matches a bank transaction to a payment
func (r *GORMRepository) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.BankTransaction{}).
		Where("id = ? AND tenant_id = ? AND status = ?", transactionID, tenantID, StatusUnmatched).
		Updates(map[string]interface{}{
			"matched_payment_id": paymentID,
			"status":             StatusMatched,
		})
	if result.Error != nil {
		return fmt.Errorf("match transaction: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTransactionAlreadyMatched
	}
	return nil
}

// UnmatchTransaction removes the match from a bank transaction
func (r *GORMRepository) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.BankTransaction{}).
		Where("id = ? AND tenant_id = ? AND status = ?", transactionID, tenantID, StatusMatched).
		Updates(map[string]interface{}{
			"matched_payment_id": nil,
			"status":             StatusUnmatched,
		})
	if result.Error != nil {
		return fmt.Errorf("unmatch transaction: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTransactionNotMatched
	}
	return nil
}

// CreateTransaction inserts a new bank transaction
func (r *GORMRepository) CreateTransaction(ctx context.Context, schemaName string, t *BankTransaction) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	transactionModel := bankTransactionToModel(t)
	if err := db.Create(transactionModel).Error; err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

// IsTransactionDuplicate checks if a transaction is a duplicate
func (r *GORMRepository) IsTransactionDuplicate(ctx context.Context, schemaName, tenantID, bankAccountID string, date time.Time, amount decimal.Decimal, externalID string) (bool, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	// First check by external ID if provided
	if externalID != "" {
		var count int64
		err := db.Model(&models.BankTransaction{}).
			Where("tenant_id = ? AND bank_account_id = ? AND external_id = ?", tenantID, bankAccountID, externalID).
			Count(&count).Error
		if err != nil {
			return false, fmt.Errorf("check duplicate: %w", err)
		}
		if count > 0 {
			return true, nil
		}
	}

	// Check by date and amount
	var count int64
	err := db.Model(&models.BankTransaction{}).
		Where("tenant_id = ? AND bank_account_id = ? AND transaction_date = ? AND amount = ?",
			tenantID, bankAccountID, date, amount.String()).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check duplicate: %w", err)
	}
	return count > 0, nil
}

// CreateReconciliation inserts a new reconciliation
func (r *GORMRepository) CreateReconciliation(ctx context.Context, schemaName string, rec *BankReconciliation) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	recModel := bankReconciliationToModel(rec)
	if err := db.Create(recModel).Error; err != nil {
		return fmt.Errorf("create reconciliation: %w", err)
	}
	return nil
}

// GetReconciliation retrieves a reconciliation by ID
func (r *GORMRepository) GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var recModel models.BankReconciliation
	err := db.Where("id = ? AND tenant_id = ?", reconciliationID, tenantID).First(&recModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrReconciliationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get reconciliation: %w", err)
	}

	return modelToBankReconciliation(&recModel), nil
}

// ListReconciliations lists reconciliations for a bank account
func (r *GORMRepository) ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var recModels []models.BankReconciliation
	if err := db.Where("tenant_id = ? AND bank_account_id = ?", tenantID, bankAccountID).
		Order("statement_date DESC").
		Find(&recModels).Error; err != nil {
		return nil, fmt.Errorf("list reconciliations: %w", err)
	}

	reconciliations := make([]BankReconciliation, len(recModels))
	for i, rm := range recModels {
		reconciliations[i] = *modelToBankReconciliation(&rm)
	}

	return reconciliations, nil
}

// CompleteReconciliation marks a reconciliation as complete
func (r *GORMRepository) CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Update reconciliation status
		result := tx.Model(&models.BankReconciliation{}).
			Where("id = ? AND tenant_id = ? AND status = ?", reconciliationID, tenantID, ReconciliationInProgress).
			Updates(map[string]interface{}{
				"status":       ReconciliationCompleted,
				"completed_at": now,
			})
		if result.Error != nil {
			return fmt.Errorf("complete reconciliation: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrReconciliationAlreadyDone
		}

		// Mark all matched transactions in this reconciliation as reconciled
		if err := tx.Model(&models.BankTransaction{}).
			Where("tenant_id = ? AND reconciliation_id = ? AND status = ?", tenantID, reconciliationID, StatusMatched).
			Updates(map[string]interface{}{
				"status": StatusReconciled,
			}).Error; err != nil {
			return fmt.Errorf("update transactions: %w", err)
		}

		return nil
	})
}

// AddTransactionToReconciliation adds a transaction to a reconciliation session
func (r *GORMRepository) AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.BankTransaction{}).
		Where("id = ? AND tenant_id = ?", transactionID, tenantID).
		Update("reconciliation_id", reconciliationID)
	if result.Error != nil {
		return fmt.Errorf("add to reconciliation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

// CreateImportRecord creates an import record
func (r *GORMRepository) CreateImportRecord(ctx context.Context, schemaName string, imp *BankStatementImport) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	impModel := bankStatementImportToModel(imp)
	if err := db.Create(impModel).Error; err != nil {
		return fmt.Errorf("create import record: %w", err)
	}
	return nil
}

// GetImportHistory retrieves import history for a bank account
func (r *GORMRepository) GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var impModels []models.BankStatementImport
	if err := db.Where("tenant_id = ? AND bank_account_id = ?", tenantID, bankAccountID).
		Order("created_at DESC").
		Limit(50).
		Find(&impModels).Error; err != nil {
		return nil, fmt.Errorf("get import history: %w", err)
	}

	imports := make([]BankStatementImport, len(impModels))
	for i, im := range impModels {
		imports[i] = *modelToBankStatementImport(&im)
	}

	return imports, nil
}

// Conversion helpers

func modelToBankAccount(m *models.BankAccount) *BankAccount {
	return &BankAccount{
		ID:            m.ID,
		TenantID:      m.TenantID,
		Name:          m.Name,
		AccountNumber: m.AccountNumber,
		BankName:      m.BankName,
		SwiftCode:     m.SwiftCode,
		Currency:      m.Currency,
		GLAccountID:   m.GLAccountID,
		IsDefault:     m.IsDefault,
		IsActive:      m.IsActive,
		CreatedAt:     m.CreatedAt,
	}
}

func bankAccountToModel(a *BankAccount) *models.BankAccount {
	return &models.BankAccount{
		ID:            a.ID,
		TenantID:      a.TenantID,
		Name:          a.Name,
		AccountNumber: a.AccountNumber,
		BankName:      a.BankName,
		SwiftCode:     a.SwiftCode,
		Currency:      a.Currency,
		GLAccountID:   a.GLAccountID,
		IsDefault:     a.IsDefault,
		IsActive:      a.IsActive,
		CreatedAt:     a.CreatedAt,
	}
}

func modelToBankTransaction(m *models.BankTransaction) *BankTransaction {
	return &BankTransaction{
		ID:                  m.ID,
		TenantID:            m.TenantID,
		BankAccountID:       m.BankAccountID,
		TransactionDate:     m.TransactionDate,
		ValueDate:           m.ValueDate,
		Amount:              m.Amount.Decimal,
		Currency:            m.Currency,
		Description:         m.Description,
		Reference:           m.Reference,
		CounterpartyName:    m.CounterpartyName,
		CounterpartyAccount: m.CounterpartyAccount,
		Status:              TransactionStatus(m.Status),
		MatchedPaymentID:    m.MatchedPaymentID,
		JournalEntryID:      m.JournalEntryID,
		ReconciliationID:    m.ReconciliationID,
		ImportedAt:          m.ImportedAt,
		ExternalID:          m.ExternalID,
	}
}

func bankTransactionToModel(t *BankTransaction) *models.BankTransaction {
	return &models.BankTransaction{
		ID:                  t.ID,
		TenantID:            t.TenantID,
		BankAccountID:       t.BankAccountID,
		TransactionDate:     t.TransactionDate,
		ValueDate:           t.ValueDate,
		Amount:              models.Decimal{Decimal: t.Amount},
		Currency:            t.Currency,
		Description:         t.Description,
		Reference:           t.Reference,
		CounterpartyName:    t.CounterpartyName,
		CounterpartyAccount: t.CounterpartyAccount,
		Status:              models.TransactionStatus(t.Status),
		MatchedPaymentID:    t.MatchedPaymentID,
		JournalEntryID:      t.JournalEntryID,
		ReconciliationID:    t.ReconciliationID,
		ImportedAt:          t.ImportedAt,
		ExternalID:          t.ExternalID,
	}
}

func modelToBankReconciliation(m *models.BankReconciliation) *BankReconciliation {
	return &BankReconciliation{
		ID:             m.ID,
		TenantID:       m.TenantID,
		BankAccountID:  m.BankAccountID,
		StatementDate:  m.StatementDate,
		OpeningBalance: m.OpeningBalance.Decimal,
		ClosingBalance: m.ClosingBalance.Decimal,
		Status:         ReconciliationStatus(m.Status),
		CompletedAt:    m.CompletedAt,
		CreatedAt:      m.CreatedAt,
		CreatedBy:      m.CreatedBy,
	}
}

func bankReconciliationToModel(r *BankReconciliation) *models.BankReconciliation {
	return &models.BankReconciliation{
		ID:             r.ID,
		TenantID:       r.TenantID,
		BankAccountID:  r.BankAccountID,
		StatementDate:  r.StatementDate,
		OpeningBalance: models.Decimal{Decimal: r.OpeningBalance},
		ClosingBalance: models.Decimal{Decimal: r.ClosingBalance},
		Status:         models.ReconciliationStatus(r.Status),
		CompletedAt:    r.CompletedAt,
		CreatedAt:      r.CreatedAt,
		CreatedBy:      r.CreatedBy,
	}
}

func modelToBankStatementImport(m *models.BankStatementImport) *BankStatementImport {
	return &BankStatementImport{
		ID:                   m.ID,
		TenantID:             m.TenantID,
		BankAccountID:        m.BankAccountID,
		FileName:             m.FileName,
		TransactionsImported: m.TransactionsImported,
		TransactionsMatched:  m.TransactionsMatched,
		DuplicatesSkipped:    m.DuplicatesSkipped,
		CreatedAt:            m.CreatedAt,
	}
}

func bankStatementImportToModel(i *BankStatementImport) *models.BankStatementImport {
	return &models.BankStatementImport{
		ID:                   i.ID,
		TenantID:             i.TenantID,
		BankAccountID:        i.BankAccountID,
		FileName:             i.FileName,
		TransactionsImported: i.TransactionsImported,
		TransactionsMatched:  i.TransactionsMatched,
		DuplicatesSkipped:    i.DuplicatesSkipped,
		CreatedAt:            i.CreatedAt,
	}
}
