package tax

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKMDHistoryImportWave8ParserBranches(t *testing.T) {
	_, err := parseKMDHistoryImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	rows, err := parseKMDHistoryImportRows("year,month,row_code,\n,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	_, err = buildKMDHistoryImportRecord(kmdHistoryImportRow{rowNumber: 2, values: map[string]string{
		"year":       "2026",
		"month":      "5",
		"row_code":   "1",
		"tax_base":   "",
		"tax_amount": "",
	}})
	require.ErrorContains(t, err, "tax_base or tax_amount is required")

	base := kmdHistoryImportRow{rowNumber: 2, values: map[string]string{
		"year":       "2026",
		"month":      "5",
		"row_code":   "1",
		"status":     "DRAFT",
		"tax_base":   "100",
		"tax_amount": "22",
	}}
	for _, tt := range []struct {
		name string
		key  string
		val  string
		want string
	}{
		{name: "invalid status", key: "status", val: "sent", want: "invalid status"},
		{name: "invalid tax base", key: "tax_base", val: "base", want: "invalid tax_base"},
		{name: "invalid tax amount", key: "tax_amount", val: "vat", want: "invalid tax_amount"},
		{name: "invalid total output VAT", key: "total_output_vat", val: "output", want: "invalid total_output_vat"},
		{name: "invalid total input VAT", key: "total_input_vat", val: "input", want: "invalid total_input_vat"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := kmdHistoryImportRow{rowNumber: base.rowNumber, values: map[string]string{}}
			for key, value := range base.values {
				row.values[key] = value
			}
			row.values[tt.key] = tt.val

			_, err := buildKMDHistoryImportRecord(row)

			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestKMDHistoryImportWave8DeclarationDescriptionReplacement(t *testing.T) {
	group := &kmdHistoryImportGroup{
		year:   2026,
		month:  5,
		status: KMDStatusDraft,
		records: []*kmdHistoryImportRecord{
			{row: KMDRow{Code: KMDRow1, Description: "Unknown", TaxBase: decimal.NewFromInt(10), TaxAmount: decimal.NewFromInt(2)}},
			{row: KMDRow{Code: KMDRow1, Description: "Taxable turnover", TaxBase: decimal.NewFromInt(20), TaxAmount: decimal.NewFromInt(4)}},
		},
	}

	declaration := buildKMDHistoryDeclaration("tenant-1", group)

	require.Len(t, declaration.Rows, 1)
	assert.Equal(t, "Taxable turnover", declaration.Rows[0].Description)
	assert.True(t, declaration.Rows[0].TaxBase.Equal(decimal.NewFromInt(30)))
	assert.True(t, declaration.Rows[0].TaxAmount.Equal(decimal.NewFromInt(6)))
}

func TestKMDHistoryImportWave8DeclarationUsesTotalRows(t *testing.T) {
	group := &kmdHistoryImportGroup{
		year:   2026,
		month:  5,
		status: KMDStatusDraft,
		records: []*kmdHistoryImportRecord{
			{row: KMDRow{Code: KMDRow1, Description: "Sales", TaxBase: decimal.NewFromInt(100), TaxAmount: decimal.NewFromInt(22)}},
			{row: KMDRow{Code: KMDRow4, Description: "Input VAT", TaxAmount: decimal.NewFromInt(6)}},
			{row: KMDRow{Code: KMDRow8, Description: "Total output VAT", TaxAmount: decimal.NewFromInt(33)}},
			{row: KMDRow{Code: KMDRow9, Description: "Total input VAT", TaxAmount: decimal.NewFromInt(7)}},
		},
	}

	declaration := buildKMDHistoryDeclaration("tenant-1", group)

	assert.True(t, declaration.TotalOutputVAT.Equal(decimal.NewFromInt(33)))
	assert.True(t, declaration.TotalInputVAT.Equal(decimal.NewFromInt(7)))
}
