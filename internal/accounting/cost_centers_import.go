package accounting

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type costCenterImportRow struct {
	rowNumber int
	values    map[string]string
}

type costCenterImportPending struct {
	row        costCenterImportRow
	req        *CreateCostCenterRequest
	parentCode string
}

var costCenterImportHeaderAliases = map[string]string{
	"code":                    "code",
	"cost_center_code":        "code",
	"cc_code":                 "code",
	"name":                    "name",
	"cost_center_name":        "name",
	"cc_name":                 "name",
	"description":             "description",
	"parent_id":               "parent_id",
	"parent_code":             "parent_code",
	"parent":                  "parent_code",
	"parent_cost_center_code": "parent_code",
	"budget_amount":           "budget_amount",
	"budget":                  "budget_amount",
	"budget_period":           "budget_period",
	"is_active":               "is_active",
	"active":                  "is_active",
	"status":                  "status",
}

// ImportCostCentersCSV imports cost center master data from CSV content.
func (s *CostCenterService) ImportCostCentersCSV(ctx context.Context, schemaName, tenantID string, req *ImportCostCentersRequest) (*ImportCostCentersResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseCostCenterImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no cost centers found in CSV")
	}

	existingCostCenters, err := s.repo.List(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing cost centers: %w", err)
	}
	codeToID := make(map[string]string, len(existingCostCenters)+len(rows))
	usedCodes := make(map[string]string, len(existingCostCenters)+len(rows))
	for _, costCenter := range existingCostCenters {
		key := normalizedAccountImportKey(costCenter.Code)
		if key == "" {
			continue
		}
		codeToID[key] = costCenter.ID
		usedCodes[key] = costCenter.Name
	}

	result := &ImportCostCentersResult{
		FileName: req.FileName,
		Errors:   []ImportCostCentersRowError{},
	}

	pending := make([]costCenterImportPending, 0, len(rows))
	for _, row := range rows {
		result.RowsProcessed++

		createReq, parentCode, err := buildCreateCostCenterRequestFromImportRow(row)
		if err != nil {
			appendCostCenterImportRowError(result, row, err)
			continue
		}

		codeKey := normalizedAccountImportKey(createReq.Code)
		if existingName, exists := usedCodes[codeKey]; exists {
			appendCostCenterImportRowError(result, row, fmt.Errorf("duplicate code %q matches existing cost center %q", createReq.Code, existingName))
			continue
		}

		usedCodes[codeKey] = createReq.Name
		pending = append(pending, costCenterImportPending{
			row:        row,
			req:        createReq,
			parentCode: parentCode,
		})
	}

	for len(pending) > 0 {
		progressed := false
		remaining := make([]costCenterImportPending, 0, len(pending))

		for _, item := range pending {
			if item.parentCode != "" {
				parentID, ok := codeToID[normalizedAccountImportKey(item.parentCode)]
				if !ok {
					remaining = append(remaining, item)
					continue
				}
				item.req.ParentID = &parentID
			}

			costCenter, err := s.CreateCostCenter(ctx, schemaName, tenantID, item.req)
			if err != nil {
				appendCostCenterImportRowError(result, item.row, err)
				continue
			}

			codeToID[normalizedAccountImportKey(costCenter.Code)] = costCenter.ID
			result.CostCentersCreated++
			progressed = true
		}

		if progressed {
			pending = remaining
			continue
		}

		for _, item := range remaining {
			appendCostCenterImportRowError(result, item.row, fmt.Errorf("parent_code %q was not found", item.parentCode))
		}
		break
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func appendCostCenterImportRowError(result *ImportCostCentersResult, row costCenterImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportCostCentersRowError{
		Row:     row.rowNumber,
		Code:    strings.TrimSpace(row.values["code"]),
		Name:    strings.TrimSpace(row.values["name"]),
		Message: err.Error(),
	})
}

func parseCostCenterImportRows(content string) ([]costCenterImportRow, error) {
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
	hasCodeColumn := false
	hasNameColumn := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalCostCenterImportHeader(header)
		switch canonicalHeaders[i] {
		case "code":
			hasCodeColumn = true
		case "name":
			hasNameColumn = true
		}
	}
	if !hasCodeColumn || !hasNameColumn {
		return nil, fmt.Errorf("missing required columns: code, name")
	}

	rows := make([]costCenterImportRow, 0)
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
		rows = append(rows, costCenterImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildCreateCostCenterRequestFromImportRow(row costCenterImportRow) (*CreateCostCenterRequest, string, error) {
	code := strings.TrimSpace(row.values["code"])
	if code == "" {
		return nil, "", fmt.Errorf("code is required")
	}
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, "", fmt.Errorf("name is required")
	}

	parentCode := strings.TrimSpace(row.values["parent_code"])
	if parentCode != "" && normalizedAccountImportKey(parentCode) == normalizedAccountImportKey(code) {
		return nil, "", fmt.Errorf("parent_code cannot match code")
	}

	budgetAmount, err := parseCostCenterImportBudgetAmount(row.values["budget_amount"])
	if err != nil {
		return nil, "", err
	}
	budgetPeriod, err := parseCostCenterImportBudgetPeriod(row.values["budget_period"])
	if err != nil {
		return nil, "", err
	}
	isActive, err := parseCostCenterImportActive(row.values["status"], row.values["is_active"])
	if err != nil {
		return nil, "", err
	}
	parentID, err := optionalCostCenterImportParentID(row.values["parent_id"])
	if err != nil {
		return nil, "", err
	}

	return &CreateCostCenterRequest{
		Code:         code,
		Name:         name,
		Description:  strings.TrimSpace(row.values["description"]),
		ParentID:     parentID,
		IsActive:     isActive,
		BudgetAmount: budgetAmount,
		BudgetPeriod: budgetPeriod,
	}, parentCode, nil
}

func optionalCostCenterImportParentID(value string) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parent_id must be a valid UUID")
	}
	canonicalID := parsedID.String()
	return &canonicalID, nil
}

func parseCostCenterImportBudgetAmount(value string) (*decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return nil, fmt.Errorf("budget_amount must be a decimal")
	}
	if parsed.IsNegative() {
		return nil, fmt.Errorf("budget_amount cannot be negative")
	}
	return &parsed, nil
}

func parseCostCenterImportBudgetPeriod(value string) (BudgetPeriod, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return BudgetPeriodAnnual, nil
	}
	switch BudgetPeriod(normalized) {
	case BudgetPeriodMonthly, BudgetPeriodQuarterly, BudgetPeriodAnnual:
		return BudgetPeriod(normalized), nil
	default:
		return "", fmt.Errorf("invalid budget_period %q", value)
	}
}

func parseCostCenterImportActive(statusValue, activeValue string) (bool, error) {
	status := strings.ToUpper(strings.TrimSpace(statusValue))
	if status != "" {
		switch status {
		case "ACTIVE":
			return true, nil
		case "INACTIVE":
			return false, nil
		default:
			return false, fmt.Errorf("invalid status %q", statusValue)
		}
	}
	if strings.TrimSpace(activeValue) == "" {
		return true, nil
	}
	return parseCostCenterImportBool("is_active", activeValue)
}

func parseCostCenterImportBool(field, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "yes", "y", "1":
		return true, nil
	case "false", "f", "no", "n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", field)
	}
}

func canonicalCostCenterImportHeader(header string) string {
	normalized := normalizedAccountImportHeader(header)
	if canonical, ok := costCenterImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}
