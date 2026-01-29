package invoicing

import (
	"time"

	"github.com/shopspring/decimal"
)

// InvoiceInterest represents a calculated interest record for an invoice
type InvoiceInterest struct {
	ID                string          `json:"id"`
	InvoiceID         string          `json:"invoice_id"`
	CalculatedAt      time.Time       `json:"calculated_at"`
	DaysOverdue       int             `json:"days_overdue"`
	PrincipalAmount   decimal.Decimal `json:"principal_amount"`
	InterestRate      decimal.Decimal `json:"interest_rate"`
	InterestAmount    decimal.Decimal `json:"interest_amount"`
	TotalWithInterest decimal.Decimal `json:"total_with_interest"`
	CreatedAt         time.Time       `json:"created_at"`
}

// InterestCalculationResult represents the result of an interest calculation
type InterestCalculationResult struct {
	InvoiceID         string          `json:"invoice_id"`
	InvoiceNumber     string          `json:"invoice_number"`
	DueDate           time.Time       `json:"due_date"`
	DaysOverdue       int             `json:"days_overdue"`
	OutstandingAmount decimal.Decimal `json:"outstanding_amount"`
	InterestRate      decimal.Decimal `json:"interest_rate"`
	DailyInterest     decimal.Decimal `json:"daily_interest"`
	TotalInterest     decimal.Decimal `json:"total_interest"`
	TotalWithInterest decimal.Decimal `json:"total_with_interest"`
	CalculatedAt      time.Time       `json:"calculated_at"`
	Currency          string          `json:"currency"`
}

// InterestSettings represents the interest configuration for a tenant
type InterestSettings struct {
	Rate        float64 `json:"rate"`        // Daily interest rate (e.g., 0.0005 = 0.05%)
	AnnualRate  float64 `json:"annual_rate"` // Annualized rate for display (rate * 365)
	Description string  `json:"description"` // Human-readable description
	IsEnabled   bool    `json:"is_enabled"`  // Whether interest calculation is enabled
}

// UpdateInterestSettingsRequest is the request to update interest settings
type UpdateInterestSettingsRequest struct {
	Rate float64 `json:"rate"` // Daily interest rate
}

// Validate validates the interest settings update request
func (r *UpdateInterestSettingsRequest) Validate() error {
	if r.Rate < 0 {
		return &ValidationError{Message: "interest rate cannot be negative"}
	}
	if r.Rate > 0.01 { // Max 1% daily = 365% annually (usury protection)
		return &ValidationError{Message: "interest rate exceeds maximum allowed (1% daily)"}
	}
	return nil
}
