package tax

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxWave4NewServiceNilPool(t *testing.T) {
	service := NewService(nil)

	if service == nil {
		t.Fatal("NewService(nil) returned nil")
	}
	assert.Nil(t, service.repo)
}

func TestTaxWave4KMDHistoryImportRecordInvalidEdges(t *testing.T) {
	_, err := buildKMDHistoryImportRecord(kmdHistoryImportRow{values: map[string]string{
		"year":       "1899",
		"month":      "12",
		"row_code":   "1",
		"tax_amount": "22",
	}})
	require.ErrorContains(t, err, "year must be between 1900 and 2200")

	_, err = buildKMDHistoryImportRecord(kmdHistoryImportRow{values: map[string]string{
		"year":             "2025",
		"month":            "12",
		"row_code":         "1",
		"tax_amount":       "22",
		"total_output_vat": "bad",
	}})
	require.ErrorContains(t, err, "invalid total_output_vat")

	assert.Equal(t, "submitted_at", canonicalKMDHistoryImportHeader("submitted-date"))
	assert.Empty(t, canonicalKMDHistoryImportHeader("legacy"))
}

func TestTaxWave4ImportNilErrorSliceOnCleanImport(t *testing.T) {
	repo := &MockRepository{}
	service := NewServiceWithRepository(repo)

	result, err := service.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
		CSVContent: "year,month,row_code,tax_base,tax_amount,total_output_vat,total_input_vat\n" +
			"2025,12,1,100.00,22.00,22.00,0.00\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Nil(t, result.Errors)
	require.Len(t, repo.savedDeclarations, 1)
	assert.True(t, repo.savedDeclarations[0].TotalOutputVAT.Equal(decimal.RequireFromString("22.00")))
}
