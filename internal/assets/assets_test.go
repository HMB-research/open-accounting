package assets

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	mu                  sync.RWMutex
	Categories          map[string]*AssetCategory
	Assets              map[string]*FixedAsset
	DepreciationEntries map[string][]DepreciationEntry
	AssetNumberSeq      int
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		Categories:          make(map[string]*AssetCategory),
		Assets:              make(map[string]*FixedAsset),
		DepreciationEntries: make(map[string][]DepreciationEntry),
		AssetNumberSeq:      0,
	}
}

// CreateCategory implements Repository
func (r *MockRepository) CreateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Categories[cat.ID] = cat
	return nil
}

// GetCategoryByID implements Repository
func (r *MockRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*AssetCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cat, exists := r.Categories[categoryID]
	if !exists {
		return nil, ErrCategoryNotFound
	}
	if cat.TenantID != tenantID {
		return nil, ErrCategoryNotFound
	}
	return cat, nil
}

// ListCategories implements Repository
func (r *MockRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]AssetCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []AssetCategory
	for _, cat := range r.Categories {
		if cat.TenantID == tenantID {
			result = append(result, *cat)
		}
	}
	return result, nil
}

// UpdateCategory implements Repository
func (r *MockRepository) UpdateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Categories[cat.ID]; !exists {
		return ErrCategoryNotFound
	}
	r.Categories[cat.ID] = cat
	return nil
}

// DeleteCategory implements Repository
func (r *MockRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cat, exists := r.Categories[categoryID]
	if !exists || cat.TenantID != tenantID {
		return ErrCategoryNotFound
	}
	delete(r.Categories, categoryID)
	return nil
}

// Create implements Repository
func (r *MockRepository) Create(ctx context.Context, schemaName string, asset *FixedAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Assets[asset.ID] = asset
	return nil
}

// GetByID implements Repository
func (r *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, assetID string) (*FixedAsset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	asset, exists := r.Assets[assetID]
	if !exists {
		return nil, ErrAssetNotFound
	}
	if asset.TenantID != tenantID {
		return nil, ErrAssetNotFound
	}
	// Return a copy to avoid data races
	assetCopy := *asset
	return &assetCopy, nil
}

// List implements Repository
func (r *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *AssetFilter) ([]FixedAsset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []FixedAsset
	for _, asset := range r.Assets {
		if asset.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.Status != "" && asset.Status != filter.Status {
				continue
			}
			if filter.CategoryID != "" && (asset.CategoryID == nil || *asset.CategoryID != filter.CategoryID) {
				continue
			}
		}
		result = append(result, *asset)
	}
	return result, nil
}

// Update implements Repository
func (r *MockRepository) Update(ctx context.Context, schemaName string, asset *FixedAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.Assets[asset.ID]
	if !exists {
		return ErrAssetNotFound
	}
	if existing.Status != AssetStatusDraft && existing.Status != AssetStatusActive {
		return ErrAssetNotFound
	}
	r.Assets[asset.ID] = asset
	return nil
}

// UpdateStatus implements Repository
func (r *MockRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, assetID string, status AssetStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset, exists := r.Assets[assetID]
	if !exists || asset.TenantID != tenantID {
		return ErrAssetNotFound
	}
	asset.Status = status
	return nil
}

// Delete implements Repository
func (r *MockRepository) Delete(ctx context.Context, schemaName, tenantID, assetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset, exists := r.Assets[assetID]
	if !exists || asset.TenantID != tenantID || asset.Status != AssetStatusDraft {
		return ErrAssetNotFound
	}
	delete(r.Assets, assetID)
	return nil
}

// GenerateNumber implements Repository
func (r *MockRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.AssetNumberSeq++
	return fmt.Sprintf("FA-%05d", r.AssetNumberSeq), nil
}

// CreateDepreciationEntry implements Repository
func (r *MockRepository) CreateDepreciationEntry(ctx context.Context, schemaName string, entry *DepreciationEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.DepreciationEntries[entry.AssetID] = append(r.DepreciationEntries[entry.AssetID], *entry)
	return nil
}

// ListDepreciationEntries implements Repository
func (r *MockRepository) ListDepreciationEntries(ctx context.Context, schemaName, tenantID, assetID string) ([]DepreciationEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := r.DepreciationEntries[assetID]
	var result []DepreciationEntry
	for _, e := range entries {
		if e.TenantID == tenantID {
			result = append(result, e)
		}
	}
	return result, nil
}

// UpdateAssetDepreciation implements Repository
func (r *MockRepository) UpdateAssetDepreciation(ctx context.Context, schemaName string, asset *FixedAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.Assets[asset.ID]
	if !exists {
		return ErrAssetNotFound
	}
	existing.AccumulatedDepreciation = asset.AccumulatedDepreciation
	existing.BookValue = asset.BookValue
	existing.LastDepreciationDate = asset.LastDepreciationDate
	return nil
}

// Test Fixtures
func TestAssetStatusConstants(t *testing.T) {
	assert.Equal(t, AssetStatus("DRAFT"), AssetStatusDraft)
	assert.Equal(t, AssetStatus("ACTIVE"), AssetStatusActive)
	assert.Equal(t, AssetStatus("DISPOSED"), AssetStatusDisposed)
	assert.Equal(t, AssetStatus("SOLD"), AssetStatusSold)
}

func TestDepreciationMethodConstants(t *testing.T) {
	assert.Equal(t, DepreciationMethod("STRAIGHT_LINE"), DepreciationStraightLine)
	assert.Equal(t, DepreciationMethod("DECLINING_BALANCE"), DepreciationDecliningBalance)
	assert.Equal(t, DepreciationMethod("UNITS_OF_PRODUCTION"), DepreciationUnitsOfProd)
}

func TestDisposalMethodConstants(t *testing.T) {
	assert.Equal(t, DisposalMethod("SOLD"), DisposalSold)
	assert.Equal(t, DisposalMethod("SCRAPPED"), DisposalScrapped)
	assert.Equal(t, DisposalMethod("DONATED"), DisposalDonated)
	assert.Equal(t, DisposalMethod("LOST"), DisposalLost)
}

// MockRepository Tests
func TestMockRepository_CreateCategory(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	cat := &AssetCategory{
		ID:                      "cat-1",
		TenantID:                "tenant-1",
		Name:                    "Furniture",
		DepreciationMethod:      DepreciationStraightLine,
		DefaultUsefulLifeMonths: 60,
	}

	err := repo.CreateCategory(ctx, "test_schema", cat)
	require.NoError(t, err)

	// Verify stored
	stored, err := repo.GetCategoryByID(ctx, "test_schema", "tenant-1", "cat-1")
	require.NoError(t, err)
	assert.Equal(t, "Furniture", stored.Name)
}

func TestMockRepository_GetCategoryByID(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	repo.Categories["cat-1"] = &AssetCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Equipment",
	}

	// Test successful retrieval
	cat, err := repo.GetCategoryByID(ctx, "test_schema", "tenant-1", "cat-1")
	require.NoError(t, err)
	assert.Equal(t, "Equipment", cat.Name)

	// Test not found
	_, err = repo.GetCategoryByID(ctx, "test_schema", "tenant-1", "nonexistent")
	assert.ErrorIs(t, err, ErrCategoryNotFound)

	// Test wrong tenant
	_, err = repo.GetCategoryByID(ctx, "test_schema", "wrong-tenant", "cat-1")
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestMockRepository_ListCategories(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	repo.Categories["cat-1"] = &AssetCategory{ID: "cat-1", TenantID: "tenant-1", Name: "Furniture"}
	repo.Categories["cat-2"] = &AssetCategory{ID: "cat-2", TenantID: "tenant-1", Name: "Equipment"}
	repo.Categories["cat-3"] = &AssetCategory{ID: "cat-3", TenantID: "tenant-2", Name: "Vehicles"}

	categories, err := repo.ListCategories(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)
	assert.Len(t, categories, 2)
}

func TestMockRepository_DeleteCategory(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	repo.Categories["cat-1"] = &AssetCategory{ID: "cat-1", TenantID: "tenant-1", Name: "Equipment"}

	err := repo.DeleteCategory(ctx, "test_schema", "tenant-1", "cat-1")
	require.NoError(t, err)

	// Verify deleted
	_, err = repo.GetCategoryByID(ctx, "test_schema", "tenant-1", "cat-1")
	assert.ErrorIs(t, err, ErrCategoryNotFound)

	// Test delete wrong tenant
	repo.Categories["cat-2"] = &AssetCategory{ID: "cat-2", TenantID: "tenant-2", Name: "Other"}
	err = repo.DeleteCategory(ctx, "test_schema", "tenant-1", "cat-2")
	assert.ErrorIs(t, err, ErrCategoryNotFound)
}

func TestMockRepository_CreateAsset(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	asset := &FixedAsset{
		ID:           "asset-1",
		TenantID:     "tenant-1",
		AssetNumber:  "FA-00001",
		Name:         "Office Desk",
		PurchaseCost: decimal.NewFromInt(500),
		Status:       AssetStatusDraft,
	}

	err := repo.Create(ctx, "test_schema", asset)
	require.NoError(t, err)

	stored, err := repo.GetByID(ctx, "test_schema", "tenant-1", "asset-1")
	require.NoError(t, err)
	assert.Equal(t, "Office Desk", stored.Name)
}

func TestMockRepository_ListAssets(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	catID := "cat-1"
	repo.Assets["a1"] = &FixedAsset{ID: "a1", TenantID: "tenant-1", Name: "Desk", Status: AssetStatusActive, CategoryID: &catID}
	repo.Assets["a2"] = &FixedAsset{ID: "a2", TenantID: "tenant-1", Name: "Chair", Status: AssetStatusDraft, CategoryID: &catID}
	repo.Assets["a3"] = &FixedAsset{ID: "a3", TenantID: "tenant-2", Name: "Laptop", Status: AssetStatusActive}

	// List all for tenant-1
	assets, err := repo.List(ctx, "test_schema", "tenant-1", nil)
	require.NoError(t, err)
	assert.Len(t, assets, 2)

	// Filter by status
	assets, err = repo.List(ctx, "test_schema", "tenant-1", &AssetFilter{Status: AssetStatusActive})
	require.NoError(t, err)
	assert.Len(t, assets, 1)
	assert.Equal(t, "Desk", assets[0].Name)

	// Filter by category
	assets, err = repo.List(ctx, "test_schema", "tenant-1", &AssetFilter{CategoryID: "cat-1"})
	require.NoError(t, err)
	assert.Len(t, assets, 2)
}

func TestMockRepository_DeleteAsset(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	repo.Assets["a1"] = &FixedAsset{ID: "a1", TenantID: "tenant-1", Name: "Desk", Status: AssetStatusDraft}

	err := repo.Delete(ctx, "test_schema", "tenant-1", "a1")
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.ErrorIs(t, err, ErrAssetNotFound)

	// Cannot delete non-draft asset
	repo.Assets["a2"] = &FixedAsset{ID: "a2", TenantID: "tenant-1", Name: "Chair", Status: AssetStatusActive}
	err = repo.Delete(ctx, "test_schema", "tenant-1", "a2")
	assert.ErrorIs(t, err, ErrAssetNotFound)
}

func TestMockRepository_GenerateNumber(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	num1, err := repo.GenerateNumber(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "FA-00001", num1)

	num2, err := repo.GenerateNumber(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "FA-00002", num2)
}

// FixedAsset Tests
func TestFixedAsset_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		asset   FixedAsset
		wantErr string
	}{
		{
			name: "valid asset",
			asset: FixedAsset{
				Name:             "Desk",
				PurchaseDate:     now,
				PurchaseCost:     decimal.NewFromInt(500),
				UsefulLifeMonths: 60,
				ResidualValue:    decimal.NewFromInt(50),
			},
			wantErr: "",
		},
		{
			name: "missing name",
			asset: FixedAsset{
				PurchaseDate:     now,
				PurchaseCost:     decimal.NewFromInt(500),
				UsefulLifeMonths: 60,
			},
			wantErr: "name is required",
		},
		{
			name: "missing purchase date",
			asset: FixedAsset{
				Name:             "Desk",
				PurchaseCost:     decimal.NewFromInt(500),
				UsefulLifeMonths: 60,
			},
			wantErr: "purchase date is required",
		},
		{
			name: "zero purchase cost",
			asset: FixedAsset{
				Name:             "Desk",
				PurchaseDate:     now,
				PurchaseCost:     decimal.Zero,
				UsefulLifeMonths: 60,
			},
			wantErr: "purchase cost must be positive",
		},
		{
			name: "negative useful life",
			asset: FixedAsset{
				Name:             "Desk",
				PurchaseDate:     now,
				PurchaseCost:     decimal.NewFromInt(500),
				UsefulLifeMonths: -1,
			},
			wantErr: "useful life must be positive",
		},
		{
			name: "negative residual value",
			asset: FixedAsset{
				Name:             "Desk",
				PurchaseDate:     now,
				PurchaseCost:     decimal.NewFromInt(500),
				UsefulLifeMonths: 60,
				ResidualValue:    decimal.NewFromInt(-100),
			},
			wantErr: "residual value cannot be negative",
		},
		{
			name: "residual exceeds purchase cost",
			asset: FixedAsset{
				Name:             "Desk",
				PurchaseDate:     now,
				PurchaseCost:     decimal.NewFromInt(500),
				UsefulLifeMonths: 60,
				ResidualValue:    decimal.NewFromInt(600),
			},
			wantErr: "residual value cannot exceed purchase cost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.asset.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestFixedAsset_CalculateMonthlyDepreciation(t *testing.T) {
	tests := []struct {
		name     string
		asset    FixedAsset
		expected decimal.Decimal
	}{
		{
			name: "straight line - basic",
			asset: FixedAsset{
				PurchaseCost:       decimal.NewFromInt(1200),
				ResidualValue:      decimal.NewFromInt(0),
				UsefulLifeMonths:   12,
				BookValue:          decimal.NewFromInt(1200),
				DepreciationMethod: DepreciationStraightLine,
			},
			expected: decimal.NewFromInt(100),
		},
		{
			name: "straight line - with residual",
			asset: FixedAsset{
				PurchaseCost:       decimal.NewFromInt(1200),
				ResidualValue:      decimal.NewFromInt(200),
				UsefulLifeMonths:   10,
				BookValue:          decimal.NewFromInt(1200),
				DepreciationMethod: DepreciationStraightLine,
			},
			expected: decimal.NewFromInt(100),
		},
		{
			name: "zero useful life",
			asset: FixedAsset{
				PurchaseCost:       decimal.NewFromInt(1200),
				UsefulLifeMonths:   0,
				DepreciationMethod: DepreciationStraightLine,
			},
			expected: decimal.Zero,
		},
		{
			name: "residual equals purchase cost",
			asset: FixedAsset{
				PurchaseCost:       decimal.NewFromInt(1200),
				ResidualValue:      decimal.NewFromInt(1200),
				UsefulLifeMonths:   12,
				DepreciationMethod: DepreciationStraightLine,
			},
			expected: decimal.Zero,
		},
		{
			name: "declining balance",
			asset: FixedAsset{
				PurchaseCost:       decimal.NewFromInt(12000),
				ResidualValue:      decimal.NewFromInt(0),
				UsefulLifeMonths:   60, // 5 years
				BookValue:          decimal.NewFromInt(12000),
				DepreciationMethod: DepreciationDecliningBalance,
			},
			// Rate = 2 / 5 years = 0.4, monthly = 0.4/12, 12000 * 0.4/12 = 400
			expected: decimal.NewFromInt(400),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.asset.CalculateMonthlyDepreciation()
			assert.True(t, tt.expected.Equal(result), "expected %s, got %s", tt.expected, result)
		})
	}
}

// Service Tests
type testService struct {
	repo *MockRepository
	svc  *Service
}

func newTestService() *testService {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	return &testService{repo: repo, svc: svc}
}

func TestService_CreateCategory(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateCategoryRequest{
		Name:                    "Furniture",
		Description:             "Office furniture",
		DepreciationMethod:      DepreciationStraightLine,
		DefaultUsefulLifeMonths: 60,
	}

	cat, err := ts.svc.CreateCategory(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, cat.ID)
	assert.Equal(t, "Furniture", cat.Name)
	assert.Equal(t, DepreciationStraightLine, cat.DepreciationMethod)
}

func TestService_CreateCategory_Defaults(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateCategoryRequest{
		Name: "Equipment",
	}

	cat, err := ts.svc.CreateCategory(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, DepreciationStraightLine, cat.DepreciationMethod)
	assert.Equal(t, 60, cat.DefaultUsefulLifeMonths)
}

func TestService_GetCategoryByID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-1"] = &AssetCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Furniture",
	}

	cat, err := ts.svc.GetCategoryByID(ctx, "tenant-1", "test_schema", "cat-1")
	require.NoError(t, err)
	assert.Equal(t, "Furniture", cat.Name)

	// Not found
	_, err = ts.svc.GetCategoryByID(ctx, "tenant-1", "test_schema", "nonexistent")
	assert.Error(t, err)
}

func TestService_ListCategories(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-1"] = &AssetCategory{ID: "cat-1", TenantID: "tenant-1", Name: "Furniture"}
	ts.repo.Categories["cat-2"] = &AssetCategory{ID: "cat-2", TenantID: "tenant-1", Name: "Equipment"}

	categories, err := ts.svc.ListCategories(ctx, "tenant-1", "test_schema")
	require.NoError(t, err)
	assert.Len(t, categories, 2)
}

func TestService_DeleteCategory(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-1"] = &AssetCategory{ID: "cat-1", TenantID: "tenant-1", Name: "Furniture"}

	err := ts.svc.DeleteCategory(ctx, "tenant-1", "test_schema", "cat-1")
	require.NoError(t, err)

	_, err = ts.svc.GetCategoryByID(ctx, "tenant-1", "test_schema", "cat-1")
	assert.Error(t, err)
}

func TestService_CreateAsset(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateAssetRequest{
		Name:               "Office Desk",
		Description:        "Wooden desk",
		PurchaseDate:       time.Now(),
		PurchaseCost:       decimal.NewFromInt(500),
		UsefulLifeMonths:   60,
		ResidualValue:      decimal.NewFromInt(50),
		DepreciationMethod: DepreciationStraightLine,
		UserID:             "user-1",
	}

	asset, err := ts.svc.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, asset.ID)
	assert.Equal(t, "FA-00001", asset.AssetNumber)
	assert.Equal(t, "Office Desk", asset.Name)
	assert.Equal(t, AssetStatusDraft, asset.Status)
	assert.True(t, asset.BookValue.Equal(decimal.NewFromInt(500)))
}

func TestService_CreateAsset_Defaults(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateAssetRequest{
		Name:         "Chair",
		PurchaseDate: time.Now(),
		PurchaseCost: decimal.NewFromInt(200),
		UserID:       "user-1",
	}

	asset, err := ts.svc.Create(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, DepreciationStraightLine, asset.DepreciationMethod)
	assert.Equal(t, 60, asset.UsefulLifeMonths)
}

func TestService_CreateAsset_ValidationError(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateAssetRequest{
		Name:         "", // Empty name
		PurchaseDate: time.Now(),
		PurchaseCost: decimal.NewFromInt(500),
	}

	_, err := ts.svc.Create(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestService_GetByID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
	}

	asset, err := ts.svc.GetByID(ctx, "tenant-1", "test_schema", "a1")
	require.NoError(t, err)
	assert.Equal(t, "Desk", asset.Name)
}

func TestService_List(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{ID: "a1", TenantID: "tenant-1", Name: "Desk", Status: AssetStatusActive}
	ts.repo.Assets["a2"] = &FixedAsset{ID: "a2", TenantID: "tenant-1", Name: "Chair", Status: AssetStatusDraft}

	assets, err := ts.svc.List(ctx, "tenant-1", "test_schema", nil)
	require.NoError(t, err)
	assert.Len(t, assets, 2)

	// With filter
	assets, err = ts.svc.List(ctx, "tenant-1", "test_schema", &AssetFilter{Status: AssetStatusActive})
	require.NoError(t, err)
	assert.Len(t, assets, 1)
}

func TestService_Update(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:                 "a1",
		TenantID:           "tenant-1",
		Name:               "Old Name",
		Status:             AssetStatusDraft,
		PurchaseDate:       time.Now(),
		PurchaseCost:       decimal.NewFromInt(500),
		UsefulLifeMonths:   60,
		DepreciationMethod: DepreciationStraightLine,
	}

	req := &UpdateAssetRequest{
		Name:               "New Name",
		Description:        "Updated description",
		UsefulLifeMonths:   48,
		DepreciationMethod: DepreciationStraightLine,
	}

	asset, err := ts.svc.Update(ctx, "tenant-1", "test_schema", "a1", req)
	require.NoError(t, err)
	assert.Equal(t, "New Name", asset.Name)
	assert.Equal(t, 48, asset.UsefulLifeMonths)
}

func TestService_Update_NotDraft(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
		Status:   AssetStatusDisposed,
	}

	req := &UpdateAssetRequest{Name: "New Name"}
	_, err := ts.svc.Update(ctx, "tenant-1", "test_schema", "a1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only draft or active assets can be updated")
}

func TestService_Activate(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
		Status:   AssetStatusDraft,
	}

	err := ts.svc.Activate(ctx, "tenant-1", "test_schema", "a1")
	require.NoError(t, err)

	asset, err := ts.repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	require.NoError(t, err)
	assert.Equal(t, AssetStatusActive, asset.Status)
}

func TestService_Activate_NotDraft(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
		Status:   AssetStatusActive,
	}

	err := ts.svc.Activate(ctx, "tenant-1", "test_schema", "a1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in draft status")
}

func TestService_Dispose(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
		Status:   AssetStatusActive,
	}

	req := &DisposeAssetRequest{
		DisposalDate:     time.Now(),
		DisposalMethod:   DisposalSold,
		DisposalProceeds: decimal.NewFromInt(100),
		DisposalNotes:    "Sold to company X",
	}

	err := ts.svc.Dispose(ctx, "tenant-1", "test_schema", "a1", req)
	require.NoError(t, err)

	asset, _ := ts.repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.Equal(t, AssetStatusSold, asset.Status)
}

func TestService_Dispose_NotActive(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
		Status:   AssetStatusDraft,
	}

	req := &DisposeAssetRequest{
		DisposalDate:   time.Now(),
		DisposalMethod: DisposalScrapped,
	}

	err := ts.svc.Dispose(ctx, "tenant-1", "test_schema", "a1", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only active assets can be disposed")
}

func TestService_Dispose_Scrapped(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Old Computer",
		Status:   AssetStatusActive,
	}

	req := &DisposeAssetRequest{
		DisposalDate:   time.Now(),
		DisposalMethod: DisposalScrapped,
	}

	err := ts.svc.Dispose(ctx, "tenant-1", "test_schema", "a1", req)
	require.NoError(t, err)

	asset, _ := ts.repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.Equal(t, AssetStatusDisposed, asset.Status)
}

func TestService_Delete(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Desk",
		Status:   AssetStatusDraft,
	}

	err := ts.svc.Delete(ctx, "tenant-1", "test_schema", "a1")
	require.NoError(t, err)

	_, err = ts.svc.GetByID(ctx, "tenant-1", "test_schema", "a1")
	assert.Error(t, err)
}

func TestService_RecordDepreciation(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:                      "a1",
		TenantID:                "tenant-1",
		Name:                    "Equipment",
		Status:                  AssetStatusActive,
		PurchaseCost:            decimal.NewFromInt(12000),
		ResidualValue:           decimal.NewFromInt(0),
		UsefulLifeMonths:        12,
		DepreciationMethod:      DepreciationStraightLine,
		AccumulatedDepreciation: decimal.Zero,
		BookValue:               decimal.NewFromInt(12000),
	}

	now := time.Now()
	entry, err := ts.svc.RecordDepreciation(ctx, "tenant-1", "test_schema", "a1", "user-1", now.AddDate(0, -1, 0), now)
	require.NoError(t, err)
	assert.True(t, entry.DepreciationAmount.Equal(decimal.NewFromInt(1000)))
	assert.True(t, entry.AccumulatedTotal.Equal(decimal.NewFromInt(1000)))
	assert.True(t, entry.BookValueAfter.Equal(decimal.NewFromInt(11000)))
}

func TestService_RecordDepreciation_NotActive(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:       "a1",
		TenantID: "tenant-1",
		Name:     "Equipment",
		Status:   AssetStatusDraft,
	}

	now := time.Now()
	_, err := ts.svc.RecordDepreciation(ctx, "tenant-1", "test_schema", "a1", "user-1", now, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only active assets can be depreciated")
}

func TestService_RecordDepreciation_FullyDepreciated(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["a1"] = &FixedAsset{
		ID:                      "a1",
		TenantID:                "tenant-1",
		Name:                    "Equipment",
		Status:                  AssetStatusActive,
		PurchaseCost:            decimal.NewFromInt(1000),
		ResidualValue:           decimal.NewFromInt(100),
		UsefulLifeMonths:        12,
		DepreciationMethod:      DepreciationStraightLine,
		AccumulatedDepreciation: decimal.NewFromInt(900), // Fully depreciated
		BookValue:               decimal.NewFromInt(100),
	}

	now := time.Now()
	_, err := ts.svc.RecordDepreciation(ctx, "tenant-1", "test_schema", "a1", "user-1", now, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fully depreciated")
}

func TestService_GetDepreciationHistory(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.DepreciationEntries["a1"] = []DepreciationEntry{
		{ID: "e1", TenantID: "tenant-1", AssetID: "a1", DepreciationAmount: decimal.NewFromInt(100)},
		{ID: "e2", TenantID: "tenant-1", AssetID: "a1", DepreciationAmount: decimal.NewFromInt(100)},
	}

	entries, err := ts.svc.GetDepreciationHistory(ctx, "tenant-1", "test_schema", "a1")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestNewService(t *testing.T) {
	// Test that NewService creates a service with PostgresRepository
	svc := NewService(nil)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepository(repo)
	assert.NotNil(t, svc)
	assert.Equal(t, repo, svc.repo)
}
