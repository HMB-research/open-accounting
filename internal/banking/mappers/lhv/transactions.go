package lhv

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/banking/mappers"
)

// DetectTransactions reports whether content matches a supported LHV statement layout.
func DetectTransactions(content string) bool {
	if DetectCAMTTransactions(content) {
		return true
	}
	return DetectCSVTransactions(content)
}

// DetectCSVTransactions reports whether the header matches LHV's 2026 account statement CSV layout.
func DetectCSVTransactions(content string) bool {
	parsed, err := mappers.ParseCSV(content, "bank transaction")
	if err != nil {
		return false
	}
	return hasLHVHeaders(parsed.Index)
}

// DetectCAMTTransactions reports whether content appears to be an LHV camt.053 account statement.
func DetectCAMTTransactions(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "<") &&
		(strings.Contains(trimmed, "camt.053") || strings.Contains(trimmed, "<BkToCstmrStmt>"))
}

// ParseTransactions parses supported LHV statement rows.
func ParseTransactions(content string) ([]banking.CSVTransactionRow, error) {
	if DetectCAMTTransactions(content) {
		return ParseCAMTTransactions(content)
	}
	return ParseCSVTransactions(content)
}

// ParseCSVTransactions parses LHV Internet Bank account statement CSV rows.
func ParseCSVTransactions(content string) ([]banking.CSVTransactionRow, error) {
	parsed, err := mappers.ParseCSV(content, "LHV bank transaction")
	if err != nil {
		return nil, err
	}
	if !hasLHVHeaders(parsed.Index) {
		return nil, fmt.Errorf("LHV bank transaction CSV headers not recognized")
	}

	var rows []banking.CSVTransactionRow
	for i, record := range parsed.Rows {
		rowNum := i + 2
		date, err := normalizeDate(mappers.Field(record, parsed.Index, "Date", "Kuupäev"))
		if err != nil {
			return nil, fmt.Errorf("LHV bank transaction CSV row %d has invalid date: %w", rowNum, err)
		}
		amount, err := normalizeAmount(
			mappers.Field(record, parsed.Index, "Amount", "Summa"),
			mappers.Field(record, parsed.Index, "Debit/Credit (D/C)", "Deebet/Kreedit (D/C)", "D/C"),
		)
		if err != nil {
			return nil, fmt.Errorf("LHV bank transaction CSV row %d has invalid amount: %w", rowNum, err)
		}

		details := mappers.Field(record, parsed.Index, "Details", "Selgitus")
		counterpartyName := mappers.Field(record, parsed.Index, "Beneficiary's/remitter's name", "Beneficiary’s/remitter’s name", "Saaja/maksja nimi")
		description := details
		if description == "" {
			description = counterpartyName
		}
		if description == "" {
			description = mappers.Field(record, parsed.Index, "Document number", "Dokumendi number")
		}
		if date == "" || amount == "" {
			return nil, fmt.Errorf("LHV bank transaction CSV row %d requires date and amount", rowNum)
		}
		if description == "" {
			description = "LHV account statement entry"
		}

		rows = append(rows, banking.CSVTransactionRow{
			Date:                date,
			Amount:              amount,
			Description:         description,
			Reference:           mappers.Field(record, parsed.Index, "Reference number", "Viitenumber"),
			CounterpartyName:    counterpartyName,
			CounterpartyAccount: mappers.Field(record, parsed.Index, "Beneficiary's/remitter's account", "Beneficiary’s/remitter’s account", "Saaja/maksja konto"),
			ExternalID: firstNonEmpty(
				mappers.Field(record, parsed.Index, "Account service provider's reference", "Account service provider’s reference", "Konto teenusepakkuja viide"),
				mappers.Field(record, parsed.Index, "Entry reference", "Kande viide"),
				mappers.Field(record, parsed.Index, "Archival ID", "Arhiveerimistunnus"),
			),
		})
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("LHV bank transaction CSV contains no transactions")
	}
	return rows, nil
}

// ParseCAMTTransactions parses LHV Connect camt.053 account statement XML rows.
func ParseCAMTTransactions(content string) ([]banking.CSVTransactionRow, error) {
	var document camtDocument
	if err := xml.Unmarshal([]byte(strings.TrimSpace(content)), &document); err != nil {
		return nil, fmt.Errorf("parse LHV camt.053 XML: %w", err)
	}
	if len(document.Statement.Statements) == 0 {
		return nil, fmt.Errorf("LHV camt.053 XML contains no statements")
	}

	var rows []banking.CSVTransactionRow
	for _, statement := range document.Statement.Statements {
		for entryIndex, entry := range statement.Entries {
			entryRows, err := rowsFromCAMTEntry(entry, entryIndex+1)
			if err != nil {
				return nil, err
			}
			rows = append(rows, entryRows...)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("LHV camt.053 XML contains no transactions")
	}
	return rows, nil
}

func rowsFromCAMTEntry(entry camtEntry, entryNum int) ([]banking.CSVTransactionRow, error) {
	date, err := normalizeCAMTDate(firstNonEmpty(entry.BookingDate.Date, entry.ValueDate.Date, entry.ValueDate.DateTime))
	if err != nil {
		return nil, fmt.Errorf("LHV camt.053 entry %d has invalid date: %w", entryNum, err)
	}
	amount, err := normalizeAmount(entry.Amount.Value, entry.CreditDebitIndicator)
	if err != nil {
		return nil, fmt.Errorf("LHV camt.053 entry %d has invalid amount: %w", entryNum, err)
	}
	if date == "" || amount == "" {
		return nil, fmt.Errorf("LHV camt.053 entry %d requires date and amount", entryNum)
	}

	transactionDetails := flattenTransactionDetails(entry)
	if len(transactionDetails) == 0 {
		return []banking.CSVTransactionRow{rowFromCAMTDetail(entry, camtTransactionDetails{}, date, amount)}, nil
	}

	rows := make([]banking.CSVTransactionRow, 0, len(transactionDetails))
	for _, detail := range transactionDetails {
		rows = append(rows, rowFromCAMTDetail(entry, detail, date, amount))
	}
	return rows, nil
}

func rowFromCAMTDetail(entry camtEntry, detail camtTransactionDetails, date, amount string) banking.CSVTransactionRow {
	counterpartyName, counterpartyAccount := camtCounterparty(entry.CreditDebitIndicator, detail.RelatedParties)
	description := firstNonEmpty(
		firstNonEmpty(detail.RemittanceInfo.Unstructured...),
		detail.References.EndToEndID,
		detail.References.PaymentInformationID,
		counterpartyName,
		entry.AccountServicerReference,
		entry.EntryReference,
		"LHV account statement entry",
	)

	return banking.CSVTransactionRow{
		Date:                date,
		Amount:              amount,
		Description:         description,
		Reference:           detail.RemittanceInfo.Reference(),
		CounterpartyName:    counterpartyName,
		CounterpartyAccount: counterpartyAccount,
		ExternalID: firstNonEmpty(
			detail.References.AccountServicerReference,
			entry.AccountServicerReference,
			detail.References.InstructionID,
			detail.References.EndToEndID,
			entry.EntryReference,
		),
	}
}

func flattenTransactionDetails(entry camtEntry) []camtTransactionDetails {
	var details []camtTransactionDetails
	for _, entryDetails := range entry.EntryDetails {
		details = append(details, entryDetails.TransactionDetails...)
	}
	return details
}

func camtCounterparty(direction string, parties camtRelatedParties) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "DBIT", "D", "DEBIT", "DEEBET":
		return parties.Creditor.Name, parties.CreditorAccount.ID.IBAN
	case "CRDT", "C", "K", "CREDIT", "KREEDIT":
		return parties.Debtor.Name, parties.DebtorAccount.ID.IBAN
	default:
		return firstNonEmpty(parties.Creditor.Name, parties.Debtor.Name),
			firstNonEmpty(parties.CreditorAccount.ID.IBAN, parties.DebtorAccount.ID.IBAN)
	}
}

func hasLHVHeaders(index map[string]int) bool {
	return mappers.HasAnyHeader(index, "Client account", "Kliendi konto") &&
		mappers.HasAnyHeader(index, "Document number", "Dokumendi number") &&
		mappers.HasAnyHeader(index, "Debit/Credit (D/C)", "Deebet/Kreedit (D/C)") &&
		mappers.HasAnyHeader(index, "Account service provider's reference", "Account service provider’s reference", "Konto teenusepakkuja viide")
}

func normalizeAmount(value, direction string) (string, error) {
	amountText := strings.TrimSpace(value)
	if amountText == "" {
		return "", nil
	}
	amountText = strings.ReplaceAll(amountText, " ", "")
	if strings.Contains(amountText, ",") && !strings.Contains(amountText, ".") {
		amountText = strings.ReplaceAll(amountText, ",", ".")
	}
	amount, err := decimal.NewFromString(amountText)
	if err != nil {
		return "", err
	}

	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "D", "DBIT", "DEBIT", "DEEBET":
		if amount.IsPositive() {
			amount = amount.Neg()
		}
	case "C", "K", "CRDT", "CREDIT", "KREEDIT":
		if amount.IsNegative() {
			amount = amount.Abs()
		}
	}
	return amount.String(), nil
}

func normalizeDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := banking.ParseDateFormats(trimmed)
	if err != nil {
		return "", err
	}
	return parsed.Format("2006-01-02"), nil
}

func normalizeCAMTDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) >= len("2006-01-02") && strings.Contains(trimmed[:10], "-") {
		trimmed = trimmed[:10]
	}
	return normalizeDate(trimmed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type camtDocument struct {
	Statement camtBankToCustomerStatement `xml:"BkToCstmrStmt"`
}

type camtBankToCustomerStatement struct {
	Statements []camtStatement `xml:"Stmt"`
}

type camtStatement struct {
	Entries []camtEntry `xml:"Ntry"`
}

type camtEntry struct {
	EntryReference           string             `xml:"NtryRef"`
	Amount                   camtAmount         `xml:"Amt"`
	CreditDebitIndicator     string             `xml:"CdtDbtInd"`
	BookingDate              camtDateChoice     `xml:"BookgDt"`
	ValueDate                camtDateChoice     `xml:"ValDt"`
	AccountServicerReference string             `xml:"AcctSvcrRef"`
	EntryDetails             []camtEntryDetails `xml:"NtryDtls"`
}

type camtEntryDetails struct {
	TransactionDetails []camtTransactionDetails `xml:"TxDtls"`
}

type camtTransactionDetails struct {
	References     camtReferences     `xml:"Refs"`
	RelatedParties camtRelatedParties `xml:"RltdPties"`
	RemittanceInfo camtRemittanceInfo `xml:"RmtInf"`
}

type camtReferences struct {
	AccountServicerReference string `xml:"AcctSvcrRef"`
	PaymentInformationID     string `xml:"PmtInfId"`
	InstructionID            string `xml:"InstrId"`
	EndToEndID               string `xml:"EndToEndId"`
}

type camtRelatedParties struct {
	Debtor          camtParty        `xml:"Dbtr"`
	DebtorAccount   camtPartyAccount `xml:"DbtrAcct"`
	Creditor        camtParty        `xml:"Cdtr"`
	CreditorAccount camtPartyAccount `xml:"CdtrAcct"`
}

type camtParty struct {
	Name string `xml:"Nm"`
}

type camtPartyAccount struct {
	ID camtAccountID `xml:"Id"`
}

type camtAccountID struct {
	IBAN string `xml:"IBAN"`
}

type camtRemittanceInfo struct {
	Unstructured []string                   `xml:"Ustrd"`
	Structured   []camtStructuredRemittance `xml:"Strd"`
}

func (info camtRemittanceInfo) Reference() string {
	for _, structured := range info.Structured {
		if ref := strings.TrimSpace(structured.CreditorReference.Reference); ref != "" {
			return ref
		}
	}
	return ""
}

type camtStructuredRemittance struct {
	CreditorReference camtCreditorReference `xml:"CdtrRefInf"`
}

type camtCreditorReference struct {
	Reference string `xml:"Ref"`
}

type camtDateChoice struct {
	Date     string `xml:"Dt"`
	DateTime string `xml:"DtTm"`
}

type camtAmount struct {
	Value string `xml:",chardata"`
}
