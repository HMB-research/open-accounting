package banking

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

func ParseBankAccountCSVRows(content string) ([]CSVBankAccountRow, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, errors.New("bank account CSV is empty")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectBankAccountCSVDelimiter(trimmed)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read bank account CSV header: %w", err)
	}

	index := make(map[string]int, len(headers))
	for i, header := range headers {
		key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(header)), "-", "_")
		key = strings.ReplaceAll(key, " ", "_")
		index[key] = i
	}

	get := func(record []string, names ...string) string {
		for _, name := range names {
			if i, ok := index[name]; ok && i < len(record) {
				return strings.TrimSpace(record[i])
			}
		}
		return ""
	}

	var rows []CSVBankAccountRow
	for rowNum := 2; ; rowNum++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read bank account CSV row %d: %w", rowNum, err)
		}
		empty := true
		for _, field := range record {
			if strings.TrimSpace(field) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		row := CSVBankAccountRow{
			Name:          get(record, "name", "account_name", "bank_account_name"),
			AccountNumber: get(record, "account_number", "iban", "bank_account", "account_no", "account"),
			BankName:      get(record, "bank_name", "bank"),
			SwiftCode:     get(record, "swift_code", "swift", "bic"),
			Currency:      get(record, "currency"),
			GLAccountID:   get(record, "gl_account_id", "ledger_account_id"),
			GLAccountCode: get(record, "gl_account_code", "ledger_account_code", "cash_account_code"),
			IsDefault:     get(record, "is_default", "default"),
			IsActive:      get(record, "is_active", "active"),
		}
		if row.Name == "" || row.AccountNumber == "" {
			return nil, fmt.Errorf("bank account CSV row %d requires name and account_number", rowNum)
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, errors.New("bank account CSV contains no accounts")
	}
	return rows, nil
}

func detectBankAccountCSVDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexAny(content, "\r\n"); idx >= 0 {
		firstLine = content[:idx]
	}
	candidates := []rune{',', ';', '\t'}
	best := ','
	bestCount := -1
	for _, candidate := range candidates {
		count := strings.Count(firstLine, string(candidate))
		if count > bestCount {
			best = candidate
			bestCount = count
		}
	}
	return best
}
