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

const (
	// SourceTypeJournalTemplate marks journal entries generated from reusable templates.
	SourceTypeJournalTemplate = "JOURNAL_TEMPLATE"
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

// ImportAccountsRequest contains CSV payload for bulk account import.
type ImportAccountsRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportAccountsResult summarizes a bulk account import.
type ImportAccountsResult struct {
	FileName        string                   `json:"file_name,omitempty"`
	RowsProcessed   int                      `json:"rows_processed"`
	AccountsCreated int                      `json:"accounts_created"`
	RowsSkipped     int                      `json:"rows_skipped"`
	Errors          []ImportAccountsRowError `json:"errors,omitempty"`
}

// ImportAccountsRowError describes a row-level import failure.
type ImportAccountsRowError struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// ImportOpeningBalancesRequest contains CSV payload for opening-balance import.
type ImportOpeningBalancesRequest struct {
	EntryDate   string `json:"entry_date"`
	CSVContent  string `json:"csv_content"`
	FileName    string `json:"file_name,omitempty"`
	Description string `json:"description,omitempty"`
	Reference   string `json:"reference,omitempty"`
	UserID      string `json:"-"`
}

// ImportOpeningBalancesResult summarizes a successful opening-balance import.
type ImportOpeningBalancesResult struct {
	FileName      string          `json:"file_name,omitempty"`
	RowsProcessed int             `json:"rows_processed"`
	LinesImported int             `json:"lines_imported"`
	TotalDebit    decimal.Decimal `json:"total_debit"`
	TotalCredit   decimal.Decimal `json:"total_credit"`
	JournalEntry  *JournalEntry   `json:"journal_entry"`
}

// ImportJournalEntriesRequest contains CSV payload for historical journal import.
type ImportJournalEntriesRequest struct {
	CSVContent     string     `json:"csv_content"`
	FileName       string     `json:"file_name,omitempty"`
	SourceType     string     `json:"source_type,omitempty"`
	PostEntries    bool       `json:"post_entries,omitempty"`
	UserID         string     `json:"-"`
	PeriodLockDate *time.Time `json:"-"`
}

// ImportJournalEntriesResult summarizes a historical journal CSV import.
type ImportJournalEntriesResult struct {
	FileName       string                         `json:"file_name,omitempty"`
	RowsProcessed  int                            `json:"rows_processed"`
	EntriesCreated int                            `json:"entries_created"`
	LinesImported  int                            `json:"lines_imported"`
	RowsSkipped    int                            `json:"rows_skipped"`
	TotalDebit     decimal.Decimal                `json:"total_debit"`
	TotalCredit    decimal.Decimal                `json:"total_credit"`
	JournalEntries []JournalEntry                 `json:"journal_entries,omitempty"`
	Errors         []ImportJournalEntriesRowError `json:"errors,omitempty"`
}

// ImportJournalEntriesRowError describes a journal import group or row failure.
type ImportJournalEntriesRowError struct {
	Row            int    `json:"row"`
	EntryReference string `json:"entry_reference,omitempty"`
	Message        string `json:"message"`
}

// JournalEntry represents an immutable accounting transaction
type JournalEntry struct {
	ID               string             `json:"id"`
	TenantID         string             `json:"tenant_id"`
	EntryNumber      string             `json:"entry_number"`
	EntryDate        time.Time          `json:"entry_date"`
	Description      string             `json:"description"`
	Reference        string             `json:"reference,omitempty"`
	SourceType       string             `json:"source_type,omitempty"`
	SourceID         *string            `json:"source_id,omitempty"`
	RequiresEvidence bool               `json:"requires_evidence"`
	Status           JournalEntryStatus `json:"status"`
	Lines            []JournalEntryLine `json:"lines"`
	PostedAt         *time.Time         `json:"posted_at,omitempty"`
	PostedBy         *string            `json:"posted_by,omitempty"`
	VoidedAt         *time.Time         `json:"voided_at,omitempty"`
	VoidedBy         *string            `json:"voided_by,omitempty"`
	VoidReason       string             `json:"void_reason,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	CreatedBy        string             `json:"created_by"`
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
	EntryDate        time.Time                   `json:"entry_date"`
	Description      string                      `json:"description"`
	Reference        string                      `json:"reference,omitempty"`
	SourceType       string                      `json:"source_type,omitempty"`
	SourceID         *string                     `json:"source_id,omitempty"`
	RequiresEvidence bool                        `json:"requires_evidence,omitempty"`
	Lines            []CreateJournalEntryLineReq `json:"lines"`
	UserID           string                      `json:"-"`
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

// JournalEntryTemplate represents a reusable balanced journal entry pattern.
type JournalEntryTemplate struct {
	ID               string                     `json:"id"`
	TenantID         string                     `json:"tenant_id"`
	Name             string                     `json:"name"`
	Description      string                     `json:"description"`
	Reference        string                     `json:"reference,omitempty"`
	RequiresEvidence bool                       `json:"requires_evidence"`
	IsActive         bool                       `json:"is_active"`
	LineCount        int                        `json:"line_count"`
	Lines            []JournalEntryTemplateLine `json:"lines,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
	CreatedBy        string                     `json:"created_by"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

// JournalEntryTemplateLine is one line in a reusable journal entry template.
type JournalEntryTemplateLine struct {
	ID           string          `json:"id"`
	TemplateID   string          `json:"template_id"`
	LineNumber   int             `json:"line_number"`
	AccountID    string          `json:"account_id"`
	Description  string          `json:"description,omitempty"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
	Currency     string          `json:"currency"`
	ExchangeRate decimal.Decimal `json:"exchange_rate"`
}

// CreateJournalEntryTemplateRequest is the request to create a journal entry template.
type CreateJournalEntryTemplateRequest struct {
	Name             string                      `json:"name"`
	Description      string                      `json:"description"`
	Reference        string                      `json:"reference,omitempty"`
	RequiresEvidence bool                        `json:"requires_evidence,omitempty"`
	Lines            []CreateJournalEntryLineReq `json:"lines"`
	UserID           string                      `json:"-"`
}

// ApplyJournalEntryTemplateRequest creates a journal entry from a template.
type ApplyJournalEntryTemplateRequest struct {
	EntryDate   time.Time `json:"entry_date"`
	Description string    `json:"description,omitempty"`
	Reference   string    `json:"reference,omitempty"`
	Post        bool      `json:"post,omitempty"`
	UserID      string    `json:"-"`
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

// AccountGroup represents a group of accounts in a financial report
type AccountGroup struct {
	Code     string           `json:"code"`
	Name     string           `json:"name"`
	Balance  decimal.Decimal  `json:"balance"`
	Children []AccountBalance `json:"children,omitempty"`
}

// BalanceSheet represents a balance sheet report as of a specific date
type BalanceSheet struct {
	TenantID         string           `json:"tenant_id"`
	AsOfDate         time.Time        `json:"as_of_date"`
	GeneratedAt      time.Time        `json:"generated_at"`
	Assets           []AccountBalance `json:"assets"`
	Liabilities      []AccountBalance `json:"liabilities"`
	Equity           []AccountBalance `json:"equity"`
	TotalAssets      decimal.Decimal  `json:"total_assets"`
	TotalLiabilities decimal.Decimal  `json:"total_liabilities"`
	TotalEquity      decimal.Decimal  `json:"total_equity"`
	RetainedEarnings decimal.Decimal  `json:"retained_earnings"`
	IsBalanced       bool             `json:"is_balanced"`
}

// IncomeStatement represents an income statement (P&L) for a period
type IncomeStatement struct {
	TenantID      string           `json:"tenant_id"`
	StartDate     time.Time        `json:"start_date"`
	EndDate       time.Time        `json:"end_date"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Revenue       []AccountBalance `json:"revenue"`
	Expenses      []AccountBalance `json:"expenses"`
	TotalRevenue  decimal.Decimal  `json:"total_revenue"`
	TotalExpenses decimal.Decimal  `json:"total_expenses"`
	NetIncome     decimal.Decimal  `json:"net_income"`
}
