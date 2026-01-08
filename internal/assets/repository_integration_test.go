//go:build integration

package assets

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func strPtr(s string) *string {
	return &s
}

func createTestCategory(t *testing.T, repo *PostgresRepository, schemaName, tenantID string) *AssetCategory {
	t.Helper()
	cat := &AssetCategory{
		ID:                           uuid.New().String(),
		TenantID:                     tenantID,
		Name:                         "Test Category " + uuid.New().String()[:8],
		Description:                  "Test category description",
		DepreciationMethod:           DepreciationStraightLine,
		DefaultUsefulLifeMonths:      60,
		DefaultResidualValuePercent:  decimal.NewFromFloat(10.0),
		AssetAccountID:               strPtr("asset-account-1"),
		DepreciationExpenseAccountID: strPtr("dep-exp-account-1"),
		AccumulatedDepreciationAcctID: strPtr("acc-dep-account-1"),
		CreatedAt:                    time.Now(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.CreateCategory(context.Background(), schemaName, cat); err != nil {
		t.Fatalf("createTestCategory failed: %v", err)
	}
	return cat
}

func TestRepository_CreateAndGetCategory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := &AssetCategory{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		Name:                         "Vehicles",
		Description:                  "Company vehicles",
		DepreciationMethod:           DepreciationStraightLine,
		DefaultUsefulLifeMonths:      60,
		DefaultResidualValuePercent:  decimal.NewFromFloat(10.0),
		AssetAccountID:               strPtr("asset-acct-1"),
		DepreciationExpenseAccountID: strPtr("dep-exp-acct-1"),
		AccumulatedDepreciationAcctID: strPtr("acc-dep-acct-1"),
		CreatedAt:                    time.Now(),
		UpdatedAt:                    time.Now(),
	}

	err := repo.CreateCategory(ctx, tenant.SchemaName, cat)
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	retrieved, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, cat.ID)
	if err != nil {
		t.Fatalf("GetCategoryByID failed: %v", err)
	}

	if retrieved.Name != cat.Name {
		t.Errorf("expected name %s, got %s", cat.Name, retrieved.Name)
	}
	if retrieved.DepreciationMethod != cat.DepreciationMethod {
		t.Errorf("expected depreciation method %s, got %s", cat.DepreciationMethod, retrieved.DepreciationMethod)
	}
}

func TestRepository_GetCategoryByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, "nonexistent")
	if err != ErrCategoryNotFound {
		t.Errorf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestRepository_ListCategories(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create multiple categories
	for i := 0; i < 3; i++ {
		cat := &AssetCategory{
			ID:                           uuid.New().String(),
			TenantID:                     tenant.ID,
			Name:                         "Category " + uuid.New().String()[:8],
			DepreciationMethod:           DepreciationStraightLine,
			DefaultUsefulLifeMonths:      60,
			DefaultResidualValuePercent:  decimal.NewFromFloat(10.0),
			AssetAccountID:               strPtr("asset-acct"),
			DepreciationExpenseAccountID: strPtr("dep-exp-acct"),
			AccumulatedDepreciationAcctID: strPtr("acc-dep-acct"),
			CreatedAt:                    time.Now(),
			UpdatedAt:                    time.Now(),
		}
		if err := repo.CreateCategory(ctx, tenant.SchemaName, cat); err != nil {
			t.Fatalf("CreateCategory failed: %v", err)
		}
	}

	categories, err := repo.ListCategories(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListCategories failed: %v", err)
	}

	if len(categories) != 3 {
		t.Errorf("expected 3 categories, got %d", len(categories))
	}
}

func TestRepository_UpdateCategory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	// Update
	cat.Name = "Updated Category Name"
	cat.Description = "Updated description"
	cat.DefaultUsefulLifeMonths = 120

	if err := repo.UpdateCategory(ctx, tenant.SchemaName, cat); err != nil {
		t.Fatalf("UpdateCategory failed: %v", err)
	}

	retrieved, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, cat.ID)
	if err != nil {
		t.Fatalf("GetCategoryByID failed: %v", err)
	}

	if retrieved.Name != "Updated Category Name" {
		t.Errorf("expected name 'Updated Category Name', got '%s'", retrieved.Name)
	}
	if retrieved.DefaultUsefulLifeMonths != 120 {
		t.Errorf("expected 120 months, got %d", retrieved.DefaultUsefulLifeMonths)
	}
}

func TestRepository_UpdateCategory_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := &AssetCategory{
		ID:                           "nonexistent",
		TenantID:                     tenant.ID,
		Name:                         "Test",
		DepreciationMethod:           DepreciationStraightLine,
		DefaultUsefulLifeMonths:      60,
		DefaultResidualValuePercent:  decimal.Zero,
		AssetAccountID:               strPtr("acct"),
		DepreciationExpenseAccountID: strPtr("acct"),
		AccumulatedDepreciationAcctID: strPtr("acct"),
	}

	err := repo.UpdateCategory(ctx, tenant.SchemaName, cat)
	if err != ErrCategoryNotFound {
		t.Errorf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestRepository_DeleteCategory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	if err := repo.DeleteCategory(ctx, tenant.SchemaName, tenant.ID, cat.ID); err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}

	_, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, cat.ID)
	if err != ErrCategoryNotFound {
		t.Errorf("expected ErrCategoryNotFound after deletion, got %v", err)
	}
}

func TestRepository_DeleteCategory_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	err := repo.DeleteCategory(ctx, tenant.SchemaName, tenant.ID, "nonexistent")
	if err != ErrCategoryNotFound {
		t.Errorf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestRepository_CreateAndGetAsset(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	purchaseDate := time.Now().AddDate(0, -6, 0)
	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-00001",
		Name:                         "Company Car",
		Description:                  "Tesla Model 3",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusActive,
		PurchaseDate:                 purchaseDate,
		PurchaseCost:                 decimal.NewFromFloat(45000.00),
		SerialNumber:                 "VIN12345678901234",
		Location:                     "Head Office",
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.NewFromFloat(4500.00),
		DepreciationStartDate:        &purchaseDate,
		AccumulatedDepreciation:      decimal.NewFromFloat(4050.00),
		BookValue:                    decimal.NewFromFloat(40950.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}

	err := repo.Create(ctx, tenant.SchemaName, asset)
	if err != nil {
		t.Fatalf("Create asset failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, asset.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != asset.Name {
		t.Errorf("expected name %s, got %s", asset.Name, retrieved.Name)
	}
	if retrieved.AssetNumber != asset.AssetNumber {
		t.Errorf("expected asset number %s, got %s", asset.AssetNumber, retrieved.AssetNumber)
	}
	if !retrieved.PurchaseCost.Equal(asset.PurchaseCost) {
		t.Errorf("expected purchase cost %s, got %s", asset.PurchaseCost, retrieved.PurchaseCost)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, "nonexistent")
	if err != ErrAssetNotFound {
		t.Errorf("expected ErrAssetNotFound, got %v", err)
	}
}

func TestRepository_ListAssets(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	// Create multiple assets
	for i := 0; i < 3; i++ {
		asset := &FixedAsset{
			ID:                           uuid.New().String(),
			TenantID:                     tenant.ID,
			AssetNumber:                  uuid.New().String()[:10],
			Name:                         "Asset " + uuid.New().String()[:8],
			CategoryID:                   &cat.ID,
			Status:                       AssetStatusActive,
			PurchaseDate:                 time.Now(),
			PurchaseCost:                 decimal.NewFromFloat(float64(1000 * (i + 1))),
			DepreciationMethod:           DepreciationStraightLine,
			UsefulLifeMonths:             60,
			ResidualValue:                decimal.Zero,
			AccumulatedDepreciation:      decimal.Zero,
			BookValue:                    decimal.NewFromFloat(float64(1000 * (i + 1))),
			AssetAccountID:               cat.AssetAccountID,
			DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
			AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
			CreatedAt:                    time.Now(),
			CreatedBy:                    uuid.New().String(),
			UpdatedAt:                    time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
			t.Fatalf("Create asset failed: %v", err)
		}
	}

	assets, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(assets) != 3 {
		t.Errorf("expected 3 assets, got %d", len(assets))
	}
}

func TestRepository_ListAssets_WithFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	// Create active asset
	activeAsset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-ACTIVE",
		Name:                         "Active Asset",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusActive,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(5000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(5000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, activeAsset); err != nil {
		t.Fatalf("Create active asset failed: %v", err)
	}

	// Create disposed asset
	disposedAsset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-DISPOSED",
		Name:                         "Disposed Asset",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusDisposed,
		PurchaseDate:                 time.Now().AddDate(-2, 0, 0),
		PurchaseCost:                 decimal.NewFromFloat(3000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.NewFromFloat(3000.00),
		BookValue:                    decimal.Zero,
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, disposedAsset); err != nil {
		t.Fatalf("Create disposed asset failed: %v", err)
	}

	// Filter by status
	filter := &AssetFilter{Status: AssetStatusActive}
	assets, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}

	if len(assets) != 1 {
		t.Errorf("expected 1 active asset, got %d", len(assets))
	}
	if assets[0].Status != AssetStatusActive {
		t.Errorf("expected status %s, got %s", AssetStatusActive, assets[0].Status)
	}
}

func TestRepository_UpdateAsset(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-UPDATE",
		Name:                         "Original Name",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusDraft,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(5000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(5000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	asset.Name = "Updated Name"
	asset.Description = "Updated description"
	asset.Location = "New Location"

	if err := repo.Update(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, asset.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if retrieved.Location != "New Location" {
		t.Errorf("expected location 'New Location', got '%s'", retrieved.Location)
	}
}

func TestRepository_UpdateStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-STATUS",
		Name:                         "Status Test Asset",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusDraft,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(5000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(5000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update status to active
	if err := repo.UpdateStatus(ctx, tenant.SchemaName, tenant.ID, asset.ID, AssetStatusActive); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, asset.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Status != AssetStatusActive {
		t.Errorf("expected status %s, got %s", AssetStatusActive, retrieved.Status)
	}
}

func TestRepository_DeleteAsset(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-DELETE",
		Name:                         "Asset to Delete",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusDraft,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(5000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(5000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, asset.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, asset.ID)
	if err != ErrAssetNotFound {
		t.Errorf("expected ErrAssetNotFound after deletion, got %v", err)
	}
}

func TestRepository_GenerateNumber(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	// First number
	num, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber failed: %v", err)
	}
	if num != "AST-00001" {
		t.Errorf("expected 'AST-00001', got '%s'", num)
	}

	// Create an asset with this number
	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  num,
		Name:                         "First Asset",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusDraft,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(5000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(5000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Second number
	num2, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber (second) failed: %v", err)
	}
	if num2 != "AST-00002" {
		t.Errorf("expected 'AST-00002', got '%s'", num2)
	}
}

func TestRepository_DepreciationEntry(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-DEP",
		Name:                         "Depreciation Test Asset",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusActive,
		PurchaseDate:                 time.Now().AddDate(-1, 0, 0),
		PurchaseCost:                 decimal.NewFromFloat(12000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.NewFromFloat(1200.00),
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(12000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Create asset failed: %v", err)
	}

	// Create depreciation entry
	entry := &DepreciationEntry{
		ID:                 uuid.New().String(),
		TenantID:           tenant.ID,
		AssetID:            asset.ID,
		DepreciationDate:   time.Now(),
		PeriodStart:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:          time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC),
		DepreciationAmount: decimal.NewFromFloat(180.00), // (12000-1200)/60 = 180 per month
		AccumulatedTotal:   decimal.NewFromFloat(180.00),
		BookValueAfter:     decimal.NewFromFloat(11820.00),
		CreatedAt:          time.Now(),
		CreatedBy:          "test-user",
	}
	if err := repo.CreateDepreciationEntry(ctx, tenant.SchemaName, entry); err != nil {
		t.Fatalf("CreateDepreciationEntry failed: %v", err)
	}

	// List depreciation entries
	entries, err := repo.ListDepreciationEntries(ctx, tenant.SchemaName, tenant.ID, asset.ID)
	if err != nil {
		t.Fatalf("ListDepreciationEntries failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if !entries[0].DepreciationAmount.Equal(decimal.NewFromFloat(180.00)) {
		t.Errorf("expected amount 180.00, got %s", entries[0].DepreciationAmount)
	}
}

func TestRepository_UpdateAssetDepreciation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	asset := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-UPDATE-DEP",
		Name:                         "Update Depreciation Test",
		CategoryID:                   &cat.ID,
		Status:                       AssetStatusActive,
		PurchaseDate:                 time.Now().AddDate(-1, 0, 0),
		PurchaseCost:                 decimal.NewFromFloat(12000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.NewFromFloat(1200.00),
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(12000.00),
		AssetAccountID:               cat.AssetAccountID,
		DepreciationExpenseAccountID: cat.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("Create asset failed: %v", err)
	}

	// Update depreciation values
	now := time.Now()
	asset.AccumulatedDepreciation = decimal.NewFromFloat(2160.00) // 12 months
	asset.BookValue = decimal.NewFromFloat(9840.00)
	asset.LastDepreciationDate = &now

	if err := repo.UpdateAssetDepreciation(ctx, tenant.SchemaName, asset); err != nil {
		t.Fatalf("UpdateAssetDepreciation failed: %v", err)
	}

	// Verify
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, asset.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if !retrieved.AccumulatedDepreciation.Equal(decimal.NewFromFloat(2160.00)) {
		t.Errorf("expected accumulated depreciation 2160.00, got %s", retrieved.AccumulatedDepreciation)
	}
	if !retrieved.BookValue.Equal(decimal.NewFromFloat(9840.00)) {
		t.Errorf("expected book value 9840.00, got %s", retrieved.BookValue)
	}
}

func TestRepository_ListAssets_WithCategoryFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	cat1 := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)
	cat2 := createTestCategory(t, repo, tenant.SchemaName, tenant.ID)

	// Create asset in category 1
	asset1 := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-CAT1",
		Name:                         "Category 1 Asset",
		CategoryID:                   &cat1.ID,
		Status:                       AssetStatusActive,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(5000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(5000.00),
		AssetAccountID:               cat1.AssetAccountID,
		DepreciationExpenseAccountID: cat1.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat1.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset1); err != nil {
		t.Fatalf("Create asset 1 failed: %v", err)
	}

	// Create asset in category 2
	asset2 := &FixedAsset{
		ID:                           uuid.New().String(),
		TenantID:                     tenant.ID,
		AssetNumber:                  "AST-CAT2",
		Name:                         "Category 2 Asset",
		CategoryID:                   &cat2.ID,
		Status:                       AssetStatusActive,
		PurchaseDate:                 time.Now(),
		PurchaseCost:                 decimal.NewFromFloat(3000.00),
		DepreciationMethod:           DepreciationStraightLine,
		UsefulLifeMonths:             60,
		ResidualValue:                decimal.Zero,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromFloat(3000.00),
		AssetAccountID:               cat2.AssetAccountID,
		DepreciationExpenseAccountID: cat2.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: cat2.AccumulatedDepreciationAcctID,
		CreatedAt:                    time.Now(),
		CreatedBy:                    uuid.New().String(),
		UpdatedAt:                    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, asset2); err != nil {
		t.Fatalf("Create asset 2 failed: %v", err)
	}

	// Filter by category 1
	filter := &AssetFilter{CategoryID: cat1.ID}
	assets, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with category filter failed: %v", err)
	}

	if len(assets) != 1 {
		t.Errorf("expected 1 asset in category 1, got %d", len(assets))
	}
	if assets[0].CategoryID != cat1.ID {
		t.Errorf("expected category ID %s, got %s", cat1.ID, assets[0].CategoryID)
	}
}
