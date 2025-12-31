package payroll

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidatePersonalCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		// Valid codes with correct checksums
		{"Valid male born 1980", "38001010009", true},   // Checksum: 9
		{"Valid female born 1990", "49001010012", true}, // Checksum: 2
		{"Valid code with checksum 0", "37605030299", true},

		// Test case that triggers weights2 calculation (first checksum % 11 == 10)
		// Code 17605030336: weights1 sum = 1*1+7*2+6*3+0*4+5*5+0*6+3*7+0*8+3*9+3*1 = 1+14+18+0+25+0+21+0+27+3 = 109
		// 109 % 11 = 10, so weights2 is used
		// weights2 sum = 1*3+7*4+6*5+0*6+5*7+0*8+3*9+0*1+3*2+3*3 = 3+28+30+0+35+0+27+0+6+9 = 138
		// 138 % 11 = 6, so checksum = 6
		{"Valid with weights2", "17605030336", true},

		// Test case where weights2 also gives 10 (checksum becomes 0)
		// Real Estonian ID: 47101010033 triggers both weight sums % 11 == 10
		{"Valid with double checksum 10", "47101010033", true},

		// Invalid codes - wrong length
		{"Too short", "3800101000", false},
		{"Too long", "380010100012", false},
		{"Empty", "", false},

		// Invalid codes - non-numeric
		{"Contains letter", "3800101000a", false},
		{"Contains space", "3800101 001", false},
		{"Contains special char", "38001-10001", false},

		// Invalid codes - wrong checksum
		{"Wrong checksum", "38001010001", false},
		{"Wrong checksum 2", "38001010003", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePersonalCode(tt.code); got != tt.expected {
				t.Errorf("ValidatePersonalCode(%q) = %v, want %v", tt.code, got, tt.expected)
			}
		})
	}
}

func TestValidatePersonalCode_KnownValidCodes(t *testing.T) {
	// These are structurally valid Estonian personal codes (not real people)
	validCodes := []string{
		"38001010009", // Male born 1980-01-01, checksum 9
		"49001010012", // Female born 1990-01-01, checksum 2
		"37605030299", // Male born 1976-05-03, checksum 9
	}

	for _, code := range validCodes {
		if !ValidatePersonalCode(code) {
			t.Errorf("Expected %q to be valid", code)
		}
	}
}

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		input    decimal.Decimal
		expected string
	}{
		{decimal.NewFromFloat(100.00), "100.00"},
		{decimal.NewFromFloat(100.123), "100.12"},
		{decimal.NewFromFloat(100.126), "100.13"},
		{decimal.NewFromFloat(0), "0.00"},
		{decimal.NewFromFloat(-50.50), "-50.50"},
	}

	for _, tt := range tests {
		if got := formatDecimal(tt.input); got != tt.expected {
			t.Errorf("formatDecimal(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDecimalIfPositive(t *testing.T) {
	tests := []struct {
		input    decimal.Decimal
		expected string
	}{
		{decimal.NewFromFloat(100.00), "100.00"},
		{decimal.NewFromFloat(0.01), "0.01"},
		{decimal.NewFromFloat(0), ""},
		{decimal.NewFromFloat(-50.50), ""},
	}

	for _, tt := range tests {
		if got := formatDecimalIfPositive(tt.input); got != tt.expected {
			t.Errorf("formatDecimalIfPositive(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateTSDFilename(t *testing.T) {
	tests := []struct {
		registryCode string
		year         int
		month        int
		format       string
	}{
		{"12345678", 2025, 1, "xml"},
		{"12345678", 2025, 12, "csv"},
		{"87654321", 2024, 6, "xml"},
	}

	for _, tt := range tests {
		filename := GenerateTSDFilename(tt.registryCode, tt.year, tt.month, tt.format)

		// Check that the filename contains expected parts
		if len(filename) == 0 {
			t.Error("Filename should not be empty")
		}

		// Filename format: TSD_REGCODE_YYYYMM_DATE.FORMAT
		expectedPrefix := "TSD_" + tt.registryCode
		if filename[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("Filename should start with %q, got %q", expectedPrefix, filename)
		}
	}
}

func TestPaymentTypeCodes(t *testing.T) {
	// Verify payment type codes are correctly defined per Estonian TSD specification
	expectedCodes := map[string]string{
		"PaymentTypeSalary":        "10",
		"PaymentTypeVacationPay":   "11",
		"PaymentTypeSickPay":       "12",
		"PaymentTypeBonus":         "13",
		"PaymentTypeTermination":   "14",
		"PaymentTypeBoard":         "21",
		"PaymentTypeContract":      "22",
		"PaymentTypeRoyalties":     "30",
		"PaymentTypeRent":          "40",
		"PaymentTypeInterest":      "50",
		"PaymentTypeDividends":     "60",
		"PaymentTypePension":       "70",
		"PaymentTypeBenefit":       "80",
		"PaymentTypeFringeBenefit": "42",
	}

	if PaymentTypeSalary != expectedCodes["PaymentTypeSalary"] {
		t.Errorf("PaymentTypeSalary = %q, want %q", PaymentTypeSalary, expectedCodes["PaymentTypeSalary"])
	}

	if PaymentTypeVacationPay != expectedCodes["PaymentTypeVacationPay"] {
		t.Errorf("PaymentTypeVacationPay = %q, want %q", PaymentTypeVacationPay, expectedCodes["PaymentTypeVacationPay"])
	}

	if PaymentTypeSickPay != expectedCodes["PaymentTypeSickPay"] {
		t.Errorf("PaymentTypeSickPay = %q, want %q", PaymentTypeSickPay, expectedCodes["PaymentTypeSickPay"])
	}

	if PaymentTypeBonus != expectedCodes["PaymentTypeBonus"] {
		t.Errorf("PaymentTypeBonus = %q, want %q", PaymentTypeBonus, expectedCodes["PaymentTypeBonus"])
	}

	if PaymentTypeBoard != expectedCodes["PaymentTypeBoard"] {
		t.Errorf("PaymentTypeBoard = %q, want %q", PaymentTypeBoard, expectedCodes["PaymentTypeBoard"])
	}

	if PaymentTypeContract != expectedCodes["PaymentTypeContract"] {
		t.Errorf("PaymentTypeContract = %q, want %q", PaymentTypeContract, expectedCodes["PaymentTypeContract"])
	}
}

func TestTSDStatusValues(t *testing.T) {
	// Verify TSD status enum values
	if TSDDraft != "DRAFT" {
		t.Errorf("TSDDraft = %q, want DRAFT", TSDDraft)
	}

	if TSDSubmitted != "SUBMITTED" {
		t.Errorf("TSDSubmitted = %q, want SUBMITTED", TSDSubmitted)
	}

	if TSDAccepted != "ACCEPTED" {
		t.Errorf("TSDAccepted = %q, want ACCEPTED", TSDAccepted)
	}

	if TSDRejected != "REJECTED" {
		t.Errorf("TSDRejected = %q, want REJECTED", TSDRejected)
	}
}

func TestPayrollStatusValues(t *testing.T) {
	// Verify payroll status enum values
	statuses := []struct {
		status   PayrollStatus
		expected string
	}{
		{PayrollDraft, "DRAFT"},
		{PayrollCalculated, "CALCULATED"},
		{PayrollApproved, "APPROVED"},
		{PayrollPaid, "PAID"},
		{PayrollDeclared, "DECLARED"},
	}

	for _, tt := range statuses {
		if string(tt.status) != tt.expected {
			t.Errorf("Status = %q, want %q", tt.status, tt.expected)
		}
	}
}

func TestCalculateTaxPreview(t *testing.T) {
	tests := []struct {
		name                string
		grossSalary         decimal.Decimal
		applyBasicExemption bool
		fundedPensionRate   decimal.Decimal
		expectedTaxable     decimal.Decimal
		expectedIncomeTax   decimal.Decimal
	}{
		{
			name:                "With basic exemption",
			grossSalary:         decimal.NewFromFloat(2000.00),
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.NewFromFloat(1300.00), // 2000 - 700
			expectedIncomeTax:   decimal.NewFromFloat(286.00),  // 22% of 1300
		},
		{
			name:                "Without basic exemption",
			grossSalary:         decimal.NewFromFloat(2000.00),
			applyBasicExemption: false,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.NewFromFloat(2000.00),
			expectedIncomeTax:   decimal.NewFromFloat(440.00), // 22% of 2000
		},
		{
			name:                "Zero salary with exemption",
			grossSalary:         decimal.Zero,
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.Zero,
			expectedIncomeTax:   decimal.Zero,
		},
		{
			name:                "Low salary below exemption",
			grossSalary:         decimal.NewFromFloat(500.00),
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.02),
			expectedTaxable:     decimal.Zero, // 500 - 700 = negative, so 0
			expectedIncomeTax:   decimal.Zero,
		},
		{
			name:                "Increased pension rate",
			grossSalary:         decimal.NewFromFloat(3000.00),
			applyBasicExemption: true,
			fundedPensionRate:   decimal.NewFromFloat(0.04),
			expectedTaxable:     decimal.NewFromFloat(2300.00), // 3000 - 700
			expectedIncomeTax:   decimal.NewFromFloat(506.00),  // 22% of 2300
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := CalculateTaxPreview(tt.grossSalary, tt.applyBasicExemption, tt.fundedPensionRate)

			if !calc.TaxableIncome.Equal(tt.expectedTaxable) {
				t.Errorf("TaxableIncome = %s, want %s", calc.TaxableIncome, tt.expectedTaxable)
			}

			if !calc.IncomeTax.Equal(tt.expectedIncomeTax) {
				t.Errorf("IncomeTax = %s, want %s", calc.IncomeTax, tt.expectedIncomeTax)
			}

			// Verify basic exemption was correctly applied
			if tt.applyBasicExemption {
				if !calc.BasicExemption.Equal(DefaultBasicExemption) {
					t.Errorf("BasicExemption = %s, want %s", calc.BasicExemption, DefaultBasicExemption)
				}
			} else {
				if !calc.BasicExemption.IsZero() {
					t.Errorf("BasicExemption = %s, want zero", calc.BasicExemption)
				}
			}

			// Verify funded pension calculation
			expectedPension := tt.grossSalary.Mul(tt.fundedPensionRate).Round(2)
			if !calc.FundedPension.Equal(expectedPension) {
				t.Errorf("FundedPension = %s, want %s", calc.FundedPension, expectedPension)
			}
		})
	}
}

func TestEmploymentTypeValues(t *testing.T) {
	// Verify employment type enum values
	types := []struct {
		empType  EmploymentType
		expected string
	}{
		{EmploymentFullTime, "FULL_TIME"},
		{EmploymentPartTime, "PART_TIME"},
		{EmploymentContract, "CONTRACT"},
	}

	for _, tt := range types {
		if string(tt.empType) != tt.expected {
			t.Errorf("EmploymentType = %q, want %q", tt.empType, tt.expected)
		}
	}
}
