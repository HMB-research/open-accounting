package inventory

import (
	"context"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestInventoryWave8ReservationQuantityValidation(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	req := &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "0",
	}

	if _, err := service.ReserveStock(context.Background(), "tenant-1", "tenant_demo", req); err == nil || !strings.Contains(err.Error(), "quantity must be positive") {
		t.Fatalf("ReserveStock() error = %v, want quantity validation", err)
	}
	if _, err := service.ReleaseStock(context.Background(), "tenant-1", "tenant_demo", req); err == nil || !strings.Contains(err.Error(), "quantity must be positive") {
		t.Fatalf("ReleaseStock() error = %v, want quantity validation", err)
	}
}

func TestInventoryWave8LotAllocationEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("reserve tracked lot with no positions", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())
		err := service.reserveLotAllocations(ctx, "tenant-1", "tenant_demo",
			Product{ID: inventoryStockProductID, TenantID: "tenant-1"},
			&StockLevel{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID},
			&StockReservationRequest{LotNumber: "LOT-A"},
			decimal.NewFromInt(1),
		)
		if err == nil || !strings.Contains(err.Error(), "insufficient available tracked lot stock") {
			t.Fatalf("reserveLotAllocations() error = %v, want tracked stock error", err)
		}
	})

	t.Run("issue skips fully reserved tracked position", func(t *testing.T) {
		repo := NewMockRepository()
		repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "")] = &InventoryLotReservation{
			TenantID:    "tenant-1",
			ProductID:   inventoryStockProductID,
			WarehouseID: inventoryStockWarehouseID,
			LotNumber:   "LOT-A",
			Quantity:    decimal.NewFromInt(1),
		}
		service := NewServiceWithRepository(repo)
		product := Product{ID: inventoryStockProductID, TenantID: "tenant-1", PurchasePrice: decimal.NewFromInt(4)}
		level := &StockLevel{
			ProductID:   inventoryStockProductID,
			WarehouseID: inventoryStockWarehouseID,
			Quantity:    decimal.NewFromInt(3),
			ReservedQty: decimal.NewFromInt(1),
		}
		movements := []InventoryMovement{{
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(1),
			UnitCost:     decimal.NewFromInt(5),
			TotalCost:    decimal.NewFromInt(5),
			LotNumber:    "LOT-A",
		}}

		allocations, err := service.issueStockAllocations(ctx, "tenant-1", "tenant_demo", product, level, movements, decimal.NewFromInt(2), InventoryIssueCostingMethodLot, "", "", "")
		if err != nil {
			t.Fatalf("issueStockAllocations() error = %v", err)
		}
		if len(allocations) != 1 || allocations[0].key.lotNumber != "" || !allocations[0].quantity.Equal(decimal.NewFromInt(2)) {
			t.Fatalf("issueStockAllocations() allocations = %#v, want untracked fallback only", allocations)
		}
	})
}

func TestInventoryWave8FIFOCostSkipsNonPositiveLayers(t *testing.T) {
	product := Product{ID: inventoryStockProductID, TenantID: "tenant-1", PurchasePrice: decimal.NewFromInt(9)}
	got := fifoInventoryUnitCost(product, []InventoryMovement{{
		TenantID:     "tenant-1",
		MovementType: MovementTypeIn,
		Quantity:     decimal.Zero,
		UnitCost:     decimal.NewFromInt(5),
	}}, "tenant-1", decimal.NewFromInt(1))
	if !got.Equal(decimal.NewFromInt(9)) {
		t.Fatalf("fifoInventoryUnitCost() = %s, want purchase price fallback", got)
	}
}
