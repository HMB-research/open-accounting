package tax

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportKMDHistoryCSV_CreatesDeclaration(t *testing.T) {
	repo := &MockRepository{}
	svc := NewServiceWithRepository(repo)

	result, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
		FileName: "kmd-history.csv",
		CSVContent: `year,month,status,submitted_at,row_code,description,tax_base,tax_amount
2025,12,SUBMITTED,2026-01-20,1,Taxable sales,1000.00,220.00
2025,12,SUBMITTED,,4,Input VAT,363.64,80.00
`,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Equal(t, 2, result.RowsImported)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)

	require.Len(t, repo.savedDeclarations, 1)
	declaration := repo.savedDeclarations[0]
	assert.Equal(t, "tenant-1", declaration.TenantID)
	assert.Equal(t, 2025, declaration.Year)
	assert.Equal(t, 12, declaration.Month)
	assert.Equal(t, "SUBMITTED", declaration.Status)
	require.NotNil(t, declaration.SubmittedAt)
	assert.Equal(t, "2026-01-20", declaration.SubmittedAt.Format("2006-01-02"))
	assert.True(t, declaration.TotalOutputVAT.Equal(decimal.RequireFromString("220.00")))
	assert.True(t, declaration.TotalInputVAT.Equal(decimal.RequireFromString("80.00")))
	require.Len(t, declaration.Rows, 2)
	assert.Equal(t, KMDRow1, declaration.Rows[0].Code)
	assert.Equal(t, KMDRow4, declaration.Rows[1].Code)
}

func TestImportKMDHistoryCSV_AggregatesDuplicateRowCodesAndUsesTotals(t *testing.T) {
	repo := &MockRepository{}
	svc := NewServiceWithRepository(repo)

	result, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
		CSVContent: `year;month;row_code;tax_base;tax_amount;total_output_vat;total_input_vat
2024;11;1;100,00;22,00;30,00;0,00
2024;11;1;36,36;8,00;30,00;0,00
`,
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.DeclarationsCreated)
	assert.Equal(t, 2, result.RowsImported)

	require.Len(t, repo.savedDeclarations, 1)
	declaration := repo.savedDeclarations[0]
	assert.True(t, declaration.TotalOutputVAT.Equal(decimal.RequireFromString("30.00")))
	assert.True(t, declaration.TotalInputVAT.Equal(decimal.Zero))
	require.Len(t, declaration.Rows, 1)
	assert.True(t, declaration.Rows[0].TaxBase.Equal(decimal.RequireFromString("136.36")))
	assert.True(t, declaration.Rows[0].TaxAmount.Equal(decimal.RequireFromString("30.00")))
}

func TestImportKMDHistoryCSV_RejectsVATReconciliationMismatch(t *testing.T) {
	tests := []struct {
		name         string
		csvContent   string
		errorSnippet string
	}{
		{
			name: "declared output total mismatch",
			csvContent: `year,month,row_code,tax_base,tax_amount,total_output_vat,total_input_vat
2024,11,1,100.00,22.00,30.00,0.00
2024,11,2,50.00,11.00,30.00,0.00
`,
			errorSnippet: "total_output_vat 30 does not match supporting KMD output VAT rows",
		},
		{
			name: "row 8 output total mismatch",
			csvContent: `year,month,row_code,tax_base,tax_amount
2024,11,1,100.00,22.00
2024,11,2,50.00,11.00
2024,11,8,0.00,30.00
`,
			errorSnippet: "KMD row 8 tax_amount 30 does not match supporting KMD output VAT rows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockRepository{}
			svc := NewServiceWithRepository(repo)

			result, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
				CSVContent: tt.csvContent,
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Zero(t, result.DeclarationsCreated)
			assert.Zero(t, result.RowsImported)
			assert.NotZero(t, result.RowsSkipped)
			require.NotEmpty(t, result.Errors)
			assert.Contains(t, result.Errors[0].Message, tt.errorSnippet)
			assert.Empty(t, repo.savedDeclarations)
		})
	}
}

func TestImportKMDHistoryCSV_SkipsInvalidAndExistingRows(t *testing.T) {
	repo := &MockRepository{
		existingDeclarations: map[string]*KMDDeclaration{
			"2025-12": {ID: "existing", Year: 2025, Month: 12},
		},
	}
	svc := NewServiceWithRepository(repo)

	result, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
		CSVContent: `year,month,row_code,tax_base,tax_amount
2025,12,1,1000.00,220.00
2026,13,1,1000.00,220.00
2026,1,,1000.00,220.00
`,
	})

	require.NoError(t, err)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Zero(t, result.DeclarationsCreated)
	assert.Zero(t, result.RowsImported)
	assert.Equal(t, 3, result.RowsSkipped)
	require.Len(t, result.Errors, 3)
	assert.Contains(t, result.Errors[0].Message, "month must be between 1 and 12")
	assert.Contains(t, result.Errors[1].Message, "row_code is required")
	assert.Contains(t, result.Errors[2].Message, "already exists")
	assert.Empty(t, repo.savedDeclarations)
}

func TestImportKMDHistoryCSV_RejectsMissingHeaders(t *testing.T) {
	svc := NewServiceWithRepository(&MockRepository{})

	_, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
		CSVContent: "year,month,tax_base,tax_amount\n2025,12,1000.00,220.00\n",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required year, month, or row_code column")
}
