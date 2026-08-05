package cutover

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalizeBundleFileCSVPassThroughAndErrors(t *testing.T) {
	original := BundleFile{
		Kind:       KindAccounts,
		FileName:   "accounts.csv",
		CSVContent: "Account Code;Name\n1000;Cash\n",
	}

	got, err := CanonicalizeBundleFileCSV(original, MigrationProviderPresetGeneric)
	require.NoError(t, err)
	assert.Equal(t, original, got)

	eInvoice := BundleFile{Kind: KindEInvoices, FileName: "invoice.xml", CSVContent: "<xml/>"}
	got, err = CanonicalizeBundleFileCSV(eInvoice, MigrationProviderPresetDirecto)
	require.NoError(t, err)
	assert.Equal(t, eInvoice, got)

	empty := BundleFile{Kind: KindAccounts, FileName: "empty.csv"}
	got, err = CanonicalizeBundleFileCSV(empty, MigrationProviderPresetDirecto)
	require.NoError(t, err)
	assert.Equal(t, empty, got)

	_, err = CanonicalizeBundleFileCSV(BundleFile{Kind: FileKind("unsupported"), CSVContent: "a\nb\n"}, MigrationProviderPresetDirecto)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported migration file kind")

	_, err = CanonicalizeBundleFileCSV(original, MigrationProviderPreset("unknown"))
	require.Error(t, err)
}

func TestCanonicalizeCSVHeadersNormalizesBOMAliasesAndRows(t *testing.T) {
	content, err := canonicalizeCSVHeaders("\ufeffSource Code;Name\n1000;Cash\n", fileSpec{
		aliases: map[string]string{
			"source_code": "code",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "code,name\n1000,Cash\n", content)

	_, err = canonicalizeCSVHeaders("", fileSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "csv_content is required")

	_, err = canonicalizeCSVHeaders("\"unterminated", fileSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse csv header")
}

func TestCanonicalizeBundleFileCSVSmartAccountsNormalizesDistinctDuplicatePayments(t *testing.T) {
	file := BundleFile{
		Kind:     KindPayments,
		FileName: "smartaccounts-payments.csv",
		CSVContent: "payment_no,payment_kind,payment_date,paid_amount,currency_code,reference_no,document_no,allocated_amount\n" +
			"PAY-7,RECEIVED,2026-06-01,100,EUR,REF-A,INV-1,100\n" +
			"PAY-7,RECEIVED,2026-06-02,50,EUR,REF-B,INV-2,50\n",
	}

	got, err := CanonicalizeBundleFileCSV(file, MigrationProviderPresetSmartAccounts)
	require.NoError(t, err)
	assert.Contains(t, got.CSVContent, "payment_number,payment_type,payment_date,amount,currency,reference,invoice_number,allocation_amount\n")
	assert.Contains(t, got.CSVContent, "PAY-7~SA01,RECEIVED,2026-06-01,100,EUR,REF-A,INV-1,100\n")
	assert.Contains(t, got.CSVContent, "PAY-7~SA02,RECEIVED,2026-06-02,50,EUR,REF-B,INV-2,50\n")
}

func TestCanonicalizeBundleFileCSVSmartAccountsKeepsGroupedPaymentDuplicates(t *testing.T) {
	file := BundleFile{
		Kind:     KindPayments,
		FileName: "smartaccounts-payments.csv",
		CSVContent: "payment_no,payment_kind,payment_date,paid_amount,currency_code,reference_no,document_no,allocated_amount\n" +
			"PAY-7,RECEIVED,2026-06-01,150,EUR,REF-A,INV-1,100\n" +
			"PAY-7,RECEIVED,2026-06-01,150,EUR,REF-A,INV-2,50\n",
	}

	got, err := CanonicalizeBundleFileCSV(file, MigrationProviderPresetSmartAccounts)
	require.NoError(t, err)
	assert.Contains(t, got.CSVContent, "PAY-7,RECEIVED,2026-06-01,150,EUR,REF-A,INV-1,100\n")
	assert.Contains(t, got.CSVContent, "PAY-7,RECEIVED,2026-06-01,150,EUR,REF-A,INV-2,50\n")
	assert.NotContains(t, got.CSVContent, "PAY-7~SA")
}

func TestCanonicalizeBundleFileCSVSmartAccountsMapsGridInvoiceNumber(t *testing.T) {
	file := BundleFile{
		Kind:       KindInvoices,
		FileName:   "smartaccounts-invoices.csv",
		CSVContent: "nr,kp,tahtaeg\nINV-1,2026-06-01,2026-06-15\n",
	}

	got, err := CanonicalizeBundleFileCSV(file, MigrationProviderPresetSmartAccounts)
	require.NoError(t, err)

	header, _, _ := strings.Cut(got.CSVContent, "\n")
	assert.Equal(t, "invoice_number,issue_date,due_date", header)
}
