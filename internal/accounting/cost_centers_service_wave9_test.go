package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestCostCentersWave9RepositoryConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := costCentersWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
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

func costCentersWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
