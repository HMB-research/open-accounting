package registry

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTransactionsAutoDetectsGenericCSV(t *testing.T) {
	rows, err := ParseTransactions("date,amount,description\n2026-03-15,42.00,Generic payment\n", "auto")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-03-15", rows[0].Date)
	assert.Equal(t, "42.00", rows[0].Amount)
	assert.Equal(t, "Generic payment", rows[0].Description)
}

func TestParseTransactionsAutoDetectsLHVCSV(t *testing.T) {
	content := "Client account;Document number;Date;Beneficiary's/remitter's account;Beneficiary's/remitter's name;Debit/Credit (D/C);Amount;Reference number;Archival ID;Details;Currency;Personal identification code or registry code;Beneficiary's/remitter's bank's BIC;Payment initiator's name;Entry reference;Account service provider's reference\n" +
		"EE457700771000676899;123;2026-03-15;EE867700771000681884;Test Client;D;12,50;100513845;202603150001;EUR payment;EUR;12345678;LHVBEE22;;ENTRY-1;LHV-UNIQUE-1\n"

	rows, err := ParseTransactions(content, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "-12.5", rows[0].Amount)
	assert.Equal(t, "LHV-UNIQUE-1", rows[0].ExternalID)
}

func TestParseTransactionsRoutesExplicitFormats(t *testing.T) {
	t.Run("generic", func(t *testing.T) {
		rows, err := ParseTransactions("date;amount;description\n2026-03-15;42.00;Generic payment\n", "GENERIC")
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "Generic payment", rows[0].Description)
	})

	t.Run("lhv csv", func(t *testing.T) {
		content := "Client account;Document number;Date;Beneficiary's/remitter's account;Beneficiary's/remitter's name;Debit/Credit (D/C);Amount;Reference number;Archival ID;Details;Currency;Personal identification code or registry code;Beneficiary's/remitter's bank's BIC;Payment initiator's name;Entry reference;Account service provider's reference\n" +
			"EE457700771000676899;123;2026-03-15;EE867700771000681884;Test Client;C;12,50;100513845;202603150001;EUR payment;EUR;12345678;LHVBEE22;;ENTRY-1;LHV-UNIQUE-1\n"
		rows, err := ParseTransactions(content, " lhv ")
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "12.5", rows[0].Amount)
	})

	t.Run("lhv camt", func(t *testing.T) {
		content, err := os.ReadFile("../lhv/testdata/account_statement_camt053_official.xml")
		require.NoError(t, err)
		rows, err := ParseTransactions(string(content), "LHV-CAMT")
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Equal(t, "GB12LHVB04031312345678", rows[0].SourceAccount)
	})
}

func TestParseTransactionsRejectsUnsupportedFormat(t *testing.T) {
	_, err := ParseTransactions("date,amount,description\n2026-03-15,42.00,Generic payment\n", "other-bank")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported bank transaction import format "other-bank"`)
}
