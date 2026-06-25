package inventory

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contactrefs"
)

func TestInventoryImportWave8ProductServiceBranches(t *testing.T) {
	ctx := context.Background()
	req := &ImportProductsRequest{CSVContent: "name,sales_price\nWidget,10\n"}

	t.Run("returns product parse errors", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())

		_, err := service.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{CSVContent: `"unterminated`})

		require.ErrorContains(t, err, "parse csv header")
	})

	t.Run("returns no product rows error", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())

		_, err := service.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{CSVContent: "name,sales_price\n,\n"})

		require.ErrorContains(t, err, "no products found in CSV")
	})

	t.Run("wraps product list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ErrOnListProducts = true
		service := NewServiceWithRepository(repo)

		_, err := service.ImportProductsCSV(ctx, "tenant-1", "test_schema", req)

		require.ErrorContains(t, err, "list existing products")
	})

	t.Run("wraps category list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ErrOnListCategories = true
		service := NewServiceWithRepository(repo)

		_, err := service.ImportProductsCSV(ctx, "tenant-1", "test_schema", req)

		require.ErrorContains(t, err, "list product categories")
	})

	t.Run("records generated code errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ErrOnGenerate = true
		service := NewServiceWithRepository(repo)

		result, err := service.ImportProductsCSV(ctx, "tenant-1", "test_schema", req)

		require.NoError(t, err)
		assert.Zero(t, result.ProductsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "generate code")
	})

	rows, err := parseProductImportRows("name,sales_price,\n,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	for _, tt := range []struct {
		name string
		key  string
		val  string
		want string
	}{
		{name: "invalid purchase price", key: "purchase_price", val: "costly", want: "purchase_price must be a decimal"},
		{name: "invalid sales price", key: "sales_price", val: "free", want: "sales_price must be a decimal"},
		{name: "invalid VAT rate", key: "vat_rate", val: "tax", want: "vat_rate must be a decimal"},
		{name: "invalid minimum stock", key: "min_stock_level", val: "low", want: "min_stock_level must be a decimal"},
		{name: "invalid reorder point", key: "reorder_point", val: "soon", want: "reorder_point must be a decimal"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := productImportRow{rowNumber: 2, values: map[string]string{
				"name":        "Widget",
				"sales_price": "10",
			}}
			row.values[tt.key] = tt.val

			_, err := buildProductFromImportRow(row, "tenant-1", nil, nil, nil, contactrefs.NewSupplierLookup(nil))

			require.ErrorContains(t, err, tt.want)
		})
	}

	accountID := "11111111-1111-4111-8111-111111111111"
	got, err := resolveOptionalProductImportAccountID(productImportRow{
		values: map[string]string{"sale_account_id": accountID},
	}, "sale_account_id", "sale_account_code", nil)
	require.NoError(t, err)
	assert.Equal(t, accountID, got)
}

func TestInventoryImportWave8CategoryBranches(t *testing.T) {
	ctx := context.Background()

	service := NewServiceWithRepository(NewMockRepository())
	_, err := service.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", nil)
	require.ErrorContains(t, err, "csv_content is required")

	_, err = service.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{CSVContent: `"unterminated`})
	require.ErrorContains(t, err, "parse csv header")

	_, err = service.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{CSVContent: "name\n \n"})
	require.ErrorContains(t, err, "no product categories found in CSV")

	rows, err := parseCategoryImportRows("name,\n,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	repo := NewMockRepository()
	repo.Categories["blank"] = &ProductCategory{ID: "blank", TenantID: "tenant-1", Name: " "}
	service = NewServiceWithRepository(repo)

	result, err := service.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{
		CSVContent: "name\nWidgets\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.CategoriesCreated)
	assert.Empty(t, result.Errors)

	repo = NewMockRepository()
	repo.ErrOnCreateCategory = true
	service = NewServiceWithRepository(repo)

	result, err = service.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{
		CSVContent: "name\nWidgets\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.CategoriesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "mock error on create category")
}

func TestInventoryImportWave8WarehouseBranches(t *testing.T) {
	ctx := context.Background()

	service := NewServiceWithRepository(NewMockRepository())
	_, err := service.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", nil)
	require.ErrorContains(t, err, "csv_content is required")

	_, err = service.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{CSVContent: `"unterminated`})
	require.ErrorContains(t, err, "parse csv header")

	_, err = service.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{CSVContent: "code,name\n,\n"})
	require.ErrorContains(t, err, "no warehouses found in CSV")

	rows, err := parseWarehouseImportRows("code,name,\n,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	_, err = buildWarehouseFromImportRow(warehouseImportRow{values: map[string]string{
		"code":       "MAIN",
		"name":       "Main",
		"is_default": "maybe",
	}}, "tenant-1")
	require.ErrorContains(t, err, "is_default")

	_, err = buildWarehouseFromImportRow(warehouseImportRow{values: map[string]string{
		"code":      "MAIN",
		"name":      "Main",
		"is_active": "perhaps",
	}}, "tenant-1")
	require.ErrorContains(t, err, "is_active")

	repo := NewMockRepository()
	repo.Warehouses["blank"] = &Warehouse{ID: "blank", TenantID: "tenant-1", Code: " ", Name: "Blank"}
	service = NewServiceWithRepository(repo)

	result, err := service.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{
		CSVContent: "code,name\nMAIN,Main warehouse\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.WarehousesCreated)
	assert.Empty(t, result.Errors)

	repo = NewMockRepository()
	repo.ErrOnCreateWarehouse = true
	service = NewServiceWithRepository(repo)

	result, err = service.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{
		CSVContent: "code,name\nMAIN,Main warehouse\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.WarehousesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "mock error on create warehouse")
}

func TestInventoryImportWave8StockBranches(t *testing.T) {
	ctx := context.Background()

	service := NewServiceWithRepository(NewMockRepository())
	_, err := service.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", nil)
	require.ErrorContains(t, err, "csv_content is required")

	_, err = service.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{CSVContent: `"unterminated`})
	require.ErrorContains(t, err, "parse csv header")

	_, err = service.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{CSVContent: "product_code,warehouse_code,quantity\n,,\n"})
	require.ErrorContains(t, err, "no stock adjustments found in CSV")

	rows, err := parseStockImportRows("product_code,warehouse_code,quantity,\n,,,ignored\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	repo := NewMockRepository()
	repo.ErrOnListProducts = true
	service = NewServiceWithRepository(repo)

	_, err = service.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
		CSVContent: "product_code,warehouse_code,quantity\nSKU-1,MAIN,1\n",
	})
	require.ErrorContains(t, err, "list products")

	repo = NewMockRepository()
	repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Code:           "SKU-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		TrackInventory: true,
	}
	repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main",
		IsActive: true,
	}
	repo.ErrOnGet = true
	service = NewServiceWithRepository(repo)

	result, err := service.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
		CSVContent: "product_code,warehouse_code,quantity\nSKU-1,MAIN,1\n",
	})

	require.NoError(t, err)
	assert.Zero(t, result.AdjustmentsImported)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.True(t, strings.Contains(result.Errors[0].Message, "get product") || strings.Contains(result.Errors[0].Message, "mock error on get"))
}
