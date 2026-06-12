package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	costAllocationJournalLineID1 = "11111111-1111-4111-8111-111111111111"
	costAllocationJournalLineID2 = "22222222-2222-4222-8222-222222222222"
	costAllocationJournalLineID3 = "33333333-3333-4333-8333-333333333333"
	costAllocationJournalLineID4 = "44444444-4444-4444-8444-444444444444"
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
		repo: repo,
	}
	return &testCostCenterService{repo: repo, svc: svc}
}

type costCenterImportListErrorRepository struct {
	CostCenterRepository
}

func (costCenterImportListErrorRepository) List(_ context.Context, _, _ string, _ bool) ([]CostCenter, error) {
	return nil, errors.New("list unavailable")
}

type costCenterImportCreateErrorRepository struct {
	CostCenterRepository
}

func (costCenterImportCreateErrorRepository) List(_ context.Context, _, _ string, _ bool) ([]CostCenter, error) {
	return nil, nil
}

func (costCenterImportCreateErrorRepository) Create(_ context.Context, _ string, _ *CostCenter) error {
	return errors.New("create unavailable")
}

func TestCostCenterService_CreateCostCenter(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	// Test successful creation
	budget := decimal.NewFromInt(10000)
	parentID := "11111111-1111-4111-8111-111111111111"
	parentIDWithSpaces := " " + parentID + " "
	req := &CreateCostCenterRequest{
		Code:         "CC001",
		Name:         "Marketing",
		Description:  "Marketing department",
		ParentID:     &parentIDWithSpaces,
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: BudgetPeriodMonthly,
	}

	cc, err := ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", req)
	require.NoError(t, err)
	assert.NotEmpty(t, cc.ID)
	assert.Equal(t, "CC001", cc.Code)
	assert.Equal(t, "Marketing", cc.Name)
	require.NotNil(t, cc.ParentID)
	assert.Equal(t, parentID, *cc.ParentID)
	assert.Equal(t, BudgetPeriodMonthly, cc.BudgetPeriod)

	// Test missing code
	_, err = ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", &CreateCostCenterRequest{Name: "Test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "code is required")

	// Test missing name
	_, err = ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", &CreateCostCenterRequest{Code: "CC"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")

	// Test invalid parent id
	badParentID := "legacy-parent"
	_, err = ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", &CreateCostCenterRequest{
		Code:     "CCBAD",
		Name:     "Invalid Parent",
		ParentID: &badParentID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent_id must be a valid UUID")

	// Test default budget period
	req2 := &CreateCostCenterRequest{Code: "CC002", Name: "Sales", IsActive: true}
	cc2, err := ts.svc.CreateCostCenter(ctx, "test_schema", "tenant-1", req2)
	require.NoError(t, err)
	assert.Equal(t, BudgetPeriodAnnual, cc2.BudgetPeriod)
}

func TestCostCenterService_ImportCostCentersCSV(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	ts.repo.CostCenters["cc-existing"] = &CostCenter{
		ID:       "cc-existing",
		TenantID: "tenant-1",
		Code:     "CC999",
		Name:     "Existing",
		IsActive: true,
	}
	ts.repo.CostCenters["cc-empty-code"] = &CostCenter{
		ID:       "cc-empty-code",
		TenantID: "tenant-1",
		Code:     " \t ",
		Name:     "Incomplete existing row",
		IsActive: true,
	}

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		FileName: "cost-centers.csv",
		CSVContent: "code,name,description,parent_code,budget_amount,budget_period,status\n" +
			"CC-CHILD,Child,Child team,CC-PARENT,500.00,MONTHLY,ACTIVE\n" +
			"CC-PARENT,Parent,Parent team,,1000.00,ANNUAL,ACTIVE\n" +
			"CC999,Duplicate,Duplicate team,,100.00,MONTHLY,ACTIVE\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "cost-centers.csv", result.FileName)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 2, result.CostCentersCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 4, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "duplicate code")

	var parent *CostCenter
	var child *CostCenter
	for _, costCenter := range ts.repo.CostCenters {
		switch costCenter.Code {
		case "CC-PARENT":
			parent = costCenter
		case "CC-CHILD":
			child = costCenter
		}
	}

	require.NotNil(t, parent)
	assert.Equal(t, "Parent", parent.Name)
	require.NotNil(t, parent.BudgetAmount)
	assert.True(t, parent.BudgetAmount.Equal(decimal.RequireFromString("1000.00")))
	assert.Equal(t, BudgetPeriodAnnual, parent.BudgetPeriod)

	require.NotNil(t, child)
	assert.Equal(t, "Child", child.Name)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parent.ID, *child.ParentID)
	require.NotNil(t, child.BudgetAmount)
	assert.True(t, child.BudgetAmount.Equal(decimal.RequireFromString("500.00")))
	assert.Equal(t, BudgetPeriodMonthly, child.BudgetPeriod)
	assert.True(t, child.IsActive)
}

func TestCostCenterService_ImportCostCentersCSVRejectsInvalidParentID(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "code,name,parent_id\nCC-CHILD,Child,legacy-parent\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.CostCentersCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "CC-CHILD", result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "parent_id must be a valid UUID")
}

func TestCostCenterService_ImportCostCentersCSVParsesActiveAliases(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "code,name,is_active\n" +
			"CC-ACTIVE,Active,yes\n" +
			"CC-INACTIVE,Inactive,0\n" +
			"CC-BAD,Bad,maybe\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 2, result.CostCentersCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 4, result.Errors[0].Row)
	assert.Equal(t, "CC-BAD", result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "is_active must be true or false")

	var active *CostCenter
	var inactive *CostCenter
	for _, costCenter := range ts.repo.CostCenters {
		switch costCenter.Code {
		case "CC-ACTIVE":
			active = costCenter
		case "CC-INACTIVE":
			inactive = costCenter
		}
	}

	require.NotNil(t, active)
	assert.True(t, active.IsActive)
	require.NotNil(t, inactive)
	assert.False(t, inactive.IsActive)
}

func TestCostCenterService_ImportCostCentersCSVImportsWithoutErrors(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		FileName: "clean-cost-centers.csv",
		CSVContent: "code,name,description,budget_amount,budget_period,status\n" +
			"CC-MKT,Marketing,Marketing team,1000.00,MONTHLY,ACTIVE\n" +
			"CC-SALES,Sales,Sales team,,ANNUAL,INACTIVE\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "clean-cost-centers.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 2, result.CostCentersCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)
}

func TestCostCenterService_ImportCostCentersCSVRejectsMissingContent(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "csv_content is required")
}

func TestCostCenterService_ImportCostCentersCSVReturnsParserErrors(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "\"code,name\n",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "parse csv header")
}

func TestCostCenterService_ImportCostCentersCSVRejectsHeaderOnlyCSV(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "code,name\n",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no cost centers found in CSV")
}

func TestCostCenterService_ImportCostCentersCSVWrapsListErrors(t *testing.T) {
	svc := NewCostCenterServiceWithRepository(costCenterImportListErrorRepository{})

	result, err := svc.ImportCostCentersCSV(context.Background(), "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "code,name\nCC-OPS,Operations\n",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list existing cost centers: list unavailable")
}

func TestCostCenterService_ImportCostCentersCSVRecordsCreateErrors(t *testing.T) {
	svc := NewCostCenterServiceWithRepository(costCenterImportCreateErrorRepository{})

	result, err := svc.ImportCostCentersCSV(context.Background(), "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "code,name\nCC-OPS,Operations\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.CostCentersCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "CC-OPS", result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "create unavailable")
}

func TestCostCenterService_ImportCostCentersCSVRejectsUnresolvedParentCode(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostCentersCSV(ctx, "test_schema", "tenant-1", &ImportCostCentersRequest{
		CSVContent: "code,name,parent_code\nCC-CHILD,Child,CC-MISSING\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.CostCentersCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "CC-CHILD", result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "parent_code \"CC-MISSING\" was not found")
}

func TestParseCostCenterImportRows(t *testing.T) {
	t.Run("parses aliased semicolon headers and skips blank rows", func(t *testing.T) {
		rows, err := parseCostCenterImportRows("\ufeff Cost Center Code ; CC Name ; Description ; Parent ; Budget ; Budget Period ; Active ; custom field\n" +
			" CC-MKT ; Marketing ; Marketing team ; CC-ROOT ; 1,250.00 ; MONTHLY ; yes ; ignored\n" +
			" ; ; ; ; ; ; ; \n" +
			"CC-SALES ; Sales ; ; ; ; ; no ; ignored\n")

		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, 2, rows[0].rowNumber)
		assert.Equal(t, "CC-MKT", rows[0].values["code"])
		assert.Equal(t, "Marketing", rows[0].values["name"])
		assert.Equal(t, "Marketing team", rows[0].values["description"])
		assert.Equal(t, "CC-ROOT", rows[0].values["parent_code"])
		assert.Equal(t, "1,250.00", rows[0].values["budget_amount"])
		assert.Equal(t, "MONTHLY", rows[0].values["budget_period"])
		assert.Equal(t, "yes", rows[0].values["is_active"])
		assert.Equal(t, "ignored", rows[0].values["custom_field"])

		assert.Equal(t, 4, rows[1].rowNumber)
		assert.Equal(t, "CC-SALES", rows[1].values["code"])
		assert.Equal(t, "Sales", rows[1].values["name"])
		assert.Empty(t, rows[1].values["description"])
		assert.Empty(t, rows[1].values["parent_code"])
		assert.Empty(t, rows[1].values["budget_amount"])
		assert.Empty(t, rows[1].values["budget_period"])
		assert.Equal(t, "no", rows[1].values["is_active"])
	})

	t.Run("allows header-only csv", func(t *testing.T) {
		rows, err := parseCostCenterImportRows("code,name\n")

		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("ignores blank header columns", func(t *testing.T) {
		rows, err := parseCostCenterImportRows("code,name,\nCC-OPS,Operations,ignored\n")

		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "CC-OPS", rows[0].values["code"])
		assert.Equal(t, "Operations", rows[0].values["name"])
		_, ok := rows[0].values[""]
		assert.False(t, ok)
	})

	t.Run("requires content", func(t *testing.T) {
		rows, err := parseCostCenterImportRows(" \t\n ")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "csv_content is required")
	})

	t.Run("requires code and name columns", func(t *testing.T) {
		rows, err := parseCostCenterImportRows("code,description\nCC-OPS,Operations\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "missing required columns: code, name")
	})

	t.Run("reports malformed csv headers", func(t *testing.T) {
		rows, err := parseCostCenterImportRows("\"code,name\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("reports malformed csv rows", func(t *testing.T) {
		rows, err := parseCostCenterImportRows("code,name\nCC-OPS,\"Operations\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv row 2")
	})
}

func TestBuildCreateCostCenterRequestFromImportRow(t *testing.T) {
	t.Run("normalizes request fields and returns parent code", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{
				"code":          " CC-MKT ",
				"name":          " Marketing ",
				"description":   "  Marketing team  ",
				"parent_code":   " CC-ROOT ",
				"budget_amount": " 123.45 ",
				"budget_period": " quarterly ",
				"status":        " inactive ",
			},
		})

		require.NoError(t, err)
		assert.Equal(t, "CC-MKT", req.Code)
		assert.Equal(t, "Marketing", req.Name)
		assert.Equal(t, "Marketing team", req.Description)
		assert.Nil(t, req.ParentID)
		assert.Equal(t, "CC-ROOT", parentCode)
		require.NotNil(t, req.BudgetAmount)
		assert.True(t, decimal.RequireFromString("123.45").Equal(*req.BudgetAmount))
		assert.Equal(t, BudgetPeriodQuarterly, req.BudgetPeriod)
		assert.False(t, req.IsActive)
	})

	t.Run("parses parent id and defaults optional fields", func(t *testing.T) {
		parentID := "11111111-1111-4111-8111-111111111111"
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{
				"code":      "CC-OPS",
				"name":      "Operations",
				"parent_id": " " + parentID + " ",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, req.ParentID)
		assert.Equal(t, parentID, *req.ParentID)
		assert.Empty(t, parentCode)
		assert.Nil(t, req.BudgetAmount)
		assert.Equal(t, BudgetPeriodAnnual, req.BudgetPeriod)
		assert.True(t, req.IsActive)
	})

	t.Run("requires code", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"name": "Operations"},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "code is required")
	})

	t.Run("requires name", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"code": "CC-OPS"},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("rejects self parent code", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"code": "CC-OPS", "name": "Operations", "parent_code": " cc-ops "},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "parent_code cannot match code")
	})

	t.Run("rejects invalid budget amount", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"code": "CC-OPS", "name": "Operations", "budget_amount": "bad"},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "budget_amount must be a decimal")
	})

	t.Run("rejects invalid budget period", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"code": "CC-OPS", "name": "Operations", "budget_period": "WEEKLY"},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "invalid budget_period")
	})

	t.Run("rejects invalid status", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"code": "CC-OPS", "name": "Operations", "status": "paused"},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("rejects invalid parent id", func(t *testing.T) {
		req, parentCode, err := buildCreateCostCenterRequestFromImportRow(costCenterImportRow{
			values: map[string]string{"code": "CC-OPS", "name": "Operations", "parent_id": "legacy-parent"},
		})

		require.Error(t, err)
		assert.Nil(t, req)
		assert.Empty(t, parentCode)
		assert.Contains(t, err.Error(), "parent_id must be a valid UUID")
	})
}

func TestCostCenterImportHelpers(t *testing.T) {
	t.Run("parses optional parent id", func(t *testing.T) {
		parentID, err := optionalCostCenterImportParentID(" 11111111-1111-4111-8111-111111111111 ")

		require.NoError(t, err)
		require.NotNil(t, parentID)
		assert.Equal(t, "11111111-1111-4111-8111-111111111111", *parentID)
	})

	t.Run("allows blank parent id", func(t *testing.T) {
		parentID, err := optionalCostCenterImportParentID(" \t ")

		require.NoError(t, err)
		assert.Nil(t, parentID)
	})

	t.Run("rejects invalid parent id", func(t *testing.T) {
		parentID, err := optionalCostCenterImportParentID("legacy-parent")

		require.Error(t, err)
		assert.Nil(t, parentID)
		assert.Contains(t, err.Error(), "parent_id must be a valid UUID")
	})

	t.Run("allows blank budget amount", func(t *testing.T) {
		amount, err := parseCostCenterImportBudgetAmount(" \t ")

		require.NoError(t, err)
		assert.Nil(t, amount)
	})

	t.Run("parses budget amount", func(t *testing.T) {
		amount, err := parseCostCenterImportBudgetAmount(" 1000.50 ")

		require.NoError(t, err)
		require.NotNil(t, amount)
		assert.True(t, decimal.RequireFromString("1000.50").Equal(*amount))
	})

	t.Run("rejects invalid budget amount", func(t *testing.T) {
		amount, err := parseCostCenterImportBudgetAmount("bad")

		require.Error(t, err)
		assert.Nil(t, amount)
		assert.Contains(t, err.Error(), "budget_amount must be a decimal")
	})

	t.Run("rejects negative budget amount", func(t *testing.T) {
		amount, err := parseCostCenterImportBudgetAmount("-1")

		require.Error(t, err)
		assert.Nil(t, amount)
		assert.Contains(t, err.Error(), "budget_amount cannot be negative")
	})

	t.Run("defaults blank budget period to annual", func(t *testing.T) {
		period, err := parseCostCenterImportBudgetPeriod(" \t ")

		require.NoError(t, err)
		assert.Equal(t, BudgetPeriodAnnual, period)
	})

	t.Run("parses supported budget periods", func(t *testing.T) {
		for _, tt := range []struct {
			value string
			want  BudgetPeriod
		}{
			{value: "monthly", want: BudgetPeriodMonthly},
			{value: "QUARTERLY", want: BudgetPeriodQuarterly},
			{value: " Annual ", want: BudgetPeriodAnnual},
		} {
			period, err := parseCostCenterImportBudgetPeriod(tt.value)

			require.NoError(t, err)
			assert.Equal(t, tt.want, period)
		}
	})

	t.Run("rejects invalid budget period", func(t *testing.T) {
		period, err := parseCostCenterImportBudgetPeriod("WEEKLY")

		require.Error(t, err)
		assert.Empty(t, period)
		assert.Contains(t, err.Error(), "invalid budget_period")
	})

	t.Run("parses status before active flag", func(t *testing.T) {
		active, err := parseCostCenterImportActive(" INACTIVE ", "yes")

		require.NoError(t, err)
		assert.False(t, active)
	})

	t.Run("parses active status", func(t *testing.T) {
		active, err := parseCostCenterImportActive("active", "")

		require.NoError(t, err)
		assert.True(t, active)
	})

	t.Run("rejects invalid status", func(t *testing.T) {
		active, err := parseCostCenterImportActive("paused", "")

		require.Error(t, err)
		assert.False(t, active)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("defaults blank active values to true", func(t *testing.T) {
		active, err := parseCostCenterImportActive("", " \t ")

		require.NoError(t, err)
		assert.True(t, active)
	})

	t.Run("parses boolean aliases", func(t *testing.T) {
		active, err := parseCostCenterImportBool("is_active", "t")
		require.NoError(t, err)
		assert.True(t, active)

		active, err = parseCostCenterImportBool("is_active", "N")
		require.NoError(t, err)
		assert.False(t, active)
	})

	t.Run("rejects invalid boolean aliases", func(t *testing.T) {
		active, err := parseCostCenterImportBool("is_active", "maybe")

		require.Error(t, err)
		assert.False(t, active)
		assert.Contains(t, err.Error(), "is_active must be true or false")
	})

	t.Run("canonicalizes cost center header aliases", func(t *testing.T) {
		assert.Equal(t, "code", canonicalCostCenterImportHeader("Cost Center Code"))
		assert.Equal(t, "name", canonicalCostCenterImportHeader("cc_name"))
		assert.Equal(t, "parent_code", canonicalCostCenterImportHeader("Parent Cost Center Code"))
		assert.Equal(t, "budget_amount", canonicalCostCenterImportHeader("Budget"))
		assert.Equal(t, "is_active", canonicalCostCenterImportHeader("active"))
		assert.Equal(t, "status", canonicalCostCenterImportHeader("Status"))
		assert.Equal(t, "custom_field", canonicalCostCenterImportHeader("Custom Field"))
	})
}

func TestCostCenterService_ImportCostAllocationsCSV(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()
	salesCostCenterID := "55555555-5555-4555-8555-555555555555"
	adminCostCenterID := "66666666-6666-4666-8666-666666666666"

	ts.repo.CostCenters[salesCostCenterID] = &CostCenter{
		ID:       salesCostCenterID,
		TenantID: "tenant-1",
		Code:     "SALES",
		Name:     "Sales",
		IsActive: true,
	}
	ts.repo.CostCenters[adminCostCenterID] = &CostCenter{
		ID:       adminCostCenterID,
		TenantID: "tenant-1",
		Code:     "ADMIN",
		Name:     "Administration",
		IsActive: true,
	}

	result, err := ts.svc.ImportCostAllocationsCSV(ctx, "test_schema", "tenant-1", &ImportCostAllocationsRequest{
		FileName: "cost-allocations.csv",
		CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_percentage,allocation_date,notes\n" +
			"SALES," + costAllocationJournalLineID1 + ",125.50,50,2026-03-20,Shared rent\n" +
			"ADMIN," + costAllocationJournalLineID2 + ",300.00,,2026-03-21,Admin payroll\n" +
			"UNKNOWN," + costAllocationJournalLineID3 + ",10.00,10,2026-03-22,Missing center\n" +
			"SALES," + costAllocationJournalLineID4 + ",0,10,2026-03-23,Bad amount\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "cost-allocations.csv", result.FileName)
	assert.Equal(t, 4, result.RowsProcessed)
	assert.Equal(t, 2, result.AllocationsImported)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Equal(t, 4, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, `cost_center_code "UNKNOWN" was not found`)
	assert.Equal(t, 5, result.Errors[1].Row)
	assert.Contains(t, result.Errors[1].Message, "amount must be greater than zero")

	allocations, err := ts.svc.ListCostAllocations(ctx, "test_schema", "tenant-1", CostAllocationFilters{})
	require.NoError(t, err)
	require.Len(t, allocations, 2)

	byLine := map[string]CostAllocation{}
	for _, allocation := range allocations {
		byLine[allocation.JournalEntryLineID] = allocation
	}

	salesAllocation := byLine[costAllocationJournalLineID1]
	assert.Equal(t, salesCostCenterID, salesAllocation.CostCenterID)
	assert.True(t, salesAllocation.Amount.Equal(decimal.RequireFromString("125.50")))
	require.NotNil(t, salesAllocation.AllocationPercentage)
	assert.True(t, salesAllocation.AllocationPercentage.Equal(decimal.RequireFromString("50")))
	assert.Equal(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), salesAllocation.AllocationDate)
	assert.Equal(t, "Shared rent", salesAllocation.Notes)

	adminAllocation := byLine[costAllocationJournalLineID2]
	assert.Equal(t, adminCostCenterID, adminAllocation.CostCenterID)
	assert.True(t, adminAllocation.Amount.Equal(decimal.RequireFromString("300.00")))
	assert.Nil(t, adminAllocation.AllocationPercentage)
}

func TestCostCenterService_ImportCostAllocationsCSVByID(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()
	costCenterID := "11111111-1111-1111-1111-111111111111"

	ts.repo.CostCenters[costCenterID] = &CostCenter{
		ID:       costCenterID,
		TenantID: "tenant-1",
		Code:     "DIRECT",
		Name:     "Direct costs",
		IsActive: true,
	}

	result, err := ts.svc.ImportCostAllocationsCSV(ctx, "test_schema", "tenant-1", &ImportCostAllocationsRequest{
		CSVContent: "cost_center_id,journal_entry_line_id,amount,allocation_date\n" +
			costCenterID + "," + costAllocationJournalLineID1 + ",42.00,2026-04-01\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.AllocationsImported)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)
	assert.Len(t, ts.repo.Allocations[costCenterID], 1)
	assert.Equal(t, costAllocationJournalLineID1, ts.repo.Allocations[costCenterID][0].JournalEntryLineID)
}

func TestCostCenterService_ImportCostAllocationsCSVRejectsInvalidCostCenterID(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	result, err := ts.svc.ImportCostAllocationsCSV(ctx, "test_schema", "tenant-1", &ImportCostAllocationsRequest{
		CSVContent: "cost_center_id,journal_entry_line_id,amount,allocation_date\n" +
			"legacy-cc," + costAllocationJournalLineID1 + ",42.00,2026-04-01\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.AllocationsImported)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "legacy-cc", result.Errors[0].CostCenterID)
	assert.Contains(t, result.Errors[0].Message, "cost_center_id must be a valid UUID")
}

func TestCostCenterService_ImportCostAllocationsCSVRejectsInvalidJournalEntryLineID(t *testing.T) {
	ts := newTestCostCenterService()
	ctx := context.Background()

	ts.repo.CostCenters["cc-sales"] = &CostCenter{
		ID:       "cc-sales",
		TenantID: "tenant-1",
		Code:     "SALES",
		Name:     "Sales",
		IsActive: true,
	}

	result, err := ts.svc.ImportCostAllocationsCSV(ctx, "test_schema", "tenant-1", &ImportCostAllocationsRequest{
		CSVContent: "cost_center_code,journal_entry_line_id,amount,allocation_date\n" +
			"SALES,legacy-line,42.00,2026-04-01\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.AllocationsImported)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "legacy-line", result.Errors[0].JournalEntryLineID)
	assert.Contains(t, result.Errors[0].Message, "journal_entry_line_id must be a valid UUID")
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
	parentID := "22222222-2222-4222-8222-222222222222"
	req := &UpdateCostCenterRequest{
		Code:     "CC001-NEW",
		Name:     "Updated Name",
		ParentID: &parentID,
		IsActive: false,
	}

	cc, err := ts.svc.UpdateCostCenter(ctx, "test_schema", "tenant-1", "cc-123", req)
	require.NoError(t, err)
	assert.Equal(t, "CC001-NEW", cc.Code)
	assert.Equal(t, "Updated Name", cc.Name)
	require.NotNil(t, cc.ParentID)
	assert.Equal(t, parentID, *cc.ParentID)
	assert.False(t, cc.IsActive)

	// Test invalid parent id
	badParentID := "legacy-parent"
	_, err = ts.svc.UpdateCostCenter(ctx, "test_schema", "tenant-1", "cc-123", &UpdateCostCenterRequest{
		Code:     "CC001-NEW",
		Name:     "Updated Name",
		ParentID: &badParentID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent_id must be a valid UUID")

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
