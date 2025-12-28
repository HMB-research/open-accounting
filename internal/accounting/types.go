package accounting

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
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
	StatusDraft  JournalEntryStatus = "DRAFT"
	StatusPosted JournalEntryStatus = "POSTED"
	StatusVoided JournalEntryStatus = "VOIDED"
)

// Account represents a GL account in the chart of accounts
type Account struct {
	ID          string      `json:"id"`
	TenantID    string      `json:"tenant_id"`
	Code        string      `json:"code"`
	Name        string      `json:"name"`
	AccountType AccountType `json:"account_type"`
	ParentID    *string     `json:"parent_id,omitempty"`
	IsActive    bool        `json:"is_active"`
	IsSystem    bool        `json:"is_system"`
	Description string      `json:"description,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// JournalEntry represents an immutable accounting transaction
type JournalEntry struct {
	ID          string             `json:"id"`
	TenantID    string             `json:"tenant_id"`
	EntryNumber string             `json:"entry_number"`
	EntryDate   time.Time          `json:"entry_date"`
	Description string             `json:"description"`
	Reference   string             `json:"reference,omitempty"`
	SourceType  string             `json:"source_type,omitempty"`
	SourceID    *string            `json:"source_id,omitempty"`
	Status      JournalEntryStatus `json:"status"`
	Lines       []JournalEntryLine `json:"lines"`
	PostedAt    *time.Time         `json:"posted_at,omitempty"`
	PostedBy    *string            `json:"posted_by,omitempty"`
	VoidedAt    *time.Time         `json:"voided_at,omitempty"`
	VoidedBy    *string            `json:"voided_by,omitempty"`
	VoidReason  string             `json:"void_reason,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	CreatedBy   string             `json:"created_by"`
}

// JournalEntryLine represents a single debit or credit in a journal entry
type JournalEntryLine struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	JournalEntryID string          `json:"journal_entry_id"`
	AccountID      string          `json:"account_id"`
	Account        *Account        `json:"account,omitempty"`
	Description    string          `json:"description,omitempty"`
	DebitAmount    decimal.Decimal `json:"debit_amount"`
	CreditAmount   decimal.Decimal `json:"credit_amount"`
	Currency       string          `json:"currency"`
	ExchangeRate   decimal.Decimal `json:"exchange_rate"`
	BaseDebit      decimal.Decimal `json:"base_debit"`
	BaseCredit     decimal.Decimal `json:"base_credit"`
}

// Validate ensures the journal entry is balanced and valid
func (je *JournalEntry) Validate() error {
	if len(je.Lines) == 0 {
		return errors.New("journal entry must have at least one line")
	}

	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, line := range je.Lines {
		// Validate line has either debit or credit, not both
		if line.DebitAmount.GreaterThan(decimal.Zero) && line.CreditAmount.GreaterThan(decimal.Zero) {
			return errors.New("line cannot have both debit and credit amounts")
		}
		if line.DebitAmount.LessThan(decimal.Zero) || line.CreditAmount.LessThan(decimal.Zero) {
			return errors.New("amounts cannot be negative")
		}

		totalDebits = totalDebits.Add(line.BaseDebit)
		totalCredits = totalCredits.Add(line.BaseCredit)
	}

	if !totalDebits.Equal(totalCredits) {
		return fmt.Errorf("journal entry does not balance: debits=%s, credits=%s",
			totalDebits.String(), totalCredits.String())
	}

	if totalDebits.IsZero() {
		return errors.New("journal entry cannot have zero amounts")
	}

	return nil
}

// TotalDebits returns the sum of all debit amounts
func (je *JournalEntry) TotalDebits() decimal.Decimal {
	total := decimal.Zero
	for _, line := range je.Lines {
		total = total.Add(line.BaseDebit)
	}
	return total
}

// TotalCredits returns the sum of all credit amounts
func (je *JournalEntry) TotalCredits() decimal.Decimal {
	total := decimal.Zero
	for _, line := range je.Lines {
		total = total.Add(line.BaseCredit)
	}
	return total
}

// IsBalanced returns true if debits equal credits
func (je *JournalEntry) IsBalanced() bool {
	return je.TotalDebits().Equal(je.TotalCredits())
}

// CreateJournalEntryRequest is the request to create a new journal entry
type CreateJournalEntryRequest struct {
	EntryDate   time.Time                   `json:"entry_date"`
	Description string                      `json:"description"`
	Reference   string                      `json:"reference,omitempty"`
	SourceType  string                      `json:"source_type,omitempty"`
	SourceID    *string                     `json:"source_id,omitempty"`
	Lines       []CreateJournalEntryLineReq `json:"lines"`
	UserID      string                      `json:"-"`
}

// CreateJournalEntryLineReq is a line in the create request
type CreateJournalEntryLineReq struct {
	AccountID    string          `json:"account_id"`
	Description  string          `json:"description,omitempty"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
	Currency     string          `json:"currency,omitempty"`
	ExchangeRate decimal.Decimal `json:"exchange_rate,omitempty"`
}

// AccountBalance represents an account's balance at a point in time
type AccountBalance struct {
	AccountID     string          `json:"account_id"`
	AccountCode   string          `json:"account_code"`
	AccountName   string          `json:"account_name"`
	AccountType   AccountType     `json:"account_type"`
	DebitBalance  decimal.Decimal `json:"debit_balance"`
	CreditBalance decimal.Decimal `json:"credit_balance"`
	NetBalance    decimal.Decimal `json:"net_balance"`
}
