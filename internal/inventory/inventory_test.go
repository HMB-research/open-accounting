package inventory

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	mu              sync.RWMutex
	Products        map[string]*Product
	Categories      map[string]*ProductCategory
	Warehouses      map[string]*Warehouse
	StockLevels     map[string]*StockLevel // key: productID-warehouseID
	Movements       map[string][]InventoryMovement
	ProductCodeSeq  int
	ErrOnCreate     bool
	ErrOnGet        bool
	ErrOnUpdate     bool
	ErrOnDelete     bool
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		Products:       make(map[string]*Product),
		Categories:     make(map[string]*ProductCategory),
		Warehouses:     make(map[string]*Warehouse),
		StockLevels:    make(map[string]*StockLevel),
		Movements:      make(map[string][]InventoryMovement),
		ProductCodeSeq: 0,
	}
}

// Products
func (r *MockRepository) CreateProduct(ctx context.Context, schemaName string, product *Product) error {
	if r.ErrOnCreate {
		return fmt.Errorf("mock error on create")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Products[product.ID] = product
	return nil
}

func (r *MockRepository) GetProductByID(ctx context.Context, schemaName, tenantID, productID string) (*Product, error) {
	if r.ErrOnGet {
		return nil, fmt.Errorf("mock error on get")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, exists := r.Products[productID]
	if !exists || p.TenantID != tenantID {
		return nil, fmt.Errorf("product not found")
	}
	productCopy := *p
	return &productCopy, nil
}

func (r *MockRepository) ListProducts(ctx context.Context, schemaName, tenantID string, filter *ProductFilter) ([]Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Product
	for _, p := range r.Products {
		if p.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.ProductType != "" && p.ProductType != filter.ProductType {
				continue
			}
			if filter.CategoryID != "" && p.CategoryID != filter.CategoryID {
				continue
			}
			if filter.Status == ProductStatusActive && !p.IsActive {
				continue
			}
			if filter.Status == ProductStatusInactive && p.IsActive {
				continue
			}
			if filter.LowStock && !p.CurrentStock.LessThanOrEqual(p.ReorderPoint) {
				continue
			}
		}
		result = append(result, *p)
	}
	return result, nil
}

func (r *MockRepository) UpdateProduct(ctx context.Context, schemaName string, product *Product) error {
	if r.ErrOnUpdate {
		return fmt.Errorf("mock error on update")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Products[product.ID]; !exists {
		return fmt.Errorf("product not found")
	}
	r.Products[product.ID] = product
	return nil
}

func (r *MockRepository) DeleteProduct(ctx context.Context, schemaName, tenantID, productID string) error {
	if r.ErrOnDelete {
		return fmt.Errorf("mock error on delete")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, exists := r.Products[productID]
	if !exists || p.TenantID != tenantID {
		return fmt.Errorf("product not found")
	}
	delete(r.Products, productID)
	return nil
}

func (r *MockRepository) GenerateCode(ctx context.Context, schemaName, tenantID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ProductCodeSeq++
	return fmt.Sprintf("PRD-%05d", r.ProductCodeSeq), nil
}

// Categories
func (r *MockRepository) CreateCategory(ctx context.Context, schemaName string, category *ProductCategory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Categories[category.ID] = category
	return nil
}

func (r *MockRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*ProductCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, exists := r.Categories[categoryID]
	if !exists || c.TenantID != tenantID {
		return nil, fmt.Errorf("category not found")
	}
	return c, nil
}

func (r *MockRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]ProductCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ProductCategory
	for _, c := range r.Categories {
		if c.TenantID == tenantID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (r *MockRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, exists := r.Categories[categoryID]
	if !exists || c.TenantID != tenantID {
		return fmt.Errorf("category not found")
	}
	delete(r.Categories, categoryID)
	return nil
}

// Warehouses
func (r *MockRepository) CreateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Warehouses[warehouse.ID] = warehouse
	return nil
}

func (r *MockRepository) GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*Warehouse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, exists := r.Warehouses[warehouseID]
	if !exists || w.TenantID != tenantID {
		return nil, fmt.Errorf("warehouse not found")
	}
	warehouseCopy := *w
	return &warehouseCopy, nil
}

func (r *MockRepository) ListWarehouses(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Warehouse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Warehouse
	for _, w := range r.Warehouses {
		if w.TenantID != tenantID {
			continue
		}
		if activeOnly && !w.IsActive {
			continue
		}
		result = append(result, *w)
	}
	return result, nil
}

func (r *MockRepository) UpdateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Warehouses[warehouse.ID]; !exists {
		return fmt.Errorf("warehouse not found")
	}
	r.Warehouses[warehouse.ID] = warehouse
	return nil
}

func (r *MockRepository) DeleteWarehouse(ctx context.Context, schemaName, tenantID, warehouseID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, exists := r.Warehouses[warehouseID]
	if !exists || w.TenantID != tenantID {
		return fmt.Errorf("warehouse not found")
	}
	delete(r.Warehouses, warehouseID)
	return nil
}

// Stock Levels
func (r *MockRepository) GetStockLevel(ctx context.Context, schemaName, tenantID, productID, warehouseID string) (*StockLevel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := productID + "-" + warehouseID
	s, exists := r.StockLevels[key]
	if !exists || s.TenantID != tenantID {
		return nil, fmt.Errorf("stock level not found")
	}
	return s, nil
}

func (r *MockRepository) GetStockLevelsByProduct(ctx context.Context, schemaName, tenantID, productID string) ([]StockLevel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []StockLevel
	for _, s := range r.StockLevels {
		if s.TenantID == tenantID && s.ProductID == productID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (r *MockRepository) UpsertStockLevel(ctx context.Context, schemaName string, level *StockLevel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := level.ProductID + "-" + level.WarehouseID
	r.StockLevels[key] = level
	return nil
}

// Movements
func (r *MockRepository) CreateMovement(ctx context.Context, schemaName string, movement *InventoryMovement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Movements[movement.ProductID] = append(r.Movements[movement.ProductID], *movement)
	return nil
}

func (r *MockRepository) ListMovements(ctx context.Context, schemaName, tenantID, productID string) ([]InventoryMovement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	movements := r.Movements[productID]
	var result []InventoryMovement
	for _, m := range movements {
		if m.TenantID == tenantID {
			result = append(result, m)
		}
	}
	return result, nil
}

// Stock updates
func (r *MockRepository) UpdateProductStock(ctx context.Context, schemaName, tenantID, productID string, newStock decimal.Decimal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, exists := r.Products[productID]
	if !exists || p.TenantID != tenantID {
		return fmt.Errorf("product not found")
	}
	p.CurrentStock = newStock
	return nil
}

// Test Constants
func TestProductTypeConstants(t *testing.T) {
	assert.Equal(t, ProductType("GOODS"), ProductTypeGoods)
	assert.Equal(t, ProductType("SERVICE"), ProductTypeService)
}

func TestProductStatusConstants(t *testing.T) {
	assert.Equal(t, ProductStatus("ACTIVE"), ProductStatusActive)
	assert.Equal(t, ProductStatus("INACTIVE"), ProductStatusInactive)
}

func TestMovementTypeConstants(t *testing.T) {
	assert.Equal(t, MovementType("IN"), MovementTypeIn)
	assert.Equal(t, MovementType("OUT"), MovementTypeOut)
	assert.Equal(t, MovementType("ADJUSTMENT"), MovementTypeAdjustment)
	assert.Equal(t, MovementType("TRANSFER"), MovementTypeTransfer)
}

// Product Validation
func TestProduct_Validate(t *testing.T) {
	tests := []struct {
		name    string
		product Product
		wantErr string
	}{
		{
			name: "valid goods product",
			product: Product{
				Name:        "Test Product",
				ProductType: ProductTypeGoods,
				SalesPrice:  decimal.NewFromInt(100),
			},
			wantErr: "",
		},
		{
			name: "valid service product",
			product: Product{
				Name:        "Test Service",
				ProductType: ProductTypeService,
				SalesPrice:  decimal.NewFromInt(50),
			},
			wantErr: "",
		},
		{
			name: "missing name",
			product: Product{
				ProductType: ProductTypeGoods,
				SalesPrice:  decimal.NewFromInt(100),
			},
			wantErr: "product name is required",
		},
		{
			name: "missing product type",
			product: Product{
				Name:       "Test",
				SalesPrice: decimal.NewFromInt(100),
			},
			wantErr: "product type is required",
		},
		{
			name: "invalid product type",
			product: Product{
				Name:        "Test",
				ProductType: "INVALID",
				SalesPrice:  decimal.NewFromInt(100),
			},
			wantErr: "invalid product type",
		},
		{
			name: "negative sales price",
			product: Product{
				Name:        "Test",
				ProductType: ProductTypeGoods,
				SalesPrice:  decimal.NewFromInt(-100),
			},
			wantErr: "sales price cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.product.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// MockRepository Tests
func TestMockRepository_Products(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	product := &Product{
		ID:          "p1",
		TenantID:    "tenant-1",
		Code:        "PRD-00001",
		Name:        "Widget",
		ProductType: ProductTypeGoods,
		SalesPrice:  decimal.NewFromInt(99),
		IsActive:    true,
	}

	// Create
	err := repo.CreateProduct(ctx, "test_schema", product)
	require.NoError(t, err)

	// Get
	retrieved, err := repo.GetProductByID(ctx, "test_schema", "tenant-1", "p1")
	require.NoError(t, err)
	assert.Equal(t, "Widget", retrieved.Name)

	// Get not found
	_, err = repo.GetProductByID(ctx, "test_schema", "tenant-1", "nonexistent")
	assert.Error(t, err)

	// Get wrong tenant
	_, err = repo.GetProductByID(ctx, "test_schema", "wrong-tenant", "p1")
	assert.Error(t, err)

	// List
	products, err := repo.ListProducts(ctx, "test_schema", "tenant-1", nil)
	require.NoError(t, err)
	assert.Len(t, products, 1)

	// Update
	product.Name = "Updated Widget"
	err = repo.UpdateProduct(ctx, "test_schema", product)
	require.NoError(t, err)

	// Delete
	err = repo.DeleteProduct(ctx, "test_schema", "tenant-1", "p1")
	require.NoError(t, err)

	_, err = repo.GetProductByID(ctx, "test_schema", "tenant-1", "p1")
	assert.Error(t, err)
}

func TestMockRepository_ListProducts_WithFilter(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// p1: low stock (current 5 <= reorder 10)
	repo.Products["p1"] = &Product{ID: "p1", TenantID: "tenant-1", Name: "Goods 1", ProductType: ProductTypeGoods, CategoryID: "cat-1", IsActive: true, CurrentStock: decimal.NewFromInt(5), ReorderPoint: decimal.NewFromInt(10)}
	// p2: not low stock (current 50 > reorder 10)
	repo.Products["p2"] = &Product{ID: "p2", TenantID: "tenant-1", Name: "Goods 2", ProductType: ProductTypeGoods, CategoryID: "cat-2", IsActive: false, CurrentStock: decimal.NewFromInt(50), ReorderPoint: decimal.NewFromInt(10)}
	// p3: not low stock (current 100 > reorder 20)
	repo.Products["p3"] = &Product{ID: "p3", TenantID: "tenant-1", Name: "Service 1", ProductType: ProductTypeService, IsActive: true, CurrentStock: decimal.NewFromInt(100), ReorderPoint: decimal.NewFromInt(20)}

	// Filter by product type
	products, err := repo.ListProducts(ctx, "test_schema", "tenant-1", &ProductFilter{ProductType: ProductTypeGoods})
	require.NoError(t, err)
	assert.Len(t, products, 2)

	// Filter by category
	products, err = repo.ListProducts(ctx, "test_schema", "tenant-1", &ProductFilter{CategoryID: "cat-1"})
	require.NoError(t, err)
	assert.Len(t, products, 1)

	// Filter by status active
	products, err = repo.ListProducts(ctx, "test_schema", "tenant-1", &ProductFilter{Status: ProductStatusActive})
	require.NoError(t, err)
	assert.Len(t, products, 2)

	// Filter by low stock
	products, err = repo.ListProducts(ctx, "test_schema", "tenant-1", &ProductFilter{LowStock: true})
	require.NoError(t, err)
	assert.Len(t, products, 1)
	assert.Equal(t, "p1", products[0].ID)
}

func TestMockRepository_GenerateCode(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	code1, err := repo.GenerateCode(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "PRD-00001", code1)

	code2, err := repo.GenerateCode(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "PRD-00002", code2)
}

func TestMockRepository_Categories(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	cat := &ProductCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Electronics",
	}

	// Create
	err := repo.CreateCategory(ctx, "test_schema", cat)
	require.NoError(t, err)

	// Get
	retrieved, err := repo.GetCategoryByID(ctx, "test_schema", "tenant-1", "cat-1")
	require.NoError(t, err)
	assert.Equal(t, "Electronics", retrieved.Name)

	// List
	categories, err := repo.ListCategories(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)
	assert.Len(t, categories, 1)

	// Delete
	err = repo.DeleteCategory(ctx, "test_schema", "tenant-1", "cat-1")
	require.NoError(t, err)

	_, err = repo.GetCategoryByID(ctx, "test_schema", "tenant-1", "cat-1")
	assert.Error(t, err)
}

func TestMockRepository_Warehouses(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	wh := &Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Code:     "WH-001",
		Name:     "Main Warehouse",
		IsActive: true,
	}

	// Create
	err := repo.CreateWarehouse(ctx, "test_schema", wh)
	require.NoError(t, err)

	// Get
	retrieved, err := repo.GetWarehouseByID(ctx, "test_schema", "tenant-1", "wh-1")
	require.NoError(t, err)
	assert.Equal(t, "Main Warehouse", retrieved.Name)

	// List all
	warehouses, err := repo.ListWarehouses(ctx, "test_schema", "tenant-1", false)
	require.NoError(t, err)
	assert.Len(t, warehouses, 1)

	// Add inactive warehouse
	repo.Warehouses["wh-2"] = &Warehouse{ID: "wh-2", TenantID: "tenant-1", Name: "Inactive", IsActive: false}

	// List active only
	warehouses, err = repo.ListWarehouses(ctx, "test_schema", "tenant-1", true)
	require.NoError(t, err)
	assert.Len(t, warehouses, 1)

	// Update
	wh.Name = "Updated Warehouse"
	err = repo.UpdateWarehouse(ctx, "test_schema", wh)
	require.NoError(t, err)

	// Delete
	err = repo.DeleteWarehouse(ctx, "test_schema", "tenant-1", "wh-1")
	require.NoError(t, err)
}

func TestMockRepository_StockLevels(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	level := &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    "p1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.NewFromInt(100),
		ReservedQty:  decimal.NewFromInt(10),
		AvailableQty: decimal.NewFromInt(90),
	}

	// Upsert
	err := repo.UpsertStockLevel(ctx, "test_schema", level)
	require.NoError(t, err)

	// Get
	retrieved, err := repo.GetStockLevel(ctx, "test_schema", "tenant-1", "p1", "wh-1")
	require.NoError(t, err)
	assert.True(t, retrieved.Quantity.Equal(decimal.NewFromInt(100)))

	// Get by product
	levels, err := repo.GetStockLevelsByProduct(ctx, "test_schema", "tenant-1", "p1")
	require.NoError(t, err)
	assert.Len(t, levels, 1)
}

func TestMockRepository_Movements(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	movement := &InventoryMovement{
		ID:           "m1",
		TenantID:     "tenant-1",
		ProductID:    "p1",
		WarehouseID:  "wh-1",
		MovementType: MovementTypeIn,
		Quantity:     decimal.NewFromInt(50),
	}

	// Create
	err := repo.CreateMovement(ctx, "test_schema", movement)
	require.NoError(t, err)

	// List
	movements, err := repo.ListMovements(ctx, "test_schema", "tenant-1", "p1")
	require.NoError(t, err)
	assert.Len(t, movements, 1)
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

func TestService_CreateProduct(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateProductRequest{
		Name:        "Test Widget",
		ProductType: "GOODS",
		SalesPrice:  "99.99",
		VATRate:     "20",
	}

	product, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, product.ID)
	assert.Equal(t, "PRD-00001", product.Code)
	assert.Equal(t, "Test Widget", product.Name)
	assert.Equal(t, ProductTypeGoods, product.ProductType)
	assert.True(t, product.SalesPrice.Equal(decimal.RequireFromString("99.99")))
	assert.True(t, product.VATRate.Equal(decimal.NewFromInt(20)))
	assert.True(t, product.IsActive)
}

func TestService_CreateProduct_WithCustomCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateProductRequest{
		Code:        "CUSTOM-001",
		Name:        "Custom Product",
		ProductType: "SERVICE",
		SalesPrice:  "50",
	}

	product, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM-001", product.Code)
}

func TestService_CreateProduct_Defaults(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateProductRequest{
		Name:       "Default Product",
		SalesPrice: "100",
	}

	product, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, ProductTypeGoods, product.ProductType) // Default
	assert.Equal(t, "pcs", product.Unit)                   // Default
	assert.True(t, product.VATRate.Equal(decimal.NewFromInt(22))) // Default Estonian VAT
}

func TestService_CreateProduct_InvalidSalesPrice(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateProductRequest{
		Name:        "Test",
		ProductType: "GOODS",
		SalesPrice:  "invalid",
	}

	_, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sales price")
}

func TestService_CreateProduct_ValidationError(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateProductRequest{
		Name:        "", // Missing name
		ProductType: "GOODS",
		SalesPrice:  "100",
	}

	_, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestService_GetProductByID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:       "p1",
		TenantID: "tenant-1",
		Name:     "Widget",
	}

	product, err := ts.svc.GetProductByID(ctx, "tenant-1", "test_schema", "p1")
	require.NoError(t, err)
	assert.Equal(t, "Widget", product.Name)

	// Not found
	_, err = ts.svc.GetProductByID(ctx, "tenant-1", "test_schema", "nonexistent")
	assert.Error(t, err)
}

func TestService_ListProducts(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{ID: "p1", TenantID: "tenant-1", Name: "Product 1", IsActive: true}
	ts.repo.Products["p2"] = &Product{ID: "p2", TenantID: "tenant-1", Name: "Product 2", IsActive: false}

	products, err := ts.svc.ListProducts(ctx, "tenant-1", "test_schema", nil)
	require.NoError(t, err)
	assert.Len(t, products, 2)
}

func TestService_UpdateProduct(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:          "p1",
		TenantID:    "tenant-1",
		Name:        "Original",
		ProductType: ProductTypeGoods,
		SalesPrice:  decimal.NewFromInt(100),
	}

	req := &UpdateProductRequest{
		Name:       "Updated",
		SalesPrice: "150",
		IsActive:   true,
	}

	product, err := ts.svc.UpdateProduct(ctx, "tenant-1", "test_schema", "p1", req)
	require.NoError(t, err)
	assert.Equal(t, "Updated", product.Name)
	assert.True(t, product.SalesPrice.Equal(decimal.NewFromInt(150)))
}

func TestService_DeleteProduct(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:       "p1",
		TenantID: "tenant-1",
		Name:     "To Delete",
	}

	err := ts.svc.DeleteProduct(ctx, "tenant-1", "test_schema", "p1")
	require.NoError(t, err)

	_, err = ts.svc.GetProductByID(ctx, "tenant-1", "test_schema", "p1")
	assert.Error(t, err)
}

func TestService_CreateCategory(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateCategoryRequest{
		Name:        "Electronics",
		Description: "Electronic products",
	}

	cat, err := ts.svc.CreateCategory(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, cat.ID)
	assert.Equal(t, "Electronics", cat.Name)
}

func TestService_GetCategoryByID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-1"] = &ProductCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "Electronics",
	}

	cat, err := ts.svc.GetCategoryByID(ctx, "tenant-1", "test_schema", "cat-1")
	require.NoError(t, err)
	assert.Equal(t, "Electronics", cat.Name)
}

func TestService_ListCategories(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-1"] = &ProductCategory{ID: "cat-1", TenantID: "tenant-1", Name: "Electronics"}
	ts.repo.Categories["cat-2"] = &ProductCategory{ID: "cat-2", TenantID: "tenant-1", Name: "Furniture"}

	categories, err := ts.svc.ListCategories(ctx, "tenant-1", "test_schema")
	require.NoError(t, err)
	assert.Len(t, categories, 2)
}

func TestService_DeleteCategory(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-1"] = &ProductCategory{
		ID:       "cat-1",
		TenantID: "tenant-1",
		Name:     "To Delete",
	}

	err := ts.svc.DeleteCategory(ctx, "tenant-1", "test_schema", "cat-1")
	require.NoError(t, err)
}

func TestService_CreateWarehouse(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &CreateWarehouseRequest{
		Code:      "WH-001",
		Name:      "Main Warehouse",
		Address:   "123 Main St",
		IsDefault: true,
	}

	wh, err := ts.svc.CreateWarehouse(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, wh.ID)
	assert.Equal(t, "Main Warehouse", wh.Name)
	assert.True(t, wh.IsActive)
	assert.True(t, wh.IsDefault)
}

func TestService_GetWarehouseByID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Warehouses["wh-1"] = &Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Main",
	}

	wh, err := ts.svc.GetWarehouseByID(ctx, "tenant-1", "test_schema", "wh-1")
	require.NoError(t, err)
	assert.Equal(t, "Main", wh.Name)
}

func TestService_ListWarehouses(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.Warehouses["wh-2"] = &Warehouse{ID: "wh-2", TenantID: "tenant-1", Name: "Secondary", IsActive: false}

	// All warehouses
	warehouses, err := ts.svc.ListWarehouses(ctx, "tenant-1", "test_schema", false)
	require.NoError(t, err)
	assert.Len(t, warehouses, 2)

	// Active only
	warehouses, err = ts.svc.ListWarehouses(ctx, "tenant-1", "test_schema", true)
	require.NoError(t, err)
	assert.Len(t, warehouses, 1)
}

func TestService_UpdateWarehouse(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Warehouses["wh-1"] = &Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Original",
		IsActive: true,
	}

	req := &UpdateWarehouseRequest{
		Name:      "Updated",
		Address:   "456 New St",
		IsDefault: false,
		IsActive:  true,
	}

	wh, err := ts.svc.UpdateWarehouse(ctx, "tenant-1", "test_schema", "wh-1", req)
	require.NoError(t, err)
	assert.Equal(t, "Updated", wh.Name)
	assert.Equal(t, "456 New St", wh.Address)
}

func TestService_DeleteWarehouse(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Warehouses["wh-1"] = &Warehouse{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "To Delete",
	}

	err := ts.svc.DeleteWarehouse(ctx, "tenant-1", "test_schema", "wh-1")
	require.NoError(t, err)
}

func TestService_AdjustStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:           "p1",
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(100),
	}

	req := &AdjustStockRequest{
		ProductID:   "p1",
		WarehouseID: "wh-1",
		Quantity:    "50",
		UnitCost:    "10",
		Reason:      "Received shipment",
		UserID:      "user-1",
	}

	movement, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, MovementTypeIn, movement.MovementType)
	assert.True(t, movement.Quantity.Equal(decimal.NewFromInt(50)))

	// Check product stock updated
	product, _ := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", "p1")
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(150)))
}

func TestService_AdjustStock_Negative(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:           "p1",
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(100),
	}

	req := &AdjustStockRequest{
		ProductID:   "p1",
		WarehouseID: "wh-1",
		Quantity:    "-30",
		Reason:      "Damaged goods",
		UserID:      "user-1",
	}

	movement, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, MovementTypeOut, movement.MovementType)
	assert.True(t, movement.Quantity.Equal(decimal.NewFromInt(30))) // Absolute value

	// Check product stock updated
	product, _ := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", "p1")
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(70)))
}

func TestService_AdjustStock_InvalidQuantity(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &AdjustStockRequest{
		ProductID:   "p1",
		WarehouseID: "wh-1",
		Quantity:    "invalid",
	}

	_, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid quantity")
}

func TestService_TransferStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &TransferStockRequest{
		ProductID:       "p1",
		FromWarehouseID: "wh-1",
		ToWarehouseID:   "wh-2",
		Quantity:        "25",
		Notes:           "Transfer between warehouses",
		UserID:          "user-1",
	}

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)

	// Check movements created
	movements := ts.repo.Movements["p1"]
	assert.Len(t, movements, 2) // OUT and IN movements
}

func TestService_TransferStock_InvalidQuantity(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &TransferStockRequest{
		ProductID:       "p1",
		FromWarehouseID: "wh-1",
		ToWarehouseID:   "wh-2",
		Quantity:        "invalid",
	}

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid quantity")
}

func TestService_TransferStock_NegativeQuantity(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	req := &TransferStockRequest{
		ProductID:       "p1",
		FromWarehouseID: "wh-1",
		ToWarehouseID:   "wh-2",
		Quantity:        "-10",
	}

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity must be positive")
}

func TestService_GetStockLevels(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.StockLevels["p1-wh-1"] = &StockLevel{
		ID:          "sl-1",
		TenantID:    "tenant-1",
		ProductID:   "p1",
		WarehouseID: "wh-1",
		Quantity:    decimal.NewFromInt(50),
	}

	levels, err := ts.svc.GetStockLevels(ctx, "tenant-1", "test_schema", "p1")
	require.NoError(t, err)
	assert.Len(t, levels, 1)
}

func TestService_GetMovements(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Movements["p1"] = []InventoryMovement{
		{ID: "m1", TenantID: "tenant-1", ProductID: "p1", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(100)},
		{ID: "m2", TenantID: "tenant-1", ProductID: "p1", MovementType: MovementTypeOut, Quantity: decimal.NewFromInt(25)},
	}

	movements, err := ts.svc.GetMovements(ctx, "tenant-1", "test_schema", "p1")
	require.NoError(t, err)
	assert.Len(t, movements, 2)
}

func TestNewService(t *testing.T) {
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
