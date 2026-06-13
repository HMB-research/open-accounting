package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostCenterGORMRepository_NilDatabase(t *testing.T) {
	repo := NewCostCenterGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	costCenterID := "cc-1"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				costCenter, err := repo.GetByID(ctx, schemaName, tenantID, costCenterID)
				assert.Nil(t, costCenter)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				costCenters, err := repo.List(ctx, schemaName, tenantID, true)
				assert.Nil(t, costCenters)
				return err
			},
		},
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, schemaName, &CostCenter{TenantID: tenantID, Code: "OPS", Name: "Operations"})
			},
		},
		{
			name: "Update",
			run: func(t *testing.T) error {
				return repo.Update(ctx, schemaName, &CostCenter{ID: costCenterID, TenantID: tenantID, Code: "OPS", Name: "Operations"})
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				return repo.Delete(ctx, schemaName, tenantID, costCenterID)
			},
		},
		{
			name: "GetExpensesByPeriod",
			run: func(t *testing.T) error {
				total, err := repo.GetExpensesByPeriod(ctx, schemaName, tenantID, costCenterID, startDate, endDate)
				assert.True(t, total.IsZero())
				return err
			},
		},
		{
			name: "CreateAllocation",
			run: func(t *testing.T) error {
				return repo.CreateAllocation(ctx, schemaName, &CostAllocation{
					TenantID:           tenantID,
					CostCenterID:       costCenterID,
					JournalEntryLineID: "line-1",
					Amount:             decimal.RequireFromString("100.00"),
					AllocationDate:     startDate,
				})
			},
		},
		{
			name: "ListAllocations",
			run: func(t *testing.T) error {
				allocations, err := repo.ListAllocations(ctx, schemaName, tenantID, CostAllocationFilters{
					CostCenterID:       costCenterID,
					JournalEntryLineID: "line-1",
					StartDate:          &startDate,
					EndDate:            &endDate,
				})
				assert.Nil(t, allocations)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				db, err := repo.tenantTable(ctx, schemaName, "cost_centers")
				assert.Nil(t, db)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cost center repository database is not configured")
		})
	}
}

func TestCostCenterModelMappingRoundTrip(t *testing.T) {
	parentID := "parent-cost-center-id"
	budget := decimal.RequireFromString("12500.75")
	createdAt := time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)
	costCenter := &CostCenter{
		ID:           "cost-center-id",
		TenantID:     "tenant-id",
		Code:         "OPS",
		Name:         "Operations",
		Description:  "Operations department",
		ParentID:     &parentID,
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: BudgetPeriodQuarterly,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	model := costCenterToModel(costCenter)
	require.NotNil(t, model.BudgetAmount)
	assert.True(t, model.BudgetAmount.Decimal.Equal(budget))
	assert.Equal(t, string(BudgetPeriodQuarterly), model.BudgetPeriod)

	roundTrip := costCenterFromModel(model)
	assert.Equal(t, costCenter, roundTrip)
}

func TestCostCenterBudgetAmountMappingNil(t *testing.T) {
	assert.Nil(t, costCenterBudgetAmountToModel(nil))
	assert.Nil(t, costCenterBudgetAmountFromModel(nil))
}

func TestCostAllocationModelMappingRoundTrip(t *testing.T) {
	percentage := decimal.RequireFromString("37.5")
	allocationDate := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.June, 1, 8, 30, 0, 0, time.UTC)
	allocation := &CostAllocation{
		ID:                   "allocation-id",
		TenantID:             "tenant-id",
		CostCenterID:         "cost-center-id",
		JournalEntryLineID:   "journal-line-id",
		Amount:               decimal.RequireFromString("250.45"),
		AllocationPercentage: &percentage,
		AllocationDate:       allocationDate,
		Notes:                "Allocated payroll expense",
		CreatedAt:            createdAt,
	}

	model := costAllocationToModel(allocation)
	assert.True(t, model.Amount.Decimal.Equal(allocation.Amount))
	require.NotNil(t, model.AllocationPercentage)
	assert.True(t, model.AllocationPercentage.Decimal.Equal(percentage))

	roundTrip := costAllocationFromModel(model)
	assert.Equal(t, allocation, roundTrip)
}
