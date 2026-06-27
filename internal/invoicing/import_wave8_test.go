package invoicing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importrefs"
)

func TestInvoiceImportWave8ParserBranches(t *testing.T) {
	_, err := parseInvoiceImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	rows, err := parseInvoiceImportRows("invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,\n,,,,,,,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	service := NewServiceWithRepository(NewMockRepository(), nil)
	_, err = service.ImportCSV(context.Background(), "tenant-1", "test_schema", nil, nil, &ImportInvoicesRequest{
		CSVContent: `"unterminated`,
	}, nil)
	require.ErrorContains(t, err, "parse csv header")
}

func TestInvoiceImportWave8BuildImportedInvoiceErrorsBecomeRowErrors(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	result, err := service.ImportCSV(context.Background(), "tenant-1", "test_schema", []contacts.Contact{{
		ID:   "contact-1",
		Code: "CUST-1",
	}}, nil, &ImportInvoicesRequest{
		CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,amount_paid\n" +
			"INV-PAID-TOO-MUCH,SALES,CUST-1,2026-03-01,2026-03-15,Work,1,100,0,150\n",
	}, nil)

	require.NoError(t, err)
	assert.Zero(t, result.InvoicesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "amount_paid cannot exceed invoice total")
	assert.Empty(t, repo.invoices)
}

func TestInvoiceImportWave8DataRowParseErrors(t *testing.T) {
	base := invoiceImportRow{rowNumber: 2, values: map[string]string{
		"invoice_number":   "INV-1",
		"invoice_type":     "SALES",
		"contact_code":     "CUST-1",
		"issue_date":       "2026-03-01",
		"due_date":         "2026-03-15",
		"status":           "DRAFT",
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
		{name: "invalid invoice type", key: "invoice_type", val: "memo", want: "invalid invoice_type"},
		{name: "invalid status", key: "status", val: "settled", want: "invalid status"},
		{name: "invalid quantity", key: "quantity", val: "many", want: "invalid quantity"},
		{name: "invalid unit price", key: "unit_price", val: "free", want: "invalid unit_price"},
		{name: "invalid VAT rate", key: "vat_rate", val: "tax", want: "invalid vat_rate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := invoiceImportRow{rowNumber: base.rowNumber, values: map[string]string{}}
			for key, value := range base.values {
				row.values[key] = value
			}
			row.values[tt.key] = tt.val

			_, err := parseInvoiceImportDataRow(row, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.want)
		})
	}
}
