package payroll

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculateEstonianTaxes(t *testing.T) {
	tests := []struct {
		name              string
		grossSalary       decimal.Decimal
		basicExemption    decimal.Decimal
		fundedPensionRate decimal.Decimal
		expectedIncomeTax decimal.Decimal
		expectedNetSalary decimal.Decimal
		expectedSocialTax decimal.Decimal
		expectedTotalCost decimal.Decimal
	}{
		{
			name:              "Standard salary with full exemption",
			grossSalary:       decimal.NewFromFloat(2000.00),
			basicExemption:    decimal.NewFromFloat(700.00),
			fundedPensionRate: decimal.NewFromFloat(0.02),
			expectedIncomeTax: decimal.NewFromFloat(286.00),  // 22% of (2000-700)
			expectedNetSalary: decimal.NewFromFloat(1642.00), // 2000 - 286 - 32 - 40
			expectedSocialTax: decimal.NewFromFloat(660.00),  // 33% of 2000
			expectedTotalCost: decimal.NewFromFloat(2676.00), // 2000 + 660 + 16
		},
		{
			name:              "Minimum wage with exemption",
			grossSalary:       decimal.NewFromFloat(820.00),
			basicExemption:    decimal.NewFromFloat(700.00),
			fundedPensionRate: decimal.NewFromFloat(0.02),
			expectedIncomeTax: decimal.NewFromFloat(26.40),   // 22% of (820-700)
			expectedNetSalary: decimal.NewFromFloat(764.08),  // 820 - 26.40 - 13.12 - 16.40
			expectedSocialTax: decimal.NewFromFloat(270.60),  // Minimum social tax
			expectedTotalCost: decimal.NewFromFloat(1097.16), // 820 + 270.60 + 6.56
		},
		{
			name:              "Salary without basic exemption",
			grossSalary:       decimal.NewFromFloat(3000.00),
			basicExemption:    decimal.NewFromFloat(0),
			fundedPensionRate: decimal.NewFromFloat(0.02),
			expectedIncomeTax: decimal.NewFromFloat(660.00),  // 22% of 3000
			expectedNetSalary: decimal.NewFromFloat(2232.00), // 3000 - 660 - 48 - 60
			expectedSocialTax: decimal.NewFromFloat(990.00),  // 33% of 3000
			expectedTotalCost: decimal.NewFromFloat(4014.00), // 3000 + 990 + 24
		},
		{
			name:              "Salary with increased pension rate",
			grossSalary:       decimal.NewFromFloat(2000.00),
			basicExemption:    decimal.NewFromFloat(700.00),
			fundedPensionRate: decimal.NewFromFloat(0.04),
			expectedIncomeTax: decimal.NewFromFloat(286.00),  // 22% of (2000-700)
			expectedNetSalary: decimal.NewFromFloat(1602.00), // 2000 - 286 - 32 - 80
			expectedSocialTax: decimal.NewFromFloat(660.00),  // 33% of 2000
			expectedTotalCost: decimal.NewFromFloat(2676.00), // 2000 + 660 + 16
		},
		{
			name:              "Low salary below exemption",
			grossSalary:       decimal.NewFromFloat(500.00),
			basicExemption:    decimal.NewFromFloat(700.00),
			fundedPensionRate: decimal.NewFromFloat(0.02),
			expectedIncomeTax: decimal.NewFromFloat(0),      // Taxable income is 0
			expectedNetSalary: decimal.NewFromFloat(482.00), // 500 - 0 - 8 - 10
			expectedSocialTax: decimal.NewFromFloat(270.60), // Minimum social tax
			expectedTotalCost: decimal.NewFromFloat(774.60), // 500 + 270.60 + 4
		},
		{
			name:              "No pension contributions",
			grossSalary:       decimal.NewFromFloat(2000.00),
			basicExemption:    decimal.NewFromFloat(700.00),
			fundedPensionRate: decimal.NewFromFloat(0),
			expectedIncomeTax: decimal.NewFromFloat(286.00),  // 22% of (2000-700)
			expectedNetSalary: decimal.NewFromFloat(1682.00), // 2000 - 286 - 32 - 0
			expectedSocialTax: decimal.NewFromFloat(660.00),  // 33% of 2000
			expectedTotalCost: decimal.NewFromFloat(2676.00), // 2000 + 660 + 16
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := CalculateEstonianTaxes(tt.grossSalary, tt.basicExemption, tt.fundedPensionRate)

			if !calc.IncomeTax.Equal(tt.expectedIncomeTax) {
				t.Errorf("IncomeTax = %s, want %s", calc.IncomeTax, tt.expectedIncomeTax)
			}

			if !calc.NetSalary.Equal(tt.expectedNetSalary) {
				t.Errorf("NetSalary = %s, want %s", calc.NetSalary, tt.expectedNetSalary)
			}

			if !calc.SocialTax.Equal(tt.expectedSocialTax) {
				t.Errorf("SocialTax = %s, want %s", calc.SocialTax, tt.expectedSocialTax)
			}

			if !calc.TotalEmployerCost.Equal(tt.expectedTotalCost) {
				t.Errorf("TotalEmployerCost = %s, want %s", calc.TotalEmployerCost, tt.expectedTotalCost)
			}
		})
	}
}

func TestCalculateEstonianTaxes_ZeroSalary(t *testing.T) {
	calc := CalculateEstonianTaxes(decimal.Zero, decimal.NewFromFloat(700), decimal.NewFromFloat(0.02))

	if !calc.IncomeTax.IsZero() {
		t.Errorf("IncomeTax should be zero, got %s", calc.IncomeTax)
	}

	if !calc.NetSalary.IsZero() {
		t.Errorf("NetSalary should be zero, got %s", calc.NetSalary)
	}

	if !calc.SocialTax.IsZero() {
		t.Errorf("SocialTax should be zero for zero salary, got %s", calc.SocialTax)
	}
}

func TestTaxRates(t *testing.T) {
	// Verify the 2025 Estonian tax rates are correctly defined
	if !IncomeTaxRate.Equal(decimal.NewFromFloat(0.22)) {
		t.Errorf("IncomeTaxRate = %s, want 0.22", IncomeTaxRate)
	}

	if !SocialTaxRate.Equal(decimal.NewFromFloat(0.33)) {
		t.Errorf("SocialTaxRate = %s, want 0.33", SocialTaxRate)
	}

	if !UnemploymentEmployeeRate.Equal(decimal.NewFromFloat(0.016)) {
		t.Errorf("UnemploymentEmployeeRate = %s, want 0.016", UnemploymentEmployeeRate)
	}

	if !UnemploymentEmployerRate.Equal(decimal.NewFromFloat(0.008)) {
		t.Errorf("UnemploymentEmployerRate = %s, want 0.008", UnemploymentEmployerRate)
	}

	if !DefaultBasicExemption.Equal(decimal.NewFromFloat(700.00)) {
		t.Errorf("DefaultBasicExemption = %s, want 700.00", DefaultBasicExemption)
	}

	if !MinimumSocialTaxBase.Equal(decimal.NewFromFloat(820.00)) {
		t.Errorf("MinimumSocialTaxBase = %s, want 820.00", MinimumSocialTaxBase)
	}

	if !MinimumSocialTax.Equal(decimal.NewFromFloat(270.60)) {
		t.Errorf("MinimumSocialTax = %s, want 270.60", MinimumSocialTax)
	}
}

func TestEmployee_FullName(t *testing.T) {
	tests := []struct {
		firstName string
		lastName  string
		expected  string
	}{
		{"Mari", "Maasikas", "Mari Maasikas"},
		{"John", "Doe", "John Doe"},
		{"", "Last", " Last"},
		{"First", "", "First "},
	}

	for _, tt := range tests {
		employee := Employee{
			FirstName: tt.firstName,
			LastName:  tt.lastName,
		}

		if got := employee.FullName(); got != tt.expected {
			t.Errorf("FullName() = %q, want %q", got, tt.expected)
		}
	}
}

func TestNewService(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Error("NewService should return a non-nil service")
	}
}

func TestNewService_WithNilPool(t *testing.T) {
	// Service can be created without a pool for testing
	service := NewService(nil)
	if service == nil {
		t.Error("NewService should return a non-nil service even with nil pool")
	}
}

func TestEmploymentTypeConstants(t *testing.T) {
	tests := []struct {
		empType  EmploymentType
		expected string
	}{
		{EmploymentFullTime, "FULL_TIME"},
		{EmploymentPartTime, "PART_TIME"},
		{EmploymentContract, "CONTRACT"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.empType) != tt.expected {
				t.Errorf("EmploymentType = %q, want %q", tt.empType, tt.expected)
			}
		})
	}
}

func TestPayrollStatusConstants(t *testing.T) {
	tests := []struct {
		status   PayrollStatus
		expected string
	}{
		{PayrollDraft, "DRAFT"},
		{PayrollCalculated, "CALCULATED"},
		{PayrollApproved, "APPROVED"},
		{PayrollPaid, "PAID"},
		{PayrollDeclared, "DECLARED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("PayrollStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestTSDStatusConstants(t *testing.T) {
	tests := []struct {
		status   TSDStatus
		expected string
	}{
		{TSDDraft, "DRAFT"},
		{TSDSubmitted, "SUBMITTED"},
		{TSDAccepted, "ACCEPTED"},
		{TSDRejected, "REJECTED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("TSDStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestFundedPensionRates(t *testing.T) {
	if !FundedPensionRateDefault.Equal(decimal.NewFromFloat(0.02)) {
		t.Errorf("FundedPensionRateDefault = %s, want 0.02", FundedPensionRateDefault)
	}

	if !FundedPensionRateIncreased.Equal(decimal.NewFromFloat(0.04)) {
		t.Errorf("FundedPensionRateIncreased = %s, want 0.04", FundedPensionRateIncreased)
	}
}

func TestTaxCalculation_Deductions(t *testing.T) {
	// Test the deduction calculations explicitly
	calc := CalculateEstonianTaxes(
		decimal.NewFromFloat(2000.00),
		decimal.NewFromFloat(700.00),
		decimal.NewFromFloat(0.02),
	)

	// Unemployment insurance (employee): 2000 * 0.016 = 32
	expectedUnemploymentEE := decimal.NewFromFloat(32.00)
	if !calc.UnemploymentEE.Equal(expectedUnemploymentEE) {
		t.Errorf("UnemploymentEE = %s, want %s", calc.UnemploymentEE, expectedUnemploymentEE)
	}

	// Funded pension: 2000 * 0.02 = 40
	expectedFundedPension := decimal.NewFromFloat(40.00)
	if !calc.FundedPension.Equal(expectedFundedPension) {
		t.Errorf("FundedPension = %s, want %s", calc.FundedPension, expectedFundedPension)
	}

	// Total deductions: 286 + 32 + 40 = 358
	expectedTotalDeductions := decimal.NewFromFloat(358.00)
	if !calc.TotalDeductions.Equal(expectedTotalDeductions) {
		t.Errorf("TotalDeductions = %s, want %s", calc.TotalDeductions, expectedTotalDeductions)
	}

	// Unemployment insurance (employer): 2000 * 0.008 = 16
	expectedUnemploymentER := decimal.NewFromFloat(16.00)
	if !calc.UnemploymentER.Equal(expectedUnemploymentER) {
		t.Errorf("UnemploymentER = %s, want %s", calc.UnemploymentER, expectedUnemploymentER)
	}
}

func TestTaxCalculation_TaxableIncome(t *testing.T) {
	tests := []struct {
		name            string
		grossSalary     decimal.Decimal
		basicExemption  decimal.Decimal
		expectedTaxable decimal.Decimal
	}{
		{
			name:            "Normal taxable income",
			grossSalary:     decimal.NewFromFloat(2000.00),
			basicExemption:  decimal.NewFromFloat(700.00),
			expectedTaxable: decimal.NewFromFloat(1300.00),
		},
		{
			name:            "Zero taxable income when exemption exceeds gross",
			grossSalary:     decimal.NewFromFloat(500.00),
			basicExemption:  decimal.NewFromFloat(700.00),
			expectedTaxable: decimal.Zero,
		},
		{
			name:            "No exemption",
			grossSalary:     decimal.NewFromFloat(1500.00),
			basicExemption:  decimal.Zero,
			expectedTaxable: decimal.NewFromFloat(1500.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := CalculateEstonianTaxes(tt.grossSalary, tt.basicExemption, decimal.NewFromFloat(0.02))
			if !calc.TaxableIncome.Equal(tt.expectedTaxable) {
				t.Errorf("TaxableIncome = %s, want %s", calc.TaxableIncome, tt.expectedTaxable)
			}
		})
	}
}

func TestEmployee_Fields(t *testing.T) {
	emp := Employee{
		ID:                  "emp-123",
		TenantID:            "tenant-456",
		EmployeeNumber:      "E001",
		FirstName:           "Mari",
		LastName:            "Maasikas",
		PersonalCode:        "38501234567",
		Email:               "mari@example.com",
		Phone:               "+372 5551234",
		Position:            "Developer",
		Department:          "Engineering",
		EmploymentType:      EmploymentFullTime,
		TaxResidency:        "EE",
		ApplyBasicExemption: true,
		IsActive:            true,
	}

	if emp.FullName() != "Mari Maasikas" {
		t.Errorf("FullName() = %q, want %q", emp.FullName(), "Mari Maasikas")
	}

	if emp.EmploymentType != EmploymentFullTime {
		t.Errorf("EmploymentType = %q, want %q", emp.EmploymentType, EmploymentFullTime)
	}

	if !emp.ApplyBasicExemption {
		t.Error("ApplyBasicExemption should be true")
	}

	if !emp.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestPayrollRun_Fields(t *testing.T) {
	run := PayrollRun{
		ID:                "run-123",
		TenantID:          "tenant-456",
		PeriodYear:        2025,
		PeriodMonth:       1,
		Status:            PayrollDraft,
		TotalGross:        decimal.NewFromFloat(10000.00),
		TotalNet:          decimal.NewFromFloat(7500.00),
		TotalEmployerCost: decimal.NewFromFloat(13500.00),
	}

	if run.Status != PayrollDraft {
		t.Errorf("Status = %q, want %q", run.Status, PayrollDraft)
	}

	if run.PeriodYear != 2025 {
		t.Errorf("PeriodYear = %d, want 2025", run.PeriodYear)
	}

	if run.PeriodMonth != 1 {
		t.Errorf("PeriodMonth = %d, want 1", run.PeriodMonth)
	}
}

func TestPayslip_Fields(t *testing.T) {
	slip := Payslip{
		ID:                      "slip-123",
		TenantID:                "tenant-456",
		PayrollRunID:            "run-123",
		EmployeeID:              "emp-123",
		GrossSalary:             decimal.NewFromFloat(2000.00),
		TaxableIncome:           decimal.NewFromFloat(1300.00),
		IncomeTax:               decimal.NewFromFloat(286.00),
		UnemploymentInsuranceEE: decimal.NewFromFloat(32.00),
		FundedPension:           decimal.NewFromFloat(40.00),
		NetSalary:               decimal.NewFromFloat(1642.00),
		SocialTax:               decimal.NewFromFloat(660.00),
		UnemploymentInsuranceER: decimal.NewFromFloat(16.00),
		TotalEmployerCost:       decimal.NewFromFloat(2676.00),
		PaymentStatus:           "PENDING",
	}

	if !slip.GrossSalary.Equal(decimal.NewFromFloat(2000.00)) {
		t.Errorf("GrossSalary = %s, want 2000.00", slip.GrossSalary)
	}

	if !slip.NetSalary.Equal(decimal.NewFromFloat(1642.00)) {
		t.Errorf("NetSalary = %s, want 1642.00", slip.NetSalary)
	}

	if slip.PaymentStatus != "PENDING" {
		t.Errorf("PaymentStatus = %q, want PENDING", slip.PaymentStatus)
	}
}

func TestTSDDeclaration_Fields(t *testing.T) {
	decl := TSDDeclaration{
		ID:                  "tsd-123",
		TenantID:            "tenant-456",
		PeriodYear:          2025,
		PeriodMonth:         1,
		TotalPayments:       decimal.NewFromFloat(10000.00),
		TotalIncomeTax:      decimal.NewFromFloat(1430.00),
		TotalSocialTax:      decimal.NewFromFloat(3300.00),
		TotalUnemploymentER: decimal.NewFromFloat(80.00),
		TotalUnemploymentEE: decimal.NewFromFloat(160.00),
		TotalFundedPension:  decimal.NewFromFloat(200.00),
		Status:              TSDDraft,
	}

	if decl.Status != TSDDraft {
		t.Errorf("Status = %q, want %q", decl.Status, TSDDraft)
	}

	if !decl.TotalPayments.Equal(decimal.NewFromFloat(10000.00)) {
		t.Errorf("TotalPayments = %s, want 10000.00", decl.TotalPayments)
	}
}

func TestCreateEmployeeRequest_Fields(t *testing.T) {
	req := CreateEmployeeRequest{
		FirstName:            "Mari",
		LastName:             "Maasikas",
		PersonalCode:         "38501234567",
		Email:                "mari@example.com",
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: decimal.NewFromFloat(700.00),
		FundedPensionRate:    decimal.NewFromFloat(0.02),
	}

	if req.FirstName != "Mari" {
		t.Errorf("FirstName = %q, want Mari", req.FirstName)
	}

	if req.EmploymentType != EmploymentFullTime {
		t.Errorf("EmploymentType = %q, want FULL_TIME", req.EmploymentType)
	}
}
