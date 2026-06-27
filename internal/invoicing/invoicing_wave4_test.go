package invoicing

import (
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/importrefs"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoicingWave4UnpaidInvoiceStatusBranches(t *testing.T) {
	now := time.Now()

	assert.Equal(t, StatusDraft, unpaidInvoiceStatus(&Invoice{Status: StatusDraft, DueDate: now.Add(-24 * time.Hour)}))
	assert.Equal(t, StatusVoided, unpaidInvoiceStatus(&Invoice{Status: StatusVoided, DueDate: now.Add(-24 * time.Hour)}))
	assert.Equal(t, StatusOverdue, unpaidInvoiceStatus(&Invoice{Status: StatusPartiallyPaid, DueDate: now.Add(-24 * time.Hour)}))
	assert.Equal(t, StatusSent, unpaidInvoiceStatus(&Invoice{Status: StatusPartiallyPaid, DueDate: now.Add(24 * time.Hour)}))
}

func TestInvoicingWave4ImportRowsRequiredColumnEdges(t *testing.T) {
	_, err := parseInvoiceImportRows("invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,vat_rate\n" +
		"INV-1,SALES,CUST-1,2026-03-01,2026-03-15,Consulting,1,22\n")
	require.ErrorContains(t, err, "missing required unit_price column")

	_, err = parseInvoiceImportRows("invoice_number,invoice_type,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
		"INV-1,SALES,2026-03-01,2026-03-15,Consulting,1,100,22\n")
	require.ErrorContains(t, err, "missing contact identifier column")
}

func TestInvoicingWave4ImportDataRowValidAliases(t *testing.T) {
	parsed, err := parseInvoiceImportDataRow(invoiceImportRow{
		rowNumber: 2,
		values: map[string]string{
			"invoice_number":   "INV-1",
			"invoice_type":     "credit note",
			"contact_email":    "billing@example.com",
			"issue_date":       "2026-03-01",
			"due_date":         "2026-03-15",
			"currency":         " usd ",
			"exchange_rate":    "1,25",
			"status":           "partial",
			"amount_paid":      "10,50",
			"line_description": "Credit adjustment",
			"quantity":         "1",
			"unit_price":       "100",
			"discount_percent": "5",
			"vat_rate":         "22",
			"vat_treatment":    "normal",
		},
	}, newInvoiceWave4ProductLookup())

	require.NoError(t, err)
	assert.Equal(t, InvoiceTypeCreditNote, parsed.header.invoiceType)
	assert.Equal(t, "USD", parsed.header.currency)
	assert.Equal(t, StatusPartiallyPaid, parsed.header.explicitStatus)
	assert.True(t, parsed.header.amountPaid.Equal(decimal.RequireFromString("10.50")))
	assert.Equal(t, VATTreatmentStandard, parsed.line.vatTreatment)
}

func newInvoiceWave4ProductLookup() importrefs.ProductLookup {
	return importrefs.NewProductLookup(nil)
}
