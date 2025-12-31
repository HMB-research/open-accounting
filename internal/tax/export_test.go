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

func TestExportKMDToXML_AllRowCodes(t *testing.T) {
	decl := &KMDDeclaration{
		ID:             "test-all-rows",
		TenantID:       "tenant-1",
		Year:           2025,
		Month:          3,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(500),
		TotalInputVAT:  decimal.NewFromInt(200),
		Rows: []KMDRow{
			{Code: KMDRow1, Description: "Standard rate 20%", TaxBase: decimal.NewFromInt(1000), TaxAmount: decimal.NewFromInt(200)},
			{Code: KMDRow2, Description: "Reduced rate 9%", TaxBase: decimal.NewFromInt(500), TaxAmount: decimal.NewFromInt(45)},
			{Code: KMDRow21, Description: "Accommodation 13%", TaxBase: decimal.NewFromInt(300), TaxAmount: decimal.NewFromInt(39)},
			{Code: KMDRow3, Description: "Zero-rated exports", TaxBase: decimal.NewFromInt(200), TaxAmount: decimal.Zero},
			{Code: KMDRow31, Description: "Intra-EU supplies", TaxBase: decimal.NewFromInt(150), TaxAmount: decimal.Zero},
			{Code: KMDRow4, Description: "Input VAT domestic", TaxBase: decimal.NewFromInt(400), TaxAmount: decimal.NewFromInt(80)},
			{Code: KMDRow5, Description: "Input VAT imports", TaxBase: decimal.NewFromInt(100), TaxAmount: decimal.NewFromInt(20)},
			{Code: KMDRow6, Description: "Fixed assets VAT", TaxBase: decimal.NewFromInt(500), TaxAmount: decimal.NewFromInt(100)},
			{Code: KMDRow7, Description: "VAT adjustments", TaxBase: decimal.Zero, TaxAmount: decimal.NewFromInt(10)},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	xml, err := ExportKMDToXML(decl, "EE12345678")
	assert.NoError(t, err)

	// Verify all row codes are present in XML
	xmlStr := string(xml)
	assert.Contains(t, xmlStr, "<rida1>1000")
	assert.Contains(t, xmlStr, "<rida1Km>200")
	assert.Contains(t, xmlStr, "<rida2>500")
	assert.Contains(t, xmlStr, "<rida2Km>45")
	assert.Contains(t, xmlStr, "<rida21>300")
	assert.Contains(t, xmlStr, "<rida21Km>39")
	assert.Contains(t, xmlStr, "<rida3>200")
	assert.Contains(t, xmlStr, "<rida31>150")
	assert.Contains(t, xmlStr, "<rida4>80")
	assert.Contains(t, xmlStr, "<rida5>20")
	assert.Contains(t, xmlStr, "<rida6>100")
	assert.Contains(t, xmlStr, "<rida7>10")
	assert.Contains(t, xmlStr, "<rida8>500")  // Total output VAT
	assert.Contains(t, xmlStr, "<rida9>200")  // Total input VAT
	assert.Contains(t, xmlStr, "<rida10>300") // VAT payable (500-200)
}

func TestExportKMDToXML_ZeroPayable(t *testing.T) {
	decl := &KMDDeclaration{
		ID:             "test-zero",
		TenantID:       "tenant-1",
		Year:           2025,
		Month:          4,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(100),
		TotalInputVAT:  decimal.NewFromInt(100),
		Rows:           []KMDRow{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	xml, err := ExportKMDToXML(decl, "EE12345678")
	assert.NoError(t, err)

	xmlStr := string(xml)
	// Neither row10 nor row11 should be present when payable is zero
	assert.NotContains(t, xmlStr, "<rida10>")
	assert.NotContains(t, xmlStr, "<rida11>")
}
