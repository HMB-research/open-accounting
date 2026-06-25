package recurring

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importrefs"
)

func TestRecurringImportWave8RepositoryGuardAndParserBranches(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)

	_, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportRecurringInvoicesRequest{
		CSVContent: "name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\n" +
			"Monthly,CUST-1,MONTHLY,2026-03-01,Work,1,100,22\n",
	})
	require.ErrorContains(t, err, "repository not available")

	service = NewServiceWithDependencies(NewMockRepository(), nil, nil, nil, nil, nil)
	_, err = service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportRecurringInvoicesRequest{
		CSVContent: `"unterminated`,
	})
	require.ErrorContains(t, err, "parse csv header")

	rows, err := parseRecurringImportRows("name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate,\n,,,,,,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestRecurringImportWave8LineAndLookupBranches(t *testing.T) {
	base := recurringImportRow{rowNumber: 2, values: map[string]string{
		"name":             "Monthly",
		"contact_code":     "CUST-1",
		"frequency":        "MONTHLY",
		"start_date":       "2026-03-01",
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
		{name: "invalid quantity", key: "quantity", val: "many", want: "invalid quantity"},
		{name: "invalid unit price", key: "unit_price", val: "free", want: "invalid unit_price"},
		{name: "invalid VAT rate", key: "vat_rate", val: "tax", want: "invalid vat_rate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := recurringImportRow{rowNumber: base.rowNumber, values: map[string]string{}}
			for key, value := range base.values {
				row.values[key] = value
			}
			row.values[tt.key] = tt.val

			_, err := parseRecurringImportLine(row, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.want)
		})
	}

	lookup := buildRecurringImportContactLookup(nil)
	_, err := lookup.find(recurringImportContactRef{regCode: "12345678"})
	require.ErrorContains(t, err, `contact_reg_code "12345678" was not found`)

	lookup = buildRecurringImportContactLookup([]contacts.Contact{{
		ID:      "contact-1",
		RegCode: "12345678",
	}})
	contact, err := lookup.find(recurringImportContactRef{regCode: "12345678"})
	require.NoError(t, err)
	assert.Equal(t, "contact-1", contact.ID)
}

func TestRecurringImportWave8LastGeneratedAt(t *testing.T) {
	row := recurringImportRow{rowNumber: 2, values: map[string]string{
		"name":              "Monthly",
		"contact_code":      "CUST-1",
		"frequency":         "MONTHLY",
		"start_date":        "2026-03-01",
		"last_generated_at": "2026-03-31",
		"line_description":  "Work",
		"quantity":          "1",
		"unit_price":        "100",
		"vat_rate":          "22",
	}}

	parsed, err := parseRecurringImportDataRow(row, importrefs.NewProductLookup(nil))

	require.NoError(t, err)
	require.NotNil(t, parsed.header.lastGeneratedAt)
	assert.Equal(t, "2026-03-31", parsed.header.lastGeneratedAt.Format("2006-01-02"))
}
