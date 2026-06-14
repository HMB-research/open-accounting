package assets

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

type assetImportRow struct {
	rowNumber int
	values    map[string]string
}

var assetImportHeaderAliases = map[string]string{
	"asset_number":                          "asset_number",
	"asset_no":                              "asset_number",
	"asset_code":                            "asset_number",
	"number":                                "asset_number",
	"code":                                  "asset_number",
	"fixed_asset_number":                    "asset_number",
	"name":                                  "name",
	"asset_name":                            "name",
	"description":                           "description",
	"category_id":                           "category_id",
	"category":                              "category_name",
	"category_name":                         "category_name",
	"status":                                "status",
	"purchase_date":                         "purchase_date",
	"acquisition_date":                      "purchase_date",
	"date":                                  "purchase_date",
	"purchase_cost":                         "purchase_cost",
	"acquisition_cost":                      "purchase_cost",
	"cost":                                  "purchase_cost",
	"price":                                 "purchase_cost",
	"supplier_id":                           "supplier_id",
	"supplier_code":                         "supplier_code",
	"vendor_code":                           "supplier_code",
	"supplier_name":                         "supplier_name",
	"vendor_name":                           "supplier_name",
	"supplier_reg_code":                     "supplier_reg_code",
	"supplier_registration":                 "supplier_reg_code",
	"supplier_registry_code":                "supplier_reg_code",
	"vendor_reg_code":                       "supplier_reg_code",
	"supplier_vat_number":                   "supplier_vat_number",
	"supplier_vat":                          "supplier_vat_number",
	"supplier_vat_no":                       "supplier_vat_number",
	"vendor_vat_number":                     "supplier_vat_number",
	"supplier_email":                        "supplier_email",
	"vendor_email":                          "supplier_email",
	"invoice_id":                            "invoice_id",
	"serial_number":                         "serial_number",
	"serial_no":                             "serial_number",
	"location":                              "location",
	"depreciation_method":                   "depreciation_method",
	"useful_life_months":                    "useful_life_months",
	"life_months":                           "useful_life_months",
	"residual_value":                        "residual_value",
	"depreciation_start_date":               "depreciation_start_date",
	"accumulated_depreciation":              "accumulated_depreciation",
	"book_value":                            "book_value",
	"carrying_value":                        "book_value",
	"last_depreciation_date":                "last_depreciation_date",
	"disposal_date":                         "disposal_date",
	"disposal_method":                       "disposal_method",
	"disposal_proceeds":                     "disposal_proceeds",
	"disposal_notes":                        "disposal_notes",
	"asset_account_id":                      "asset_account_id",
	"asset_account_code":                    "asset_account_code",
	"depreciation_expense_account_id":       "depreciation_expense_account_id",
	"depreciation_expense_account_code":     "depreciation_expense_account_code",
	"accumulated_depreciation_account_id":   "accumulated_depreciation_account_id",
	"accumulated_depreciation_acct_id":      "accumulated_depreciation_account_id",
	"accumulated_depreciation_account":      "accumulated_depreciation_account_id",
	"accumulated_depreciation_account_uuid": "accumulated_depreciation_account_id",
	"accumulated_depreciation_account_code": "accumulated_depreciation_account_code",
	"accumulated_depreciation_acct_code":    "accumulated_depreciation_account_code",
}

// ImportAssetsCSV imports fixed assets from CSV content.
func (s *Service) ImportAssetsCSV(ctx context.Context, tenantID, schemaName string, req *ImportAssetsRequest) (*ImportAssetsResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseAssetImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no assets found in CSV")
	}

	existingAssets, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing assets: %w", err)
	}
	usedAssetNumbers := make(map[string]string, len(existingAssets)+len(rows))
	for _, asset := range existingAssets {
		key := normalizedAssetImportKey(asset.AssetNumber)
		if key == "" {
			continue
		}
		usedAssetNumbers[key] = asset.Name
	}

	categories, err := s.repo.ListCategories(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list asset categories: %w", err)
	}
	categoryNameToID := make(map[string]string, len(categories))
	for _, category := range categories {
		categoryNameToID[normalizedAssetImportKey(category.Name)] = category.ID
	}

	accountIDsByCode, err := s.assetImportAccountIDsByCode(ctx, schemaName, tenantID, rows)
	if err != nil {
		return nil, err
	}
	supplierLookup, err := s.assetImportSupplierLookup(ctx, schemaName, tenantID, rows)
	if err != nil {
		return nil, err
	}

	result := &ImportAssetsResult{
		FileName: req.FileName,
		Errors:   []ImportAssetsRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		asset, err := buildFixedAssetFromImportRow(row, tenantID, req.UserID, categoryNameToID, accountIDsByCode, supplierLookup)
		if err != nil {
			appendAssetImportRowError(result, row, err)
			continue
		}

		if strings.TrimSpace(asset.AssetNumber) == "" {
			assetNumber, err := s.repo.GenerateNumber(ctx, schemaName, tenantID)
			if err != nil {
				appendAssetImportRowError(result, row, fmt.Errorf("generate asset number: %w", err))
				continue
			}
			asset.AssetNumber = assetNumber
		}

		numberKey := normalizedAssetImportKey(asset.AssetNumber)
		if numberKey == "" {
			appendAssetImportRowError(result, row, fmt.Errorf("asset_number is required"))
			continue
		}
		if existingName, exists := usedAssetNumbers[numberKey]; exists {
			appendAssetImportRowError(result, row, fmt.Errorf("duplicate asset_number %q matches existing asset %q", asset.AssetNumber, existingName))
			continue
		}

		if err := s.repo.Create(ctx, schemaName, asset); err != nil {
			appendAssetImportRowError(result, row, err)
			continue
		}

		usedAssetNumbers[numberKey] = asset.Name
		result.AssetsCreated++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func appendAssetImportRowError(result *ImportAssetsResult, row assetImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportAssetsRowError{
		Row:         row.rowNumber,
		AssetNumber: strings.TrimSpace(row.values["asset_number"]),
		Name:        strings.TrimSpace(row.values["name"]),
		Message:     err.Error(),
	})
}

func parseAssetImportRows(content string) ([]assetImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectAssetImportDelimiter(trimmed)
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
	required := map[string]bool{
		"name":          false,
		"purchase_date": false,
		"purchase_cost": false,
	}
	for i, header := range headers {
		canonicalHeaders[i] = canonicalAssetImportHeader(header)
		if _, ok := required[canonicalHeaders[i]]; ok {
			required[canonicalHeaders[i]] = true
		}
	}

	var missing []string
	for _, name := range []string{"name", "purchase_date", "purchase_cost"} {
		if !required[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}

	rows := make([]assetImportRow, 0)
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

		rows = append(rows, assetImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func buildFixedAssetFromImportRow(
	row assetImportRow,
	tenantID, userID string,
	categoryNameToID map[string]string,
	accountIDsByCode map[string]string,
	supplierLookup contactrefs.SupplierLookup,
) (*FixedAsset, error) {
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	purchaseDate, err := parseAssetImportRequiredDate("purchase_date", row.values["purchase_date"])
	if err != nil {
		return nil, err
	}
	purchaseCost, err := parseAssetImportRequiredDecimal("purchase_cost", row.values["purchase_cost"])
	if err != nil {
		return nil, err
	}
	status, err := parseAssetImportStatus(row.values["status"])
	if err != nil {
		return nil, err
	}
	depreciationMethod, err := parseAssetImportDepreciationMethod(row.values["depreciation_method"])
	if err != nil {
		return nil, err
	}
	usefulLifeMonths, err := parseAssetImportUsefulLifeMonths(row.values["useful_life_months"])
	if err != nil {
		return nil, err
	}
	residualValue, err := parseAssetImportOptionalDecimal("residual_value", row.values["residual_value"], decimal.Zero)
	if err != nil {
		return nil, err
	}
	accumulatedDepreciation, err := parseAssetImportOptionalDecimal("accumulated_depreciation", row.values["accumulated_depreciation"], decimal.Zero)
	if err != nil {
		return nil, err
	}
	if accumulatedDepreciation.IsNegative() {
		return nil, fmt.Errorf("accumulated_depreciation cannot be negative")
	}

	expectedBookValue := purchaseCost.Sub(accumulatedDepreciation)
	bookValue, bookValueProvided, err := parseAssetImportBookValue(row.values["book_value"], expectedBookValue)
	if err != nil {
		return nil, err
	}
	if bookValue.IsNegative() {
		return nil, fmt.Errorf("book_value cannot be negative")
	}
	if bookValueProvided && !bookValue.Equal(expectedBookValue) {
		return nil, fmt.Errorf("book_value must equal purchase_cost minus accumulated_depreciation")
	}

	depreciationStartDate, err := parseAssetImportOptionalDate("depreciation_start_date", row.values["depreciation_start_date"])
	if err != nil {
		return nil, err
	}
	lastDepreciationDate, err := parseAssetImportOptionalDate("last_depreciation_date", row.values["last_depreciation_date"])
	if err != nil {
		return nil, err
	}
	disposalDate, err := parseAssetImportOptionalDate("disposal_date", row.values["disposal_date"])
	if err != nil {
		return nil, err
	}
	disposalMethod, err := parseAssetImportDisposalMethod(row.values["disposal_method"])
	if err != nil {
		return nil, err
	}
	disposalProceeds, err := parseAssetImportOptionalDecimal("disposal_proceeds", row.values["disposal_proceeds"], decimal.Zero)
	if err != nil {
		return nil, err
	}
	if disposalProceeds.IsNegative() {
		return nil, fmt.Errorf("disposal_proceeds cannot be negative")
	}

	categoryID, err := resolveAssetImportCategoryID(row, categoryNameToID)
	if err != nil {
		return nil, err
	}
	assetAccountID, err := resolveOptionalAssetImportAccountID(row, "asset_account_id", "asset_account_code", accountIDsByCode)
	if err != nil {
		return nil, err
	}
	depreciationExpenseAccountID, err := resolveOptionalAssetImportAccountID(row, "depreciation_expense_account_id", "depreciation_expense_account_code", accountIDsByCode)
	if err != nil {
		return nil, err
	}
	accumulatedDepreciationAccountID, err := resolveOptionalAssetImportAccountID(row, "accumulated_depreciation_account_id", "accumulated_depreciation_account_code", accountIDsByCode)
	if err != nil {
		return nil, err
	}
	supplierID, err := resolveOptionalAssetImportSupplierID(row, supplierLookup)
	if err != nil {
		return nil, err
	}
	invoiceID, err := parseOptionalAssetImportUUID("invoice_id", row.values["invoice_id"])
	if err != nil {
		return nil, err
	}

	now := time.Now()
	asset := &FixedAsset{
		ID:                            uuid.New().String(),
		TenantID:                      tenantID,
		AssetNumber:                   strings.TrimSpace(row.values["asset_number"]),
		Name:                          name,
		Description:                   strings.TrimSpace(row.values["description"]),
		CategoryID:                    categoryID,
		Status:                        status,
		PurchaseDate:                  purchaseDate,
		PurchaseCost:                  purchaseCost,
		SupplierID:                    supplierID,
		InvoiceID:                     invoiceID,
		SerialNumber:                  strings.TrimSpace(row.values["serial_number"]),
		Location:                      strings.TrimSpace(row.values["location"]),
		DepreciationMethod:            depreciationMethod,
		UsefulLifeMonths:              usefulLifeMonths,
		ResidualValue:                 residualValue,
		DepreciationStartDate:         depreciationStartDate,
		AccumulatedDepreciation:       accumulatedDepreciation,
		BookValue:                     bookValue,
		LastDepreciationDate:          lastDepreciationDate,
		DisposalDate:                  disposalDate,
		DisposalMethod:                disposalMethod,
		DisposalProceeds:              disposalProceeds,
		DisposalNotes:                 strings.TrimSpace(row.values["disposal_notes"]),
		AssetAccountID:                assetAccountID,
		DepreciationExpenseAccountID:  depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: accumulatedDepreciationAccountID,
		CreatedAt:                     now,
		CreatedBy:                     userID,
		UpdatedAt:                     now,
	}

	if err := asset.Validate(); err != nil {
		return nil, err
	}
	if asset.AccumulatedDepreciation.GreaterThan(asset.PurchaseCost.Sub(asset.ResidualValue)) {
		return nil, fmt.Errorf("accumulated_depreciation cannot exceed depreciable amount")
	}

	return asset, nil
}

func (s *Service) assetImportAccountIDsByCode(ctx context.Context, schemaName, tenantID string, rows []assetImportRow) (map[string]string, error) {
	usesAccountCodes := false
	for _, row := range rows {
		if strings.TrimSpace(row.values["asset_account_code"]) != "" ||
			strings.TrimSpace(row.values["depreciation_expense_account_code"]) != "" ||
			strings.TrimSpace(row.values["accumulated_depreciation_account_code"]) != "" {
			usesAccountCodes = true
			break
		}
	}
	if !usesAccountCodes {
		return nil, nil
	}
	if s.ledger == nil {
		return nil, fmt.Errorf("accounting service is required to resolve asset account codes")
	}

	accounts, err := s.ledger.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list accounts for asset import: %w", err)
	}
	accountIDsByCode := make(map[string]string, len(accounts))
	for _, account := range accounts {
		key := normalizedAssetImportKey(account.Code)
		if key != "" {
			accountIDsByCode[key] = account.ID
		}
	}
	return accountIDsByCode, nil
}

func (s *Service) assetImportSupplierLookup(ctx context.Context, schemaName, tenantID string, rows []assetImportRow) (contactrefs.SupplierLookup, error) {
	usesSupplierReferences := false
	for _, row := range rows {
		if hasAssetImportSupplierLookupReference(row) {
			usesSupplierReferences = true
			break
		}
	}
	if !usesSupplierReferences {
		return contactrefs.SupplierLookup{}, nil
	}
	if s.contacts == nil {
		return contactrefs.SupplierLookup{}, fmt.Errorf("contact service is required to resolve asset supplier references")
	}

	contacts, err := s.contacts.List(ctx, tenantID, schemaName, nil)
	if err != nil {
		return contactrefs.SupplierLookup{}, fmt.Errorf("list contacts for asset import: %w", err)
	}
	return contactrefs.NewSupplierLookup(contacts), nil
}

func resolveAssetImportCategoryID(row assetImportRow, categoryNameToID map[string]string) (*string, error) {
	if categoryID := strings.TrimSpace(row.values["category_id"]); categoryID != "" {
		return parseOptionalAssetImportUUID("category_id", categoryID)
	}
	categoryName := strings.TrimSpace(row.values["category_name"])
	if categoryName == "" {
		return nil, nil
	}
	categoryID, ok := categoryNameToID[normalizedAssetImportKey(categoryName)]
	if !ok {
		return nil, fmt.Errorf("category_name %q was not found", categoryName)
	}
	return &categoryID, nil
}

func resolveOptionalAssetImportAccountID(row assetImportRow, idField, codeField string, accountIDsByCode map[string]string) (*string, error) {
	if accountID := strings.TrimSpace(row.values[idField]); accountID != "" {
		return parseOptionalAssetImportUUID(idField, accountID)
	}
	accountCode := strings.TrimSpace(row.values[codeField])
	if accountCode == "" {
		return nil, nil
	}
	accountID, ok := accountIDsByCode[normalizedAssetImportKey(accountCode)]
	if !ok {
		return nil, fmt.Errorf("account code %q was not found for %s", accountCode, codeField)
	}
	return &accountID, nil
}

func resolveOptionalAssetImportSupplierID(row assetImportRow, supplierLookup contactrefs.SupplierLookup) (*string, error) {
	supplierID, err := supplierLookup.ResolveID(row.values["supplier_id"], assetImportSupplierReferences(row)...)
	if err != nil {
		return nil, err
	}
	if supplierID == nil {
		return nil, nil
	}
	return supplierID, nil
}

func hasAssetImportSupplierLookupReference(row assetImportRow) bool {
	for _, ref := range assetImportSupplierReferences(row) {
		if strings.TrimSpace(ref.Value) != "" {
			return true
		}
	}
	return false
}

func assetImportSupplierReferences(row assetImportRow) []contactrefs.Reference {
	return []contactrefs.Reference{
		{Field: "supplier_code", Value: row.values["supplier_code"]},
		{Field: "supplier_reg_code", Value: row.values["supplier_reg_code"]},
		{Field: "supplier_vat_number", Value: row.values["supplier_vat_number"]},
		{Field: "supplier_email", Value: row.values["supplier_email"]},
		{Field: "supplier_name", Value: row.values["supplier_name"]},
	}
}

func parseOptionalAssetImportUUID(field, value string) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", field)
	}
	canonicalID := parsedID.String()
	return &canonicalID, nil
}

func parseAssetImportStatus(value string) (AssetStatus, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return AssetStatusDraft, nil
	}
	switch AssetStatus(normalized) {
	case AssetStatusDraft, AssetStatusActive, AssetStatusDisposed, AssetStatusSold:
		return AssetStatus(normalized), nil
	default:
		return "", fmt.Errorf("invalid status %q", value)
	}
}

func parseAssetImportDepreciationMethod(value string) (DepreciationMethod, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if normalized == "" {
		return DepreciationStraightLine, nil
	}
	switch DepreciationMethod(normalized) {
	case DepreciationStraightLine, DepreciationDecliningBalance, DepreciationUnitsOfProd:
		return DepreciationMethod(normalized), nil
	default:
		return "", fmt.Errorf("invalid depreciation_method %q", value)
	}
}

func parseAssetImportDisposalMethod(value string) (*DisposalMethod, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return nil, nil
	}
	switch DisposalMethod(normalized) {
	case DisposalSold, DisposalScrapped, DisposalDonated, DisposalLost:
		method := DisposalMethod(normalized)
		return &method, nil
	default:
		return nil, fmt.Errorf("invalid disposal_method %q", value)
	}
}

func parseAssetImportUsefulLifeMonths(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 60, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("useful_life_months must be an integer")
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("useful_life_months must be positive")
	}
	return parsed, nil
}

func parseAssetImportRequiredDecimal(field, value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, fmt.Errorf("%s is required", field)
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s must be a decimal", field)
	}
	return parsed, nil
}

func parseAssetImportOptionalDecimal(field, value string, fallback decimal.Decimal) (decimal.Decimal, error) {
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

func parseAssetImportBookValue(value string, fallback decimal.Decimal) (decimal.Decimal, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, false, nil
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, true, fmt.Errorf("book_value must be a decimal")
	}
	return parsed, true, nil
}

func parseAssetImportRequiredDate(field, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	parsed, err := parseAssetImportDate(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be a date in YYYY-MM-DD or RFC3339 format", field)
	}
	return parsed, nil
}

func parseAssetImportOptionalDate(field, value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := parseAssetImportDate(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be a date in YYYY-MM-DD or RFC3339 format", field)
	}
	return &parsed, nil
}

func parseAssetImportDate(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date")
}

func detectAssetImportDelimiter(content string) rune {
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

func canonicalAssetImportHeader(header string) string {
	normalized := normalizedAssetImportHeader(header)
	if canonical, ok := assetImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedAssetImportHeader(header string) string {
	normalized := strings.TrimSpace(strings.ToLower(header))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_")
	return replacer.Replace(normalized)
}

func normalizedAssetImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
