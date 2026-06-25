package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCostCentersWave9RepositoryConstructorPanicsForGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create cost center GORM repository: pool unavailable", func() {
		_ = NewCostCenterRepository(pool)
	})
}

func TestCostCentersWave9MockAllocationFiltersSkipNonMatches(t *testing.T) {
	repo := NewMockCostCenterRepository()
	allocationDate := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	repo.Allocations["cc-1"] = []CostAllocation{
		{ID: "match", TenantID: "tenant-1", CostCenterID: "cc-1", JournalEntryLineID: "line-1", AllocationDate: allocationDate},
		{ID: "other-tenant", TenantID: "tenant-2", CostCenterID: "cc-1", JournalEntryLineID: "line-1", AllocationDate: allocationDate},
		{ID: "other-line", TenantID: "tenant-1", CostCenterID: "cc-1", JournalEntryLineID: "line-2", AllocationDate: allocationDate},
	}

	allocations, err := repo.ListAllocations(context.Background(), "tenant_demo", "tenant-1", CostAllocationFilters{
		CostCenterID:       " cc-1 ",
		JournalEntryLineID: " line-1 ",
		StartDate:          ptrTime(allocationDate.AddDate(0, 0, -1)),
		EndDate:            ptrTime(allocationDate.AddDate(0, 0, 1)),
	})

	require.NoError(t, err)
	require.Len(t, allocations, 1)
	require.Equal(t, "match", allocations[0].ID)
}
