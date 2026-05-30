package generic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTransactions(t *testing.T) {
	rows, err := ParseTransactions("date;amount;description;reference;counterparty_name;counterparty_account;external_id\n2026-03-15;100.00;Client payment;REF-1;Acme;EE111;ext-1\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-03-15", rows[0].Date)
	assert.Equal(t, "100.00", rows[0].Amount)
	assert.Equal(t, "Client payment", rows[0].Description)
	assert.Equal(t, "REF-1", rows[0].Reference)
	assert.Equal(t, "Acme", rows[0].CounterpartyName)
	assert.Equal(t, "EE111", rows[0].CounterpartyAccount)
	assert.Equal(t, "ext-1", rows[0].ExternalID)
}

func TestParseTransactionsRequiresCoreFields(t *testing.T) {
	_, err := ParseTransactions("date;amount\n2026-03-15;100.00\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires date, amount, and description")
}
