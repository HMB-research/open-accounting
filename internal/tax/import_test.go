package tax

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestImportKMDHistoryCSV_ErrorBranches(t *testing.T) {
	t.Run("requires content and data rows", func(t *testing.T) {
		svc := NewServiceWithRepository(&MockRepository{})

		_, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{CSVContent: " \n\t "})
		require.EqualError(t, err, "csv_content is required")

		_, err = svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
			CSVContent: "year,month,row_code,tax_base,tax_amount\n",
		})
		require.EqualError(t, err, "no KMD rows found in CSV")
	})

	t.Run("skips inconsistent rows while importing the valid group records", func(t *testing.T) {
		repo := &MockRepository{}
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
			CSVContent: `year,month,status,row_code,tax_base,tax_amount
2025,12,ACCEPTED,1,100.00,22.00
2025,12,DRAFT,4,50.00,11.00
`,
		})

		require.NoError(t, err)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.DeclarationsCreated)
		assert.Equal(t, 1, result.RowsImported)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2025, result.Errors[0].Year)
		assert.Equal(t, 12, result.Errors[0].Month)
		assert.Equal(t, KMDRow4, result.Errors[0].RowCode)
		assert.Contains(t, result.Errors[0].Message, "status must match other rows")
		require.Len(t, repo.savedDeclarations, 1)
		require.Len(t, repo.savedDeclarations[0].Rows, 1)
	})

	t.Run("returns get declaration repository errors", func(t *testing.T) {
		svc := NewServiceWithRepository(&MockRepository{getDeclarationErr: errors.New("read failed")})

		_, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
			CSVContent: `year,month,row_code,tax_base,tax_amount
2025,12,1,100.00,22.00
`,
		})

		require.ErrorContains(t, err, "check existing KMD declaration for 2025-12: read failed")
	})

	t.Run("records save declaration errors on each group row", func(t *testing.T) {
		repo := &MockRepository{saveDeclarationErr: errors.New("write failed")}
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
			CSVContent: `year,month,row_code,tax_base,tax_amount
2025,12,1,100.00,22.00
2025,12,4,50.00,11.00
`,
		})

		require.NoError(t, err)
		assert.Zero(t, result.DeclarationsCreated)
		assert.Zero(t, result.RowsImported)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 2)
		assert.Contains(t, result.Errors[0].Message, "save KMD declaration: write failed")
		assert.Empty(t, repo.savedDeclarations)
	})
}

func TestImportKMDHistoryCSV_RejectsMissingHeaders(t *testing.T) {
	svc := NewServiceWithRepository(&MockRepository{})

	_, err := svc.ImportKMDHistoryCSV(context.Background(), "tenant_schema", "tenant-1", &ImportKMDHistoryRequest{
		CSVContent: "year,month,tax_base,tax_amount\n2025,12,1000.00,220.00\n",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required year, month, or row_code column")
}

func TestKMDHistoryImportGroupConsistencyBranches(t *testing.T) {
	submittedAt := time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC)
	otherSubmittedAt := submittedAt.AddDate(0, 0, 1)
	outputTotal := decimal.NewFromInt(220)
	otherOutputTotal := decimal.NewFromInt(221)
	inputTotal := decimal.NewFromInt(55)
	otherInputTotal := decimal.NewFromInt(56)

	tests := []struct {
		name       string
		group      *kmdHistoryImportGroup
		record     *kmdHistoryImportRecord
		want       string
		assertions func(t *testing.T, group *kmdHistoryImportGroup)
	}{
		{
			name: "status mismatch",
			group: &kmdHistoryImportGroup{
				year:   2026,
				month:  1,
				status: KMDStatusDraft,
			},
			record: &kmdHistoryImportRecord{status: KMDStatusSubmitted},
			want:   "status must match other rows",
		},
		{
			name: "submitted date mismatch",
			group: &kmdHistoryImportGroup{
				year:        2026,
				month:       1,
				status:      KMDStatusSubmitted,
				submittedAt: &submittedAt,
			},
			record: &kmdHistoryImportRecord{status: KMDStatusSubmitted, submittedAt: &otherSubmittedAt},
			want:   "submitted_at must match other rows",
		},
		{
			name: "output total mismatch",
			group: &kmdHistoryImportGroup{
				year:           2026,
				month:          1,
				status:         KMDStatusAccepted,
				totalOutputVAT: &outputTotal,
			},
			record: &kmdHistoryImportRecord{status: KMDStatusAccepted, totalOutputVAT: &otherOutputTotal},
			want:   "total_output_vat must match other rows",
		},
		{
			name: "input total mismatch",
			group: &kmdHistoryImportGroup{
				year:          2026,
				month:         1,
				status:        KMDStatusAccepted,
				totalInputVAT: &inputTotal,
			},
			record: &kmdHistoryImportRecord{status: KMDStatusAccepted, totalInputVAT: &otherInputTotal},
			want:   "total_input_vat must match other rows",
		},
		{
			name: "adopts missing metadata",
			group: &kmdHistoryImportGroup{
				year:   2026,
				month:  1,
				status: KMDStatusSubmitted,
			},
			record: &kmdHistoryImportRecord{
				status:         KMDStatusSubmitted,
				submittedAt:    &submittedAt,
				totalOutputVAT: &outputTotal,
				totalInputVAT:  &inputTotal,
			},
			assertions: func(t *testing.T, group *kmdHistoryImportGroup) {
				t.Helper()
				require.NotNil(t, group.submittedAt)
				assert.True(t, group.submittedAt.Equal(submittedAt))
				require.NotNil(t, group.totalOutputVAT)
				assert.True(t, group.totalOutputVAT.Equal(outputTotal))
				require.NotNil(t, group.totalInputVAT)
				assert.True(t, group.totalInputVAT.Equal(inputTotal))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateKMDHistoryGroupConsistency(tt.group, tt.record)

			if tt.want != "" {
				assert.Contains(t, got, tt.want)
				return
			}
			assert.Empty(t, got)
			tt.assertions(t, tt.group)
		})
	}
}

func TestKMDHistoryVATReconciliationBranches(t *testing.T) {
	outputTotal := decimal.NewFromInt(219)
	inputTotal := decimal.NewFromInt(54)

	tests := []struct {
		name  string
		group *kmdHistoryImportGroup
		want  string
	}{
		{
			name: "output total row mismatch without support rows",
			group: &kmdHistoryImportGroup{
				year:           2026,
				month:          1,
				totalOutputVAT: &outputTotal,
				records: []*kmdHistoryImportRecord{
					kmdHistoryRecord(KMDRow8, 220),
				},
			},
			want: "does not match KMD row 8 tax_amount",
		},
		{
			name: "input total row mismatch against support rows",
			group: &kmdHistoryImportGroup{
				year:  2026,
				month: 1,
				records: []*kmdHistoryImportRecord{
					kmdHistoryRecord(KMDRow4, 55),
					kmdHistoryRecord(KMDRow9, 54),
				},
			},
			want: "KMD row 9 tax_amount 54 does not match supporting KMD input VAT rows",
		},
		{
			name: "input declared total mismatch against support rows",
			group: &kmdHistoryImportGroup{
				year:          2026,
				month:         1,
				totalInputVAT: &inputTotal,
				records: []*kmdHistoryImportRecord{
					kmdHistoryRecord(KMDRow4, 55),
				},
			},
			want: "total_input_vat 54 does not match supporting KMD input VAT rows",
		},
		{
			name: "input total row mismatch without support rows",
			group: &kmdHistoryImportGroup{
				year:          2026,
				month:         1,
				totalInputVAT: &inputTotal,
				records: []*kmdHistoryImportRecord{
					kmdHistoryRecord(KMDRow9, 55),
				},
			},
			want: "does not match KMD row 9 tax_amount",
		},
		{
			name: "matching output and input support",
			group: &kmdHistoryImportGroup{
				year:  2026,
				month: 1,
				records: []*kmdHistoryImportRecord{
					kmdHistoryRecord(KMDRow1, 220),
					kmdHistoryRecord(KMDRow8, 220),
					kmdHistoryRecord(KMDRow4, 55),
					kmdHistoryRecord(KMDRow9, 55),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateKMDHistoryVATReconciliation(tt.group)

			if tt.want != "" {
				assert.Contains(t, got, tt.want)
				return
			}
			assert.Empty(t, got)
		})
	}
}

func TestKMDHistoryImportHelperBranches(t *testing.T) {
	_, err := parseKMDHistoryImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseKMDHistoryImportRows("year,month,row_code,tax_base,tax_amount\n2025,12,1,\"100.00\n")
	require.ErrorContains(t, err, "parse csv row 2")

	parsed, err := parseOptionalKMDHistoryDecimal(" 1 234,56 ", "tax_base")
	require.NoError(t, err)
	assert.True(t, parsed.Equal(decimal.RequireFromString("1234.56")))

	blank, err := parseOptionalKMDHistoryDecimal(" ", "tax_amount")
	require.NoError(t, err)
	assert.True(t, blank.IsZero())

	_, err = parseOptionalKMDHistoryDecimal("not-a-number", "tax_amount")
	require.ErrorContains(t, err, "invalid tax_amount")

	pointer, err := parseOptionalKMDHistoryDecimalPointer(" 55,50 ", "total_input_vat")
	require.NoError(t, err)
	require.NotNil(t, pointer)
	assert.True(t, pointer.Equal(decimal.RequireFromString("55.50")))

	_, err = parseOptionalKMDHistoryDecimalPointer("bad", "total_input_vat")
	require.ErrorContains(t, err, "invalid total_input_vat")

	status, err := parseKMDHistoryStatus("filed")
	require.NoError(t, err)
	assert.Equal(t, KMDStatusSubmitted, status)

	_, err = parseKMDHistoryStatus("unknown")
	require.ErrorContains(t, err, "invalid status")

	year, err := parseKMDHistoryImportYear(" 2025 ")
	require.NoError(t, err)
	assert.Equal(t, 2025, year)

	_, err = parseKMDHistoryImportYear("1899")
	require.ErrorContains(t, err, "year must be between 1900 and 2200")

	_, err = parseKMDHistoryImportYear("2201")
	require.ErrorContains(t, err, "year must be between 1900 and 2200")

	assert.Equal(t, "output", kmdHistoryVATSupportClass(KMDRow21))
	assert.Equal(t, "output", kmdHistoryVATSupportClass(KMDRow31))
	assert.Equal(t, "input", kmdHistoryVATSupportClass(KMDRow6))
	assert.Equal(t, "input", kmdHistoryVATSupportClass(KMDRow7))
	assert.Equal(t, "output_total", kmdHistoryVATSupportClass(KMDRow8))
	assert.Equal(t, "input_total", kmdHistoryVATSupportClass(KMDRow9))
	assert.Empty(t, kmdHistoryVATSupportClass("99"))

	assert.Equal(t, "001", kmdHistoryRowSortKey("1"))
	assert.Equal(t, "021", kmdHistoryRowSortKey("21"))
	assert.Equal(t, "A", kmdHistoryRowSortKey("A"))

	record, err := buildKMDHistoryImportRecord(kmdHistoryImportRow{
		rowNumber: 2,
		values: map[string]string{
			"year":         "2025",
			"month":        "12",
			"status":       "accepted",
			"submitted_at": "20.01.2026",
			"row_code":     "row_1",
			"description":  "Custom row",
			"tax_base":     "100",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2025, record.year)
	assert.Equal(t, 12, record.month)
	assert.Equal(t, KMDStatusAccepted, record.status)
	require.NotNil(t, record.submittedAt)
	assert.Equal(t, KMDRow1, record.row.Code)
	assert.Equal(t, "Custom row", record.row.Description)

	_, err = buildKMDHistoryImportRecord(kmdHistoryImportRow{
		values: map[string]string{
			"year":         "2025",
			"month":        "12",
			"submitted_at": "2026/01/20",
			"row_code":     "1",
			"tax_base":     "100",
		},
	})
	require.ErrorContains(t, err, "submitted_at must be in YYYY-MM-DD format")

	_, err = buildKMDHistoryImportRecord(kmdHistoryImportRow{
		values: map[string]string{
			"year":     "2025",
			"month":    "12",
			"row_code": "1",
		},
	})
	require.ErrorContains(t, err, "tax_base or tax_amount is required")
}

func kmdHistoryRecord(code string, amount int64) *kmdHistoryImportRecord {
	return &kmdHistoryImportRecord{
		row: KMDRow{
			Code:      code,
			TaxAmount: decimal.NewFromInt(amount),
		},
	}
}
