package cutover

import (
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
