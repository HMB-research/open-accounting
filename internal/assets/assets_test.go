package assets

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
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

func (f fakeAssetContactLister) List(_ context.Context, _, _ string, _ *contacts.ContactFilter) ([]contacts.Contact, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contacts, nil
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
