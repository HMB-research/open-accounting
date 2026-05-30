package inventory

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/shopspring/decimal"
)

type stockImportRow struct {
	rowNumber int
	values    map[string]string
}

var stockImportHeaderAliases = map[string]string{
	"product_id":      "product_id",
	"product":         "product_code",
	"product_code":    "product_code",
	"sku":             "product_code",
	"item_code":       "product_code",
	"warehouse_id":    "warehouse_id",
	"warehouse":       "warehouse_code",
	"warehouse_code":  "warehouse_code",
	"location_code":   "warehouse_code",
	"quantity":        "quantity",
	"qty":             "quantity",
	"opening_qty":     "quantity",
	"opening_stock":   "quantity",
	"unit_cost":       "unit_cost",
	"cost":            "unit_cost",
	"lot":             "lot_number",
	"lot_number":      "lot_number",
	"batch":           "lot_number",
	"batch_number":    "lot_number",
	"serial":          "serial_number",
	"serial_number":   "serial_number",
	"expiry":          "expiry_date",
	"expiry_date":     "expiry_date",
	"expiration":      "expiry_date",
	"expiration_date": "expiry_date",
	"expires_at":      "expiry_date",
	"reason":          "reason",
	"notes":           "reason",
	"description":     "reason",
}

// ImportStockAdjustmentsCSV imports stock adjustment rows from CSV content.
func (s *Service) ImportStockAdjustmentsCSV(ctx context.Context, tenantID, schemaName string, req *ImportStockAdjustmentsRequest) (*ImportStockAdjustmentsResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseStockImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no stock adjustments found in CSV")
	}

	products, err := s.repo.ListProducts(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	productCodeToID := make(map[string]string, len(products))
	for _, product := range products {
		productCodeToID[normalizedStockImportKey(product.Code)] = product.ID
	}

	warehouses, err := s.repo.ListWarehouses(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	warehouseCodeToID := make(map[string]string, len(warehouses))
	for _, warehouse := range warehouses {
		warehouseCodeToID[normalizedStockImportKey(warehouse.Code)] = warehouse.ID
	}

	result := &ImportStockAdjustmentsResult{
		FileName: req.FileName,
		Errors:   []ImportStockAdjustmentsRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		adjustReq, err := buildStockAdjustmentFromImportRow(row, req.UserID, productCodeToID, warehouseCodeToID)
		if err != nil {
			appendStockImportRowError(result, row, err)
			continue
		}

		if _, err := s.AdjustStock(ctx, tenantID, schemaName, adjustReq); err != nil {
			appendStockImportRowError(result, row, err)
			continue
		}

		result.AdjustmentsImported++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func appendStockImportRowError(result *ImportStockAdjustmentsResult, row stockImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportStockAdjustmentsRowError{
		Row:          row.rowNumber,
		ProductRef:   firstNonBlank(row.values["product_id"], row.values["product_code"]),
		WarehouseRef: firstNonBlank(row.values["warehouse_id"], row.values["warehouse_code"]),
		Quantity:     strings.TrimSpace(row.values["quantity"]),
		Message:      err.Error(),
	})
}

func parseStockImportRows(content string) ([]stockImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectStockImportDelimiter(trimmed)
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
	columns := make(map[string]bool, len(headers))
	for i, header := range headers {
		canonicalHeaders[i] = canonicalStockImportHeader(header)
		if canonicalHeaders[i] != "" {
			columns[canonicalHeaders[i]] = true
		}
	}

	if !columns["quantity"] {
		return nil, fmt.Errorf("missing required columns: quantity")
	}
	if !columns["product_id"] && !columns["product_code"] {
		return nil, fmt.Errorf("missing required product identifier column: product_id or product_code")
	}
	if !columns["warehouse_id"] && !columns["warehouse_code"] {
		return nil, fmt.Errorf("missing required warehouse identifier column: warehouse_id or warehouse_code")
	}

	rows := make([]stockImportRow, 0)
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
		rows = append(rows, stockImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildStockAdjustmentFromImportRow(row stockImportRow, userID string, productCodeToID, warehouseCodeToID map[string]string) (*AdjustStockRequest, error) {
	productID, err := resolveStockImportProductID(row, productCodeToID)
	if err != nil {
		return nil, err
	}
	warehouseID, err := resolveStockImportWarehouseID(row, warehouseCodeToID)
	if err != nil {
		return nil, err
	}

	quantity := strings.TrimSpace(row.values["quantity"])
	if quantity == "" {
		return nil, fmt.Errorf("quantity is required")
	}
	parsedQuantity, err := decimal.NewFromString(quantity)
	if err != nil {
		return nil, fmt.Errorf("quantity must be a decimal")
	}
	if parsedQuantity.IsZero() {
		return nil, fmt.Errorf("quantity must not be zero")
	}

	unitCost := strings.TrimSpace(row.values["unit_cost"])
	if unitCost != "" {
		parsedUnitCost, err := decimal.NewFromString(unitCost)
		if err != nil {
			return nil, fmt.Errorf("unit_cost must be a decimal")
		}
		if parsedUnitCost.IsNegative() {
			return nil, fmt.Errorf("unit_cost cannot be negative")
		}
	}

	return &AdjustStockRequest{
		ProductID:    productID,
		WarehouseID:  warehouseID,
		Quantity:     parsedQuantity.String(),
		UnitCost:     unitCost,
		LotNumber:    strings.TrimSpace(row.values["lot_number"]),
		SerialNumber: strings.TrimSpace(row.values["serial_number"]),
		ExpiryDate:   strings.TrimSpace(row.values["expiry_date"]),
		Reason:       strings.TrimSpace(row.values["reason"]),
		UserID:       userID,
	}, nil
}

func resolveStockImportProductID(row stockImportRow, productCodeToID map[string]string) (string, error) {
	if productID := strings.TrimSpace(row.values["product_id"]); productID != "" {
		return productID, nil
	}
	productCode := strings.TrimSpace(row.values["product_code"])
	if productCode == "" {
		return "", fmt.Errorf("product_id or product_code is required")
	}
	productID, ok := productCodeToID[normalizedStockImportKey(productCode)]
	if !ok {
		return "", fmt.Errorf("product_code %q was not found", productCode)
	}
	return productID, nil
}

func resolveStockImportWarehouseID(row stockImportRow, warehouseCodeToID map[string]string) (string, error) {
	if warehouseID := strings.TrimSpace(row.values["warehouse_id"]); warehouseID != "" {
		return warehouseID, nil
	}
	warehouseCode := strings.TrimSpace(row.values["warehouse_code"])
	if warehouseCode == "" {
		return "", fmt.Errorf("warehouse_id or warehouse_code is required")
	}
	warehouseID, ok := warehouseCodeToID[normalizedStockImportKey(warehouseCode)]
	if !ok {
		return "", fmt.Errorf("warehouse_code %q was not found", warehouseCode)
	}
	return warehouseID, nil
}

func detectStockImportDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}

	bestDelimiter := ','
	bestCount := strings.Count(firstLine, string(bestDelimiter))
	for _, delimiter := range []rune{';', '\t'} {
		count := strings.Count(firstLine, string(delimiter))
		if count > bestCount {
			bestDelimiter = delimiter
			bestCount = count
		}
	}
	return bestDelimiter
}

func canonicalStockImportHeader(header string) string {
	normalized := normalizedStockImportHeader(header)
	if canonical, ok := stockImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedStockImportHeader(header string) string {
	normalized := strings.TrimSpace(strings.ToLower(header))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_")
	return replacer.Replace(normalized)
}

func normalizedStockImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
