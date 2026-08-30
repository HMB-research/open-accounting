package inventory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	inventoryStockProductID    = "11111111-1111-4111-8111-111111111111"
	inventoryStockWarehouseID  = "22222222-2222-4222-8222-222222222222"
	inventoryStockWarehouseID2 = "33333333-3333-4333-8333-333333333333"
)

func inventoryStockLevelKey(productID, warehouseID string) string {
	return productID + "-" + warehouseID
}

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	mu                         sync.RWMutex
	Products                   map[string]*Product
	Categories                 map[string]*ProductCategory
	Warehouses                 map[string]*Warehouse
	StockLevels                map[string]*StockLevel // key: productID-warehouseID
	Movements                  map[string][]InventoryMovement
	LotReservations            map[string]*InventoryLotReservation
	ProductCodeSeq             int
	ErrOnCreate                bool
	ErrOnGenerate              bool
	ErrOnGet                   bool
	ErrOnGetWarehouse          bool
	ErrOnUpdate                bool
	ErrOnDelete                bool
	ErrOnListProducts          bool
	ErrOnListCategories        bool
	ErrOnListWarehouses        bool
	ErrOnCreateCategory        bool
	ErrOnCreateWarehouse       bool
	ErrOnUpdateWarehouse       bool
	ErrOnDeleteCategory        bool
	ErrOnDeleteWarehouse       bool
	ErrOnGetStockLevels        bool
	ErrOnUpsertStockLevel      bool
	ErrOnCreateMovement        bool
	ErrOnListMovements         bool
	ErrOnUpdateProductStock    bool
	ErrOnListLotReservations   bool
	ErrOnUpsertLotReservation  bool
	ErrOnReleaseLotReservation bool
}

// NewMockRepository creates a new mock repository
func NewMockRepository() *MockRepository {
	return &MockRepository{
		Products:        make(map[string]*Product),
		Categories:      make(map[string]*ProductCategory),
		Warehouses:      make(map[string]*Warehouse),
		StockLevels:     make(map[string]*StockLevel),
		Movements:       make(map[string][]InventoryMovement),
		LotReservations: make(map[string]*InventoryLotReservation),
		ProductCodeSeq:  0,
	}
}

type mockRepositorySnapshot struct {
	products        map[string]*Product
	categories      map[string]*ProductCategory
	warehouses      map[string]*Warehouse
	stockLevels     map[string]*StockLevel
	movements       map[string][]InventoryMovement
	lotReservations map[string]*InventoryLotReservation
	productCodeSeq  int
}

func (r *MockRepository) WithInventoryLedgerTransaction(ctx context.Context, ledger accountingPoster, fn func(repo Repository, ledger accountingPoster) error) error {
	snapshot := r.snapshot()
	if err := fn(r, ledger); err != nil {
		r.restore(snapshot)
		return err
	}
	return nil
}

func (r *MockRepository) snapshot() mockRepositorySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := mockRepositorySnapshot{
		products:        make(map[string]*Product, len(r.Products)),
		categories:      make(map[string]*ProductCategory, len(r.Categories)),
		warehouses:      make(map[string]*Warehouse, len(r.Warehouses)),
		stockLevels:     make(map[string]*StockLevel, len(r.StockLevels)),
		movements:       make(map[string][]InventoryMovement, len(r.Movements)),
		lotReservations: make(map[string]*InventoryLotReservation, len(r.LotReservations)),
		productCodeSeq:  r.ProductCodeSeq,
	}
	for id, product := range r.Products {
		copy := *product
		snapshot.products[id] = &copy
	}
	for id, category := range r.Categories {
		copy := *category
		snapshot.categories[id] = &copy
	}
	for id, warehouse := range r.Warehouses {
		copy := *warehouse
		snapshot.warehouses[id] = &copy
	}
	for id, level := range r.StockLevels {
		copy := *level
		snapshot.stockLevels[id] = &copy
	}
	for id, movements := range r.Movements {
		snapshot.movements[id] = append([]InventoryMovement(nil), movements...)
	}
	for id, reservation := range r.LotReservations {
		copy := *reservation
		snapshot.lotReservations[id] = &copy
	}
	return snapshot
}

func (r *MockRepository) restore(snapshot mockRepositorySnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Products = snapshot.products
	r.Categories = snapshot.categories
	r.Warehouses = snapshot.warehouses
	r.StockLevels = snapshot.stockLevels
	r.Movements = snapshot.movements
	r.LotReservations = snapshot.lotReservations
	r.ProductCodeSeq = snapshot.productCodeSeq
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
	if r.ErrOnListProducts {
		return nil, fmt.Errorf("mock error on list products")
	}
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
	if r.ErrOnGenerate {
		return "", fmt.Errorf("mock error on generate code")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ProductCodeSeq++
	return fmt.Sprintf("PRD-%05d", r.ProductCodeSeq), nil
}

// Categories
func (r *MockRepository) CreateCategory(ctx context.Context, schemaName string, category *ProductCategory) error {
	if r.ErrOnCreateCategory {
		return fmt.Errorf("mock error on create category")
	}
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
	if r.ErrOnListCategories {
		return nil, fmt.Errorf("mock error on list categories")
	}
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
	if r.ErrOnDeleteCategory {
		return fmt.Errorf("mock error on delete category")
	}
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
	if r.ErrOnCreateWarehouse {
		return fmt.Errorf("mock error on create warehouse")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Warehouses[warehouse.ID] = warehouse
	return nil
}

func (r *MockRepository) GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*Warehouse, error) {
	if r.ErrOnGetWarehouse {
		return nil, fmt.Errorf("mock error on get warehouse")
	}
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
	if r.ErrOnListWarehouses {
		return nil, fmt.Errorf("mock error on list warehouses")
	}
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
	if r.ErrOnUpdateWarehouse {
		return fmt.Errorf("mock error on update warehouse")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.Warehouses[warehouse.ID]; !exists {
		return fmt.Errorf("warehouse not found")
	}
	r.Warehouses[warehouse.ID] = warehouse
	return nil
}

func (r *MockRepository) DeleteWarehouse(ctx context.Context, schemaName, tenantID, warehouseID string) error {
	if r.ErrOnDeleteWarehouse {
		return fmt.Errorf("mock error on delete warehouse")
	}
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
	if r.ErrOnGetStockLevels {
		return nil, fmt.Errorf("mock error on get stock levels")
	}
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
	if r.ErrOnUpsertStockLevel {
		return fmt.Errorf("mock error on upsert stock level")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := level.ProductID + "-" + level.WarehouseID
	r.StockLevels[key] = level
	return nil
}

func inventoryLotReservationKey(productID, warehouseID, lotNumber, serialNumber, expiryDate string) string {
	return strings.Join([]string{
		productID,
		warehouseID,
		strings.TrimSpace(lotNumber),
		strings.TrimSpace(serialNumber),
		strings.TrimSpace(expiryDate),
	}, "|")
}

func (r *MockRepository) ListLotReservations(ctx context.Context, schemaName, tenantID, productID, warehouseID string) ([]InventoryLotReservation, error) {
	if r.ErrOnListLotReservations {
		return nil, fmt.Errorf("mock error on list lot reservations")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []InventoryLotReservation
	for _, reservation := range r.LotReservations {
		if reservation.TenantID == tenantID &&
			reservation.ProductID == productID &&
			reservation.WarehouseID == warehouseID &&
			reservation.Quantity.GreaterThan(decimal.Zero) {
			result = append(result, *reservation)
		}
	}
	return result, nil
}

func (r *MockRepository) UpsertLotReservation(ctx context.Context, schemaName string, reservation *InventoryLotReservation) error {
	if r.ErrOnUpsertLotReservation {
		return fmt.Errorf("mock error on upsert lot reservation")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := inventoryLotReservationKey(reservation.ProductID, reservation.WarehouseID, reservation.LotNumber, reservation.SerialNumber, reservation.ExpiryDate)
	existing := r.LotReservations[key]
	if existing == nil {
		copy := *reservation
		r.LotReservations[key] = &copy
		return nil
	}
	existing.Quantity = existing.Quantity.Add(reservation.Quantity)
	existing.Reason = reservation.Reason
	existing.UpdatedAt = reservation.UpdatedAt
	existing.CreatedBy = reservation.CreatedBy
	return nil
}

func (r *MockRepository) ReleaseLotReservation(ctx context.Context, schemaName, tenantID, productID, warehouseID, lotNumber, serialNumber, expiryDate string, quantity decimal.Decimal, reason, releasedBy string) (*InventoryLotReservation, error) {
	if r.ErrOnReleaseLotReservation {
		return nil, fmt.Errorf("mock error on release lot reservation")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := inventoryLotReservationKey(productID, warehouseID, lotNumber, serialNumber, expiryDate)
	reservation := r.LotReservations[key]
	if reservation == nil || reservation.TenantID != tenantID || reservation.Quantity.LessThan(quantity) {
		return nil, fmt.Errorf("tracked lot reservation not found")
	}
	reservation.Quantity = reservation.Quantity.Sub(quantity)
	reservation.Reason = reason
	reservation.UpdatedAt = time.Now()
	reservation.CreatedBy = releasedBy
	copy := *reservation
	return &copy, nil
}

// Movements
func (r *MockRepository) CreateMovement(ctx context.Context, schemaName string, movement *InventoryMovement) error {
	if r.ErrOnCreateMovement {
		return fmt.Errorf("mock error on create movement")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Movements[movement.ProductID] = append(r.Movements[movement.ProductID], *movement)
	return nil
}

func (r *MockRepository) ListMovements(ctx context.Context, schemaName, tenantID, productID string) ([]InventoryMovement, error) {
	if r.ErrOnListMovements {
		return nil, fmt.Errorf("mock error on list movements")
	}
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
	if r.ErrOnUpdateProductStock {
		return fmt.Errorf("mock error on update product stock")
	}
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
		LotNumber:    "LOT-2026-01",
		SerialNumber: "SN-001",
		ExpiryDate:   "2027-01-31",
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

type fakeInventoryAccountLister struct {
	accounts []accounting.Account
	err      error
}

func (f fakeInventoryAccountLister) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.accounts, nil
}

type fakeInventoryContactLister struct {
	contacts []contacts.Contact
	err      error
}

func (f fakeInventoryContactLister) List(_ context.Context, _, _ string, _ *contacts.ContactFilter) ([]contacts.Contact, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.contacts, nil
}

type fakeInventoryLedger struct {
	accounts       []accounting.Account
	createdRequest *accounting.CreateJournalEntryRequest
	postedIDs      []string
	postErr        error
}

func (f *fakeInventoryLedger) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	return f.accounts, nil
}

func (f *fakeInventoryLedger) CreateJournalEntry(_ context.Context, _, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	f.createdRequest = req
	return &accounting.JournalEntry{ID: "journal-1", TenantID: tenantID, EntryNumber: "JE-00001", Status: accounting.StatusDraft}, nil
}

func (f *fakeInventoryLedger) PostJournalEntry(_ context.Context, _, _, entryID, _, _ string) error {
	if f.postErr != nil {
		return f.postErr
	}
	f.postedIDs = append(f.postedIDs, entryID)
	return nil
}

type fakeInventoryBalancer struct {
	accounts   []accounting.Account
	balances   map[string]decimal.Decimal
	listErr    error
	balanceErr error
}

func (f fakeInventoryBalancer) ListAccounts(_ context.Context, _, _ string, _ bool) ([]accounting.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.accounts, nil
}

func (f fakeInventoryBalancer) GetAccountBalance(_ context.Context, _, _, accountID string, _ time.Time) (decimal.Decimal, error) {
	if f.balanceErr != nil {
		return decimal.Zero, f.balanceErr
	}
	if balance, ok := f.balances[accountID]; ok {
		return balance, nil
	}
	return decimal.Zero, nil
}

func TestService_CreateProduct(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	categoryID := "11111111-1111-4111-8111-111111111111"
	saleAccountID := "22222222-2222-4222-8222-222222222222"
	purchaseAccountID := "33333333-3333-4333-8333-333333333333"
	inventoryAccountID := "44444444-4444-4444-8444-444444444444"
	supplierID := "55555555-5555-4555-8555-555555555555"

	req := &CreateProductRequest{
		Name:               "Test Widget",
		ProductType:        "GOODS",
		CategoryID:         " " + categoryID + " ",
		SalesPrice:         "99.99",
		VATRate:            "20",
		SaleAccountID:      " " + saleAccountID + " ",
		PurchaseAccountID:  " " + purchaseAccountID + " ",
		InventoryAccountID: " " + inventoryAccountID + " ",
		SupplierID:         " " + supplierID + " ",
	}

	product, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, product.ID)
	assert.Equal(t, "PRD-00001", product.Code)
	assert.Equal(t, "Test Widget", product.Name)
	assert.Equal(t, ProductTypeGoods, product.ProductType)
	assert.True(t, product.SalesPrice.Equal(decimal.RequireFromString("99.99")))
	assert.True(t, product.VATRate.Equal(decimal.NewFromInt(20)))
	assert.Equal(t, categoryID, product.CategoryID)
	assert.Equal(t, saleAccountID, product.SaleAccountID)
	assert.Equal(t, purchaseAccountID, product.PurchaseAccountID)
	assert.Equal(t, inventoryAccountID, product.InventoryAccountID)
	assert.Equal(t, supplierID, product.SupplierID)
	assert.True(t, product.IsActive)
}

func TestService_GetInventorySubledgerReconciliationFlagsMappingExceptions(t *testing.T) {
	repo := NewMockRepository()
	balancer := fakeInventoryBalancer{
		accounts: []accounting.Account{
			{ID: "11111111-1111-4111-8111-111111111111", Code: "1300", Name: "Inventory", AccountType: accounting.AccountTypeAsset},
			{ID: "22222222-2222-4222-8222-222222222222", Code: "5000", Name: "COGS", AccountType: accounting.AccountTypeExpense},
		},
		balances: map[string]decimal.Decimal{
			"11111111-1111-4111-8111-111111111111": decimal.RequireFromString("120.00"),
		},
	}
	svc := NewServiceWithRepositoryAndAccounting(repo, balancer)
	ctx := context.Background()

	repo.Products["prod-1"] = &Product{
		ID:                 "prod-1",
		TenantID:           "tenant-1",
		Code:               "PRD-001",
		Name:               "Mapped widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("10.00"),
		TrackInventory:     true,
		InventoryAccountID: "11111111-1111-4111-8111-111111111111",
	}
	repo.Products["prod-2"] = &Product{
		ID:             "prod-2",
		TenantID:       "tenant-1",
		Code:           "PRD-002",
		Name:           "Unmapped widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("8.00"),
		TrackInventory: true,
	}
	repo.Products["prod-3"] = &Product{
		ID:                 "prod-3",
		TenantID:           "tenant-1",
		Code:               "PRD-003",
		Name:               "Expense mapped widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("5.00"),
		TrackInventory:     true,
		InventoryAccountID: "22222222-2222-4222-8222-222222222222",
	}
	repo.Products["prod-4"] = &Product{
		ID:                 "prod-4",
		TenantID:           "tenant-1",
		Code:               "PRD-004",
		Name:               "Unknown account widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("4.00"),
		TrackInventory:     true,
		InventoryAccountID: "33333333-3333-4333-8333-333333333333",
	}
	repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main"}
	repo.StockLevels["prod-1-wh-1"] = &StockLevel{TenantID: "tenant-1", ProductID: "prod-1", WarehouseID: "wh-1", Quantity: decimal.RequireFromString("12.00"), AvailableQty: decimal.RequireFromString("12.00")}
	repo.StockLevels["prod-2-wh-1"] = &StockLevel{TenantID: "tenant-1", ProductID: "prod-2", WarehouseID: "wh-1", Quantity: decimal.RequireFromString("1.00"), AvailableQty: decimal.RequireFromString("1.00")}
	repo.StockLevels["prod-3-wh-1"] = &StockLevel{TenantID: "tenant-1", ProductID: "prod-3", WarehouseID: "wh-1", Quantity: decimal.RequireFromString("2.00"), AvailableQty: decimal.RequireFromString("2.00")}
	repo.StockLevels["prod-4-wh-1"] = &StockLevel{TenantID: "tenant-1", ProductID: "prod-4", WarehouseID: "wh-1", Quantity: decimal.RequireFromString("3.00"), AvailableQty: decimal.RequireFromString("3.00")}

	report, err := svc.GetInventorySubledgerReconciliation(ctx, "tenant-1", "test_schema", "wh-1", InventoryValuationMethodStandardCost, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.False(t, report.Ready)
	assert.Equal(t, 1, report.MissingAccountLineCount)
	assert.Equal(t, 1, report.InvalidAccountTypeLineCount)
	assert.Equal(t, 1, report.UnknownAccountLineCount)
	assert.Equal(t, 3, report.BlockingExceptionLineCount)
	assert.Equal(t, 0, report.DifferenceAccountCount)
	assert.True(t, report.TotalSubledgerValue.Equal(decimal.RequireFromString("150.00")))
	assert.True(t, report.TotalGeneralLedgerBalance.Equal(decimal.RequireFromString("120.00")))
	assert.True(t, report.TotalDifference.Equal(decimal.RequireFromString("30.00")))
	require.Len(t, report.AccountLines, 1)
	assert.Equal(t, "1300", report.AccountLines[0].AccountCode)
	assert.True(t, report.AccountLines[0].Balanced)
	assert.True(t, report.AccountLines[0].SubledgerValue.Equal(decimal.RequireFromString("120.00")))
	require.Len(t, report.Lines, 4)
	statuses := make(map[string]bool)
	for _, line := range report.Lines {
		statuses[line.Status] = true
	}
	assert.True(t, statuses[inventorySubledgerStatusMapped])
	assert.True(t, statuses[inventorySubledgerStatusMissingAccount])
	assert.True(t, statuses[inventorySubledgerStatusInvalidAccountType])
	assert.True(t, statuses[inventorySubledgerStatusUnknownAccount])
}

func TestService_GetInventorySubledgerReconciliationFlagsLedgerOnlyBalance(t *testing.T) {
	repo := NewMockRepository()
	accountID := "11111111-1111-4111-8111-111111111111"
	balancer := fakeInventoryBalancer{
		accounts: []accounting.Account{{ID: accountID, Code: "1300", Name: "Inventory", AccountType: accounting.AccountTypeAsset}},
		balances: map[string]decimal.Decimal{accountID: decimal.RequireFromString("5.00")},
	}
	svc := NewServiceWithRepositoryAndAccounting(repo, balancer)
	ctx := context.Background()

	repo.Products["prod-1"] = &Product{
		ID:                 "prod-1",
		TenantID:           "tenant-1",
		Code:               "PRD-001",
		Name:               "Mapped widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("10.00"),
		TrackInventory:     true,
		InventoryAccountID: accountID,
	}

	report, err := svc.GetInventorySubledgerReconciliation(ctx, "tenant-1", "test_schema", "", "", time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.False(t, report.Ready)
	assert.Equal(t, 1, report.DifferenceAccountCount)
	assert.Equal(t, 1, report.BlockingExceptionAccountCount)
	assert.True(t, report.TotalSubledgerValue.IsZero())
	assert.True(t, report.TotalGeneralLedgerBalance.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, report.TotalDifference.Equal(decimal.RequireFromString("-5.00")))
	require.Len(t, report.AccountLines, 1)
	assert.False(t, report.AccountLines[0].Balanced)
	assert.True(t, report.AccountLines[0].Difference.Equal(decimal.RequireFromString("-5.00")))
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

func TestService_CreateProduct_UsesValidatedServerSuppliedID(t *testing.T) {
	ts := newTestService()
	requestedID := "7c856cea-6b83-4e61-8500-0d05f63855d7"

	product, err := ts.svc.CreateProduct(context.Background(), "tenant-1", "test_schema", &CreateProductRequest{
		ID:          requestedID,
		Code:        "SA-ITEM-1",
		Name:        "Source item",
		ProductType: "SERVICE",
		Unit:        "pc",
		SalesPrice:  "12.50",
		VATRate:     "24",
	})

	require.NoError(t, err)
	assert.Equal(t, requestedID, product.ID)
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
	assert.Equal(t, ProductTypeGoods, product.ProductType)        // Default
	assert.Equal(t, "pcs", product.Unit)                          // Default
	assert.True(t, product.VATRate.Equal(decimal.NewFromInt(22))) // Default Estonian VAT
}

func TestService_ImportProductsCSV(t *testing.T) {
	ts := newTestService()
	categoryID := "11111111-1111-4111-8111-111111111111"
	saleAccountID := "22222222-2222-4222-8222-222222222222"
	purchaseAccountID := "33333333-3333-4333-8333-333333333333"
	inventoryAccountID := "44444444-4444-4444-8444-444444444444"
	ts.svc.accounts = fakeInventoryAccountLister{accounts: []accounting.Account{
		{ID: saleAccountID, Code: "4000", AccountType: accounting.AccountTypeRevenue},
		{ID: purchaseAccountID, Code: "5000", AccountType: accounting.AccountTypeExpense},
		{ID: inventoryAccountID, Code: "1400", AccountType: accounting.AccountTypeAsset},
	}}
	ctx := context.Background()

	ts.repo.Categories[categoryID] = &ProductCategory{
		ID:       categoryID,
		TenantID: "tenant-1",
		Name:     "Parts",
	}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		FileName: "products.csv",
		CSVContent: "code,name,product_type,category_name,sales_price,purchase_price,vat_rate,track_inventory,status,sale_account_code,purchase_account_code,inventory_account_code\n" +
			"SKU-001,Widget,GOODS,Parts,15.00,10.50,22,true,ACTIVE,4000,5000,1400\n" +
			",Missing price,GOODS,Parts,,10.50,22,true,ACTIVE,,,\n" +
			",Consulting,SERVICE,,120.00,0,22,,INACTIVE,,,\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "products.csv", result.FileName)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 2, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 3, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "sales_price is required")

	var widget *Product
	var consulting *Product
	for _, product := range ts.repo.Products {
		switch product.Name {
		case "Widget":
			widget = product
		case "Consulting":
			consulting = product
		}
	}

	require.NotNil(t, widget)
	assert.Equal(t, "SKU-001", widget.Code)
	assert.Equal(t, ProductTypeGoods, widget.ProductType)
	assert.Equal(t, categoryID, widget.CategoryID)
	assert.Equal(t, saleAccountID, widget.SaleAccountID)
	assert.Equal(t, purchaseAccountID, widget.PurchaseAccountID)
	assert.Equal(t, inventoryAccountID, widget.InventoryAccountID)
	assert.True(t, widget.TrackInventory)
	assert.True(t, widget.IsActive)
	assert.True(t, widget.SalesPrice.Equal(decimal.RequireFromString("15.00")))

	require.NotNil(t, consulting)
	assert.Equal(t, "PRD-00001", consulting.Code)
	assert.Equal(t, ProductTypeService, consulting.ProductType)
	assert.False(t, consulting.TrackInventory)
	assert.False(t, consulting.IsActive)
}

func TestService_ImportProductsCSV_DuplicateCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["prod-1"] = &Product{
		ID:       "prod-1",
		TenantID: "tenant-1",
		Code:     "SKU-001",
		Name:     "Existing widget",
	}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price\nSKU-001,Widget,15.00\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 0, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "duplicate code")
}

func TestService_ImportProductsCSVReportsMissingAccountCode(t *testing.T) {
	repo := NewMockRepository()
	svc := NewServiceWithRepositoryAndAccounting(repo, fakeInventoryAccountLister{accounts: []accounting.Account{
		{ID: "22222222-2222-4222-8222-222222222222", Code: "4000", AccountType: accounting.AccountTypeRevenue},
	}})
	ctx := context.Background()

	result, err := svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,sale_account_code,purchase_account_code\nSKU-001,Widget,15.00,4000,5999\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 0, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `account code "5999" was not found for purchase_account_code`)
}

func TestService_ImportProductsCSVReportsInvalidAccountID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		FileName:   "products.csv",
		CSVContent: "code,name,sales_price,sale_account_id\nSKU-001,Widget,15.00,legacy-account\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 0, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "sale_account_id must be a valid UUID")
}

func TestService_ImportProductsCSVResolvesCategoryID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	categoryID := "11111111-1111-1111-1111-111111111111"
	ts.repo.Categories[categoryID] = &ProductCategory{
		ID:       categoryID,
		TenantID: "tenant-1",
		Name:     "Parts",
	}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,category_id\nSKU-001,Widget,15.00,11111111-1111-1111-1111-111111111111\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.ProductsCreated)
	assert.Equal(t, 0, result.RowsSkipped)
	assert.Empty(t, result.Errors)

	var imported *Product
	for _, product := range ts.repo.Products {
		if product.Code == "SKU-001" {
			imported = product
		}
	}
	require.NotNil(t, imported)
	assert.Equal(t, categoryID, imported.CategoryID)
}

func TestService_ImportProductsCSVReportsInvalidCategoryID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,category_id\nSKU-001,Widget,15.00,legacy-id\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 0, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "category_id must be a valid UUID")
}

func TestService_ImportProductsCSVReportsInvalidSupplierID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,supplier_id\nSKU-001,Widget,15.00,legacy-supplier\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "supplier_id must be a valid UUID")
}

func TestService_ImportProductsCSVResolvesSupplierCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	supplierID := "55555555-5555-4555-8555-555555555555"
	ts.svc.contacts = fakeInventoryContactLister{contacts: []contacts.Contact{
		{ID: supplierID, Code: "SUP-001", ContactType: contacts.ContactTypeSupplier},
	}}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,supplier_code\nSKU-001,Widget,15.00,SUP-001\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.ProductsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)

	var imported *Product
	for _, product := range ts.repo.Products {
		if product.Code == "SKU-001" {
			imported = product
		}
	}
	require.NotNil(t, imported)
	assert.Equal(t, supplierID, imported.SupplierID)
}

func TestService_ImportProductsCSVReportsMissingSupplierCode(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	ts.svc.contacts = fakeInventoryContactLister{contacts: []contacts.Contact{
		{ID: "55555555-5555-4555-8555-555555555555", Code: "SUP-001", ContactType: contacts.ContactTypeSupplier},
	}}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,supplier_code\nSKU-001,Widget,15.00,SUP-404\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `supplier_code "SUP-404" was not found`)
}

func TestService_ImportProductsCSVResolvesSupplierIdentityFields(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	supplierID := "55555555-5555-4555-8555-555555555555"
	ts.svc.contacts = fakeInventoryContactLister{contacts: []contacts.Contact{
		{
			ID:        supplierID,
			Name:      "Supplier One",
			RegCode:   "12345678",
			VATNumber: "EE12345678",
			Email:     "billing@supplier.example",
		},
	}}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,supplier_reg_code,supplier_vat_number,supplier_email,supplier_name\nSKU-001,Widget,15.00,12345678,EE12345678,billing@supplier.example,Supplier One\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.ProductsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Empty(t, result.Errors)

	var imported *Product
	for _, product := range ts.repo.Products {
		if product.Code == "SKU-001" {
			imported = product
		}
	}
	require.NotNil(t, imported)
	assert.Equal(t, supplierID, imported.SupplierID)
}

func TestService_ImportProductsCSVReportsAmbiguousSupplierName(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	ts.svc.contacts = fakeInventoryContactLister{contacts: []contacts.Contact{
		{ID: "55555555-5555-4555-8555-555555555555", Name: "Supplier One", ContactType: contacts.ContactTypeSupplier},
		{ID: "66666666-6666-4666-8666-666666666666", Name: " supplier one ", ContactType: contacts.ContactTypeSupplier},
	}}

	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,supplier_name\nSKU-001,Widget,15.00,Supplier One\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `supplier_name "Supplier One" matched multiple contacts`)
}

func TestService_ImportProductsCSVReportsMissingCategoryID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["11111111-1111-1111-1111-111111111111"] = &ProductCategory{
		ID:       "11111111-1111-1111-1111-111111111111",
		TenantID: "tenant-1",
		Name:     "Parts",
	}

	missingCategoryID := "22222222-2222-2222-2222-222222222222"
	result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
		CSVContent: "code,name,sales_price,category_id\nSKU-001,Widget,15.00," + missingCategoryID + "\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 0, result.ProductsCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `category_id "`+missingCategoryID+`" was not found`)
}

func TestService_ImportProductCategoriesCSV(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Categories["cat-existing"] = &ProductCategory{
		ID:       "cat-existing",
		TenantID: "tenant-1",
		Name:     "Existing",
	}

	result, err := ts.svc.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{
		FileName: "categories.csv",
		CSVContent: "category_id,category_name,description,parent_name,parent_category_id\n" +
			"11111111-1111-1111-1111-111111111111,Parts,Spare parts,,\n" +
			"22222222-2222-2222-2222-222222222222,Fasteners,Bolts and screws,,11111111-1111-1111-1111-111111111111\n" +
			",Existing,Duplicate,,\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "categories.csv", result.FileName)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 2, result.CategoriesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 4, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "duplicate name")

	var parts *ProductCategory
	var fasteners *ProductCategory
	for _, category := range ts.repo.Categories {
		switch category.Name {
		case "Parts":
			parts = category
		case "Fasteners":
			fasteners = category
		}
	}

	require.NotNil(t, parts)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", parts.ID)
	assert.Equal(t, "Spare parts", parts.Description)
	assert.Empty(t, parts.ParentID)

	require.NotNil(t, fasteners)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", fasteners.ID)
	assert.Equal(t, "Bolts and screws", fasteners.Description)
	assert.Equal(t, parts.ID, fasteners.ParentID)
}

func TestService_ImportProductCategoriesCSVRejectsInvalidParentID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	result, err := ts.svc.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{
		CSVContent: "category_name,parent_category_id\nFasteners,legacy-parent\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.CategoriesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "Fasteners", result.Errors[0].Name)
	assert.Contains(t, result.Errors[0].Message, "parent_id must be a valid UUID")
}

func TestService_ImportWarehousesCSV(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Warehouses["wh-existing"] = &Warehouse{
		ID:       "wh-existing",
		TenantID: "tenant-1",
		Code:     "EXISTING",
		Name:     "Existing warehouse",
		IsActive: true,
	}

	result, err := ts.svc.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{
		FileName: "warehouses.csv",
		CSVContent: "warehouse_code,warehouse_name,address,default,status\n" +
			"MAIN,Main warehouse,Tallinn,yes,ACTIVE\n" +
			"SECOND,Secondary warehouse,Tartu,no,INACTIVE\n" +
			"EXISTING,Duplicate warehouse,,no,ACTIVE\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "warehouses.csv", result.FileName)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 2, result.WarehousesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 4, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "duplicate code")

	var mainWarehouse *Warehouse
	var secondaryWarehouse *Warehouse
	for _, warehouse := range ts.repo.Warehouses {
		switch warehouse.Code {
		case "MAIN":
			mainWarehouse = warehouse
		case "SECOND":
			secondaryWarehouse = warehouse
		}
	}

	require.NotNil(t, mainWarehouse)
	assert.Equal(t, "Main warehouse", mainWarehouse.Name)
	assert.Equal(t, "Tallinn", mainWarehouse.Address)
	assert.True(t, mainWarehouse.IsDefault)
	assert.True(t, mainWarehouse.IsActive)

	require.NotNil(t, secondaryWarehouse)
	assert.Equal(t, "Secondary warehouse", secondaryWarehouse.Name)
	assert.Equal(t, "Tartu", secondaryWarehouse.Address)
	assert.False(t, secondaryWarehouse.IsDefault)
	assert.False(t, secondaryWarehouse.IsActive)
}

func TestService_ImportStockAdjustmentsCSV(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Code:         "SKU-001",
		Name:         "Widget",
		ProductType:  ProductTypeGoods,
		CurrentStock: decimal.Zero,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main",
		IsActive: true,
	}

	result, err := ts.svc.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
		FileName: "stock.csv",
		UserID:   "user-1",
		CSVContent: "product_code,warehouse_code,quantity,unit_cost,lot_number,serial_number,expiry_date,reason\n" +
			"SKU-001,MAIN,1,10.50,LOT-2026-01,SN-001,2027-01-31,Opening stock\n" +
			"MISSING,MAIN,5,10.50,,,,Missing product\n",
	})

	require.NoError(t, err)
	assert.Equal(t, "stock.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.AdjustmentsImported)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 3, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "product_code")

	product := ts.repo.Products[inventoryStockProductID]
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(1)))

	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.NewFromInt(1)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(1)))

	require.Len(t, ts.repo.Movements[inventoryStockProductID], 1)
	movement := ts.repo.Movements[inventoryStockProductID][0]
	assert.Equal(t, MovementTypeIn, movement.MovementType)
	assert.True(t, movement.Quantity.Equal(decimal.NewFromInt(1)))
	assert.True(t, movement.UnitCost.Equal(decimal.RequireFromString("10.50")))
	assert.Equal(t, "LOT-2026-01", movement.LotNumber)
	assert.Equal(t, "SN-001", movement.SerialNumber)
	assert.Equal(t, "2027-01-31", movement.ExpiryDate)
	assert.Equal(t, "Opening stock", movement.Notes)
	assert.Equal(t, "user-1", movement.CreatedBy)
}

func TestService_ImportStockAdjustmentsCSVReportsSerializedStockIssues(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Code:         "SKU-001",
		Name:         "Widget",
		ProductType:  ProductTypeGoods,
		CurrentStock: decimal.Zero,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main",
		IsActive: true,
	}

	result, err := ts.svc.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
		FileName: "stock.csv",
		UserID:   "user-1",
		CSVContent: "product_code,warehouse_code,quantity,serial_number\n" +
			"SKU-001,MAIN,2,SN-001\n" +
			"SKU-001,MAIN,1,SN-002\n" +
			"SKU-001,MAIN,1,SN-002\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 1, result.AdjustmentsImported)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Contains(t, result.Errors[0].Message, "serial_number requires quantity 1 or -1")
	assert.Equal(t, 4, result.Errors[1].Row)
	assert.Contains(t, result.Errors[1].Message, "duplicates row 3")

	product := ts.repo.Products[inventoryStockProductID]
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(1)))
	require.Len(t, ts.repo.Movements[inventoryStockProductID], 1)
	assert.Equal(t, "SN-002", ts.repo.Movements[inventoryStockProductID][0].SerialNumber)
}

func TestService_ImportStockAdjustmentsCSVReportsInvalidUUIDReferences(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	result, err := ts.svc.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
		FileName: "stock.csv",
		UserID:   "user-1",
		CSVContent: "product_id,warehouse_id,quantity\n" +
			"legacy-product,11111111-1111-4111-8111-111111111111,1\n" +
			"11111111-1111-4111-8111-111111111111,legacy-warehouse,1\n",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Zero(t, result.AdjustmentsImported)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "product_id must be a valid UUID")
	assert.Contains(t, result.Errors[1].Message, "warehouse_id must be a valid UUID")
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

func TestService_CreateProduct_InvalidReferenceID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	tests := []struct {
		name    string
		mutate  func(*CreateProductRequest)
		wantErr string
	}{
		{
			name: "category",
			mutate: func(req *CreateProductRequest) {
				req.CategoryID = "legacy-category"
			},
			wantErr: "category_id must be a valid UUID",
		},
		{
			name: "sale account",
			mutate: func(req *CreateProductRequest) {
				req.SaleAccountID = "legacy-account"
			},
			wantErr: "sale_account_id must be a valid UUID",
		},
		{
			name: "purchase account",
			mutate: func(req *CreateProductRequest) {
				req.PurchaseAccountID = "legacy-account"
			},
			wantErr: "purchase_account_id must be a valid UUID",
		},
		{
			name: "inventory account",
			mutate: func(req *CreateProductRequest) {
				req.InventoryAccountID = "legacy-account"
			},
			wantErr: "inventory_account_id must be a valid UUID",
		},
		{
			name: "supplier",
			mutate: func(req *CreateProductRequest) {
				req.SupplierID = "legacy-supplier"
			},
			wantErr: "supplier_id must be a valid UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CreateProductRequest{
				Name:       "Bad Reference",
				SalesPrice: "100",
			}
			tt.mutate(req)

			_, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

	_, err = ts.svc.ListProducts(ctx, "tenant-1", "test_schema", &ProductFilter{CategoryID: "legacy-category"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category_id must be a valid UUID")
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
		CategoryID: "11111111-1111-4111-8111-111111111111",
		SalesPrice: "150",
		SupplierID: "22222222-2222-4222-8222-222222222222",
		IsActive:   true,
	}

	product, err := ts.svc.UpdateProduct(ctx, "tenant-1", "test_schema", "p1", req)
	require.NoError(t, err)
	assert.Equal(t, "Updated", product.Name)
	assert.Equal(t, "11111111-1111-4111-8111-111111111111", product.CategoryID)
	assert.Equal(t, "22222222-2222-4222-8222-222222222222", product.SupplierID)
	assert.True(t, product.SalesPrice.Equal(decimal.NewFromInt(150)))

	for _, tt := range []struct {
		name    string
		mutate  func(*UpdateProductRequest)
		wantErr string
	}{
		{
			name: "category",
			mutate: func(req *UpdateProductRequest) {
				req.CategoryID = "legacy-category"
			},
			wantErr: "category_id must be a valid UUID",
		},
		{
			name: "sale account",
			mutate: func(req *UpdateProductRequest) {
				req.SaleAccountID = "legacy-account"
			},
			wantErr: "sale_account_id must be a valid UUID",
		},
		{
			name: "purchase account",
			mutate: func(req *UpdateProductRequest) {
				req.PurchaseAccountID = "legacy-account"
			},
			wantErr: "purchase_account_id must be a valid UUID",
		},
		{
			name: "inventory account",
			mutate: func(req *UpdateProductRequest) {
				req.InventoryAccountID = "legacy-account"
			},
			wantErr: "inventory_account_id must be a valid UUID",
		},
		{
			name: "supplier",
			mutate: func(req *UpdateProductRequest) {
				req.SupplierID = "legacy-supplier"
			},
			wantErr: "supplier_id must be a valid UUID",
		},
	} {
		t.Run("invalid reference "+tt.name, func(t *testing.T) {
			req := &UpdateProductRequest{
				Name:       "Updated",
				SalesPrice: "150",
				IsActive:   true,
			}
			tt.mutate(req)

			_, err := ts.svc.UpdateProduct(ctx, "tenant-1", "test_schema", "p1", req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

	parentID := "11111111-1111-1111-1111-111111111111"
	req := &CreateCategoryRequest{
		Name:        "Electronics",
		Description: "Electronic products",
		ParentID:    " " + parentID + " ",
	}

	cat, err := ts.svc.CreateCategory(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.NotEmpty(t, cat.ID)
	assert.Equal(t, "Electronics", cat.Name)
	assert.Equal(t, parentID, cat.ParentID)

	_, err = ts.svc.CreateCategory(ctx, "tenant-1", "test_schema", &CreateCategoryRequest{
		Name:     "Bad parent",
		ParentID: "legacy-parent",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parent_id must be a valid UUID")
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

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(100),
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}

	req := &AdjustStockRequest{
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     "50",
		UnitCost:     "10",
		LotNumber:    " LOT-2026-01 ",
		SerialNumber: " SN-001 ",
		ExpiryDate:   "2027-01-31",
		Reason:       "Received shipment",
		UserID:       "user-1",
	}

	movement, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, MovementTypeIn, movement.MovementType)
	assert.True(t, movement.Quantity.Equal(decimal.NewFromInt(50)))
	assert.Equal(t, "LOT-2026-01", movement.LotNumber)
	assert.Equal(t, "SN-001", movement.SerialNumber)
	assert.Equal(t, "2027-01-31", movement.ExpiryDate)

	// Check product stock updated
	product, _ := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(150)))

	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.NewFromInt(50)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(50)))
}

func TestService_AdjustStock_Negative(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("7.00"),
		CurrentStock:   decimal.NewFromInt(100),
		TrackInventory: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(40),
		ReservedQty:  decimal.NewFromInt(5),
		AvailableQty: decimal.NewFromInt(35),
	}

	req := &AdjustStockRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "-30",
		Reason:      "Damaged goods",
		UserID:      "user-1",
	}

	movement, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)
	assert.Equal(t, MovementTypeOut, movement.MovementType)
	assert.True(t, movement.Quantity.Equal(decimal.NewFromInt(30))) // Absolute value

	// Check product stock updated
	product, _ := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(70)))

	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.NewFromInt(10)))
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(5)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(5)))
}

func TestService_AdjustStock_WarehouseStockCannotGoNegative(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(100),
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}

	_, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", &AdjustStockRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "-1",
		UserID:      "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "warehouse stock negative")
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

func TestService_AdjustStock_InvalidExpiryDate(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	_, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", &AdjustStockRequest{
		ProductID:   "p1",
		WarehouseID: "wh-1",
		Quantity:    "1",
		ExpiryDate:  "31-01-2027",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expiry_date must use YYYY-MM-DD")
}

func TestService_AdjustStock_InvalidReferenceID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	for _, tt := range []struct {
		name    string
		req     AdjustStockRequest
		wantErr string
	}{
		{
			name: "product",
			req: AdjustStockRequest{
				ProductID:   "legacy-product",
				WarehouseID: inventoryStockWarehouseID,
				Quantity:    "1",
			},
			wantErr: "product_id must be a valid UUID",
		},
		{
			name: "warehouse",
			req: AdjustStockRequest{
				ProductID:   inventoryStockProductID,
				WarehouseID: "legacy-warehouse",
				Quantity:    "1",
			},
			wantErr: "warehouse_id must be a valid UUID",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", &tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestService_TransferStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(100),
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID2] = &Warehouse{
		ID:       inventoryStockWarehouseID2,
		TenantID: "tenant-1",
		Name:     "Branch",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(60),
		ReservedQty:  decimal.NewFromInt(10),
		AvailableQty: decimal.NewFromInt(50),
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID2)] = &StockLevel{
		ID:           "sl-2",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID2,
		Quantity:     decimal.NewFromInt(40),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.NewFromInt(40),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "mov-lot-receipt",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(60),
			UnitCost:     decimal.RequireFromString("8.25"),
			TotalCost:    decimal.RequireFromString("495.00"),
			LotNumber:    "LOT-2026-01",
			SerialNumber: "SN-001",
			ExpiryDate:   "2027-01-31",
		},
	}

	req := &TransferStockRequest{
		ProductID:       inventoryStockProductID,
		FromWarehouseID: inventoryStockWarehouseID,
		ToWarehouseID:   inventoryStockWarehouseID2,
		Quantity:        "25",
		LotNumber:       " LOT-2026-01 ",
		SerialNumber:    " SN-001 ",
		ExpiryDate:      " 2027-01-31 ",
		Notes:           "Transfer between warehouses",
		UserID:          "user-1",
	}

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", req)
	require.NoError(t, err)

	// Check movements created
	movements := ts.repo.Movements[inventoryStockProductID]
	require.Len(t, movements, 3) // Existing receipt plus OUT and IN movements
	transferMovements := movements[1:]
	assert.Equal(t, MovementTypeOut, transferMovements[0].MovementType)
	assert.Equal(t, MovementTypeIn, transferMovements[1].MovementType)
	for _, movement := range transferMovements {
		assert.Equal(t, "LOT-2026-01", movement.LotNumber)
		assert.Equal(t, "SN-001", movement.SerialNumber)
		assert.Equal(t, "2027-01-31", movement.ExpiryDate)
		assert.True(t, movement.UnitCost.Equal(decimal.RequireFromString("8.25")))
		assert.True(t, movement.TotalCost.Equal(decimal.RequireFromString("206.25")))
	}

	sourceLevel := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, sourceLevel)
	assert.True(t, sourceLevel.Quantity.Equal(decimal.NewFromInt(35)))
	assert.True(t, sourceLevel.ReservedQty.Equal(decimal.NewFromInt(10)))
	assert.True(t, sourceLevel.AvailableQty.Equal(decimal.NewFromInt(25)))

	destinationLevel := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID2)]
	require.NotNil(t, destinationLevel)
	assert.True(t, destinationLevel.Quantity.Equal(decimal.NewFromInt(65)))
	assert.True(t, destinationLevel.AvailableQty.Equal(decimal.NewFromInt(65)))

	product, err := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	require.NoError(t, err)
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(100)))
}

func TestService_TransferStockRejectsInsufficientTrackedLotQuantity(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("7.00"),
		CurrentStock:   decimal.NewFromInt(10),
		TrackInventory: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID2] = &Warehouse{
		ID:       inventoryStockWarehouseID2,
		TenantID: "tenant-1",
		Name:     "Branch",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(10),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.NewFromInt(10),
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID2)] = &StockLevel{
		ID:           "sl-2",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID2,
		Quantity:     decimal.Zero,
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.Zero,
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "mov-small-lot",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(2),
			UnitCost:     decimal.RequireFromString("8.25"),
			TotalCost:    decimal.RequireFromString("16.50"),
			LotNumber:    "LOT-2026-01",
		},
	}

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", &TransferStockRequest{
		ProductID:       inventoryStockProductID,
		FromWarehouseID: inventoryStockWarehouseID,
		ToWarehouseID:   inventoryStockWarehouseID2,
		Quantity:        "3",
		LotNumber:       "LOT-2026-01",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient tracked lot stock in source warehouse")
	assert.Len(t, ts.repo.Movements[inventoryStockProductID], 1)
	sourceLevel := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	assert.True(t, sourceLevel.Quantity.Equal(decimal.NewFromInt(10)))
	destinationLevel := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID2)]
	assert.True(t, destinationLevel.Quantity.IsZero())
}

func TestService_IssueStockConsumesSpecificTrackedLotWithAccountingLines(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	cogsAccountID := "44444444-4444-4444-8444-444444444444"
	inventoryAccountID := "55555555-5555-4555-8555-555555555555"
	ts.svc.accounts = fakeInventoryAccountLister{accounts: []accounting.Account{
		{ID: cogsAccountID, AccountType: accounting.AccountTypeExpense},
		{ID: inventoryAccountID, AccountType: accounting.AccountTypeAsset},
	}}

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:                 inventoryStockProductID,
		TenantID:           "tenant-1",
		Code:               "SKU-001",
		Name:               "Widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("7.00"),
		CurrentStock:       decimal.RequireFromString("12.00"),
		TrackInventory:     true,
		InventoryAccountID: inventoryAccountID,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("12.00"),
		ReservedQty:  decimal.RequireFromString("2.00"),
		AvailableQty: decimal.RequireFromString("10.00"),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "mov-lot-in",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("12.00"),
			UnitCost:     decimal.RequireFromString("8.25"),
			TotalCost:    decimal.RequireFromString("99.00"),
			LotNumber:    "LOT-2026-01",
			SerialNumber: "SN-001",
			ExpiryDate:   "2027-01-31",
		},
	}
	ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2026-01", "SN-001", "2027-01-31")] = &InventoryLotReservation{
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		LotNumber:    "LOT-2026-01",
		SerialNumber: "SN-001",
		ExpiryDate:   "2027-01-31",
		Quantity:     decimal.RequireFromString("2.00"),
	}

	result, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:                inventoryStockProductID,
		WarehouseID:              inventoryStockWarehouseID,
		Quantity:                 "3",
		LotNumber:                " LOT-2026-01 ",
		SerialNumber:             " SN-001 ",
		ExpiryDate:               " 2027-01-31 ",
		Reference:                "Invoice INV-001",
		SourceType:               "SALES_INVOICE",
		SourceID:                 "66666666-6666-4666-8666-666666666666",
		Reason:                   "Shipped goods",
		CostOfGoodsSoldAccountID: cogsAccountID,
		UserID:                   "user-1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Quantity.Equal(decimal.RequireFromString("3")))
	assert.True(t, result.UnitCost.Equal(decimal.RequireFromString("8.25")))
	assert.True(t, result.TotalCost.Equal(decimal.RequireFromString("24.75")))
	require.Len(t, result.Movements, 1)
	assert.Equal(t, MovementTypeOut, result.Movements[0].MovementType)
	assert.Equal(t, "LOT-2026-01", result.Movements[0].LotNumber)
	assert.Equal(t, "SALES_INVOICE", result.Movements[0].SourceType)
	assert.Equal(t, "66666666-6666-4666-8666-666666666666", result.Movements[0].SourceID)
	require.NotNil(t, result.Accounting)
	require.Len(t, result.Accounting.Lines, 2)
	assert.Equal(t, inventoryIssueAccountingRoleCOGS, result.Accounting.Lines[0].Role)
	assert.Equal(t, cogsAccountID, result.Accounting.Lines[0].AccountID)
	assert.True(t, result.Accounting.Lines[0].DebitAmount.Equal(decimal.RequireFromString("24.75")))
	assert.Equal(t, inventoryIssueAccountingRoleAsset, result.Accounting.Lines[1].Role)
	assert.Equal(t, inventoryAccountID, result.Accounting.Lines[1].AccountID)
	assert.True(t, result.Accounting.Lines[1].CreditAmount.Equal(decimal.RequireFromString("24.75")))

	product, err := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	require.NoError(t, err)
	assert.True(t, product.CurrentStock.Equal(decimal.RequireFromString("9.00")))
	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.RequireFromString("9.00")))
	assert.True(t, level.ReservedQty.Equal(decimal.RequireFromString("2.00")))
	assert.True(t, level.AvailableQty.Equal(decimal.RequireFromString("7.00")))
}

func TestService_IssueStockPostsLedgerEntryWhenRequested(t *testing.T) {
	repo := NewMockRepository()
	ledger := &fakeInventoryLedger{accounts: []accounting.Account{
		{ID: "44444444-4444-4444-8444-444444444444", AccountType: accounting.AccountTypeExpense},
		{ID: "55555555-5555-4555-8555-555555555555", AccountType: accounting.AccountTypeAsset},
	}}
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	repo.Products[inventoryStockProductID] = &Product{
		ID:                 inventoryStockProductID,
		TenantID:           "tenant-1",
		Code:               "SKU-001",
		Name:               "Widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("6.00"),
		CurrentStock:       decimal.RequireFromString("5.00"),
		TrackInventory:     true,
		InventoryAccountID: "55555555-5555-4555-8555-555555555555",
	}
	repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("5.00"),
		AvailableQty: decimal.RequireFromString("5.00"),
	}
	repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "lot-in",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("5.00"),
			UnitCost:     decimal.RequireFromString("6.50"),
			TotalCost:    decimal.RequireFromString("32.50"),
			LotNumber:    "LOT-POST",
		},
	}

	result, err := svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:                inventoryStockProductID,
		WarehouseID:              inventoryStockWarehouseID,
		Quantity:                 "2",
		LotNumber:                "LOT-POST",
		Reference:                "Invoice INV-002",
		Reason:                   "Shipment",
		CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
		PostToLedger:             true,
		UserID:                   "user-1",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Accounting)
	assert.True(t, result.Accounting.Posted)
	assert.Equal(t, "journal-1", result.Accounting.JournalID)
	assert.Equal(t, "JE-00001", result.Accounting.JournalNo)
	require.Len(t, result.Movements, 1)
	assert.Equal(t, result.Accounting.SourceID, result.Movements[0].SourceID)
	assert.NotEmpty(t, result.Accounting.SourceID)
	_, err = uuid.Parse(result.Accounting.SourceID)
	require.NoError(t, err)

	require.NotNil(t, ledger.createdRequest)
	assert.Equal(t, inventoryIssueSourceTypeDefault, ledger.createdRequest.SourceType)
	require.NotNil(t, ledger.createdRequest.SourceID)
	assert.Equal(t, result.Accounting.SourceID, *ledger.createdRequest.SourceID)
	assert.Equal(t, "Invoice INV-002", ledger.createdRequest.Reference)
	require.Len(t, ledger.createdRequest.Lines, 2)
	assert.Equal(t, "44444444-4444-4444-8444-444444444444", ledger.createdRequest.Lines[0].AccountID)
	assert.True(t, ledger.createdRequest.Lines[0].DebitAmount.Equal(decimal.RequireFromString("13.00")))
	assert.Equal(t, "55555555-5555-4555-8555-555555555555", ledger.createdRequest.Lines[1].AccountID)
	assert.True(t, ledger.createdRequest.Lines[1].CreditAmount.Equal(decimal.RequireFromString("13.00")))
	assert.Equal(t, []string{"journal-1"}, ledger.postedIDs)
}

func TestService_IssueStockRollsBackInventoryWhenLedgerPostingFails(t *testing.T) {
	repo := NewMockRepository()
	ledger := &fakeInventoryLedger{
		accounts: []accounting.Account{
			{ID: "44444444-4444-4444-8444-444444444444", AccountType: accounting.AccountTypeExpense},
			{ID: "55555555-5555-4555-8555-555555555555", AccountType: accounting.AccountTypeAsset},
		},
		postErr: fmt.Errorf("ledger unavailable"),
	}
	svc := NewServiceWithRepositoryAndAccounting(repo, ledger)
	ctx := context.Background()

	repo.Products[inventoryStockProductID] = &Product{
		ID:                 inventoryStockProductID,
		TenantID:           "tenant-1",
		Code:               "SKU-001",
		Name:               "Widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("6.00"),
		CurrentStock:       decimal.RequireFromString("5.00"),
		TrackInventory:     true,
		InventoryAccountID: "55555555-5555-4555-8555-555555555555",
	}
	repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("5.00"),
		AvailableQty: decimal.RequireFromString("5.00"),
	}
	repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "lot-in",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("5.00"),
			UnitCost:     decimal.RequireFromString("6.50"),
			TotalCost:    decimal.RequireFromString("32.50"),
			LotNumber:    "LOT-POST",
		},
	}

	result, err := svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:                inventoryStockProductID,
		WarehouseID:              inventoryStockWarehouseID,
		Quantity:                 "2",
		LotNumber:                "LOT-POST",
		Reference:                "Invoice INV-003",
		Reason:                   "Shipment",
		CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
		PostToLedger:             true,
		UserID:                   "user-1",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "post inventory issue journal entry")
	assert.ErrorContains(t, err, "ledger unavailable")
	assert.Nil(t, result)
	require.NotNil(t, ledger.createdRequest)
	assert.Empty(t, ledger.postedIDs)

	product, err := repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	require.NoError(t, err)
	assert.True(t, product.CurrentStock.Equal(decimal.RequireFromString("5.00")))
	level := repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, level.AvailableQty.Equal(decimal.RequireFromString("5.00")))
	require.Len(t, repo.Movements[inventoryStockProductID], 1)
	assert.Equal(t, "lot-in", repo.Movements[inventoryStockProductID][0].ID)
}

func TestService_IssueStockAutoAllocatesTrackedLots(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Code:           "SKU-001",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("10.00"),
		CurrentStock:   decimal.RequireFromString("8.00"),
		TrackInventory: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("8.00"),
		ReservedQty:  decimal.RequireFromString("1.00"),
		AvailableQty: decimal.RequireFromString("7.00"),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "lot-b",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("4.00"),
			UnitCost:     decimal.RequireFromString("7.00"),
			TotalCost:    decimal.RequireFromString("28.00"),
			LotNumber:    "LOT-B",
			ExpiryDate:   "2027-05-31",
		},
		{
			ID:           "lot-a",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("4.00"),
			UnitCost:     decimal.RequireFromString("5.00"),
			TotalCost:    decimal.RequireFromString("20.00"),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
		},
	}
	ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "2027-01-31")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-A",
		ExpiryDate:  "2027-01-31",
		Quantity:    decimal.RequireFromString("1.00"),
	}

	result, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "5",
		Reference:   "Shipment",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Movements, 2)
	assert.Equal(t, "LOT-A", result.Movements[0].LotNumber)
	assert.True(t, result.Movements[0].Quantity.Equal(decimal.RequireFromString("3.00")))
	assert.True(t, result.Movements[0].UnitCost.Equal(decimal.RequireFromString("5.00")))
	assert.Equal(t, "LOT-B", result.Movements[1].LotNumber)
	assert.True(t, result.Movements[1].Quantity.Equal(decimal.RequireFromString("2.00")))
	assert.True(t, result.Movements[1].UnitCost.Equal(decimal.RequireFromString("7.00")))
	assert.True(t, result.TotalCost.Equal(decimal.RequireFromString("29.00")))
	assert.Nil(t, result.Accounting)
	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	require.NotNil(t, level)
	assert.True(t, level.Quantity.Equal(decimal.RequireFromString("3.00")))
	assert.True(t, level.ReservedQty.Equal(decimal.RequireFromString("1.00")))
	assert.True(t, level.AvailableQty.Equal(decimal.RequireFromString("2.00")))
}

func TestService_IssueStockSupportsCostingMethods(t *testing.T) {
	ctx := context.Background()
	seed := func() *testService {
		ts := newTestService()
		ts.repo.Products[inventoryStockProductID] = &Product{
			ID:             inventoryStockProductID,
			TenantID:       "tenant-1",
			Code:           "SKU-001",
			Name:           "Widget",
			ProductType:    ProductTypeGoods,
			PurchasePrice:  decimal.RequireFromString("9.00"),
			CurrentStock:   decimal.RequireFromString("8.00"),
			TrackInventory: true,
		}
		ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
		ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
			ID:           "sl-1",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			Quantity:     decimal.RequireFromString("8.00"),
			AvailableQty: decimal.RequireFromString("8.00"),
		}
		ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
			{
				ID:           "lot-a",
				TenantID:     "tenant-1",
				ProductID:    inventoryStockProductID,
				WarehouseID:  inventoryStockWarehouseID,
				MovementType: MovementTypeIn,
				Quantity:     decimal.RequireFromString("4.00"),
				UnitCost:     decimal.RequireFromString("5.00"),
				TotalCost:    decimal.RequireFromString("20.00"),
				LotNumber:    "LOT-A",
				ExpiryDate:   "2027-01-31",
			},
			{
				ID:           "lot-b",
				TenantID:     "tenant-1",
				ProductID:    inventoryStockProductID,
				WarehouseID:  inventoryStockWarehouseID,
				MovementType: MovementTypeIn,
				Quantity:     decimal.RequireFromString("4.00"),
				UnitCost:     decimal.RequireFromString("7.00"),
				TotalCost:    decimal.RequireFromString("28.00"),
				LotNumber:    "LOT-B",
				ExpiryDate:   "2027-05-31",
			},
		}
		return ts
	}

	for _, tc := range []struct {
		name          string
		method        string
		wantMethod    string
		wantUnitCost  string
		wantTotalCost string
	}{
		{name: "default lot layer cost", method: "", wantMethod: InventoryIssueCostingMethodLot, wantUnitCost: "5.4", wantTotalCost: "27.00"},
		{name: "weighted average", method: "weighted-average", wantMethod: InventoryIssueCostingMethodWeightedAverage, wantUnitCost: "6", wantTotalCost: "30.00"},
		{name: "standard cost", method: "standard-cost", wantMethod: InventoryIssueCostingMethodStandardCost, wantUnitCost: "9", wantTotalCost: "45.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := seed()
			result, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
				ProductID:     inventoryStockProductID,
				WarehouseID:   inventoryStockWarehouseID,
				Quantity:      "5",
				CostingMethod: tc.method,
				Reference:     "Shipment",
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantMethod, result.CostingMethod)
			require.Len(t, result.Movements, 2)
			assert.Equal(t, "LOT-A", result.Movements[0].LotNumber)
			assert.Equal(t, "LOT-B", result.Movements[1].LotNumber)
			assert.True(t, result.UnitCost.Equal(decimal.RequireFromString(tc.wantUnitCost)))
			assert.True(t, result.TotalCost.Equal(decimal.RequireFromString(tc.wantTotalCost)))
		})
	}

	ts := seed()
	_, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:     inventoryStockProductID,
		WarehouseID:   inventoryStockWarehouseID,
		Quantity:      "1",
		CostingMethod: "lifo",
		Reference:     "Shipment",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid costing_method")
}

func TestService_IssueStockRejectsReservedTrackedLotAndBadAccountingAccount(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()
	cogsAccountID := "44444444-4444-4444-8444-444444444444"
	inventoryAccountID := "55555555-5555-4555-8555-555555555555"
	ts.svc.accounts = fakeInventoryAccountLister{accounts: []accounting.Account{
		{ID: cogsAccountID, AccountType: accounting.AccountTypeAsset},
		{ID: inventoryAccountID, AccountType: accounting.AccountTypeAsset},
	}}

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:                 inventoryStockProductID,
		TenantID:           "tenant-1",
		Name:               "Widget",
		ProductType:        ProductTypeGoods,
		PurchasePrice:      decimal.RequireFromString("8.00"),
		CurrentStock:       decimal.RequireFromString("3.00"),
		TrackInventory:     true,
		InventoryAccountID: inventoryAccountID,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.RequireFromString("3.00"),
		ReservedQty:  decimal.RequireFromString("2.00"),
		AvailableQty: decimal.RequireFromString("1.00"),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "lot-a",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("3.00"),
			UnitCost:     decimal.RequireFromString("8.00"),
			TotalCost:    decimal.RequireFromString("24.00"),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
		},
	}
	ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "2027-01-31")] = &InventoryLotReservation{
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-A",
		ExpiryDate:  "2027-01-31",
		Quantity:    decimal.RequireFromString("2.00"),
	}

	_, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "2",
		LotNumber:   "LOT-A",
		ExpiryDate:  "2027-01-31",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient available")

	_, err = ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
		ProductID:                inventoryStockProductID,
		WarehouseID:              inventoryStockWarehouseID,
		Quantity:                 "1",
		LotNumber:                "LOT-A",
		ExpiryDate:               "2027-01-31",
		CostOfGoodsSoldAccountID: cogsAccountID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cost_of_goods_sold_account_id must reference an EXPENSE account")
}

func TestService_ReserveStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(20),
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(20),
		ReservedQty:  decimal.NewFromInt(5),
		AvailableQty: decimal.NewFromInt(15),
	}

	level, err := ts.svc.ReserveStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "4",
		Reason:      "Sales order allocation",
		UserID:      "user-1",
	})
	require.NoError(t, err)
	assert.True(t, level.Quantity.Equal(decimal.NewFromInt(20)))
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(9)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(11)))
	assert.Empty(t, ts.repo.Movements[inventoryStockProductID])

	product, err := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	require.NoError(t, err)
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(20)))
}

func TestService_ReserveStockTracksSpecificLot(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		CurrentStock:   decimal.NewFromInt(10),
		TrackInventory: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(10),
		ReservedQty:  decimal.NewFromInt(3),
		AvailableQty: decimal.NewFromInt(7),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "mov-lot-a",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(10),
			UnitCost:     decimal.RequireFromString("8.25"),
			TotalCost:    decimal.RequireFromString("82.50"),
			LotNumber:    "LOT-2026-01",
			ExpiryDate:   "2027-01-31",
		},
	}
	ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2026-01", "", "2027-01-31")] = &InventoryLotReservation{
		ID:          "lot-res-1",
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-2026-01",
		ExpiryDate:  "2027-01-31",
		Quantity:    decimal.NewFromInt(3),
	}

	level, err := ts.svc.ReserveStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "4",
		LotNumber:   "LOT-2026-01",
		ExpiryDate:  "2027-01-31",
		Reason:      "Sales order allocation",
		UserID:      "user-1",
	})

	require.NoError(t, err)
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(7)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(3)))
	reservation := ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2026-01", "", "2027-01-31")]
	require.NotNil(t, reservation)
	assert.True(t, reservation.Quantity.Equal(decimal.NewFromInt(7)))
	assert.Equal(t, "Sales order allocation", reservation.Reason)
}

func TestService_ReserveStockRejectsInsufficientTrackedLotAvailability(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		CurrentStock:   decimal.NewFromInt(8),
		TrackInventory: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(8),
		ReservedQty:  decimal.NewFromInt(4),
		AvailableQty: decimal.NewFromInt(4),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "mov-lot-a",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(5),
			LotNumber:    "LOT-2026-01",
		},
	}

	_, err := ts.svc.ReserveStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "2",
		LotNumber:   "LOT-2026-01",
		UserID:      "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient available tracked lot stock")
	assert.Empty(t, ts.repo.LotReservations)
	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(4)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(4)))
}

func TestService_ReserveStockAutoAllocatesTrackedLots(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		CurrentStock:   decimal.NewFromInt(10),
		TrackInventory: true,
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(10),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.NewFromInt(10),
	}
	ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
		{
			ID:           "mov-late",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(5),
			LotNumber:    "LOT-B",
			ExpiryDate:   "2027-06-30",
		},
		{
			ID:           "mov-early",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(5),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
		},
	}

	_, err := ts.svc.ReserveStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "7",
		Reason:      "Pick list",
		UserID:      "user-1",
	})

	require.NoError(t, err)
	early := ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "2027-01-31")]
	late := ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-B", "", "2027-06-30")]
	require.NotNil(t, early)
	require.NotNil(t, late)
	assert.True(t, early.Quantity.Equal(decimal.NewFromInt(5)))
	assert.True(t, late.Quantity.Equal(decimal.NewFromInt(2)))
}

func TestService_ReserveStock_InsufficientAvailableStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{ID: inventoryStockProductID, TenantID: "tenant-1", Name: "Widget"}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(8),
		ReservedQty:  decimal.NewFromInt(6),
		AvailableQty: decimal.NewFromInt(2),
	}

	_, err := ts.svc.ReserveStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "3",
		UserID:      "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient available stock")

	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(6)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(2)))
}

func TestService_ReleaseStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(20),
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(20),
		ReservedQty:  decimal.NewFromInt(8),
		AvailableQty: decimal.NewFromInt(12),
	}

	level, err := ts.svc.ReleaseStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "5",
		Reason:      "Order canceled",
		UserID:      "user-1",
	})
	require.NoError(t, err)
	assert.True(t, level.Quantity.Equal(decimal.NewFromInt(20)))
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(3)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(17)))
	assert.Empty(t, ts.repo.Movements[inventoryStockProductID])

	product, err := ts.repo.GetProductByID(ctx, "test_schema", "tenant-1", inventoryStockProductID)
	require.NoError(t, err)
	assert.True(t, product.CurrentStock.Equal(decimal.NewFromInt(20)))
}

func TestService_ReleaseStockReleasesTrackedLot(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{
		ID:           inventoryStockProductID,
		TenantID:     "tenant-1",
		Name:         "Widget",
		CurrentStock: decimal.NewFromInt(20),
	}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{
		ID:       inventoryStockWarehouseID,
		TenantID: "tenant-1",
		Name:     "Main",
		IsActive: true,
	}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(20),
		ReservedQty:  decimal.NewFromInt(8),
		AvailableQty: decimal.NewFromInt(12),
	}
	ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2026-01", "", "2027-01-31")] = &InventoryLotReservation{
		ID:          "lot-res-1",
		TenantID:    "tenant-1",
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		LotNumber:   "LOT-2026-01",
		ExpiryDate:  "2027-01-31",
		Quantity:    decimal.NewFromInt(6),
	}

	level, err := ts.svc.ReleaseStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "4",
		LotNumber:   "LOT-2026-01",
		ExpiryDate:  "2027-01-31",
		Reason:      "Order canceled",
		UserID:      "user-1",
	})

	require.NoError(t, err)
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(4)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(16)))
	reservation := ts.repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-2026-01", "", "2027-01-31")]
	assert.True(t, reservation.Quantity.Equal(decimal.NewFromInt(2)))
}

func TestService_ReleaseStock_TooMuch(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{ID: inventoryStockProductID, TenantID: "tenant-1", Name: "Widget"}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(8),
		ReservedQty:  decimal.NewFromInt(2),
		AvailableQty: decimal.NewFromInt(6),
	}

	_, err := ts.svc.ReleaseStock(ctx, "tenant-1", "test_schema", &StockReservationRequest{
		ProductID:   inventoryStockProductID,
		WarehouseID: inventoryStockWarehouseID,
		Quantity:    "3",
		UserID:      "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot release more than reserved stock")

	level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
	assert.True(t, level.ReservedQty.Equal(decimal.NewFromInt(2)))
	assert.True(t, level.AvailableQty.Equal(decimal.NewFromInt(6)))
}

func TestService_TransferStock_InsufficientSourceStock(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products[inventoryStockProductID] = &Product{ID: inventoryStockProductID, TenantID: "tenant-1", Name: "Widget"}
	ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
	ts.repo.Warehouses[inventoryStockWarehouseID2] = &Warehouse{ID: inventoryStockWarehouseID2, TenantID: "tenant-1", Name: "Branch", IsActive: true}
	ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		ID:           "sl-1",
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(10),
		ReservedQty:  decimal.NewFromInt(4),
		AvailableQty: decimal.NewFromInt(6),
	}

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", &TransferStockRequest{
		ProductID:       inventoryStockProductID,
		FromWarehouseID: inventoryStockWarehouseID,
		ToWarehouseID:   inventoryStockWarehouseID2,
		Quantity:        "7",
		UserID:          "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient available stock")
	assert.Empty(t, ts.repo.Movements[inventoryStockProductID])
}

func TestService_TransferStock_InvalidExpiryDate(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", &TransferStockRequest{
		ProductID:       inventoryStockProductID,
		FromWarehouseID: inventoryStockWarehouseID,
		ToWarehouseID:   inventoryStockWarehouseID2,
		Quantity:        "5",
		ExpiryDate:      "2027/01/31",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expiry_date must use YYYY-MM-DD")
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

func TestService_TransferStock_InvalidReferenceID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	for _, tt := range []struct {
		name    string
		req     TransferStockRequest
		wantErr string
	}{
		{
			name: "product",
			req: TransferStockRequest{
				ProductID:       "legacy-product",
				FromWarehouseID: inventoryStockWarehouseID,
				ToWarehouseID:   inventoryStockWarehouseID2,
				Quantity:        "1",
			},
			wantErr: "product_id must be a valid UUID",
		},
		{
			name: "source warehouse",
			req: TransferStockRequest{
				ProductID:       inventoryStockProductID,
				FromWarehouseID: "legacy-warehouse",
				ToWarehouseID:   inventoryStockWarehouseID2,
				Quantity:        "1",
			},
			wantErr: "from_warehouse_id must be a valid UUID",
		},
		{
			name: "destination warehouse",
			req: TransferStockRequest{
				ProductID:       inventoryStockProductID,
				FromWarehouseID: inventoryStockWarehouseID,
				ToWarehouseID:   "legacy-warehouse",
				Quantity:        "1",
			},
			wantErr: "to_warehouse_id must be a valid UUID",
		},
		{
			name: "same warehouse",
			req: TransferStockRequest{
				ProductID:       inventoryStockProductID,
				FromWarehouseID: inventoryStockWarehouseID,
				ToWarehouseID:   " " + inventoryStockWarehouseID + " ",
				Quantity:        "1",
			},
			wantErr: "source and destination warehouses must differ",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", &tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestService_StockReservation_InvalidReferenceID(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	for _, tt := range []struct {
		name    string
		req     StockReservationRequest
		call    func(context.Context, string, string, *StockReservationRequest) (*StockLevel, error)
		wantErr string
	}{
		{
			name: "reserve product",
			req: StockReservationRequest{
				ProductID:   "legacy-product",
				WarehouseID: inventoryStockWarehouseID,
				Quantity:    "1",
			},
			call:    ts.svc.ReserveStock,
			wantErr: "product_id must be a valid UUID",
		},
		{
			name: "reserve warehouse",
			req: StockReservationRequest{
				ProductID:   inventoryStockProductID,
				WarehouseID: "legacy-warehouse",
				Quantity:    "1",
			},
			call:    ts.svc.ReserveStock,
			wantErr: "warehouse_id must be a valid UUID",
		},
		{
			name: "release product",
			req: StockReservationRequest{
				ProductID:   "legacy-product",
				WarehouseID: inventoryStockWarehouseID,
				Quantity:    "1",
			},
			call:    ts.svc.ReleaseStock,
			wantErr: "product_id must be a valid UUID",
		},
		{
			name: "release warehouse",
			req: StockReservationRequest{
				ProductID:   inventoryStockProductID,
				WarehouseID: "legacy-warehouse",
				Quantity:    "1",
			},
			call:    ts.svc.ReleaseStock,
			wantErr: "warehouse_id must be a valid UUID",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call(ctx, "tenant-1", "test_schema", &tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

func TestService_GetInventoryValuation(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:             "p1",
		TenantID:       "tenant-1",
		Code:           "SKU-001",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("10.50"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Products["p2"] = &Product{
		ID:             "p2",
		TenantID:       "tenant-1",
		Code:           "SRV-001",
		Name:           "Consulting",
		ProductType:    ProductTypeService,
		PurchasePrice:  decimal.RequireFromString("99.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Products["p3"] = &Product{
		ID:             "p3",
		TenantID:       "tenant-1",
		Code:           "SKU-002",
		Name:           "Untracked part",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("2.00"),
		TrackInventory: false,
		IsActive:       true,
	}
	ts.repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main warehouse", IsActive: true}
	ts.repo.Warehouses["wh-2"] = &Warehouse{ID: "wh-2", TenantID: "tenant-1", Code: "BRANCH", Name: "Branch warehouse", IsActive: true}
	ts.repo.StockLevels["p1-wh-1"] = &StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    "p1",
		WarehouseID:  "wh-1",
		Quantity:     decimal.RequireFromString("12.00"),
		ReservedQty:  decimal.RequireFromString("2.00"),
		AvailableQty: decimal.RequireFromString("10.00"),
	}
	ts.repo.StockLevels["p1-wh-2"] = &StockLevel{
		ID:           "stock-2",
		TenantID:     "tenant-1",
		ProductID:    "p1",
		WarehouseID:  "wh-2",
		Quantity:     decimal.RequireFromString("3.00"),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.RequireFromString("3.00"),
	}
	oldReceiptDate := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	newReceiptDate := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	ts.repo.Movements["p1"] = []InventoryMovement{
		{
			ID:           "mov-1",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("8.00"),
			TotalCost:    decimal.RequireFromString("80.00"),
			MovementDate: oldReceiptDate,
		},
		{
			ID:           "mov-2",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("12.00"),
			TotalCost:    decimal.RequireFromString("120.00"),
			MovementDate: newReceiptDate,
		},
		{
			ID:           "mov-out",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			MovementType: MovementTypeOut,
			Quantity:     decimal.RequireFromString("2.00"),
			UnitCost:     decimal.RequireFromString("99.00"),
			TotalCost:    decimal.RequireFromString("198.00"),
		},
	}

	report, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", "")
	require.NoError(t, err)
	require.Len(t, report.Lines, 2)
	assert.Equal(t, InventoryValuationMethodStandardCost, report.ValuationMethod)
	assert.True(t, report.TotalQuantity.Equal(decimal.RequireFromString("15.00")))
	assert.True(t, report.TotalReserved.Equal(decimal.RequireFromString("2.00")))
	assert.True(t, report.TotalAvailable.Equal(decimal.RequireFromString("13.00")))
	assert.True(t, report.TotalValue.Equal(decimal.RequireFromString("157.5000")))
	assert.Equal(t, "SKU-001", report.Lines[0].ProductCode)
	assert.Equal(t, "BRANCH", report.Lines[0].WarehouseCode)
	assert.True(t, report.Lines[0].InventoryValue.Equal(decimal.RequireFromString("31.5000")))

	weighted, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", "weighted-average")
	require.NoError(t, err)
	require.Len(t, weighted.Lines, 2)
	assert.Equal(t, InventoryValuationMethodWeightedAverage, weighted.ValuationMethod)
	assert.True(t, weighted.Lines[0].UnitCost.Equal(decimal.RequireFromString("10.00")))
	assert.True(t, weighted.TotalValue.Equal(decimal.RequireFromString("150.00")))

	fifo, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", "fifo")
	require.NoError(t, err)
	require.Len(t, fifo.Lines, 2)
	assert.Equal(t, InventoryValuationMethodFIFO, fifo.ValuationMethod)
	assert.True(t, fifo.Lines[0].UnitCost.Round(4).Equal(decimal.RequireFromString("10.6667")))
	assert.True(t, fifo.TotalValue.Round(2).Equal(decimal.RequireFromString("160.00")))

	filtered, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "wh-1", "")
	require.NoError(t, err)
	require.Len(t, filtered.Lines, 1)
	assert.Equal(t, "wh-1", filtered.WarehouseID)
	assert.Equal(t, "MAIN", filtered.Lines[0].WarehouseCode)
	assert.True(t, filtered.TotalQuantity.Equal(decimal.RequireFromString("12.00")))
	assert.True(t, filtered.TotalValue.Equal(decimal.RequireFromString("126.0000")))

	_, err = ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", "lifo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid valuation method")
}

func TestService_GetInventoryValuationFallsBackToPurchasePriceWithoutCostLayers(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:             "p1",
		TenantID:       "tenant-1",
		Code:           "SKU-FALLBACK",
		Name:           "Fallback cost item",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("9.00"),
		CurrentStock:   decimal.RequireFromString("5.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Products["p2"] = &Product{
		ID:             "p2",
		TenantID:       "tenant-1",
		Code:           "SKU-ZERO",
		Name:           "Zero stock item",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("3.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main warehouse", IsActive: true}
	ts.repo.StockLevels["p2-wh-1"] = &StockLevel{
		ID:           "stock-zero",
		TenantID:     "tenant-1",
		ProductID:    "p2",
		WarehouseID:  "wh-1",
		Quantity:     decimal.Zero,
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.Zero,
	}

	weighted, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", "weighted-average")
	require.NoError(t, err)
	fallbackLine := findInventoryValuationLine(t, weighted, "p1", "")
	assert.Equal(t, "UNASSIGNED", fallbackLine.WarehouseCode)
	assert.True(t, fallbackLine.UnitCost.Equal(decimal.RequireFromString("9.00")))
	assert.True(t, fallbackLine.InventoryValue.Equal(decimal.RequireFromString("45.00")))

	fifo, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", "fifo")
	require.NoError(t, err)
	fifoFallbackLine := findInventoryValuationLine(t, fifo, "p1", "")
	assert.True(t, fifoFallbackLine.UnitCost.Equal(decimal.RequireFromString("9.00")))
	assert.True(t, fifoFallbackLine.InventoryValue.Equal(decimal.RequireFromString("45.00")))
	zeroLine := findInventoryValuationLine(t, fifo, "p2", "wh-1")
	assert.True(t, zeroLine.Quantity.IsZero())
	assert.True(t, zeroLine.UnitCost.Equal(decimal.RequireFromString("3.00")))
	assert.True(t, zeroLine.InventoryValue.IsZero())
}

func TestFIFOInventoryUnitCostUsesTotalCostLayersAndPurchaseFallback(t *testing.T) {
	product := Product{TenantID: "tenant-1", PurchasePrice: decimal.RequireFromString("9.00")}
	movements := []InventoryMovement{
		{
			TenantID:     "tenant-2",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("100.00"),
			UnitCost:     decimal.RequireFromString("1.00"),
		},
		{
			TenantID:     "tenant-1",
			MovementType: MovementTypeOut,
			Quantity:     decimal.RequireFromString("100.00"),
			UnitCost:     decimal.RequireFromString("1.00"),
		},
		{
			TenantID:     "tenant-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("2.00"),
			TotalCost:    decimal.RequireFromString("10.00"),
			MovementDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			TenantID:     "tenant-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("2.00"),
			MovementDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	unitCost := fifoInventoryUnitCost(product, movements, "tenant-1", decimal.RequireFromString("5.00"))
	assert.True(t, unitCost.Equal(decimal.RequireFromString("7.40")))
	assert.True(t, fifoInventoryUnitCost(product, movements, "tenant-1", decimal.Zero).Equal(decimal.RequireFromString("9.00")))
}

func TestService_GetInventoryLotReport(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:             "p1",
		TenantID:       "tenant-1",
		Code:           "SKU-001",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("6.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Products["p2"] = &Product{
		ID:             "p2",
		TenantID:       "tenant-1",
		Code:           "SRV-001",
		Name:           "Consulting",
		ProductType:    ProductTypeService,
		PurchasePrice:  decimal.RequireFromString("99.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Products["p3"] = &Product{
		ID:             "p3",
		TenantID:       "tenant-1",
		Code:           "SKU-002",
		Name:           "Untracked part",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("2.00"),
		TrackInventory: false,
		IsActive:       true,
	}
	ts.repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main warehouse", IsActive: true}
	ts.repo.Warehouses["wh-2"] = &Warehouse{ID: "wh-2", TenantID: "tenant-1", Code: "BRANCH", Name: "Branch warehouse", IsActive: true}

	lotReceiptDate := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	lotIssueDate := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	serialReceiptDate := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	ts.repo.Movements["p1"] = []InventoryMovement{
		{
			ID:           "mov-lot-in",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("5.00"),
			TotalCost:    decimal.RequireFromString("50.00"),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
			MovementDate: lotReceiptDate,
		},
		{
			ID:           "mov-lot-out",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeOut,
			Quantity:     decimal.RequireFromString("3.00"),
			LotNumber:    "LOT-A",
			ExpiryDate:   "2027-01-31",
			MovementDate: lotIssueDate,
		},
		{
			ID:           "mov-untracked-in",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("4.00"),
			UnitCost:     decimal.RequireFromString("4.00"),
			TotalCost:    decimal.RequireFromString("16.00"),
			MovementDate: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:           "mov-zero-in",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("2.00"),
			UnitCost:     decimal.RequireFromString("7.00"),
			TotalCost:    decimal.RequireFromString("14.00"),
			LotNumber:    "LOT-ZERO",
		},
		{
			ID:           "mov-zero-out",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeOut,
			Quantity:     decimal.RequireFromString("2.00"),
			LotNumber:    "LOT-ZERO",
		},
		{
			ID:           "mov-serial-in",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-2",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("1.00"),
			UnitCost:     decimal.RequireFromString("9.00"),
			TotalCost:    decimal.RequireFromString("9.00"),
			SerialNumber: "SN-001",
			MovementDate: serialReceiptDate,
		},
		{
			ID:           "mov-other-tenant",
			TenantID:     "tenant-2",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("100.00"),
			LotNumber:    "OTHER",
		},
	}
	ts.repo.Movements["p2"] = []InventoryMovement{
		{
			ID:           "mov-service",
			TenantID:     "tenant-1",
			ProductID:    "p2",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("100.00"),
			LotNumber:    "SERVICE",
		},
	}
	ts.repo.Movements["p3"] = []InventoryMovement{
		{
			ID:           "mov-untracked-product",
			TenantID:     "tenant-1",
			ProductID:    "p3",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("100.00"),
			LotNumber:    "UNTRACKED-PRODUCT",
		},
	}

	report, err := ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "", "", false)
	require.NoError(t, err)
	require.Len(t, report.Lines, 3)
	assert.Equal(t, "tenant-1", report.TenantID)
	assert.False(t, report.IncludeEmpty)
	assert.True(t, report.TotalQuantity.Equal(decimal.RequireFromString("12.00")))
	assert.True(t, report.TotalValue.Equal(decimal.RequireFromString("60.00")))

	lotLine := findInventoryLotLine(t, report, "wh-1", "LOT-A", "", "2027-01-31")
	assert.Equal(t, "SKU-001", lotLine.ProductCode)
	assert.Equal(t, "MAIN", lotLine.WarehouseCode)
	assert.True(t, lotLine.Quantity.Equal(decimal.RequireFromString("7.00")))
	assert.True(t, lotLine.UnitCost.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, lotLine.InventoryValue.Equal(decimal.RequireFromString("35.00")))
	assert.Equal(t, lotIssueDate, lotLine.LastMovementDate)

	untrackedLine := findInventoryLotLine(t, report, "wh-1", "", "", "")
	assert.True(t, untrackedLine.Quantity.Equal(decimal.RequireFromString("4.00")))
	assert.True(t, untrackedLine.UnitCost.Equal(decimal.RequireFromString("4.00")))

	serialLine := findInventoryLotLine(t, report, "wh-2", "", "SN-001", "")
	assert.Equal(t, "BRANCH", serialLine.WarehouseCode)
	assert.True(t, serialLine.Quantity.Equal(decimal.RequireFromString("1.00")))
	assert.True(t, serialLine.InventoryValue.Equal(decimal.RequireFromString("9.00")))
	assert.Equal(t, serialReceiptDate, serialLine.LastMovementDate)

	filtered, err := ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "p1", "wh-1", false)
	require.NoError(t, err)
	require.Len(t, filtered.Lines, 2)
	assert.Equal(t, "p1", filtered.ProductID)
	assert.Equal(t, "wh-1", filtered.WarehouseID)
	assert.True(t, filtered.TotalQuantity.Equal(decimal.RequireFromString("11.00")))

	withEmpty, err := ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "", "", true)
	require.NoError(t, err)
	require.Len(t, withEmpty.Lines, 4)
	assert.True(t, withEmpty.IncludeEmpty)
	emptyLine := findInventoryLotLine(t, withEmpty, "wh-1", "LOT-ZERO", "", "")
	assert.True(t, emptyLine.Quantity.IsZero())
	assert.True(t, emptyLine.InventoryValue.IsZero())

	_, err = ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "missing", "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get product")

	_, err = ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "", "missing", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get warehouse")
}

func TestService_GetInventoryLotReportAllocatesLotTransfersByWarehouse(t *testing.T) {
	ts := newTestService()
	ctx := context.Background()

	ts.repo.Products["p1"] = &Product{
		ID:             "p1",
		TenantID:       "tenant-1",
		Code:           "SKU-LOT",
		Name:           "Lot tracked widget",
		ProductType:    ProductTypeGoods,
		PurchasePrice:  decimal.RequireFromString("9.00"),
		TrackInventory: true,
		IsActive:       true,
	}
	ts.repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main warehouse", IsActive: true}
	ts.repo.Warehouses["wh-2"] = &Warehouse{ID: "wh-2", TenantID: "tenant-1", Code: "BRANCH", Name: "Branch warehouse", IsActive: true}

	receiptDate := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	transferDate := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	adjustmentDate := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)
	recountDate := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	ts.repo.Movements["p1"] = []InventoryMovement{
		{
			ID:           "mov-lot-receipt",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-1",
			MovementType: MovementTypeIn,
			Quantity:     decimal.RequireFromString("10.00"),
			UnitCost:     decimal.RequireFromString("5.00"),
			TotalCost:    decimal.RequireFromString("50.00"),
			LotNumber:    "LOT-X",
			MovementDate: receiptDate,
		},
		{
			ID:            "mov-lot-transfer",
			TenantID:      "tenant-1",
			ProductID:     "p1",
			WarehouseID:   "wh-1",
			ToWarehouseID: "wh-2",
			MovementType:  MovementTypeTransfer,
			Quantity:      decimal.RequireFromString("4.00"),
			UnitCost:      decimal.RequireFromString("5.00"),
			LotNumber:     "LOT-X",
			MovementDate:  transferDate,
		},
		{
			ID:           "mov-lot-adjustment",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-2",
			MovementType: MovementTypeAdjustment,
			Quantity:     decimal.RequireFromString("-1.00"),
			LotNumber:    "LOT-X",
			MovementDate: adjustmentDate,
		},
		{
			ID:           "mov-lot-recount",
			TenantID:     "tenant-1",
			ProductID:    "p1",
			WarehouseID:  "wh-2",
			MovementType: MovementTypeTransfer,
			Quantity:     decimal.RequireFromString("2.00"),
			UnitCost:     decimal.RequireFromString("7.00"),
			LotNumber:    "LOT-RECOUNT",
			MovementDate: recountDate,
		},
	}

	report, err := ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "p1", "", false)
	require.NoError(t, err)
	require.Len(t, report.Lines, 3)
	assert.True(t, report.TotalQuantity.Equal(decimal.RequireFromString("11.00")))
	assert.True(t, report.TotalValue.Equal(decimal.RequireFromString("59.00")))

	sourceLine := findInventoryLotLine(t, report, "wh-1", "LOT-X", "", "")
	assert.Equal(t, "MAIN", sourceLine.WarehouseCode)
	assert.True(t, sourceLine.Quantity.Equal(decimal.RequireFromString("6.00")))
	assert.True(t, sourceLine.UnitCost.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, sourceLine.InventoryValue.Equal(decimal.RequireFromString("30.00")))
	assert.Equal(t, transferDate, sourceLine.LastMovementDate)

	destinationLine := findInventoryLotLine(t, report, "wh-2", "LOT-X", "", "")
	assert.Equal(t, "BRANCH", destinationLine.WarehouseCode)
	assert.True(t, destinationLine.Quantity.Equal(decimal.RequireFromString("3.00")))
	assert.True(t, destinationLine.UnitCost.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, destinationLine.InventoryValue.Equal(decimal.RequireFromString("15.00")))
	assert.Equal(t, adjustmentDate, destinationLine.LastMovementDate)

	recountLine := findInventoryLotLine(t, report, "wh-2", "LOT-RECOUNT", "", "")
	assert.True(t, recountLine.Quantity.Equal(decimal.RequireFromString("2.00")))
	assert.True(t, recountLine.UnitCost.Equal(decimal.RequireFromString("7.00")))
	assert.True(t, recountLine.InventoryValue.Equal(decimal.RequireFromString("14.00")))
	assert.Equal(t, recountDate, recountLine.LastMovementDate)

	filtered, err := ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", "p1", "wh-2", false)
	require.NoError(t, err)
	require.Len(t, filtered.Lines, 2)
	assert.True(t, filtered.TotalQuantity.Equal(decimal.RequireFromString("5.00")))
	assert.True(t, filtered.TotalValue.Equal(decimal.RequireFromString("29.00")))
}

func findInventoryValuationLine(t *testing.T, report *InventoryValuationReport, productID, warehouseID string) InventoryValuationLine {
	t.Helper()
	for _, line := range report.Lines {
		if line.ProductID == productID && line.WarehouseID == warehouseID {
			return line
		}
	}
	require.Failf(t, "valuation line not found", "product=%s warehouse=%s", productID, warehouseID)
	return InventoryValuationLine{}
}

func findInventoryLotLine(t *testing.T, report *InventoryLotReport, warehouseID, lotNumber, serialNumber, expiryDate string) InventoryLotLine {
	t.Helper()
	for _, line := range report.Lines {
		if line.WarehouseID == warehouseID && line.LotNumber == lotNumber && line.SerialNumber == serialNumber && line.ExpiryDate == expiryDate {
			return line
		}
	}
	require.Failf(t, "lot line not found", "warehouse=%s lot=%s serial=%s expiry=%s", warehouseID, lotNumber, serialNumber, expiryDate)
	return InventoryLotLine{}
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

func TestService_ProductCategoryWarehouseErrorEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("create product parses optional decimals and repository errors", func(t *testing.T) {
		ts := newTestService()
		product, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", &CreateProductRequest{
			Name:          "Widget",
			SalesPrice:    "12.50",
			PurchasePrice: "6.25",
			VATRate:       "20",
			MinStockLevel: "2",
			ReorderPoint:  "5",
		})
		require.NoError(t, err)
		assert.Equal(t, ProductTypeGoods, product.ProductType)
		assert.Equal(t, "pcs", product.Unit)
		assert.True(t, product.PurchasePrice.Equal(decimal.RequireFromString("6.25")))
		assert.True(t, product.VATRate.Equal(decimal.NewFromInt(20)))
		assert.True(t, product.MinStockLevel.Equal(decimal.NewFromInt(2)))
		assert.True(t, product.ReorderPoint.Equal(decimal.NewFromInt(5)))

		for _, tt := range []struct {
			name    string
			req     CreateProductRequest
			mutate  func(*MockRepository)
			wantErr string
		}{
			{name: "invalid purchase price", req: CreateProductRequest{Name: "Widget", SalesPrice: "12.50", PurchasePrice: "bad"}, wantErr: "invalid purchase price"},
			{name: "invalid VAT rate", req: CreateProductRequest{Name: "Widget", SalesPrice: "12.50", VATRate: "bad"}, wantErr: "invalid VAT rate"},
			{name: "generate code error", req: CreateProductRequest{Name: "Widget", SalesPrice: "12.50"}, mutate: func(repo *MockRepository) { repo.ErrOnGenerate = true }, wantErr: "generate code"},
			{name: "create product error", req: CreateProductRequest{Name: "Widget", SalesPrice: "12.50", Code: "SKU-1"}, mutate: func(repo *MockRepository) { repo.ErrOnCreate = true }, wantErr: "create product"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				ts := newTestService()
				if tt.mutate != nil {
					tt.mutate(ts.repo)
				}
				_, err := ts.svc.CreateProduct(ctx, "tenant-1", "test_schema", &tt.req)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("list products wraps repository error", func(t *testing.T) {
		ts := newTestService()
		ts.repo.ErrOnListProducts = true
		_, err := ts.svc.ListProducts(ctx, "tenant-1", "test_schema", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list products")
	})

	t.Run("update product parses optional decimals and repository errors", func(t *testing.T) {
		ts := newTestService()
		ts.repo.Products["p1"] = &Product{ID: "p1", TenantID: "tenant-1", Name: "Original", ProductType: ProductTypeGoods, SalesPrice: decimal.NewFromInt(10)}
		updated, err := ts.svc.UpdateProduct(ctx, "tenant-1", "test_schema", "p1", &UpdateProductRequest{
			Name:          "Updated",
			Unit:          "kg",
			PurchasePrice: "3.25",
			SalesPrice:    "7.50",
			VATRate:       "9",
			MinStockLevel: "1",
			ReorderPoint:  "4",
			IsActive:      true,
		})
		require.NoError(t, err)
		assert.Equal(t, "kg", updated.Unit)
		assert.True(t, updated.PurchasePrice.Equal(decimal.RequireFromString("3.25")))
		assert.True(t, updated.VATRate.Equal(decimal.NewFromInt(9)))
		assert.True(t, updated.MinStockLevel.Equal(decimal.NewFromInt(1)))
		assert.True(t, updated.ReorderPoint.Equal(decimal.NewFromInt(4)))

		ts.repo.ErrOnGet = true
		_, err = ts.svc.UpdateProduct(ctx, "tenant-1", "test_schema", "p1", &UpdateProductRequest{Name: "Updated", IsActive: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get product")

		ts = newTestService()
		ts.repo.Products["p1"] = &Product{ID: "p1", TenantID: "tenant-1", Name: "Original", ProductType: ProductTypeGoods, SalesPrice: decimal.NewFromInt(10)}
		ts.repo.ErrOnUpdate = true
		_, err = ts.svc.UpdateProduct(ctx, "tenant-1", "test_schema", "p1", &UpdateProductRequest{Name: "Updated", IsActive: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "update product")
	})

	for _, tt := range []struct {
		name    string
		call    func(*Service) error
		mutate  func(*MockRepository)
		wantErr string
	}{
		{name: "delete product", call: func(s *Service) error { return s.DeleteProduct(ctx, "tenant-1", "test_schema", "p1") }, mutate: func(repo *MockRepository) { repo.ErrOnDelete = true }, wantErr: "delete product"},
		{name: "create category", call: func(s *Service) error {
			_, err := s.CreateCategory(ctx, "tenant-1", "test_schema", &CreateCategoryRequest{Name: "Parts"})
			return err
		}, mutate: func(repo *MockRepository) { repo.ErrOnCreateCategory = true }, wantErr: "create category"},
		{name: "get category", call: func(s *Service) error {
			_, err := s.GetCategoryByID(ctx, "tenant-1", "test_schema", "missing")
			return err
		}, wantErr: "get category"},
		{name: "list categories", call: func(s *Service) error { _, err := s.ListCategories(ctx, "tenant-1", "test_schema"); return err }, mutate: func(repo *MockRepository) { repo.ErrOnListCategories = true }, wantErr: "list categories"},
		{name: "delete category", call: func(s *Service) error { return s.DeleteCategory(ctx, "tenant-1", "test_schema", "cat-1") }, mutate: func(repo *MockRepository) { repo.ErrOnDeleteCategory = true }, wantErr: "delete category"},
		{name: "create warehouse", call: func(s *Service) error {
			_, err := s.CreateWarehouse(ctx, "tenant-1", "test_schema", &CreateWarehouseRequest{Code: "MAIN", Name: "Main"})
			return err
		}, mutate: func(repo *MockRepository) { repo.ErrOnCreateWarehouse = true }, wantErr: "create warehouse"},
		{name: "get warehouse", call: func(s *Service) error {
			_, err := s.GetWarehouseByID(ctx, "tenant-1", "test_schema", "missing")
			return err
		}, wantErr: "get warehouse"},
		{name: "list warehouses", call: func(s *Service) error { _, err := s.ListWarehouses(ctx, "tenant-1", "test_schema", false); return err }, mutate: func(repo *MockRepository) { repo.ErrOnListWarehouses = true }, wantErr: "list warehouses"},
		{name: "update warehouse get", call: func(s *Service) error {
			_, err := s.UpdateWarehouse(ctx, "tenant-1", "test_schema", "missing", &UpdateWarehouseRequest{Name: "Main"})
			return err
		}, wantErr: "get warehouse"},
		{name: "update warehouse save", call: func(s *Service) error {
			_, err := s.UpdateWarehouse(ctx, "tenant-1", "test_schema", "wh-1", &UpdateWarehouseRequest{Name: "Main"})
			return err
		}, mutate: func(repo *MockRepository) {
			repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Name: "Main"}
			repo.ErrOnUpdateWarehouse = true
		}, wantErr: "update warehouse"},
		{name: "delete warehouse", call: func(s *Service) error { return s.DeleteWarehouse(ctx, "tenant-1", "test_schema", "wh-1") }, mutate: func(repo *MockRepository) { repo.ErrOnDeleteWarehouse = true }, wantErr: "delete warehouse"},
	} {
		t.Run(tt.name+" repository error", func(t *testing.T) {
			ts := newTestService()
			ts.repo.Products["p1"] = &Product{ID: "p1", TenantID: "tenant-1", Name: "Widget"}
			ts.repo.Categories["cat-1"] = &ProductCategory{ID: "cat-1", TenantID: "tenant-1", Name: "Parts"}
			ts.repo.Warehouses["wh-1"] = &Warehouse{ID: "wh-1", TenantID: "tenant-1", Name: "Main"}
			if tt.mutate != nil {
				tt.mutate(ts.repo)
			}
			err := tt.call(ts.svc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestService_StockOperationErrorEdges(t *testing.T) {
	ctx := context.Background()

	seed := func(ts *testService) {
		ts.repo.Products[inventoryStockProductID] = &Product{
			ID:             inventoryStockProductID,
			TenantID:       "tenant-1",
			Name:           "Widget",
			ProductType:    ProductTypeGoods,
			PurchasePrice:  decimal.NewFromInt(7),
			CurrentStock:   decimal.NewFromInt(10),
			TrackInventory: true,
		}
		ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
		ts.repo.Warehouses[inventoryStockWarehouseID2] = &Warehouse{ID: inventoryStockWarehouseID2, TenantID: "tenant-1", Name: "Branch", IsActive: true}
		ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
			ID:           "sl-1",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			Quantity:     decimal.NewFromInt(10),
			ReservedQty:  decimal.NewFromInt(3),
			AvailableQty: decimal.NewFromInt(7),
		}
		ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID2)] = &StockLevel{
			ID:           "sl-2",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID2,
			Quantity:     decimal.NewFromInt(0),
			ReservedQty:  decimal.Zero,
			AvailableQty: decimal.Zero,
		}
	}
	adjustReq := func(quantity string) *AdjustStockRequest {
		return &AdjustStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: quantity, UnitCost: "2.50", UserID: "user-1"}
	}

	for _, tt := range []struct {
		name    string
		req     *AdjustStockRequest
		mutate  func(*MockRepository)
		wantErr string
	}{
		{name: "invalid unit cost", req: &AdjustStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: "1", UnitCost: "bad"}, wantErr: "invalid unit cost"},
		{name: "get product", req: adjustReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGet = true }, wantErr: "get product"},
		{name: "get warehouse", req: adjustReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGetWarehouse = true }, wantErr: "get warehouse"},
		{name: "get stock level", req: adjustReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGetStockLevels = true }, wantErr: "get stock level"},
		{name: "below reserved", req: adjustReq("-8"), wantErr: "below reserved quantity"},
		{name: "product stock negative", req: adjustReq("-11"), mutate: func(repo *MockRepository) {
			level := repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
			level.Quantity = decimal.NewFromInt(20)
			level.ReservedQty = decimal.Zero
			level.AvailableQty = decimal.NewFromInt(20)
		}, wantErr: "product stock negative"},
		{name: "create movement", req: adjustReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnCreateMovement = true }, wantErr: "create movement"},
		{name: "update product stock", req: adjustReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnUpdateProductStock = true }, wantErr: "update product stock"},
		{name: "upsert stock level", req: adjustReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnUpsertStockLevel = true }, wantErr: "update stock level"},
	} {
		t.Run("adjust stock "+tt.name, func(t *testing.T) {
			ts := newTestService()
			seed(ts)
			if tt.mutate != nil {
				tt.mutate(ts.repo)
			}
			_, err := ts.svc.AdjustStock(ctx, "tenant-1", "test_schema", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	transferReq := func() *TransferStockRequest {
		return &TransferStockRequest{ProductID: inventoryStockProductID, FromWarehouseID: inventoryStockWarehouseID, ToWarehouseID: inventoryStockWarehouseID2, Quantity: "1", UserID: "user-1"}
	}
	for _, tt := range []struct {
		name    string
		req     *TransferStockRequest
		mutate  func(*MockRepository)
		wantErr string
	}{
		{name: "invalid quantity", req: &TransferStockRequest{ProductID: inventoryStockProductID, FromWarehouseID: inventoryStockWarehouseID, ToWarehouseID: inventoryStockWarehouseID2, Quantity: "bad"}, wantErr: "invalid quantity"},
		{name: "nonpositive quantity", req: &TransferStockRequest{ProductID: inventoryStockProductID, FromWarehouseID: inventoryStockWarehouseID, ToWarehouseID: inventoryStockWarehouseID2, Quantity: "0"}, wantErr: "quantity must be positive"},
		{name: "same warehouse", req: &TransferStockRequest{ProductID: inventoryStockProductID, FromWarehouseID: inventoryStockWarehouseID, ToWarehouseID: inventoryStockWarehouseID, Quantity: "1"}, wantErr: "must differ"},
		{name: "get product", req: transferReq(), mutate: func(repo *MockRepository) { repo.ErrOnGet = true }, wantErr: "get product"},
		{name: "get source warehouse", req: transferReq(), mutate: func(repo *MockRepository) { repo.ErrOnGetWarehouse = true }, wantErr: "get source warehouse"},
		{name: "get source stock level", req: transferReq(), mutate: func(repo *MockRepository) { repo.ErrOnGetStockLevels = true }, wantErr: "get source stock level"},
		{name: "insufficient available", req: &TransferStockRequest{ProductID: inventoryStockProductID, FromWarehouseID: inventoryStockWarehouseID, ToWarehouseID: inventoryStockWarehouseID2, Quantity: "8"}, wantErr: "insufficient available stock"},
		{name: "list movements", req: transferReq(), mutate: func(repo *MockRepository) { repo.ErrOnListMovements = true }, wantErr: "list movements for transfer costing"},
		{name: "tracked lot unavailable", req: &TransferStockRequest{ProductID: inventoryStockProductID, FromWarehouseID: inventoryStockWarehouseID, ToWarehouseID: inventoryStockWarehouseID2, Quantity: "1", LotNumber: "missing"}, wantErr: "insufficient tracked lot stock"},
		{name: "create out movement", req: transferReq(), mutate: func(repo *MockRepository) { repo.ErrOnCreateMovement = true }, wantErr: "create out movement"},
		{name: "update source stock level", req: transferReq(), mutate: func(repo *MockRepository) { repo.ErrOnUpsertStockLevel = true }, wantErr: "update source stock level"},
	} {
		t.Run("transfer stock "+tt.name, func(t *testing.T) {
			ts := newTestService()
			seed(ts)
			if tt.mutate != nil {
				tt.mutate(ts.repo)
			}
			err := ts.svc.TransferStock(ctx, "tenant-1", "test_schema", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("get stock levels wraps repository error", func(t *testing.T) {
		ts := newTestService()
		ts.repo.ErrOnGetStockLevels = true
		_, err := ts.svc.GetStockLevels(ctx, "tenant-1", "test_schema", inventoryStockProductID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get stock levels")
	})
	t.Run("get movements wraps repository error", func(t *testing.T) {
		ts := newTestService()
		ts.repo.ErrOnListMovements = true
		_, err := ts.svc.GetMovements(ctx, "tenant-1", "test_schema", inventoryStockProductID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list movements")
	})
}

func TestService_ImportEntrypointErrorEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("product import repository and resolver errors", func(t *testing.T) {
		ts := newTestService()
		ts.repo.ErrOnListProducts = true
		_, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price\nWidget,12.50\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list existing products")

		ts = newTestService()
		ts.repo.ErrOnListCategories = true
		_, err = ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price\nWidget,12.50\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list product categories")

		ts = newTestService()
		ts.svc.accounts = fakeInventoryAccountLister{err: fmt.Errorf("accounts unavailable")}
		_, err = ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price,sale_account_code\nWidget,12.50,4000\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list accounts for product import")

		ts = newTestService()
		ts.svc.contacts = fakeInventoryContactLister{err: fmt.Errorf("contacts unavailable")}
		_, err = ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price,supplier_code\nWidget,12.50,SUP-1\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list contacts for product import")

		ts = newTestService()
		ts.repo.ErrOnGenerate = true
		result, err := ts.svc.ImportProductsCSV(ctx, "tenant-1", "test_schema", &ImportProductsRequest{
			CSVContent: "name,sales_price\nWidget,12.50\n",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "generate code")
	})

	t.Run("category warehouse and stock import repository errors", func(t *testing.T) {
		ts := newTestService()
		ts.repo.ErrOnListCategories = true
		_, err := ts.svc.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{
			CSVContent: "category_name\nParts\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list existing product categories")

		ts = newTestService()
		ts.repo.ErrOnCreateCategory = true
		categoryResult, err := ts.svc.ImportProductCategoriesCSV(ctx, "tenant-1", "test_schema", &ImportProductCategoriesRequest{
			CSVContent: "category_name\nParts\n",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, categoryResult.RowsSkipped)
		require.Len(t, categoryResult.Errors, 1)
		assert.Contains(t, categoryResult.Errors[0].Message, "mock error on create category")

		ts = newTestService()
		ts.repo.ErrOnListWarehouses = true
		_, err = ts.svc.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{
			CSVContent: "warehouse_code,warehouse_name\nMAIN,Main\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list existing warehouses")

		ts = newTestService()
		ts.repo.ErrOnCreateWarehouse = true
		warehouseResult, err := ts.svc.ImportWarehousesCSV(ctx, "tenant-1", "test_schema", &ImportWarehousesRequest{
			CSVContent: "warehouse_code,warehouse_name\nMAIN,Main\n",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, warehouseResult.RowsSkipped)
		require.Len(t, warehouseResult.Errors, 1)
		assert.Contains(t, warehouseResult.Errors[0].Message, "mock error on create warehouse")

		ts = newTestService()
		ts.repo.ErrOnListProducts = true
		_, err = ts.svc.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
			CSVContent: "product_code,warehouse_code,quantity\nSKU-1,MAIN,1\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list products")

		ts = newTestService()
		ts.repo.ErrOnListWarehouses = true
		_, err = ts.svc.ImportStockAdjustmentsCSV(ctx, "tenant-1", "test_schema", &ImportStockAdjustmentsRequest{
			CSVContent: "product_code,warehouse_code,quantity\nSKU-1,MAIN,1\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list warehouses")
	})
}

func TestService_IssueAndReservationErrorEdges(t *testing.T) {
	ctx := context.Background()

	seed := func(ts *testService) {
		ts.repo.Products[inventoryStockProductID] = &Product{
			ID:                 inventoryStockProductID,
			TenantID:           "tenant-1",
			Name:               "Widget",
			ProductType:        ProductTypeGoods,
			PurchasePrice:      decimal.NewFromInt(7),
			CurrentStock:       decimal.NewFromInt(10),
			TrackInventory:     true,
			InventoryAccountID: "55555555-5555-4555-8555-555555555555",
		}
		ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main", IsActive: true}
		ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
			ID:           "sl-1",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			Quantity:     decimal.NewFromInt(10),
			ReservedQty:  decimal.NewFromInt(2),
			AvailableQty: decimal.NewFromInt(8),
		}
		ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
			{
				ID:           "mov-lot",
				TenantID:     "tenant-1",
				ProductID:    inventoryStockProductID,
				WarehouseID:  inventoryStockWarehouseID,
				MovementType: MovementTypeIn,
				Quantity:     decimal.NewFromInt(5),
				UnitCost:     decimal.NewFromInt(8),
				TotalCost:    decimal.NewFromInt(40),
				LotNumber:    "LOT-1",
				MovementDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			},
		}
	}
	issueReq := func(quantity string) *IssueStockRequest {
		return &IssueStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: quantity, UserID: "user-1"}
	}

	for _, tt := range []struct {
		name    string
		req     *IssueStockRequest
		mutate  func(*testService)
		wantErr string
	}{
		{name: "invalid quantity", req: &IssueStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: "bad"}, wantErr: "invalid quantity"},
		{name: "invalid product id", req: &IssueStockRequest{ProductID: "legacy-product", WarehouseID: inventoryStockWarehouseID, Quantity: "1"}, wantErr: "product_id must be a valid UUID"},
		{name: "invalid warehouse id", req: &IssueStockRequest{ProductID: inventoryStockProductID, WarehouseID: "legacy-warehouse", Quantity: "1"}, wantErr: "warehouse_id must be a valid UUID"},
		{name: "invalid source id", req: &IssueStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: "1", SourceID: "legacy-source"}, wantErr: "source_id must be a valid UUID"},
		{name: "invalid expiry", req: &IssueStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: "1", ExpiryDate: "31-01-2027"}, wantErr: "expiry_date must use YYYY-MM-DD"},
		{name: "get product", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnGet = true }, wantErr: "get product"},
		{name: "get warehouse", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnGetWarehouse = true }, wantErr: "get warehouse"},
		{name: "get stock level", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnGetStockLevels = true }, wantErr: "get stock level"},
		{name: "insufficient available", req: issueReq("9"), wantErr: "insufficient available stock"},
		{name: "insufficient product", req: issueReq("11"), mutate: func(ts *testService) {
			level := ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)]
			level.Quantity = decimal.NewFromInt(20)
			level.AvailableQty = decimal.NewFromInt(20)
		}, wantErr: "insufficient product stock"},
		{name: "list movements", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnListMovements = true }, wantErr: "list movements for issue costing"},
		{name: "list reservations", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnListLotReservations = true }, wantErr: "list lot reservations"},
		{name: "tracked lot unavailable", req: &IssueStockRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: "6", LotNumber: "LOT-1"}, wantErr: "insufficient available tracked lot stock"},
		{name: "create issue movement", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnCreateMovement = true }, wantErr: "create issue movement"},
		{name: "update issue product stock", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnUpdateProductStock = true }, wantErr: "update product stock"},
		{name: "update issue stock level", req: issueReq("1"), mutate: func(ts *testService) { ts.repo.ErrOnUpsertStockLevel = true }, wantErr: "update stock level"},
	} {
		t.Run("issue stock "+tt.name, func(t *testing.T) {
			ts := newTestService()
			seed(ts)
			if tt.mutate != nil {
				tt.mutate(ts)
			}
			_, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("post to ledger without transaction ledger fails early", func(t *testing.T) {
		ts := newTestService()
		seed(ts)
		_, err := ts.svc.IssueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
			ProductID:                inventoryStockProductID,
			WarehouseID:              inventoryStockWarehouseID,
			Quantity:                 "1",
			CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
			PostToLedger:             true,
			UserID:                   "user-1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accounting transaction is unavailable")
	})

	t.Run("issue account validation branches", func(t *testing.T) {
		ts := newTestService()
		seed(ts)
		_, err := ts.svc.issueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			Quantity:     "1",
			PostToLedger: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user id is required")

		ts = newTestService()
		seed(ts)
		_, err = ts.svc.issueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
			ProductID:                inventoryStockProductID,
			WarehouseID:              inventoryStockWarehouseID,
			Quantity:                 "1",
			CostOfGoodsSoldAccountID: "bad-account",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cost_of_goods_sold_account_id must be a valid UUID")

		ts = newTestService()
		seed(ts)
		ts.repo.Products[inventoryStockProductID].InventoryAccountID = ""
		_, err = ts.svc.issueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
			ProductID:                inventoryStockProductID,
			WarehouseID:              inventoryStockWarehouseID,
			Quantity:                 "1",
			CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "both required")

		ts = newTestService()
		seed(ts)
		ts.svc.accounts = fakeInventoryAccountLister{err: fmt.Errorf("accounts unavailable")}
		_, err = ts.svc.issueStock(ctx, "tenant-1", "test_schema", &IssueStockRequest{
			ProductID:                inventoryStockProductID,
			WarehouseID:              inventoryStockWarehouseID,
			Quantity:                 "1",
			CostOfGoodsSoldAccountID: "44444444-4444-4444-8444-444444444444",
			InventoryAccountID:       "55555555-5555-4555-8555-555555555555",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list accounts for issue accounting")
	})

	reservationReq := func(quantity string) *StockReservationRequest {
		return &StockReservationRequest{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, Quantity: quantity, UserID: "user-1"}
	}
	for _, tt := range []struct {
		name    string
		call    func(*Service, *StockReservationRequest) (*StockLevel, error)
		req     *StockReservationRequest
		mutate  func(*MockRepository)
		wantErr string
	}{
		{name: "reserve get product", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReserveStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGet = true }, wantErr: "get product"},
		{name: "reserve get warehouse", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReserveStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGetWarehouse = true }, wantErr: "get warehouse"},
		{name: "reserve get stock level", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReserveStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGetStockLevels = true }, wantErr: "get stock level"},
		{name: "reserve insufficient available", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReserveStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("9"), wantErr: "insufficient available stock"},
		{name: "reserve list movements", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReserveStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnListMovements = true }, wantErr: "list movements for product"},
		{name: "reserve upsert stock level", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReserveStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnUpsertStockLevel = true }, wantErr: "update stock level"},
		{name: "release get product", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReleaseStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGet = true }, wantErr: "get product"},
		{name: "release get warehouse", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReleaseStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGetWarehouse = true }, wantErr: "get warehouse"},
		{name: "release get stock level", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReleaseStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnGetStockLevels = true }, wantErr: "get stock level"},
		{name: "release too much", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReleaseStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("3"), wantErr: "cannot release more than reserved stock"},
		{name: "release list reservations", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReleaseStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnListLotReservations = true }, wantErr: "list lot reservations"},
		{name: "release upsert stock level", call: func(s *Service, req *StockReservationRequest) (*StockLevel, error) {
			return s.ReleaseStock(ctx, "tenant-1", "test_schema", req)
		}, req: reservationReq("1"), mutate: func(repo *MockRepository) { repo.ErrOnUpsertStockLevel = true }, wantErr: "update stock level"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestService()
			seed(ts)
			if tt.mutate != nil {
				tt.mutate(ts.repo)
			}
			_, err := tt.call(ts.svc, tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestService_ReportAndCostingEdgeBranches(t *testing.T) {
	ctx := context.Background()

	seedReportData := func(ts *testService) {
		ts.repo.Products[inventoryStockProductID] = &Product{
			ID:                 inventoryStockProductID,
			TenantID:           "tenant-1",
			Code:               "SKU-1",
			Name:               "Widget",
			ProductType:        ProductTypeGoods,
			PurchasePrice:      decimal.NewFromInt(7),
			CurrentStock:       decimal.NewFromInt(5),
			TrackInventory:     true,
			InventoryAccountID: "55555555-5555-4555-8555-555555555555",
		}
		ts.repo.Products["service-1"] = &Product{ID: "service-1", TenantID: "tenant-1", Code: "SVC", Name: "Service", ProductType: ProductTypeService, TrackInventory: false}
		ts.repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}
		ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
			ID:           "sl-1",
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			Quantity:     decimal.NewFromInt(5),
			ReservedQty:  decimal.NewFromInt(1),
			AvailableQty: decimal.NewFromInt(4),
		}
		ts.repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, "other-tenant-warehouse")] = &StockLevel{
			ID:          "sl-other",
			TenantID:    "tenant-other",
			ProductID:   inventoryStockProductID,
			WarehouseID: "other-tenant-warehouse",
			Quantity:    decimal.NewFromInt(99),
		}
		ts.repo.Movements[inventoryStockProductID] = []InventoryMovement{
			{ID: "out", TenantID: "tenant-1", ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, MovementType: MovementTypeOut, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(99)},
			{ID: "zero-qty", TenantID: "tenant-1", ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, MovementType: MovementTypeIn, Quantity: decimal.Zero, UnitCost: decimal.NewFromInt(5)},
			{ID: "zero-cost", TenantID: "tenant-1", ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(1), UnitCost: decimal.Zero, TotalCost: decimal.Zero},
			{ID: "other-tenant", TenantID: "tenant-other", ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(100)},
			{ID: "receipt", TenantID: "tenant-1", ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(5), UnitCost: decimal.NewFromInt(8), MovementDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		}
	}

	for _, tt := range []struct {
		name        string
		method      string
		warehouseID string
		mutate      func(*testService)
		wantErr     string
	}{
		{name: "warehouse lookup", warehouseID: inventoryStockWarehouseID, mutate: func(ts *testService) { ts.repo.ErrOnGetWarehouse = true }, wantErr: "get warehouse"},
		{name: "list products", mutate: func(ts *testService) { ts.repo.ErrOnListProducts = true }, wantErr: "list products"},
		{name: "list warehouses", mutate: func(ts *testService) { ts.repo.ErrOnListWarehouses = true }, wantErr: "list warehouses"},
		{name: "stock levels", mutate: func(ts *testService) { ts.repo.ErrOnGetStockLevels = true }, wantErr: "get stock levels"},
		{name: "cost movements", method: InventoryValuationMethodWeightedAverage, mutate: func(ts *testService) { ts.repo.ErrOnListMovements = true }, wantErr: "list movements"},
	} {
		t.Run("valuation error "+tt.name, func(t *testing.T) {
			ts := newTestService()
			seedReportData(ts)
			if tt.mutate != nil {
				tt.mutate(ts)
			}
			_, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", tt.warehouseID, tt.method)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("valuation weighted average skips unusable movement layers", func(t *testing.T) {
		ts := newTestService()
		seedReportData(ts)
		report, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", InventoryValuationMethodWeightedAverage)
		require.NoError(t, err)
		require.Len(t, report.Lines, 1)
		assert.True(t, report.Lines[0].UnitCost.Equal(decimal.NewFromInt(8)))
		assert.True(t, report.TotalQuantity.Equal(decimal.NewFromInt(5)))
	})

	t.Run("valuation falls back to unassigned stock without levels", func(t *testing.T) {
		ts := newTestService()
		seedReportData(ts)
		delete(ts.repo.StockLevels, inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID))
		delete(ts.repo.StockLevels, inventoryStockLevelKey(inventoryStockProductID, "other-tenant-warehouse"))
		report, err := ts.svc.GetInventoryValuation(ctx, "tenant-1", "test_schema", "", InventoryValuationMethodStandardCost)
		require.NoError(t, err)
		require.Len(t, report.Lines, 1)
		assert.Equal(t, "UNASSIGNED", report.Lines[0].WarehouseCode)
	})

	t.Run("fifo helper covers fallback and created-at ordering", func(t *testing.T) {
		product := Product{ID: "p1", PurchasePrice: decimal.NewFromInt(7)}
		assert.True(t, fifoInventoryUnitCost(product, nil, "tenant-1", decimal.Zero).Equal(decimal.NewFromInt(7)))
		got := fifoInventoryUnitCost(product, []InventoryMovement{
			{TenantID: "tenant-other", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(100)},
			{TenantID: "tenant-1", MovementType: MovementTypeOut, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(100)},
			{TenantID: "tenant-1", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(1), UnitCost: decimal.Zero, TotalCost: decimal.Zero},
			{TenantID: "tenant-1", MovementType: MovementTypeIn, Quantity: decimal.NewFromInt(2), UnitCost: decimal.Zero, TotalCost: decimal.NewFromInt(20), CreatedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)},
		}, "tenant-1", decimal.NewFromInt(3))
		assert.True(t, got.Equal(decimal.NewFromInt(9)))
	})

	t.Run("subledger configured errors", func(t *testing.T) {
		ts := newTestService()
		seedReportData(ts)
		ts.svc.accounts = fakeInventoryBalancer{accounts: []accounting.Account{}, listErr: fmt.Errorf("accounts down")}
		_, err := ts.svc.GetInventorySubledgerReconciliation(ctx, "tenant-1", "test_schema", "", InventoryValuationMethodStandardCost, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list accounts")

		ts = newTestService()
		seedReportData(ts)
		ts.svc.accounts = fakeInventoryBalancer{
			accounts:   []accounting.Account{{ID: "55555555-5555-4555-8555-555555555555", Code: "1400", Name: "Inventory", AccountType: accounting.AccountTypeAsset}},
			balances:   map[string]decimal.Decimal{},
			balanceErr: fmt.Errorf("balance down"),
		}
		_, err = ts.svc.GetInventorySubledgerReconciliation(ctx, "tenant-1", "test_schema", "", InventoryValuationMethodStandardCost, time.Time{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get account balance")
	})

	for _, tt := range []struct {
		name        string
		productID   string
		warehouseID string
		mutate      func(*testService)
		wantErr     string
	}{
		{name: "warehouse lookup", warehouseID: inventoryStockWarehouseID, mutate: func(ts *testService) { ts.repo.ErrOnGetWarehouse = true }, wantErr: "get warehouse"},
		{name: "specific product lookup", productID: inventoryStockProductID, mutate: func(ts *testService) { ts.repo.ErrOnGet = true }, wantErr: "get product"},
		{name: "list products", mutate: func(ts *testService) { ts.repo.ErrOnListProducts = true }, wantErr: "list products"},
		{name: "list warehouses", mutate: func(ts *testService) { ts.repo.ErrOnListWarehouses = true }, wantErr: "list warehouses"},
		{name: "list movements", mutate: func(ts *testService) { ts.repo.ErrOnListMovements = true }, wantErr: "list movements"},
	} {
		t.Run("lot report error "+tt.name, func(t *testing.T) {
			ts := newTestService()
			seedReportData(ts)
			if tt.mutate != nil {
				tt.mutate(ts)
			}
			_, err := ts.svc.GetInventoryLotReport(ctx, "tenant-1", "test_schema", tt.productID, tt.warehouseID, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
