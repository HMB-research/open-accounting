package camt053

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTransactionsFromOfficialLHVCAMT053Sample(t *testing.T) {
	// Fixture source: LHV Connect Account Statement "Statement data" sample.
	content, err := os.ReadFile("../lhv/testdata/account_statement_camt053_official.xml")
	require.NoError(t, err)

	assert.True(t, DetectTransactions(string(content)))
	rows, err := ParseTransactions(string(content))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "2025-06-05", rows[0].Date)
	assert.Equal(t, "2025-06-05", rows[0].ValueDate)
	assert.Equal(t, "-1", rows[0].Amount)
	assert.Equal(t, "GBP", rows[0].Currency)
	assert.Equal(t, "GB12LHVB04031312345678", rows[0].SourceAccount)
	assert.Equal(t, "GBP payment", rows[0].Description)
	assert.Equal(t, "C0924B9E44C044D39A828B7E34F4D145", rows[0].ExternalID)

	assert.Equal(t, "2025-06-05", rows[1].Date)
	assert.Equal(t, "2025-06-05", rows[1].ValueDate)
	assert.Equal(t, "-1", rows[1].Amount)
	assert.Equal(t, "EUR", rows[1].Currency)
	assert.Equal(t, "GB12LHVB04031312345679", rows[1].SourceAccount)
	assert.Equal(t, "EUR payment", rows[1].Description)
	assert.Equal(t, "7CAC8F3C708940C1AF9F0B3E4EB64478", rows[1].ExternalID)
}

func TestDetectTransactionsRecognizesPrefixedBankStatementRoot(t *testing.T) {
	content := `<doc:Document xmlns:doc="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02">
  <doc:BkToCstmrStmt>
    <doc:Stmt>
      <doc:Acct><doc:Id><doc:IBAN>EE123</doc:IBAN></doc:Id><doc:Ccy>EUR</doc:Ccy></doc:Acct>
      <doc:Ntry>
        <doc:Amt Ccy="EUR">42.00</doc:Amt>
        <doc:CdtDbtInd>CRDT</doc:CdtDbtInd>
        <doc:BookgDt><doc:Dt>2026-03-15</doc:Dt></doc:BookgDt>
        <doc:AcctSvcrRef>CAMT-1</doc:AcctSvcrRef>
      </doc:Ntry>
    </doc:Stmt>
  </doc:BkToCstmrStmt>
</doc:Document>`

	assert.True(t, DetectTransactions(content))
	rows, err := ParseTransactions(content)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-03-15", rows[0].Date)
	assert.Equal(t, "42", rows[0].Amount)
	assert.Equal(t, "EUR", rows[0].Currency)
	assert.Equal(t, "EE123", rows[0].SourceAccount)
	assert.Equal(t, "CAMT-1", rows[0].ExternalID)
}

func TestParseTransactionsRejectsMissingStatements(t *testing.T) {
	_, err := ParseTransactions(`<Document><BkToCstmrStmt></BkToCstmrStmt></Document>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains no statements")
}
