package accounting

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const defaultJournalImportSourceType = "HISTORICAL_JOURNAL_IMPORT"

type journalImportRow struct {
	rowNumber int
	values    map[string]string
}

type journalImportGroup struct {
	key       string
	firstRow  int
	rows      []journalImportRow
	reference string
}

var journalImportHeaderAliases = map[string]string{
	"entry_reference":     "entry_reference",
	"reference":           "entry_reference",
	"document_number":     "entry_reference",
	"voucher_number":      "entry_reference",
	"journal_number":      "entry_reference",
	"entry_date":          "entry_date",
	"date":                "entry_date",
	"posting_date":        "entry_date",
	"entry_description":   "entry_description",
	"journal_description": "entry_description",
	"entry_memo":          "entry_description",
	"account_code":        "account_code",
	"code":                "account_code",
	"account":             "account_code",
	"line_description":    "line_description",
	"description":         "line_description",
	"memo":                "line_description",
	"debit":               "debit",
	"debit_amount":        "debit",
	"credit":              "credit",
	"credit_amount":       "credit",
	"currency":            "currency",
	"exchange_rate":       "exchange_rate",
	"source_type":         "source_type",
	"source_id":           "source_id",
}

// ImportJournalEntriesCSV imports historical double-entry journals from grouped CSV rows.
func (s *Service) ImportJournalEntriesCSV(ctx context.Context, schemaName, tenantID string, req *ImportJournalEntriesRequest) (*ImportJournalEntriesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	rows, err := parseJournalImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no journal rows found in CSV")
	}

	accounts, err := s.repo.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	accountByCode := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		accountByCode[normalizedAccountImportKey(account.Code)] = account
	}

	groups := groupJournalImportRows(rows)
	result := &ImportJournalEntriesResult{
		FileName:      req.FileName,
		RowsProcessed: len(rows),
		Errors:        []ImportJournalEntriesRowError{},
	}

	for _, group := range groups {
		entry, groupDebit, groupCredit, err := s.buildJournalEntryFromImportGroup(ctx, schemaName, tenantID, req, group, accountByCode)
		if err != nil {
			result.RowsSkipped += len(group.rows)
			result.Errors = append(result.Errors, ImportJournalEntriesRowError{
				Row:            group.firstRow,
				EntryReference: group.reference,
				Message:        err.Error(),
			})
			continue
		}

		result.EntriesCreated++
		result.LinesImported += len(entry.Lines)
		result.TotalDebit = result.TotalDebit.Add(groupDebit)
		result.TotalCredit = result.TotalCredit.Add(groupCredit)
		result.JournalEntries = append(result.JournalEntries, *entry)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func (s *Service) buildJournalEntryFromImportGroup(
	ctx context.Context,
	schemaName, tenantID string,
	req *ImportJournalEntriesRequest,
	group *journalImportGroup,
	accountByCode map[string]Account,
) (*JournalEntry, decimal.Decimal, decimal.Decimal, error) {
	firstValues := group.rows[0].values
	if strings.TrimSpace(group.reference) == "" {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("entry_reference is required")
	}
	entryDate, err := parseJournalImportDate(firstValues["entry_date"])
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, err
	}
	for _, row := range group.rows[1:] {
		rowDate, err := parseJournalImportDate(row.values["entry_date"])
		if err != nil {
			return nil, decimal.Zero, decimal.Zero, fmt.Errorf("row %d: %w", row.rowNumber, err)
		}
		if !sameAccountingDate(entryDate, rowDate) {
			return nil, decimal.Zero, decimal.Zero, fmt.Errorf("row %d: entry_date must match the group date %s", row.rowNumber, entryDate.Format("2006-01-02"))
		}
	}
	if req.PeriodLockDate != nil && !entryDate.After(*req.PeriodLockDate) {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("period locked through %s", req.PeriodLockDate.Format("2006-01-02"))
	}

	lines := make([]CreateJournalEntryLineReq, 0, len(group.rows))
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	for _, row := range group.rows {
		accountCode := strings.TrimSpace(row.values["account_code"])
		account, ok := accountByCode[normalizedAccountImportKey(accountCode)]
		if !ok {
			return nil, decimal.Zero, decimal.Zero, fmt.Errorf("row %d: account_code %q was not found", row.rowNumber, accountCode)
		}

		debit, credit, err := parseJournalImportAmounts(row)
		if err != nil {
			return nil, decimal.Zero, decimal.Zero, fmt.Errorf("row %d: %w", row.rowNumber, err)
		}
		exchangeRate, err := parseJournalImportExchangeRate(row.values["exchange_rate"])
		if err != nil {
			return nil, decimal.Zero, decimal.Zero, fmt.Errorf("row %d: %w", row.rowNumber, err)
		}
		currency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
		if currency == "" {
			currency = "EUR"
		}

		lines = append(lines, CreateJournalEntryLineReq{
			AccountID:    account.ID,
			Description:  strings.TrimSpace(row.values["line_description"]),
			DebitAmount:  debit,
			CreditAmount: credit,
			Currency:     currency,
			ExchangeRate: exchangeRate,
		})
		totalDebit = totalDebit.Add(debit.Mul(exchangeRate))
		totalCredit = totalCredit.Add(credit.Mul(exchangeRate))
	}

	if len(lines) < 2 {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("journal entry must have at least two lines")
	}
	if !totalDebit.Equal(totalCredit) {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("journal entry does not balance: debits=%s credits=%s", totalDebit.String(), totalCredit.String())
	}

	description := strings.TrimSpace(firstValues["entry_description"])
	if description == "" {
		description = fmt.Sprintf("Imported journal %s", group.reference)
	}
	sourceType := strings.TrimSpace(firstValues["source_type"])
	if sourceType == "" {
		sourceType = strings.TrimSpace(req.SourceType)
	}
	if sourceType == "" {
		sourceType = defaultJournalImportSourceType
	}
	sourceID, err := optionalJournalImportUUID("source_id", firstValues["source_id"])
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, err
	}

	createReq := &CreateJournalEntryRequest{
		EntryDate:   entryDate,
		Description: description,
		Reference:   group.reference,
		SourceType:  sourceType,
		SourceID:    sourceID,
		Lines:       lines,
		UserID:      req.UserID,
	}
	entry, err := s.CreateJournalEntry(ctx, schemaName, tenantID, createReq)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, err
	}
	if !req.PostEntries {
		return entry, totalDebit, totalCredit, nil
	}

	if err := s.PostJournalEntry(ctx, schemaName, tenantID, entry.ID, req.UserID); err != nil {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("post imported journal entry: %w", err)
	}
	postedEntry, err := s.GetJournalEntry(ctx, schemaName, tenantID, entry.ID)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, fmt.Errorf("load imported journal entry: %w", err)
	}
	return postedEntry, totalDebit, totalCredit, nil
}

func parseJournalImportRows(content string) ([]journalImportRow, error) {
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
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	required := map[string]bool{
		"entry_reference": false,
		"entry_date":      false,
		"account_code":    false,
		"debit":           false,
		"credit":          false,
	}
	for i, header := range headers {
		canonicalHeaders[i] = canonicalJournalImportHeader(header)
		if _, ok := required[canonicalHeaders[i]]; ok {
			required[canonicalHeaders[i]] = true
		}
	}
	for column, found := range required {
		if !found {
			return nil, fmt.Errorf("missing required column: %s", column)
		}
	}

	rows := make([]journalImportRow, 0)
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
		rows = append(rows, journalImportRow{rowNumber: rowNumber, values: values})
	}
	return rows, nil
}

func groupJournalImportRows(rows []journalImportRow) []*journalImportGroup {
	groupByReference := make(map[string]*journalImportGroup)
	groups := make([]*journalImportGroup, 0)
	for _, row := range rows {
		reference := strings.TrimSpace(row.values["entry_reference"])
		key := normalizedAccountImportKey(reference)
		if key == "" {
			key = fmt.Sprintf("row-%d", row.rowNumber)
		}
		group, ok := groupByReference[key]
		if !ok {
			group = &journalImportGroup{
				key:       key,
				firstRow:  row.rowNumber,
				reference: reference,
			}
			groupByReference[key] = group
			groups = append(groups, group)
		}
		group.rows = append(group.rows, row)
	}
	return groups
}

func parseJournalImportDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("entry_date is required")
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("entry_date must be in YYYY-MM-DD format")
	}
	return parsed, nil
}

func parseJournalImportAmounts(row journalImportRow) (decimal.Decimal, decimal.Decimal, error) {
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

func parseJournalImportExchangeRate(value string) (decimal.Decimal, error) {
	exchangeRate, err := parseOpeningBalanceDecimal(value, "exchange_rate")
	if err != nil {
		return decimal.Zero, err
	}
	if exchangeRate.IsZero() {
		return decimal.NewFromInt(1), nil
	}
	if exchangeRate.LessThan(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("exchange_rate cannot be negative")
	}
	return exchangeRate, nil
}

func canonicalJournalImportHeader(header string) string {
	normalized := normalizedAccountImportHeader(header)
	if canonical, ok := journalImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func optionalJournalImportUUID(field, value string) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", field)
	}
	id := parsedID.String()
	return &id, nil
}

func sameAccountingDate(a, b time.Time) bool {
	return a.Format("2006-01-02") == b.Format("2006-01-02")
}
