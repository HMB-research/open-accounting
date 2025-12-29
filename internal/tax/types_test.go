package tax

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestKMDRow_Validate(t *testing.T) {
	row := KMDRow{
		Code:        "1",
		Description: "Taxable sales at standard rate",
		TaxBase:     decimal.NewFromInt(1000),
		TaxAmount:   decimal.NewFromInt(220),
	}
	err := row.Validate()
	assert.NoError(t, err)
}

func TestKMDRow_Validate_InvalidCode(t *testing.T) {
	row := KMDRow{
		Code:        "",
		Description: "Test",
		TaxBase:     decimal.NewFromInt(1000),
		TaxAmount:   decimal.NewFromInt(220),
	}
	err := row.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "code is required")
}

func TestKMDDeclaration_Period(t *testing.T) {
	decl := KMDDeclaration{
		TenantID:    "test-tenant",
		Year:        2025,
		Month:       1,
		SubmittedAt: nil,
	}
	assert.Equal(t, "2025-01", decl.Period())
}

func TestKMDDeclaration_CalculatePayable(t *testing.T) {
	decl := KMDDeclaration{
		TotalOutputVAT: decimal.NewFromInt(500),
		TotalInputVAT:  decimal.NewFromInt(200),
	}
	payable := decl.CalculatePayable()
	assert.True(t, payable.Equal(decimal.NewFromInt(300)))
}
