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
