package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetPeriodConstants(t *testing.T) {
	assert.Equal(t, BudgetPeriod("MONTHLY"), BudgetPeriodMonthly)
	assert.Equal(t, BudgetPeriod("QUARTERLY"), BudgetPeriodQuarterly)
	assert.Equal(t, BudgetPeriod("ANNUAL"), BudgetPeriodAnnual)
}

func TestMockCostCenterRepository_Create(t *testing.T) {
	repo := NewMockCostCenterRepository()
	ctx := context.Background()

	cc := &CostCenter{
		TenantID:     "tenant-1",
		Code:         "CC001",
		Name:         "Marketing",
		Description:  "Marketing department",
		IsActive:     true,
		BudgetPeriod: BudgetPeriodAnnual,
	}

	err := repo.Create(ctx, "test_schema", cc)
	require.NoError(t, err)
	assert.NotEmpty(t, cc.ID)

	// Verify it was stored
	stored, err := repo.GetByID(ctx, "test_schema", "tenant-1", cc.ID)
	require.NoError(t, err)
	assert.Equal(t, cc.Code, stored.Code)
	assert.Equal(t, cc.Name, stored.Name)
}

func TestMockCostCenterRepository_GetByID(t *testing.T) {
	repo := NewMockCostCenterRepository()
	ctx := context.Background()

	// Create a cost center
	cc := &CostCenter{
		ID:           "cc-123",
		TenantID:     "tenant-1",
		Code:         "CC001",
		Name:         "Sales",
		IsActive:     true,
		BudgetPeriod: BudgetPeriodMonthly,
	}
	repo.CostCenters[cc.ID] = cc

	// Test successful retrieval
	result, err := repo.GetByID(ctx, "test_schema", "tenant-1", "cc-123")
	require.NoError(t, err)
	assert.Equal(t, "CC001", result.Code)
	assert.Equal(t, "Sales", result.Name)

	// Test not found
	_, err = repo.GetByID(ctx, "test_schema", "tenant-1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test wrong tenant
	_, err = repo.GetByID(ctx, "test_schema", "wrong-tenant", "cc-123")
	assert.Error(t, err)
}

func TestMockCostCenterRepository_List(t *testing.T) {
	repo := NewMockCostCenterRepository()
	ctx := context.Background()

	// Add multiple cost centers
	repo.CostCenters["cc-1"] = &CostCenter{ID: "cc-1", TenantID: "tenant-1", Code: "CC001", Name: "Sales", IsActive: true}
	repo.CostCenters["cc-2"] = &CostCenter{ID: "cc-2", TenantID: "tenant-1", Code: "CC002", Name: "Marketing", IsActive: false}
	repo.CostCenters["cc-3"] = &CostCenter{ID: "cc-3", TenantID: "tenant-2", Code: "CC003", Name: "HR", IsActive: true}

	// List all for tenant-1
	results, err := repo.List(ctx, "test_schema", "tenant-1", false)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// List active only for tenant-1
	results, err = repo.List(ctx, "test_schema", "tenant-1", true)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Sales", results[0].Name)

	// List for tenant-2
	results, err = repo.List(ctx, "test_schema", "tenant-2", false)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMockCostCenterRepository_Update(t *testing.T) {
	repo := NewMockCostCenterRepository()
	ctx := context.Background()

	// Create initial cost center
	cc := &CostCenter{
		ID:       "cc-123",
		TenantID: "tenant-1",
		Code:     "CC001",
		Name:     "Original Name",
		IsActive: true,
	}
	repo.CostCenters[cc.ID] = cc

	// Update it
	cc.Name = "Updated Name"
	cc.IsActive = false
	err := repo.Update(ctx, "test_schema", cc)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.GetByID(ctx, "test_schema", "tenant-1", "cc-123")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.False(t, updated.IsActive)

	// Test update nonexistent
	notFound := &CostCenter{ID: "nonexistent", TenantID: "tenant-1"}
	err = repo.Update(ctx, "test_schema", notFound)
	assert.Error(t, err)
}

func TestMockCostCenterRepository_Delete(t *testing.T) {
	repo := NewMockCostCenterRepository()
	ctx := context.Background()

	// Create cost center
	repo.CostCenters["cc-123"] = &CostCenter{ID: "cc-123", TenantID: "tenant-1", Code: "CC001"}

	// Delete it
	err := repo.Delete(ctx, "test_schema", "tenant-1", "cc-123")
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.GetByID(ctx, "test_schema", "tenant-1", "cc-123")
	assert.Error(t, err)

	// Test delete nonexistent
	err = repo.Delete(ctx, "test_schema", "tenant-1", "nonexistent")
	assert.Error(t, err)

	// Test delete wrong tenant
	repo.CostCenters["cc-456"] = &CostCenter{ID: "cc-456", TenantID: "tenant-2", Code: "CC002"}
	err = repo.Delete(ctx, "test_schema", "tenant-1", "cc-456")
	assert.Error(t, err)
}

func TestMockCostCenterRepository_GetExpensesByPeriod(t *testing.T) {
	repo := NewMockCostCenterRepository()
	ctx := context.Background()

	// Add allocations
	now := time.Now()
	repo.Allocations["cc-123"] = []CostAllocation{
		{TenantID: "tenant-1", CostCenterID: "cc-123", Amount: decimal.NewFromInt(100), AllocationDate: now.AddDate(0, 0, -5)},
		{TenantID: "tenant-1", CostCenterID: "cc-123", Amount: decimal.NewFromInt(200), AllocationDate: now.AddDate(0, 0, -3)},
		{TenantID: "tenant-1", CostCenterID: "cc-123", Amount: decimal.NewFromInt(50), AllocationDate: now.AddDate(0, 0, -30)}, // Outside period
		{TenantID: "tenant-2", CostCenterID: "cc-123", Amount: decimal.NewFromInt(1000), AllocationDate: now},                  // Wrong tenant
	}

	// Get expenses for last week
	start := now.AddDate(0, 0, -7)
	end := now

	total, err := repo.GetExpensesByPeriod(ctx, "test_schema", "tenant-1", "cc-123", start, end)
	require.NoError(t, err)
	assert.Equal(t, decimal.NewFromInt(300), total)

	// Get expenses for empty cost center
	total, err = repo.GetExpensesByPeriod(ctx, "test_schema", "tenant-1", "cc-empty", start, end)
	require.NoError(t, err)
	assert.True(t, total.IsZero())
}

// Test CostCenterService with mock repository
type testCostCenterService struct {
	repo *MockCostCenterRepository
	svc  *CostCenterService
}

func newTestCostCenterService() *testCostCenterService {
	repo := NewMockCostCenterRepository()
	svc := &CostCenterService{
		db:   nil,
		repo: repo,
	}
	return &testCostCenterService{repo: repo, svc: svc}
}

func TestCostCenterService_CreateCostCenter(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Test successful creation
	budget := decimal.NewFromInt(10000)
	req := &CreateCostCenterRequest{
		Code:         "CC001",
		Name:         "Marketing",
		Description:  "Marketing department",
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: BudgetPeriodMonthly,
	}

	cc, err := ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", req)
	require.NoError(t, err)
	assert.NotEmpty(t, cc.ID)
	assert.Equal(t, "CC001", cc.Code)
	assert.Equal(t, "Marketing", cc.Name)
	assert.Equal(t, BudgetPeriodMonthly, cc.BudgetPeriod)

	// Test missing code
	_, err = ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", &CreateCostCenterRequest{Name: "Test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "code is required")

	// Test missing name
	_, err = ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", &CreateCostCenterRequest{Code: "CC"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")

	// Test default budget period
	req2 := &CreateCostCenterRequest{Code: "CC002", Name: "Sales", IsActive: true}
	cc2, err := ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", req2)
	require.NoError(t, err)
	assert.Equal(t, BudgetPeriodAnnual, cc2.BudgetPeriod)
}

func TestCostCenterService_GetCostCenter(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Add a cost center
	ts.repo.CostCenters["cc-123"] = &CostCenter{
		ID:       "cc-123",
		TenantID: "tenant-1",
		Code:     "CC001",
		Name:     "Sales",
	}

	// Test successful retrieval
	cc, err := ts.svc.GetCostCenter(ctx, "test_schema", "tenant-1", "cc-123")
	require.NoError(t, err)
	assert.Equal(t, "Sales", cc.Name)

	// Test not found
	_, err = ts.svc.GetCostCenter(ctx, "test_schema", "tenant-1", "nonexistent")
	assert.Error(t, err)
}

func TestCostCenterService_ListCostCenters(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Add cost centers
	ts.repo.CostCenters["cc-1"] = &CostCenter{ID: "cc-1", TenantID: "tenant-1", Code: "CC001", Name: "Sales", IsActive: true}
	ts.repo.CostCenters["cc-2"] = &CostCenter{ID: "cc-2", TenantID: "tenant-1", Code: "CC002", Name: "Marketing", IsActive: false}

	// List all
	results, err := ts.svc.ListCostCenters(ctx, "test_schema", "tenant-1", false)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// List active only
	results, err = ts.svc.ListCostCenters(ctx, "test_schema", "tenant-1", true)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestCostCenterService_UpdateCostCenter(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Add a cost center
	ts.repo.CostCenters["cc-123"] = &CostCenter{
		ID:       "cc-123",
		TenantID: "tenant-1",
		Code:     "CC001",
		Name:     "Original",
		IsActive: true,
	}

	// Update it
	req := &UpdateCostCenterRequest{
		Code:     "CC001-NEW",
		Name:     "Updated Name",
		IsActive: false,
	}

	cc, err := ts.svc.UpdateCostCenter(ctx, "test_schema", "tenant-1", "cc-123", req)
	require.NoError(t, err)
	assert.Equal(t, "CC001-NEW", cc.Code)
	assert.Equal(t, "Updated Name", cc.Name)
	assert.False(t, cc.IsActive)

	// Test update nonexistent
	_, err = ts.svc.UpdateCostCenter(ctx, "test_schema", "tenant-1", "nonexistent", req)
	assert.Error(t, err)
}

func TestCostCenterService_DeleteCostCenter(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Add a cost center
	ts.repo.CostCenters["cc-123"] = &CostCenter{ID: "cc-123", TenantID: "tenant-1", Code: "CC001"}

	// Delete it
	err := ts.svc.DeleteCostCenter(ctx, "test_schema", "tenant-1", "cc-123")
	require.NoError(t, err)

	// Verify deleted
	_, err = ts.svc.GetCostCenter(ctx, "test_schema", "tenant-1", "cc-123")
	assert.Error(t, err)
}

func TestCostCenterService_GetCostCenterReport(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	budget1 := decimal.NewFromInt(1000)
	budget2 := decimal.NewFromInt(500)

	// Add cost centers
	ts.repo.CostCenters["cc-1"] = &CostCenter{
		ID:           "cc-1",
		TenantID:     "tenant-1",
		Code:         "CC001",
		Name:         "Sales",
		IsActive:     true,
		BudgetAmount: &budget1,
		BudgetPeriod: BudgetPeriodMonthly,
	}
	ts.repo.CostCenters["cc-2"] = &CostCenter{
		ID:           "cc-2",
		TenantID:     "tenant-1",
		Code:         "CC002",
		Name:         "Marketing",
		IsActive:     true,
		BudgetAmount: &budget2,
		BudgetPeriod: BudgetPeriodMonthly,
	}

	// Add allocations
	now := time.Now()
	ts.repo.Allocations["cc-1"] = []CostAllocation{
		{TenantID: "tenant-1", Amount: decimal.NewFromInt(800), AllocationDate: now},
	}
	ts.repo.Allocations["cc-2"] = []CostAllocation{
		{TenantID: "tenant-1", Amount: decimal.NewFromInt(600), AllocationDate: now}, // Over budget
	}

	// Generate report
	start := now.AddDate(0, 0, -7)
	end := now.AddDate(0, 0, 1)

	report, err := ts.svc.GetCostCenterReport(ctx, "test_schema", "tenant-1", start, end)
	require.NoError(t, err)

	assert.Equal(t, "tenant-1", report.TenantID)
	assert.Len(t, report.CostCenters, 2)
	assert.Equal(t, decimal.NewFromInt(1400), report.TotalExpenses)
	assert.Equal(t, decimal.NewFromInt(1500), report.TotalBudget)

	// Check individual summaries
	for _, summary := range report.CostCenters {
		if summary.CostCenter.Code == "CC001" {
			assert.False(t, summary.IsOverBudget)
			assert.Equal(t, decimal.NewFromInt(800), summary.TotalExpenses)
		} else if summary.CostCenter.Code == "CC002" {
			assert.True(t, summary.IsOverBudget)
			assert.Equal(t, decimal.NewFromInt(600), summary.TotalExpenses)
		}
	}
}

func TestCostCenterService_GetCostCenterReport_NoBudget(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Cost center without budget
	ts.repo.CostCenters["cc-1"] = &CostCenter{
		ID:           "cc-1",
		TenantID:     "tenant-1",
		Code:         "CC001",
		Name:         "General",
		IsActive:     true,
		BudgetAmount: nil, // No budget set
	}

	now := time.Now()
	ts.repo.Allocations["cc-1"] = []CostAllocation{
		{TenantID: "tenant-1", Amount: decimal.NewFromInt(500), AllocationDate: now},
	}

	report, err := ts.svc.GetCostCenterReport(ctx, "test_schema", "tenant-1", now.AddDate(0, 0, -7), now.AddDate(0, 0, 1))
	require.NoError(t, err)

	assert.Len(t, report.CostCenters, 1)
	summary := report.CostCenters[0]
	assert.False(t, summary.IsOverBudget) // No budget = never over budget
	assert.True(t, summary.BudgetUsed.IsZero())
}

func TestCostCenterService_CreateCostCenter_ValidationErrors(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *CreateCostCenterRequest
		wantErr string
	}{
		{
			name:    "empty code",
			req:     &CreateCostCenterRequest{Code: "", Name: "Test"},
			wantErr: "cost center code is required",
		},
		{
			name:    "empty name",
			req:     &CreateCostCenterRequest{Code: "CC001", Name: ""},
			wantErr: "cost center name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCostCenterService_CreateCostCenter_DefaultBudgetPeriod(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	req := &CreateCostCenterRequest{
		Code:         "CC001",
		Name:         "Test Center",
		IsActive:     true,
		BudgetPeriod: "", // Empty - should default to ANNUAL
	}

	cc, err := ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", req)
	require.NoError(t, err)
	assert.Equal(t, BudgetPeriodAnnual, cc.BudgetPeriod)
}

func TestCostCenterService_GetCostCenterReport_EmptyList(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// No cost centers added
	now := time.Now()
	report, err := ts.svc.GetCostCenterReport(ctx, "test_schema", "tenant-1", now.AddDate(0, 0, -7), now.AddDate(0, 0, 1))
	require.NoError(t, err)

	assert.Empty(t, report.CostCenters)
	assert.True(t, report.TotalExpenses.IsZero())
	assert.True(t, report.TotalBudget.IsZero())
}
