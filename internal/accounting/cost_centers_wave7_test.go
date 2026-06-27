package accounting

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestCostCentersWave7NilRepositoryGuards(t *testing.T) {
	ctx := context.Background()
	repo := NewCostCenterRepository(nil)
	if repo == nil || repo.db != nil {
		t.Fatalf("NewCostCenterRepository(nil) = %#v, want repository with nil db", repo)
	}

	_, err := repo.GetByID(ctx, "tenant_demo", "tenant-1", "cc-1")
	if err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("GetByID() error = %v, want nil database guard", err)
	}
	if err := repo.Delete(ctx, "tenant_demo", "tenant-1", "cc-1"); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("Delete() error = %v, want nil database guard", err)
	}
	_, err = repo.GetExpensesByPeriod(ctx, "tenant_demo", "tenant-1", "cc-1", time.Now(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("GetExpensesByPeriod() error = %v, want nil database guard", err)
	}
	if err := repo.CreateAllocation(ctx, "tenant_demo", &CostAllocation{TenantID: "tenant-1"}); err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("CreateAllocation() error = %v, want nil database guard", err)
	}
	_, err = repo.ListAllocations(ctx, "tenant_demo", "tenant-1", CostAllocationFilters{})
	if err == nil || !strings.Contains(err.Error(), "database is not configured") {
		t.Fatalf("ListAllocations() error = %v, want nil database guard", err)
	}
}

func TestCostCentersWave7ReportAndAllocationValidation(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	costCenterID := "11111111-1111-4111-8111-111111111111"
	otherCostCenterID := "22222222-2222-4222-8222-222222222222"
	lineID := "33333333-3333-4333-8333-333333333333"
	budget := decimal.NewFromInt(100)
	repo := NewMockCostCenterRepository()
	repo.CostCenters[costCenterID] = &CostCenter{
		ID:           costCenterID,
		TenantID:     tenantID,
		Code:         "OPS",
		Name:         "Operations",
		IsActive:     true,
		BudgetAmount: &budget,
	}
	repo.CostCenters[otherCostCenterID] = &CostCenter{
		ID:       otherCostCenterID,
		TenantID: tenantID,
		Code:     "ADMIN",
		Name:     "Admin",
		IsActive: true,
	}
	repo.Allocations[costCenterID] = []CostAllocation{{
		ID:                 "alloc-1",
		TenantID:           tenantID,
		CostCenterID:       costCenterID,
		JournalEntryLineID: lineID,
		Amount:             decimal.NewFromInt(125),
		AllocationDate:     time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
	}}
	service := NewCostCenterServiceWithRepository(repo)

	report, err := service.GetCostCenterReport(ctx, "tenant_demo", tenantID, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetCostCenterReport() error = %v", err)
	}
	if len(report.CostCenters) != 2 || !report.TotalExpenses.Equal(decimal.NewFromInt(125)) || !report.TotalBudget.Equal(budget) {
		t.Fatalf("GetCostCenterReport() = %#v", report)
	}
	if !report.CostCenters[0].IsOverBudget && !report.CostCenters[1].IsOverBudget {
		t.Fatalf("expected one cost center to be over budget: %#v", report.CostCenters)
	}

	badEnd := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err = service.ListCostAllocations(ctx, "tenant_demo", tenantID, CostAllocationFilters{StartDate: &start, EndDate: &badEnd})
	if err == nil || !strings.Contains(err.Error(), "end_date") {
		t.Fatalf("ListCostAllocations() date error = %v", err)
	}
	_, err = service.ListCostAllocations(ctx, "tenant_demo", tenantID, CostAllocationFilters{CostCenterID: "bad"})
	if err == nil || !strings.Contains(err.Error(), "cost_center_id must be a valid UUID") {
		t.Fatalf("ListCostAllocations() uuid error = %v", err)
	}

	tooHigh := decimal.NewFromInt(101)
	_, err = service.CreateCostAllocation(ctx, "tenant_demo", tenantID, &CreateCostAllocationRequest{
		CostCenterID:         costCenterID,
		JournalEntryLineID:   lineID,
		Amount:               decimal.NewFromInt(10),
		AllocationDate:       time.Now(),
		AllocationPercentage: &tooHigh,
	})
	if err == nil || !strings.Contains(err.Error(), "allocation_percentage") {
		t.Fatalf("CreateCostAllocation() percentage error = %v", err)
	}
	_, err = service.CreateCostAllocation(ctx, "tenant_demo", tenantID, &CreateCostAllocationRequest{
		CostCenterID:       costCenterID,
		JournalEntryLineID: lineID,
		Amount:             decimal.Zero,
		AllocationDate:     time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "amount must be greater than zero") {
		t.Fatalf("CreateCostAllocation() amount error = %v", err)
	}
	_, err = service.CreateCostAllocation(ctx, "tenant_demo", tenantID, &CreateCostAllocationRequest{
		CostCenterID:       costCenterID,
		JournalEntryLineID: lineID,
		Amount:             decimal.NewFromInt(10),
	})
	if err == nil || !strings.Contains(err.Error(), "allocation_date is required") {
		t.Fatalf("CreateCostAllocation() date error = %v", err)
	}
}
