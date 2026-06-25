package inventory

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryWave4ReleaseLotAllocationsSpansMultipleReservations(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	req := &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Reason:      "customer order cancelled",
		UserID:      "user-1",
	}
	repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-1", "", "")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-1",
		Quantity:    decimal.NewFromInt(2),
	}
	repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2", "", "")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-2",
		Quantity:    decimal.NewFromInt(2),
	}

	err := service.releaseLotAllocations(ctx, "tenant-1", "test_schema", req, decimal.NewFromInt(3))

	require.NoError(t, err)
	assert.True(t, repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-1", "", "")].Quantity.IsZero())
	assert.True(t, repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2", "", "")].Quantity.Equal(decimal.NewFromInt(1)))
}

func TestInventoryWave4IssueAccountingHelperEdges(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	ctx := context.Background()
	product := Product{
		Name:               "Widget",
		InventoryAccountID: "55555555-5555-4555-8555-555555555555",
	}

	accounting, err := service.inventoryIssueAccounting(ctx, "test_schema", "tenant-1", product, &IssueStockRequest{}, "", decimal.NewFromInt(10))
	require.NoError(t, err)
	assert.Nil(t, accounting)

	accounting, err = service.inventoryIssueAccounting(ctx, "test_schema", "tenant-1", product, &IssueStockRequest{
		CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
	}, "source-1", decimal.Zero)
	require.NoError(t, err)
	assert.Nil(t, accounting)

	accounting, err = service.inventoryIssueAccounting(ctx, "test_schema", "tenant-1", product, &IssueStockRequest{
		CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
		InventoryAccountID:       "55555555-5555-4555-8555-555555555555",
		Reference:                "SO-1",
		SourceType:               "SALES_ORDER",
	}, "33333333-3333-4333-8333-333333333333", decimal.NewFromInt(12))
	require.NoError(t, err)
	require.NotNil(t, accounting)
	assert.Equal(t, "SALES_ORDER", accounting.SourceType)
	assert.Equal(t, "SO-1", accounting.Reference)
	require.Len(t, accounting.Lines, 2)
	assert.True(t, accounting.Lines[0].DebitAmount.Equal(decimal.NewFromInt(12)))
	assert.True(t, accounting.Lines[1].CreditAmount.Equal(decimal.NewFromInt(12)))
}

func TestInventoryWave4ReservationQuantityHelpers(t *testing.T) {
	reservations := []InventoryLotReservation{
		{ProductID: "p1", WarehouseID: "w1", LotNumber: " LOT-A ", Quantity: decimal.NewFromInt(3)},
		{ProductID: "p1", WarehouseID: "w1", LotNumber: "LOT-A", Quantity: decimal.NewFromInt(2)},
		{ProductID: "p1", WarehouseID: "w1", LotNumber: "LOT-B", Quantity: decimal.NewFromInt(-1)},
	}

	quantities := inventoryLotReservationQuantities(reservations)
	key := inventoryLotKey{productID: "p1", warehouseID: "w1", lotNumber: "LOT-A"}

	assert.True(t, quantities[key].Equal(decimal.NewFromInt(5)))
	assert.True(t, unallocatedReservedQuantity(decimal.NewFromInt(8), reservations).Equal(decimal.NewFromInt(3)))
	assert.True(t, unallocatedReservedQuantity(decimal.NewFromInt(4), reservations).IsZero())
}
