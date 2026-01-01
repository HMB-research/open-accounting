package tax

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKMDDeclaration_Period(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    int
		expected string
	}{
		{"January 2025", 2025, 1, "2025-01"},
		{"December 2024", 2024, 12, "2024-12"},
		{"October 2025", 2025, 10, "2025-10"},
		{"Single digit month", 2025, 5, "2025-05"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := KMDDeclaration{Year: tt.year, Month: tt.month}
			assert.Equal(t, tt.expected, d.Period())
		})
	}
}

func TestKMDDeclaration_CalculatePayable(t *testing.T) {
	tests := []struct {
		name      string
		outputVAT decimal.Decimal
		inputVAT  decimal.Decimal
		expected  decimal.Decimal
	}{
		{
			"Positive payable - more output than input",
			decimal.NewFromFloat(1000),
			decimal.NewFromFloat(400),
			decimal.NewFromFloat(600),
		},
		{
			"Zero payable - equal VAT",
			decimal.NewFromFloat(500),
			decimal.NewFromFloat(500),
			decimal.Zero,
		},
		{
			"Negative payable - refund due",
			decimal.NewFromFloat(200),
			decimal.NewFromFloat(500),
			decimal.NewFromFloat(-300),
		},
		{
			"Large amounts",
			decimal.NewFromFloat(100000.50),
			decimal.NewFromFloat(45000.25),
			decimal.NewFromFloat(55000.25),
		},
		{
			"Zero values",
			decimal.Zero,
			decimal.Zero,
			decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := KMDDeclaration{
				TotalOutputVAT: tt.outputVAT,
				TotalInputVAT:  tt.inputVAT,
			}
			assert.True(t, d.CalculatePayable().Equal(tt.expected),
				"got %s, want %s", d.CalculatePayable(), tt.expected)
		})
	}
}

func TestKMDRow_Validate(t *testing.T) {
	t.Run("valid row with code", func(t *testing.T) {
		row := KMDRow{
			Code:        KMDRow1,
			Description: "Taxable sales",
			TaxBase:     decimal.NewFromFloat(1000),
			TaxAmount:   decimal.NewFromFloat(220),
		}
		err := row.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid row - empty code", func(t *testing.T) {
		row := KMDRow{
			Code:        "",
			Description: "Invalid row",
			TaxBase:     decimal.NewFromFloat(1000),
			TaxAmount:   decimal.NewFromFloat(220),
		}
		err := row.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "code is required")
	})
}

func TestKMDRowCodes(t *testing.T) {
	// Verify the KMD row codes match Estonian tax authority definitions
	assert.Equal(t, "1", KMDRow1)
	assert.Equal(t, "2", KMDRow2)
	assert.Equal(t, "21", KMDRow21)
	assert.Equal(t, "3", KMDRow3)
	assert.Equal(t, "31", KMDRow31)
	assert.Equal(t, "4", KMDRow4)
	assert.Equal(t, "5", KMDRow5)
	assert.Equal(t, "6", KMDRow6)
	assert.Equal(t, "7", KMDRow7)
	assert.Equal(t, "8", KMDRow8)
	assert.Equal(t, "9", KMDRow9)
	assert.Equal(t, "10", KMDRow10)
	assert.Equal(t, "11", KMDRow11)
}

func TestKMDExportFormat_Values(t *testing.T) {
	assert.Equal(t, KMDExportFormat("XML"), KMDFormatXML)
	assert.Equal(t, KMDExportFormat("JSON"), KMDFormatJSON)
}

func TestCreateKMDRequest_Fields(t *testing.T) {
	req := CreateKMDRequest{
		Year:  2025,
		Month: 6,
	}
	assert.Equal(t, 2025, req.Year)
	assert.Equal(t, 6, req.Month)
}
