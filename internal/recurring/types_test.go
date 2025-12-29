package recurring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestFrequencyConstants(t *testing.T) {
	tests := []struct {
		freq     Frequency
		expected string
	}{
		{FrequencyWeekly, "WEEKLY"},
		{FrequencyBiweekly, "BIWEEKLY"},
		{FrequencyMonthly, "MONTHLY"},
		{FrequencyQuarterly, "QUARTERLY"},
		{FrequencyYearly, "YEARLY"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.freq) != tt.expected {
				t.Errorf("Frequency = %q, want %q", tt.freq, tt.expected)
			}
		})
	}
}

func TestIsValidFrequency(t *testing.T) {
	tests := []struct {
		name     string
		freq     Frequency
		expected bool
	}{
		{"Weekly is valid", FrequencyWeekly, true},
		{"Biweekly is valid", FrequencyBiweekly, true},
		{"Monthly is valid", FrequencyMonthly, true},
		{"Quarterly is valid", FrequencyQuarterly, true},
		{"Yearly is valid", FrequencyYearly, true},
		{"Empty is invalid", Frequency(""), false},
		{"Daily is invalid", Frequency("DAILY"), false},
		{"Invalid string", Frequency("INVALID"), false},
		{"Lowercase monthly", Frequency("monthly"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidFrequency(tt.freq)
			if result != tt.expected {
				t.Errorf("isValidFrequency(%q) = %v, want %v", tt.freq, result, tt.expected)
			}
		})
	}
}

func TestRecurringInvoice_Validate(t *testing.T) {
	validLine := RecurringInvoiceLine{
		Description: "Service",
		Quantity:    decimal.NewFromInt(1),
		UnitPrice:   decimal.NewFromInt(100),
	}

	baseInvoice := func() RecurringInvoice {
		return RecurringInvoice{
			ID:                 "test-id",
			TenantID:           "tenant-id",
			Name:               "Monthly Services",
			ContactID:          "contact-id",
			Frequency:          FrequencyMonthly,
			StartDate:          time.Now(),
			PaymentTermsDays:   14,
			Lines:              []RecurringInvoiceLine{validLine},
		}
	}

	tests := []struct {
		name        string
		modify      func(*RecurringInvoice)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid recurring invoice",
			modify:      func(ri *RecurringInvoice) {},
			expectError: false,
		},
		{
			name:        "Missing name",
			modify:      func(ri *RecurringInvoice) { ri.Name = "" },
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name:        "Missing contact",
			modify:      func(ri *RecurringInvoice) { ri.ContactID = "" },
			expectError: true,
			errorMsg:    "contact is required",
		},
		{
			name:        "Missing start date",
			modify:      func(ri *RecurringInvoice) { ri.StartDate = time.Time{} },
			expectError: true,
			errorMsg:    "start date is required",
		},
		{
			name: "End date before start date",
			modify: func(ri *RecurringInvoice) {
				yesterday := ri.StartDate.Add(-24 * time.Hour)
				ri.EndDate = &yesterday
			},
			expectError: true,
			errorMsg:    "end date cannot be before start date",
		},
		{
			name:        "Invalid frequency",
			modify:      func(ri *RecurringInvoice) { ri.Frequency = Frequency("INVALID") },
			expectError: true,
			errorMsg:    "invalid frequency",
		},
		{
			name:        "Negative payment terms",
			modify:      func(ri *RecurringInvoice) { ri.PaymentTermsDays = -1 },
			expectError: true,
			errorMsg:    "payment terms days cannot be negative",
		},
		{
			name:        "No line items",
			modify:      func(ri *RecurringInvoice) { ri.Lines = []RecurringInvoiceLine{} },
			expectError: true,
			errorMsg:    "at least one line item is required",
		},
		{
			name: "Line without description",
			modify: func(ri *RecurringInvoice) {
				ri.Lines = []RecurringInvoiceLine{{Description: "", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(100)}}
			},
			expectError: true,
			errorMsg:    "line description is required",
		},
		{
			name: "Line with zero quantity",
			modify: func(ri *RecurringInvoice) {
				ri.Lines = []RecurringInvoiceLine{{Description: "Test", Quantity: decimal.Zero, UnitPrice: decimal.NewFromInt(100)}}
			},
			expectError: true,
			errorMsg:    "line quantity must be positive",
		},
		{
			name: "Line with negative quantity",
			modify: func(ri *RecurringInvoice) {
				ri.Lines = []RecurringInvoiceLine{{Description: "Test", Quantity: decimal.NewFromInt(-1), UnitPrice: decimal.NewFromInt(100)}}
			},
			expectError: true,
			errorMsg:    "line quantity must be positive",
		},
		{
			name: "Line with negative unit price",
			modify: func(ri *RecurringInvoice) {
				ri.Lines = []RecurringInvoiceLine{{Description: "Test", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(-100)}}
			},
			expectError: true,
			errorMsg:    "line unit price cannot be negative",
		},
		{
			name: "Valid end date",
			modify: func(ri *RecurringInvoice) {
				tomorrow := ri.StartDate.Add(24 * time.Hour)
				ri.EndDate = &tomorrow
			},
			expectError: false,
		},
		{
			name: "Zero payment terms is valid",
			modify: func(ri *RecurringInvoice) {
				ri.PaymentTermsDays = 0
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ri := baseInvoice()
			tt.modify(&ri)
			err := ri.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errorMsg)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestRecurringInvoice_CalculateNextDate(t *testing.T) {
	baseDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		freq     Frequency
		expected time.Time
	}{
		{
			name:     "Weekly",
			freq:     FrequencyWeekly,
			expected: time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Biweekly",
			freq:     FrequencyBiweekly,
			expected: time.Date(2025, 1, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Monthly",
			freq:     FrequencyMonthly,
			expected: time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Quarterly",
			freq:     FrequencyQuarterly,
			expected: time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Yearly",
			freq:     FrequencyYearly,
			expected: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Unknown defaults to monthly",
			freq:     Frequency("UNKNOWN"),
			expected: time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ri := RecurringInvoice{Frequency: tt.freq}
			result := ri.CalculateNextDate(baseDate)
			if !result.Equal(tt.expected) {
				t.Errorf("CalculateNextDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRecurringInvoice_CalculateNextDate_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		freq     Frequency
		from     time.Time
		expected time.Time
	}{
		{
			name:     "Monthly from Jan 31",
			freq:     FrequencyMonthly,
			from:     time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC), // Feb 31 rolls to Mar 3
		},
		{
			name:     "Yearly from Feb 29 leap year",
			freq:     FrequencyYearly,
			from:     time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), // 2024 is leap year
			expected: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),  // 2025 Feb 29 doesn't exist
		},
		{
			name:     "Weekly year boundary",
			freq:     FrequencyWeekly,
			from:     time.Date(2024, 12, 28, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Quarterly year boundary",
			freq:     FrequencyQuarterly,
			from:     time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ri := RecurringInvoice{Frequency: tt.freq}
			result := ri.CalculateNextDate(tt.from)
			if !result.Equal(tt.expected) {
				t.Errorf("CalculateNextDate(%v) = %v, want %v", tt.from, result, tt.expected)
			}
		})
	}
}

func TestRecurringInvoice_IsDue(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)
	lastMonth := now.AddDate(0, -1, 0)

	tests := []struct {
		name               string
		isActive           bool
		endDate            *time.Time
		nextGenerationDate time.Time
		expected           bool
	}{
		{
			name:               "Active and due (next date in past)",
			isActive:           true,
			endDate:            nil,
			nextGenerationDate: yesterday,
			expected:           true,
		},
		{
			name:               "Active and due (next date is now)",
			isActive:           true,
			endDate:            nil,
			nextGenerationDate: now.Add(-time.Second), // Just before now
			expected:           true,
		},
		{
			name:               "Active but not due (next date in future)",
			isActive:           true,
			endDate:            nil,
			nextGenerationDate: tomorrow,
			expected:           false,
		},
		{
			name:               "Inactive is never due",
			isActive:           false,
			endDate:            nil,
			nextGenerationDate: yesterday,
			expected:           false,
		},
		{
			name:               "Active but past end date",
			isActive:           true,
			endDate:            &lastMonth,
			nextGenerationDate: yesterday,
			expected:           false,
		},
		{
			name:               "Active with future end date and due",
			isActive:           true,
			endDate:            &tomorrow,
			nextGenerationDate: yesterday,
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ri := RecurringInvoice{
				IsActive:           tt.isActive,
				EndDate:            tt.endDate,
				NextGenerationDate: tt.nextGenerationDate,
			}
			result := ri.IsDue()
			if result != tt.expected {
				t.Errorf("IsDue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRecurringInvoice_JSONSerialization(t *testing.T) {
	endDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	ri := RecurringInvoice{
		ID:                 "ri-123",
		TenantID:           "tenant-1",
		Name:               "Monthly Support",
		ContactID:          "contact-1",
		ContactName:        "Acme Corp",
		InvoiceType:        "SALES",
		Currency:           "EUR",
		Frequency:          FrequencyMonthly,
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            &endDate,
		NextGenerationDate: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		PaymentTermsDays:   14,
		Reference:          "REF-001",
		Notes:              "Monthly support services",
		IsActive:           true,
		GeneratedCount:     5,
		Lines: []RecurringInvoiceLine{
			{
				ID:          "line-1",
				Description: "Support Service",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(500.00),
				VATRate:     decimal.NewFromFloat(22.00),
			},
		},
	}

	data, err := json.Marshal(ri)
	if err != nil {
		t.Fatalf("Failed to marshal RecurringInvoice: %v", err)
	}

	var parsed RecurringInvoice
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal RecurringInvoice: %v", err)
	}

	if parsed.ID != ri.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, ri.ID)
	}
	if parsed.Name != ri.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, ri.Name)
	}
	if parsed.Frequency != ri.Frequency {
		t.Errorf("Frequency = %q, want %q", parsed.Frequency, ri.Frequency)
	}
	if parsed.IsActive != ri.IsActive {
		t.Errorf("IsActive = %v, want %v", parsed.IsActive, ri.IsActive)
	}
	if len(parsed.Lines) != len(ri.Lines) {
		t.Errorf("Lines length = %d, want %d", len(parsed.Lines), len(ri.Lines))
	}
}

func TestRecurringInvoiceLine_JSONSerialization(t *testing.T) {
	accountID := "acc-1"
	productID := "prod-1"
	line := RecurringInvoiceLine{
		ID:                 "line-123",
		RecurringInvoiceID: "ri-123",
		LineNumber:         1,
		Description:        "Consulting Services",
		Quantity:           decimal.NewFromFloat(10.5),
		Unit:               "hours",
		UnitPrice:          decimal.NewFromFloat(150.00),
		DiscountPercent:    decimal.NewFromFloat(5.00),
		VATRate:            decimal.NewFromFloat(22.00),
		AccountID:          &accountID,
		ProductID:          &productID,
	}

	data, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("Failed to marshal RecurringInvoiceLine: %v", err)
	}

	var parsed RecurringInvoiceLine
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal RecurringInvoiceLine: %v", err)
	}

	if parsed.Description != line.Description {
		t.Errorf("Description = %q, want %q", parsed.Description, line.Description)
	}
	if !parsed.Quantity.Equal(line.Quantity) {
		t.Errorf("Quantity = %s, want %s", parsed.Quantity, line.Quantity)
	}
	if !parsed.UnitPrice.Equal(line.UnitPrice) {
		t.Errorf("UnitPrice = %s, want %s", parsed.UnitPrice, line.UnitPrice)
	}
}

func TestCreateRecurringInvoiceRequest_JSONSerialization(t *testing.T) {
	endDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	req := CreateRecurringInvoiceRequest{
		Name:             "Annual Contract",
		ContactID:        "contact-1",
		InvoiceType:      "SALES",
		Currency:         "EUR",
		Frequency:        FrequencyYearly,
		StartDate:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:          &endDate,
		PaymentTermsDays: 30,
		Reference:        "CONTRACT-2025",
		Notes:            "Annual service contract",
		Lines: []CreateRecurringInvoiceLineRequest{
			{
				Description: "Annual Support",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromFloat(12000.00),
				VATRate:     decimal.NewFromFloat(22.00),
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal CreateRecurringInvoiceRequest: %v", err)
	}

	var parsed CreateRecurringInvoiceRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal CreateRecurringInvoiceRequest: %v", err)
	}

	if parsed.Name != req.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, req.Name)
	}
	if parsed.Frequency != req.Frequency {
		t.Errorf("Frequency = %q, want %q", parsed.Frequency, req.Frequency)
	}
}

func TestGenerationResult_JSONSerialization(t *testing.T) {
	result := GenerationResult{
		RecurringInvoiceID:     "ri-123",
		GeneratedInvoiceID:     "inv-456",
		GeneratedInvoiceNumber: "INV-2025-001",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal GenerationResult: %v", err)
	}

	var parsed GenerationResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal GenerationResult: %v", err)
	}

	if parsed.RecurringInvoiceID != result.RecurringInvoiceID {
		t.Errorf("RecurringInvoiceID = %q, want %q", parsed.RecurringInvoiceID, result.RecurringInvoiceID)
	}
	if parsed.GeneratedInvoiceID != result.GeneratedInvoiceID {
		t.Errorf("GeneratedInvoiceID = %q, want %q", parsed.GeneratedInvoiceID, result.GeneratedInvoiceID)
	}
	if parsed.GeneratedInvoiceNumber != result.GeneratedInvoiceNumber {
		t.Errorf("GeneratedInvoiceNumber = %q, want %q", parsed.GeneratedInvoiceNumber, result.GeneratedInvoiceNumber)
	}
}

func TestLineCalculations(t *testing.T) {
	// Test that line amount calculations work correctly
	tests := []struct {
		name            string
		quantity        decimal.Decimal
		unitPrice       decimal.Decimal
		discountPercent decimal.Decimal
		vatRate         decimal.Decimal
		expectedNet     decimal.Decimal
		expectedVAT     decimal.Decimal
		expectedGross   decimal.Decimal
	}{
		{
			name:            "Simple calculation",
			quantity:        decimal.NewFromInt(2),
			unitPrice:       decimal.NewFromInt(100),
			discountPercent: decimal.Zero,
			vatRate:         decimal.NewFromFloat(20),
			expectedNet:     decimal.NewFromInt(200),
			expectedVAT:     decimal.NewFromInt(40),
			expectedGross:   decimal.NewFromInt(240),
		},
		{
			name:            "With discount",
			quantity:        decimal.NewFromInt(1),
			unitPrice:       decimal.NewFromInt(1000),
			discountPercent: decimal.NewFromFloat(10),
			vatRate:         decimal.NewFromFloat(20),
			expectedNet:     decimal.NewFromInt(900),
			expectedVAT:     decimal.NewFromInt(180),
			expectedGross:   decimal.NewFromInt(1080),
		},
		{
			name:            "Zero VAT",
			quantity:        decimal.NewFromFloat(1.5),
			unitPrice:       decimal.NewFromInt(100),
			discountPercent: decimal.Zero,
			vatRate:         decimal.Zero,
			expectedNet:     decimal.NewFromInt(150),
			expectedVAT:     decimal.Zero,
			expectedGross:   decimal.NewFromInt(150),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate line amount
			lineAmount := tt.quantity.Mul(tt.unitPrice)
			discount := lineAmount.Mul(tt.discountPercent).Div(decimal.NewFromInt(100))
			netAmount := lineAmount.Sub(discount)
			vatAmount := netAmount.Mul(tt.vatRate).Div(decimal.NewFromInt(100))
			grossAmount := netAmount.Add(vatAmount)

			if !netAmount.Equal(tt.expectedNet) {
				t.Errorf("Net = %s, want %s", netAmount, tt.expectedNet)
			}
			if !vatAmount.Equal(tt.expectedVAT) {
				t.Errorf("VAT = %s, want %s", vatAmount, tt.expectedVAT)
			}
			if !grossAmount.Equal(tt.expectedGross) {
				t.Errorf("Gross = %s, want %s", grossAmount, tt.expectedGross)
			}
		})
	}
}

func TestUpdateRecurringInvoiceRequest_PartialUpdate(t *testing.T) {
	// Test that partial updates work with pointer fields
	name := "Updated Name"
	freq := FrequencyQuarterly
	paymentTerms := 30

	req := UpdateRecurringInvoiceRequest{
		Name:             &name,
		Frequency:        &freq,
		PaymentTermsDays: &paymentTerms,
	}

	if req.Name == nil || *req.Name != "Updated Name" {
		t.Error("Name not set correctly")
	}
	if req.Frequency == nil || *req.Frequency != FrequencyQuarterly {
		t.Error("Frequency not set correctly")
	}
	if req.PaymentTermsDays == nil || *req.PaymentTermsDays != 30 {
		t.Error("PaymentTermsDays not set correctly")
	}
	// These should be nil for partial update
	if req.ContactID != nil {
		t.Error("ContactID should be nil for partial update")
	}
	if req.Reference != nil {
		t.Error("Reference should be nil for partial update")
	}
}
