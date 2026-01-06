package payments

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ErrPaymentNotFound is returned when a payment is not found
var ErrPaymentNotFound = errors.New("payment not found")

// PaymentType represents the type of payment
type PaymentType string

const (
	PaymentTypeReceived PaymentType = "RECEIVED"
	PaymentTypeMade     PaymentType = "MADE"
)

// Payment represents a payment received or made
type Payment struct {
	ID             string              `json:"id"`
	TenantID       string              `json:"tenant_id"`
	PaymentNumber  string              `json:"payment_number"`
	PaymentType    PaymentType         `json:"payment_type"`
	ContactID      *string             `json:"contact_id,omitempty"`
	PaymentDate    time.Time           `json:"payment_date"`
	Amount         decimal.Decimal     `json:"amount"`
	Currency       string              `json:"currency"`
	ExchangeRate   decimal.Decimal     `json:"exchange_rate"`
	BaseAmount     decimal.Decimal     `json:"base_amount"`
	PaymentMethod  string              `json:"payment_method,omitempty"`
	BankAccount    string              `json:"bank_account,omitempty"`
	Reference      string              `json:"reference,omitempty"`
	Notes          string              `json:"notes,omitempty"`
	Allocations    []PaymentAllocation `json:"allocations,omitempty"`
	JournalEntryID *string             `json:"journal_entry_id,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	CreatedBy      string              `json:"created_by"`
}

// PaymentAllocation represents how a payment is allocated to invoices
type PaymentAllocation struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	PaymentID string          `json:"payment_id"`
	InvoiceID string          `json:"invoice_id"`
	Amount    decimal.Decimal `json:"amount"`
	CreatedAt time.Time       `json:"created_at"`
}

// TotalAllocated returns the total amount allocated to invoices
func (p *Payment) TotalAllocated() decimal.Decimal {
	total := decimal.Zero
	for _, a := range p.Allocations {
		total = total.Add(a.Amount)
	}
	return total
}

// UnallocatedAmount returns the amount not yet allocated to invoices
func (p *Payment) UnallocatedAmount() decimal.Decimal {
	return p.Amount.Sub(p.TotalAllocated())
}

// CreatePaymentRequest is the request to create a payment
type CreatePaymentRequest struct {
	PaymentType   PaymentType         `json:"payment_type"`
	ContactID     *string             `json:"contact_id,omitempty"`
	PaymentDate   time.Time           `json:"payment_date"`
	Amount        decimal.Decimal     `json:"amount"`
	Currency      string              `json:"currency,omitempty"`
	ExchangeRate  decimal.Decimal     `json:"exchange_rate,omitempty"`
	PaymentMethod string              `json:"payment_method,omitempty"`
	BankAccount   string              `json:"bank_account,omitempty"`
	Reference     string              `json:"reference,omitempty"`
	Notes         string              `json:"notes,omitempty"`
	Allocations   []AllocationRequest `json:"allocations,omitempty"`
	UserID        string              `json:"-"`
}

// AllocationRequest represents a payment allocation in a request
type AllocationRequest struct {
	InvoiceID string          `json:"invoice_id"`
	Amount    decimal.Decimal `json:"amount"`
}

// PaymentFilter provides filtering options
type PaymentFilter struct {
	PaymentType   PaymentType
	PaymentMethod string
	ContactID     string
	FromDate      *time.Time
	ToDate        *time.Time
}
