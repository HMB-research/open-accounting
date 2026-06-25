package inventory

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryWave11StockImportSuccessClearsErrors(t *testing.T) {
	repo := NewMockRepository()
	productID := "11111111-1111-4111-8111-111111111111"
	warehouseID := "22222222-2222-4222-8222-222222222222"
	repo.Products[productID] = &Product{
		ID:             productID,
		TenantID:       "tenant-1",
		Code:           "SKU-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		CurrentStock:   decimal.NewFromInt(5),
		TrackInventory: true,
	}
	repo.Warehouses[warehouseID] = &Warehouse{
		ID:       warehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main",
		IsActive: true,
	}
	service := NewServiceWithRepository(repo)

	result, err := service.ImportStockAdjustmentsCSV(context.Background(), "tenant-1", "tenant_test", &ImportStockAdjustmentsRequest{
		FileName:   "stock.csv",
		UserID:     "user-1",
		CSVContent: "product_code,warehouse_code,quantity,unit_cost\nSKU-1,MAIN,2,3.50\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "stock.csv", result.FileName)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.AdjustmentsImported)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)
}

func TestInventoryWave11WarehouseImportRecordsBuildErrors(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())

	result, err := service.ImportWarehousesCSV(context.Background(), "tenant-1", "tenant_test", &ImportWarehousesRequest{
		CSVContent: "code,name\n,Main warehouse\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.WarehousesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "Main warehouse", result.Errors[0].Name)
	assert.Contains(t, result.Errors[0].Message, "code is required")
}
