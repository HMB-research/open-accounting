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

type categoryImportRow struct {
	rowNumber int
	values    map[string]string
}

var categoryImportHeaderAliases = map[string]string{
	"id":                  "id",
	"category_id":         "id",
	"product_category_id": "id",
	"name":                "name",
	"category":            "name",
	"category_name":       "name",
	"product_category":    "name",
	"description":         "description",
	"parent_id":           "parent_id",
	"parent_category_id":  "parent_id",
	"parent":              "parent_name",
	"parent_name":         "parent_name",
	"parent_category":     "parent_name",
}

// ImportProductCategoriesCSV imports product category master data from CSV content.
func (s *Service) ImportProductCategoriesCSV(ctx context.Context, tenantID, schemaName string, req *ImportProductCategoriesRequest) (*ImportProductCategoriesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseCategoryImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no product categories found in CSV")
	}

	existingCategories, err := s.repo.ListCategories(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list existing product categories: %w", err)
	}
	nameToID := make(map[string]string, len(existingCategories)+len(rows))
	for _, category := range existingCategories {
		key := normalizedProductImportKey(category.Name)
		if key == "" {
			continue
		}
		nameToID[key] = category.ID
	}

	result := &ImportProductCategoriesResult{
		FileName: req.FileName,
		Errors:   []ImportProductCategoriesRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		category, err := buildCategoryFromImportRow(row, tenantID, nameToID)
		if err != nil {
			appendCategoryImportRowError(result, row, err)
			continue
		}

		nameKey := normalizedProductImportKey(category.Name)
		if existingID, exists := nameToID[nameKey]; exists {
			appendCategoryImportRowError(result, row, fmt.Errorf("duplicate name %q matches existing category %q", category.Name, existingID))
			continue
		}

		if err := s.repo.CreateCategory(ctx, schemaName, category); err != nil {
			appendCategoryImportRowError(result, row, err)
			continue
		}

		nameToID[nameKey] = category.ID
		result.CategoriesCreated++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func appendCategoryImportRowError(result *ImportProductCategoriesResult, row categoryImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportProductCategoriesRowError{
		Row:     row.rowNumber,
		Name:    strings.TrimSpace(row.values["name"]),
		Message: err.Error(),
	})
}

func parseCategoryImportRows(content string) ([]categoryImportRow, error) {
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
	hasName := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalCategoryImportHeader(header)
		if canonicalHeaders[i] == "name" {
			hasName = true
		}
	}
	if !hasName {
		return nil, fmt.Errorf("missing required columns: name")
	}

	rows := make([]categoryImportRow, 0)
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
		rows = append(rows, categoryImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildCategoryFromImportRow(row categoryImportRow, tenantID string, nameToID map[string]string) (*ProductCategory, error) {
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	id := strings.TrimSpace(row.values["id"])
	if id != "" {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("invalid id")
		}
		id = parsedID.String()
	} else {
		id = uuid.New().String()
	}

	parentID, err := resolveCategoryImportParentID(row, nameToID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &ProductCategory{
		ID:          id,
		TenantID:    tenantID,
		Name:        name,
		Description: strings.TrimSpace(row.values["description"]),
		ParentID:    parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func resolveCategoryImportParentID(row categoryImportRow, nameToID map[string]string) (string, error) {
	if parentID := strings.TrimSpace(row.values["parent_id"]); parentID != "" {
		parsedID, err := uuid.Parse(parentID)
		if err != nil {
			return "", fmt.Errorf("parent_id must be a valid UUID")
		}
		return parsedID.String(), nil
	}
	parentName := strings.TrimSpace(row.values["parent_name"])
	if parentName == "" {
		return "", nil
	}
	parentID, ok := nameToID[normalizedProductImportKey(parentName)]
	if !ok {
		return "", fmt.Errorf("parent_name %q was not found", parentName)
	}
	return parentID, nil
}

func canonicalCategoryImportHeader(header string) string {
	normalized := normalizedProductImportHeader(header)
	if canonical, ok := categoryImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}
