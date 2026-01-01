package invoicing

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

// InvoiceType represents the type of invoice
type InvoiceType string

const (
	InvoiceTypeSales      InvoiceType = "SALES"
	InvoiceTypePurchase   InvoiceType = "PURCHASE"
	InvoiceTypeCreditNote InvoiceType = "CREDIT_NOTE"
)

// InvoiceStatus represents the status of an invoice
type InvoiceStatus string

const (
	StatusDraft         InvoiceStatus = "DRAFT"
	StatusSent          InvoiceStatus = "SENT"
	StatusPartiallyPaid InvoiceStatus = "PARTIALLY_PAID"
	StatusPaid          InvoiceStatus = "PAID"
	StatusOverdue       InvoiceStatus = "OVERDUE"
	StatusVoided        InvoiceStatus = "VOIDED"
)

// Invoice represents a sales or purchase invoice
type Invoice struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenant_id"`
	InvoiceNumber  string            `json:"invoice_number"`
	InvoiceType    InvoiceType       `json:"invoice_type"`
	ContactID      string            `json:"contact_id"`
	Contact        *contacts.Contact `json:"contact,omitempty"`
	IssueDate      time.Time         `json:"issue_date"`
	DueDate        time.Time         `json:"due_date"`
	Currency       string            `json:"currency"`
	ExchangeRate   decimal.Decimal   `json:"exchange_rate"`
	Subtotal       decimal.Decimal   `json:"subtotal"`
	VATAmount      decimal.Decimal   `json:"vat_amount"`
	Total          decimal.Decimal   `json:"total"`
	BaseSubtotal   decimal.Decimal   `json:"base_subtotal"`
	BaseVATAmount  decimal.Decimal   `json:"base_vat_amount"`
	BaseTotal      decimal.Decimal   `json:"base_total"`
	AmountPaid     decimal.Decimal   `json:"amount_paid"`
	Status         InvoiceStatus     `json:"status"`
	Reference      string            `json:"reference,omitempty"`
	Notes          string            `json:"notes,omitempty"`
	Lines          []InvoiceLine     `json:"lines"`
	JournalEntryID *string           `json:"journal_entry_id,omitempty"`
	EInvoiceSentAt *time.Time        `json:"einvoice_sent_at,omitempty"`
	EInvoiceID     *string           `json:"einvoice_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	CreatedBy      string            `json:"created_by"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// InvoiceLine represents a line item on an invoice
type InvoiceLine struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id"`
	InvoiceID       string          `json:"invoice_id"`
	LineNumber      int             `json:"line_number"`
	Description     string          `json:"description"`
	Quantity        decimal.Decimal `json:"quantity"`
	Unit            string          `json:"unit,omitempty"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	DiscountPercent decimal.Decimal `json:"discount_percent"`
	VATRate         decimal.Decimal `json:"vat_rate"`
	LineSubtotal    decimal.Decimal `json:"line_subtotal"`
	LineVAT         decimal.Decimal `json:"line_vat"`
	LineTotal       decimal.Decimal `json:"line_total"`
	AccountID       *string         `json:"account_id,omitempty"`
	ProductID       *string         `json:"product_id,omitempty"`
}

// Calculate computes the line totals
func (l *InvoiceLine) Calculate() {
	// Subtotal = quantity * unit_price * (1 - discount/100)
	grossAmount := l.Quantity.Mul(l.UnitPrice)
	discountAmount := grossAmount.Mul(l.DiscountPercent).Div(decimal.NewFromInt(100))
	l.LineSubtotal = grossAmount.Sub(discountAmount).Round(2)

	// VAT = subtotal * vat_rate/100
	l.LineVAT = l.LineSubtotal.Mul(l.VATRate).Div(decimal.NewFromInt(100)).Round(2)

	// Total = subtotal + VAT
	l.LineTotal = l.LineSubtotal.Add(l.LineVAT)
}

// Calculate computes the invoice totals from lines
func (inv *Invoice) Calculate() {
	inv.Subtotal = decimal.Zero
	inv.VATAmount = decimal.Zero
	inv.Total = decimal.Zero

	for i := range inv.Lines {
		inv.Lines[i].Calculate()
		inv.Subtotal = inv.Subtotal.Add(inv.Lines[i].LineSubtotal)
		inv.VATAmount = inv.VATAmount.Add(inv.Lines[i].LineVAT)
		inv.Total = inv.Total.Add(inv.Lines[i].LineTotal)
	}

	// Calculate base currency amounts
	inv.BaseSubtotal = inv.Subtotal.Mul(inv.ExchangeRate).Round(2)
	inv.BaseVATAmount = inv.VATAmount.Mul(inv.ExchangeRate).Round(2)
	inv.BaseTotal = inv.Total.Mul(inv.ExchangeRate).Round(2)
}

// Validate validates the invoice
func (inv *Invoice) Validate() error {
	if len(inv.Lines) == 0 {
		return errors.New("invoice must have at least one line")
	}

	if inv.ContactID == "" {
		return errors.New("contact is required")
	}

	if inv.IssueDate.IsZero() {
		return errors.New("issue date is required")
	}

	if inv.DueDate.Before(inv.IssueDate) {
		return errors.New("due date cannot be before issue date")
	}

	for i, line := range inv.Lines {
		if line.Description == "" {
			return errors.New("line description is required")
		}
		if line.Quantity.LessThanOrEqual(decimal.Zero) {
			return errors.New("line quantity must be positive")
		}
		if line.UnitPrice.LessThan(decimal.Zero) {
			return errors.New("line unit price cannot be negative")
		}
		if line.VATRate.LessThan(decimal.Zero) {
			return errors.New("line VAT rate cannot be negative")
		}
		if line.DiscountPercent.LessThan(decimal.Zero) || line.DiscountPercent.GreaterThan(decimal.NewFromInt(100)) {
			return errors.New("line discount must be between 0 and 100")
		}
		// Ensure line numbers are sequential
		if line.LineNumber != i+1 {
			inv.Lines[i].LineNumber = i + 1
		}
	}

	return nil
}

// AmountDue returns the amount still owed
func (inv *Invoice) AmountDue() decimal.Decimal {
	return inv.Total.Sub(inv.AmountPaid)
}

// IsPaid returns true if the invoice is fully paid
func (inv *Invoice) IsPaid() bool {
	return inv.AmountPaid.GreaterThanOrEqual(inv.Total)
}

// IsOverdue returns true if the invoice is past due and not fully paid
func (inv *Invoice) IsOverdue() bool {
	return !inv.IsPaid() && time.Now().After(inv.DueDate) && inv.Status != StatusVoided
}

// CreateInvoiceRequest is the request to create an invoice
type CreateInvoiceRequest struct {
	InvoiceType  InvoiceType                `json:"invoice_type"`
	ContactID    string                     `json:"contact_id"`
	IssueDate    time.Time                  `json:"issue_date"`
	DueDate      time.Time                  `json:"due_date"`
	Currency     string                     `json:"currency,omitempty"`
	ExchangeRate decimal.Decimal            `json:"exchange_rate,omitempty"`
	Reference    string                     `json:"reference,omitempty"`
	Notes        string                     `json:"notes,omitempty"`
	Lines        []CreateInvoiceLineRequest `json:"lines"`
	UserID       string                     `json:"-"`
}

// CreateInvoiceLineRequest is a line in the create invoice request
type CreateInvoiceLineRequest struct {
	Description     string          `json:"description"`
	Quantity        decimal.Decimal `json:"quantity"`
	Unit            string          `json:"unit,omitempty"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	DiscountPercent decimal.Decimal `json:"discount_percent,omitempty"`
	VATRate         decimal.Decimal `json:"vat_rate"`
	AccountID       *string         `json:"account_id,omitempty"`
	ProductID       *string         `json:"product_id,omitempty"`
}

// InvoiceFilter provides filtering options
type InvoiceFilter struct {
	InvoiceType InvoiceType
	Status      InvoiceStatus
	ContactID   string
	FromDate    *time.Time
	ToDate      *time.Time
	Search      string
}
