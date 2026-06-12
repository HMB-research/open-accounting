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

type costAllocationImportRow struct {
	rowNumber int
	values    map[string]string
}

var costAllocationImportHeaderAliases = map[string]string{
	"cost_center_id":        "cost_center_id",
	"cost_center":           "cost_center_code",
	"cost_center_code":      "cost_center_code",
	"cc_code":               "cost_center_code",
	"journal_entry_line_id": "journal_entry_line_id",
	"journal_line_id":       "journal_entry_line_id",
	"line_id":               "journal_entry_line_id",
	"amount":                "amount",
	"allocation_amount":     "amount",
	"allocation_percentage": "allocation_percentage",
	"percentage":            "allocation_percentage",
	"allocation_percent":    "allocation_percentage",
	"allocation_date":       "allocation_date",
	"date":                  "allocation_date",
	"notes":                 "notes",
	"note":                  "notes",
	"memo":                  "notes",
	"description":           "notes",
}

// ImportCostAllocationsCSV imports historical cost allocation rows from CSV content.
func (s *CostCenterService) ImportCostAllocationsCSV(ctx context.Context, schemaName, tenantID string, req *ImportCostAllocationsRequest) (*ImportCostAllocationsResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseCostAllocationImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no cost allocations found in CSV")
	}

	costCenters, err := s.repo.List(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list cost centers: %w", err)
	}
	costCenterCodeToID := make(map[string]string, len(costCenters))
	for _, costCenter := range costCenters {
		key := normalizedAccountImportKey(costCenter.Code)
		if key != "" {
			costCenterCodeToID[key] = costCenter.ID
		}
	}

	result := &ImportCostAllocationsResult{
		FileName: req.FileName,
		Errors:   []ImportCostAllocationsRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		createReq, err := buildCreateCostAllocationRequestFromImportRow(row, costCenterCodeToID)
		if err != nil {
			appendCostAllocationImportRowError(result, row, err)
			continue
		}

		if _, err := s.CreateCostAllocation(ctx, schemaName, tenantID, createReq); err != nil {
			appendCostAllocationImportRowError(result, row, err)
			continue
		}
		result.AllocationsImported++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseCostAllocationImportRows(content string) ([]costAllocationImportRow, error) {
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
	hasCostCenterIDColumn := false
	hasCostCenterCodeColumn := false
	hasJournalLineColumn := false
	hasAmountColumn := false
	hasDateColumn := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalCostAllocationImportHeader(header)
		switch canonicalHeaders[i] {
		case "cost_center_id":
			hasCostCenterIDColumn = true
		case "cost_center_code":
			hasCostCenterCodeColumn = true
		case "journal_entry_line_id":
			hasJournalLineColumn = true
		case "amount":
			hasAmountColumn = true
		case "allocation_date":
			hasDateColumn = true
		}
	}
	if !hasCostCenterIDColumn && !hasCostCenterCodeColumn {
		return nil, fmt.Errorf("missing required columns: cost_center_id or cost_center_code")
	}
	if !hasJournalLineColumn || !hasAmountColumn || !hasDateColumn {
		return nil, fmt.Errorf("missing required columns: journal_entry_line_id, amount, allocation_date")
	}

	rows := make([]costAllocationImportRow, 0)
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
		rows = append(rows, costAllocationImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildCreateCostAllocationRequestFromImportRow(row costAllocationImportRow, costCenterCodeToID map[string]string) (*CreateCostAllocationRequest, error) {
	costCenterID := strings.TrimSpace(row.values["cost_center_id"])
	costCenterCode := strings.TrimSpace(row.values["cost_center_code"])
	if costCenterID != "" {
		parsedID, err := uuid.Parse(costCenterID)
		if err != nil {
			return nil, fmt.Errorf("cost_center_id must be a valid UUID")
		}
		costCenterID = parsedID.String()
	} else {
		if costCenterCode == "" {
			return nil, fmt.Errorf("cost_center_id or cost_center_code is required")
		}
		resolvedID, ok := costCenterCodeToID[normalizedAccountImportKey(costCenterCode)]
		if !ok {
			return nil, fmt.Errorf("cost_center_code %q was not found", costCenterCode)
		}
		costCenterID = resolvedID
	}

	journalEntryLineID := strings.TrimSpace(row.values["journal_entry_line_id"])
	if journalEntryLineID == "" {
		return nil, fmt.Errorf("journal_entry_line_id is required")
	}

	amount, err := parseCostAllocationImportPositiveDecimal("amount", row.values["amount"])
	if err != nil {
		return nil, err
	}

	allocationPercentage, err := parseCostAllocationImportPercentage(row.values["allocation_percentage"])
	if err != nil {
		return nil, err
	}

	allocationDate, err := parseCostAllocationImportDate(row.values["allocation_date"])
	if err != nil {
		return nil, err
	}

	return &CreateCostAllocationRequest{
		CostCenterID:         costCenterID,
		JournalEntryLineID:   journalEntryLineID,
		Amount:               amount,
		AllocationPercentage: allocationPercentage,
		AllocationDate:       allocationDate,
		Notes:                strings.TrimSpace(row.values["notes"]),
	}, nil
}

func appendCostAllocationImportRowError(result *ImportCostAllocationsResult, row costAllocationImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportCostAllocationsRowError{
		Row:                row.rowNumber,
		CostCenterID:       strings.TrimSpace(row.values["cost_center_id"]),
		CostCenterCode:     strings.TrimSpace(row.values["cost_center_code"]),
		JournalEntryLineID: strings.TrimSpace(row.values["journal_entry_line_id"]),
		Message:            err.Error(),
	})
}

func parseCostAllocationImportPositiveDecimal(field, value string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, fmt.Errorf("%s is required", field)
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s must be a decimal", field)
	}
	if !parsed.GreaterThan(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("%s must be greater than zero", field)
	}
	return parsed, nil
}

func parseCostAllocationImportPercentage(value string) (*decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("allocation_percentage must be a decimal")
	}
	if parsed.LessThan(decimal.Zero) || parsed.GreaterThan(decimal.NewFromInt(100)) {
		return nil, fmt.Errorf("allocation_percentage must be between 0 and 100")
	}
	return &parsed, nil
}

func parseCostAllocationImportDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("allocation_date is required")
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("allocation_date must use YYYY-MM-DD")
	}
	return parsed, nil
}

func canonicalCostAllocationImportHeader(header string) string {
	normalized := normalizedAccountImportHeader(header)
	if canonical, ok := costAllocationImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}
