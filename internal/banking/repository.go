package banking

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
)

// Repository defines the contract for banking data access.
type Repository interface {
	CreateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error
	GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error)
	ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error)
	UpdateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error
	DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error
	UnsetDefaultAccounts(ctx context.Context, schemaName, tenantID string) error
	CountTransactionsForAccount(ctx context.Context, schemaName, accountID string) (int, error)
	CalculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error)

	CreateBankMatchRule(ctx context.Context, schemaName string, rule *BankMatchRule) error
	GetBankMatchRule(ctx context.Context, schemaName, tenantID, ruleID string) (*BankMatchRule, error)
	ListBankMatchRules(ctx context.Context, schemaName, tenantID string, filter *BankMatchRuleFilter) ([]BankMatchRule, error)
	UpdateBankMatchRule(ctx context.Context, schemaName string, rule *BankMatchRule) error
	DeleteBankMatchRule(ctx context.Context, schemaName, tenantID, ruleID string) error

	ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error)
	GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error)
	ListPaymentMatchCandidates(ctx context.Context, schemaName, tenantID string, paymentType payments.PaymentType, amount decimal.Decimal, limit int) ([]PaymentForMatching, error)
	MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error
	UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error
	UpdateTransactionReview(ctx context.Context, schemaName, tenantID, transactionID string, update TransactionReviewUpdate) (*BankTransaction, error)
	CreateTransaction(ctx context.Context, schemaName string, t *BankTransaction) error
	CreatePaymentFromTransaction(ctx context.Context, schemaName, tenantID, userID string, transaction *BankTransaction) (string, error)
	IsTransactionDuplicate(ctx context.Context, schemaName, tenantID, bankAccountID string, date time.Time, amount decimal.Decimal, externalID string) (bool, error)

	CreateReconciliation(ctx context.Context, schemaName string, r *BankReconciliation) error
	GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error)
	ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error)
	CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error
	AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error

	CreateImportRecord(ctx context.Context, schemaName string, imp *BankStatementImport) error
	IncrementLatestImportMatchedCount(ctx context.Context, schemaName, tenantID, bankAccountID string, matchedCount int) error
	GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error)
}

var (
	ErrBankAccountNotFound       = fmt.Errorf("bank account not found")
	ErrTransactionNotFound       = fmt.Errorf("transaction not found")
	ErrReconciliationNotFound    = fmt.Errorf("reconciliation not found")
	ErrAccountHasTransactions    = fmt.Errorf("cannot delete bank account with transactions")
	ErrTransactionAlreadyMatched = fmt.Errorf("transaction not found or already matched")
	ErrTransactionNotMatched     = fmt.Errorf("transaction not found or not matched")
	ErrReconciliationAlreadyDone = fmt.Errorf("reconciliation not found or already completed")
	ErrBankMatchRuleNotFound     = fmt.Errorf("bank match rule not found")
)
