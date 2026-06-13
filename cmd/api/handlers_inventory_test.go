package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

const (
	apiInventoryStockProductID    = "11111111-1111-4111-8111-111111111111"
	apiInventoryStockWarehouseID  = "22222222-2222-4222-8222-222222222222"
	apiInventoryStockWarehouseID2 = "33333333-3333-4333-8333-333333333333"
)

func apiInventoryStockLevelKey(productID, warehouseID string) string {
	return productID + "-" + warehouseID
}

// mockInventoryRepository implements inventory.Repository for testing
type mockInventoryRepository struct {
	products        map[string]*inventory.Product
	categories      map[string]*inventory.ProductCategory
	warehouses      map[string]*inventory.Warehouse
	stockLevels     map[string]*inventory.StockLevel
	movements       map[string][]inventory.InventoryMovement
	lotReservations map[string]*inventory.InventoryLotReservation
	productCode     int

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
		products:        make(map[string]*inventory.Product),
		categories:      make(map[string]*inventory.ProductCategory),
		warehouses:      make(map[string]*inventory.Warehouse),
		stockLevels:     make(map[string]*inventory.StockLevel),
		movements:       make(map[string][]inventory.InventoryMovement),
		lotReservations: make(map[string]*inventory.InventoryLotReservation),
		productCode:     1,
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

func apiInventoryLotReservationKey(productID, warehouseID, lotNumber, serialNumber, expiryDate string) string {
	return strings.Join([]string{
		productID,
		warehouseID,
		strings.TrimSpace(lotNumber),
		strings.TrimSpace(serialNumber),
		strings.TrimSpace(expiryDate),
	}, "|")
}

func (m *mockInventoryRepository) ListLotReservations(ctx context.Context, schemaName, tenantID, productID, warehouseID string) ([]inventory.InventoryLotReservation, error) {
	var result []inventory.InventoryLotReservation
	for _, reservation := range m.lotReservations {
		if reservation.TenantID == tenantID &&
			reservation.ProductID == productID &&
			reservation.WarehouseID == warehouseID &&
			reservation.Quantity.GreaterThan(decimal.Zero) {
			result = append(result, *reservation)
		}
	}
	return result, nil
}

func (m *mockInventoryRepository) UpsertLotReservation(ctx context.Context, schemaName string, reservation *inventory.InventoryLotReservation) error {
	key := apiInventoryLotReservationKey(reservation.ProductID, reservation.WarehouseID, reservation.LotNumber, reservation.SerialNumber, reservation.ExpiryDate)
	existing := m.lotReservations[key]
	if existing == nil {
		copy := *reservation
		m.lotReservations[key] = &copy
		return nil
	}
	existing.Quantity = existing.Quantity.Add(reservation.Quantity)
	existing.Reason = reservation.Reason
	existing.UpdatedAt = reservation.UpdatedAt
	existing.CreatedBy = reservation.CreatedBy
	return nil
}

func (m *mockInventoryRepository) ReleaseLotReservation(ctx context.Context, schemaName, tenantID, productID, warehouseID, lotNumber, serialNumber, expiryDate string, quantity decimal.Decimal, reason, releasedBy string) (*inventory.InventoryLotReservation, error) {
	key := apiInventoryLotReservationKey(productID, warehouseID, lotNumber, serialNumber, expiryDate)
	reservation := m.lotReservations[key]
	if reservation == nil || reservation.TenantID != tenantID || reservation.Quantity.LessThan(quantity) {
		return nil, errors.New("tracked lot reservation not found")
	}
	reservation.Quantity = reservation.Quantity.Sub(quantity)
	reservation.Reason = reason
	reservation.UpdatedAt = time.Now()
	reservation.CreatedBy = releasedBy
	copy := *reservation
	return &copy, nil
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

func newInventoryJSONRequest(t *testing.T, method, path string, body interface{}, params map[string]string) *http.Request {
	t.Helper()

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, params)
	return req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))
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
		wantErr    string
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
		{
			name:       "invalid category filter",
			query:      "?category_id=legacy-category",
			wantStatus: http.StatusBadRequest,
			wantErr:    "category_id must be a valid UUID",
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
			} else if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
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
			name: "invalid category id",
			body: map[string]interface{}{
				"name":         "Invalid category",
				"product_type": "GOODS",
				"sales_price":  "100.00",
				"category_id":  "legacy-category",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "category_id must be a valid UUID",
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

func TestUpdateDeleteProductAndStockViews(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products["prod-1"] = &inventory.Product{
		ID:             "prod-1",
		TenantID:       "tenant-1",
		Code:           "PROD-001",
		Name:           "Old product",
		ProductType:    inventory.ProductTypeGoods,
		SalesPrice:     decimal.RequireFromString("10.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	repo.stockLevels["prod-1-wh-1"] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    "prod-1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.RequireFromString("7.00"),
		ReservedQty:  decimal.RequireFromString("2.00"),
		AvailableQty: decimal.RequireFromString("5.00"),
	}
	repo.movements["prod-1"] = []inventory.InventoryMovement{
		{
			ID:           "mov-1",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			WarehouseID:  "wh-1",
			MovementType: inventory.MovementTypeAdjustment,
			Quantity:     decimal.RequireFromString("7.00"),
		},
	}

	updateReq := newInventoryJSONRequest(t, http.MethodPut, "/tenants/tenant-1/products/prod-1", map[string]interface{}{
		"name":            "Updated product",
		"description":     "Updated description",
		"unit":            "pcs",
		"sales_price":     "12.50",
		"purchase_price":  "8.25",
		"track_inventory": true,
		"is_active":       true,
	}, map[string]string{"tenantID": "tenant-1", "productID": "prod-1"})
	updateRR := httptest.NewRecorder()
	h.UpdateProduct(updateRR, updateReq)

	require.Equal(t, http.StatusOK, updateRR.Code)
	var updated inventory.Product
	require.NoError(t, json.Unmarshal(updateRR.Body.Bytes(), &updated))
	assert.Equal(t, "Updated product", updated.Name)
	assert.Equal(t, "pcs", updated.Unit)
	assert.True(t, updated.SalesPrice.Equal(decimal.RequireFromString("12.50")))

	invalidUpdateReq := newInventoryJSONRequest(t, http.MethodPut, "/tenants/tenant-1/products/prod-1", map[string]interface{}{
		"name":        "Updated product",
		"sales_price": "12.50",
		"category_id": "legacy-category",
	}, map[string]string{"tenantID": "tenant-1", "productID": "prod-1"})
	invalidUpdateRR := httptest.NewRecorder()
	h.UpdateProduct(invalidUpdateRR, invalidUpdateReq)
	require.Equal(t, http.StatusBadRequest, invalidUpdateRR.Code)
	assert.Contains(t, invalidUpdateRR.Body.String(), "category_id must be a valid UUID")

	stockReq := newInventoryJSONRequest(t, http.MethodGet, "/tenants/tenant-1/products/prod-1/stock-levels", nil, map[string]string{
		"tenantID":  "tenant-1",
		"productID": "prod-1",
	})
	stockRR := httptest.NewRecorder()
	h.GetStockLevels(stockRR, stockReq)

	require.Equal(t, http.StatusOK, stockRR.Code)
	var levels []inventory.StockLevel
	require.NoError(t, json.Unmarshal(stockRR.Body.Bytes(), &levels))
	require.Len(t, levels, 1)
	assert.Equal(t, "wh-1", levels[0].WarehouseID)

	movementsReq := newInventoryJSONRequest(t, http.MethodGet, "/tenants/tenant-1/products/prod-1/movements", nil, map[string]string{
		"tenantID":  "tenant-1",
		"productID": "prod-1",
	})
	movementsRR := httptest.NewRecorder()
	h.GetInventoryMovements(movementsRR, movementsReq)

	require.Equal(t, http.StatusOK, movementsRR.Code)
	var movements []inventory.InventoryMovement
	require.NoError(t, json.Unmarshal(movementsRR.Body.Bytes(), &movements))
	require.Len(t, movements, 1)
	assert.Equal(t, inventory.MovementTypeAdjustment, movements[0].MovementType)

	deleteReq := newInventoryJSONRequest(t, http.MethodDelete, "/tenants/tenant-1/products/prod-1", nil, map[string]string{
		"tenantID":  "tenant-1",
		"productID": "prod-1",
	})
	deleteRR := httptest.NewRecorder()
	h.DeleteProduct(deleteRR, deleteReq)

	require.Equal(t, http.StatusOK, deleteRR.Code)
	assert.NotContains(t, repo.products, "prod-1")
	assert.Contains(t, deleteRR.Body.String(), "deleted")
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

func TestGetUpdateDeleteWarehouse(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:        "wh-1",
		TenantID:  "tenant-1",
		Code:      "MAIN",
		Name:      "Main Warehouse",
		Address:   "Old address",
		IsDefault: true,
		IsActive:  true,
	}

	getReq := newInventoryJSONRequest(t, http.MethodGet, "/tenants/tenant-1/warehouses/wh-1", nil, map[string]string{
		"tenantID":    "tenant-1",
		"warehouseID": "wh-1",
	})
	getRR := httptest.NewRecorder()
	h.GetWarehouse(getRR, getReq)

	require.Equal(t, http.StatusOK, getRR.Code)
	var got inventory.Warehouse
	require.NoError(t, json.Unmarshal(getRR.Body.Bytes(), &got))
	assert.Equal(t, "MAIN", got.Code)

	updateReq := newInventoryJSONRequest(t, http.MethodPut, "/tenants/tenant-1/warehouses/wh-1", map[string]interface{}{
		"name":       "Back room",
		"address":    "New address",
		"is_default": false,
		"is_active":  true,
	}, map[string]string{"tenantID": "tenant-1", "warehouseID": "wh-1"})
	updateRR := httptest.NewRecorder()
	h.UpdateWarehouse(updateRR, updateReq)

	require.Equal(t, http.StatusOK, updateRR.Code)
	var updated inventory.Warehouse
	require.NoError(t, json.Unmarshal(updateRR.Body.Bytes(), &updated))
	assert.Equal(t, "Back room", updated.Name)
	assert.Equal(t, "New address", updated.Address)
	assert.False(t, updated.IsDefault)

	deleteReq := newInventoryJSONRequest(t, http.MethodDelete, "/tenants/tenant-1/warehouses/wh-1", nil, map[string]string{
		"tenantID":    "tenant-1",
		"warehouseID": "wh-1",
	})
	deleteRR := httptest.NewRecorder()
	h.DeleteWarehouse(deleteRR, deleteReq)

	require.Equal(t, http.StatusOK, deleteRR.Code)
	assert.NotContains(t, repo.warehouses, "wh-1")
	assert.Contains(t, deleteRR.Body.String(), "deleted")
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

	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:           apiInventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Product A",
		CurrentStock: decimal.NewFromInt(100),
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
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
				"product_id":   apiInventoryStockProductID,
				"warehouse_id": apiInventoryStockWarehouseID,
				"quantity":     "10",
				"reason":       "Stock count correction",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing product_id",
			body: map[string]interface{}{
				"warehouse_id": apiInventoryStockWarehouseID,
				"quantity":     "10",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "product_id is required",
		},
		{
			name: "invalid product id",
			body: map[string]interface{}{
				"product_id":   "legacy-product",
				"warehouse_id": apiInventoryStockWarehouseID,
				"quantity":     "10",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "product_id must be a valid UUID",
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

func TestIssueStock(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()
	cogsAccountID := "44444444-4444-4444-8444-444444444444"
	inventoryAccountID := "55555555-5555-4555-8555-555555555555"

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}
	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:                 apiInventoryStockProductID,
		TenantID:           "tenant-1",
		Code:               "SKU-001",
		Name:               "Widget",
		ProductType:        inventory.ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("8.00"),
		CurrentStock:       decimal.RequireFromString("12.00"),
		TrackInventory:     true,
		InventoryAccountID: inventoryAccountID,
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main Warehouse",
	}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("12.00"),
		ReservedQty:  decimal.RequireFromString("2.00"),
		AvailableQty: decimal.RequireFromString("10.00"),
	}
	repo.movements[apiInventoryStockProductID] = []inventory.InventoryMovement{
		{
			ID:           "mov-lot-in",
			TenantID:     "tenant-1",
			ProductID:    apiInventoryStockProductID,
			WarehouseID:  apiInventoryStockWarehouseID,
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.RequireFromString("12.00"),
			UnitCost:     decimal.RequireFromString("8.25"),
			TotalCost:    decimal.RequireFromString("99.00"),
			LotNumber:    "LOT-2026-01",
			ExpiryDate:   "2027-01-31",
		},
	}

	body := map[string]interface{}{
		"product_id":                    apiInventoryStockProductID,
		"warehouse_id":                  apiInventoryStockWarehouseID,
		"quantity":                      "3",
		"costing_method":                "weighted-average",
		"lot_number":                    "LOT-2026-01",
		"expiry_date":                   "2027-01-31",
		"reference":                     "Invoice INV-001",
		"source_type":                   "SALES_INVOICE",
		"source_id":                     "66666666-6666-4666-8666-666666666666",
		"reason":                        "Shipped goods",
		"cost_of_goods_sold_account_id": cogsAccountID,
	}
	req := newInventoryJSONRequest(t, http.MethodPost, "/tenants/tenant-1/inventory/issue", body, map[string]string{"tenantID": "tenant-1"})

	rr := httptest.NewRecorder()
	h.IssueStock(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var result inventory.IssueStockResult
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, inventory.InventoryIssueCostingMethodWeightedAverage, result.CostingMethod)
	assert.True(t, result.TotalCost.Equal(decimal.RequireFromString("24.75")))
	require.Len(t, result.Movements, 1)
	assert.Equal(t, "LOT-2026-01", result.Movements[0].LotNumber)
	assert.Equal(t, "SALES_INVOICE", result.Movements[0].SourceType)
	require.NotNil(t, result.Accounting)
	require.Len(t, result.Accounting.Lines, 2)
	assert.Equal(t, cogsAccountID, result.Accounting.Lines[0].AccountID)
	assert.Equal(t, inventoryAccountID, result.Accounting.Lines[1].AccountID)
	level := repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.RequireFromString("9.00")))
	assert.True(t, level.AvailableQty.Equal(decimal.RequireFromString("7.00")))
}

func TestIssueStockUsesTenantCostingPolicyWhenOmitted(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()
	cogsAccountID := "44444444-4444-4444-8444-444444444444"
	inventoryAccountID := "55555555-5555-4555-8555-555555555555"

	settings := tenant.DefaultSettings()
	settings.InventoryIssueCostingMethod = tenant.InventoryIssueCostingMethodStandardCost
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
		Settings:   settings,
	}
	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:                 apiInventoryStockProductID,
		TenantID:           "tenant-1",
		Code:               "SKU-001",
		Name:               "Widget",
		ProductType:        inventory.ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("8.00"),
		CurrentStock:       decimal.RequireFromString("12.00"),
		TrackInventory:     true,
		InventoryAccountID: inventoryAccountID,
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main Warehouse",
	}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("12.00"),
		AvailableQty: decimal.RequireFromString("12.00"),
	}
	repo.movements[apiInventoryStockProductID] = []inventory.InventoryMovement{{
		ID:           "mov-lot-in",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		MovementType: inventory.MovementTypeIn,
		Quantity:     decimal.RequireFromString("12.00"),
		UnitCost:     decimal.RequireFromString("8.25"),
		TotalCost:    decimal.RequireFromString("99.00"),
		LotNumber:    "LOT-2026-01",
	}}

	body := map[string]interface{}{
		"product_id":                    apiInventoryStockProductID,
		"warehouse_id":                  apiInventoryStockWarehouseID,
		"quantity":                      "3",
		"lot_number":                    "LOT-2026-01",
		"cost_of_goods_sold_account_id": cogsAccountID,
	}
	req := newInventoryJSONRequest(t, http.MethodPost, "/tenants/tenant-1/inventory/issue", body, map[string]string{"tenantID": "tenant-1"})

	rr := httptest.NewRecorder()
	h.IssueStock(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "response body: %s", rr.Body.String())
	var result inventory.IssueStockResult
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, inventory.InventoryIssueCostingMethodStandardCost, result.CostingMethod)
	assert.True(t, result.TotalCost.Equal(decimal.RequireFromString("24.00")))
	require.NotNil(t, result.Accounting)
	require.Len(t, result.Accounting.Lines, 2)
	assert.True(t, result.Accounting.Lines[0].DebitAmount.Equal(decimal.RequireFromString("24.00")))
}

func TestImportStockAdjustments(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:           apiInventoryStockProductID,
		TenantID:     "tenant-1",
		Code:         "SKU-001",
		Name:         "Widget",
		CurrentStock: decimal.Zero,
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
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
	assert.True(t, repo.products[apiInventoryStockProductID].CurrentStock.Equal(decimal.NewFromInt(12)))
}

func TestReserveAndReleaseStock(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:           apiInventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Product A",
		CurrentStock: decimal.NewFromInt(12),
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main Warehouse",
	}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(12),
		ReservedQty:  decimal.NewFromInt(2),
		AvailableQty: decimal.NewFromInt(10),
	}
	repo.movements[apiInventoryStockProductID] = []inventory.InventoryMovement{
		{
			ID:           "mov-lot",
			TenantID:     "tenant-1",
			ProductID:    apiInventoryStockProductID,
			WarehouseID:  apiInventoryStockWarehouseID,
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.NewFromInt(12),
			LotNumber:    "LOT-2026-01",
			ExpiryDate:   "2027-01-31",
		},
	}

	claims := createTestClaims("user-1", "test@example.com", "tenant-1", "owner")

	reserveBody, _ := json.Marshal(map[string]interface{}{
		"product_id":   apiInventoryStockProductID,
		"warehouse_id": apiInventoryStockWarehouseID,
		"quantity":     "3",
		"lot_number":   "LOT-2026-01",
		"expiry_date":  "2027-01-31",
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
	lotReservation := repo.lotReservations[apiInventoryLotReservationKey(apiInventoryStockProductID, apiInventoryStockWarehouseID, "LOT-2026-01", "", "2027-01-31")]
	require.NotNil(t, lotReservation)
	assert.True(t, lotReservation.Quantity.Equal(decimal.NewFromInt(3)))

	releaseBody, _ := json.Marshal(map[string]interface{}{
		"product_id":   apiInventoryStockProductID,
		"warehouse_id": apiInventoryStockWarehouseID,
		"quantity":     "2",
		"lot_number":   "LOT-2026-01",
		"expiry_date":  "2027-01-31",
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
	assert.True(t, lotReservation.Quantity.Equal(decimal.NewFromInt(1)))
}

func TestReleaseStockRejectsOverRelease(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:       apiInventoryStockProductID,
		TenantID: "tenant-1",
		Name:     "Product A",
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main Warehouse",
	}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(5),
		ReservedQty:  decimal.NewFromInt(1),
		AvailableQty: decimal.NewFromInt(4),
	}

	body, _ := json.Marshal(map[string]interface{}{
		"product_id":   apiInventoryStockProductID,
		"warehouse_id": apiInventoryStockWarehouseID,
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

func TestTransferStock(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:             apiInventoryStockProductID,
		TenantID:       "tenant-1",
		Name:           "Product A",
		ProductType:    inventory.ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("7.00"),
		TrackInventory: true,
	}
	repo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{ID: apiInventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main"}
	repo.warehouses[apiInventoryStockWarehouseID2] = &inventory.Warehouse{ID: apiInventoryStockWarehouseID2, TenantID: "tenant-1", Name: "Overflow"}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(9),
		ReservedQty:  decimal.NewFromInt(1),
		AvailableQty: decimal.NewFromInt(8),
	}
	repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID2)] = &inventory.StockLevel{
		ID:           "stock-2",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID2,
		Quantity:     decimal.Zero,
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.Zero,
	}
	repo.movements[apiInventoryStockProductID] = []inventory.InventoryMovement{
		{
			ID:           "mov-lot-receipt",
			TenantID:     "tenant-1",
			ProductID:    apiInventoryStockProductID,
			WarehouseID:  apiInventoryStockWarehouseID,
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.NewFromInt(9),
			UnitCost:     decimal.RequireFromString("8.25"),
			TotalCost:    decimal.RequireFromString("74.25"),
			LotNumber:    "LOT-2026-01",
			SerialNumber: "SN-001",
			ExpiryDate:   "2027-01-31",
		},
	}

	body := map[string]interface{}{
		"product_id":        apiInventoryStockProductID,
		"from_warehouse_id": apiInventoryStockWarehouseID,
		"to_warehouse_id":   apiInventoryStockWarehouseID2,
		"quantity":          "4",
		"lot_number":        "LOT-2026-01",
		"serial_number":     "SN-001",
		"expiry_date":       "2027-01-31",
		"notes":             "rebalance stock",
	}
	req := newInventoryJSONRequest(t, http.MethodPost, "/tenants/tenant-1/inventory/transfer", body, map[string]string{"tenantID": "tenant-1"})

	rr := httptest.NewRecorder()
	h.TransferStock(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "transferred")
	assert.True(t, repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)].Quantity.Equal(decimal.NewFromInt(5)))
	assert.True(t, repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)].AvailableQty.Equal(decimal.NewFromInt(4)))
	assert.True(t, repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID2)].Quantity.Equal(decimal.NewFromInt(4)))
	assert.True(t, repo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID2)].AvailableQty.Equal(decimal.NewFromInt(4)))
	require.Len(t, repo.movements[apiInventoryStockProductID], 3)
	transferMovements := repo.movements[apiInventoryStockProductID][1:]
	assert.Equal(t, inventory.MovementTypeOut, transferMovements[0].MovementType)
	assert.Equal(t, inventory.MovementTypeIn, transferMovements[1].MovementType)
	for _, movement := range transferMovements {
		assert.Equal(t, "LOT-2026-01", movement.LotNumber)
		assert.Equal(t, "SN-001", movement.SerialNumber)
		assert.Equal(t, "2027-01-31", movement.ExpiryDate)
		assert.True(t, movement.UnitCost.Equal(decimal.RequireFromString("8.25")))
		assert.True(t, movement.TotalCost.Equal(decimal.RequireFromString("33.00")))
	}
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

func TestGetInventoryValuationUsesTenantPolicyWhenMethodOmitted(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	settings := tenant.DefaultSettings()
	settings.InventoryValuationMethod = tenant.InventoryValuationMethodFIFO
	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
		Settings:   settings,
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
		Quantity:     decimal.RequireFromString("5.00"),
		AvailableQty: decimal.RequireFromString("5.00"),
	}
	repo.movements["prod-1"] = []inventory.InventoryMovement{
		{
			ID:           "mov-1",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.RequireFromString("2.00"),
			UnitCost:     decimal.RequireFromString("8.00"),
			TotalCost:    decimal.RequireFromString("16.00"),
		},
		{
			ID:           "mov-2",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.RequireFromString("3.00"),
			UnitCost:     decimal.RequireFromString("12.00"),
			TotalCost:    decimal.RequireFromString("36.00"),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/valuation?warehouse_id=wh-1", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetInventoryValuation(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "response body: %s", rr.Body.String())
	var result inventory.InventoryValuationReport
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	assert.Equal(t, inventory.InventoryValuationMethodFIFO, result.ValuationMethod)
	require.Len(t, result.Lines, 1)
	assert.True(t, result.TotalValue.Equal(decimal.RequireFromString("52.00")))
}

func TestGetInventoryLotReport(t *testing.T) {
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
		PurchasePrice:  decimal.RequireFromString("6.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	repo.warehouses["wh-1"] = &inventory.Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main Warehouse",
	}
	repo.movements["prod-1"] = []inventory.InventoryMovement{
		{
			ID:           "mov-1",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			WarehouseID:  "wh-1",
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("5.00"),
			TotalCost:    decimal.RequireFromString("50.00"),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
			MovementDate: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:           "mov-2",
			TenantID:     "tenant-1",
			ProductID:    "prod-1",
			WarehouseID:  "wh-1",
			MovementType: inventory.MovementTypeOut,
			Quantity:     decimal.RequireFromString("3.00"),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
			MovementDate: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/lots?product_id=prod-1&warehouse_id=wh-1&include_empty=true", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetInventoryLotReport(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result inventory.InventoryLotReport
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
	require.Len(t, result.Lines, 1)
	assert.Equal(t, "tenant-1", result.TenantID)
	assert.Equal(t, "prod-1", result.ProductID)
	assert.Equal(t, "wh-1", result.WarehouseID)
	assert.True(t, result.IncludeEmpty)
	assert.Equal(t, "SKU-001", result.Lines[0].ProductCode)
	assert.Equal(t, "MAIN", result.Lines[0].WarehouseCode)
	assert.Equal(t, "LOT-A", result.Lines[0].LotNumber)
	assert.Equal(t, "2027-01-31", result.Lines[0].ExpiryDate)
	assert.True(t, result.Lines[0].Quantity.Equal(decimal.RequireFromString("7.00")))
	assert.True(t, result.Lines[0].UnitCost.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, result.Lines[0].InventoryValue.Equal(decimal.RequireFromString("35.00")))
	assert.True(t, result.TotalQuantity.Equal(decimal.RequireFromString("7.00")))
	assert.True(t, result.TotalValue.Equal(decimal.RequireFromString("35.00")))

	badBoolReq := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/inventory/lots?include_empty=maybe", nil)
	badBoolReq = withURLParams(badBoolReq, map[string]string{"tenantID": "tenant-1"})
	badBoolReq = badBoolReq.WithContext(contextWithClaims(badBoolReq.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	badBoolRR := httptest.NewRecorder()
	h.GetInventoryLotReport(badBoolRR, badBoolReq)

	assert.Equal(t, http.StatusBadRequest, badBoolRR.Code)
	assert.Contains(t, badBoolRR.Body.String(), "include_empty must be a boolean")
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

func TestProductCategoryCRUD(t *testing.T) {
	h, repo, tenantRepo := setupInventoryTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	invalidParentReq := newInventoryJSONRequest(t, http.MethodPost, "/tenants/tenant-1/product-categories", map[string]interface{}{
		"name":      "Invalid parent",
		"parent_id": "legacy-parent",
	}, map[string]string{"tenantID": "tenant-1"})
	invalidParentRR := httptest.NewRecorder()
	h.CreateProductCategory(invalidParentRR, invalidParentReq)
	require.Equal(t, http.StatusBadRequest, invalidParentRR.Code)
	assert.Contains(t, invalidParentRR.Body.String(), "parent_id must be a valid UUID")

	createReq := newInventoryJSONRequest(t, http.MethodPost, "/tenants/tenant-1/product-categories", map[string]interface{}{
		"name":        "Spare parts",
		"description": "Replacement inventory",
	}, map[string]string{"tenantID": "tenant-1"})
	createRR := httptest.NewRecorder()
	h.CreateProductCategory(createRR, createReq)

	require.Equal(t, http.StatusCreated, createRR.Code)
	var created inventory.ProductCategory
	require.NoError(t, json.Unmarshal(createRR.Body.Bytes(), &created))
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "Spare parts", created.Name)

	getReq := newInventoryJSONRequest(t, http.MethodGet, "/tenants/tenant-1/product-categories/"+created.ID, nil, map[string]string{
		"tenantID":   "tenant-1",
		"categoryID": created.ID,
	})
	getRR := httptest.NewRecorder()
	h.GetProductCategory(getRR, getReq)

	require.Equal(t, http.StatusOK, getRR.Code)
	var got inventory.ProductCategory
	require.NoError(t, json.Unmarshal(getRR.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.ID)

	deleteReq := newInventoryJSONRequest(t, http.MethodDelete, "/tenants/tenant-1/product-categories/"+created.ID, nil, map[string]string{
		"tenantID":   "tenant-1",
		"categoryID": created.ID,
	})
	deleteRR := httptest.NewRecorder()
	h.DeleteProductCategory(deleteRR, deleteReq)

	require.Equal(t, http.StatusOK, deleteRR.Code)
	assert.NotContains(t, repo.categories, created.ID)
	assert.Contains(t, deleteRR.Body.String(), "deleted")
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
