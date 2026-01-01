package models

import (
	"time"
)

// TransactionStatus represents the reconciliation status of a bank transaction
type TransactionStatus string

const (
	TransactionStatusUnmatched  TransactionStatus = "UNMATCHED"
	TransactionStatusMatched    TransactionStatus = "MATCHED"
	TransactionStatusReconciled TransactionStatus = "RECONCILED"
)

// ReconciliationStatus represents the status of a reconciliation session
type ReconciliationStatus string

const (
	ReconciliationInProgress ReconciliationStatus = "IN_PROGRESS"
	ReconciliationCompleted  ReconciliationStatus = "COMPLETED"
)

// BankAccount represents a bank account (GORM model)
type BankAccount struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID      string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name          string    `gorm:"size:255;not null" json:"name"`
	AccountNumber string    `gorm:"column:account_number;size:50;not null" json:"account_number"`
	BankName      string    `gorm:"column:bank_name;size:255" json:"bank_name,omitempty"`
	SwiftCode     string    `gorm:"column:swift_code;size:20" json:"swift_code,omitempty"`
	Currency      string    `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	GLAccountID   *string   `gorm:"column:gl_account_id;type:uuid" json:"gl_account_id,omitempty"`
	IsDefault     bool      `gorm:"column:is_default;not null;default:false" json:"is_default"`
	IsActive      bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt     time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	Transactions []BankTransaction `gorm:"foreignKey:BankAccountID" json:"transactions,omitempty"`
}

// TableName returns the table name for GORM
func (BankAccount) TableName() string {
	return "bank_accounts"
}

// BankTransaction represents an imported bank transaction (GORM model)
type BankTransaction struct {
	ID                  string            `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID            string            `gorm:"type:uuid;not null;index" json:"tenant_id"`
	BankAccountID       string            `gorm:"column:bank_account_id;type:uuid;not null;index" json:"bank_account_id"`
	TransactionDate     time.Time         `gorm:"column:transaction_date;type:date;not null" json:"transaction_date"`
	ValueDate           *time.Time        `gorm:"column:value_date;type:date" json:"value_date,omitempty"`
	Amount              Decimal           `gorm:"type:numeric(28,8);not null" json:"amount"`
	Currency            string            `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	Description         string            `gorm:"type:text" json:"description,omitempty"`
	Reference           string            `gorm:"size:255" json:"reference,omitempty"`
	CounterpartyName    string            `gorm:"column:counterparty_name;size:255" json:"counterparty_name,omitempty"`
	CounterpartyAccount string            `gorm:"column:counterparty_account;size:100" json:"counterparty_account,omitempty"`
	Status              TransactionStatus `gorm:"size:20;not null;default:'UNMATCHED'" json:"status"`
	MatchedPaymentID    *string           `gorm:"column:matched_payment_id;type:uuid" json:"matched_payment_id,omitempty"`
	JournalEntryID      *string           `gorm:"column:journal_entry_id;type:uuid" json:"journal_entry_id,omitempty"`
	ReconciliationID    *string           `gorm:"column:reconciliation_id;type:uuid" json:"reconciliation_id,omitempty"`
	ImportedAt          time.Time         `gorm:"column:imported_at;not null;default:now()" json:"imported_at"`
	ExternalID          string            `gorm:"column:external_id;size:255" json:"external_id,omitempty"`

	// Relations
	BankAccount *BankAccount `gorm:"foreignKey:BankAccountID" json:"bank_account,omitempty"`
}

// TableName returns the table name for GORM
func (BankTransaction) TableName() string {
	return "bank_transactions"
}

// BankReconciliation represents a reconciliation session (GORM model)
type BankReconciliation struct {
	ID             string               `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string               `gorm:"type:uuid;not null;index" json:"tenant_id"`
	BankAccountID  string               `gorm:"column:bank_account_id;type:uuid;not null;index" json:"bank_account_id"`
	StatementDate  time.Time            `gorm:"column:statement_date;type:date;not null" json:"statement_date"`
	OpeningBalance Decimal              `gorm:"column:opening_balance;type:numeric(28,8);not null" json:"opening_balance"`
	ClosingBalance Decimal              `gorm:"column:closing_balance;type:numeric(28,8);not null" json:"closing_balance"`
	Status         ReconciliationStatus `gorm:"size:20;not null;default:'IN_PROGRESS'" json:"status"`
	CompletedAt    *time.Time           `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt      time.Time            `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy      string               `gorm:"type:uuid;not null" json:"created_by"`

	// Relations
	BankAccount *BankAccount `gorm:"foreignKey:BankAccountID" json:"bank_account,omitempty"`
}

// TableName returns the table name for GORM
func (BankReconciliation) TableName() string {
	return "bank_reconciliations"
}

// BankStatementImport tracks an import session (GORM model)
type BankStatementImport struct {
	ID                   string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	BankAccountID        string    `gorm:"column:bank_account_id;type:uuid;not null;index" json:"bank_account_id"`
	FileName             string    `gorm:"column:file_name;size:255;not null" json:"file_name"`
	TransactionsImported int       `gorm:"column:transactions_imported;not null;default:0" json:"transactions_imported"`
	TransactionsMatched  int       `gorm:"column:transactions_matched;not null;default:0" json:"transactions_matched"`
	DuplicatesSkipped    int       `gorm:"column:duplicates_skipped;not null;default:0" json:"duplicates_skipped"`
	CreatedAt            time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	BankAccount *BankAccount `gorm:"foreignKey:BankAccountID" json:"bank_account,omitempty"`
}

// TableName returns the table name for GORM
func (BankStatementImport) TableName() string {
	return "bank_statement_imports"
}
