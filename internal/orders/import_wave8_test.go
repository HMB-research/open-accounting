package orders

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importrefs"
)

func TestOrderImportWave8ParserBranches(t *testing.T) {
	_, err := parseOrderImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	service := NewServiceWithRepository(NewMockRepository())

	_, err = service.ImportCSVWithQuoteReferences(context.Background(), "tenant-1", "test_schema", nil, nil, nil, &ImportOrdersRequest{
		CSVContent: `"unterminated`,
	})
	require.ErrorContains(t, err, "parse csv header")

	rows, err := parseOrderImportRows("order_number,contact_code,order_date,line_description,quantity,unit_price,vat_rate,\n,,,,,,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestOrderImportWave8DataRowAndLookupBranches(t *testing.T) {
	base := orderImportRow{rowNumber: 2, values: map[string]string{
		"order_number":     "ORD-1",
		"contact_code":     "CUST-1",
		"order_date":       "2026-03-01",
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
		{name: "invalid order date", key: "order_date", val: "2026/03/01", want: "order_date must use YYYY-MM-DD"},
		{name: "invalid quantity", key: "quantity", val: "many", want: "invalid quantity"},
		{name: "invalid unit price", key: "unit_price", val: "free", want: "invalid unit_price"},
		{name: "invalid VAT rate", key: "vat_rate", val: "tax", want: "invalid vat_rate"},
		{name: "invalid quote id", key: "quote_id", val: "legacy-quote", want: "quote_id must be a valid UUID"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := orderImportRow{rowNumber: base.rowNumber, values: map[string]string{}}
			for key, value := range base.values {
				row.values[key] = value
			}
			row.values[tt.key] = tt.val

			_, err := parseOrderImportDataRow(row, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.want)
		})
	}

	lookup := buildOrderImportContactLookup(nil)
	_, err := lookup.find(orderImportContactRef{regCode: "12345678"})
	require.ErrorContains(t, err, `contact_reg_code "12345678" was not found`)

	lookup = buildOrderImportContactLookup([]contacts.Contact{{
		ID:      "contact-1",
		RegCode: "12345678",
	}})
	contact, err := lookup.find(orderImportContactRef{regCode: "12345678"})
	require.NoError(t, err)
	assert.Equal(t, "contact-1", contact.ID)
}
