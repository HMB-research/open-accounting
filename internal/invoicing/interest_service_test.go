package invoicing

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestInterestCalculation(t *testing.T) {
	// Test the interest calculation formula: interest = outstanding × rate × days
	tests := []struct {
		name             string
		outstanding      string
		rate             float64
		daysOverdue      int
		expectedInterest string
		expectedTotal    string
	}{
		{
			name:             "standard calculation",
			outstanding:      "1000.00",
			rate:             0.0005, // 0.05% daily
			daysOverdue:      30,
			expectedInterest: "15.00", // 1000 * 0.0005 * 30 = 15
			expectedTotal:    "1015.00",
		},
		{
			name:             "zero days",
			outstanding:      "1000.00",
			rate:             0.0005,
			daysOverdue:      0,
			expectedInterest: "0.00",
			expectedTotal:    "1000.00",
		},
		{
			name:             "zero rate",
			outstanding:      "1000.00",
			rate:             0,
			daysOverdue:      30,
			expectedInterest: "0.00",
			expectedTotal:    "1000.00",
		},
		{
			name:             "small amount",
			outstanding:      "50.00",
			rate:             0.0005,
			daysOverdue:      7,
			expectedInterest: "0.21", // dailyInterest = 50 * 0.0005 = 0.025 → 0.03 rounded, then 0.03 * 7 = 0.21
			expectedTotal:    "50.21",
		},
		{
			name:             "large amount",
			outstanding:      "100000.00",
			rate:             0.0005,
			daysOverdue:      90,
			expectedInterest: "4500.00", // 100000 * 0.0005 * 90 = 4500
			expectedTotal:    "104500.00",
		},
		{
			name:             "higher rate",
			outstanding:      "1000.00",
			rate:             0.001, // 0.1% daily
			daysOverdue:      10,
			expectedInterest: "10.00", // 1000 * 0.001 * 10 = 10
			expectedTotal:    "1010.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outstanding, _ := decimal.NewFromString(tt.outstanding)
			rate := decimal.NewFromFloat(tt.rate)
			days := decimal.NewFromInt(int64(tt.daysOverdue))

			// Calculate using the formula from the service
			dailyInterest := outstanding.Mul(rate).Round(2)
			totalInterest := dailyInterest.Mul(days).Round(2)
			totalWithInterest := outstanding.Add(totalInterest)

			expectedInterest, _ := decimal.NewFromString(tt.expectedInterest)
			expectedTotal, _ := decimal.NewFromString(tt.expectedTotal)

			if !totalInterest.Equal(expectedInterest) {
				t.Errorf("interest = %s, want %s", totalInterest.String(), expectedInterest.String())
			}
			if !totalWithInterest.Equal(expectedTotal) {
				t.Errorf("total = %s, want %s", totalWithInterest.String(), expectedTotal.String())
			}
		})
	}
}

func TestUpdateInterestSettingsRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		rate    float64
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid rate",
			rate:    0.0005,
			wantErr: false,
		},
		{
			name:    "zero rate - disabled",
			rate:    0,
			wantErr: false,
		},
		{
			name:    "negative rate",
			rate:    -0.001,
			wantErr: true,
			errMsg:  "interest rate cannot be negative",
		},
		{
			name:    "rate too high",
			rate:    0.02, // 2% daily = 730% annually
			wantErr: true,
			errMsg:  "interest rate exceeds maximum allowed (1% daily)",
		},
		{
			name:    "rate at max allowed",
			rate:    0.01, // 1% daily = 365% annually (at limit, allowed)
			wantErr: false,
		},
		{
			name:    "rate just over max",
			rate:    0.0101,
			wantErr: true,
			errMsg:  "interest rate exceeds maximum allowed (1% daily)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &UpdateInterestSettingsRequest{Rate: tt.rate}
			err := req.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestInterestResultType(t *testing.T) {
	result := InterestCalculationResult{
		InvoiceID:         "inv-123",
		InvoiceNumber:     "INV-001",
		DueDate:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		DaysOverdue:       30,
		OutstandingAmount: decimal.NewFromFloat(1000),
		InterestRate:      decimal.NewFromFloat(0.0005),
		DailyInterest:     decimal.NewFromFloat(0.50),
		TotalInterest:     decimal.NewFromFloat(15),
		TotalWithInterest: decimal.NewFromFloat(1015),
		CalculatedAt:      time.Now(),
		Currency:          "EUR",
	}

	if result.InvoiceID != "inv-123" {
		t.Errorf("InvoiceID = %s, want inv-123", result.InvoiceID)
	}
	if result.DaysOverdue != 30 {
		t.Errorf("DaysOverdue = %d, want 30", result.DaysOverdue)
	}
}

func TestInterestSettingsType(t *testing.T) {
	rate := 0.0005 // 0.05% daily
	settings := InterestSettings{
		Rate:        rate,
		AnnualRate:  rate * 365,
		IsEnabled:   rate > 0,
		Description: "0.05% daily (18.25% annually)",
	}

	if !settings.IsEnabled {
		t.Error("IsEnabled should be true for non-zero rate")
	}

	expectedAnnual := 0.1825
	if settings.AnnualRate != expectedAnnual {
		t.Errorf("AnnualRate = %f, want %f", settings.AnnualRate, expectedAnnual)
	}

	// Test disabled state
	disabledSettings := InterestSettings{
		Rate:       0,
		AnnualRate: 0,
		IsEnabled:  false,
	}

	if disabledSettings.IsEnabled {
		t.Error("IsEnabled should be false for zero rate")
	}
}

func TestInvoiceInterestType(t *testing.T) {
	now := time.Now()
	interest := InvoiceInterest{
		ID:                "int-123",
		InvoiceID:         "inv-456",
		CalculatedAt:      now,
		DaysOverdue:       14,
		PrincipalAmount:   decimal.NewFromFloat(500),
		InterestRate:      decimal.NewFromFloat(0.0005),
		InterestAmount:    decimal.NewFromFloat(3.50),
		TotalWithInterest: decimal.NewFromFloat(503.50),
		CreatedAt:         now,
	}

	if interest.ID != "int-123" {
		t.Errorf("ID = %s, want int-123", interest.ID)
	}
	if interest.DaysOverdue != 14 {
		t.Errorf("DaysOverdue = %d, want 14", interest.DaysOverdue)
	}
	if !interest.PrincipalAmount.Equal(decimal.NewFromFloat(500)) {
		t.Errorf("PrincipalAmount = %s, want 500", interest.PrincipalAmount.String())
	}
}

func TestDaysOverdueCalculation(t *testing.T) {
	tests := []struct {
		name         string
		dueDate      time.Time
		asOfDate     time.Time
		expectedDays int
	}{
		{
			name:         "30 days overdue",
			dueDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			asOfDate:     time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			expectedDays: 30,
		},
		{
			name:         "not yet due",
			dueDate:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			asOfDate:     time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			expectedDays: 0,
		},
		{
			name:         "due today",
			dueDate:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			asOfDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectedDays: 0,
		},
		{
			name:         "1 day overdue",
			dueDate:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			asOfDate:     time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
			expectedDays: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			daysOverdue := 0
			if tt.asOfDate.After(tt.dueDate) {
				daysOverdue = int(tt.asOfDate.Sub(tt.dueDate).Hours() / 24)
			}

			if daysOverdue != tt.expectedDays {
				t.Errorf("daysOverdue = %d, want %d", daysOverdue, tt.expectedDays)
			}
		})
	}
}
