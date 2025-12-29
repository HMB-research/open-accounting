package banking

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionStatus represents the reconciliation status of a bank transaction
type TransactionStatus string

const (
	StatusUnmatched  TransactionStatus = "UNMATCHED"
	StatusMatched    TransactionStatus = "MATCHED"
	StatusReconciled TransactionStatus = "RECONCILED"
)

// ReconciliationStatus represents the status of a reconciliation session
type ReconciliationStatus string

const (
	ReconciliationInProgress ReconciliationStatus = "IN_PROGRESS"
	ReconciliationCompleted  ReconciliationStatus = "COMPLETED"
)

// BankAccount represents a bank account
type BankAccount struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	Name          string          `json:"name"`
	AccountNumber string          `json:"account_number"`
	BankName      string          `json:"bank_name,omitempty"`
	SwiftCode     string          `json:"swift_code,omitempty"`
	Currency      string          `json:"currency"`
	GLAccountID   *string         `json:"gl_account_id,omitempty"`
	IsDefault     bool            `json:"is_default"`
	IsActive      bool            `json:"is_active"`
	CreatedAt     time.Time       `json:"created_at"`
	Balance       decimal.Decimal `json:"balance,omitempty"` // Calculated field
}

// BankTransaction represents an imported bank transaction
type BankTransaction struct {
	ID                  string            `json:"id"`
	TenantID            string            `json:"tenant_id"`
	BankAccountID       string            `json:"bank_account_id"`
	TransactionDate     time.Time         `json:"transaction_date"`
	ValueDate           *time.Time        `json:"value_date,omitempty"`
	Amount              decimal.Decimal   `json:"amount"`
	Currency            string            `json:"currency"`
	Description         string            `json:"description,omitempty"`
	Reference           string            `json:"reference,omitempty"`
	CounterpartyName    string            `json:"counterparty_name,omitempty"`
	CounterpartyAccount string            `json:"counterparty_account,omitempty"`
	Status              TransactionStatus `json:"status"`
	MatchedPaymentID    *string           `json:"matched_payment_id,omitempty"`
	JournalEntryID      *string           `json:"journal_entry_id,omitempty"`
	ReconciliationID    *string           `json:"reconciliation_id,omitempty"`
	ImportedAt          time.Time         `json:"imported_at"`
	ExternalID          string            `json:"external_id,omitempty"`
}

// BankReconciliation represents a reconciliation session
type BankReconciliation struct {
	ID             string               `json:"id"`
	TenantID       string               `json:"tenant_id"`
	BankAccountID  string               `json:"bank_account_id"`
	StatementDate  time.Time            `json:"statement_date"`
	OpeningBalance decimal.Decimal      `json:"opening_balance"`
	ClosingBalance decimal.Decimal      `json:"closing_balance"`
	Status         ReconciliationStatus `json:"status"`
	CompletedAt    *time.Time           `json:"completed_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	CreatedBy      string               `json:"created_by"`
}

// BankStatementImport tracks an import session
type BankStatementImport struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id"`
	BankAccountID        string    `json:"bank_account_id"`
	FileName             string    `json:"file_name"`
	TransactionsImported int       `json:"transactions_imported"`
	TransactionsMatched  int       `json:"transactions_matched"`
	DuplicatesSkipped    int       `json:"duplicates_skipped"`
	CreatedAt            time.Time `json:"created_at"`
}

// MatchSuggestion represents a suggested match between bank transaction and payment
type MatchSuggestion struct {
	PaymentID     string          `json:"payment_id"`
	PaymentNumber string          `json:"payment_number"`
	PaymentDate   time.Time       `json:"payment_date"`
	Amount        decimal.Decimal `json:"amount"`
	ContactName   string          `json:"contact_name,omitempty"`
	Reference     string          `json:"reference,omitempty"`
	Confidence    float64         `json:"confidence"` // 0.0 - 1.0
	MatchReason   string          `json:"match_reason"`
}

// CreateBankAccountRequest is the request to create a bank account
type CreateBankAccountRequest struct {
	Name          string  `json:"name"`
	AccountNumber string  `json:"account_number"`
	BankName      string  `json:"bank_name,omitempty"`
	SwiftCode     string  `json:"swift_code,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	GLAccountID   *string `json:"gl_account_id,omitempty"`
	IsDefault     bool    `json:"is_default"`
}

// UpdateBankAccountRequest is the request to update a bank account
type UpdateBankAccountRequest struct {
	Name        string  `json:"name,omitempty"`
	BankName    string  `json:"bank_name,omitempty"`
	SwiftCode   string  `json:"swift_code,omitempty"`
	GLAccountID *string `json:"gl_account_id,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	IsDefault   *bool   `json:"is_default,omitempty"`
}

// ImportCSVRequest is the request to import transactions from CSV
type ImportCSVRequest struct {
	FileName       string              `json:"-"`
	Transactions   []CSVTransactionRow `json:"transactions"`
	SkipDuplicates bool                `json:"skip_duplicates"`
}

// CSVTransactionRow represents a row in the CSV import
type CSVTransactionRow struct {
	Date                string `json:"date"`
	ValueDate           string `json:"value_date,omitempty"`
	Amount              string `json:"amount"`
	Description         string `json:"description"`
	Reference           string `json:"reference,omitempty"`
	CounterpartyName    string `json:"counterparty_name,omitempty"`
	CounterpartyAccount string `json:"counterparty_account,omitempty"`
	ExternalID          string `json:"external_id,omitempty"`
}

// ImportResult is the result of a CSV import
type ImportResult struct {
	ImportID             string   `json:"import_id"`
	TransactionsImported int      `json:"transactions_imported"`
	TransactionsMatched  int      `json:"transactions_matched"`
	DuplicatesSkipped    int      `json:"duplicates_skipped"`
	Errors               []string `json:"errors,omitempty"`
}

// CreateReconciliationRequest is the request to start a reconciliation
type CreateReconciliationRequest struct {
	StatementDate  string          `json:"statement_date"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
	ClosingBalance decimal.Decimal `json:"closing_balance"`
}

// MatchTransactionRequest is the request to match a transaction
type MatchTransactionRequest struct {
	PaymentID string `json:"payment_id"`
}

// TransactionFilter provides filtering options for bank transactions
type TransactionFilter struct {
	BankAccountID string
	Status        TransactionStatus
	FromDate      *time.Time
	ToDate        *time.Time
	MinAmount     *decimal.Decimal
	MaxAmount     *decimal.Decimal
}

// BankAccountFilter provides filtering options for bank accounts
type BankAccountFilter struct {
	IsActive *bool
	Currency string
}
