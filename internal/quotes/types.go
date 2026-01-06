package quotes

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

// QuoteStatus represents the status of a quote
type QuoteStatus string

const (
	QuoteStatusDraft     QuoteStatus = "DRAFT"
	QuoteStatusSent      QuoteStatus = "SENT"
	QuoteStatusAccepted  QuoteStatus = "ACCEPTED"
	QuoteStatusRejected  QuoteStatus = "REJECTED"
	QuoteStatusExpired   QuoteStatus = "EXPIRED"
	QuoteStatusConverted QuoteStatus = "CONVERTED"
)

// Quote represents a sales quote/offer
type Quote struct {
	ID                   string            `json:"id"`
	TenantID             string            `json:"tenant_id"`
	QuoteNumber          string            `json:"quote_number"`
	ContactID            string            `json:"contact_id"`
	Contact              *contacts.Contact `json:"contact,omitempty"`
	QuoteDate            time.Time         `json:"quote_date"`
	ValidUntil           *time.Time        `json:"valid_until,omitempty"`
	Status               QuoteStatus       `json:"status"`
	Currency             string            `json:"currency"`
	ExchangeRate         decimal.Decimal   `json:"exchange_rate"`
	Subtotal             decimal.Decimal   `json:"subtotal"`
	VATAmount            decimal.Decimal   `json:"vat_amount"`
	Total                decimal.Decimal   `json:"total"`
	Notes                string            `json:"notes,omitempty"`
	ConvertedToOrderID   *string           `json:"converted_to_order_id,omitempty"`
	ConvertedToInvoiceID *string           `json:"converted_to_invoice_id,omitempty"`
	Lines                []QuoteLine       `json:"lines"`
	CreatedAt            time.Time         `json:"created_at"`
	CreatedBy            string            `json:"created_by"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// QuoteLine represents a line item on a quote
type QuoteLine struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id"`
	QuoteID         string          `json:"quote_id"`
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
	ProductID       *string         `json:"product_id,omitempty"`
}

// Calculate computes the line totals
func (l *QuoteLine) Calculate() {
	// Subtotal = quantity * unit_price * (1 - discount/100)
	grossAmount := l.Quantity.Mul(l.UnitPrice)
	discountAmount := grossAmount.Mul(l.DiscountPercent).Div(decimal.NewFromInt(100))
	l.LineSubtotal = grossAmount.Sub(discountAmount).Round(2)

	// VAT = subtotal * vat_rate/100
	l.LineVAT = l.LineSubtotal.Mul(l.VATRate).Div(decimal.NewFromInt(100)).Round(2)

	// Total = subtotal + VAT
	l.LineTotal = l.LineSubtotal.Add(l.LineVAT)
}

// Calculate computes the quote totals from lines
func (q *Quote) Calculate() {
	q.Subtotal = decimal.Zero
	q.VATAmount = decimal.Zero
	q.Total = decimal.Zero

	for i := range q.Lines {
		q.Lines[i].Calculate()
		q.Subtotal = q.Subtotal.Add(q.Lines[i].LineSubtotal)
		q.VATAmount = q.VATAmount.Add(q.Lines[i].LineVAT)
		q.Total = q.Total.Add(q.Lines[i].LineTotal)
	}
}

// Validate validates the quote
func (q *Quote) Validate() error {
	if len(q.Lines) == 0 {
		return errors.New("quote must have at least one line")
	}

	if q.ContactID == "" {
		return errors.New("contact is required")
	}

	if q.QuoteDate.IsZero() {
		return errors.New("quote date is required")
	}

	if q.ValidUntil != nil && q.ValidUntil.Before(q.QuoteDate) {
		return errors.New("valid until date cannot be before quote date")
	}

	for i, line := range q.Lines {
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
			q.Lines[i].LineNumber = i + 1
		}
	}

	return nil
}

// IsExpired returns true if the quote is past its valid date
func (q *Quote) IsExpired() bool {
	if q.ValidUntil == nil {
		return false
	}
	return time.Now().After(*q.ValidUntil) && q.Status != QuoteStatusConverted && q.Status != QuoteStatusAccepted
}

// CreateQuoteRequest is the request to create a quote
type CreateQuoteRequest struct {
	ContactID    string                   `json:"contact_id"`
	QuoteDate    time.Time                `json:"quote_date"`
	ValidUntil   *time.Time               `json:"valid_until,omitempty"`
	Currency     string                   `json:"currency,omitempty"`
	ExchangeRate decimal.Decimal          `json:"exchange_rate,omitempty"`
	Notes        string                   `json:"notes,omitempty"`
	Lines        []CreateQuoteLineRequest `json:"lines"`
	UserID       string                   `json:"-"`
}

// CreateQuoteLineRequest is a line in the create quote request
type CreateQuoteLineRequest struct {
	Description     string          `json:"description"`
	Quantity        decimal.Decimal `json:"quantity"`
	Unit            string          `json:"unit,omitempty"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	DiscountPercent decimal.Decimal `json:"discount_percent,omitempty"`
	VATRate         decimal.Decimal `json:"vat_rate"`
	ProductID       *string         `json:"product_id,omitempty"`
}

// UpdateQuoteRequest is the request to update a quote
type UpdateQuoteRequest struct {
	ContactID    string                   `json:"contact_id"`
	QuoteDate    time.Time                `json:"quote_date"`
	ValidUntil   *time.Time               `json:"valid_until,omitempty"`
	Currency     string                   `json:"currency,omitempty"`
	ExchangeRate decimal.Decimal          `json:"exchange_rate,omitempty"`
	Notes        string                   `json:"notes,omitempty"`
	Lines        []CreateQuoteLineRequest `json:"lines"`
}

// QuoteFilter provides filtering options
type QuoteFilter struct {
	Status    QuoteStatus
	ContactID string
	FromDate  *time.Time
	ToDate    *time.Time
	Search    string
}
