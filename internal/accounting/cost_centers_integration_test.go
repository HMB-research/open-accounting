package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/testutil"
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
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.cost_allocations (
			id, tenant_id, cost_center_id, journal_entry_line_id, amount, allocation_date, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7), ($8, $2, $3, $9, $10, $11, $12)
	`,
		uuid.New().String(), tenant.ID, cc.ID, uuid.New().String(), decimal.NewFromInt(250), inRangeDate, "in range",
		uuid.New().String(), uuid.New().String(), decimal.NewFromInt(999), outOfRangeDate, "out of range",
	); err != nil {
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

	if _, err := pool.Exec(ctx, `DELETE FROM `+tenant.SchemaName+`.cost_allocations WHERE cost_center_id = $1`, cc.ID); err != nil {
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
