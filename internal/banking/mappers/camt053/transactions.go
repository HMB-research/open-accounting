package camt053

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/banking"
)

// DetectTransactions reports whether content appears to be a camt.053 account statement.
func DetectTransactions(content string) bool {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "<") {
		return false
	}
	return strings.Contains(trimmed, "camt.053") ||
		strings.Contains(trimmed, "<BkToCstmrStmt") ||
		strings.Contains(trimmed, ":BkToCstmrStmt")
}

// ParseTransactions parses ISO 20022 camt.053 account statement XML rows.
func ParseTransactions(content string) ([]banking.CSVTransactionRow, error) {
	var document camtDocument
	if err := xml.Unmarshal([]byte(strings.TrimSpace(content)), &document); err != nil {
		return nil, fmt.Errorf("parse camt.053 XML: %w", err)
	}
	if len(document.Statement.Statements) == 0 {
		return nil, fmt.Errorf("camt.053 XML contains no statements")
	}

	var rows []banking.CSVTransactionRow
	for _, statement := range document.Statement.Statements {
		for entryIndex, entry := range statement.Entries {
			entryRows, err := rowsFromEntry(statement, entry, entryIndex+1)
			if err != nil {
				return nil, err
			}
			rows = append(rows, entryRows...)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("camt.053 XML contains no transactions")
	}
	return rows, nil
}

func rowsFromEntry(statement camtStatement, entry camtEntry, entryNum int) ([]banking.CSVTransactionRow, error) {
	date, err := normalizeDate(firstNonEmpty(entry.BookingDate.Date, entry.ValueDate.Date, entry.ValueDate.DateTime))
	if err != nil {
		return nil, fmt.Errorf("camt.053 entry %d has invalid date: %w", entryNum, err)
	}
	amount, err := normalizeAmount(entry.Amount.Value, entry.CreditDebitIndicator)
	if err != nil {
		return nil, fmt.Errorf("camt.053 entry %d has invalid amount: %w", entryNum, err)
	}
	if date == "" || amount == "" {
		return nil, fmt.Errorf("camt.053 entry %d requires date and amount", entryNum)
	}

	transactionDetails := flattenTransactionDetails(entry)
	if len(transactionDetails) == 0 {
		return []banking.CSVTransactionRow{rowFromDetail(statement, entry, camtTransactionDetails{}, date, amount)}, nil
	}

	rows := make([]banking.CSVTransactionRow, 0, len(transactionDetails))
	for _, detail := range transactionDetails {
		rows = append(rows, rowFromDetail(statement, entry, detail, date, amount))
	}
	return rows, nil
}

func rowFromDetail(statement camtStatement, entry camtEntry, detail camtTransactionDetails, date, amount string) banking.CSVTransactionRow {
	counterpartyName, counterpartyAccount := counterparty(entry.CreditDebitIndicator, detail.RelatedParties)
	description := firstNonEmpty(
		firstNonEmpty(detail.RemittanceInfo.Unstructured...),
		detail.References.EndToEndID,
		detail.References.PaymentInformationID,
		counterpartyName,
		entry.AccountServicerReference,
		entry.EntryReference,
		"camt.053 account statement entry",
	)

	return banking.CSVTransactionRow{
		Date:                date,
		ValueDate:           normalizeDateOrEmpty(firstNonEmpty(entry.ValueDate.Date, entry.ValueDate.DateTime)),
		Amount:              amount,
		Currency:            strings.ToUpper(firstNonEmpty(entry.Amount.Currency, detail.AmountDetails.TransactionAmount.Amount.Currency, detail.AmountDetails.InstructedAmount.Amount.Currency, statement.Account.Currency)),
		SourceAccount:       statement.Account.ID.IBAN,
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

func counterparty(direction string, parties camtRelatedParties) (string, string) {
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
	if len(trimmed) >= len("2006-01-02") && strings.Contains(trimmed[:10], "-") {
		trimmed = trimmed[:10]
	}
	parsed, err := banking.ParseDateFormats(trimmed)
	if err != nil {
		return "", err
	}
	return parsed.Format("2006-01-02"), nil
}

func normalizeDateOrEmpty(value string) string {
	normalized, err := normalizeDate(value)
	if err != nil {
		return ""
	}
	return normalized
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
	Account camtStatementAccount `xml:"Acct"`
	Entries []camtEntry          `xml:"Ntry"`
}

type camtStatementAccount struct {
	ID       camtAccountID `xml:"Id"`
	Currency string        `xml:"Ccy"`
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
	AmountDetails  camtAmountDetails  `xml:"AmtDtls"`
	References     camtReferences     `xml:"Refs"`
	RelatedParties camtRelatedParties `xml:"RltdPties"`
	RemittanceInfo camtRemittanceInfo `xml:"RmtInf"`
}

type camtAmountDetails struct {
	InstructedAmount  camtAmountWrapper `xml:"InstdAmt"`
	TransactionAmount camtAmountWrapper `xml:"TxAmt"`
}

type camtAmountWrapper struct {
	Amount camtAmount `xml:"Amt"`
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
	Currency string `xml:"Ccy,attr"`
	Value    string `xml:",chardata"`
}
