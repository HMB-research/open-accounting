package cutover

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCutoverWave8CrossFileSkipBranches(t *testing.T) {
	invoiceFile := wave6ParsedFile(KindInvoices, "invoices.csv", []string{"invoice_number", "invoice_type", "issue_date", "contact_code", "quantity", "unit_price", "vat_rate"},
		map[string]string{"invoice_number": "INV-1", "invoice_type": "SALES", "issue_date": "2026-05-10", "contact_code": "CUST-1", "quantity": "1", "unit_price": "100", "vat_rate": "0"},
	)
	paymentsFile := wave6ParsedFile(KindPayments, "payments.csv", []string{"invoice_number", "amount"},
		map[string]string{"amount": "10"},
		map[string]string{"invoice_number": "INV-1", "amount": "bad"},
	)

	report := &BundleValidationReport{}
	validateCrossFileConsistency(report, []parsedFile{invoiceFile, paymentsFile}, EInvoiceContactModeBoth)
	assert.Empty(t, report.Issues)

	kmdFile := wave6ParsedFile(KindKMDHistory, "kmd.csv", []string{"year", "month", "row_code", "tax_amount"},
		map[string]string{"year": "bad", "month": "5", "row_code": "1", "tax_amount": "22"},
	)
	validateKMDHistoryVATReconciliation(report, []parsedFile{kmdFile}, nil)
	assert.Empty(t, report.Issues)

	targets := buildCutoverInvoiceAllocationTargets([]parsedFile{
		wave6ParsedFile(KindEInvoices, "einvoices.csv", []string{"invoice_number", "invoice_total"},
			map[string]string{"invoice_number": "EINV-1", "invoice_total": "bad"},
		),
	}, EInvoiceContactModeBoth)
	assert.Empty(t, targets)
}

func TestCutoverWave8HelperBranches(t *testing.T) {
	lineID := "11111111-1111-4111-8111-111111111111"
	targets := buildCutoverJournalLineAmountTargets([]parsedFile{
		wave6ParsedFile(KindJournalEntries, "journal.csv", []string{"line_id", "debit", "credit"},
			map[string]string{"line_id": lineID, "debit": "0", "credit": "0"},
		),
	})
	assert.Empty(t, targets)

	accountType, ok := cutoverNormalizedAccountType("ASSET")
	require.True(t, ok)
	assert.Equal(t, "ASSET", accountType)

	report := &BundleValidationReport{}
	file := wave6ParsedFile(KindInvoices, "commercial.csv", []string{"status"}, map[string]string{"status": "custom"})
	checkCommercialStatus(report, file, file.rows[0], "status", normalizeCutoverUpper)
	assert.Empty(t, report.Issues)

	assert.False(t, hasPaymentInvoiceTypeMismatch(report, wave6ParsedFile(KindPayments, "payments.csv", []string{"payment_type"}, nil), parsedRow{
		number: 2,
		values: map[string]string{"payment_type": "REFUND"},
	}, cutoverInvoiceAllocationTarget{display: "INV-1", invoiceType: "SALES", targetKind: KindInvoices}))

	validateImportedInvoiceAmountPaidConsistency(report, []parsedFile{
		wave6ParsedFile(KindInvoices, "invoices.csv", []string{"invoice_number", "amount_paid", "quantity", "unit_price", "vat_rate"},
			map[string]string{"invoice_number": "INV-1", "amount_paid": "0", "quantity": "1", "unit_price": "100", "vat_rate": "0"},
		),
	})
	assert.Empty(t, report.Issues)

	invoiceID := "22222222-2222-4222-8222-222222222222"
	fixedAssetFile := wave6ParsedFile(KindFixedAssets, "assets.csv", []string{"invoice_id", "purchase_date", "purchase_cost"},
		map[string]string{"invoice_id": invoiceID, "purchase_date": "2026-05-11", "purchase_cost": "bad"},
	)
	validateFixedAssetInvoiceConsistency(report, []parsedFile{fixedAssetFile}, map[string]cutoverInvoiceAllocationTarget{
		cutoverInvoiceAllocationTargetKey("invoice_id", invoiceID): {
			key:                cutoverInvoiceAllocationTargetKey("invoice_id", invoiceID),
			display:            "INV-1",
			total:              decimal.NewFromInt(100),
			invoiceType:        "PURCHASE",
			issueDate:          time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
			issueDateSpecified: true,
			targetKind:         KindInvoices,
		},
	})
	assert.Empty(t, report.Issues)
}
