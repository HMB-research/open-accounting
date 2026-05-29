package reports

import (
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
)

// AnnualReport bundles the fiscal-year close pack with a cash-flow statement.
type AnnualReport struct {
	TenantID            string                         `json:"tenant_id"`
	PeriodEndDate       string                         `json:"period_end_date"`
	FiscalYearLabel     string                         `json:"fiscal_year_label"`
	FiscalYearStartDate string                         `json:"fiscal_year_start_date"`
	FiscalYearEndDate   string                         `json:"fiscal_year_end_date"`
	CloseStatus         *accounting.YearEndCloseStatus `json:"close_status"`
	TrialBalance        *accounting.TrialBalance       `json:"trial_balance"`
	BalanceSheet        *accounting.BalanceSheet       `json:"balance_sheet"`
	IncomeStatement     *accounting.IncomeStatement    `json:"income_statement"`
	CashFlowStatement   *CashFlowStatement             `json:"cash_flow_statement"`
	GeneratedAt         time.Time                      `json:"generated_at"`
}

// CashFlowStatement represents an Estonian-standard cash flow statement
type CashFlowStatement struct {
	TenantID            string                    `json:"tenant_id"`
	StartDate           string                    `json:"start_date"`
	EndDate             string                    `json:"end_date"`
	Method              string                    `json:"method"`
	MappingOverrides    *CashFlowMappingOverrides `json:"mapping_overrides,omitempty"`
	OperatingActivities []CashFlowItem            `json:"operating_activities"`
	InvestingActivities []CashFlowItem            `json:"investing_activities"`
	FinancingActivities []CashFlowItem            `json:"financing_activities"`
	TotalOperating      decimal.Decimal           `json:"total_operating"`
	TotalInvesting      decimal.Decimal           `json:"total_investing"`
	TotalFinancing      decimal.Decimal           `json:"total_financing"`
	NetCashChange       decimal.Decimal           `json:"net_cash_change"`
	OpeningCash         decimal.Decimal           `json:"opening_cash"`
	ClosingCash         decimal.Decimal           `json:"closing_cash"`
	GeneratedAt         time.Time                 `json:"generated_at"`
}

// CashFlowItem represents a line item in the cash flow statement
type CashFlowItem struct {
	Code          string          `json:"code"`
	Description   string          `json:"description"`
	DescriptionET string          `json:"description_et"`
	Amount        decimal.Decimal `json:"amount"`
	IsSubtotal    bool            `json:"is_subtotal"`
}

// CashFlowRequest represents a request to generate cash flow statement
type CashFlowRequest struct {
	StartDate        string                   `json:"start_date"`
	EndDate          string                   `json:"end_date"`
	Method           string                   `json:"method,omitempty"`       // "direct" or "indirect"
	CompareType      string                   `json:"compare_type,omitempty"` // "", "previous_months", "previous_years", "quarters", "objects"
	MappingOverrides CashFlowMappingOverrides `json:"mapping_overrides,omitempty"`
}

// CashFlowMappingOverrides contains per-request account-code mapping overrides for custom charts.
type CashFlowMappingOverrides struct {
	OperatingAccountCodes []string `json:"operating_account_codes,omitempty"`
	InvestingAccountCodes []string `json:"investing_account_codes,omitempty"`
	FinancingAccountCodes []string `json:"financing_account_codes,omitempty"`
}

// UpdateCashFlowMappingRequest updates tenant-level cash-flow account mappings.
type UpdateCashFlowMappingRequest = CashFlowMappingOverrides

// JournalEntryWithLines represents a journal entry with its lines for reporting
type JournalEntryWithLines struct {
	ID          string        `json:"id"`
	EntryDate   time.Time     `json:"entry_date"`
	Description string        `json:"description"`
	Lines       []JournalLine `json:"lines"`
}

// JournalLine represents a single journal line
type JournalLine struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	AccountType string          `json:"account_type"`
	Debit       decimal.Decimal `json:"debit"`
	Credit      decimal.Decimal `json:"credit"`
}

// Estonian cash flow codes
const (
	// Operating Activities - Rahavood äritegevusest
	CFOperReceipts   = "CF_OPER_RECEIPTS"    // Kaupade või teenuste müügist laekunud raha
	CFOperPayments   = "CF_OPER_PAYMENTS"    // Kaupade, materjalide ja teenuste eest makstud raha
	CFOperWages      = "CF_OPER_WAGES"       // Makstud palgad
	CFOperTaxes      = "CF_OPER_TAXES"       // Makstud tulumaks
	CFOperInterestPd = "CF_OPER_INTEREST_PD" // Makstud intressid
	CFOperOther      = "CF_OPER_OTHER"       // Muud rahavood äritegevusest
	CFOperTotal      = "CF_OPER_TOTAL"       // Rahavood äritegevusest kokku

	// Operating Activities - indirect method
	CFOperNetIncome    = "CF_OPER_NET_INCOME"   // Aruandeperioodi kasum või kahjum
	CFOperDepreciation = "CF_OPER_DEPRECIATION" // Kulum ja amortisatsioon
	CFOperReceivables  = "CF_OPER_RECEIVABLES"  // Nõuete muutus
	CFOperInventory    = "CF_OPER_INVENTORY"    // Varude muutus
	CFOperPayables     = "CF_OPER_PAYABLES"     // Võlgnevuste muutus

	// Investing Activities - Rahavood investeerimistegevusest
	CFInvFixedAssets  = "CF_INV_FIXED_ASSETS" // Materiaalse põhivara ost/müük
	CFInvProperty     = "CF_INV_PROPERTY"     // Kinnisvarainvesteeringud
	CFInvSubsidiaries = "CF_INV_SUBSIDIARIES" // Tütar- ja sidusettevõtted
	CFInvLoansGiven   = "CF_INV_LOANS_GIVEN"  // Teistele osapooltele antud laenud
	CFInvLoansRcvd    = "CF_INV_LOANS_RCVD"   // Antud laenude laekumised
	CFInvDividends    = "CF_INV_DIVIDENDS"    // Saadud intressid ja dividendid
	CFInvTotal        = "CF_INV_TOTAL"        // Rahavood investeerimistegevusest kokku

	// Financing Activities - Rahavood finantseerimistegevusest
	CFFinLoansRcvd   = "CF_FIN_LOANS_RCVD"   // Laenude saamine
	CFFinLoansRepaid = "CF_FIN_LOANS_REPAID" // Saadud laenude tagasimaksmine
	CFFinLease       = "CF_FIN_LEASE"        // Kapitalirendi maksed
	CFFinShares      = "CF_FIN_SHARES"       // Aktsiate emiteerimine
	CFFinDividendsPd = "CF_FIN_DIVIDENDS_PD" // Dividendide maksmine
	CFFinTotal       = "CF_FIN_TOTAL"        // Rahavood finantseerimistegevusest kokku
)

// BalanceConfirmationType represents the type of balance confirmation
type BalanceConfirmationType string

const (
	BalanceTypeReceivable BalanceConfirmationType = "RECEIVABLE" // Customer balance
	BalanceTypePayable    BalanceConfirmationType = "PAYABLE"    // Supplier balance
)

// BalanceConfirmation represents a balance confirmation statement for a contact
type BalanceConfirmation struct {
	ID           string                  `json:"id"`
	TenantID     string                  `json:"tenant_id"`
	ContactID    string                  `json:"contact_id"`
	ContactName  string                  `json:"contact_name"`
	ContactCode  string                  `json:"contact_code,omitempty"`
	ContactEmail string                  `json:"contact_email,omitempty"`
	Type         BalanceConfirmationType `json:"type"`
	AsOfDate     string                  `json:"as_of_date"`
	TotalBalance decimal.Decimal         `json:"total_balance"`
	Invoices     []BalanceInvoice        `json:"invoices"`
	GeneratedAt  time.Time               `json:"generated_at"`
}

// BalanceInvoice represents an invoice in a balance confirmation
type BalanceInvoice struct {
	InvoiceID         string          `json:"invoice_id"`
	InvoiceNumber     string          `json:"invoice_number"`
	InvoiceDate       string          `json:"invoice_date"`
	DueDate           string          `json:"due_date"`
	TotalAmount       decimal.Decimal `json:"total_amount"`
	AmountPaid        decimal.Decimal `json:"amount_paid"`
	OutstandingAmount decimal.Decimal `json:"outstanding_amount"`
	Currency          string          `json:"currency"`
	DaysOverdue       int             `json:"days_overdue"`
}

// BalanceConfirmationRequest represents a request to generate balance confirmations
type BalanceConfirmationRequest struct {
	ContactID string `json:"contact_id,omitempty"` // Optional: specific contact
	Type      string `json:"type"`                 // "RECEIVABLE" or "PAYABLE"
	AsOfDate  string `json:"as_of_date"`           // Date for balance calculation
}

// ContactStatementRequest represents a request to generate a contact activity statement.
type ContactStatementRequest struct {
	ContactID string `json:"contact_id"`
	Type      string `json:"type"`       // "RECEIVABLE" or "PAYABLE"
	StartDate string `json:"start_date"` // Start date for statement activity
	EndDate   string `json:"end_date"`   // End date for statement activity
}

// BalanceConfirmationSummary represents a summary of all balances for a type
type BalanceConfirmationSummary struct {
	Type         BalanceConfirmationType `json:"type"`
	AsOfDate     string                  `json:"as_of_date"`
	TotalBalance decimal.Decimal         `json:"total_balance"`
	ContactCount int                     `json:"contact_count"`
	InvoiceCount int                     `json:"invoice_count"`
	Contacts     []ContactBalance        `json:"contacts"`
	GeneratedAt  time.Time               `json:"generated_at"`
}

// ContactBalance represents a contact's balance in the summary
type ContactBalance struct {
	ContactID     string          `json:"contact_id"`
	ContactName   string          `json:"contact_name"`
	ContactCode   string          `json:"contact_code,omitempty"`
	ContactEmail  string          `json:"contact_email,omitempty"`
	Balance       decimal.Decimal `json:"balance"`
	InvoiceCount  int             `json:"invoice_count"`
	OldestInvoice string          `json:"oldest_invoice,omitempty"`
}

// ContactStatement represents a customer or supplier statement for a period.
type ContactStatement struct {
	ID             string                  `json:"id"`
	TenantID       string                  `json:"tenant_id"`
	ContactID      string                  `json:"contact_id"`
	ContactName    string                  `json:"contact_name"`
	ContactCode    string                  `json:"contact_code,omitempty"`
	ContactEmail   string                  `json:"contact_email,omitempty"`
	Type           BalanceConfirmationType `json:"type"`
	StartDate      string                  `json:"start_date"`
	EndDate        string                  `json:"end_date"`
	OpeningBalance decimal.Decimal         `json:"opening_balance"`
	ClosingBalance decimal.Decimal         `json:"closing_balance"`
	TotalInvoiced  decimal.Decimal         `json:"total_invoiced"`
	TotalPaid      decimal.Decimal         `json:"total_paid"`
	Entries        []ContactStatementEntry `json:"entries"`
	GeneratedAt    time.Time               `json:"generated_at"`
}

// ContactStatementEntry represents one invoice or payment row on a contact statement.
type ContactStatementEntry struct {
	Date            string          `json:"date"`
	DocumentType    string          `json:"document_type"`
	DocumentID      string          `json:"document_id"`
	DocumentNumber  string          `json:"document_number"`
	DueDate         string          `json:"due_date,omitempty"`
	Description     string          `json:"description,omitempty"`
	Reference       string          `json:"reference,omitempty"`
	Currency        string          `json:"currency"`
	DocumentAmount  decimal.Decimal `json:"document_amount"`
	StatementAmount decimal.Decimal `json:"statement_amount"`
	IncreaseAmount  decimal.Decimal `json:"increase_amount"`
	DecreaseAmount  decimal.Decimal `json:"decrease_amount"`
	Balance         decimal.Decimal `json:"balance"`
}

// SalesMarginRequest represents a request to generate sales margin reporting.
type SalesMarginRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// SalesMarginReport represents revenue, cost, and margin for sales invoice lines.
type SalesMarginReport struct {
	TenantID      string               `json:"tenant_id"`
	StartDate     string               `json:"start_date"`
	EndDate       string               `json:"end_date"`
	TotalRevenue  decimal.Decimal      `json:"total_revenue"`
	TotalCost     decimal.Decimal      `json:"total_cost"`
	TotalMargin   decimal.Decimal      `json:"total_margin"`
	MarginPercent decimal.Decimal      `json:"margin_percent"`
	LineCount     int                  `json:"line_count"`
	ByContact     []SalesMarginContact `json:"by_contact"`
	Lines         []SalesMarginLine    `json:"lines"`
	GeneratedAt   time.Time            `json:"generated_at"`
}

// SalesMarginContact represents margin totals grouped by customer.
type SalesMarginContact struct {
	ContactID     string          `json:"contact_id"`
	ContactName   string          `json:"contact_name"`
	Revenue       decimal.Decimal `json:"revenue"`
	Cost          decimal.Decimal `json:"cost"`
	Margin        decimal.Decimal `json:"margin"`
	MarginPercent decimal.Decimal `json:"margin_percent"`
	LineCount     int             `json:"line_count"`
}

// SalesMarginLine represents one sales invoice line's estimated margin.
type SalesMarginLine struct {
	InvoiceID     string          `json:"invoice_id"`
	InvoiceNumber string          `json:"invoice_number"`
	InvoiceDate   string          `json:"invoice_date"`
	ContactID     string          `json:"contact_id"`
	ContactName   string          `json:"contact_name"`
	ProductID     string          `json:"product_id,omitempty"`
	ProductCode   string          `json:"product_code,omitempty"`
	ProductName   string          `json:"product_name,omitempty"`
	Description   string          `json:"description"`
	Quantity      decimal.Decimal `json:"quantity"`
	Revenue       decimal.Decimal `json:"revenue"`
	UnitCost      decimal.Decimal `json:"unit_cost"`
	Cost          decimal.Decimal `json:"cost"`
	Margin        decimal.Decimal `json:"margin"`
	MarginPercent decimal.Decimal `json:"margin_percent"`
}
