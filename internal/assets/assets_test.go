package assets

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contactrefs"
	"github.com/HMB-research/open-accounting/internal/contacts"
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

// UpdateDisposal implements Repository
func (r *MockRepository) UpdateDisposal(ctx context.Context, schemaName string, asset *FixedAsset, status AssetStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.Assets[asset.ID]
	if !exists || existing.TenantID != asset.TenantID || existing.Status != AssetStatusActive {
		return ErrAssetNotFound
	}
	existing.Status = status
	existing.DisposalDate = asset.DisposalDate
	existing.DisposalMethod = asset.DisposalMethod
	existing.DisposalProceeds = asset.DisposalProceeds
	existing.DisposalNotes = asset.DisposalNotes
	existing.DisposalJournalEntryID = asset.DisposalJournalEntryID
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

func TestService_CreateAsset_InheritsCategoryDefaults(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	assetAccountID := "asset-account"
	depreciationExpenseAccountID := "depreciation-expense"
	accumulatedDepreciationAccountID := "accumulated-depreciation"
	ts.repo.Categories["cat-1"] = &AssetCategory{
		ID:                            "cat-1",
		TenantID:                      "tenant-1",
		Name:                          "Equipment",
		DepreciationMethod:            DepreciationDecliningBalance,
		DefaultUsefulLifeMonths:       36,
		DefaultResidualValuePercent:   decimal.NewFromInt(10),
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
	}
	categoryID := "cat-1"

	asset, err := ts.svc.Create(ctx, "tenant-1", "test_schema", &CreateAssetRequest{
		Name:         "Laptop",
		CategoryID:   &categoryID,
		PurchaseDate: time.Now(),
		PurchaseCost: decimal.NewFromInt(1200),
		UserID:       "user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, DepreciationDecliningBalance, asset.DepreciationMethod)
	assert.Equal(t, 36, asset.UsefulLifeMonths)
	assert.True(t, asset.ResidualValue.Equal(decimal.NewFromInt(120)))
	require.NotNil(t, asset.AssetAccountID)
	assert.Equal(t, assetAccountID, *asset.AssetAccountID)
	require.NotNil(t, asset.DepreciationExpenseAccountID)
	assert.Equal(t, depreciationExpenseAccountID, *asset.DepreciationExpenseAccountID)
	require.NotNil(t, asset.AccumulatedDepreciationAcctID)
	assert.Equal(t, accumulatedDepreciationAccountID, *asset.AccumulatedDepreciationAcctID)
}

func TestService_ImportAssetsCSV(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	ts.svc.ledger = newFakeAssetAccountingPoster()

	ts.repo.Categories["cat-1"] = &AssetCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Equipment",
	}

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		FileName: "assets.csv",
		UserID:   "user-1",
		CSVContent: "asset_number,name,category_name,status,purchase_date,purchase_cost,accumulated_depreciation,book_value,useful_life_months,asset_account_code,depreciation_expense_account_code,accumulated_depreciation_account_code\n" +
			"LEG-001,Laptop,Equipment,ACTIVE,2025-01-10,1200.00,300.00,900.00,36,FA,DEP-EXP,ACC-DEP\n" +
			",Missing date,,ACTIVE,,500.00,,,60,,,\n" +
			",Generated desk,,DRAFT,2026-02-01,600.00,,,60,,,\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "assets.csv", result.FileName)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 2, result.AssetsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 3, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "purchase_date is required")

	var legacyAsset *FixedAsset
	var generatedAsset *FixedAsset
	for _, asset := range ts.repo.Assets {
		switch asset.Name {
		case "Laptop":
			legacyAsset = asset
		case "Generated desk":
			generatedAsset = asset
		}
	}

	require.NotNil(t, legacyAsset)
	assert.Equal(t, "LEG-001", legacyAsset.AssetNumber)
	assert.Equal(t, AssetStatusActive, legacyAsset.Status)
	assert.True(t, legacyAsset.BookValue.Equal(decimal.RequireFromString("900.00")))
	assert.True(t, legacyAsset.AccumulatedDepreciation.Equal(decimal.RequireFromString("300.00")))
	require.NotNil(t, legacyAsset.CategoryID)
	assert.Equal(t, "cat-1", *legacyAsset.CategoryID)
	require.NotNil(t, legacyAsset.AssetAccountID)
	assert.Equal(t, "fixed-assets", *legacyAsset.AssetAccountID)
	require.NotNil(t, legacyAsset.DepreciationExpenseAccountID)
	assert.Equal(t, "depreciation-expense", *legacyAsset.DepreciationExpenseAccountID)
	require.NotNil(t, legacyAsset.AccumulatedDepreciationAcctID)
	assert.Equal(t, "accumulated-depreciation", *legacyAsset.AccumulatedDepreciationAcctID)
	assert.Equal(t, "user-1", legacyAsset.CreatedBy)

	require.NotNil(t, generatedAsset)
	assert.Equal(t, "FA-00001", generatedAsset.AssetNumber)
	assert.Equal(t, AssetStatusDraft, generatedAsset.Status)
	assert.True(t, generatedAsset.BookValue.Equal(decimal.RequireFromString("600.00")))
}

func TestAssetImportParsesCSVEdgeCases(t *testing.T) {
	rows, err := parseAssetImportRows("\ufeffasset_no;asset_name;date;cost;unknown.header\nLEG-001;Laptop;2025-01-10;1200.00;ignored\n;;;;\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].rowNumber)
	assert.Equal(t, "LEG-001", rows[0].values["asset_number"])
	assert.Equal(t, "Laptop", rows[0].values["name"])
	assert.Equal(t, "2025-01-10", rows[0].values["purchase_date"])
	assert.Equal(t, "1200.00", rows[0].values["purchase_cost"])
	assert.Equal(t, "ignored", rows[0].values["unknown_header"])

	rows, err = parseAssetImportRows("name\tpurchase_date\tpurchase_cost\nDesk\t2025-01-10\t500.00\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Desk", rows[0].values["name"])

	rows, err = parseAssetImportRows("name,,purchase_date,purchase_cost\nDesk,ignored,2025-01-10,500.00\n")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.NotContains(t, rows[0].values, "")
	assert.Equal(t, "Desk", rows[0].values["name"])

	rows, err = parseAssetImportRows("name,purchase_date,purchase_cost\n,,\n")
	require.NoError(t, err)
	assert.Empty(t, rows)

	_, err = parseAssetImportRows(" ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseAssetImportRows("name,purchase_cost\nLaptop,1200.00\n")
	require.ErrorContains(t, err, "missing required columns: purchase_date")

	_, err = parseAssetImportRows("\"unterminated")
	require.ErrorContains(t, err, "parse csv header")

	_, err = parseAssetImportRows("name,purchase_date,purchase_cost\n\"unterminated")
	require.ErrorContains(t, err, "parse csv row 2")
}

func TestAssetImportParsersCoverDefaultsAndVariants(t *testing.T) {
	status, err := parseAssetImportStatus(" ")
	require.NoError(t, err)
	assert.Equal(t, AssetStatusDraft, status)
	status, err = parseAssetImportStatus("sold")
	require.NoError(t, err)
	assert.Equal(t, AssetStatusSold, status)
	_, err = parseAssetImportStatus("retired")
	require.ErrorContains(t, err, "invalid status")

	method, err := parseAssetImportDepreciationMethod(" ")
	require.NoError(t, err)
	assert.Equal(t, DepreciationStraightLine, method)
	method, err = parseAssetImportDepreciationMethod("declining-balance")
	require.NoError(t, err)
	assert.Equal(t, DepreciationDecliningBalance, method)
	method, err = parseAssetImportDepreciationMethod("units of production")
	require.NoError(t, err)
	assert.Equal(t, DepreciationUnitsOfProd, method)
	_, err = parseAssetImportDepreciationMethod("manual")
	require.ErrorContains(t, err, "invalid depreciation_method")

	disposalMethod, err := parseAssetImportDisposalMethod(" ")
	require.NoError(t, err)
	assert.Nil(t, disposalMethod)
	for _, value := range []string{"sold", "scrapped", "donated", "lost"} {
		disposalMethod, err = parseAssetImportDisposalMethod(value)
		require.NoError(t, err)
		assert.Equal(t, DisposalMethod(strings.ToUpper(value)), *disposalMethod)
	}
	_, err = parseAssetImportDisposalMethod("recycled")
	require.ErrorContains(t, err, "invalid disposal_method")

	usefulLife, err := parseAssetImportUsefulLifeMonths(" ")
	require.NoError(t, err)
	assert.Equal(t, 60, usefulLife)
	_, err = parseAssetImportUsefulLifeMonths("sixty")
	require.ErrorContains(t, err, "must be an integer")
	_, err = parseAssetImportUsefulLifeMonths("0")
	require.ErrorContains(t, err, "must be positive")

	value, err := parseAssetImportOptionalDecimal("residual_value", " ", decimal.NewFromInt(25))
	require.NoError(t, err)
	assert.True(t, value.Equal(decimal.NewFromInt(25)))
	_, err = parseAssetImportOptionalDecimal("residual_value", "abc", decimal.Zero)
	require.ErrorContains(t, err, "residual_value must be a decimal")

	bookValue, provided, err := parseAssetImportBookValue(" ", decimal.NewFromInt(100))
	require.NoError(t, err)
	assert.False(t, provided)
	assert.True(t, bookValue.Equal(decimal.NewFromInt(100)))
	_, provided, err = parseAssetImportBookValue("abc", decimal.Zero)
	assert.True(t, provided)
	require.ErrorContains(t, err, "book_value must be a decimal")

	parsedDate, err := parseAssetImportRequiredDate("purchase_date", "2025-01-10T12:30:00Z")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2025, 1, 10, 12, 30, 0, 0, time.UTC), parsedDate)
	_, err = parseAssetImportRequiredDate("purchase_date", " ")
	require.ErrorContains(t, err, "purchase_date is required")
	_, err = parseAssetImportOptionalDate("last_depreciation_date", "10/01/2025")
	require.ErrorContains(t, err, "last_depreciation_date must be a date")

	parsedID, err := parseOptionalAssetImportUUID("category_id", " ")
	require.NoError(t, err)
	assert.Nil(t, parsedID)
}

func TestBuildFixedAssetFromImportRowEdgeCases(t *testing.T) {
	categoryID := "11111111-1111-4111-8111-111111111111"
	supplierID := "22222222-2222-4222-8222-222222222222"
	invoiceID := "33333333-3333-4333-8333-333333333333"
	accountIDsByCode := map[string]string{
		"fa":      "asset-account-id",
		"dep-exp": "depreciation-expense-account-id",
		"acc-dep": "accumulated-depreciation-account-id",
	}
	categoryNameToID := map[string]string{"equipment": categoryID}

	row := assetImportTestRow(map[string]string{
		"asset_number":                          " LEG-001 ",
		"description":                           " Workstation ",
		"category_name":                         "Equipment",
		"status":                                "active",
		"depreciation_method":                   "declining-balance",
		"useful_life_months":                    "36",
		"residual_value":                        "100.00",
		"depreciation_start_date":               "2025-02-01",
		"accumulated_depreciation":              "300.00",
		"book_value":                            "900.00",
		"last_depreciation_date":                "2025-12-31T00:00:00Z",
		"disposal_date":                         "2026-01-15 12:30:00",
		"disposal_method":                       "sold",
		"disposal_proceeds":                     "50.00",
		"disposal_notes":                        " Sold after upgrade ",
		"asset_account_code":                    "FA",
		"depreciation_expense_account_code":     "DEP-EXP",
		"accumulated_depreciation_account_code": "ACC-DEP",
		"supplier_id":                           supplierID,
		"serial_number":                         "SN-1",
		"location":                              "Office",
	})

	asset, err := buildFixedAssetFromImportRow(row, "tenant-1", "user-1", categoryNameToID, accountIDsByCode, contactrefs.SupplierLookup{}, &invoiceID)
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", asset.TenantID)
	assert.Equal(t, "LEG-001", asset.AssetNumber)
	assert.Equal(t, "Laptop", asset.Name)
	assert.Equal(t, AssetStatusActive, asset.Status)
	require.NotNil(t, asset.CategoryID)
	assert.Equal(t, categoryID, *asset.CategoryID)
	assert.Equal(t, DepreciationDecliningBalance, asset.DepreciationMethod)
	assert.Equal(t, 36, asset.UsefulLifeMonths)
	assert.True(t, asset.ResidualValue.Equal(decimal.RequireFromString("100.00")))
	assert.True(t, asset.AccumulatedDepreciation.Equal(decimal.RequireFromString("300.00")))
	assert.True(t, asset.BookValue.Equal(decimal.RequireFromString("900.00")))
	require.NotNil(t, asset.DepreciationStartDate)
	require.NotNil(t, asset.LastDepreciationDate)
	require.NotNil(t, asset.DisposalDate)
	require.NotNil(t, asset.DisposalMethod)
	assert.Equal(t, DisposalSold, *asset.DisposalMethod)
	assert.True(t, asset.DisposalProceeds.Equal(decimal.RequireFromString("50.00")))
	require.NotNil(t, asset.AssetAccountID)
	assert.Equal(t, "asset-account-id", *asset.AssetAccountID)
	require.NotNil(t, asset.DepreciationExpenseAccountID)
	assert.Equal(t, "depreciation-expense-account-id", *asset.DepreciationExpenseAccountID)
	require.NotNil(t, asset.AccumulatedDepreciationAcctID)
	assert.Equal(t, "accumulated-depreciation-account-id", *asset.AccumulatedDepreciationAcctID)
	require.NotNil(t, asset.SupplierID)
	assert.Equal(t, supplierID, *asset.SupplierID)
	require.NotNil(t, asset.InvoiceID)
	assert.Equal(t, invoiceID, *asset.InvoiceID)
	assert.Equal(t, "user-1", asset.CreatedBy)

	tests := []struct {
		name             string
		overrides        map[string]string
		categoryNameToID map[string]string
		accountIDsByCode map[string]string
		wantErr          string
	}{
		{name: "missing name", overrides: map[string]string{"name": " "}, wantErr: "name is required"},
		{name: "invalid purchase date", overrides: map[string]string{"purchase_date": "01/10/2025"}, wantErr: "purchase_date must be a date"},
		{name: "missing purchase cost", overrides: map[string]string{"purchase_cost": " "}, wantErr: "purchase_cost is required"},
		{name: "invalid purchase cost", overrides: map[string]string{"purchase_cost": "abc"}, wantErr: "purchase_cost must be a decimal"},
		{name: "nonpositive purchase cost", overrides: map[string]string{"purchase_cost": "0"}, wantErr: "purchase cost must be positive"},
		{name: "invalid status", overrides: map[string]string{"status": "retired"}, wantErr: "invalid status"},
		{name: "invalid depreciation method", overrides: map[string]string{"depreciation_method": "manual"}, wantErr: "invalid depreciation_method"},
		{name: "invalid useful life", overrides: map[string]string{"useful_life_months": "zero"}, wantErr: "useful_life_months must be an integer"},
		{name: "negative useful life", overrides: map[string]string{"useful_life_months": "-1"}, wantErr: "useful_life_months must be positive"},
		{name: "invalid residual", overrides: map[string]string{"residual_value": "abc"}, wantErr: "residual_value must be a decimal"},
		{name: "negative residual", overrides: map[string]string{"residual_value": "-1"}, wantErr: "residual value cannot be negative"},
		{name: "residual exceeds cost", overrides: map[string]string{"residual_value": "1300"}, wantErr: "residual value cannot exceed purchase cost"},
		{name: "invalid accumulated depreciation", overrides: map[string]string{"accumulated_depreciation": "abc"}, wantErr: "accumulated_depreciation must be a decimal"},
		{name: "negative accumulated depreciation", overrides: map[string]string{"accumulated_depreciation": "-1"}, wantErr: "accumulated_depreciation cannot be negative"},
		{name: "invalid book value", overrides: map[string]string{"book_value": "abc"}, wantErr: "book_value must be a decimal"},
		{name: "negative book value", overrides: map[string]string{"purchase_cost": "100", "accumulated_depreciation": "101"}, wantErr: "book_value cannot be negative"},
		{name: "book value mismatch", overrides: map[string]string{"accumulated_depreciation": "100", "book_value": "1200"}, wantErr: "book_value must equal purchase_cost minus accumulated_depreciation"},
		{name: "invalid depreciation start date", overrides: map[string]string{"depreciation_start_date": "02/01/2025"}, wantErr: "depreciation_start_date must be a date"},
		{name: "invalid last depreciation date", overrides: map[string]string{"last_depreciation_date": "12/31/2025"}, wantErr: "last_depreciation_date must be a date"},
		{name: "invalid disposal date", overrides: map[string]string{"disposal_date": "01/15/2026"}, wantErr: "disposal_date must be a date"},
		{name: "invalid disposal method", overrides: map[string]string{"disposal_method": "recycled"}, wantErr: "invalid disposal_method"},
		{name: "invalid disposal proceeds", overrides: map[string]string{"disposal_proceeds": "abc"}, wantErr: "disposal_proceeds must be a decimal"},
		{name: "negative disposal proceeds", overrides: map[string]string{"disposal_proceeds": "-1"}, wantErr: "disposal_proceeds cannot be negative"},
		{name: "missing category", overrides: map[string]string{"category_name": "Vehicles"}, categoryNameToID: map[string]string{}, wantErr: `category_name "Vehicles" was not found`},
		{name: "invalid category id", overrides: map[string]string{"category_id": "legacy-category"}, wantErr: "category_id must be a valid UUID"},
		{name: "missing account code", overrides: map[string]string{"asset_account_code": "MISSING"}, accountIDsByCode: map[string]string{}, wantErr: `account code "MISSING" was not found for asset_account_code`},
		{name: "invalid supplier id", overrides: map[string]string{"supplier_id": "legacy-supplier"}, wantErr: "supplier_id must be a valid UUID"},
		{
			name:      "accumulated depreciation exceeds depreciable amount",
			overrides: map[string]string{"purchase_cost": "1000", "residual_value": "500", "accumulated_depreciation": "600", "book_value": "400"},
			wantErr:   "accumulated_depreciation cannot exceed depreciable amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			categories := categoryNameToID
			if tt.categoryNameToID != nil {
				categories = tt.categoryNameToID
			}
			accounts := accountIDsByCode
			if tt.accountIDsByCode != nil {
				accounts = tt.accountIDsByCode
			}

			_, err := buildFixedAssetFromImportRow(assetImportTestRow(tt.overrides), "tenant-1", "user-1", categories, accounts, contactrefs.SupplierLookup{}, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func assetImportTestRow(overrides map[string]string) assetImportRow {
	values := map[string]string{
		"name":          "Laptop",
		"purchase_date": "2025-01-10",
		"purchase_cost": "1200.00",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return assetImportRow{
		rowNumber: 2,
		values:    values,
	}
}

func TestService_ImportAssetsCSVRejectsInvalidPayloads(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	_, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", nil)
	require.ErrorContains(t, err, "csv_content is required")

	_, err = ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{CSVContent: " "})
	require.ErrorContains(t, err, "csv_content is required")

	_, err = ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "name,purchase_date,purchase_cost\n,,\n",
	})
	require.ErrorContains(t, err, "no assets found in CSV")

	_, err = ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "name,purchase_cost\nLaptop,1200.00\n",
	})
	require.ErrorContains(t, err, "missing required columns: purchase_date")
}

func TestAssetImportServiceHelperErrors(t *testing.T) {
	ctx := context.Background()
	accountCodeRow := assetImportTestRow(map[string]string{"asset_account_code": "FA"})

	svc := &Service{}
	_, err := svc.assetImportAccountIDsByCode(ctx, "test_schema", "tenant-1", []assetImportRow{accountCodeRow})
	require.ErrorContains(t, err, "accounting service is required")

	ledger := newFakeAssetAccountingPoster()
	ledger.listErr = assert.AnError
	svc.ledger = ledger
	_, err = svc.assetImportAccountIDsByCode(ctx, "test_schema", "tenant-1", []assetImportRow{accountCodeRow})
	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "list accounts for asset import")

	ledger = newFakeAssetAccountingPoster()
	ledger.accounts["blank-code"] = &accounting.Account{ID: "blank-code", Code: " "}
	svc.ledger = ledger
	accountIDsByCode, err := svc.assetImportAccountIDsByCode(ctx, "test_schema", "tenant-1", []assetImportRow{accountCodeRow})
	require.NoError(t, err)
	assert.NotContains(t, accountIDsByCode, "")
	assert.Equal(t, "fixed-assets", accountIDsByCode["fa"])

	supplierRow := assetImportTestRow(map[string]string{"supplier_code": "SUP-001"})
	svc = &Service{}
	_, err = svc.assetImportSupplierLookup(ctx, "test_schema", "tenant-1", []assetImportRow{supplierRow})
	require.ErrorContains(t, err, "contact service is required")

	svc.contacts = fakeAssetContactLister{err: assert.AnError}
	_, err = svc.assetImportSupplierLookup(ctx, "test_schema", "tenant-1", []assetImportRow{supplierRow})
	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "list contacts for asset import")
}

func TestService_ImportAssetsCSVReportsMissingAccountCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	ts.svc.ledger = newFakeAssetAccountingPoster()

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost,asset_account_code\nLEG-001,Laptop,2025-01-10,1200.00,MISSING\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.AssetsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `account code "MISSING" was not found for asset_account_code`)
}

func TestService_ImportAssetsCSVResolvesSupplierCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	supplierID := "11111111-1111-1111-1111-111111111111"
	ts.svc.contacts = fakeAssetContactLister{
		contacts: []contacts.Contact{
			{ID: supplierID, TenantID: "tenant-1", Name: "Supplier One", Code: "SUP-001", ContactType: contacts.ContactTypeSupplier},
		},
	}

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code\nLEG-001,Laptop,2025-01-10,1200.00,SUP-001\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.AssetsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)
	require.Len(t, ts.repo.Assets, 1)
	var imported *FixedAsset
	for _, asset := range ts.repo.Assets {
		imported = asset
	}
	require.NotNil(t, imported)
	require.NotNil(t, imported.SupplierID)
	assert.Equal(t, supplierID, *imported.SupplierID)
}

func TestService_ImportAssetsCSVReportsMissingSupplierCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	ts.svc.contacts = fakeAssetContactLister{
		contacts: []contacts.Contact{
			{ID: "11111111-1111-1111-1111-111111111111", TenantID: "tenant-1", Name: "Supplier One", Code: "SUP-001", ContactType: contacts.ContactTypeSupplier},
		},
	}

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_code\nLEG-001,Laptop,2025-01-10,1200.00,SUP-404\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.AssetsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `supplier_code "SUP-404" was not found`)
}

func TestService_ImportAssetsCSVResolvesSupplierIdentityFields(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	supplierID := "11111111-1111-1111-1111-111111111111"
	ts.svc.contacts = fakeAssetContactLister{
		contacts: []contacts.Contact{
			{
				ID:        supplierID,
				TenantID:  "tenant-1",
				Name:      "Supplier One",
				RegCode:   "12345678",
				VATNumber: "EE12345678",
				Email:     "billing@supplier.example",
			},
		},
	}

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_email,supplier_name\nLEG-001,Laptop,2025-01-10,1200.00,billing@supplier.example,Supplier One\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.AssetsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)
	require.Len(t, ts.repo.Assets, 1)
	var imported *FixedAsset
	for _, asset := range ts.repo.Assets {
		imported = asset
	}
	require.NotNil(t, imported)
	require.NotNil(t, imported.SupplierID)
	assert.Equal(t, supplierID, *imported.SupplierID)
}

func TestService_ImportAssetsCSVReportsAmbiguousSupplierName(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	ts.svc.contacts = fakeAssetContactLister{
		contacts: []contacts.Contact{
			{ID: "11111111-1111-1111-1111-111111111111", TenantID: "tenant-1", Name: "Supplier One", ContactType: contacts.ContactTypeSupplier},
			{ID: "22222222-2222-2222-2222-222222222222", TenantID: "tenant-1", Name: " supplier one ", ContactType: contacts.ContactTypeSupplier},
		},
	}

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost,supplier_name\nLEG-001,Laptop,2025-01-10,1200.00,Supplier One\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.AssetsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `supplier_name "Supplier One" matched multiple contacts`)
}

func TestService_ImportAssetsCSVResolvesInvoiceNumberAliases(t *testing.T) {
	invoiceID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name   string
		header string
	}{
		{name: "invoice_number", header: "invoice_number"},
		{name: "invoice_no", header: "invoice_no"},
		{name: "smartaccounts_purchase_invoice_no", header: "purchase_invoice_no"},
		{name: "smartaccounts_document_no", header: "document_no"},
		{name: "merit_arve_nr", header: "arve_nr"},
		{name: "merit_ostuarve_nr", header: "ostuarve_nr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestService()
			ts.svc.invoicing = &fakeAssetInvoiceResolver{invoiceIDsByNumber: map[string]string{"BILL-1": invoiceID}}

			result, err := ts.svc.ImportAssetsCSV(context.Background(), "tenant-1", "test_schema", &ImportAssetsRequest{
				CSVContent: "asset_number,name,purchase_date,purchase_cost," + tt.header + "\n" +
					"LEG-001,Laptop,2025-01-10,1200.00,BILL-1\n",
			})

			require.NoError(t, err)
			assert.Equal(t, 1, result.RowsProcessed)
			assert.Equal(t, 1, result.AssetsCreated)
			assert.Zero(t, result.RowsSkipped)
			assert.Empty(t, result.Errors)
			require.Len(t, ts.repo.Assets, 1)
			for _, asset := range ts.repo.Assets {
				require.NotNil(t, asset.InvoiceID)
				assert.Equal(t, invoiceID, *asset.InvoiceID)
			}
		})
	}
}

func TestService_ImportAssetsCSVInvoiceIDWinsOverInvoiceNumber(t *testing.T) {
	ts := newTestService()
	explicitInvoiceID := "11111111-1111-4111-8111-111111111111"
	resolvedInvoiceID := "22222222-2222-4222-8222-222222222222"
	resolver := &fakeAssetInvoiceResolver{invoiceIDsByNumber: map[string]string{"BILL-1": resolvedInvoiceID}}
	ts.svc.invoicing = resolver

	result, err := ts.svc.ImportAssetsCSV(context.Background(), "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost,invoice_id,invoice_number\n" +
			"LEG-001,Laptop,2025-01-10,1200.00," + explicitInvoiceID + ",BILL-1\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.AssetsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)
	assert.Empty(t, resolver.calls)
	require.Len(t, ts.repo.Assets, 1)
	for _, asset := range ts.repo.Assets {
		require.NotNil(t, asset.InvoiceID)
		assert.Equal(t, explicitInvoiceID, *asset.InvoiceID)
	}
}

func TestService_ImportAssetsCSVReportsInvoiceNumberResolutionErrors(t *testing.T) {
	tests := []struct {
		name       string
		resolver   assetInvoiceResolver
		wantErrMsg string
	}{
		{
			name:       "missing",
			resolver:   &fakeAssetInvoiceResolver{},
			wantErrMsg: `resolve invoice_number "BILL-404": invoice not found`,
		},
		{
			name: "ambiguous",
			resolver: &fakeAssetInvoiceResolver{errorsByNumber: map[string]error{
				"BILL-404": fmt.Errorf(`invoice_number "BILL-404" matched multiple invoices`),
			}},
			wantErrMsg: `resolve invoice_number "BILL-404": invoice_number "BILL-404" matched multiple invoices`,
		},
		{
			name:       "no resolver",
			resolver:   nil,
			wantErrMsg: `invoice_number "BILL-404" cannot be resolved without invoicing service`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestService()
			ts.svc.invoicing = tt.resolver

			result, err := ts.svc.ImportAssetsCSV(context.Background(), "tenant-1", "test_schema", &ImportAssetsRequest{
				CSVContent: "asset_number,name,purchase_date,purchase_cost,invoice_number\n" +
					"LEG-001,Laptop,2025-01-10,1200.00,BILL-404\n",
			})

			require.NoError(t, err)
			assert.Equal(t, 1, result.RowsProcessed)
			assert.Zero(t, result.AssetsCreated)
			assert.Equal(t, 1, result.RowsSkipped)
			require.Len(t, result.Errors, 1)
			assert.Equal(t, 2, result.Errors[0].Row)
			assert.Contains(t, result.Errors[0].Message, tt.wantErrMsg)
			assert.Empty(t, ts.repo.Assets)
		})
	}
}

func TestService_ImportAssetsCSVReportsInvalidUUIDFields(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "name,purchase_date,purchase_cost,category_id,asset_account_id,supplier_id,invoice_id\n" +
			"Bad category,2025-01-10,1200.00,legacy-category,,,\n" +
			"Bad account,2025-01-10,1200.00,,legacy-account,,\n" +
			"Bad supplier,2025-01-10,1200.00,,,legacy-supplier,\n" +
			"Bad invoice,2025-01-10,1200.00,,,,legacy-invoice\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 4, result.RowsProcessed)
	assert.Zero(t, result.AssetsCreated)
	assert.Equal(t, 4, result.RowsSkipped)
	require.Len(t, result.Errors, 4)
	assert.Contains(t, result.Errors[0].Message, "category_id must be a valid UUID")
	assert.Contains(t, result.Errors[1].Message, "asset_account_id must be a valid UUID")
	assert.Contains(t, result.Errors[2].Message, "supplier_id must be a valid UUID")
	assert.Contains(t, result.Errors[3].Message, "invoice_id must be a valid UUID")
}

func TestService_ImportAssetsCSV_DuplicateAssetNumber(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Assets["existing"] = &FixedAsset{
		ID:          "existing",
		TenantID:    "tenant-1",
		AssetNumber: "LEG-001",
		Name:        "Existing laptop",
	}

	result, err := ts.svc.ImportAssetsCSV(ctx, "tenant-1", "test_schema", &ImportAssetsRequest{
		CSVContent: "asset_number,name,purchase_date,purchase_cost\nLEG-001,Laptop,2025-01-10,1200.00\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 0, result.AssetsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "duplicate asset_number")
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

func TestService_UpdatePreservesCategoryAndAccountsWhenOmitted(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	categoryID := "cat-1"
	assetAccountID := "asset-account"
	depreciationExpenseAccountID := "depreciation-expense"
	accumulatedDepreciationAccountID := "accumulated-depreciation"
	ts.repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		Name:                          "Laptop",
		CategoryID:                    &categoryID,
		Status:                        AssetStatusActive,
		PurchaseDate:                  time.Now(),
		PurchaseCost:                  decimal.NewFromInt(1200),
		UsefulLifeMonths:              36,
		ResidualValue:                 decimal.NewFromInt(100),
		DepreciationMethod:            DepreciationStraightLine,
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
	}

	asset, err := ts.svc.Update(ctx, "tenant-1", "test_schema", "a1", &UpdateAssetRequest{
		Name: "Updated laptop",
	})

	require.NoError(t, err)
	require.NotNil(t, asset.CategoryID)
	assert.Equal(t, categoryID, *asset.CategoryID)
	require.NotNil(t, asset.AssetAccountID)
	assert.Equal(t, assetAccountID, *asset.AssetAccountID)
	require.NotNil(t, asset.DepreciationExpenseAccountID)
	assert.Equal(t, depreciationExpenseAccountID, *asset.DepreciationExpenseAccountID)
	require.NotNil(t, asset.AccumulatedDepreciationAcctID)
	assert.Equal(t, accumulatedDepreciationAccountID, *asset.AccumulatedDepreciationAcctID)
}

func TestService_UpdateTrimsAndClearsExplicitAccountIDs(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	assetAccountID := "old-asset-account"
	depreciationExpenseAccountID := "old-depreciation-expense"
	accumulatedDepreciationAccountID := "old-accumulated-depreciation"
	ts.repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		Name:                          "Laptop",
		Status:                        AssetStatusActive,
		PurchaseDate:                  time.Now(),
		PurchaseCost:                  decimal.NewFromInt(1200),
		UsefulLifeMonths:              36,
		ResidualValue:                 decimal.NewFromInt(100),
		DepreciationMethod:            DepreciationStraightLine,
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
	}

	blankAssetAccountID := " \t "
	newDepreciationExpenseAccountID := " depreciation-expense "
	newAccumulatedDepreciationAccountID := "\naccumulated-depreciation\t"
	asset, err := ts.svc.Update(ctx, "tenant-1", "test_schema", "a1", &UpdateAssetRequest{
		AssetAccountID:                &blankAssetAccountID,
		DepreciationExpenseAccountID:  &newDepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &newAccumulatedDepreciationAccountID,
	})

	require.NoError(t, err)
	assert.Nil(t, asset.AssetAccountID)
	require.NotNil(t, asset.DepreciationExpenseAccountID)
	assert.Equal(t, "depreciation-expense", *asset.DepreciationExpenseAccountID)
	require.NotNil(t, asset.AccumulatedDepreciationAcctID)
	assert.Equal(t, "accumulated-depreciation", *asset.AccumulatedDepreciationAcctID)
}

func TestService_UpdateInheritsChangedCategoryDefaults(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	assetAccountID := "asset-account-2"
	depreciationExpenseAccountID := "depreciation-expense-2"
	accumulatedDepreciationAccountID := "accumulated-depreciation-2"
	ts.repo.Categories["cat-2"] = &AssetCategory{
		ID:                            "cat-2",
		TenantID:                      "tenant-1",
		Name:                          "Vehicles",
		DepreciationMethod:            DepreciationDecliningBalance,
		DefaultUsefulLifeMonths:       48,
		DefaultResidualValuePercent:   decimal.NewFromInt(20),
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
	}
	ts.repo.Assets["a1"] = &FixedAsset{
		ID:                 "a1",
		TenantID:           "tenant-1",
		Name:               "Truck",
		Status:             AssetStatusActive,
		PurchaseDate:       time.Now(),
		PurchaseCost:       decimal.NewFromInt(10000),
		UsefulLifeMonths:   60,
		ResidualValue:      decimal.NewFromInt(500),
		DepreciationMethod: DepreciationStraightLine,
	}
	categoryID := "cat-2"

	asset, err := ts.svc.Update(ctx, "tenant-1", "test_schema", "a1", &UpdateAssetRequest{
		CategoryID: &categoryID,
	})

	require.NoError(t, err)
	require.NotNil(t, asset.CategoryID)
	assert.Equal(t, categoryID, *asset.CategoryID)
	assert.Equal(t, DepreciationDecliningBalance, asset.DepreciationMethod)
	assert.Equal(t, 48, asset.UsefulLifeMonths)
	assert.True(t, asset.ResidualValue.Equal(decimal.NewFromInt(2000)))
	require.NotNil(t, asset.AssetAccountID)
	assert.Equal(t, assetAccountID, *asset.AssetAccountID)
	require.NotNil(t, asset.DepreciationExpenseAccountID)
	assert.Equal(t, depreciationExpenseAccountID, *asset.DepreciationExpenseAccountID)
	require.NotNil(t, asset.AccumulatedDepreciationAcctID)
	assert.Equal(t, accumulatedDepreciationAccountID, *asset.AccumulatedDepreciationAcctID)
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

func TestAssetJournalDescriptionAndReferenceFallbacks(t *testing.T) {
	tests := []struct {
		name                 string
		asset                *FixedAsset
		entry                *DepreciationEntry
		disposalDate         time.Time
		wantDepreciationDesc string
		wantDepreciationRef  string
		wantDisposalDesc     string
		wantDisposalRef      string
	}{
		{
			name: "number and name",
			asset: &FixedAsset{
				ID:          "asset-1",
				AssetNumber: " FA-001 ",
				Name:        " Laptop ",
			},
			entry:                &DepreciationEntry{PeriodEnd: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)},
			disposalDate:         time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			wantDepreciationDesc: "Depreciation FA-001 - Laptop",
			wantDepreciationRef:  "FA-001-2026-05",
			wantDisposalDesc:     "Disposal FA-001 - Laptop",
			wantDisposalRef:      "FA-001-2026-06-01",
		},
		{
			name: "number only",
			asset: &FixedAsset{
				ID:          "asset-2",
				AssetNumber: " FA-002 ",
			},
			entry:                &DepreciationEntry{},
			disposalDate:         time.Time{},
			wantDepreciationDesc: "Depreciation FA-002",
			wantDepreciationRef:  "FA-002",
			wantDisposalDesc:     "Disposal FA-002",
			wantDisposalRef:      "FA-002",
		},
		{
			name: "name only",
			asset: &FixedAsset{
				ID:   "asset-3",
				Name: " Desk ",
			},
			entry:                &DepreciationEntry{PeriodEnd: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
			disposalDate:         time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
			wantDepreciationDesc: "Depreciation Desk",
			wantDepreciationRef:  "asset-3-2026-07",
			wantDisposalDesc:     "Disposal Desk",
			wantDisposalRef:      "asset-3-2026-08-02",
		},
		{
			name: "id fallback",
			asset: &FixedAsset{
				ID: "asset-4",
			},
			entry:                &DepreciationEntry{},
			disposalDate:         time.Time{},
			wantDepreciationDesc: "Depreciation asset asset-4",
			wantDepreciationRef:  "asset-4",
			wantDisposalDesc:     "Disposal asset asset-4",
			wantDisposalRef:      "asset-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantDepreciationDesc, depreciationJournalDescription(tt.asset))
			assert.Equal(t, tt.wantDepreciationRef, depreciationJournalReference(tt.asset, tt.entry))
			assert.Equal(t, tt.wantDisposalDesc, disposalJournalDescription(tt.asset))
			assert.Equal(t, tt.wantDisposalRef, disposalJournalReference(tt.asset, tt.disposalDate))
		})
	}

	assert.True(t, isValidDisposalMethod(DisposalSold))
	assert.False(t, isValidDisposalMethod(DisposalMethod("RECYCLED")))
}

func TestService_Dispose(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()
	disposalDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	assetAccountID := "fixed-assets"
	accumulatedAccountID := "accumulated-depreciation"
	proceedsAccountID := "cash"
	gainAccountID := "asset-disposal-gain"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		Name:                          "Desk",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(1000),
		AccumulatedDepreciation:       decimal.NewFromInt(950),
		BookValue:                     decimal.NewFromInt(50),
		AssetAccountID:                &assetAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	req := &DisposeAssetRequest{
		DisposalDate:              disposalDate,
		DisposalMethod:            DisposalSold,
		DisposalProceeds:          decimal.NewFromInt(100),
		DisposalNotes:             "Sold to company X",
		DisposalProceedsAccountID: &proceedsAccountID,
		DisposalGainLossAccountID: &gainAccountID,
		UserID:                    "user-1",
	}

	err := svc.Dispose(ctx, "tenant-1", "test_schema", "a1", req)
	require.NoError(t, err)

	asset, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.Equal(t, AssetStatusSold, asset.Status)
	require.NotNil(t, asset.DisposalDate)
	assert.Equal(t, disposalDate, *asset.DisposalDate)
	require.NotNil(t, asset.DisposalMethod)
	assert.Equal(t, DisposalSold, *asset.DisposalMethod)
	assert.True(t, asset.DisposalProceeds.Equal(decimal.NewFromInt(100)))
	assert.Equal(t, "Sold to company X", asset.DisposalNotes)
	require.NotNil(t, asset.DisposalJournalEntryID)
}

func TestService_DisposeCreatesAndPostsGainJournalWhenAccountsConfigured(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()
	disposalDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	assetAccountID := "fixed-assets"
	accumulatedAccountID := "accumulated-depreciation"
	proceedsAccountID := "cash"
	gainAccountID := "asset-disposal-gain"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		AssetNumber:                   "FA-00001",
		Name:                          "Equipment",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(1200),
		AccumulatedDepreciation:       decimal.NewFromInt(300),
		BookValue:                     decimal.NewFromInt(900),
		AssetAccountID:                &assetAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	req := &DisposeAssetRequest{
		DisposalDate:              disposalDate,
		DisposalMethod:            DisposalSold,
		DisposalProceeds:          decimal.NewFromInt(950),
		DisposalProceedsAccountID: &proceedsAccountID,
		DisposalGainLossAccountID: &gainAccountID,
		UserID:                    "user-1",
	}

	err := svc.Dispose(ctx, "tenant-1", "test_schema", "a1", req)
	require.NoError(t, err)

	asset, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.Equal(t, AssetStatusSold, asset.Status)
	require.NotNil(t, asset.DisposalJournalEntryID)
	assert.Equal(t, "je-1", *asset.DisposalJournalEntryID)
	assert.Equal(t, []string{"je-1"}, ledger.postedIDs)
	require.NotNil(t, ledger.createdRequest)
	assert.Equal(t, SourceTypeAssetDisposal, ledger.createdRequest.SourceType)
	require.NotNil(t, ledger.createdRequest.SourceID)
	assert.Equal(t, "a1", *ledger.createdRequest.SourceID)
	assert.Equal(t, disposalDate, ledger.createdRequest.EntryDate)
	assert.Equal(t, "FA-00001-2026-05-01", ledger.createdRequest.Reference)
	require.Len(t, ledger.createdRequest.Lines, 4)
	assert.Equal(t, accumulatedAccountID, ledger.createdRequest.Lines[0].AccountID)
	assert.True(t, ledger.createdRequest.Lines[0].DebitAmount.Equal(decimal.NewFromInt(300)))
	assert.Equal(t, proceedsAccountID, ledger.createdRequest.Lines[1].AccountID)
	assert.True(t, ledger.createdRequest.Lines[1].DebitAmount.Equal(decimal.NewFromInt(950)))
	assert.Equal(t, assetAccountID, ledger.createdRequest.Lines[2].AccountID)
	assert.True(t, ledger.createdRequest.Lines[2].CreditAmount.Equal(decimal.NewFromInt(1200)))
	assert.Equal(t, gainAccountID, ledger.createdRequest.Lines[3].AccountID)
	assert.True(t, ledger.createdRequest.Lines[3].CreditAmount.Equal(decimal.NewFromInt(50)))
}

func TestService_DisposeCreatesLossJournalWhenScrappedWithBookValue(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	assetAccountID := "fixed-assets"
	accumulatedAccountID := "accumulated-depreciation"
	lossAccountID := "asset-disposal-loss"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		AssetNumber:                   "FA-00002",
		Name:                          "Old equipment",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(1200),
		AccumulatedDepreciation:       decimal.NewFromInt(300),
		BookValue:                     decimal.NewFromInt(900),
		AssetAccountID:                &assetAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	err := svc.Dispose(ctx, "tenant-1", "test_schema", "a1", &DisposeAssetRequest{
		DisposalDate:              time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		DisposalMethod:            DisposalScrapped,
		DisposalGainLossAccountID: &lossAccountID,
		UserID:                    "user-1",
	})
	require.NoError(t, err)

	asset, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.Equal(t, AssetStatusDisposed, asset.Status)
	require.NotNil(t, asset.DisposalJournalEntryID)
	require.NotNil(t, ledger.createdRequest)
	require.Len(t, ledger.createdRequest.Lines, 3)
	assert.Equal(t, accumulatedAccountID, ledger.createdRequest.Lines[0].AccountID)
	assert.True(t, ledger.createdRequest.Lines[0].DebitAmount.Equal(decimal.NewFromInt(300)))
	assert.Equal(t, lossAccountID, ledger.createdRequest.Lines[1].AccountID)
	assert.True(t, ledger.createdRequest.Lines[1].DebitAmount.Equal(decimal.NewFromInt(900)))
	assert.Equal(t, assetAccountID, ledger.createdRequest.Lines[2].AccountID)
	assert.True(t, ledger.createdRequest.Lines[2].CreditAmount.Equal(decimal.NewFromInt(1200)))
}

func TestService_DisposeRejectsPartialDisposalAccountingConfiguration(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	assetAccountID := "fixed-assets"
	accumulatedAccountID := "accumulated-depreciation"
	proceedsAccountID := "cash"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		Name:                          "Equipment",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(1200),
		AccumulatedDepreciation:       decimal.NewFromInt(300),
		BookValue:                     decimal.NewFromInt(900),
		AssetAccountID:                &assetAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	err := svc.Dispose(ctx, "tenant-1", "test_schema", "a1", &DisposeAssetRequest{
		DisposalDate:              time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		DisposalMethod:            DisposalSold,
		DisposalProceeds:          decimal.NewFromInt(950),
		DisposalProceedsAccountID: &proceedsAccountID,
		UserID:                    "user-1",
	})
	require.ErrorIs(t, err, ErrAssetAccountingInvalid)
	assert.Empty(t, ledger.postedIDs)
}

func TestService_DisposeRejectsMissingDisposalAccountingConfiguration(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	repo.Assets["a1"] = &FixedAsset{
		ID:                      "a1",
		TenantID:                "tenant-1",
		Name:                    "Equipment",
		Status:                  AssetStatusActive,
		PurchaseCost:            decimal.NewFromInt(1200),
		AccumulatedDepreciation: decimal.NewFromInt(300),
		BookValue:               decimal.NewFromInt(900),
	}

	err := svc.Dispose(ctx, "tenant-1", "test_schema", "a1", &DisposeAssetRequest{
		DisposalDate:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		DisposalMethod: DisposalScrapped,
		UserID:         "user-1",
	})
	require.ErrorIs(t, err, ErrAssetAccountingInvalid)
	assert.Empty(t, ledger.postedIDs)
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
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	assetAccountID := "fixed-assets"
	accumulatedAccountID := "accumulated-depreciation"
	lossAccountID := "asset-disposal-loss"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		Name:                          "Old Computer",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(1000),
		AccumulatedDepreciation:       decimal.NewFromInt(900),
		BookValue:                     decimal.NewFromInt(100),
		AssetAccountID:                &assetAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	req := &DisposeAssetRequest{
		DisposalDate:              time.Now(),
		DisposalMethod:            DisposalScrapped,
		DisposalGainLossAccountID: &lossAccountID,
		UserID:                    "user-1",
	}

	err := svc.Dispose(ctx, "tenant-1", "test_schema", "a1", req)
	require.NoError(t, err)

	asset, _ := repo.GetByID(ctx, "test_schema", "tenant-1", "a1")
	assert.Equal(t, AssetStatusDisposed, asset.Status)
	require.NotNil(t, asset.DisposalJournalEntryID)
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
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	expenseAccountID := "depreciation-expense"
	accumulatedAccountID := "accumulated-depreciation"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		Name:                          "Equipment",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(12000),
		ResidualValue:                 decimal.NewFromInt(0),
		UsefulLifeMonths:              12,
		DepreciationMethod:            DepreciationStraightLine,
		AccumulatedDepreciation:       decimal.Zero,
		BookValue:                     decimal.NewFromInt(12000),
		DepreciationExpenseAccountID:  &expenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	now := time.Now()
	entry, err := svc.RecordDepreciation(ctx, "tenant-1", "test_schema", "a1", "user-1", now.AddDate(0, -1, 0), now)
	require.NoError(t, err)
	assert.True(t, entry.DepreciationAmount.Equal(decimal.NewFromInt(1000)))
	assert.True(t, entry.AccumulatedTotal.Equal(decimal.NewFromInt(1000)))
	assert.True(t, entry.BookValueAfter.Equal(decimal.NewFromInt(11000)))
	require.NotNil(t, entry.JournalEntryID)
}

func TestService_RecordDepreciationCreatesAndPostsJournalWhenAccountsConfigured(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	expenseAccountID := "depreciation-expense"
	accumulatedAccountID := "accumulated-depreciation"
	repo.Assets["a1"] = &FixedAsset{
		ID:                            "a1",
		TenantID:                      "tenant-1",
		AssetNumber:                   "FA-00001",
		Name:                          "Equipment",
		Status:                        AssetStatusActive,
		PurchaseCost:                  decimal.NewFromInt(12000),
		ResidualValue:                 decimal.NewFromInt(0),
		UsefulLifeMonths:              12,
		DepreciationMethod:            DepreciationStraightLine,
		AccumulatedDepreciation:       decimal.Zero,
		BookValue:                     decimal.NewFromInt(12000),
		DepreciationExpenseAccountID:  &expenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedAccountID,
	}

	periodStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	entry, err := svc.RecordDepreciation(ctx, "tenant-1", "test_schema", "a1", "user-1", periodStart, periodEnd)
	require.NoError(t, err)

	require.NotNil(t, entry.JournalEntryID)
	assert.Equal(t, "je-1", *entry.JournalEntryID)
	assert.Equal(t, []string{"je-1"}, ledger.postedIDs)
	require.NotNil(t, ledger.createdRequest)
	assert.Equal(t, SourceTypeAssetDepreciation, ledger.createdRequest.SourceType)
	require.NotNil(t, ledger.createdRequest.SourceID)
	assert.Equal(t, entry.ID, *ledger.createdRequest.SourceID)
	assert.Equal(t, "FA-00001-2026-04", ledger.createdRequest.Reference)
	assert.Equal(t, periodEnd, ledger.createdRequest.EntryDate)
	require.Len(t, ledger.createdRequest.Lines, 2)
	assert.Equal(t, expenseAccountID, ledger.createdRequest.Lines[0].AccountID)
	assert.True(t, ledger.createdRequest.Lines[0].DebitAmount.Equal(decimal.NewFromInt(1000)))
	assert.Equal(t, accumulatedAccountID, ledger.createdRequest.Lines[1].AccountID)
	assert.True(t, ledger.createdRequest.Lines[1].CreditAmount.Equal(decimal.NewFromInt(1000)))
}

func TestService_RecordDepreciationRejectsPartialAccountingConfiguration(t *testing.T) {
	repo := NewMockRepository()
	ledger := newFakeAssetAccountingPoster()
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	expenseAccountID := "depreciation-expense"
	repo.Assets["a1"] = &FixedAsset{
		ID:                           "a1",
		TenantID:                     "tenant-1",
		Name:                         "Equipment",
		Status:                       AssetStatusActive,
		PurchaseCost:                 decimal.NewFromInt(12000),
		ResidualValue:                decimal.NewFromInt(0),
		UsefulLifeMonths:             12,
		DepreciationMethod:           DepreciationStraightLine,
		AccumulatedDepreciation:      decimal.Zero,
		BookValue:                    decimal.NewFromInt(12000),
		DepreciationExpenseAccountID: &expenseAccountID,
	}

	now := time.Now()
	_, err := svc.RecordDepreciation(ctx, "tenant-1", "test_schema", "a1", "user-1", now.AddDate(0, -1, 0), now)
	require.ErrorIs(t, err, ErrAssetAccountingInvalid)
	assert.Empty(t, ledger.postedIDs)
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

type fakeAssetAccountingPoster struct {
	accounts       map[string]*accounting.Account
	createdRequest *accounting.CreateJournalEntryRequest
	postedIDs      []string
	listErr        error
}

func newFakeAssetAccountingPoster() *fakeAssetAccountingPoster {
	return &fakeAssetAccountingPoster{
		accounts: map[string]*accounting.Account{
			"depreciation-expense":     {ID: "depreciation-expense", Code: "DEP-EXP", AccountType: accounting.AccountTypeExpense},
			"accumulated-depreciation": {ID: "accumulated-depreciation", Code: "ACC-DEP", AccountType: accounting.AccountTypeAsset},
			"fixed-assets":             {ID: "fixed-assets", Code: "FA", AccountType: accounting.AccountTypeAsset},
			"cash":                     {ID: "cash", Code: "CASH", AccountType: accounting.AccountTypeAsset},
			"asset-disposal-gain":      {ID: "asset-disposal-gain", Code: "GAIN", AccountType: accounting.AccountTypeRevenue},
			"asset-disposal-loss":      {ID: "asset-disposal-loss", Code: "LOSS", AccountType: accounting.AccountTypeExpense},
		},
	}
}

func (f *fakeAssetAccountingPoster) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	accounts := make([]accounting.Account, 0, len(f.accounts))
	for _, account := range f.accounts {
		accounts = append(accounts, *account)
	}
	return accounts, nil
}

func (f *fakeAssetAccountingPoster) GetAccount(_ context.Context, _, _, accountID string) (*accounting.Account, error) {
	account, ok := f.accounts[accountID]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	return account, nil
}

func (f *fakeAssetAccountingPoster) CreateJournalEntry(_ context.Context, _, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	f.createdRequest = req
	return &accounting.JournalEntry{ID: "je-1", TenantID: tenantID, Status: accounting.StatusDraft}, nil
}

func (f *fakeAssetAccountingPoster) PostJournalEntry(_ context.Context, _, _, entryID, _ string) error {
	f.postedIDs = append(f.postedIDs, entryID)
	return nil
}

type fakeAssetContactLister struct {
	contacts []contacts.Contact
	err      error
}

type fakeAssetInvoiceResolver struct {
	invoiceIDsByNumber map[string]string
	errorsByNumber     map[string]error
	calls              []string
}

func (f fakeAssetContactLister) List(_ context.Context, _, _ string, _ *contacts.ContactFilter) ([]contacts.Contact, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contacts, nil
}

func (f *fakeAssetInvoiceResolver) ResolveInvoiceIDByNumber(_ context.Context, _, _ string, invoiceNumber string) (string, error) {
	trimmed := strings.TrimSpace(invoiceNumber)
	f.calls = append(f.calls, trimmed)
	if err, ok := f.errorsByNumber[trimmed]; ok {
		return "", err
	}
	if invoiceID, ok := f.invoiceIDsByNumber[trimmed]; ok {
		return invoiceID, nil
	}
	return "", fmt.Errorf("invoice not found")
}

func TestNewService(t *testing.T) {
	// Test that NewService creates a service with a repository.
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
