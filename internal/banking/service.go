package banking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

type accountingLister interface {
	ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.Account, error)
}

// Service provides bank reconciliation operations
type Service struct {
	repo     Repository
	accounts accountingLister
}

// NewService creates a new banking service
func NewService(db *pgxpool.Pool) *Service {
	if db == nil {
		return &Service{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create banking GORM repository: %w", err))
	}
	return &Service{
		repo:     NewGORMRepository(gormDB),
		accounts: accounting.NewService(db),
	}
}

// NewServiceWithGORM creates a banking service backed by an existing GORM handle.
func NewServiceWithGORM(db *gorm.DB) *Service {
	return &Service{
		repo: NewGORMRepository(db),
	}
}

// NewServiceWithRepository creates a new banking service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return NewServiceWithRepositoryAndAccounting(repo, nil)
}

// NewServiceWithRepositoryAndAccounting creates a new banking service with a custom repository and accounting account lister.
func NewServiceWithRepositoryAndAccounting(repo Repository, accounts accountingLister) *Service {
	return &Service{
		repo:     repo,
		accounts: accounts,
	}
}

// =============================================================================
// Bank Account Operations
// =============================================================================

// CreateBankAccount creates a new bank account
func (s *Service) CreateBankAccount(ctx context.Context, schemaName, tenantID string, req *CreateBankAccountRequest) (*BankAccount, error) {
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "EUR"
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	glAccountID, err := normalizeOptionalBankAccountUUIDPtr(req.GLAccountID, "gl_account_id")
	if err != nil {
		return nil, err
	}

	account := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		Name:          strings.TrimSpace(req.Name),
		AccountNumber: strings.TrimSpace(req.AccountNumber),
		BankName:      strings.TrimSpace(req.BankName),
		SwiftCode:     strings.ToUpper(strings.TrimSpace(req.SwiftCode)),
		Currency:      currency,
		GLAccountID:   glAccountID,
		IsDefault:     req.IsDefault,
		IsActive:      isActive,
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

// ImportBankAccounts imports bank account master data from parsed CSV rows.
func (s *Service) ImportBankAccounts(ctx context.Context, schemaName, tenantID string, req *ImportBankAccountsRequest) (*ImportBankAccountsResult, error) {
	result := &ImportBankAccountsResult{
		FileName: strings.TrimSpace(req.FileName),
	}
	if result.FileName == "" {
		result.FileName = "bank_accounts_import.csv"
	}

	existing, err := s.repo.ListBankAccounts(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list bank accounts before import: %w", err)
	}
	seenAccountNumbers := make(map[string]bool, len(existing)+len(req.Rows))
	for _, account := range existing {
		if key := normalizeBankAccountNumber(account.AccountNumber); key != "" {
			seenAccountNumbers[key] = true
		}
	}

	accountIDsByCode, err := s.bankAccountImportAccountIDsByCode(ctx, schemaName, tenantID, req.Rows)
	if err != nil {
		return nil, err
	}

	for i, row := range req.Rows {
		rowNum := i + 1
		result.RowsProcessed++

		name := strings.TrimSpace(row.Name)
		accountNumber := strings.TrimSpace(row.AccountNumber)
		if name == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: name is required", rowNum))
			result.RowsSkipped++
			continue
		}
		if accountNumber == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: account_number is required", rowNum))
			result.RowsSkipped++
			continue
		}

		accountNumberKey := normalizeBankAccountNumber(accountNumber)
		if seenAccountNumbers[accountNumberKey] {
			if !req.SkipDuplicates {
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d: duplicate account_number %q", rowNum, accountNumber))
			}
			result.RowsSkipped++
			continue
		}

		isDefault, err := parseOptionalImportBool(row.IsDefault, false)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid is_default: %v", rowNum, err))
			result.RowsSkipped++
			continue
		}
		isActive, err := parseOptionalImportBool(row.IsActive, true)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid is_active: %v", rowNum, err))
			result.RowsSkipped++
			continue
		}

		glAccountID, err := resolveOptionalBankAccountImportAccountID(row, accountIDsByCode)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %v", rowNum, err))
			result.RowsSkipped++
			continue
		}

		if _, err := s.CreateBankAccount(ctx, schemaName, tenantID, &CreateBankAccountRequest{
			Name:          name,
			AccountNumber: accountNumber,
			BankName:      row.BankName,
			SwiftCode:     row.SwiftCode,
			Currency:      row.Currency,
			GLAccountID:   glAccountID,
			IsDefault:     isDefault,
			IsActive:      &isActive,
		}); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: create bank account failed: %v", rowNum, err))
			result.RowsSkipped++
			continue
		}

		seenAccountNumbers[accountNumberKey] = true
		result.AccountsImported++
	}

	return result, nil
}

func (s *Service) bankAccountImportAccountIDsByCode(ctx context.Context, schemaName, tenantID string, rows []CSVBankAccountRow) (map[string]string, error) {
	usesAccountCodes := false
	for _, row := range rows {
		if strings.TrimSpace(row.GLAccountCode) != "" {
			usesAccountCodes = true
			break
		}
	}
	if !usesAccountCodes {
		return nil, nil
	}
	if s.accounts == nil {
		return nil, fmt.Errorf("accounting service is required to resolve bank account ledger account codes")
	}

	accounts, err := s.accounts.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list accounts for bank account import: %w", err)
	}
	accountIDsByCode := make(map[string]string, len(accounts))
	for _, account := range accounts {
		key := normalizedBankAccountImportKey(account.Code)
		if key != "" {
			accountIDsByCode[key] = account.ID
		}
	}
	return accountIDsByCode, nil
}

func resolveOptionalBankAccountImportAccountID(row CSVBankAccountRow, accountIDsByCode map[string]string) (*string, error) {
	if accountID := strings.TrimSpace(row.GLAccountID); accountID != "" {
		id, err := normalizeBankAccountUUIDValue(accountID, "gl_account_id")
		if err != nil {
			return nil, err
		}
		return &id, nil
	}
	accountCode := strings.TrimSpace(row.GLAccountCode)
	if accountCode == "" {
		return nil, nil
	}
	accountID, ok := accountIDsByCode[normalizedBankAccountImportKey(accountCode)]
	if !ok {
		return nil, fmt.Errorf("account code %q was not found for gl_account_code", accountCode)
	}
	return &accountID, nil
}

func normalizeOptionalBankAccountUUIDPtr(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	normalized, err := normalizeBankAccountUUIDValue(trimmed, field)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeBankAccountUUIDValue(value, field string) (string, error) {
	parsedID, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("%s must be a valid UUID", field)
	}
	return parsedID.String(), nil
}

func normalizedBankAccountImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseOptionalImportBool(value string, defaultValue bool) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return defaultValue, nil
	}
	switch normalized {
	case "true", "t", "yes", "y", "1":
		return true, nil
	case "false", "f", "no", "n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("expected boolean, got %q", value)
	}
}

func normalizeBankAccountNumber(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
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
		glAccountID, err := normalizeOptionalBankAccountUUIDPtr(req.GLAccountID, "gl_account_id")
		if err != nil {
			return nil, err
		}
		account.GLAccountID = glAccountID
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
// Bank Auto-Match Rule Operations
// =============================================================================

// CreateBankMatchRule creates a bank auto-match rule.
func (s *Service) CreateBankMatchRule(ctx context.Context, schemaName, tenantID string, req *CreateBankMatchRuleRequest) (*BankMatchRule, error) {
	if req == nil {
		return nil, fmt.Errorf("bank match rule request is required")
	}

	bankAccountID, err := s.normalizeBankMatchRuleAccount(ctx, schemaName, tenantID, req.BankAccountID)
	if err != nil {
		return nil, err
	}
	matchField, err := NormalizeBankMatchField(string(req.MatchField))
	if err != nil {
		return nil, err
	}
	name, pattern, err := validateBankMatchRuleText(req.Name, req.Pattern)
	if err != nil {
		return nil, err
	}

	priority := req.Priority
	if priority == 0 {
		priority = 100
	}
	minConfidence := req.MinConfidence
	if minConfidence == 0 {
		minConfidence = 0.7
	}
	maxDateDiffDays := req.MaxDateDiffDays
	if maxDateDiffDays == 0 {
		maxDateDiffDays = DefaultMatcherConfig().MaxDateDiff
	}
	if err := validateBankMatchRuleNumbers(minConfidence, maxDateDiffDays); err != nil {
		return nil, err
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	now := time.Now().UTC()
	rule := &BankMatchRule{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		BankAccountID:      bankAccountID,
		Name:               name,
		Priority:           priority,
		MatchField:         matchField,
		Pattern:            pattern,
		MinConfidence:      minConfidence,
		MaxDateDiffDays:    maxDateDiffDays,
		RequireExactAmount: req.RequireExactAmount,
		IsActive:           isActive,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.repo.CreateBankMatchRule(ctx, schemaName, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// ListBankMatchRules lists bank auto-match rules.
func (s *Service) ListBankMatchRules(ctx context.Context, schemaName, tenantID string, filter *BankMatchRuleFilter) ([]BankMatchRule, error) {
	return s.repo.ListBankMatchRules(ctx, schemaName, tenantID, filter)
}

// GetBankMatchRule retrieves a bank auto-match rule.
func (s *Service) GetBankMatchRule(ctx context.Context, schemaName, tenantID, ruleID string) (*BankMatchRule, error) {
	return s.repo.GetBankMatchRule(ctx, schemaName, tenantID, ruleID)
}

// UpdateBankMatchRule updates a bank auto-match rule.
func (s *Service) UpdateBankMatchRule(ctx context.Context, schemaName, tenantID, ruleID string, req *UpdateBankMatchRuleRequest) (*BankMatchRule, error) {
	if req == nil {
		return nil, fmt.Errorf("bank match rule request is required")
	}
	rule, err := s.repo.GetBankMatchRule(ctx, schemaName, tenantID, ruleID)
	if err != nil {
		return nil, err
	}

	if req.ClearBankAccount {
		rule.BankAccountID = nil
	} else if req.BankAccountID != nil {
		rule.BankAccountID, err = s.normalizeBankMatchRuleAccount(ctx, schemaName, tenantID, req.BankAccountID)
		if err != nil {
			return nil, err
		}
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		rule.Name = name
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.MatchField != nil {
		matchField, err := NormalizeBankMatchField(string(*req.MatchField))
		if err != nil {
			return nil, err
		}
		rule.MatchField = matchField
	}
	if req.Pattern != nil {
		pattern := strings.TrimSpace(*req.Pattern)
		if pattern == "" {
			return nil, fmt.Errorf("pattern is required")
		}
		rule.Pattern = pattern
	}
	if req.MinConfidence != nil {
		rule.MinConfidence = *req.MinConfidence
	}
	if req.MaxDateDiffDays != nil {
		rule.MaxDateDiffDays = *req.MaxDateDiffDays
	}
	if req.RequireExactAmount != nil {
		rule.RequireExactAmount = *req.RequireExactAmount
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}
	if err := validateBankMatchRuleNumbers(rule.MinConfidence, rule.MaxDateDiffDays); err != nil {
		return nil, err
	}

	rule.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateBankMatchRule(ctx, schemaName, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteBankMatchRule deletes a bank auto-match rule.
func (s *Service) DeleteBankMatchRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
	return s.repo.DeleteBankMatchRule(ctx, schemaName, tenantID, ruleID)
}

func (s *Service) normalizeBankMatchRuleAccount(ctx context.Context, schemaName, tenantID string, bankAccountID *string) (*string, error) {
	if bankAccountID == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*bankAccountID)
	if trimmed == "" {
		return nil, nil
	}
	if _, err := s.repo.GetBankAccount(ctx, schemaName, tenantID, trimmed); err != nil {
		return nil, err
	}
	return &trimmed, nil
}

func validateBankMatchRuleText(name, pattern string) (string, string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", "", fmt.Errorf("name is required")
	}
	trimmedPattern := strings.TrimSpace(pattern)
	if trimmedPattern == "" {
		return "", "", fmt.Errorf("pattern is required")
	}
	return trimmedName, trimmedPattern, nil
}

func validateBankMatchRuleNumbers(minConfidence float64, maxDateDiffDays int) error {
	if minConfidence < 0 || minConfidence > 1 {
		return fmt.Errorf("min confidence must be between 0 and 1")
	}
	if maxDateDiffDays < 0 || maxDateDiffDays > 90 {
		return fmt.Errorf("max date diff days must be between 0 and 90")
	}
	return nil
}

// =============================================================================
// Bank Transaction Operations
// =============================================================================

// ListTransactions lists bank transactions with filters
func (s *Service) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error) {
	transactions, err := s.repo.ListTransactions(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, err
	}
	normalizeTransactionSlice(transactions)
	return transactions, nil
}

// GetTransaction retrieves a single bank transaction
func (s *Service) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error) {
	transaction, err := s.repo.GetTransaction(ctx, schemaName, tenantID, transactionID)
	if err != nil {
		return nil, err
	}
	normalizeTransactionReviewFields(transaction)
	return transaction, nil
}

// MatchTransaction matches a bank transaction to a payment
func (s *Service) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	return s.repo.MatchTransaction(ctx, schemaName, tenantID, transactionID, paymentID)
}

// UnmatchTransaction removes the match from a bank transaction
func (s *Service) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	return s.repo.UnmatchTransaction(ctx, schemaName, tenantID, transactionID)
}

// UpdateTransactionReview updates accountant follow-up metadata for a bank transaction.
func (s *Service) UpdateTransactionReview(ctx context.Context, schemaName, tenantID, transactionID, reviewerID string, req *UpdateTransactionReviewRequest) (*BankTransaction, error) {
	if req == nil || (req.FollowUpStatus == nil && req.ReviewNote == nil) {
		return nil, fmt.Errorf("at least one review field is required")
	}

	update := TransactionReviewUpdate{
		ReviewedBy: reviewerID,
		ReviewedAt: time.Now().UTC(),
	}

	if req.FollowUpStatus != nil {
		normalized, err := NormalizeFollowUpStatus(string(*req.FollowUpStatus))
		if err != nil {
			return nil, err
		}
		update.FollowUpStatus = &normalized
	}

	if req.ReviewNote != nil {
		trimmed := strings.TrimSpace(*req.ReviewNote)
		if len(trimmed) > 2000 {
			return nil, fmt.Errorf("review note must be 2000 characters or less")
		}
		update.ReviewNote = &trimmed
	}

	transaction, err := s.repo.UpdateTransactionReview(ctx, schemaName, tenantID, transactionID, update)
	if err != nil {
		return nil, err
	}

	normalizeTransactionReviewFields(transaction)
	return transaction, nil
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

func normalizeTransactionSlice(transactions []BankTransaction) {
	for i := range transactions {
		normalizeTransactionReviewFields(&transactions[i])
	}
}

func normalizeTransactionReviewFields(transaction *BankTransaction) {
	if transaction == nil || transaction.FollowUpStatus != "" {
		return
	}
	transaction.FollowUpStatus = FollowUpNone
}
