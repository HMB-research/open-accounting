package tax

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type kmdHistoryImportRow struct {
	rowNumber int
	values    map[string]string
}

type kmdHistoryImportRecord struct {
	rowNumber      int
	year           int
	month          int
	status         string
	submittedAt    *time.Time
	row            KMDRow
	totalOutputVAT *decimal.Decimal
	totalInputVAT  *decimal.Decimal
}

type kmdHistoryImportGroup struct {
	year           int
	month          int
	status         string
	submittedAt    *time.Time
	totalOutputVAT *decimal.Decimal
	totalInputVAT  *decimal.Decimal
	records        []*kmdHistoryImportRecord
}

type kmdHistoryVATSupport struct {
	outputSupport    decimal.Decimal
	outputSupportSet bool
	inputSupport     decimal.Decimal
	inputSupportSet  bool
	outputTotalRow   *decimal.Decimal
	inputTotalRow    *decimal.Decimal
}

var kmdHistoryImportHeaderAliases = map[string]string{
	"year":               "year",
	"period_year":        "year",
	"declaration_year":   "year",
	"kmd_year":           "year",
	"month":              "month",
	"period_month":       "month",
	"declaration_month":  "month",
	"kmd_month":          "month",
	"status":             "status",
	"declaration_status": "status",
	"submitted_at":       "submitted_at",
	"submitted_date":     "submitted_at",
	"submission_date":    "submitted_at",
	"row_code":           "row_code",
	"code":               "row_code",
	"kmd_row":            "row_code",
	"kmd_code":           "row_code",
	"description":        "description",
	"row_description":    "description",
	"tax_base":           "tax_base",
	"base":               "tax_base",
	"taxable_amount":     "tax_base",
	"tax_amount":         "tax_amount",
	"vat_amount":         "tax_amount",
	"amount":             "tax_amount",
	"total_output_vat":   "total_output_vat",
	"output_vat":         "total_output_vat",
	"total_input_vat":    "total_input_vat",
	"input_vat":          "total_input_vat",
}

var kmdHistoryStatusAliases = map[string]string{
	"":          "ACCEPTED",
	"draft":     "DRAFT",
	"submitted": "SUBMITTED",
	"filed":     "SUBMITTED",
	"accepted":  "ACCEPTED",
	"approved":  "ACCEPTED",
	"confirmed": "ACCEPTED",
}

// ImportKMDHistoryCSV imports historical KMD declarations from CSV rows.
func (s *Service) ImportKMDHistoryCSV(
	ctx context.Context,
	schemaName, tenantID string,
	req *ImportKMDHistoryRequest,
) (*ImportKMDHistoryResult, error) {
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseKMDHistoryImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no KMD rows found in CSV")
	}

	result := &ImportKMDHistoryResult{
		FileName: req.FileName,
		Errors:   []ImportKMDHistoryRowError{},
	}

	groups := make(map[string]*kmdHistoryImportGroup)
	for _, row := range rows {
		result.RowsProcessed++

		record, err := buildKMDHistoryImportRecord(row)
		if err != nil {
			appendKMDHistoryRowError(result, row, nil, err.Error())
			continue
		}

		key := kmdHistoryGroupKey(record.year, record.month)
		group, ok := groups[key]
		if !ok {
			group = &kmdHistoryImportGroup{
				year:           record.year,
				month:          record.month,
				status:         record.status,
				submittedAt:    record.submittedAt,
				totalOutputVAT: record.totalOutputVAT,
				totalInputVAT:  record.totalInputVAT,
				records:        []*kmdHistoryImportRecord{},
			}
			groups[key] = group
		}

		if message := validateKMDHistoryGroupConsistency(group, record); message != "" {
			appendKMDHistoryRowError(result, row, record, message)
			continue
		}

		group.records = append(group.records, record)
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	for _, key := range groupKeys {
		group := groups[key]
		if message := validateKMDHistoryVATReconciliation(group); message != "" {
			appendKMDHistoryGroupError(result, group, message)
			continue
		}

		existing, err := s.repo.GetDeclaration(ctx, schemaName, tenantID, group.year, group.month)
		if err != nil {
			return nil, fmt.Errorf("check existing KMD declaration for %04d-%02d: %w", group.year, group.month, err)
		}
		if existing != nil {
			for _, record := range group.records {
				appendKMDHistoryRowError(
					result,
					kmdHistoryImportRow{rowNumber: record.rowNumber},
					record,
					fmt.Sprintf("KMD declaration already exists for %04d-%02d", group.year, group.month),
				)
			}
			continue
		}

		declaration := buildKMDHistoryDeclaration(tenantID, group)
		if err := s.repo.SaveDeclaration(ctx, schemaName, declaration); err != nil {
			for _, record := range group.records {
				appendKMDHistoryRowError(
					result,
					kmdHistoryImportRow{rowNumber: record.rowNumber},
					record,
					fmt.Sprintf("save KMD declaration: %v", err),
				)
			}
			continue
		}

		result.DeclarationsCreated++
		result.RowsImported += len(group.records)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseKMDHistoryImportRows(content string) ([]kmdHistoryImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectKMDHistoryImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	hasYear := false
	hasMonth := false
	hasCode := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalKMDHistoryImportHeader(header)
		switch canonicalHeaders[i] {
		case "year":
			hasYear = true
		case "month":
			hasMonth = true
		case "row_code":
			hasCode = true
		}
	}
	if !hasYear || !hasMonth || !hasCode {
		return nil, fmt.Errorf("missing required year, month, or row_code column")
	}

	rows := make([]kmdHistoryImportRow, 0)
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

		rows = append(rows, kmdHistoryImportRow{rowNumber: rowNumber, values: values})
	}

	return rows, nil
}

func buildKMDHistoryImportRecord(row kmdHistoryImportRow) (*kmdHistoryImportRecord, error) {
	year, err := parseKMDHistoryImportYear(row.values["year"])
	if err != nil {
		return nil, err
	}
	month, err := parseKMDHistoryImportMonth(row.values["month"])
	if err != nil {
		return nil, err
	}
	status, err := parseKMDHistoryStatus(row.values["status"])
	if err != nil {
		return nil, err
	}

	var submittedAt *time.Time
	if value := strings.TrimSpace(row.values["submitted_at"]); value != "" {
		parsed, err := parseKMDHistoryImportDate(value, "submitted_at")
		if err != nil {
			return nil, err
		}
		submittedAt = &parsed
	}

	rowCode := normalizeKMDHistoryRowCode(row.values["row_code"])
	if rowCode == "" {
		return nil, fmt.Errorf("row_code is required")
	}

	taxBase, err := parseOptionalKMDHistoryDecimal(row.values["tax_base"], "tax_base")
	if err != nil {
		return nil, err
	}
	taxAmount, err := parseOptionalKMDHistoryDecimal(row.values["tax_amount"], "tax_amount")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.values["tax_base"]) == "" && strings.TrimSpace(row.values["tax_amount"]) == "" {
		return nil, fmt.Errorf("tax_base or tax_amount is required")
	}

	description := strings.TrimSpace(row.values["description"])
	if description == "" {
		description = getKMDRowDescription(rowCode)
	}

	totalOutputVAT, err := parseOptionalKMDHistoryDecimalPointer(row.values["total_output_vat"], "total_output_vat")
	if err != nil {
		return nil, err
	}
	totalInputVAT, err := parseOptionalKMDHistoryDecimalPointer(row.values["total_input_vat"], "total_input_vat")
	if err != nil {
		return nil, err
	}

	return &kmdHistoryImportRecord{
		rowNumber:   row.rowNumber,
		year:        year,
		month:       month,
		status:      status,
		submittedAt: submittedAt,
		row: KMDRow{
			Code:        rowCode,
			Description: description,
			TaxBase:     taxBase,
			TaxAmount:   taxAmount,
		},
		totalOutputVAT: totalOutputVAT,
		totalInputVAT:  totalInputVAT,
	}, nil
}

func validateKMDHistoryGroupConsistency(group *kmdHistoryImportGroup, record *kmdHistoryImportRecord) string {
	if group.status != record.status {
		return fmt.Sprintf("status must match other rows for %04d-%02d", group.year, group.month)
	}
	if group.submittedAt != nil && record.submittedAt != nil && !group.submittedAt.Equal(*record.submittedAt) {
		return fmt.Sprintf("submitted_at must match other rows for %04d-%02d", group.year, group.month)
	}
	if group.submittedAt == nil && record.submittedAt != nil {
		group.submittedAt = record.submittedAt
	}
	if message := validateKMDHistoryDecimalPointer("total_output_vat", group.totalOutputVAT, record.totalOutputVAT, group.year, group.month); message != "" {
		return message
	}
	if message := validateKMDHistoryDecimalPointer("total_input_vat", group.totalInputVAT, record.totalInputVAT, group.year, group.month); message != "" {
		return message
	}
	if group.totalOutputVAT == nil && record.totalOutputVAT != nil {
		group.totalOutputVAT = record.totalOutputVAT
	}
	if group.totalInputVAT == nil && record.totalInputVAT != nil {
		group.totalInputVAT = record.totalInputVAT
	}
	return ""
}

func validateKMDHistoryDecimalPointer(field string, groupValue, recordValue *decimal.Decimal, year, month int) string {
	if groupValue == nil || recordValue == nil {
		return ""
	}
	if !groupValue.Equal(*recordValue) {
		return fmt.Sprintf("%s must match other rows for %04d-%02d", field, year, month)
	}
	return ""
}

func buildKMDHistoryDeclaration(tenantID string, group *kmdHistoryImportGroup) *KMDDeclaration {
	rowMap := make(map[string]*KMDRow)
	for _, record := range group.records {
		row := record.row
		existing, ok := rowMap[row.Code]
		if !ok {
			rowCopy := row
			rowMap[row.Code] = &rowCopy
			continue
		}
		existing.TaxBase = existing.TaxBase.Add(row.TaxBase)
		existing.TaxAmount = existing.TaxAmount.Add(row.TaxAmount)
		if existing.Description == "Unknown" && row.Description != "" {
			existing.Description = row.Description
		}
	}

	rows := make([]KMDRow, 0, len(rowMap))
	vatSupport := kmdHistoryGroupVATSupport(group.records)
	totalOutput := vatSupport.outputSupport
	totalInput := vatSupport.inputSupport
	for _, row := range rowMap {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return kmdHistoryRowSortKey(rows[i].Code) < kmdHistoryRowSortKey(rows[j].Code)
	})

	if vatSupport.outputTotalRow != nil {
		totalOutput = *vatSupport.outputTotalRow
	}
	if vatSupport.inputTotalRow != nil {
		totalInput = *vatSupport.inputTotalRow
	}
	if group.totalOutputVAT != nil {
		totalOutput = *group.totalOutputVAT
	}
	if group.totalInputVAT != nil {
		totalInput = *group.totalInputVAT
	}

	now := time.Now().UTC()
	return &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		Year:           group.year,
		Month:          group.month,
		Status:         group.status,
		TotalOutputVAT: totalOutput,
		TotalInputVAT:  totalInput,
		Rows:           rows,
		SubmittedAt:    group.submittedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func validateKMDHistoryVATReconciliation(group *kmdHistoryImportGroup) string {
	vatSupport := kmdHistoryGroupVATSupport(group.records)
	period := kmdHistoryGroupKey(group.year, group.month)
	if vatSupport.outputSupportSet {
		if vatSupport.outputTotalRow != nil && !vatSupport.outputTotalRow.Equal(vatSupport.outputSupport) {
			return fmt.Sprintf(
				"KMD row 8 tax_amount %s does not match supporting KMD output VAT rows for %s: supporting total %s",
				vatSupport.outputTotalRow.String(),
				period,
				vatSupport.outputSupport.String(),
			)
		}
		if group.totalOutputVAT != nil && !group.totalOutputVAT.Equal(vatSupport.outputSupport) {
			return fmt.Sprintf(
				"total_output_vat %s does not match supporting KMD output VAT rows for %s: supporting total %s",
				group.totalOutputVAT.String(),
				period,
				vatSupport.outputSupport.String(),
			)
		}
	} else if group.totalOutputVAT != nil && vatSupport.outputTotalRow != nil && !group.totalOutputVAT.Equal(*vatSupport.outputTotalRow) {
		return fmt.Sprintf(
			"total_output_vat %s does not match KMD row 8 tax_amount for %s: row total %s",
			group.totalOutputVAT.String(),
			period,
			vatSupport.outputTotalRow.String(),
		)
	}
	if vatSupport.inputSupportSet {
		if vatSupport.inputTotalRow != nil && !vatSupport.inputTotalRow.Equal(vatSupport.inputSupport) {
			return fmt.Sprintf(
				"KMD row 9 tax_amount %s does not match supporting KMD input VAT rows for %s: supporting total %s",
				vatSupport.inputTotalRow.String(),
				period,
				vatSupport.inputSupport.String(),
			)
		}
		if group.totalInputVAT != nil && !group.totalInputVAT.Equal(vatSupport.inputSupport) {
			return fmt.Sprintf(
				"total_input_vat %s does not match supporting KMD input VAT rows for %s: supporting total %s",
				group.totalInputVAT.String(),
				period,
				vatSupport.inputSupport.String(),
			)
		}
	} else if group.totalInputVAT != nil && vatSupport.inputTotalRow != nil && !group.totalInputVAT.Equal(*vatSupport.inputTotalRow) {
		return fmt.Sprintf(
			"total_input_vat %s does not match KMD row 9 tax_amount for %s: row total %s",
			group.totalInputVAT.String(),
			period,
			vatSupport.inputTotalRow.String(),
		)
	}
	return ""
}

func kmdHistoryGroupVATSupport(records []*kmdHistoryImportRecord) kmdHistoryVATSupport {
	var support kmdHistoryVATSupport
	for _, record := range records {
		switch kmdHistoryVATSupportClass(record.row.Code) {
		case "output":
			support.outputSupport = support.outputSupport.Add(record.row.TaxAmount)
			support.outputSupportSet = true
		case "input":
			support.inputSupport = support.inputSupport.Add(record.row.TaxAmount)
			support.inputSupportSet = true
		case "output_total":
			total := record.row.TaxAmount
			support.outputTotalRow = &total
		case "input_total":
			total := record.row.TaxAmount
			support.inputTotalRow = &total
		}
	}
	return support
}

func appendKMDHistoryGroupError(result *ImportKMDHistoryResult, group *kmdHistoryImportGroup, message string) {
	for _, record := range group.records {
		appendKMDHistoryRowError(result, kmdHistoryImportRow{rowNumber: record.rowNumber}, record, message)
	}
}

func appendKMDHistoryRowError(result *ImportKMDHistoryResult, row kmdHistoryImportRow, record *kmdHistoryImportRecord, message string) {
	result.RowsSkipped++
	rowError := ImportKMDHistoryRowError{
		Row:     row.rowNumber,
		Message: message,
	}
	if record != nil {
		rowError.Year = record.year
		rowError.Month = record.month
		rowError.RowCode = record.row.Code
		rowError.Description = record.row.Description
	} else {
		rowError.Year = parseOptionalKMDHistoryInt(row.values["year"])
		rowError.Month = parseOptionalKMDHistoryInt(row.values["month"])
		rowError.RowCode = normalizeKMDHistoryRowCode(row.values["row_code"])
		rowError.Description = strings.TrimSpace(row.values["description"])
	}
	result.Errors = append(result.Errors, rowError)
}

func canonicalKMDHistoryImportHeader(header string) string {
	normalized := strings.ToLower(strings.TrimSpace(header))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if canonical, ok := kmdHistoryImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return ""
}

func detectKMDHistoryImportDelimiter(content string) rune {
	firstLine := content
	if index := strings.IndexRune(content, '\n'); index >= 0 {
		firstLine = content[:index]
	}

	candidates := []rune{',', ';', '\t'}
	bestDelimiter := ','
	bestCount := -1
	for _, candidate := range candidates {
		count := strings.Count(firstLine, string(candidate))
		if count > bestCount {
			bestCount = count
			bestDelimiter = candidate
		}
	}
	return bestDelimiter
}

func parseKMDHistoryImportYear(value string) (int, error) {
	year, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || year < 1900 || year > 2200 {
		return 0, fmt.Errorf("year must be between 1900 and 2200")
	}
	return year, nil
}

func parseKMDHistoryImportMonth(value string) (int, error) {
	month, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || month < 1 || month > 12 {
		return 0, fmt.Errorf("month must be between 1 and 12")
	}
	return month, nil
}

func parseKMDHistoryStatus(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "-", "_")))
	if status, ok := kmdHistoryStatusAliases[normalized]; ok {
		return status, nil
	}
	return "", fmt.Errorf("invalid status %q", value)
}

func parseOptionalKMDHistoryDecimalPointer(value, field string) (*decimal.Decimal, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseOptionalKMDHistoryDecimal(value, field)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalKMDHistoryDecimal(value, field string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, nil
	}
	parsed, err := decimal.NewFromString(normalizeKMDHistoryDecimal(trimmed))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func parseKMDHistoryImportDate(value, field string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		"02.01.2006",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format", field)
}

func parseOptionalKMDHistoryInt(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func normalizeKMDHistoryDecimal(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	if strings.Contains(normalized, ",") && !strings.Contains(normalized, ".") {
		normalized = strings.ReplaceAll(normalized, ",", ".")
	}
	return normalized
}

func normalizeKMDHistoryRowCode(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "row_"))
}

func kmdHistoryGroupKey(year, month int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}

func kmdHistoryVATSupportClass(code string) string {
	switch code {
	case KMDRow1, KMDRow2, KMDRow21, KMDRow3, KMDRow31:
		return "output"
	case KMDRow4, KMDRow5, KMDRow6, KMDRow7:
		return "input"
	case KMDRow8:
		return "output_total"
	case KMDRow9:
		return "input_total"
	default:
		return ""
	}
}

func kmdHistoryRowSortKey(code string) string {
	if value, err := strconv.Atoi(code); err == nil {
		return fmt.Sprintf("%03d", value)
	}
	return code
}
