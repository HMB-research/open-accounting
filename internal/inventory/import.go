package inventory

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contactrefs"
)

type productImportRow struct {
	rowNumber int
	values    map[string]string
}

var productImportHeaderAliases = map[string]string{
	"code":                   "code",
	"product_code":           "code",
	"sku":                    "code",
	"item_code":              "code",
	"name":                   "name",
	"product_name":           "name",
	"item_name":              "name",
	"description":            "description",
	"product_type":           "product_type",
	"type":                   "product_type",
	"category_id":            "category_id",
	"category":               "category_name",
	"category_name":          "category_name",
	"unit":                   "unit",
	"unit_of_measure":        "unit",
	"purchase_price":         "purchase_price",
	"cost_price":             "purchase_price",
	"cost":                   "purchase_price",
	"sales_price":            "sales_price",
	"sale_price":             "sales_price",
	"selling_price":          "sales_price",
	"price":                  "sales_price",
	"vat_rate":               "vat_rate",
	"vat":                    "vat_rate",
	"min_stock_level":        "min_stock_level",
	"minimum_stock":          "min_stock_level",
	"reorder_point":          "reorder_point",
	"sale_account_id":        "sale_account_id",
	"sales_account_id":       "sale_account_id",
	"sale_account_code":      "sale_account_code",
	"sales_account_code":     "sale_account_code",
	"purchase_account_id":    "purchase_account_id",
	"purchase_account_code":  "purchase_account_code",
	"inventory_account_id":   "inventory_account_id",
	"inventory_account_code": "inventory_account_code",
	"track_inventory":        "track_inventory",
	"track_stock":            "track_inventory",
	"status":                 "status",
	"is_active":              "is_active",
	"active":                 "is_active",
	"barcode":                "barcode",
	"supplier_id":            "supplier_id",
	"supplier_code":          "supplier_code",
	"vendor_code":            "supplier_code",
	"supplier_name":          "supplier_name",
	"vendor_name":            "supplier_name",
	"supplier_reg_code":      "supplier_reg_code",
	"supplier_registration":  "supplier_reg_code",
	"supplier_registry_code": "supplier_reg_code",
	"vendor_reg_code":        "supplier_reg_code",
	"supplier_vat_number":    "supplier_vat_number",
	"supplier_vat":           "supplier_vat_number",
	"supplier_vat_no":        "supplier_vat_number",
	"vendor_vat_number":      "supplier_vat_number",
	"supplier_email":         "supplier_email",
	"vendor_email":           "supplier_email",
	"lead_time_days":         "lead_time_days",
}

// ImportProductsCSV imports product master data from CSV content.
func (s *Service) ImportProductsCSV(ctx context.Context, tenantID, schemaName string, req *ImportProductsRequest) (*ImportProductsResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseProductImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no products found in CSV")
	}

	existingProducts, err := s.repo.ListProducts(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing products: %w", err)
	}
	usedCodes := make(map[string]string, len(existingProducts)+len(rows))
	for _, product := range existingProducts {
		key := normalizedProductImportKey(product.Code)
		if key == "" {
			continue
		}
		usedCodes[key] = product.Name
	}

	categories, err := s.repo.ListCategories(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list product categories: %w", err)
	}
	categoryNameToID := make(map[string]string, len(categories))
	categoryIDs := make(map[string]bool, len(categories))
	for _, category := range categories {
		categoryNameToID[normalizedProductImportKey(category.Name)] = category.ID
		categoryIDs[strings.ToLower(strings.TrimSpace(category.ID))] = true
	}

	accountIDsByCode, err := s.productImportAccountIDsByCode(ctx, schemaName, tenantID, rows)
	if err != nil {
		return nil, err
	}
	supplierLookup, err := s.productImportSupplierLookup(ctx, schemaName, tenantID, rows)
	if err != nil {
		return nil, err
	}

	result := &ImportProductsResult{
		FileName: req.FileName,
		Errors:   []ImportProductsRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		product, err := buildProductFromImportRow(row, tenantID, categoryNameToID, categoryIDs, accountIDsByCode, supplierLookup)
		if err != nil {
			appendProductImportRowError(result, row, err)
			continue
		}

		if strings.TrimSpace(product.Code) == "" {
			code, err := s.repo.GenerateCode(ctx, schemaName, tenantID)
			if err != nil {
				appendProductImportRowError(result, row, fmt.Errorf("generate code: %w", err))
				continue
			}
			product.Code = code
		}

		codeKey := normalizedProductImportKey(product.Code)
		if codeKey == "" {
			appendProductImportRowError(result, row, fmt.Errorf("code is required"))
			continue
		}
		if existingName, exists := usedCodes[codeKey]; exists {
			appendProductImportRowError(result, row, fmt.Errorf("duplicate code %q matches existing product %q", product.Code, existingName))
			continue
		}

		if err := s.repo.CreateProduct(ctx, schemaName, product); err != nil {
			appendProductImportRowError(result, row, err)
			continue
		}

		usedCodes[codeKey] = product.Name
		result.ProductsCreated++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func appendProductImportRowError(result *ImportProductsResult, row productImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportProductsRowError{
		Row:     row.rowNumber,
		Code:    strings.TrimSpace(row.values["code"]),
		Name:    strings.TrimSpace(row.values["name"]),
		Message: err.Error(),
	})
}

func parseProductImportRows(content string) ([]productImportRow, error) {
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
		"name":        false,
		"sales_price": false,
	}
	for i, header := range headers {
		canonicalHeaders[i] = canonicalProductImportHeader(header)
		if _, ok := required[canonicalHeaders[i]]; ok {
			required[canonicalHeaders[i]] = true
		}
	}

	var missing []string
	for _, name := range []string{"name", "sales_price"} {
		if !required[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}

	rows := make([]productImportRow, 0)
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
		rows = append(rows, productImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildProductFromImportRow(
	row productImportRow,
	tenantID string,
	categoryNameToID map[string]string,
	categoryIDs map[string]bool,
	accountIDsByCode map[string]string,
	supplierLookup contactrefs.SupplierLookup,
) (*Product, error) {
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	productType, err := parseProductImportType(row.values["product_type"])
	if err != nil {
		return nil, err
	}
	purchasePrice, err := parseProductImportOptionalDecimal("purchase_price", row.values["purchase_price"], decimal.Zero)
	if err != nil {
		return nil, err
	}
	salesPrice, err := parseProductImportRequiredDecimal("sales_price", row.values["sales_price"])
	if err != nil {
		return nil, err
	}
	vatRate, err := parseProductImportOptionalDecimal("vat_rate", row.values["vat_rate"], decimal.NewFromInt(22))
	if err != nil {
		return nil, err
	}
	minStockLevel, err := parseProductImportOptionalDecimal("min_stock_level", row.values["min_stock_level"], decimal.Zero)
	if err != nil {
		return nil, err
	}
	reorderPoint, err := parseProductImportOptionalDecimal("reorder_point", row.values["reorder_point"], decimal.Zero)
	if err != nil {
		return nil, err
	}
	leadTimeDays, err := parseProductImportNonNegativeInt("lead_time_days", row.values["lead_time_days"])
	if err != nil {
		return nil, err
	}
	isActive, err := parseProductImportActive(row.values["status"], row.values["is_active"])
	if err != nil {
		return nil, err
	}
	trackInventory, err := parseProductImportTrackInventory(row.values["track_inventory"], productType)
	if err != nil {
		return nil, err
	}
	categoryID, err := resolveProductImportCategoryID(row, categoryNameToID, categoryIDs)
	if err != nil {
		return nil, err
	}
	saleAccountID, err := resolveOptionalProductImportAccountID(row, "sale_account_id", "sale_account_code", accountIDsByCode)
	if err != nil {
		return nil, err
	}
	purchaseAccountID, err := resolveOptionalProductImportAccountID(row, "purchase_account_id", "purchase_account_code", accountIDsByCode)
	if err != nil {
		return nil, err
	}
	inventoryAccountID, err := resolveOptionalProductImportAccountID(row, "inventory_account_id", "inventory_account_code", accountIDsByCode)
	if err != nil {
		return nil, err
	}
	supplierID, err := resolveOptionalProductImportSupplierID(row, supplierLookup)
	if err != nil {
		return nil, err
	}

	if purchasePrice.IsNegative() {
		return nil, fmt.Errorf("purchase_price cannot be negative")
	}
	if vatRate.IsNegative() {
		return nil, fmt.Errorf("vat_rate cannot be negative")
	}
	if minStockLevel.IsNegative() {
		return nil, fmt.Errorf("min_stock_level cannot be negative")
	}
	if reorderPoint.IsNegative() {
		return nil, fmt.Errorf("reorder_point cannot be negative")
	}

	unit := strings.TrimSpace(row.values["unit"])
	if unit == "" {
		unit = "pcs"
	}
	now := time.Now()
	product := &Product{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		Code:               strings.TrimSpace(row.values["code"]),
		Name:               name,
		Description:        strings.TrimSpace(row.values["description"]),
		ProductType:        productType,
		CategoryID:         categoryID,
		Unit:               unit,
		PurchasePrice:      purchasePrice,
		SalesPrice:         salesPrice,
		VATRate:            vatRate,
		MinStockLevel:      minStockLevel,
		CurrentStock:       decimal.Zero,
		ReorderPoint:       reorderPoint,
		SaleAccountID:      saleAccountID,
		PurchaseAccountID:  purchaseAccountID,
		InventoryAccountID: inventoryAccountID,
		TrackInventory:     trackInventory,
		IsActive:           isActive,
		Barcode:            strings.TrimSpace(row.values["barcode"]),
		SupplierID:         supplierID,
		LeadTimeDays:       leadTimeDays,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return product, product.Validate()
}

func (s *Service) productImportAccountIDsByCode(ctx context.Context, schemaName, tenantID string, rows []productImportRow) (map[string]string, error) {
	usesAccountCodes := false
	for _, row := range rows {
		if strings.TrimSpace(row.values["sale_account_code"]) != "" ||
			strings.TrimSpace(row.values["purchase_account_code"]) != "" ||
			strings.TrimSpace(row.values["inventory_account_code"]) != "" {
			usesAccountCodes = true
			break
		}
	}
	if !usesAccountCodes {
		return nil, nil
	}
	if s.accounts == nil {
		return nil, fmt.Errorf("accounting service is required to resolve product account codes")
	}

	accounts, err := s.accounts.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list accounts for product import: %w", err)
	}
	accountIDsByCode := make(map[string]string, len(accounts))
	for _, account := range accounts {
		key := normalizedProductImportKey(account.Code)
		if key != "" {
			accountIDsByCode[key] = account.ID
		}
	}
	return accountIDsByCode, nil
}

func (s *Service) productImportSupplierLookup(ctx context.Context, schemaName, tenantID string, rows []productImportRow) (contactrefs.SupplierLookup, error) {
	usesSupplierReferences := false
	for _, row := range rows {
		if hasProductImportSupplierLookupReference(row) {
			usesSupplierReferences = true
			break
		}
	}
	if !usesSupplierReferences {
		return contactrefs.SupplierLookup{}, nil
	}
	if s.contacts == nil {
		return contactrefs.SupplierLookup{}, fmt.Errorf("contact service is required to resolve product supplier references")
	}

	contacts, err := s.contacts.List(ctx, tenantID, schemaName, nil)
	if err != nil {
		return contactrefs.SupplierLookup{}, fmt.Errorf("list contacts for product import: %w", err)
	}
	return contactrefs.NewSupplierLookup(contacts), nil
}

func resolveProductImportCategoryID(row productImportRow, categoryNameToID map[string]string, categoryIDs map[string]bool) (string, error) {
	if categoryID := strings.TrimSpace(row.values["category_id"]); categoryID != "" {
		parsedID, err := uuid.Parse(categoryID)
		if err != nil {
			return "", fmt.Errorf("category_id must be a valid UUID")
		}
		categoryID = parsedID.String()
		if !categoryIDs[strings.ToLower(categoryID)] {
			return "", fmt.Errorf("category_id %q was not found", categoryID)
		}
		return categoryID, nil
	}
	categoryName := strings.TrimSpace(row.values["category_name"])
	if categoryName == "" {
		return "", nil
	}
	categoryID, ok := categoryNameToID[normalizedProductImportKey(categoryName)]
	if !ok {
		return "", fmt.Errorf("category_name %q was not found", categoryName)
	}
	return categoryID, nil
}

func resolveOptionalProductImportAccountID(row productImportRow, idField, codeField string, accountIDsByCode map[string]string) (string, error) {
	if accountID := strings.TrimSpace(row.values[idField]); accountID != "" {
		parsedID, err := uuid.Parse(accountID)
		if err != nil {
			return "", fmt.Errorf("%s must be a valid UUID", idField)
		}
		return parsedID.String(), nil
	}
	accountCode := strings.TrimSpace(row.values[codeField])
	if accountCode == "" {
		return "", nil
	}
	accountID, ok := accountIDsByCode[normalizedProductImportKey(accountCode)]
	if !ok {
		return "", fmt.Errorf("account code %q was not found for %s", accountCode, codeField)
	}
	return accountID, nil
}

func resolveOptionalProductImportSupplierID(row productImportRow, supplierLookup contactrefs.SupplierLookup) (string, error) {
	supplierID, err := supplierLookup.ResolveID(row.values["supplier_id"], productImportSupplierReferences(row)...)
	if err != nil {
		return "", err
	}
	if supplierID == nil {
		return "", nil
	}
	return *supplierID, nil
}

func hasProductImportSupplierLookupReference(row productImportRow) bool {
	for _, ref := range productImportSupplierReferences(row) {
		if strings.TrimSpace(ref.Value) != "" {
			return true
		}
	}
	return false
}

func productImportSupplierReferences(row productImportRow) []contactrefs.Reference {
	return []contactrefs.Reference{
		{Field: "supplier_code", Value: row.values["supplier_code"]},
		{Field: "supplier_reg_code", Value: row.values["supplier_reg_code"]},
		{Field: "supplier_vat_number", Value: row.values["supplier_vat_number"]},
		{Field: "supplier_email", Value: row.values["supplier_email"]},
		{Field: "supplier_name", Value: row.values["supplier_name"]},
	}
}

func parseProductImportType(value string) (ProductType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return ProductTypeGoods, nil
	}
	switch ProductType(normalized) {
	case ProductTypeGoods, ProductTypeService:
		return ProductType(normalized), nil
	default:
		return "", fmt.Errorf("invalid product_type %q", value)
	}
}

func parseProductImportActive(statusValue, activeValue string) (bool, error) {
	status := strings.ToUpper(strings.TrimSpace(statusValue))
	if status != "" {
		switch ProductStatus(status) {
		case ProductStatusActive:
			return true, nil
		case ProductStatusInactive:
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

func parseProductImportTrackInventory(value string, productType ProductType) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return productType == ProductTypeGoods, nil
	}
	return parseProductImportBool("track_inventory", value)
}

func parseProductImportBool(field, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "yes", "y", "1":
		return true, nil
	case "false", "f", "no", "n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", field)
	}
}

func parseProductImportRequiredDecimal(field, value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, fmt.Errorf("%s is required", field)
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s must be a decimal", field)
	}
	if parsed.IsNegative() {
		return decimal.Zero, fmt.Errorf("%s cannot be negative", field)
	}
	return parsed, nil
}

func parseProductImportOptionalDecimal(field, value string, fallback decimal.Decimal) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s must be a decimal", field)
	}
	return parsed, nil
}

func parseProductImportNonNegativeInt(field, value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	return parsed, nil
}

func detectProductImportDelimiter(content string) rune {
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

func canonicalProductImportHeader(header string) string {
	normalized := normalizedProductImportHeader(header)
	if canonical, ok := productImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedProductImportHeader(header string) string {
	normalized := strings.TrimSpace(strings.ToLower(header))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_")
	return replacer.Replace(normalized)
}

func normalizedProductImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
