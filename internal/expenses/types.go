package expenses

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

type ExpenseStatus string

const (
	StatusDraft     ExpenseStatus = "DRAFT"
	StatusSubmitted ExpenseStatus = "SUBMITTED"
	StatusApproved  ExpenseStatus = "APPROVED"
	StatusRejected  ExpenseStatus = "REJECTED"
	StatusPosted    ExpenseStatus = "POSTED"

	SourceTypeExpense = "EXPENSE"
)

var (
	ErrExpenseNotFound          = errors.New("expense not found")
	ErrInvalidStatusTransition  = errors.New("invalid expense status transition")
	ErrApprovedReceiptRequired  = errors.New("approved expense receipt is required")
	ErrExpenseAlreadyPosted     = errors.New("expense is already posted")
	ErrExpenseAccountingInvalid = errors.New("expense accounting configuration is invalid")
)

type Expense struct {
	ID                 string                     `json:"id"`
	TenantID           string                     `json:"tenant_id"`
	ExpenseNumber      string                     `json:"expense_number"`
	ExpenseDate        time.Time                  `json:"expense_date"`
	Merchant           string                     `json:"merchant"`
	Description        string                     `json:"description,omitempty"`
	EmployeeID         *string                    `json:"employee_id,omitempty"`
	ContactID          *string                    `json:"contact_id,omitempty"`
	ExpenseAccountID   string                     `json:"expense_account_id"`
	PaymentAccountID   string                     `json:"payment_account_id"`
	Amount             decimal.Decimal            `json:"amount"`
	Currency           string                     `json:"currency"`
	ExchangeRate       decimal.Decimal            `json:"exchange_rate"`
	BaseAmount         decimal.Decimal            `json:"base_amount"`
	RequiresReceipt    bool                       `json:"requires_receipt"`
	Status             ExpenseStatus              `json:"status"`
	JournalEntryID     *string                    `json:"journal_entry_id,omitempty"`
	RemediationActions []ExpenseRemediationAction `json:"remediation_actions,omitempty"`
	SubmittedAt        *time.Time                 `json:"submitted_at,omitempty"`
	SubmittedBy        *string                    `json:"submitted_by,omitempty"`
	ApprovedAt         *time.Time                 `json:"approved_at,omitempty"`
	ApprovedBy         *string                    `json:"approved_by,omitempty"`
	RejectedAt         *time.Time                 `json:"rejected_at,omitempty"`
	RejectedBy         *string                    `json:"rejected_by,omitempty"`
	RejectionReason    string                     `json:"rejection_reason,omitempty"`
	PostedAt           *time.Time                 `json:"posted_at,omitempty"`
	PostedBy           *string                    `json:"posted_by,omitempty"`
	CreatedAt          time.Time                  `json:"created_at"`
	CreatedBy          string                     `json:"created_by"`
	UpdatedAt          time.Time                  `json:"updated_at"`
}

// ExpenseRemediationAction describes one operator action for expense claim follow-up.
type ExpenseRemediationAction struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Scope          string `json:"scope"`
	OwnerRole      string `json:"owner_role"`
	WorkspaceQueue string `json:"workspace_queue,omitempty"`
	AssignmentKey  string `json:"assignment_key,omitempty"`
	Priority       string `json:"priority,omitempty"`
	DueInDays      int    `json:"due_in_days,omitempty"`
	Message        string `json:"message"`
	Action         string `json:"action"`
	EntityType     string `json:"entity_type,omitempty"`
	EntityID       string `json:"entity_id,omitempty"`
	ExpenseNumber  string `json:"expense_number,omitempty"`
	Status         string `json:"status,omitempty"`
	UIPath         string `json:"ui_path,omitempty"`
	CLICommand     string `json:"cli_command,omitempty"`
}

type CreateExpenseRequest struct {
	ExpenseDate      time.Time       `json:"expense_date"`
	Merchant         string          `json:"merchant"`
	Description      string          `json:"description,omitempty"`
	EmployeeID       *string         `json:"employee_id,omitempty"`
	ContactID        *string         `json:"contact_id,omitempty"`
	ExpenseAccountID string          `json:"expense_account_id"`
	PaymentAccountID string          `json:"payment_account_id"`
	Amount           decimal.Decimal `json:"amount"`
	Currency         string          `json:"currency,omitempty"`
	ExchangeRate     decimal.Decimal `json:"exchange_rate,omitempty"`
	RequiresReceipt  *bool           `json:"requires_receipt,omitempty"`
	UserID           string          `json:"-"`
}

type ImportExpensesRequest struct {
	CSVContent string     `json:"csv_content"`
	FileName   string     `json:"file_name,omitempty"`
	UserID     string     `json:"-"`
	LockDate   *time.Time `json:"-"`
}

type ImportExpensesResult struct {
	FileName        string                   `json:"file_name,omitempty"`
	RowsProcessed   int                      `json:"rows_processed"`
	ExpensesCreated int                      `json:"expenses_created"`
	RowsSkipped     int                      `json:"rows_skipped"`
	Errors          []ImportExpensesRowError `json:"errors,omitempty"`
}

type ImportExpensesRowError struct {
	Row           int    `json:"row"`
	ExpenseNumber string `json:"expense_number,omitempty"`
	Merchant      string `json:"merchant,omitempty"`
	Message       string `json:"message"`
}

type ListExpensesFilter struct {
	Status ExpenseStatus `json:"status,omitempty"`
	Limit  int           `json:"limit,omitempty"`
}

type ExpenseActionRequest struct {
	UserID string `json:"-"`
}

type RejectExpenseRequest struct {
	Reason string `json:"reason"`
	UserID string `json:"-"`
}
