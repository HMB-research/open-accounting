package lhv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTransactions(t *testing.T) {
	content := "Client account;Document number;Date;Beneficiary's/remitter's account;Beneficiary's/remitter's name;Debit/Credit (D/C);Amount;Reference number;Archival ID;Details;Currency;Personal identification code or registry code;Beneficiary's/remitter's bank's BIC;Payment initiator's name;Entry reference;Account service provider's reference\n" +
		"EE457700771000676899;123;2026-03-15;EE867700771000681884;Test Client;D;12,50;100513845;202603150001;EUR payment;EUR;12345678;LHVBEE22;;ENTRY-1;LHV-UNIQUE-1\n"

	rows, err := ParseTransactions(content)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-03-15", rows[0].Date)
	assert.Equal(t, "-12.5", rows[0].Amount)
	assert.Equal(t, "EUR payment", rows[0].Description)
	assert.Equal(t, "100513845", rows[0].Reference)
	assert.Equal(t, "Test Client", rows[0].CounterpartyName)
	assert.Equal(t, "EE867700771000681884", rows[0].CounterpartyAccount)
	assert.Equal(t, "LHV-UNIQUE-1", rows[0].ExternalID)
	assert.Equal(t, "EUR", rows[0].Currency)
	assert.Equal(t, "EE457700771000676899", rows[0].SourceAccount)
}

func TestDetectTransactionsRecognizesEstonianHeaders(t *testing.T) {
	content := "Kliendi konto;Dokumendi number;Kuupäev;Saaja/maksja konto;Saaja/maksja nimi;Deebet/Kreedit (D/C);Summa;Viitenumber;Arhiveerimistunnus;Selgitus;Valuuta;Isikukood või registrikood;Saaja/maksja panga BIC;Mak" + "se algataja nimi;Kande viide;Konto teenusepakkuja viide\n" +
		"EE457700771000676899;123;15.03.2026;EE867700771000681884;Test Client;C;12.50;100513845;202603150001;EUR payment;EUR;12345678;LHVBEE22;;ENTRY-1;LHV-UNIQUE-1\n"

	assert.True(t, DetectTransactions(content))
	rows, err := ParseTransactions(content)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-03-15", rows[0].Date)
	assert.Equal(t, "12.5", rows[0].Amount)
}

func TestParseCAMTTransactionsFromOfficialLHVSamples(t *testing.T) {
	const content = `<Document xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02" xsi:schemaLocation="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02 camt.053.001.02.xsd">
  <BkToCstmrStmt>
    <GrpHdr>
      <MsgId>2697f25bd4ca4f108f60407a507edefe</MsgId>
      <CreDtTm>2021-08-03T18:52:11.530168</CreDtTm>
    </GrpHdr>
    <Stmt>
      <Id>13122697EUR</Id>
      <CreDtTm>2021-08-03T18:52:11.530168</CreDtTm>
      <FrToDt>
        <FrDtTm>2021-08-03T18:19:00</FrDtTm>
        <ToDtTm>2021-08-03T18:21:59</ToDtTm>
      </FrToDt>
      <Acct>
        <Id>
          <IBAN>EE867700771001260974</IBAN>
        </Id>
        <Ccy>EUR</Ccy>
        <Svcr>
          <FinInstnId>
            <BIC>LHVBEE20</BIC>
            <Nm>AS LHV Pank</Nm>
            <PstlAdr>
              <AdrTp>BIZZ</AdrTp>
              <StrtNm>Tartu mnt.</StrtNm>
              <BldgNb>2</BldgNb>
              <PstCd>10145</PstCd>
              <TwnNm>Tallinn</TwnNm>
              <CtrySubDvsn>Harjumaa</CtrySubDvsn>
              <Ctry>EE</Ctry>
            </PstlAdr>
          </FinInstnId>
        </Svcr>
      </Acct>
      <Bal>
        <Tp>
          <CdOrPrtry>
            <Cd>OPBD</Cd>
          </CdOrPrtry>
        </Tp>
        <Amt Ccy="EUR">0.00</Amt>
        <CdtDbtInd>CRDT</CdtDbtInd>
        <Dt>
          <Dt>2021-08-03</Dt>
        </Dt>
      </Bal>
      <Bal>
        <Tp>
          <CdOrPrtry>
            <Cd>CLBD</Cd>
          </CdOrPrtry>
        </Tp>
        <Amt Ccy="EUR">1.00</Amt>
        <CdtDbtInd>DBIT</CdtDbtInd>
        <Dt>
          <Dt>2021-08-03</Dt>
        </Dt>
      </Bal>
      <TxsSummry>
        <TtlCdtNtries>
          <NbOfNtries>0</NbOfNtries>
          <Sum>0.00</Sum>
        </TtlCdtNtries>
        <TtlDbtNtries>
          <NbOfNtries>1</NbOfNtries>
          <Sum>1.00</Sum>
        </TtlDbtNtries>
      </TxsSummry>
      <Ntry>
        <Amt Ccy="EUR">1.00</Amt>
        <CdtDbtInd>DBIT</CdtDbtInd>
        <Sts>BOOK</Sts>
        <BookgDt>
          <Dt>2021-08-03</Dt>
        </BookgDt>
        <AcctSvcrRef>F56A3D416EF4EB11911400155D41A83F</AcctSvcrRef>
        <BkTxCd>
          <Domn>
            <Cd>PMNT</Cd>
            <Fmly>
              <Cd>ICDT</Cd>
              <SubFmlyCd>OTHR</SubFmlyCd>
            </Fmly>
          </Domn>
          <Prtry>
            <Cd>SEPA</Cd>
          </Prtry>
        </BkTxCd>
        <NtryDtls>
          <TxDtls>
            <Refs>
              <AcctSvcrRef>F56A3D416EF4EB11911400155D41A83F</AcctSvcrRef>
              <PmtInfId>PMTINFIDLHVTEST-58974</PmtInfId>
              <InstrId>INSTRIDLHVTEST-58974</InstrId>
              <EndToEndId>ENDTOENDIDLHVTEST-58974</EndToEndId>
            </Refs>
            <AmtDtls>
              <InstdAmt>
                <Amt Ccy="EUR">1.00</Amt>
              </InstdAmt>
              <TxAmt>
                <Amt Ccy="EUR">1.00</Amt>
              </TxAmt>
            </AmtDtls>
            <RltdPties>
              <Dbtr>
                <Nm>Company of John</Nm>
              </Dbtr>
              <DbtrAcct>
                <Id>
                  <IBAN>EE467777000010897825</IBAN>
                </Id>
              </DbtrAcct>
              <Cdtr>
                <Nm>John Smith</Nm>
              </Cdtr>
              <CdtrAcct>
                <Id>
                  <IBAN>ES0000000000000000000000</IBAN>
                </Id>
              </CdtrAcct>
            </RltdPties>
            <RltdAgts>
              <DbtrAgt>
                <FinInstnId>
                  <BIC>LHVBEE20</BIC>
                  <Nm>AS LHV Pank</Nm>
                </FinInstnId>
              </DbtrAgt>
              <CdtrAgt>
                <FinInstnId>
                  <BIC>BICXXX</BIC>
                  <Nm>BANK NAME</Nm>
                </FinInstnId>
              </CdtrAgt>
            </RltdAgts>
            <RmtInf>
              <Ustrd>Payment Description 58974</Ustrd>
              <Strd>
                <CdtrRefInf>
                  <Tp>
                    <CdOrPrtry>
                      <Cd>SCOR</Cd>
                    </CdOrPrtry>
                  </Tp>
                  <Ref>58974</Ref>
                </CdtrRefInf>
              </Strd>
            </RmtInf>
          </TxDtls>
        </NtryDtls>
      </Ntry>
    </Stmt>
  </BkToCstmrStmt>
</Document>`

	assert.True(t, DetectTransactions(content))
	rows, err := ParseTransactions(content)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2021-08-03", rows[0].Date)
	assert.Equal(t, "-1", rows[0].Amount)
	assert.Equal(t, "Payment Description 58974", rows[0].Description)
	assert.Equal(t, "58974", rows[0].Reference)
	assert.Equal(t, "John Smith", rows[0].CounterpartyName)
	assert.Equal(t, "ES0000000000000000000000", rows[0].CounterpartyAccount)
	assert.Equal(t, "F56A3D416EF4EB11911400155D41A83F", rows[0].ExternalID)
}

func TestParseCAMTTransactionsFromCurrentLHVDocsStatementDataSample(t *testing.T) {
	// Fixture mirrors the LHV Connect Account Statement "Statement data" sample:
	// https://docs.lhv.com/home/connect/services/account-reports/account-statement
	content, err := os.ReadFile("testdata/account_statement_camt053_official.xml")
	require.NoError(t, err)

	assert.True(t, DetectCAMTTransactions(string(content)))
	rows, err := ParseCAMTTransactions(string(content))
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

func TestDetectTransactionsRejectsUnknownContent(t *testing.T) {
	assert.False(t, DetectTransactions("date,amount\n2026-03-15,10"))
	assert.False(t, DetectCSVTransactions(" "))
	assert.False(t, DetectCSVTransactions("date,amount\n2026-03-15,10"))
}

func TestParseCSVTransactionsValidationBranches(t *testing.T) {
	_, err := ParseTransactions(" ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LHV bank transaction CSV is empty")

	_, err = ParseCSVTransactions("date;amount\n2026-03-15;10\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "headers not recognized")

	header := "Client account;Document number;Date;Debit/Credit (D/C);Amount;Account service provider's reference\n"
	_, err = ParseCSVTransactions(header + "EE123;1;bad-date;C;10;REF\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date")

	_, err = ParseCSVTransactions(header + "EE123;1;2026-03-15;C;bad;REF\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")

	_, err = ParseCSVTransactions(header + "EE123;1;;C;10;REF\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires date and amount")

	_, err = ParseCSVTransactions(header + " ; ; ; ; ; \n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contains no transactions")
}

func TestParseCSVTransactionsFallbackDescriptionAndReferences(t *testing.T) {
	header := "Client account;Document number;Date;Debit/Credit (D/C);Amount;Account service provider's reference;Entry reference;Archival ID\n"

	rows, err := ParseCSVTransactions(header + "EE123;DOC-1;2026-03-15;K;-12.50;;ENTRY-1;ARCH-1\n")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "DOC-1", rows[0].Description)
	assert.Equal(t, "12.5", rows[0].Amount)
	assert.Equal(t, "ENTRY-1", rows[0].ExternalID)

	rows, err = ParseCSVTransactions(header + "EE123;;2026-03-16;C;5.00;;;ARCH-2\n")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "LHV account statement entry", rows[0].Description)
	assert.Equal(t, "ARCH-2", rows[0].ExternalID)
}

func TestLHVNormalizationBranches(t *testing.T) {
	amount, err := normalizeAmount("", "D")
	require.NoError(t, err)
	assert.Equal(t, "", amount)

	amount, err = normalizeAmount("1 234,56", "DEEBET")
	require.NoError(t, err)
	assert.Equal(t, "-1234.56", amount)

	amount, err = normalizeAmount("-10", "KREEDIT")
	require.NoError(t, err)
	assert.Equal(t, "10", amount)

	_, err = normalizeAmount("not-number", "C")
	require.Error(t, err)

	date, err := normalizeDate("")
	require.NoError(t, err)
	assert.Equal(t, "", date)

	_, err = normalizeDate("not-a-date")
	require.Error(t, err)

	assert.Equal(t, "first", firstNonEmpty(" ", " first ", "second"))
	assert.Equal(t, "", firstNonEmpty(" ", ""))
}
