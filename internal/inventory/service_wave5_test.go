package inventory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inventoryWave5Accounting struct {
	accounts []accounting.Account
	balances map[string]decimal.Decimal
	err      error
}

func (a *inventoryWave5Accounting) ListAccounts(context.Context, string, string, bool) ([]accounting.Account, error) {
	return append([]accounting.Account(nil), a.accounts...), nil
}

func (a *inventoryWave5Accounting) GetAccountBalance(_ context.Context, _, _, accountID string, _ time.Time) (decimal.Decimal, error) {
	if a.err != nil {
		return decimal.Zero, a.err
	}
	return a.balances[accountID], nil
}

func TestInventoryWave5ListProductsNormalizesFilterCategory(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()
	categoryID := "33333333-3333-4333-8333-333333333333"
	repo.Products[inventoryStockProductID] = &Product{
		ID:          inventoryStockProductID,
		TenantID:    "tenant-1",
		Code:        "PRD-1",
		Name:        "Tracked widget",
		ProductType: ProductTypeGoods,
		CategoryID:  categoryID,
		IsActive:    true,
	}

	products, err := service.ListProducts(ctx, "tenant-1", "tenant_schema", &ProductFilter{CategoryID: " " + categoryID + " "})

	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, inventoryStockProductID, products[0].ID)

	products, err = service.ListProducts(ctx, "tenant-1", "tenant_schema", &ProductFilter{CategoryID: "not-a-uuid"})
	assert.Nil(t, products)
	require.ErrorContains(t, err, "category_id must be a valid UUID")
}

func TestInventoryWave5SubledgerReconciliationSortsAccountLines(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	const accountA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const accountB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	repo.Products["product-b"] = &Product{
		ID:                 "product-b",
		TenantID:           "tenant-1",
		Code:               "B",
		Name:               "Bravo",
		ProductType:        ProductTypeGoods,
		TrackInventory:     true,
		PurchasePrice:      decimal.NewFromInt(5),
		CurrentStock:       decimal.NewFromInt(2),
		InventoryAccountID: accountB,
	}
	repo.Products["product-a"] = &Product{
		ID:                 "product-a",
		TenantID:           "tenant-1",
		Code:               "A",
		Name:               "Alpha",
		ProductType:        ProductTypeGoods,
		TrackInventory:     true,
		PurchasePrice:      decimal.NewFromInt(10),
		CurrentStock:       decimal.NewFromInt(3),
		InventoryAccountID: accountA,
	}
	repo.StockLevels[inventoryStockLevelKey("product-b", inventoryStockWarehouseID)] = &StockLevel{
		TenantID:     "tenant-1",
		ProductID:    "product-b",
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(2),
		AvailableQty: decimal.NewFromInt(2),
	}
	repo.StockLevels[inventoryStockLevelKey("product-a", inventoryStockWarehouseID)] = &StockLevel{
		TenantID:     "tenant-1",
		ProductID:    "product-a",
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(3),
		AvailableQty: decimal.NewFromInt(3),
	}
	repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main",
	}
	accounts := &inventoryWave5Accounting{
		accounts: []accounting.Account{
			{ID: accountB, Code: "1400", Name: "Inventory B", AccountType: accounting.AccountTypeAsset},
			{ID: accountA, Code: "1300", Name: "Inventory A", AccountType: accounting.AccountTypeAsset},
		},
		balances: map[string]decimal.Decimal{
			accountA: decimal.NewFromInt(30),
			accountB: decimal.NewFromInt(10),
		},
	}
	service := NewServiceWithRepositoryAndAccounting(repo, accounts)

	report, err := service.GetInventorySubledgerReconciliation(ctx, "tenant-1", "tenant_schema", "", InventoryValuationMethodStandardCost, time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	require.Len(t, report.AccountLines, 2)
	assert.Equal(t, "1300", report.AccountLines[0].AccountCode)
	assert.Equal(t, "1400", report.AccountLines[1].AccountCode)
	assert.True(t, report.Ready)
	assert.True(t, report.TotalDifference.IsZero())
}

func TestInventoryWave5SubledgerReconciliationBalanceError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	accountID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	repo.Products[inventoryStockProductID] = &Product{
		ID:                 inventoryStockProductID,
		TenantID:           "tenant-1",
		Code:               "PRD",
		Name:               "Widget",
		ProductType:        ProductTypeGoods,
		TrackInventory:     true,
		PurchasePrice:      decimal.NewFromInt(10),
		CurrentStock:       decimal.NewFromInt(1),
		InventoryAccountID: accountID,
	}
	repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(1),
		AvailableQty: decimal.NewFromInt(1),
	}
	service := NewServiceWithRepositoryAndAccounting(repo, &inventoryWave5Accounting{
		accounts: []accounting.Account{{ID: accountID, Code: "1300", Name: "Inventory", AccountType: accounting.AccountTypeAsset}},
		err:      errors.New("balance unavailable"),
	})

	report, err := service.GetInventorySubledgerReconciliation(ctx, "tenant-1", "tenant_schema", "", InventoryValuationMethodStandardCost, time.Time{})

	assert.Nil(t, report)
	require.ErrorContains(t, err, "get account balance for inventory account")
	assert.ErrorContains(t, err, "balance unavailable")
}

func TestInventoryWave5IssueStockAllocationsAddsUntrackedRemainder(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	product := Product{
		ID:            inventoryStockProductID,
		TenantID:      "tenant-1",
		PurchasePrice: decimal.NewFromInt(9),
	}
	level := &StockLevel{
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(5),
		AvailableQty: decimal.NewFromInt(5),
	}
	movements := []InventoryMovement{
		{
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(2),
			UnitCost:     decimal.NewFromInt(7),
			TotalCost:    decimal.NewFromInt(14),
			LotNumber:    "LOT-A",
			MovementDate: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	allocations, err := service.issueStockAllocations(context.Background(), "tenant-1", "tenant_schema", product, level, movements, decimal.NewFromInt(5), InventoryIssueCostingMethodLot, "", "", "")

	require.NoError(t, err)
	require.Len(t, allocations, 2)
	assert.Equal(t, "LOT-A", allocations[0].key.lotNumber)
	assert.True(t, allocations[0].quantity.Equal(decimal.NewFromInt(2)))
	assert.Empty(t, allocations[1].key.lotNumber)
	assert.True(t, allocations[1].quantity.Equal(decimal.NewFromInt(3)))
	assert.True(t, allocations[1].unitCost.Equal(decimal.NewFromInt(7)))
}

func TestInventoryWave5LotReservationHelperBranches(t *testing.T) {
	ctx := context.Background()
	service := NewServiceWithRepository(NewMockRepository())

	require.NoError(t, service.createLotReservation(ctx, "tenant_schema", "tenant-1", inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "", decimal.Zero, " ignored ", " user-1 "))

	repo := NewMockRepository()
	repo.ErrOnUpsertLotReservation = true
	service = NewServiceWithRepository(repo)
	err := service.createLotReservation(ctx, "tenant_schema", "tenant-1", inventoryStockProductID, inventoryStockWarehouseID, " LOT-A ", " SN-1 ", " 2027-01-31 ", decimal.NewFromInt(1), " reserve ", " user-1 ")
	require.ErrorContains(t, err, "reserve tracked lot stock")

	repo = NewMockRepository()
	service = NewServiceWithRepository(repo)
	req := &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-A",
		Reason:      "release",
		UserID:      "user-1",
	}
	err = service.releaseLotAllocations(ctx, "tenant-1", "tenant_schema", req, decimal.NewFromInt(1))
	require.ErrorContains(t, err, "cannot release more than reserved tracked lot stock")

	repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-A",
		Quantity:    decimal.NewFromInt(2),
	}
	repo.ErrOnReleaseLotReservation = true
	err = service.releaseLotAllocations(ctx, "tenant-1", "tenant_schema", req, decimal.NewFromInt(1))
	require.ErrorContains(t, err, "release tracked lot stock")

	repo = NewMockRepository()
	service = NewServiceWithRepository(repo)
	repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-A",
		Quantity:    decimal.NewFromInt(1),
	}
	repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-B", "", "")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-B",
		Quantity:    decimal.NewFromInt(1),
	}
	req.LotNumber = ""
	require.NoError(t, service.releaseLotAllocations(ctx, "tenant-1", "tenant_schema", req, decimal.NewFromInt(1)))
}
