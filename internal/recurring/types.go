package recurring

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Frequency represents how often a recurring invoice is generated
type Frequency string

const (
	FrequencyWeekly    Frequency = "WEEKLY"
	FrequencyBiweekly  Frequency = "BIWEEKLY"
	FrequencyMonthly   Frequency = "MONTHLY"
	FrequencyQuarterly Frequency = "QUARTERLY"
	FrequencyYearly    Frequency = "YEARLY"
)

// RecurringInvoice represents a recurring invoice template
type RecurringInvoice struct {
	ID                 string                 `json:"id"`
	TenantID           string                 `json:"tenant_id"`
	Name               string                 `json:"name"`
	ContactID          string                 `json:"contact_id"`
	ContactName        string                 `json:"contact_name,omitempty"`
	InvoiceType        string                 `json:"invoice_type"`
	Currency           string                 `json:"currency"`
	Frequency          Frequency              `json:"frequency"`
	StartDate          time.Time              `json:"start_date"`
	EndDate            *time.Time             `json:"end_date,omitempty"`
	NextGenerationDate time.Time              `json:"next_generation_date"`
	PaymentTermsDays   int                    `json:"payment_terms_days"`
	Reference          string                 `json:"reference,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	IsActive           bool                   `json:"is_active"`
	LastGeneratedAt    *time.Time             `json:"last_generated_at,omitempty"`
	GeneratedCount     int                    `json:"generated_count"`
	Lines              []RecurringInvoiceLine `json:"lines"`
	CreatedAt          time.Time              `json:"created_at"`
	CreatedBy          string                 `json:"created_by"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// RecurringInvoiceLine represents a line item on a recurring invoice
type RecurringInvoiceLine struct {
	ID                  string          `json:"id"`
	RecurringInvoiceID  string          `json:"recurring_invoice_id"`
	LineNumber          int             `json:"line_number"`
	Description         string          `json:"description"`
	Quantity            decimal.Decimal `json:"quantity"`
	Unit                string          `json:"unit,omitempty"`
	UnitPrice           decimal.Decimal `json:"unit_price"`
	DiscountPercent     decimal.Decimal `json:"discount_percent"`
	VATRate             decimal.Decimal `json:"vat_rate"`
	AccountID           *string         `json:"account_id,omitempty"`
	ProductID           *string         `json:"product_id,omitempty"`
}

// Validate validates the recurring invoice
func (r *RecurringInvoice) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.ContactID == "" {
		return errors.New("contact is required")
	}
	if r.StartDate.IsZero() {
		return errors.New("start date is required")
	}
	if r.EndDate != nil && r.EndDate.Before(r.StartDate) {
		return errors.New("end date cannot be before start date")
	}
	if !isValidFrequency(r.Frequency) {
		return errors.New("invalid frequency")
	}
	if r.PaymentTermsDays < 0 {
		return errors.New("payment terms days cannot be negative")
	}
	if len(r.Lines) == 0 {
		return errors.New("at least one line item is required")
	}
	for _, line := range r.Lines {
		if line.Description == "" {
			return errors.New("line description is required")
		}
		if line.Quantity.LessThanOrEqual(decimal.Zero) {
			return errors.New("line quantity must be positive")
		}
		if line.UnitPrice.LessThan(decimal.Zero) {
			return errors.New("line unit price cannot be negative")
		}
	}
	return nil
}

func isValidFrequency(f Frequency) bool {
	switch f {
	case FrequencyWeekly, FrequencyBiweekly, FrequencyMonthly, FrequencyQuarterly, FrequencyYearly:
		return true
	}
	return false
}

// CalculateNextDate calculates the next generation date based on frequency
func (r *RecurringInvoice) CalculateNextDate(from time.Time) time.Time {
	switch r.Frequency {
	case FrequencyWeekly:
		return from.AddDate(0, 0, 7)
	case FrequencyBiweekly:
		return from.AddDate(0, 0, 14)
	case FrequencyMonthly:
		return from.AddDate(0, 1, 0)
	case FrequencyQuarterly:
		return from.AddDate(0, 3, 0)
	case FrequencyYearly:
		return from.AddDate(1, 0, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}

// IsDue returns true if the recurring invoice is due for generation
func (r *RecurringInvoice) IsDue() bool {
	if !r.IsActive {
		return false
	}
	if r.EndDate != nil && time.Now().After(*r.EndDate) {
		return false
	}
	return !time.Now().Before(r.NextGenerationDate)
}

// CreateRecurringInvoiceRequest is the request to create a recurring invoice
type CreateRecurringInvoiceRequest struct {
	Name             string                            `json:"name"`
	ContactID        string                            `json:"contact_id"`
	InvoiceType      string                            `json:"invoice_type"`
	Currency         string                            `json:"currency,omitempty"`
	Frequency        Frequency                         `json:"frequency"`
	StartDate        time.Time                         `json:"start_date"`
	EndDate          *time.Time                        `json:"end_date,omitempty"`
	PaymentTermsDays int                               `json:"payment_terms_days"`
	Reference        string                            `json:"reference,omitempty"`
	Notes            string                            `json:"notes,omitempty"`
	Lines            []CreateRecurringInvoiceLineRequest `json:"lines"`
	UserID           string                            `json:"-"`
}

// CreateRecurringInvoiceLineRequest is a line in the create request
type CreateRecurringInvoiceLineRequest struct {
	Description     string          `json:"description"`
	Quantity        decimal.Decimal `json:"quantity"`
	Unit            string          `json:"unit,omitempty"`
	UnitPrice       decimal.Decimal `json:"unit_price"`
	DiscountPercent decimal.Decimal `json:"discount_percent,omitempty"`
	VATRate         decimal.Decimal `json:"vat_rate"`
	AccountID       *string         `json:"account_id,omitempty"`
	ProductID       *string         `json:"product_id,omitempty"`
}

// UpdateRecurringInvoiceRequest is the request to update a recurring invoice
type UpdateRecurringInvoiceRequest struct {
	Name             *string                           `json:"name,omitempty"`
	ContactID        *string                           `json:"contact_id,omitempty"`
	Frequency        *Frequency                        `json:"frequency,omitempty"`
	EndDate          *time.Time                        `json:"end_date,omitempty"`
	PaymentTermsDays *int                              `json:"payment_terms_days,omitempty"`
	Reference        *string                           `json:"reference,omitempty"`
	Notes            *string                           `json:"notes,omitempty"`
	Lines            []CreateRecurringInvoiceLineRequest `json:"lines,omitempty"`
}

// CreateFromInvoiceRequest creates a recurring invoice from an existing invoice
type CreateFromInvoiceRequest struct {
	InvoiceID        string     `json:"invoice_id"`
	Name             string     `json:"name"`
	Frequency        Frequency  `json:"frequency"`
	StartDate        time.Time  `json:"start_date"`
	EndDate          *time.Time `json:"end_date,omitempty"`
	PaymentTermsDays int        `json:"payment_terms_days"`
	UserID           string     `json:"-"`
}

// GenerationResult represents the result of generating invoices
type GenerationResult struct {
	RecurringInvoiceID   string `json:"recurring_invoice_id"`
	GeneratedInvoiceID   string `json:"generated_invoice_id"`
	GeneratedInvoiceNumber string `json:"generated_invoice_number"`
}
