package payroll

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculateEstonianTaxes(t *testing.T) {
	tests := []struct {
		name               string
		grossSalary        decimal.Decimal
		basicExemption     decimal.Decimal
		fundedPensionRate  decimal.Decimal
		expectedIncomeTax  decimal.Decimal
		expectedNetSalary  decimal.Decimal
		expectedSocialTax  decimal.Decimal
		expectedTotalCost  decimal.Decimal
	}{
		{
			name:               "Standard salary with full exemption",
			grossSalary:        decimal.NewFromFloat(2000.00),
			basicExemption:     decimal.NewFromFloat(700.00),
			fundedPensionRate:  decimal.NewFromFloat(0.02),
			expectedIncomeTax:  decimal.NewFromFloat(286.00), // 22% of (2000-700)
			expectedNetSalary:  decimal.NewFromFloat(1642.00), // 2000 - 286 - 32 - 40
			expectedSocialTax:  decimal.NewFromFloat(660.00), // 33% of 2000
			expectedTotalCost:  decimal.NewFromFloat(2676.00), // 2000 + 660 + 16
		},
		{
			name:               "Minimum wage with exemption",
			grossSalary:        decimal.NewFromFloat(820.00),
			basicExemption:     decimal.NewFromFloat(700.00),
			fundedPensionRate:  decimal.NewFromFloat(0.02),
			expectedIncomeTax:  decimal.NewFromFloat(26.40), // 22% of (820-700)
			expectedNetSalary:  decimal.NewFromFloat(764.08), // 820 - 26.40 - 13.12 - 16.40
			expectedSocialTax:  decimal.NewFromFloat(270.60), // Minimum social tax
			expectedTotalCost:  decimal.NewFromFloat(1097.16), // 820 + 270.60 + 6.56
		},
		{
			name:               "Salary without basic exemption",
			grossSalary:        decimal.NewFromFloat(3000.00),
			basicExemption:     decimal.NewFromFloat(0),
			fundedPensionRate:  decimal.NewFromFloat(0.02),
			expectedIncomeTax:  decimal.NewFromFloat(660.00), // 22% of 3000
			expectedNetSalary:  decimal.NewFromFloat(2232.00), // 3000 - 660 - 48 - 60
			expectedSocialTax:  decimal.NewFromFloat(990.00), // 33% of 3000
			expectedTotalCost:  decimal.NewFromFloat(4014.00), // 3000 + 990 + 24
		},
		{
			name:               "Salary with increased pension rate",
			grossSalary:        decimal.NewFromFloat(2000.00),
			basicExemption:     decimal.NewFromFloat(700.00),
			fundedPensionRate:  decimal.NewFromFloat(0.04),
			expectedIncomeTax:  decimal.NewFromFloat(286.00), // 22% of (2000-700)
			expectedNetSalary:  decimal.NewFromFloat(1602.00), // 2000 - 286 - 32 - 80
			expectedSocialTax:  decimal.NewFromFloat(660.00), // 33% of 2000
			expectedTotalCost:  decimal.NewFromFloat(2676.00), // 2000 + 660 + 16
		},
		{
			name:               "Low salary below exemption",
			grossSalary:        decimal.NewFromFloat(500.00),
			basicExemption:     decimal.NewFromFloat(700.00),
			fundedPensionRate:  decimal.NewFromFloat(0.02),
			expectedIncomeTax:  decimal.NewFromFloat(0), // Taxable income is 0
			expectedNetSalary:  decimal.NewFromFloat(482.00), // 500 - 0 - 8 - 10
			expectedSocialTax:  decimal.NewFromFloat(270.60), // Minimum social tax
			expectedTotalCost:  decimal.NewFromFloat(774.60), // 500 + 270.60 + 4
		},
		{
			name:               "No pension contributions",
			grossSalary:        decimal.NewFromFloat(2000.00),
			basicExemption:     decimal.NewFromFloat(700.00),
			fundedPensionRate:  decimal.NewFromFloat(0),
			expectedIncomeTax:  decimal.NewFromFloat(286.00), // 22% of (2000-700)
			expectedNetSalary:  decimal.NewFromFloat(1682.00), // 2000 - 286 - 32 - 0
			expectedSocialTax:  decimal.NewFromFloat(660.00), // 33% of 2000
			expectedTotalCost:  decimal.NewFromFloat(2676.00), // 2000 + 660 + 16
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
