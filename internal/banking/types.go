package banking

import (
	"fmt"
	"strings"
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

// FollowUpStatus represents accountant follow-up guidance on a bank transaction.
type FollowUpStatus string

const (
	FollowUpNone             FollowUpStatus = "NONE"
	FollowUpEvidenceRequired FollowUpStatus = "EVIDENCE_REQUIRED"
	FollowUpReadyToMatch     FollowUpStatus = "READY_TO_MATCH"
)

// ReconciliationStatus represents the status of a reconciliation session
type ReconciliationStatus string

const (
	ReconciliationInProgress ReconciliationStatus = "IN_PROGRESS"
	ReconciliationCompleted  ReconciliationStatus = "COMPLETED"
)

// BankMatchField defines the transaction field inspected by a bank match rule.
type BankMatchField string

const (
	BankMatchFieldDescription         BankMatchField = "DESCRIPTION"
	BankMatchFieldReference           BankMatchField = "REFERENCE"
	BankMatchFieldCounterpartyName    BankMatchField = "COUNTERPARTY_NAME"
	BankMatchFieldCounterpartyAccount BankMatchField = "COUNTERPARTY_ACCOUNT"
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
	FollowUpStatus      FollowUpStatus    `json:"follow_up_status"`
	ReviewNote          string            `json:"review_note,omitempty"`
	ReviewedBy          *string           `json:"reviewed_by,omitempty"`
	ReviewedAt          *time.Time        `json:"reviewed_at,omitempty"`
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

// BankMatchRule tunes automatic matching for transactions that match a pattern.
type BankMatchRule struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	BankAccountID      *string        `json:"bank_account_id,omitempty"`
	Name               string         `json:"name"`
	Priority           int            `json:"priority"`
	MatchField         BankMatchField `json:"match_field"`
	Pattern            string         `json:"pattern"`
	MinConfidence      float64        `json:"min_confidence"`
	MaxDateDiffDays    int            `json:"max_date_diff_days"`
	RequireExactAmount bool           `json:"require_exact_amount"`
	IsActive           bool           `json:"is_active"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
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
	IsActive      *bool   `json:"is_active,omitempty"`
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

// CreateBankMatchRuleRequest is the request to create a bank auto-match rule.
type CreateBankMatchRuleRequest struct {
	BankAccountID      *string        `json:"bank_account_id,omitempty"`
	Name               string         `json:"name"`
	Priority           int            `json:"priority,omitempty"`
	MatchField         BankMatchField `json:"match_field,omitempty"`
	Pattern            string         `json:"pattern"`
	MinConfidence      float64        `json:"min_confidence,omitempty"`
	MaxDateDiffDays    int            `json:"max_date_diff_days,omitempty"`
	RequireExactAmount bool           `json:"require_exact_amount,omitempty"`
	IsActive           *bool          `json:"is_active,omitempty"`
}

// UpdateBankMatchRuleRequest is the request to update a bank auto-match rule.
type UpdateBankMatchRuleRequest struct {
	BankAccountID      *string         `json:"bank_account_id,omitempty"`
	ClearBankAccount   bool            `json:"clear_bank_account,omitempty"`
	Name               *string         `json:"name,omitempty"`
	Priority           *int            `json:"priority,omitempty"`
	MatchField         *BankMatchField `json:"match_field,omitempty"`
	Pattern            *string         `json:"pattern,omitempty"`
	MinConfidence      *float64        `json:"min_confidence,omitempty"`
	MaxDateDiffDays    *int            `json:"max_date_diff_days,omitempty"`
	RequireExactAmount *bool           `json:"require_exact_amount,omitempty"`
	IsActive           *bool           `json:"is_active,omitempty"`
}

// ImportCSVRequest is the request to import bank transactions from raw statement
// content or already normalized rows.
type ImportCSVRequest struct {
	FileName       string              `json:"file_name,omitempty"`
	CSVContent     string              `json:"csv_content,omitempty"`
	Format         string              `json:"format,omitempty"`
	Transactions   []CSVTransactionRow `json:"transactions,omitempty"`
	SkipDuplicates bool                `json:"skip_duplicates"`
}

// ImportBankAccountsRequest is the request to import bank account master data.
type ImportBankAccountsRequest struct {
	FileName       string              `json:"file_name,omitempty"`
	Rows           []CSVBankAccountRow `json:"rows"`
	SkipDuplicates bool                `json:"skip_duplicates"`
}

// CSVBankAccountRow represents a bank account row in CSV import payloads.
type CSVBankAccountRow struct {
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	BankName      string `json:"bank_name,omitempty"`
	SwiftCode     string `json:"swift_code,omitempty"`
	Currency      string `json:"currency,omitempty"`
	GLAccountID   string `json:"gl_account_id,omitempty"`
	IsDefault     string `json:"is_default,omitempty"`
	IsActive      string `json:"is_active,omitempty"`
}

// ImportBankAccountsResult is the result of a bank account import.
type ImportBankAccountsResult struct {
	FileName         string   `json:"file_name"`
	RowsProcessed    int      `json:"rows_processed"`
	AccountsImported int      `json:"accounts_imported"`
	RowsSkipped      int      `json:"rows_skipped"`
	Errors           []string `json:"errors,omitempty"`
}

// CSVTransactionRow represents a row in the CSV import
type CSVTransactionRow struct {
	Date                string `json:"date"`
	ValueDate           string `json:"value_date,omitempty"`
	Amount              string `json:"amount"`
	Currency            string `json:"currency,omitempty"`
	SourceAccount       string `json:"source_account,omitempty"`
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

// UpdateTransactionReviewRequest captures accountant follow-up updates for a bank transaction.
type UpdateTransactionReviewRequest struct {
	FollowUpStatus *FollowUpStatus `json:"follow_up_status,omitempty"`
	ReviewNote     *string         `json:"review_note,omitempty"`
}

// TransactionReviewUpdate is the internal mutation payload for bank transaction review metadata.
type TransactionReviewUpdate struct {
	FollowUpStatus *FollowUpStatus
	ReviewNote     *string
	ReviewedBy     string
	ReviewedAt     time.Time
}

// TransactionFilter provides filtering options for bank transactions
type TransactionFilter struct {
	BankAccountID    string
	Status           TransactionStatus
	ReconciliationID string
	FromDate         *time.Time
	ToDate           *time.Time
	MinAmount        *decimal.Decimal
	MaxAmount        *decimal.Decimal
}

// BankAccountFilter provides filtering options for bank accounts
type BankAccountFilter struct {
	IsActive *bool
	Currency string
}

// BankMatchRuleFilter provides filtering options for auto-match rules.
type BankMatchRuleFilter struct {
	BankAccountID string
	ActiveOnly    bool
	IncludeGlobal bool
}

// NormalizeFollowUpStatus validates and normalizes a follow-up status value.
func NormalizeFollowUpStatus(value string) (FollowUpStatus, error) {
	normalized := FollowUpStatus(strings.ToUpper(strings.TrimSpace(value)))
	switch normalized {
	case FollowUpNone, FollowUpEvidenceRequired, FollowUpReadyToMatch:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid follow-up status")
	}
}

// NormalizeBankMatchField validates and normalizes a bank match field.
func NormalizeBankMatchField(value string) (BankMatchField, error) {
	normalized := BankMatchField(strings.ToUpper(strings.TrimSpace(value)))
	if normalized == "" {
		return BankMatchFieldDescription, nil
	}
	switch normalized {
	case BankMatchFieldDescription, BankMatchFieldReference, BankMatchFieldCounterpartyName, BankMatchFieldCounterpartyAccount:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid bank match field")
	}
}
