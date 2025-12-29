package tax

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestService_AggregateVATByCode(t *testing.T) {
	rows := aggregateVATByCode([]VATEntry{
		{VATCode: "1", TaxBase: 1000, TaxAmount: 220, IsOutput: true},
		{VATCode: "1", TaxBase: 500, TaxAmount: 110, IsOutput: true},
		{VATCode: "4", TaxBase: 300, TaxAmount: 66, IsOutput: false},
	})

	assert.Len(t, rows, 2)

	// Find row 1 (output VAT)
	var row1 *KMDRow
	for i := range rows {
		if rows[i].Code == "1" {
			row1 = &rows[i]
			break
		}
	}
	assert.NotNil(t, row1)
	assert.Equal(t, "1500", row1.TaxBase.String())
	assert.Equal(t, "330", row1.TaxAmount.String())
}

func TestMapVATRateToKMDCode(t *testing.T) {
	tests := []struct {
		rate     decimal.Decimal
		isOutput bool
		expected string
	}{
		{decimal.NewFromInt(22), true, KMDRow1},
		{decimal.NewFromInt(24), true, KMDRow1},
		{decimal.NewFromInt(20), true, KMDRow1},
		{decimal.NewFromInt(13), true, KMDRow21},
		{decimal.NewFromInt(9), true, KMDRow2},
		{decimal.NewFromInt(0), true, KMDRow3},
		{decimal.NewFromInt(22), false, KMDRow4},
	}

	for _, tt := range tests {
		result := mapVATRateToKMDCode(tt.rate, tt.isOutput)
		assert.Equal(t, tt.expected, result, "rate=%v, isOutput=%v", tt.rate, tt.isOutput)
	}
}

func TestGetKMDRowDescription(t *testing.T) {
	desc := getKMDRowDescription(KMDRow1)
	assert.Contains(t, desc, "standard")

	desc = getKMDRowDescription(KMDRow4)
	assert.Contains(t, desc, "Input VAT")

	desc = getKMDRowDescription("unknown")
	assert.Equal(t, "Unknown", desc)
}
