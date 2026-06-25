package accounting

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCostCentersWave8NilDatabaseRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	repo := NewCostCenterRepository(nil)

	if _, err := repo.GetByID(ctx, "tenant_demo", "tenant-1", "cc-1"); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("GetByID() error = %v, want nil database error", err)
	}

	if _, err := repo.GetExpensesByPeriod(ctx, "tenant_demo", "tenant-1", "cc-1", time.Now().AddDate(0, -1, 0), time.Now()); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("GetExpensesByPeriod() error = %v, want nil database error", err)
	}

	err := repo.CreateAllocation(ctx, "tenant_demo", &CostAllocation{
		TenantID:           "tenant-1",
		CostCenterID:       "11111111-1111-4111-8111-111111111111",
		JournalEntryLineID: "22222222-2222-4222-8222-222222222222",
		Amount:             decimal.NewFromInt(10),
		AllocationDate:     time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("CreateAllocation() error = %v, want nil database error", err)
	}

	if _, err := repo.ListAllocations(ctx, "tenant_demo", "tenant-1", CostAllocationFilters{}); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("ListAllocations() error = %v, want nil database error", err)
	}
}

func TestCostCentersWave8ServiceWithNilDatabaseRepository(t *testing.T) {
	service := NewCostCenterService(nil)
	_, err := service.GetCostCenter(context.Background(), "tenant_demo", "tenant-1", "cc-1")
	if err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("GetCostCenter() error = %v, want nil database error", err)
	}
}
