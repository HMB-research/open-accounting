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

func TestDetectTransactionsRejectsNonCAMTContent(t *testing.T) {
	assert.False(t, DetectTransactions("date,amount\n2026-03-15,10"))
	assert.False(t, DetectTransactions("<Document><Other></Other></Document>"))
}

func TestParseTransactionsRejectsMalformedAndEmptyStatements(t *testing.T) {
	_, err := ParseTransactions(`<Document>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse camt.053 XML")

	_, err = ParseTransactions(`<Document><BkToCstmrStmt><Stmt><Acct><Id><IBAN>EE123</IBAN></Id></Acct></Stmt></BkToCstmrStmt></Document>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains no transactions")
}

func TestRowsFromEntryValidationBranches(t *testing.T) {
	statement := camtStatement{Account: camtStatementAccount{ID: camtAccountID{IBAN: "EE123"}, Currency: "EUR"}}

	_, err := rowsFromEntry(statement, camtEntry{
		Amount:               camtAmount{Value: "1.00", Currency: "EUR"},
		CreditDebitIndicator: "CRDT",
		BookingDate:          camtDateChoice{Date: "bad-date"},
	}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date")

	_, err = rowsFromEntry(statement, camtEntry{
		Amount:               camtAmount{Value: "not-number", Currency: "EUR"},
		CreditDebitIndicator: "CRDT",
		BookingDate:          camtDateChoice{Date: "2026-03-15"},
	}, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")

	_, err = rowsFromEntry(statement, camtEntry{
		Amount:      camtAmount{Value: "1.00", Currency: "EUR"},
		BookingDate: camtDateChoice{},
	}, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires date and amount")
}

func TestCAMTNormalizationBranches(t *testing.T) {
	name, account := counterparty("CRDT", camtRelatedParties{
		Debtor:        camtParty{Name: "Debtor"},
		DebtorAccount: camtPartyAccount{ID: camtAccountID{IBAN: "EE-DEBTOR"}},
	})
	assert.Equal(t, "Debtor", name)
	assert.Equal(t, "EE-DEBTOR", account)

	name, account = counterparty("unknown", camtRelatedParties{
		Creditor:        camtParty{Name: "Creditor"},
		CreditorAccount: camtPartyAccount{ID: camtAccountID{IBAN: "EE-CREDITOR"}},
		Debtor:          camtParty{Name: "Debtor"},
		DebtorAccount:   camtPartyAccount{ID: camtAccountID{IBAN: "EE-DEBTOR"}},
	})
	assert.Equal(t, "Creditor", name)
	assert.Equal(t, "EE-CREDITOR", account)

	amount, err := normalizeAmount("", "CRDT")
	require.NoError(t, err)
	assert.Equal(t, "", amount)

	amount, err = normalizeAmount("1 234,56", "DBIT")
	require.NoError(t, err)
	assert.Equal(t, "-1234.56", amount)

	amount, err = normalizeAmount("-12.50", "CRDT")
	require.NoError(t, err)
	assert.Equal(t, "12.5", amount)

	_, err = normalizeAmount("bad", "CRDT")
	require.Error(t, err)

	assert.Equal(t, "", normalizeDateOrEmpty("not-a-date"))
	assert.Equal(t, "", camtRemittanceInfo{}.Reference())
	assert.Equal(t, "RF123", camtRemittanceInfo{
		Structured: []camtStructuredRemittance{{
			CreditorReference: camtCreditorReference{Reference: " RF123 "},
		}},
	}.Reference())
}

func TestParseTransactionsPropagatesEntryErrors(t *testing.T) {
	_, err := ParseTransactions(`<Document><BkToCstmrStmt><Stmt><Acct><Id><IBAN>EE123</IBAN></Id></Acct><Ntry><Amt Ccy="EUR">1.00</Amt><CdtDbtInd>CRDT</CdtDbtInd><BookgDt><Dt>bad-date</Dt></BookgDt></Ntry></Stmt></BkToCstmrStmt></Document>`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date")
}
