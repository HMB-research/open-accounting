package lhv

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/banking/mappers"
	camt053mapper "github.com/HMB-research/open-accounting/internal/banking/mappers/camt053"
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
	return camt053mapper.DetectTransactions(content)
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
			Currency:            strings.ToUpper(mappers.Field(record, parsed.Index, "Currency", "Valuuta")),
			SourceAccount:       mappers.Field(record, parsed.Index, "Client account", "Kliendi konto"),
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
	return camt053mapper.ParseTransactions(content)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
