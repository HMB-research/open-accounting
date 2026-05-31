package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestCostCenterRepository_CRUDAndReportData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewCostCenterRepository(pool)
	ctx := context.Background()

	budget := decimal.NewFromInt(1200)
	cc := &CostCenter{
		TenantID:     tenant.ID,
		Code:         "ADMIN",
		Name:         "Administration",
		Description:  "Back office costs",
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: BudgetPeriodAnnual,
	}

	if err := repo.Create(ctx, tenant.SchemaName, cc); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if cc.ID == "" {
		t.Fatal("expected cost center ID to be populated")
	}

	got, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, cc.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Code != "ADMIN" || got.Name != "Administration" {
		t.Fatalf("unexpected cost center: %+v", got)
	}

	all, err := repo.List(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 cost center, got %d", len(all))
	}

	updatedBudget := decimal.NewFromInt(1500)
	got.Name = "Administration and HR"
	got.Description = "Updated description"
	got.BudgetAmount = &updatedBudget
	if err := repo.Update(ctx, tenant.SchemaName, got); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	reloaded, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, cc.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if reloaded.Name != "Administration and HR" || !reloaded.BudgetAmount.Equal(updatedBudget) {
		t.Fatalf("unexpected updated cost center: %+v", reloaded)
	}

	start := time.Now().AddDate(0, -1, 0).Truncate(24 * time.Hour)
	inRangeDate := start.AddDate(0, 0, 5)
	outOfRangeDate := start.AddDate(0, -2, 0)

	gormDB, err := database.NewGormDBFromPool(ctx, pool)
	if err != nil {
		t.Fatalf("failed to create gorm db: %v", err)
	}
	allocationsTable, err := database.TenantTable(gormDB, tenant.SchemaName, "cost_allocations")
	if err != nil {
		t.Fatalf("failed to qualify cost allocations table: %v", err)
	}
	if err := allocationsTable.Create([]models.CostAllocation{
		{
			ID:                 uuid.New().String(),
			TenantID:           tenant.ID,
			CostCenterID:       cc.ID,
			JournalEntryLineID: uuid.New().String(),
			Amount:             models.NewDecimal(decimal.NewFromInt(250)),
			AllocationDate:     inRangeDate,
			Notes:              "in range",
		},
		{
			ID:                 uuid.New().String(),
			TenantID:           tenant.ID,
			CostCenterID:       cc.ID,
			JournalEntryLineID: uuid.New().String(),
			Amount:             models.NewDecimal(decimal.NewFromInt(999)),
			AllocationDate:     outOfRangeDate,
			Notes:              "out of range",
		},
	}).Error; err != nil {
		t.Fatalf("failed to seed cost allocations: %v", err)
	}

	expenses, err := repo.GetExpensesByPeriod(ctx, tenant.SchemaName, tenant.ID, cc.ID, start, time.Now())
	if err != nil {
		t.Fatalf("GetExpensesByPeriod failed: %v", err)
	}
	if !expenses.Equal(decimal.NewFromInt(250)) {
		t.Fatalf("expected in-range expenses of 250, got %s", expenses)
	}

	if err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, cc.ID); err == nil {
		t.Fatal("expected delete to fail while allocations exist")
	}

	if err := allocationsTable.Where("cost_center_id = ?", cc.ID).Delete(&models.CostAllocation{}).Error; err != nil {
		t.Fatalf("failed to remove test allocations: %v", err)
	}

	if err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, cc.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, cc.ID); err == nil {
		t.Fatal("expected deleted cost center to be missing")
	}
}

func TestNewCostCenterServiceUsesRepository(t *testing.T) {
	svc := NewCostCenterService(nil)
	if svc == nil || svc.repo == nil {
		t.Fatal("expected cost center service with repository")
	}
}
