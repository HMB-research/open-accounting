package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// Error definitions for assets mock repository
var (
	errAssetNotFound         = errors.New("asset not found")
	errAssetCategoryNotFound = errors.New("category not found")
)

// mockAssetsRepository implements assets.Repository for testing
type mockAssetsRepository struct {
	assets              map[string]*assets.FixedAsset
	categories          map[string]*assets.AssetCategory
	depreciationEntries map[string][]assets.DepreciationEntry
	assetNumber         int

	createErr         error
	getErr            error
	listErr           error
	updateErr         error
	deleteErr         error
	createCategoryErr error
	getCategoryErr    error
	listCategoriesErr error
	deleteCategoryErr error
	depreciationErr   error
}

func newMockAssetsRepository() *mockAssetsRepository {
	return &mockAssetsRepository{
		assets:              make(map[string]*assets.FixedAsset),
		categories:          make(map[string]*assets.AssetCategory),
		depreciationEntries: make(map[string][]assets.DepreciationEntry),
		assetNumber:         1,
	}
}

// Categories
func (m *mockAssetsRepository) CreateCategory(ctx context.Context, schemaName string, cat *assets.AssetCategory) error {
	if m.createCategoryErr != nil {
		return m.createCategoryErr
	}
	m.categories[cat.ID] = cat
	return nil
}

func (m *mockAssetsRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*assets.AssetCategory, error) {
	if m.getCategoryErr != nil {
		return nil, m.getCategoryErr
	}
	if c, ok := m.categories[categoryID]; ok && c.TenantID == tenantID {
		return c, nil
	}
	return nil, errAssetCategoryNotFound
}

func (m *mockAssetsRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]assets.AssetCategory, error) {
	if m.listCategoriesErr != nil {
		return nil, m.listCategoriesErr
	}
	var result []assets.AssetCategory
	for _, c := range m.categories {
		if c.TenantID == tenantID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockAssetsRepository) UpdateCategory(ctx context.Context, schemaName string, cat *assets.AssetCategory) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.categories[cat.ID] = cat
	return nil
}

func (m *mockAssetsRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	if m.deleteCategoryErr != nil {
		return m.deleteCategoryErr
	}
	if _, ok := m.categories[categoryID]; !ok {
		return errAssetCategoryNotFound
	}
	delete(m.categories, categoryID)
	return nil
}

// Assets
func (m *mockAssetsRepository) Create(ctx context.Context, schemaName string, asset *assets.FixedAsset) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.assets[asset.ID] = asset
	return nil
}

func (m *mockAssetsRepository) GetByID(ctx context.Context, schemaName, tenantID, assetID string) (*assets.FixedAsset, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if a, ok := m.assets[assetID]; ok && a.TenantID == tenantID {
		return a, nil
	}
	return nil, errAssetNotFound
}

func (m *mockAssetsRepository) List(ctx context.Context, schemaName, tenantID string, filter *assets.AssetFilter) ([]assets.FixedAsset, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []assets.FixedAsset
	for _, a := range m.assets {
		if a.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.Status != "" && a.Status != filter.Status {
				continue
			}
			if filter.CategoryID != "" && (a.CategoryID == nil || *a.CategoryID != filter.CategoryID) {
				continue
			}
		}
		result = append(result, *a)
	}
	return result, nil
}

func (m *mockAssetsRepository) Update(ctx context.Context, schemaName string, asset *assets.FixedAsset) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.assets[asset.ID] = asset
	return nil
}

func (m *mockAssetsRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, assetID string, status assets.AssetStatus) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if a, ok := m.assets[assetID]; ok && a.TenantID == tenantID {
		a.Status = status
		return nil
	}
	return errAssetNotFound
}

func (m *mockAssetsRepository) Delete(ctx context.Context, schemaName, tenantID, assetID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.assets[assetID]; !ok {
		return errAssetNotFound
	}
	delete(m.assets, assetID)
	return nil
}

func (m *mockAssetsRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	num := m.assetNumber
	m.assetNumber++
	return "FA-" + string(rune('0'+num)), nil
}

// Depreciation
func (m *mockAssetsRepository) CreateDepreciationEntry(ctx context.Context, schemaName string, entry *assets.DepreciationEntry) error {
	if m.depreciationErr != nil {
		return m.depreciationErr
	}
	m.depreciationEntries[entry.AssetID] = append(m.depreciationEntries[entry.AssetID], *entry)
	return nil
}

func (m *mockAssetsRepository) ListDepreciationEntries(ctx context.Context, schemaName, tenantID, assetID string) ([]assets.DepreciationEntry, error) {
	if m.depreciationErr != nil {
		return nil, m.depreciationErr
	}
	return m.depreciationEntries[assetID], nil
}

func (m *mockAssetsRepository) UpdateAssetDepreciation(ctx context.Context, schemaName string, asset *assets.FixedAsset) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if existing, ok := m.assets[asset.ID]; ok {
		existing.AccumulatedDepreciation = asset.AccumulatedDepreciation
		existing.BookValue = asset.BookValue
		existing.LastDepreciationDate = asset.LastDepreciationDate
		return nil
	}
	return errAssetNotFound
}

func setupAssetsTestHandlers() (*Handlers, *mockAssetsRepository, *mockTenantRepository) {
	assetsRepo := newMockAssetsRepository()
	assetsSvc := assets.NewServiceWithRepository(assetsRepo)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		assetsService: assetsSvc,
		tenantService: tenantSvc,
	}
	return h, assetsRepo, tenantRepo
}

func TestListAssetCategories(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.categories["cat-1"] = &assets.AssetCategory{
		ID:                 "cat-1",
		TenantID:           "tenant-1",
		Name:               "Computers",
		DepreciationMethod: assets.DepreciationStraightLine,
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/assets/categories", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ListAssetCategories(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result []assets.AssetCategory
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Computers", result[0].Name)
}

func TestCreateAssetCategory(t *testing.T) {
	h, _, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	body := map[string]interface{}{
		"name":                       "Vehicles",
		"depreciation_method":        "STRAIGHT_LINE",
		"default_useful_life_months": 60,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/assets/categories", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.CreateAssetCategory(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var result assets.AssetCategory
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "Vehicles", result.Name)
}

func TestGetAssetCategory(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.categories["cat-1"] = &assets.AssetCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Computers",
	}

	tests := []struct {
		name       string
		categoryID string
		wantStatus int
	}{
		{
			name:       "existing category",
			categoryID: "cat-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent category",
			categoryID: "cat-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/assets/categories/"+tt.categoryID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "categoryID": tt.categoryID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetAssetCategory(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestListAssets(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	purchaseDate := time.Now().AddDate(-1, 0, 0)
	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:           "asset-1",
		TenantID:     "tenant-1",
		AssetNumber:  "FA-001",
		Name:         "Dell Laptop",
		Status:       assets.AssetStatusActive,
		PurchaseDate: purchaseDate,
		PurchaseCost: decimal.NewFromInt(1500),
	}
	repo.assets["asset-2"] = &assets.FixedAsset{
		ID:           "asset-2",
		TenantID:     "tenant-1",
		AssetNumber:  "FA-002",
		Name:         "Office Desk",
		Status:       assets.AssetStatusDisposed,
		PurchaseDate: purchaseDate,
		PurchaseCost: decimal.NewFromInt(500),
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all assets",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "filter by status",
			query:      "?status=ACTIVE",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/assets"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListAssets(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []assets.FixedAsset
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestCreateAsset(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid asset",
			body: map[string]interface{}{
				"name":                "New Laptop",
				"purchase_date":       time.Now().Format(time.RFC3339),
				"purchase_cost":       "2000.00",
				"useful_life_months":  36,
				"depreciation_method": "STRAIGHT_LINE",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, tenantRepo := setupAssetsTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("{invalid")
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/assets", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreateAsset(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusCreated {
				var result assets.FixedAsset
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, "New Laptop", result.Name)
			}
		})
	}
}

func TestGetAsset(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:           "asset-1",
		TenantID:     "tenant-1",
		AssetNumber:  "FA-001",
		Name:         "Dell Laptop",
		PurchaseDate: time.Now(),
		PurchaseCost: decimal.NewFromInt(1500),
	}

	tests := []struct {
		name       string
		assetID    string
		wantStatus int
	}{
		{
			name:       "existing asset",
			assetID:    "asset-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent asset",
			assetID:    "asset-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/assets/"+tt.assetID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": tt.assetID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetAsset(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestUpdateAsset(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:                 "asset-1",
		TenantID:           "tenant-1",
		AssetNumber:        "FA-001",
		Name:               "Dell Laptop",
		Status:             assets.AssetStatusDraft,
		PurchaseDate:       time.Now(),
		PurchaseCost:       decimal.NewFromInt(1500),
		UsefulLifeMonths:   36,
		ResidualValue:      decimal.NewFromInt(100),
		DepreciationMethod: assets.DepreciationStraightLine,
		BookValue:          decimal.NewFromInt(1500),
	}

	// UpdateAssetRequest needs to provide all required fields for validation
	body := map[string]interface{}{
		"name":               "Dell Laptop Pro",
		"location":           "Office A",
		"useful_life_months": 36,
		"residual_value":     "100.00",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/assets/asset-1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.UpdateAsset(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result assets.FixedAsset
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Dell Laptop Pro", result.Name)
}

func TestDeleteAsset(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:       "asset-1",
		TenantID: "tenant-1",
		Name:     "Dell Laptop",
		Status:   assets.AssetStatusDraft, // Only draft assets can be deleted
	}

	tests := []struct {
		name       string
		assetID    string
		wantStatus int
	}{
		{
			name:       "delete existing asset",
			assetID:    "asset-1",
			wantStatus: http.StatusNoContent, // Handler returns 204 on success
		},
		{
			name:       "delete non-existent asset",
			assetID:    "asset-999",
			wantStatus: http.StatusBadRequest, // Handler returns 400 on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/assets/"+tt.assetID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": tt.assetID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.DeleteAsset(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestActivateAsset(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:               "asset-1",
		TenantID:         "tenant-1",
		Name:             "Dell Laptop",
		Status:           assets.AssetStatusDraft,
		PurchaseDate:     time.Now(),
		PurchaseCost:     decimal.NewFromInt(1500),
		UsefulLifeMonths: 36,
		ResidualValue:    decimal.NewFromInt(100),
	}

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/activate", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ActivateAsset(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDisposeAsset(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:           "asset-1",
		TenantID:     "tenant-1",
		Name:         "Dell Laptop",
		Status:       assets.AssetStatusActive,
		PurchaseDate: time.Now().AddDate(-2, 0, 0),
		PurchaseCost: decimal.NewFromInt(1500),
	}

	body := map[string]interface{}{
		"disposal_date":     time.Now().Format(time.RFC3339),
		"disposal_method":   "SOLD",
		"disposal_proceeds": "500.00",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/dispose", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.DisposeAsset(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRecordDepreciation(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:                 "asset-1",
		TenantID:           "tenant-1",
		Name:               "Dell Laptop",
		Status:             assets.AssetStatusActive,
		PurchaseDate:       time.Now().AddDate(-1, 0, 0),
		PurchaseCost:       decimal.NewFromInt(1500),
		UsefulLifeMonths:   36,
		ResidualValue:      decimal.NewFromInt(150),
		DepreciationMethod: assets.DepreciationStraightLine,
		BookValue:          decimal.NewFromInt(1500),
	}

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/assets/asset-1/depreciate", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.RecordDepreciation(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code) // Handler returns 201
}

func TestGetDepreciationHistory(t *testing.T) {
	h, repo, tenantRepo := setupAssetsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.assets["asset-1"] = &assets.FixedAsset{
		ID:       "asset-1",
		TenantID: "tenant-1",
		Name:     "Dell Laptop",
	}

	repo.depreciationEntries["asset-1"] = []assets.DepreciationEntry{
		{
			ID:                 "dep-1",
			TenantID:           "tenant-1",
			AssetID:            "asset-1",
			DepreciationDate:   time.Now(),
			DepreciationAmount: decimal.NewFromFloat(37.50),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/assets/asset-1/depreciation", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "assetID": "asset-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetDepreciationHistory(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result []assets.DepreciationEntry
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
