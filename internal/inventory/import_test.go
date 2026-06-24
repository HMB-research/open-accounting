package inventory

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contactrefs"
	"github.com/HMB-research/open-accounting/internal/contacts"
)

func TestProductImportServiceEdges(t *testing.T) {
	t.Run("rejects missing content before repository access", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())

		_, err := service.ImportProductsCSV(context.Background(), "tenant-1", "test_schema", nil)

		require.ErrorContains(t, err, "csv_content is required")
	})

	t.Run("rejects files with only headers", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())

		_, err := service.ImportProductsCSV(context.Background(), "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price\n",
		})

		require.ErrorContains(t, err, "no products found in CSV")
	})

	t.Run("returns account resolver dependency errors", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())

		_, err := service.ImportProductsCSV(context.Background(), "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price,sale_account_code\nWidget,15.00,4000\n",
		})

		require.ErrorContains(t, err, "accounting service is required")
	})

	t.Run("returns supplier resolver dependency errors", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())

		_, err := service.ImportProductsCSV(context.Background(), "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price,supplier_code\nWidget,15.00,SUP-1\n",
		})

		require.ErrorContains(t, err, "contact service is required")
	})

	t.Run("records repository create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ErrOnCreate = true
		service := NewServiceWithRepository(repo)

		result, err := service.ImportProductsCSV(context.Background(), "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "code,name,sales_price\nSKU-001,Widget,15.00\n",
		})

		require.NoError(t, err)
		assert.Zero(t, result.ProductsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "mock error on create")
	})
}

func TestProductImportRowsEdges(t *testing.T) {
	_, err := parseProductImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseProductImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseProductImportRows("name,sales_price\n\"unterminated")
	require.ErrorContains(t, err, "parse csv row 2")

	_, err = parseProductImportRows("name,unit\nWidget,pcs\n")
	require.ErrorContains(t, err, "missing required columns: sales_price")

	rows, err := parseProductImportRows("\ufeffproduct_name;price;unknown.column\nWidget;15.00;ignored\n;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "Widget", rows[0].values["name"])
	assert.Equal(t, "15.00", rows[0].values["sales_price"])
	assert.Equal(t, "ignored", rows[0].values["unknown_column"])

	rows, err = parseProductImportRows("name\tprice\nConsulting\t120.00\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Consulting", rows[0].values["name"])
}

func TestBuildProductFromImportRowEdges(t *testing.T) {
	validValues := map[string]string{
		"name":        "Widget",
		"sales_price": "15.00",
	}

	tests := []struct {
		name        string
		mutate      func(map[string]string)
		wantMessage string
	}{
		{
			name:        "name required",
			mutate:      func(values map[string]string) { values["name"] = "" },
			wantMessage: "name is required",
		},
		{
			name:        "invalid product type",
			mutate:      func(values map[string]string) { values["product_type"] = "BUNDLE" },
			wantMessage: "invalid product_type",
		},
		{
			name:        "invalid sales price",
			mutate:      func(values map[string]string) { values["sales_price"] = "fifteen" },
			wantMessage: "sales_price must be a decimal",
		},
		{
			name:        "negative sales price",
			mutate:      func(values map[string]string) { values["sales_price"] = "-1" },
			wantMessage: "sales_price cannot be negative",
		},
		{
			name:        "negative purchase price",
			mutate:      func(values map[string]string) { values["purchase_price"] = "-1" },
			wantMessage: "purchase_price cannot be negative",
		},
		{
			name:        "negative VAT rate",
			mutate:      func(values map[string]string) { values["vat_rate"] = "-1" },
			wantMessage: "vat_rate cannot be negative",
		},
		{
			name:        "negative minimum stock",
			mutate:      func(values map[string]string) { values["min_stock_level"] = "-1" },
			wantMessage: "min_stock_level cannot be negative",
		},
		{
			name:        "negative reorder point",
			mutate:      func(values map[string]string) { values["reorder_point"] = "-1" },
			wantMessage: "reorder_point cannot be negative",
		},
		{
			name:        "invalid lead time",
			mutate:      func(values map[string]string) { values["lead_time_days"] = "soon" },
			wantMessage: "lead_time_days must be an integer",
		},
		{
			name:        "negative lead time",
			mutate:      func(values map[string]string) { values["lead_time_days"] = "-1" },
			wantMessage: "lead_time_days cannot be negative",
		},
		{
			name:        "invalid status",
			mutate:      func(values map[string]string) { values["status"] = "ARCHIVED" },
			wantMessage: "invalid status",
		},
		{
			name:        "invalid active flag",
			mutate:      func(values map[string]string) { values["is_active"] = "maybe" },
			wantMessage: "is_active must be true or false",
		},
		{
			name:        "invalid track inventory flag",
			mutate:      func(values map[string]string) { values["track_inventory"] = "maybe" },
			wantMessage: "track_inventory must be true or false",
		},
		{
			name:        "missing category name",
			mutate:      func(values map[string]string) { values["category_name"] = "Parts" },
			wantMessage: `category_name "Parts" was not found`,
		},
		{
			name:        "invalid purchase account id",
			mutate:      func(values map[string]string) { values["purchase_account_id"] = "legacy-account" },
			wantMessage: "purchase_account_id must be a valid UUID",
		},
		{
			name:        "missing inventory account code",
			mutate:      func(values map[string]string) { values["inventory_account_code"] = "1400" },
			wantMessage: `account code "1400" was not found for inventory_account_code`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(map[string]string, len(validValues))
			for key, value := range validValues {
				values[key] = value
			}
			tt.mutate(values)

			_, err := buildProductFromImportRow(
				productImportRow{rowNumber: 2, values: values},
				"tenant-1",
				nil,
				nil,
				nil,
				contactrefs.SupplierLookup{},
			)

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}
}

func TestBuildProductFromImportRowDefaultsAndResolutions(t *testing.T) {
	categoryID := "11111111-1111-4111-8111-111111111111"
	accountID := "22222222-2222-4222-8222-222222222222"
	supplierID := "33333333-3333-4333-8333-333333333333"
	row := productImportRow{rowNumber: 2, values: map[string]string{
		"code":                  " SKU-001 ",
		"name":                  " Widget ",
		"description":           " Inventory item ",
		"product_type":          "service",
		"category_name":         "Parts",
		"unit":                  "hour",
		"purchase_price":        "10.50",
		"sales_price":           "15.00",
		"vat_rate":              "20",
		"min_stock_level":       "2",
		"reorder_point":         "3",
		"sale_account_code":     "4000",
		"track_inventory":       "yes",
		"status":                "INACTIVE",
		"barcode":               "ABC",
		"supplier_code":         "SUP-1",
		"lead_time_days":        "5",
		"unknown_legacy_column": "ignored",
	}}
	lookup := contactrefs.NewSupplierLookup([]contacts.Contact{{ID: supplierID, Code: "SUP-1"}})

	product, err := buildProductFromImportRow(
		row,
		"tenant-1",
		map[string]string{"parts": categoryID},
		map[string]bool{categoryID: true},
		map[string]string{"4000": accountID},
		lookup,
	)

	require.NoError(t, err)
	assert.Equal(t, "tenant-1", product.TenantID)
	assert.Equal(t, "SKU-001", product.Code)
	assert.Equal(t, "Widget", product.Name)
	assert.Equal(t, ProductTypeService, product.ProductType)
	assert.Equal(t, categoryID, product.CategoryID)
	assert.Equal(t, "hour", product.Unit)
	assert.True(t, product.PurchasePrice.Equal(decimal.RequireFromString("10.50")))
	assert.True(t, product.SalesPrice.Equal(decimal.RequireFromString("15.00")))
	assert.True(t, product.VATRate.Equal(decimal.RequireFromString("20")))
	assert.Equal(t, accountID, product.SaleAccountID)
	assert.True(t, product.TrackInventory)
	assert.False(t, product.IsActive)
	assert.Equal(t, "ABC", product.Barcode)
	assert.Equal(t, supplierID, product.SupplierID)
	assert.Equal(t, 5, product.LeadTimeDays)
}

func TestProductImportParserHelpers(t *testing.T) {
	productType, err := parseProductImportType("")
	require.NoError(t, err)
	assert.Equal(t, ProductTypeGoods, productType)

	productType, err = parseProductImportType("service")
	require.NoError(t, err)
	assert.Equal(t, ProductTypeService, productType)

	_, err = parseProductImportType("bundle")
	require.ErrorContains(t, err, "invalid product_type")

	active, err := parseProductImportActive("ACTIVE", "false")
	require.NoError(t, err)
	assert.True(t, active)

	active, err = parseProductImportActive("INACTIVE", "")
	require.NoError(t, err)
	assert.False(t, active)

	_, err = parseProductImportActive("ARCHIVED", "")
	require.ErrorContains(t, err, "invalid status")

	active, err = parseProductImportActive("", "")
	require.NoError(t, err)
	assert.True(t, active)

	active, err = parseProductImportActive("", "no")
	require.NoError(t, err)
	assert.False(t, active)

	_, err = parseProductImportBool("is_active", "maybe")
	require.ErrorContains(t, err, "is_active must be true or false")

	value, err := parseProductImportRequiredDecimal("sales_price", "12.50")
	require.NoError(t, err)
	assert.True(t, value.Equal(decimal.RequireFromString("12.50")))

	_, err = parseProductImportRequiredDecimal("sales_price", "")
	require.ErrorContains(t, err, "sales_price is required")

	_, err = parseProductImportRequiredDecimal("sales_price", "bad")
	require.ErrorContains(t, err, "sales_price must be a decimal")

	_, err = parseProductImportRequiredDecimal("sales_price", "-1")
	require.ErrorContains(t, err, "sales_price cannot be negative")

	value, err = parseProductImportOptionalDecimal("vat_rate", "", decimal.NewFromInt(22))
	require.NoError(t, err)
	assert.True(t, value.Equal(decimal.NewFromInt(22)))

	_, err = parseProductImportOptionalDecimal("vat_rate", "bad", decimal.Zero)
	require.ErrorContains(t, err, "vat_rate must be a decimal")

	count, err := parseProductImportNonNegativeInt("lead_time_days", "")
	require.NoError(t, err)
	assert.Zero(t, count)

	count, err = parseProductImportNonNegativeInt("lead_time_days", "5")
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	_, err = parseProductImportNonNegativeInt("lead_time_days", "soon")
	require.ErrorContains(t, err, "lead_time_days must be an integer")

	_, err = parseProductImportNonNegativeInt("lead_time_days", "-1")
	require.ErrorContains(t, err, "lead_time_days cannot be negative")

	assert.Equal(t, ';', detectProductImportDelimiter("name;sales_price\nWidget;15"))
	assert.Equal(t, '\t', detectProductImportDelimiter("name\tsales_price\nWidget\t15"))
	assert.Equal(t, ',', detectProductImportDelimiter("name,sales_price\nWidget,15"))
	assert.Equal(t, "sales_price", canonicalProductImportHeader(" sale-price "))
	assert.Equal(t, "legacy_column", canonicalProductImportHeader("legacy column"))
}

func TestProductImportResolverDependencyEdges(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	rows := []productImportRow{{values: map[string]string{"sale_account_code": "4000"}}}

	accountIDs, err := service.productImportAccountIDsByCode(context.Background(), "test_schema", "tenant-1", nil)
	require.NoError(t, err)
	assert.Nil(t, accountIDs)

	_, err = service.productImportAccountIDsByCode(context.Background(), "test_schema", "tenant-1", rows)
	require.ErrorContains(t, err, "accounting service is required")

	lookup, err := service.productImportSupplierLookup(context.Background(), "test_schema", "tenant-1", nil)
	require.NoError(t, err)
	assert.Empty(t, lookup)

	_, err = service.productImportSupplierLookup(context.Background(), "test_schema", "tenant-1", []productImportRow{{
		values: map[string]string{"supplier_code": "SUP-1"},
	}})
	require.ErrorContains(t, err, "contact service is required")
}

func TestCategoryImportRowsAndHelpersEdges(t *testing.T) {
	_, err := parseCategoryImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseCategoryImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseCategoryImportRows("name\n\"unterminated")
	require.ErrorContains(t, err, "parse csv row 2")

	_, err = parseCategoryImportRows("description\nSpare parts\n")
	require.ErrorContains(t, err, "missing required columns: name")

	rows, err := parseCategoryImportRows("\ufeffcategory;parent;legacy.column\nParts;;ignored\n;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "Parts", rows[0].values["name"])
	assert.Equal(t, "ignored", rows[0].values["legacy_column"])

	parentID := "11111111-1111-4111-8111-111111111111"
	category, err := buildCategoryFromImportRow(categoryImportRow{values: map[string]string{
		"id":          "22222222-2222-4222-8222-222222222222",
		"name":        " Fasteners ",
		"description": " Bolts ",
		"parent_name": " Parts ",
	}}, "tenant-1", map[string]string{"parts": parentID})
	require.NoError(t, err)
	assert.Equal(t, "22222222-2222-4222-8222-222222222222", category.ID)
	assert.Equal(t, "tenant-1", category.TenantID)
	assert.Equal(t, "Fasteners", category.Name)
	assert.Equal(t, "Bolts", category.Description)
	assert.Equal(t, parentID, category.ParentID)

	_, err = buildCategoryFromImportRow(categoryImportRow{values: map[string]string{"name": ""}}, "tenant-1", nil)
	require.ErrorContains(t, err, "name is required")

	_, err = buildCategoryFromImportRow(categoryImportRow{values: map[string]string{"id": "legacy-id", "name": "Parts"}}, "tenant-1", nil)
	require.ErrorContains(t, err, "invalid id")

	_, err = resolveCategoryImportParentID(categoryImportRow{values: map[string]string{"parent_id": "legacy-parent"}}, nil)
	require.ErrorContains(t, err, "parent_id must be a valid UUID")

	_, err = resolveCategoryImportParentID(categoryImportRow{values: map[string]string{"parent_name": "Missing"}}, nil)
	require.ErrorContains(t, err, `parent_name "Missing" was not found`)

	assert.Equal(t, "parent_name", canonicalCategoryImportHeader(" parent-category "))
	assert.Equal(t, "legacy_column", canonicalCategoryImportHeader("legacy column"))
}

func TestWarehouseImportRowsAndHelpersEdges(t *testing.T) {
	_, err := parseWarehouseImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseWarehouseImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseWarehouseImportRows("code,name\n\"unterminated")
	require.ErrorContains(t, err, "parse csv row 2")

	_, err = parseWarehouseImportRows("code,address\nMAIN,Tallinn\n")
	require.ErrorContains(t, err, "missing required columns: name")

	rows, err := parseWarehouseImportRows("\ufeffwarehouse_code;warehouse_name;default;status;legacy.column\nMAIN;Main;yes;ACTIVE;ignored\n;;;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "MAIN", rows[0].values["code"])
	assert.Equal(t, "Main", rows[0].values["name"])
	assert.Equal(t, "ignored", rows[0].values["legacy_column"])

	warehouse, err := buildWarehouseFromImportRow(warehouseImportRow{values: map[string]string{
		"code":       " MAIN ",
		"name":       " Main warehouse ",
		"address":    " Tallinn ",
		"is_default": "true",
		"status":     "INACTIVE",
	}}, "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", warehouse.TenantID)
	assert.Equal(t, "MAIN", warehouse.Code)
	assert.Equal(t, "Main warehouse", warehouse.Name)
	assert.Equal(t, "Tallinn", warehouse.Address)
	assert.True(t, warehouse.IsDefault)
	assert.False(t, warehouse.IsActive)

	_, err = buildWarehouseFromImportRow(warehouseImportRow{values: map[string]string{"name": "Main"}}, "tenant-1")
	require.ErrorContains(t, err, "code is required")

	_, err = buildWarehouseFromImportRow(warehouseImportRow{values: map[string]string{"code": "MAIN"}}, "tenant-1")
	require.ErrorContains(t, err, "name is required")

	_, err = buildWarehouseFromImportRow(warehouseImportRow{values: map[string]string{"code": "MAIN", "name": "Main", "is_default": "maybe"}}, "tenant-1")
	require.ErrorContains(t, err, "is_default must be true or false")

	active, err := parseWarehouseImportActive("ACTIVE", "false")
	require.NoError(t, err)
	assert.True(t, active)

	active, err = parseWarehouseImportActive("", "")
	require.NoError(t, err)
	assert.True(t, active)

	active, err = parseWarehouseImportActive("", "no")
	require.NoError(t, err)
	assert.False(t, active)

	_, err = parseWarehouseImportActive("ARCHIVED", "")
	require.ErrorContains(t, err, "invalid status")

	_, err = parseWarehouseImportActive("", "maybe")
	require.ErrorContains(t, err, "is_active must be true or false")

	assert.Equal(t, "is_default", canonicalWarehouseImportHeader(" default-warehouse "))
	assert.Equal(t, "legacy_column", canonicalWarehouseImportHeader("legacy column"))
}

func TestStockImportRowsAndHelpersEdges(t *testing.T) {
	_, err := parseStockImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseStockImportRows(`"unterminated`)
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseStockImportRows("product_code,warehouse_code,quantity\n\"unterminated")
	require.ErrorContains(t, err, "parse csv row 2")

	_, err = parseStockImportRows("product_code,warehouse_code\nSKU-1,MAIN\n")
	require.ErrorContains(t, err, "missing required columns: quantity")

	_, err = parseStockImportRows("warehouse_code,quantity\nMAIN,1\n")
	require.ErrorContains(t, err, "missing required product identifier column")

	_, err = parseStockImportRows("product_code,quantity\nSKU-1,1\n")
	require.ErrorContains(t, err, "missing required warehouse identifier column")

	rows, err := parseStockImportRows("\ufeffsku;warehouse;qty;cost;serial;legacy.column\nSKU-1;MAIN;1;10;SN-1;ignored\n;;;;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "SKU-1", rows[0].values["product_code"])
	assert.Equal(t, "MAIN", rows[0].values["warehouse_code"])
	assert.Equal(t, "ignored", rows[0].values["legacy_column"])

	productID := "11111111-1111-4111-8111-111111111111"
	warehouseID := "22222222-2222-4222-8222-222222222222"
	req, err := buildStockAdjustmentFromImportRow(stockImportRow{values: map[string]string{
		"product_code":   " SKU-1 ",
		"warehouse_code": " MAIN ",
		"quantity":       "1",
		"unit_cost":      "10.50",
		"lot_number":     " LOT-1 ",
		"serial_number":  " SN-1 ",
		"expiry_date":    " 2027-01-31 ",
		"reason":         " Opening stock ",
	}}, "user-1", map[string]string{"sku-1": productID}, map[string]string{"main": warehouseID})
	require.NoError(t, err)
	assert.Equal(t, productID, req.ProductID)
	assert.Equal(t, warehouseID, req.WarehouseID)
	assert.Equal(t, "1", req.Quantity)
	assert.Equal(t, "10.50", req.UnitCost)
	assert.Equal(t, "LOT-1", req.LotNumber)
	assert.Equal(t, "SN-1", req.SerialNumber)
	assert.Equal(t, "2027-01-31", req.ExpiryDate)
	assert.Equal(t, "Opening stock", req.Reason)
	assert.Equal(t, "user-1", req.UserID)

	tests := []struct {
		name        string
		values      map[string]string
		wantMessage string
	}{
		{
			name:        "missing product reference",
			values:      map[string]string{"warehouse_code": "MAIN", "quantity": "1"},
			wantMessage: "product_id or product_code is required",
		},
		{
			name:        "unknown product code",
			values:      map[string]string{"product_code": "MISSING", "warehouse_code": "MAIN", "quantity": "1"},
			wantMessage: `product_code "MISSING" was not found`,
		},
		{
			name:        "missing warehouse reference",
			values:      map[string]string{"product_code": "SKU-1", "quantity": "1"},
			wantMessage: "warehouse_id or warehouse_code is required",
		},
		{
			name:        "unknown warehouse code",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MISSING", "quantity": "1"},
			wantMessage: `warehouse_code "MISSING" was not found`,
		},
		{
			name:        "quantity required",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MAIN"},
			wantMessage: "quantity is required",
		},
		{
			name:        "quantity decimal",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MAIN", "quantity": "one"},
			wantMessage: "quantity must be a decimal",
		},
		{
			name:        "quantity not zero",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MAIN", "quantity": "0"},
			wantMessage: "quantity must not be zero",
		},
		{
			name:        "serial quantity",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MAIN", "quantity": "2", "serial_number": "SN-1"},
			wantMessage: "serial_number requires quantity 1 or -1",
		},
		{
			name:        "unit cost decimal",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MAIN", "quantity": "1", "unit_cost": "ten"},
			wantMessage: "unit_cost must be a decimal",
		},
		{
			name:        "unit cost nonnegative",
			values:      map[string]string{"product_code": "SKU-1", "warehouse_code": "MAIN", "quantity": "1", "unit_cost": "-1"},
			wantMessage: "unit_cost cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildStockAdjustmentFromImportRow(stockImportRow{values: tt.values}, "user-1", map[string]string{"sku-1": productID}, map[string]string{"main": warehouseID})

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}

	assert.Empty(t, stockImportSerialKey("", "SN-1"))
	assert.Empty(t, stockImportSerialKey(productID, " "))
	assert.Equal(t, productID+"\x00sn-1", stockImportSerialKey(productID, " SN-1 "))
	assert.Equal(t, ';', detectStockImportDelimiter("sku;warehouse;qty\nSKU-1;MAIN;1"))
	assert.Equal(t, '\t', detectStockImportDelimiter("sku\twarehouse\tqty\nSKU-1\tMAIN\t1"))
	assert.Equal(t, ',', detectStockImportDelimiter("sku,warehouse,qty\nSKU-1,MAIN,1"))
	assert.Equal(t, "expiry_date", canonicalStockImportHeader(" expiration-date "))
	assert.Equal(t, "legacy_column", canonicalStockImportHeader("legacy column"))
	assert.Equal(t, "first", firstNonBlank(" ", " first ", "second"))
	assert.Empty(t, firstNonBlank(" ", ""))
}
