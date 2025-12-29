package banking

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CSVFormat represents supported CSV formats
type CSVFormat string

const (
	FormatGeneric    CSVFormat = "GENERIC"
	FormatSwedbankEE CSVFormat = "SWEDBANK_EE"
	FormatSEBEE      CSVFormat = "SEB_EE"
	FormatLHVEE      CSVFormat = "LHV_EE"
)

// CSVColumnMapping defines how to map CSV columns to transaction fields
type CSVColumnMapping struct {
	DateColumn                int
	ValueDateColumn           int // -1 if not present
	AmountColumn              int
	DescriptionColumn         int
	ReferenceColumn           int // -1 if not present
	CounterpartyNameColumn    int // -1 if not present
	CounterpartyAccountColumn int // -1 if not present
	ExternalIDColumn          int // -1 if not present
	DateFormat                string
	DecimalSeparator          string
	ThousandsSeparator        string
	SkipRows                  int
	HasHeader                 bool
}

// DefaultGenericMapping returns a generic CSV mapping
func DefaultGenericMapping() CSVColumnMapping {
	return CSVColumnMapping{
		DateColumn:                0,
		ValueDateColumn:           -1,
		AmountColumn:              1,
		DescriptionColumn:         2,
		ReferenceColumn:           -1,
		CounterpartyNameColumn:    -1,
		CounterpartyAccountColumn: -1,
		ExternalIDColumn:          -1,
		DateFormat:                "2006-01-02",
		DecimalSeparator:          ".",
		ThousandsSeparator:        ",",
		SkipRows:                  0,
		HasHeader:                 true,
	}
}

// SwedbankEEMapping returns mapping for Swedbank Estonia CSV exports
func SwedbankEEMapping() CSVColumnMapping {
	return CSVColumnMapping{
		DateColumn:                0,
		ValueDateColumn:           1,
		AmountColumn:              3,
		DescriptionColumn:         6,
		ReferenceColumn:           5,
		CounterpartyNameColumn:    7,
		CounterpartyAccountColumn: 8,
		ExternalIDColumn:          -1,
		DateFormat:                "02.01.2006",
		DecimalSeparator:          ",",
		ThousandsSeparator:        " ",
		SkipRows:                  0,
		HasHeader:                 true,
	}
}

// ImportCSV imports bank transactions from a CSV file
func (s *Service) ImportCSV(ctx context.Context, schemaName, tenantID, bankAccountID string, reader io.Reader, fileName string, mapping CSVColumnMapping, skipDuplicates bool) (*ImportResult, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	// Get bank account to determine currency
	account, err := s.GetBankAccount(ctx, schemaName, tenantID, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}

	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	result := &ImportResult{
		ImportID: uuid.New().String(),
	}

	// Skip initial rows
	for i := 0; i < mapping.SkipRows; i++ {
		_, err := csvReader.Read()
		if err != nil {
			return nil, fmt.Errorf("skip row %d: %w", i, err)
		}
	}

	// Skip header if present
	if mapping.HasHeader {
		_, err := csvReader.Read()
		if err != nil {
			return nil, fmt.Errorf("skip header: %w", err)
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rowNum := mapping.SkipRows + 1
	if mapping.HasHeader {
		rowNum++
	}

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %v", rowNum, err))
			rowNum++
			continue
		}

		// Parse date
		if mapping.DateColumn >= len(record) {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: missing date column", rowNum))
			rowNum++
			continue
		}
		dateStr := strings.TrimSpace(record[mapping.DateColumn])
		transactionDate, err := time.Parse(mapping.DateFormat, dateStr)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid date '%s': %v", rowNum, dateStr, err))
			rowNum++
			continue
		}

		// Parse value date (optional)
		var valueDate *time.Time
		if mapping.ValueDateColumn >= 0 && mapping.ValueDateColumn < len(record) {
			valueDateStr := strings.TrimSpace(record[mapping.ValueDateColumn])
			if valueDateStr != "" {
				vd, err := time.Parse(mapping.DateFormat, valueDateStr)
				if err == nil {
					valueDate = &vd
				}
			}
		}

		// Parse amount
		if mapping.AmountColumn >= len(record) {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: missing amount column", rowNum))
			rowNum++
			continue
		}
		amountStr := strings.TrimSpace(record[mapping.AmountColumn])
		amount, err := parseAmount(amountStr, mapping.DecimalSeparator, mapping.ThousandsSeparator)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid amount '%s': %v", rowNum, amountStr, err))
			rowNum++
			continue
		}

		// Parse description
		description := ""
		if mapping.DescriptionColumn >= 0 && mapping.DescriptionColumn < len(record) {
			description = strings.TrimSpace(record[mapping.DescriptionColumn])
		}

		// Parse optional fields
		reference := ""
		if mapping.ReferenceColumn >= 0 && mapping.ReferenceColumn < len(record) {
			reference = strings.TrimSpace(record[mapping.ReferenceColumn])
		}

		counterpartyName := ""
		if mapping.CounterpartyNameColumn >= 0 && mapping.CounterpartyNameColumn < len(record) {
			counterpartyName = strings.TrimSpace(record[mapping.CounterpartyNameColumn])
		}

		counterpartyAccount := ""
		if mapping.CounterpartyAccountColumn >= 0 && mapping.CounterpartyAccountColumn < len(record) {
			counterpartyAccount = strings.TrimSpace(record[mapping.CounterpartyAccountColumn])
		}

		externalID := ""
		if mapping.ExternalIDColumn >= 0 && mapping.ExternalIDColumn < len(record) {
			externalID = strings.TrimSpace(record[mapping.ExternalIDColumn])
		}

		// Check for duplicate
		if skipDuplicates {
			isDuplicate, err := s.isTransactionDuplicate(ctx, tx, schemaName, bankAccountID, transactionDate, amount, reference, externalID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d: duplicate check failed: %v", rowNum, err))
				rowNum++
				continue
			}
			if isDuplicate {
				result.DuplicatesSkipped++
				rowNum++
				continue
			}
		}

		// Insert transaction
		transactionID := uuid.New().String()
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.bank_transactions
			(id, tenant_id, bank_account_id, transaction_date, value_date, amount, currency, description, reference, counterparty_name, counterparty_account, status, external_id, imported_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'UNMATCHED', $12, NOW())
		`, schemaName), transactionID, tenantID, bankAccountID, transactionDate, valueDate, amount,
			account.Currency, description, reference, counterpartyName, counterpartyAccount, externalID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: insert failed: %v", rowNum, err))
			rowNum++
			continue
		}

		result.TransactionsImported++
		rowNum++
	}

	// Record the import
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_statement_imports (id, tenant_id, bank_account_id, file_name, transactions_imported, duplicates_skipped, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, schemaName), result.ImportID, tenantID, bankAccountID, fileName, result.TransactionsImported, result.DuplicatesSkipped)
	if err != nil {
		return nil, fmt.Errorf("record import: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}

// ImportTransactions imports pre-parsed transactions
func (s *Service) ImportTransactions(ctx context.Context, schemaName, tenantID, bankAccountID string, req *ImportCSVRequest) (*ImportResult, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	account, err := s.GetBankAccount(ctx, schemaName, tenantID, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}

	result := &ImportResult{
		ImportID: uuid.New().String(),
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, row := range req.Transactions {
		// Parse date
		transactionDate, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid date '%s'", i+1, row.Date))
			continue
		}

		// Parse value date
		var valueDate *time.Time
		if row.ValueDate != "" {
			vd, err := time.Parse("2006-01-02", row.ValueDate)
			if err == nil {
				valueDate = &vd
			}
		}

		// Parse amount
		amount, err := decimal.NewFromString(row.Amount)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: invalid amount '%s'", i+1, row.Amount))
			continue
		}

		// Check for duplicate
		if req.SkipDuplicates {
			isDuplicate, err := s.isTransactionDuplicate(ctx, tx, schemaName, bankAccountID, transactionDate, amount, row.Reference, row.ExternalID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d: duplicate check failed: %v", i+1, err))
				continue
			}
			if isDuplicate {
				result.DuplicatesSkipped++
				continue
			}
		}

		// Insert transaction
		transactionID := uuid.New().String()
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.bank_transactions
			(id, tenant_id, bank_account_id, transaction_date, value_date, amount, currency, description, reference, counterparty_name, counterparty_account, status, external_id, imported_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'UNMATCHED', $12, NOW())
		`, schemaName), transactionID, tenantID, bankAccountID, transactionDate, valueDate, amount,
			account.Currency, row.Description, row.Reference, row.CounterpartyName, row.CounterpartyAccount, row.ExternalID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: insert failed: %v", i+1, err))
			continue
		}

		result.TransactionsImported++
	}

	// Record the import
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bank_statement_imports (id, tenant_id, bank_account_id, file_name, transactions_imported, duplicates_skipped, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, schemaName), result.ImportID, tenantID, bankAccountID, req.FileName, result.TransactionsImported, result.DuplicatesSkipped)
	if err != nil {
		return nil, fmt.Errorf("record import: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return result, nil
}

func (s *Service) isTransactionDuplicate(ctx context.Context, tx interface{}, schemaName, bankAccountID string, date time.Time, amount decimal.Decimal, reference, externalID string) (bool, error) {
	var count int

	// If we have an external ID, check for exact match
	if externalID != "" {
		err := s.db.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s.bank_transactions
			WHERE bank_account_id = $1 AND external_id = $2
		`, schemaName), bankAccountID, externalID).Scan(&count)
		if err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}

	// Check for same date, amount, and reference
	if reference != "" {
		err := s.db.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s.bank_transactions
			WHERE bank_account_id = $1 AND transaction_date = $2 AND amount = $3 AND reference = $4
		`, schemaName), bankAccountID, date, amount, reference).Scan(&count)
		if err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}

	return false, nil
}

func parseAmount(s, decimalSep, thousandsSep string) (decimal.Decimal, error) {
	// Clean up the string
	s = strings.TrimSpace(s)

	// Remove currency symbols and spaces
	s = regexp.MustCompile(`[€$£\s]`).ReplaceAllString(s, "")

	// Handle thousands separator
	if thousandsSep != "" {
		s = strings.ReplaceAll(s, thousandsSep, "")
	}

	// Handle decimal separator
	if decimalSep != "." {
		s = strings.ReplaceAll(s, decimalSep, ".")
	}

	// Handle negative amounts in parentheses
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + s[1:len(s)-1]
	}

	return decimal.NewFromString(s)
}

// DetectCSVFormat attempts to detect the CSV format based on headers
func DetectCSVFormat(headers []string) CSVFormat {
	headerStr := strings.ToLower(strings.Join(headers, ","))

	if strings.Contains(headerStr, "kuupäev") && strings.Contains(headerStr, "summa") {
		// Estonian bank format
		if strings.Contains(headerStr, "saaja/maksja nimi") {
			return FormatSwedbankEE
		}
	}

	return FormatGeneric
}

// GetMappingForFormat returns the column mapping for a specific format
func GetMappingForFormat(format CSVFormat) CSVColumnMapping {
	switch format {
	case FormatSwedbankEE:
		return SwedbankEEMapping()
	default:
		return DefaultGenericMapping()
	}
}

// ParseCSVPreview parses the first few rows of a CSV for preview
func ParseCSVPreview(reader io.Reader, maxRows int) ([][]string, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	var rows [][]string
	for i := 0; i < maxRows; i++ {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, record)
	}

	return rows, nil
}

// ValidateCSVRow validates a single CSV row against the mapping
func ValidateCSVRow(record []string, mapping CSVColumnMapping) error {
	if mapping.DateColumn >= len(record) {
		return fmt.Errorf("date column %d out of range", mapping.DateColumn)
	}
	if mapping.AmountColumn >= len(record) {
		return fmt.Errorf("amount column %d out of range", mapping.AmountColumn)
	}

	// Validate date
	dateStr := strings.TrimSpace(record[mapping.DateColumn])
	if _, err := time.Parse(mapping.DateFormat, dateStr); err != nil {
		return fmt.Errorf("invalid date '%s': expected format %s", dateStr, mapping.DateFormat)
	}

	// Validate amount
	amountStr := strings.TrimSpace(record[mapping.AmountColumn])
	if _, err := parseAmount(amountStr, mapping.DecimalSeparator, mapping.ThousandsSeparator); err != nil {
		return fmt.Errorf("invalid amount '%s'", amountStr)
	}

	return nil
}

// FormatAmount formats a decimal amount as a string with thousands separators
func FormatAmount(amount decimal.Decimal, decimals int32) string {
	sign := ""
	if amount.IsNegative() {
		sign = "-"
		amount = amount.Abs()
	}

	str := amount.StringFixed(decimals)
	parts := strings.Split(str, ".")

	// Add thousands separators
	intPart := parts[0]
	// Pre-allocate: original length + estimated separators (length/3)
	result := make([]byte, 0, len(intPart)+(len(intPart)/3))
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	if len(parts) > 1 {
		return sign + string(result) + "." + parts[1]
	}
	return sign + string(result)
}

// ParseDateFormats tries to parse a date string using common formats
func ParseDateFormats(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"02.01.2006",
		"01/02/2006",
		"02/01/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		time.RFC3339,
	}

	dateStr = strings.TrimSpace(dateStr)

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// Helper to convert string to int with default
func parseIntOrDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
