package models

import (
	"time"
)

// AccountType represents the type of account in the chart of accounts
type AccountType string

const (
	AccountTypeAsset     AccountType = "ASSET"
	AccountTypeLiability AccountType = "LIABILITY"
	AccountTypeEquity    AccountType = "EQUITY"
	AccountTypeRevenue   AccountType = "REVENUE"
	AccountTypeExpense   AccountType = "EXPENSE"
)

// IsDebitNormal returns true if account type normally has debit balance
func (t AccountType) IsDebitNormal() bool {
	return t == AccountTypeAsset || t == AccountTypeExpense
}

// JournalEntryStatus represents the lifecycle status of a journal entry
type JournalEntryStatus string

const (
	JournalStatusDraft  JournalEntryStatus = "DRAFT"
	JournalStatusPosted JournalEntryStatus = "POSTED"
	JournalStatusVoided JournalEntryStatus = "VOIDED"
)

// Account represents a GL account in the chart of accounts (GORM model)
type Account struct {
	ID          string      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Code        string      `gorm:"size:20;not null" json:"code"`
	Name        string      `gorm:"size:255;not null" json:"name"`
	AccountType AccountType `gorm:"column:account_type;size:20;not null" json:"account_type"`
	ParentID    *string     `gorm:"type:uuid" json:"parent_id,omitempty"`
	IsActive    bool        `gorm:"not null;default:true" json:"is_active"`
	IsSystem    bool        `gorm:"not null;default:false" json:"is_system"`
	Description string      `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time   `gorm:"not null;default:now()" json:"created_at"`

	// Relations
	Parent   *Account  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children []Account `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

// TableName returns the table name for GORM
func (Account) TableName() string {
	return "accounts"
}

// JournalEntry represents an immutable accounting transaction (GORM model)
type JournalEntry struct {
	ID          string             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string             `gorm:"type:uuid;not null;index" json:"tenant_id"`
	EntryNumber string             `gorm:"size:50;not null" json:"entry_number"`
	EntryDate   time.Time          `gorm:"type:date;not null" json:"entry_date"`
	Description string             `gorm:"type:text;not null" json:"description"`
	Reference   string             `gorm:"size:255" json:"reference,omitempty"`
	SourceType  string             `gorm:"size:50" json:"source_type,omitempty"`
	SourceID    *string            `gorm:"type:uuid" json:"source_id,omitempty"`
	Status      JournalEntryStatus `gorm:"size:20;not null;default:'DRAFT'" json:"status"`
	PostedAt    *time.Time         `gorm:"" json:"posted_at,omitempty"`
	PostedBy    *string            `gorm:"type:uuid" json:"posted_by,omitempty"`
	VoidedAt    *time.Time         `gorm:"" json:"voided_at,omitempty"`
	VoidedBy    *string            `gorm:"type:uuid" json:"voided_by,omitempty"`
	VoidReason  string             `gorm:"type:text" json:"void_reason,omitempty"`
	CreatedAt   time.Time          `gorm:"not null;default:now()" json:"created_at"`
	CreatedBy   string             `gorm:"type:uuid;not null" json:"created_by"`

	// Relations
	Lines []JournalEntryLine `gorm:"foreignKey:JournalEntryID" json:"lines,omitempty"`
}

// TableName returns the table name for GORM
func (JournalEntry) TableName() string {
	return "journal_entries"
}

// JournalEntryLine represents a single debit or credit in a journal entry (GORM model)
type JournalEntryLine struct {
	ID             string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	JournalEntryID string  `gorm:"column:journal_entry_id;type:uuid;not null;index" json:"journal_entry_id"`
	AccountID      string  `gorm:"type:uuid;not null;index" json:"account_id"`
	Description    string  `gorm:"type:text" json:"description,omitempty"`
	DebitAmount    Decimal `gorm:"type:numeric(28,8);not null;default:0" json:"debit_amount"`
	CreditAmount   Decimal `gorm:"type:numeric(28,8);not null;default:0" json:"credit_amount"`
	Currency       string  `gorm:"size:3;not null;default:'EUR'" json:"currency"`
	ExchangeRate   Decimal `gorm:"type:numeric(18,10);not null;default:1" json:"exchange_rate"`
	BaseDebit      Decimal `gorm:"type:numeric(28,8);not null;default:0" json:"base_debit"`
	BaseCredit     Decimal `gorm:"type:numeric(28,8);not null;default:0" json:"base_credit"`

	// Relations
	JournalEntry *JournalEntry `gorm:"foreignKey:JournalEntryID" json:"journal_entry,omitempty"`
	Account      *Account      `gorm:"foreignKey:AccountID" json:"account,omitempty"`
}

// TableName returns the table name for GORM
func (JournalEntryLine) TableName() string {
	return "journal_entry_lines"
}

// AccountBalance represents an account's balance at a point in time
type AccountBalance struct {
	AccountID     string      `json:"account_id"`
	AccountCode   string      `json:"account_code"`
	AccountName   string      `json:"account_name"`
	AccountType   AccountType `json:"account_type"`
	DebitBalance  Decimal     `json:"debit_balance"`
	CreditBalance Decimal     `json:"credit_balance"`
	NetBalance    Decimal     `json:"net_balance"`
}
