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
		{"Valid male born 1980", "38001010009", true}, // Checksum: 9
		{"Valid female born 1990", "49001010012", true}, // Checksum: 2
		{"Valid code with checksum 0", "37605030299", true},

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
