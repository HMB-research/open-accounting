package tax

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestExportKMDToXML(t *testing.T) {
	decl := &KMDDeclaration{
		ID:             "test-id",
		TenantID:       "tenant-1",
		Year:           2025,
		Month:          1,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(330),
		TotalInputVAT:  decimal.NewFromInt(66),
		Rows: []KMDRow{
			{Code: "1", Description: "Standard rate", TaxBase: decimal.NewFromInt(1500), TaxAmount: decimal.NewFromInt(330)},
			{Code: "4", Description: "Input VAT", TaxBase: decimal.NewFromInt(300), TaxAmount: decimal.NewFromInt(66)},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	xml, err := ExportKMDToXML(decl, "12345678")
	assert.NoError(t, err)
	assert.Contains(t, string(xml), "<?xml version=")
	assert.Contains(t, string(xml), "<KMD>")
	assert.Contains(t, string(xml), "<periood>2025-01</periood>")
	assert.Contains(t, string(xml), "<rida1>1500")
	assert.Contains(t, string(xml), "</KMD>")
}

func TestExportKMDToXML_WithRefund(t *testing.T) {
	decl := &KMDDeclaration{
		ID:             "test-id",
		TenantID:       "tenant-1",
		Year:           2025,
		Month:          2,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(100),
		TotalInputVAT:  decimal.NewFromInt(200),
		Rows:           []KMDRow{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	xml, err := ExportKMDToXML(decl, "12345678")
	assert.NoError(t, err)
	// Should have refundable amount in row 11
	assert.Contains(t, string(xml), "<rida11>100</rida11>")
}

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		input    decimal.Decimal
		expected string
	}{
		{decimal.NewFromFloat(1234.567), "1234.57"},
		{decimal.NewFromInt(100), "100"},
		{decimal.NewFromFloat(0.005), "0.01"},
	}

	for _, tt := range tests {
		result := formatDecimal(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
