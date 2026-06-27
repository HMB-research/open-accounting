package quotes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/importrefs"
)

func TestQuoteImportWave8ServiceAndParserBranches(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())

	_, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportQuotesRequest{
		CSVContent: "quote_number,contact_code,quote_date,line_description,quantity,unit_price,vat_rate\n",
	})
	require.ErrorContains(t, err, "no quotes found in CSV")

	_, err = parseQuoteImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseQuoteImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	rows, err := parseQuoteImportRows("quote_number,contact_code,quote_date,line_description,quantity,unit_price,vat_rate,\n,,,,,,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	repo := NewMockRepository()
	repo.ListErr = assert.AnError
	service = NewServiceWithRepository(repo)

	_, err = service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportQuotesRequest{
		CSVContent: "quote_number,contact_code,quote_date,line_description,quantity,unit_price,vat_rate\n" +
			"Q-1,CUST-1,2026-03-01,Work,1,100,22\n",
	})
	require.ErrorContains(t, err, "list existing quotes")
}

func TestQuoteImportWave8DataRowAndMergeBranches(t *testing.T) {
	base := quoteImportRow{rowNumber: 2, values: map[string]string{
		"quote_number":     "Q-1",
		"contact_code":     "CUST-1",
		"quote_date":       "2026-03-01",
		"line_description": "Work",
		"quantity":         "1",
		"unit_price":       "100",
		"vat_rate":         "22",
	}}

	for _, tt := range []struct {
		name string
		key  string
		val  string
		want string
	}{
		{name: "invalid contact id", key: "contact_id", val: "legacy-contact", want: "contact_id must be a valid UUID"},
		{name: "invalid quote date", key: "quote_date", val: "2026/03/01", want: "quote_date must use YYYY-MM-DD"},
		{name: "invalid quantity", key: "quantity", val: "many", want: "invalid quantity"},
		{name: "invalid unit price", key: "unit_price", val: "free", want: "invalid unit_price"},
		{name: "invalid VAT rate", key: "vat_rate", val: "tax", want: "invalid vat_rate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := quoteImportRow{rowNumber: base.rowNumber, values: map[string]string{}}
			for key, value := range base.values {
				row.values[key] = value
			}
			row.values[tt.key] = tt.val

			_, err := parseQuoteImportDataRow(row, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.want)
		})
	}

	quoteDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	group := &quoteImportGroup{header: quoteImportHeader{id: "quote-a", quoteDate: quoteDate}}
	conflict := mergeQuoteImportGroup(group, quoteImportHeader{id: "quote-b", quoteDate: quoteDate}, 3)
	require.Contains(t, conflict, "id must be consistent")

	lookup := buildQuoteImportContactLookup(nil)
	_, err := lookup.find(quoteImportContactRef{regCode: "12345678"})
	require.ErrorContains(t, err, `contact_reg_code "12345678" was not found`)
}
