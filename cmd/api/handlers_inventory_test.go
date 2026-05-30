package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// Error definitions for inventory mock repository
var (
	errProductNotFound   = errors.New("product not found")
	errCategoryNotFound  = errors.New("category not found")
	errWarehouseNotFound = errors.New("warehouse not found")
)

// mockInventoryRepository implements inventory.Repository for testing
type mockInventoryRepository struct {
	products    map[string]*inventory.Product
	categories  map[string]*inventory.ProductCategory
	warehouses  map[string]*inventory.Warehouse
	stockLevels map[string]*inventory.StockLevel
	movements   map[string][]inventory.InventoryMovement
	productCode int

	createProductErr   error
	getProductErr      error
	listProductsErr    error
	updateProductErr   error
	deleteProductErr   error
	createCategoryErr  error
	getCategoryErr     error
	listCategoriesErr  error
	deleteCategoryErr  error
	createWarehouseErr error
	getWarehouseErr    error
	listWarehousesErr  error
	updateWarehouseErr error
	deleteWarehouseErr error
	getStockErr        error
	upsertStockErr     error
	createMovementErr  error
	listMovementsErr   error
	updateProductStock error
}

func newMockInventoryRepository() *mockInventoryRepository {
	return &mockInventoryRepository{
		products:    make(map[string]*inventory.Product),
		categories:  make(map[string]*inventory.ProductCategory),
		warehouses:  make(map[string]*inventory.Warehouse),
		stockLevels: make(map[string]*inventory.StockLevel),
		movements:   make(map[string][]inventory.InventoryMovement),
		productCode: 1,
	}
}

// Products
func (m *mockInventoryRepository) CreateProduct(ctx context.Context, schemaName string, product *inventory.Product) error {
	if m.createProductErr != nil {
		return m.createProductErr
	}
	m.products[product.ID] = product
	return nil
}

func (m *mockInventoryRepository) GetProductByID(ctx context.Context, schemaName, tenantID, productID string) (*inventory.Product, error) {
	if m.getProductErr != nil {
		return nil, m.getProductErr
	}
	if p, ok := m.products[productID]; ok && p.TenantID == tenantID {
		return p, nil
	}
	return nil, errProductNotFound
}

func (m *mockInventoryRepository) ListProducts(ctx context.Context, schemaName, tenantID string, filter *inventory.ProductFilter) ([]inventory.Product, error) {
	if m.listProductsErr != nil {
		return nil, m.listProductsErr
	}
	var result []inventory.Product
	for _, p := range m.products {
		if p.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.CategoryID != "" && p.CategoryID != filter.CategoryID {
				continue
			}
			if filter.Status == inventory.ProductStatusActive && !p.IsActive {
				continue
			}
		}
		result = append(result, *p)
	}
	return result, nil
}

func (m *mockInventoryRepository) UpdateProduct(ctx context.Context, schemaName string, product *inventory.Product) error {
	if m.updateProductErr != nil {
		return m.updateProductErr
	}
	m.products[product.ID] = product
	return nil
}

func (m *mockInventoryRepository) DeleteProduct(ctx context.Context, schemaName, tenantID, productID string) error {
	if m.deleteProductErr != nil {
		return m.deleteProductErr
	}
	if _, ok := m.products[productID]; !ok {
		return errProductNotFound
	}
	delete(m.products, productID)
	return nil
}

func (m *mockInventoryRepository) GenerateCode(ctx context.Context, schemaName, tenantID string) (string, error) {
	code := m.productCode
	m.productCode++
	return "PROD-" + string(rune('0'+code)), nil
}

// Categories
func (m *mockInventoryRepository) CreateCategory(ctx context.Context, schemaName string, category *inventory.ProductCategory) error {
	if m.createCategoryErr != nil {
		return m.createCategoryErr
	}
	m.categories[category.ID] = category
	return nil
}

func (m *mockInventoryRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*inventory.ProductCategory, error) {
	if m.getCategoryErr != nil {
		return nil, m.getCategoryErr
	}
	if c, ok := m.categories[categoryID]; ok && c.TenantID == tenantID {
		return c, nil
	}
	return nil, errCategoryNotFound
}

func (m *mockInventoryRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]inventory.ProductCategory, error) {
	if m.listCategoriesErr != nil {
		return nil, m.listCategoriesErr
	}
	var result []inventory.ProductCategory
	for _, c := range m.categories {
		if c.TenantID == tenantID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockInventoryRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	if m.deleteCategoryErr != nil {
		return m.deleteCategoryErr
	}
	if _, ok := m.categories[categoryID]; !ok {
		return errCategoryNotFound
	}
	delete(m.categories, categoryID)
	return nil
}

// Warehouses
func (m *mockInventoryRepository) CreateWarehouse(ctx context.Context, schemaName string, warehouse *inventory.Warehouse) error {
	if m.createWarehouseErr != nil {
		return m.createWarehouseErr
	}
	m.warehouses[warehouse.ID] = warehouse
	return nil
}

func (m *mockInventoryRepository) GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*inventory.Warehouse, error) {
	if m.getWarehouseErr != nil {
		return nil, m.getWarehouseErr
	}
	if w, ok := m.warehouses[warehouseID]; ok && w.TenantID == tenantID {
		return w, nil
	}
	return nil, errWarehouseNotFound
}

func (m *mockInventoryRepository) ListWarehouses(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]inventory.Warehouse, error) {
	if m.listWarehousesErr != nil {
		return nil, m.listWarehousesErr
	}
	var result []inventory.Warehouse
	for _, w := range m.warehouses {
		if w.TenantID == tenantID {
			if activeOnly && !w.IsActive {
				continue
			}
			result = append(result, *w)
		}
	}
	return result, nil
}

func (m *mockInventoryRepository) UpdateWarehouse(ctx context.Context, schemaName string, warehouse *inventory.Warehouse) error {
	if m.updateWarehouseErr != nil {
		return m.updateWarehouseErr
	}
	m.warehouses[warehouse.ID] = warehouse
	return nil
}

func (m *mockInventoryRepository) DeleteWarehouse(ctx context.Context, schemaName, tenantID, warehouseID string) error {
	if m.deleteWarehouseErr != nil {
		return m.deleteWarehouseErr
	}
	if _, ok := m.warehouses[warehouseID]; !ok {
		return errWarehouseNotFound
	}
	delete(m.warehouses, warehouseID)
	return nil
}

// Stock Levels
func (m *mockInventoryRepository) GetStockLevel(ctx context.Context, schemaName, tenantID, productID, warehouseID string) (*inventory.StockLevel, error) {
	if m.getStockErr != nil {
		return nil, m.getStockErr
	}
	key := productID + "-" + warehouseID
	if sl, ok := m.stockLevels[key]; ok {
		return sl, nil
	}
	return nil, nil
}

func (m *mockInventoryRepository) GetStockLevelsByProduct(ctx context.Context, schemaName, tenantID, productID string) ([]inventory.StockLevel, error) {
	if m.getStockErr != nil {
		return nil, m.getStockErr
	}
	var result []inventory.StockLevel
	for key, sl := range m.stockLevels {
		if sl.ProductID == productID {
			_ = key
			result = append(result, *sl)
		}
	}
	return result, nil
}

func (m *mockInventoryRepository) UpsertStockLevel(ctx context.Context, schemaName string, level *inventory.StockLevel) error {
	if m.upsertStockErr != nil {
		return m.upsertStockErr
	}
	key := level.ProductID + "-" + level.WarehouseID
	m.stockLevels[key] = level
	return nil
}

// Movements
func (m *mockInventoryRepository) CreateMovement(ctx context.Context, schemaName string, movement *inventory.InventoryMovement) error {
	if m.createMovementErr != nil {
		return m.createMovementErr
	}
	m.movements[movement.ProductID] = append(m.movements[movement.ProductID], *movement)
	return nil
}

func (m *mockInventoryRepository) ListMovements(ctx context.Context, schemaName, tenantID, productID string) ([]inventory.InventoryMovement, error) {
	if m.listMovementsErr != nil {
		return nil, m.listMovementsErr
	}
	return m.movements[productID], nil
}

func (m *mockInventoryRepository) UpdateProductStock(ctx context.Context, schemaName, tenantID, productID string, newStock decimal.Decimal) error {
	if m.updateProductStock != nil {
		return m.updateProductStock
	}
	if p, ok := m.products[productID]; ok && p.TenantID == tenantID {
		p.CurrentStock = newStock
		return nil
	}
	return errProductNotFound
}

func setupInventoryTestHandlers() (*Handlers, *mockInventoryRepository, *mockTenantRepository) {
	inventoryRepo := newMockInventoryRepository()
	inventorySvc := inventory.NewServiceWithRepository(inventoryRepo)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		inventoryService: inventorySvc,
		tenantService:    tenantSvc,
	}
	return h, inventoryRepo, tenantRepo
}

func TestListProducts(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:          "prod-1",
		TenantID:    "tenant-1",
		Code:        "PROD-001",
		Name:        "Product A",
		ProductType: inventory.ProductTypeGoods,
		IsActive:    true,
	}
	repo.products["prod-2"] = &inventory.Product{
		ID:          "prod-2",
		TenantID:    "tenant-1",
		Code:        "PROD-002",
		Name:        "Product B",
		ProductType: inventory.ProductTypeService,
		IsActive:    false,
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all products",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "list active only",
			query:      "?status=ACTIVE",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/products"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListProducts(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []inventory.Product
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestCreateProduct(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid product",
			body: map[string]interface{}{
				"name":         "Test Product",
				"product_type": "GOODS",
				"sales_price":  "100.00",
				"vat_rate":     "20",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"product_type": "GOODS",
				"sales_price":  "100.00",
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "name",
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, tenantRepo := setupInventoryTestHandlers()

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

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/products", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreateProduct(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}

			if tt.wantStatus == http.StatusCreated {
				var result inventory.Product
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, "Test Product", result.Name)
			}
		})
	}
}

func TestImportProducts(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	body := map[string]interface{}{
		"file_name":   "products.csv",
		"csv_content": "code,name,product_type,sales_price,purchase_price,track_inventory,status\nSKU-001,Widget,GOODS,15.00,10.50,true,ACTIVE\n",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/products/import", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ImportProducts(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result inventory.ImportProductsResult
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.ProductsCreated)
	assert.Equal(t, 0, result.RowsSkipped)

	require.Len(t, repo.products, 1)
	for _, product := range repo.products {
		assert.Equal(t, "SKU-001", product.Code)
		assert.Equal(t, "Widget", product.Name)
		assert.Equal(t, inventory.ProductTypeGoods, product.ProductType)
		assert.True(t, product.TrackInventory)
		assert.True(t, product.IsActive)
		assert.True(t, product.SalesPrice.Equal(decimal.RequireFromString("15.00")))
	}
}

func TestGetProduct(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:          "prod-1",
		TenantID:    "tenant-1",
		Code:        "PROD-001",
		Name:        "Product A",
		ProductType: inventory.ProductTypeGoods,
	}

	tests := []struct {
		name       string
		productID  string
		wantStatus int
	}{
		{
			name:       "existing product",
			productID:  "prod-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent product",
			productID:  "prod-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/products/"+tt.productID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "productID": tt.productID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetProduct(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestListWarehouses(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Main Warehouse",
		IsActive: true,
	}
	repo.warehouses["wh-2"] = &inventory.Warehouse{
		ID:       "wh-2",
		TenantID: "tenant-1",
		Name:     "Secondary Warehouse",
		IsActive: false,
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/warehouses", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ListWarehouses(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result []inventory.Warehouse
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestCreateWarehouse(t *testing.T) {
	h, _, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	body := map[string]interface{}{
		"code": "WH-001",
		"name": "Test Warehouse",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/warehouses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.CreateWarehouse(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var result inventory.Warehouse
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "Test Warehouse", result.Name)
}

func TestImportWarehouses(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	body := map[string]interface{}{
		"file_name":   "warehouses.csv",
		"csv_content": "code,name,address,is_default,status\nMAIN,Main Warehouse,Tallinn,true,ACTIVE\n",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/warehouses/import", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ImportWarehouses(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result inventory.ImportWarehousesResult
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.WarehousesCreated)
	assert.Equal(t, 0, result.RowsSkipped)

	require.Len(t, repo.warehouses, 1)
	for _, warehouse := range repo.warehouses {
		assert.Equal(t, "MAIN", warehouse.Code)
		assert.Equal(t, "Main Warehouse", warehouse.Name)
		assert.Equal(t, "Tallinn", warehouse.Address)
		assert.True(t, warehouse.IsDefault)
		assert.True(t, warehouse.IsActive)
	}
}

func TestAdjustStock(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:           "prod-1",
		TenantID:     "tenant-1",
		Name:         "Product A",
		CurrentStock: decimal.NewFromInt(100),
	}
	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Main Warehouse",
	}

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid adjustment",
			body: map[string]interface{}{
				"product_id":   "prod-1",
				"warehouse_id": "wh-1",
				"quantity":     "10",
				"reason":       "Stock count correction",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing product_id",
			body: map[string]interface{}{
				"warehouse_id": "wh-1",
				"quantity":     "10",
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    "product",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/adjust", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")
			req = req.WithContext(contextWithClaims(req.Context(), claims))

			rr := httptest.NewRecorder()
			h.AdjustStock(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}
		})
	}
}

func TestImportStockAdjustments(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:           "prod-1",
		TenantID:     "tenant-1",
		Code:         "SKU-001",
		Name:         "Widget",
		CurrentStock: decimal.Zero,
	}
	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main Warehouse",
	}

	body := map[string]interface{}{
		"file_name":   "stock.csv",
		"csv_content": "product_code,warehouse_code,quantity,unit_cost,reason\nSKU-001,MAIN,12,10.50,Opening stock\n",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/stock-import", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")
	req = req.WithContext(contextWithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	h.ImportStockAdjustments(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result inventory.ImportStockAdjustmentsResult
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.AdjustmentsImported)
	assert.Equal(t, 0, result.RowsSkipped)
	assert.True(t, repo.products["prod-1"].CurrentStock.Equal(decimal.NewFromInt(12)))
}

func TestReserveAndReleaseStock(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:           "prod-1",
		TenantID:     "tenant-1",
		Name:         "Product A",
		CurrentStock: decimal.NewFromInt(12),
	}
	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Main Warehouse",
	}
	repo.stockLevels["prod-1-wh-1"] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    "prod-1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.NewFromInt(12),
		ReservedQty:  decimal.NewFromInt(2),
		AvailableQty: decimal.NewFromInt(10),
	}

	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")

	reserveBody, _ := json.Marshal(map[string]interface{}{
		"product_id":   "prod-1",
		"warehouse_id": "wh-1",
		"quantity":     "3",
		"reason":       "Sales order allocation",
	})
	reserveReq := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/reserve", bytes.NewReader(reserveBody))
	reserveReq.Header.Set("Content-Type", "application/json")
	reserveReq = withURLParams(reserveReq, map[string]string{"tenantID": "tenant-1"})
	reserveReq = reserveReq.WithContext(contextWithClaims(reserveReq.Context(), claims))

	reserveRR := httptest.NewRecorder()
	h.ReserveStock(reserveRR, reserveReq)

	require.Equal(t, http.StatusOK, reserveRR.Code)
	var reservedLevel inventory.StockLevel
	require.NoError(t, json.Unmarshal(reserveRR.Body.Bytes(), &reservedLevel))
	assert.True(t, reservedLevel.ReservedQty.Equal(decimal.NewFromInt(5)))
	assert.True(t, reservedLevel.AvailableQty.Equal(decimal.NewFromInt(7)))

	releaseBody, _ := json.Marshal(map[string]interface{}{
		"product_id":   "prod-1",
		"warehouse_id": "wh-1",
		"quantity":     "2",
		"reason":       "Order canceled",
	})
	releaseReq := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/release", bytes.NewReader(releaseBody))
	releaseReq.Header.Set("Content-Type", "application/json")
	releaseReq = withURLParams(releaseReq, map[string]string{"tenantID": "tenant-1"})
	releaseReq = releaseReq.WithContext(contextWithClaims(releaseReq.Context(), claims))

	releaseRR := httptest.NewRecorder()
	h.ReleaseStock(releaseRR, releaseReq)

	require.Equal(t, http.StatusOK, releaseRR.Code)
	var releasedLevel inventory.StockLevel
	require.NoError(t, json.Unmarshal(releaseRR.Body.Bytes(), &releasedLevel))
	assert.True(t, releasedLevel.ReservedQty.Equal(decimal.NewFromInt(3)))
	assert.True(t, releasedLevel.AvailableQty.Equal(decimal.NewFromInt(9)))
}

func TestReleaseStockRejectsOverRelease(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:       "prod-1",
		TenantID: "tenant-1",
		Name:     "Product A",
	}
	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Main Warehouse",
	}
	repo.stockLevels["prod-1-wh-1"] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    "prod-1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.NewFromInt(5),
		ReservedQty:  decimal.NewFromInt(1),
		AvailableQty: decimal.NewFromInt(4),
	}

	body, _ := json.Marshal(map[string]interface{}{
		"product_id":   "prod-1",
		"warehouse_id": "wh-1",
		"quantity":     "2",
	})
	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/inventory/release", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")
	req = req.WithContext(contextWithClaims(req.Context(), claims))

	rr := httptest.NewRecorder()
	h.ReleaseStock(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "cannot release more than reserved stock")
}

func TestGetInventoryValuation(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:             "prod-1",
		TenantID:       "tenant-1",
		Code:           "SKU-001",
		Name:           "Widget",
		ProductType:    inventory.ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("10.50"),
		TrackInventory: true,
		IsActive:       true,
	}
	repo.products["prod-2"] = &inventory.Product{
		ID:             "prod-2",
		TenantID:       "tenant-1",
		Code:           "SRV-001",
		Name:           "Consulting",
		ProductType:    inventory.ProductTypeService,
		PurchasePrice:  decimal.RequireFromString("100.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main Warehouse",
	}
	repo.stockLevels["prod-1-wh-1"] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    "prod-1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.RequireFromString("12.00"),
		ReservedQty:  decimal.RequireFromString("2.00"),
		AvailableQty: decimal.RequireFromString("10.00"),
	}
	repo.movements["prod-1"] = []inventory.InventoryMovement{
		{
			ID:           "mov-1",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("8.00"),
			TotalCost:    decimal.RequireFromString("80.00"),
		},
		{
			ID:           "mov-2",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("12.00"),
			TotalCost:    decimal.RequireFromString("120.00"),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/valuation?warehouse_id=wh-1&method=weighted-average", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetInventoryValuation(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result inventory.InventoryValuationReport
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	require.Len(t, result.Lines, 1)
	assert.Equal(t, inventory.InventoryValuationMethodWeightedAverage, result.ValuationMethod)
	assert.Equal(t, "wh-1", result.WarehouseID)
	assert.Equal(t, "SKU-001", result.Lines[0].ProductCode)
	assert.Equal(t, "MAIN", result.Lines[0].WarehouseCode)
	assert.True(t, result.Lines[0].UnitCost.Equal(decimal.RequireFromString("10.00")))
	assert.True(t, result.TotalQuantity.Equal(decimal.RequireFromString("12.00")))
	assert.True(t, result.TotalValue.Equal(decimal.RequireFromString("120.00")))
}

func TestListProductCategories(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.categories["cat-1"] = &inventory.ProductCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Electronics",
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/categories", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ListProductCategories(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result []inventory.ProductCategory
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestImportProductCategories(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	body := map[string]interface{}{
		"file_name":   "categories.csv",
		"csv_content": "name,description\nParts,Spare parts\n",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/product-categories/import", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ImportProductCategories(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result inventory.ImportProductCategoriesResult
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.CategoriesCreated)
	assert.Equal(t, 0, result.RowsSkipped)

	require.Len(t, repo.categories, 1)
	for _, category := range repo.categories {
		assert.Equal(t, "Parts", category.Name)
		assert.Equal(t, "Spare parts", category.Description)
	}
}
