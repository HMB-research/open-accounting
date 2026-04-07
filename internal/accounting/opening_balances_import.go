package accounting

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type openingBalanceImportRow struct {
	rowNumber int
	values    map[string]string
}

var openingBalanceHeaderAliases = map[string]string{
	"account_code":     "account_code",
	"code":             "account_code",
	"account":          "account_code",
	"description":      "description",
	"line_description": "description",
	"debit":            "debit",
	"debit_amount":     "debit",
	"credit":           "credit",
	"credit_amount":    "credit",
}

// ImportOpeningBalancesCSV imports opening balances from CSV and posts them as a journal entry.
func (s *Service) ImportOpeningBalancesCSV(ctx context.Context, schemaName, tenantID string, req *ImportOpeningBalancesRequest) (*ImportOpeningBalancesResult, error) {
	if strings.TrimSpace(req.EntryDate) == "" {
		return nil, fmt.Errorf("entry_date is required")
	}
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	entryDate, err := time.Parse("2006-01-02", req.EntryDate)
	if err != nil {
		return nil, fmt.Errorf("entry_date must be in YYYY-MM-DD format")
	}

	rows, err := parseOpeningBalanceImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no opening balance rows found in CSV")
	}

	accounts, err := s.repo.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	accountByCode := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		accountByCode[normalizedAccountImportKey(account.Code)] = account
	}

	lines := make([]CreateJournalEntryLineReq, 0, len(rows))
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, row := range rows {
		accountCode := strings.TrimSpace(row.values["account_code"])
		account, ok := accountByCode[normalizedAccountImportKey(accountCode)]
		if !ok {
			return nil, fmt.Errorf("row %d: account_code %q was not found", row.rowNumber, accountCode)
		}

		debit, credit, err := parseOpeningBalanceAmounts(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", row.rowNumber, err)
		}

		lineDescription := strings.TrimSpace(row.values["description"])
		if lineDescription == "" {
			lineDescription = fmt.Sprintf("Opening balance for %s", account.Name)
		}

		lines = append(lines, CreateJournalEntryLineReq{
			AccountID:    account.ID,
			Description:  lineDescription,
			DebitAmount:  debit,
			CreditAmount: credit,
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
		})
		totalDebit = totalDebit.Add(debit)
		totalCredit = totalCredit.Add(credit)
	}

	if totalDebit.IsZero() || totalCredit.IsZero() {
		return nil, fmt.Errorf("opening balances must include both debit and credit totals")
	}
	if !totalDebit.Equal(totalCredit) {
		return nil, fmt.Errorf("opening balances do not balance: debits=%s credits=%s", totalDebit.String(), totalCredit.String())
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = "Opening balances"
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = fmt.Sprintf("OB-%d", entryDate.Year())
	}

	entry, err := s.CreateJournalEntry(ctx, schemaName, tenantID, &CreateJournalEntryRequest{
		EntryDate:   entryDate,
		Description: description,
		Reference:   reference,
		SourceType:  "OPENING_BALANCE",
		Lines:       lines,
		UserID:      req.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("create opening-balance journal entry: %w", err)
	}

	if err := s.PostJournalEntry(ctx, schemaName, tenantID, entry.ID, req.UserID); err != nil {
		return nil, fmt.Errorf("post opening-balance journal entry: %w", err)
	}

	postedEntry, err := s.GetJournalEntry(ctx, schemaName, tenantID, entry.ID)
	if err != nil {
		return nil, fmt.Errorf("load opening-balance journal entry: %w", err)
	}

	return &ImportOpeningBalancesResult{
		FileName:      req.FileName,
		RowsProcessed: len(rows),
		LinesImported: len(lines),
		TotalDebit:    totalDebit,
		TotalCredit:   totalCredit,
		JournalEntry:  postedEntry,
	}, nil
}

func parseOpeningBalanceImportRows(content string) ([]openingBalanceImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectAccountImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("csv file is empty")
		}
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	hasAccountCode := false
	hasDebit := false
	hasCredit := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalOpeningBalanceHeader(header)
		switch canonicalHeaders[i] {
		case "account_code":
			hasAccountCode = true
		case "debit":
			hasDebit = true
		case "credit":
			hasCredit = true
		}
	}

	if !hasAccountCode || !hasDebit || !hasCredit {
		return nil, fmt.Errorf("missing required columns: account_code, debit, credit")
	}

	rows := make([]openingBalanceImportRow, 0)
	rowNumber := 1
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse csv row %d: %w", rowNumber+1, err)
		}

		rowNumber++
		values := make(map[string]string, len(canonicalHeaders))
		isBlank := true
		for i, header := range canonicalHeaders {
			if header == "" {
				continue
			}

			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				isBlank = false
			}
			values[header] = value
		}

		if isBlank {
			continue
		}

		rows = append(rows, openingBalanceImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func parseOpeningBalanceAmounts(row openingBalanceImportRow) (decimal.Decimal, decimal.Decimal, error) {
	debit, err := parseOpeningBalanceDecimal(row.values["debit"], "debit")
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	credit, err := parseOpeningBalanceDecimal(row.values["credit"], "credit")
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	if debit.LessThan(decimal.Zero) || credit.LessThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, fmt.Errorf("amounts cannot be negative")
	}
	if debit.IsZero() && credit.IsZero() {
		return decimal.Zero, decimal.Zero, fmt.Errorf("either debit or credit is required")
	}
	if debit.GreaterThan(decimal.Zero) && credit.GreaterThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, fmt.Errorf("row cannot contain both debit and credit amounts")
	}

	return debit, credit, nil
}

func parseOpeningBalanceDecimal(value string, fieldName string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, nil
	}

	parsed, err := decimal.NewFromString(normalizeOpeningBalanceDecimal(trimmed))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", fieldName)
	}
	return parsed, nil
}

func canonicalOpeningBalanceHeader(header string) string {
	normalized := normalizedAccountImportHeader(header)
	if canonical, ok := openingBalanceHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizeOpeningBalanceDecimal(value string) string {
	if strings.Contains(value, ",") && !strings.Contains(value, ".") {
		return strings.ReplaceAll(value, ",", ".")
	}
	return strings.ReplaceAll(value, ",", "")
}
