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

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	costCenter, err := repo.GetByID(context.Background(), "tenant_schema", "tenant-1", "cc-1")
	require.Error(t, err)
	assert.Nil(t, costCenter)
	assert.Contains(t, err.Error(), "cost center repository database is not configured")
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
