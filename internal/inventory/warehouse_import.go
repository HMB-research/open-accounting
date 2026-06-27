package inventory

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

type warehouseImportRow struct {
	rowNumber int
	values    map[string]string
}

var warehouseImportHeaderAliases = map[string]string{
	"code":              "code",
	"warehouse_code":    "code",
	"location_code":     "code",
	"storage_code":      "code",
	"name":              "name",
	"warehouse_name":    "name",
	"location_name":     "name",
	"storage_name":      "name",
	"address":           "address",
	"location":          "address",
	"site_address":      "address",
	"is_default":        "is_default",
	"default":           "is_default",
	"default_warehouse": "is_default",
	"is_active":         "is_active",
	"active":            "is_active",
	"status":            "status",
}

// ImportWarehousesCSV imports warehouse master data from CSV content.
func (s *Service) ImportWarehousesCSV(ctx context.Context, tenantID, schemaName string, req *ImportWarehousesRequest) (*ImportWarehousesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseWarehouseImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no warehouses found in CSV")
	}

	existingWarehouses, err := s.repo.ListWarehouses(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing warehouses: %w", err)
	}
	usedCodes := make(map[string]string, len(existingWarehouses)+len(rows))
	for _, warehouse := range existingWarehouses {
		key := normalizedProductImportKey(warehouse.Code)
		if key == "" {
			continue
		}
		usedCodes[key] = warehouse.Name
	}

	result := &ImportWarehousesResult{
		FileName: req.FileName,
		Errors:   []ImportWarehousesRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		warehouse, err := buildWarehouseFromImportRow(row, tenantID)
		if err != nil {
			appendWarehouseImportRowError(result, row, err)
			continue
		}

		codeKey := normalizedProductImportKey(warehouse.Code)
		if existingName, exists := usedCodes[codeKey]; exists {
			appendWarehouseImportRowError(result, row, fmt.Errorf("duplicate code %q matches existing warehouse %q", warehouse.Code, existingName))
			continue
		}

		if err := s.repo.CreateWarehouse(ctx, schemaName, warehouse); err != nil {
			appendWarehouseImportRowError(result, row, err)
			continue
		}

		usedCodes[codeKey] = warehouse.Name
		result.WarehousesCreated++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func appendWarehouseImportRowError(result *ImportWarehousesResult, row warehouseImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportWarehousesRowError{
		Row:     row.rowNumber,
		Code:    strings.TrimSpace(row.values["code"]),
		Name:    strings.TrimSpace(row.values["name"]),
		Message: err.Error(),
	})
}

func parseWarehouseImportRows(content string) ([]warehouseImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectProductImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	required := map[string]bool{
		"code": false,
		"name": false,
	}
	for i, header := range headers {
		canonicalHeaders[i] = canonicalWarehouseImportHeader(header)
		if _, ok := required[canonicalHeaders[i]]; ok {
			required[canonicalHeaders[i]] = true
		}
	}

	var missing []string
	for _, name := range []string{"code", "name"} {
		if !required[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}

	rows := make([]warehouseImportRow, 0)
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
		rows = append(rows, warehouseImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildWarehouseFromImportRow(row warehouseImportRow, tenantID string) (*Warehouse, error) {
	code := strings.TrimSpace(row.values["code"])
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	isDefault := false
	if rawDefault := strings.TrimSpace(row.values["is_default"]); rawDefault != "" {
		parsedDefault, err := parseProductImportBool("is_default", rawDefault)
		if err != nil {
			return nil, err
		}
		isDefault = parsedDefault
	}
	isActive, err := parseWarehouseImportActive(row.values["status"], row.values["is_active"])
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Code:      code,
		Name:      name,
		Address:   strings.TrimSpace(row.values["address"]),
		IsDefault: isDefault,
		IsActive:  isActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func parseWarehouseImportActive(statusValue, activeValue string) (bool, error) {
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
	return parseProductImportBool("is_active", activeValue)
}

func canonicalWarehouseImportHeader(header string) string {
	normalized := normalizedProductImportHeader(header)
	if canonical, ok := warehouseImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}
