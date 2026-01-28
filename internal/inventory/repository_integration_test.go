//go:build integration

package inventory

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestRepository_CreateAndGetProduct(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	product := &Product{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		Code:           "PRD-00001",
		Name:           "Test Product",
		Description:    "Test product description",
		ProductType:    ProductTypeGoods,
		Unit:           "pcs",
		PurchasePrice:  decimal.NewFromFloat(50.00),
		SalesPrice:     decimal.NewFromFloat(100.00),
		VATRate:        decimal.NewFromFloat(20.00),
		MinStockLevel:  decimal.NewFromInt(10),
		CurrentStock:   decimal.NewFromInt(100),
		ReorderPoint:   decimal.NewFromInt(20),
		TrackInventory: true,
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := repo.CreateProduct(ctx, tenant.SchemaName, product)
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	retrieved, err := repo.GetProductByID(ctx, tenant.SchemaName, tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("GetProductByID failed: %v", err)
	}

	if retrieved.Name != product.Name {
		t.Errorf("expected name %s, got %s", product.Name, retrieved.Name)
	}
	if retrieved.Code != product.Code {
		t.Errorf("expected code %s, got %s", product.Code, retrieved.Code)
	}
	if !retrieved.SalesPrice.Equal(product.SalesPrice) {
		t.Errorf("expected sales price %s, got %s", product.SalesPrice, retrieved.SalesPrice)
	}
}

func TestRepository_GetProductByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Use valid UUID format that doesn't exist
	nonExistentID := uuid.New().String()
	_, err := repo.GetProductByID(ctx, tenant.SchemaName, tenant.ID, nonExistentID)
	if err == nil {
		t.Error("expected error for non-existent product")
	}
}

func TestRepository_ListProducts(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create multiple products
	for i := 0; i < 3; i++ {
		product := &Product{
			ID:          uuid.New().String(),
			TenantID:    tenant.ID,
			Code:        uuid.New().String()[:8],
			Name:        "Product " + uuid.New().String()[:8],
			ProductType: ProductTypeGoods,
			Unit:        "pcs",
			SalesPrice:  decimal.NewFromFloat(float64(10 * (i + 1))),
			VATRate:     decimal.NewFromFloat(20.00),
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
			t.Fatalf("CreateProduct failed: %v", err)
		}
	}

	products, err := repo.ListProducts(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("ListProducts failed: %v", err)
	}

	if len(products) != 3 {
		t.Errorf("expected 3 products, got %d", len(products))
	}
}

func TestRepository_ListProducts_WithFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create goods product
	goodsProduct := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-GOODS",
		Name:        "Goods Product",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, goodsProduct); err != nil {
		t.Fatalf("CreateProduct (goods) failed: %v", err)
	}

	// Create service product
	serviceProduct := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-SERVICE",
		Name:        "Service Product",
		ProductType: ProductTypeService,
		Unit:        "hr",
		SalesPrice:  decimal.NewFromFloat(50.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, serviceProduct); err != nil {
		t.Fatalf("CreateProduct (service) failed: %v", err)
	}

	// Filter by product type
	filter := &ProductFilter{ProductType: ProductTypeGoods}
	products, err := repo.ListProducts(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListProducts with filter failed: %v", err)
	}

	if len(products) != 1 {
		t.Errorf("expected 1 goods product, got %d", len(products))
	}
	if products[0].ProductType != ProductTypeGoods {
		t.Errorf("expected product type %s, got %s", ProductTypeGoods, products[0].ProductType)
	}
}

func TestRepository_UpdateProduct(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	product := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-UPDATE",
		Name:        "Original Name",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// Update the product
	product.Name = "Updated Name"
	product.SalesPrice = decimal.NewFromFloat(150.00)
	product.UpdatedAt = time.Now()

	if err := repo.UpdateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("UpdateProduct failed: %v", err)
	}

	retrieved, err := repo.GetProductByID(ctx, tenant.SchemaName, tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("GetProductByID failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if !retrieved.SalesPrice.Equal(decimal.NewFromFloat(150.00)) {
		t.Errorf("expected sales price 150.00, got %s", retrieved.SalesPrice)
	}
}

func TestRepository_DeleteProduct(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	product := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-DELETE",
		Name:        "Product to Delete",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	if err := repo.DeleteProduct(ctx, tenant.SchemaName, tenant.ID, product.ID); err != nil {
		t.Fatalf("DeleteProduct failed: %v", err)
	}

	// Verify deletion
	_, err := repo.GetProductByID(ctx, tenant.SchemaName, tenant.ID, product.ID)
	if err == nil {
		t.Error("expected error for deleted product")
	}
}

func TestRepository_GenerateCode(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	code, err := repo.GenerateCode(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}

	if code != "PRD-00001" {
		t.Errorf("expected code 'PRD-00001', got '%s'", code)
	}

	// Create a product with this code
	product := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        code,
		Name:        "Test Product",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// Generate next code
	nextCode, err := repo.GenerateCode(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateCode (second) failed: %v", err)
	}

	if nextCode != "PRD-00002" {
		t.Errorf("expected code 'PRD-00002', got '%s'", nextCode)
	}
}

func TestRepository_CreateAndGetCategory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	category := &ProductCategory{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Name:        "Electronics",
		Description: "Electronic devices and accessories",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := repo.CreateCategory(ctx, tenant.SchemaName, category)
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	retrieved, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, category.ID)
	if err != nil {
		t.Fatalf("GetCategoryByID failed: %v", err)
	}

	if retrieved.Name != category.Name {
		t.Errorf("expected name %s, got %s", category.Name, retrieved.Name)
	}
}

func TestRepository_GetCategoryByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Use valid UUID format that doesn't exist
	nonExistentID := uuid.New().String()
	_, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, nonExistentID)
	if err == nil {
		t.Error("expected error for non-existent category")
	}
}

func TestRepository_ListCategories(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create categories
	categories := []string{"Electronics", "Furniture", "Office Supplies"}
	for _, name := range categories {
		category := &ProductCategory{
			ID:        uuid.New().String(),
			TenantID:  tenant.ID,
			Name:      name,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.CreateCategory(ctx, tenant.SchemaName, category); err != nil {
			t.Fatalf("CreateCategory failed: %v", err)
		}
	}

	result, err := repo.ListCategories(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListCategories failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 categories, got %d", len(result))
	}
}

func TestRepository_DeleteCategory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	category := &ProductCategory{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Name:      "To Delete",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateCategory(ctx, tenant.SchemaName, category); err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	if err := repo.DeleteCategory(ctx, tenant.SchemaName, tenant.ID, category.ID); err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}

	_, err := repo.GetCategoryByID(ctx, tenant.SchemaName, tenant.ID, category.ID)
	if err == nil {
		t.Error("expected error for deleted category")
	}
}

func TestRepository_CreateAndGetWarehouse(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	warehouse := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-001",
		Name:      "Main Warehouse",
		Address:   "123 Main St",
		IsDefault: true,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse)
	if err != nil {
		t.Fatalf("CreateWarehouse failed: %v", err)
	}

	retrieved, err := repo.GetWarehouseByID(ctx, tenant.SchemaName, tenant.ID, warehouse.ID)
	if err != nil {
		t.Fatalf("GetWarehouseByID failed: %v", err)
	}

	if retrieved.Name != warehouse.Name {
		t.Errorf("expected name %s, got %s", warehouse.Name, retrieved.Name)
	}
	if retrieved.Code != warehouse.Code {
		t.Errorf("expected code %s, got %s", warehouse.Code, retrieved.Code)
	}
}

func TestRepository_GetWarehouseByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Use valid UUID format that doesn't exist
	nonExistentID := uuid.New().String()
	_, err := repo.GetWarehouseByID(ctx, tenant.SchemaName, tenant.ID, nonExistentID)
	if err == nil {
		t.Error("expected error for non-existent warehouse")
	}
}

func TestRepository_ListWarehouses(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create warehouses
	warehouse1 := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-001",
		Name:      "Active Warehouse",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	warehouse2 := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-002",
		Name:      "Inactive Warehouse",
		IsActive:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse1); err != nil {
		t.Fatalf("CreateWarehouse (active) failed: %v", err)
	}
	if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse2); err != nil {
		t.Fatalf("CreateWarehouse (inactive) failed: %v", err)
	}

	// List all
	all, err := repo.ListWarehouses(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListWarehouses (all) failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 warehouses, got %d", len(all))
	}

	// List active only
	active, err := repo.ListWarehouses(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListWarehouses (active) failed: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active warehouse, got %d", len(active))
	}
}

func TestRepository_UpdateWarehouse(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	warehouse := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-UPDATE",
		Name:      "Original Name",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse); err != nil {
		t.Fatalf("CreateWarehouse failed: %v", err)
	}

	warehouse.Name = "Updated Name"
	warehouse.Address = "New Address"
	warehouse.UpdatedAt = time.Now()

	if err := repo.UpdateWarehouse(ctx, tenant.SchemaName, warehouse); err != nil {
		t.Fatalf("UpdateWarehouse failed: %v", err)
	}

	retrieved, err := repo.GetWarehouseByID(ctx, tenant.SchemaName, tenant.ID, warehouse.ID)
	if err != nil {
		t.Fatalf("GetWarehouseByID failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if retrieved.Address != "New Address" {
		t.Errorf("expected address 'New Address', got '%s'", retrieved.Address)
	}
}

func TestRepository_DeleteWarehouse(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	warehouse := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-DELETE",
		Name:      "To Delete",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse); err != nil {
		t.Fatalf("CreateWarehouse failed: %v", err)
	}

	if err := repo.DeleteWarehouse(ctx, tenant.SchemaName, tenant.ID, warehouse.ID); err != nil {
		t.Fatalf("DeleteWarehouse failed: %v", err)
	}

	_, err := repo.GetWarehouseByID(ctx, tenant.SchemaName, tenant.ID, warehouse.ID)
	if err == nil {
		t.Error("expected error for deleted warehouse")
	}
}

func TestRepository_StockLevelOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create product
	product := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-STOCK",
		Name:        "Stock Test Product",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// Create warehouse
	warehouse := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-STOCK",
		Name:      "Stock Warehouse",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse); err != nil {
		t.Fatalf("CreateWarehouse failed: %v", err)
	}

	// Create stock level
	stockLevel := &StockLevel{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		ProductID:    product.ID,
		WarehouseID:  warehouse.ID,
		Quantity:     decimal.NewFromInt(100),
		ReservedQty:  decimal.NewFromInt(10),
		AvailableQty: decimal.NewFromInt(90),
		LastUpdated:  time.Now(),
	}
	if err := repo.UpsertStockLevel(ctx, tenant.SchemaName, stockLevel); err != nil {
		t.Fatalf("UpsertStockLevel failed: %v", err)
	}

	// Get stock level
	retrieved, err := repo.GetStockLevel(ctx, tenant.SchemaName, tenant.ID, product.ID, warehouse.ID)
	if err != nil {
		t.Fatalf("GetStockLevel failed: %v", err)
	}

	if !retrieved.Quantity.Equal(decimal.NewFromInt(100)) {
		t.Errorf("expected quantity 100, got %s", retrieved.Quantity)
	}

	// Update stock level
	stockLevel.Quantity = decimal.NewFromInt(150)
	stockLevel.AvailableQty = decimal.NewFromInt(140)
	stockLevel.LastUpdated = time.Now()
	if err := repo.UpsertStockLevel(ctx, tenant.SchemaName, stockLevel); err != nil {
		t.Fatalf("UpsertStockLevel (update) failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetStockLevel(ctx, tenant.SchemaName, tenant.ID, product.ID, warehouse.ID)
	if err != nil {
		t.Fatalf("GetStockLevel (after update) failed: %v", err)
	}

	if !updated.Quantity.Equal(decimal.NewFromInt(150)) {
		t.Errorf("expected quantity 150, got %s", updated.Quantity)
	}
}

func TestRepository_GetStockLevel_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Use valid UUID formats that don't exist
	nonExistentProductID := uuid.New().String()
	nonExistentWarehouseID := uuid.New().String()
	_, err := repo.GetStockLevel(ctx, tenant.SchemaName, tenant.ID, nonExistentProductID, nonExistentWarehouseID)
	if err == nil {
		t.Error("expected error for non-existent stock level")
	}
}

func TestRepository_GetStockLevelsByProduct(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create product
	product := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-MULTI",
		Name:        "Multi Warehouse Product",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// Create two warehouses
	for i := 0; i < 2; i++ {
		warehouse := &Warehouse{
			ID:        uuid.New().String(),
			TenantID:  tenant.ID,
			Code:      uuid.New().String()[:8],
			Name:      "Warehouse " + uuid.New().String()[:8],
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse); err != nil {
			t.Fatalf("CreateWarehouse failed: %v", err)
		}

		stockLevel := &StockLevel{
			ID:           uuid.New().String(),
			TenantID:     tenant.ID,
			ProductID:    product.ID,
			WarehouseID:  warehouse.ID,
			Quantity:     decimal.NewFromInt(int64(50 * (i + 1))),
			ReservedQty:  decimal.Zero,
			AvailableQty: decimal.NewFromInt(int64(50 * (i + 1))),
			LastUpdated:  time.Now(),
		}
		if err := repo.UpsertStockLevel(ctx, tenant.SchemaName, stockLevel); err != nil {
			t.Fatalf("UpsertStockLevel failed: %v", err)
		}
	}

	levels, err := repo.GetStockLevelsByProduct(ctx, tenant.SchemaName, tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("GetStockLevelsByProduct failed: %v", err)
	}

	if len(levels) != 2 {
		t.Errorf("expected 2 stock levels, got %d", len(levels))
	}
}

func TestRepository_CreateAndListMovements(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create product
	product := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "PRD-MOVEMENT",
		Name:        "Movement Test Product",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(100.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// Create warehouse
	warehouse := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Code:      "WH-MOVEMENT",
		Name:      "Movement Warehouse",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateWarehouse(ctx, tenant.SchemaName, warehouse); err != nil {
		t.Fatalf("CreateWarehouse failed: %v", err)
	}

	// Create movements
	movement := &InventoryMovement{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		ProductID:    product.ID,
		WarehouseID:  warehouse.ID,
		MovementType: "IN",
		Quantity:     decimal.NewFromInt(50),
		UnitCost:     decimal.NewFromFloat(10.00),
		TotalCost:    decimal.NewFromFloat(500.00),
		Reference:    "PO-001",
		Notes:        "Initial stock inbound",
		MovementDate: time.Now(),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
	}
	if err := repo.CreateMovement(ctx, tenant.SchemaName, movement); err != nil {
		t.Fatalf("CreateMovement failed: %v", err)
	}

	movements, err := repo.ListMovements(ctx, tenant.SchemaName, tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("ListMovements failed: %v", err)
	}

	if len(movements) != 1 {
		t.Errorf("expected 1 movement, got %d", len(movements))
	}
	if movements[0].MovementType != "IN" {
		t.Errorf("expected movement type 'IN', got '%s'", movements[0].MovementType)
	}
}

func TestRepository_UpdateProductStock(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	product := &Product{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		Code:         "PRD-STOCK-UPDATE",
		Name:         "Stock Update Product",
		ProductType:  ProductTypeGoods,
		Unit:         "pcs",
		SalesPrice:   decimal.NewFromFloat(100.00),
		VATRate:      decimal.NewFromFloat(20.00),
		CurrentStock: decimal.NewFromInt(100),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product); err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// Update stock
	newStock := decimal.NewFromInt(150)
	if err := repo.UpdateProductStock(ctx, tenant.SchemaName, tenant.ID, product.ID, newStock); err != nil {
		t.Fatalf("UpdateProductStock failed: %v", err)
	}

	// Verify
	retrieved, err := repo.GetProductByID(ctx, tenant.SchemaName, tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("GetProductByID failed: %v", err)
	}

	if !retrieved.CurrentStock.Equal(newStock) {
		t.Errorf("expected stock %s, got %s", newStock, retrieved.CurrentStock)
	}
}

func TestRepository_ListProducts_LowStock(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create product with low stock
	lowStockProduct := &Product{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		Code:         "PRD-LOW",
		Name:         "Low Stock Product",
		ProductType:  ProductTypeGoods,
		Unit:         "pcs",
		SalesPrice:   decimal.NewFromFloat(100.00),
		VATRate:      decimal.NewFromFloat(20.00),
		CurrentStock: decimal.NewFromInt(5),
		ReorderPoint: decimal.NewFromInt(10),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, lowStockProduct); err != nil {
		t.Fatalf("CreateProduct (low) failed: %v", err)
	}

	// Create product with sufficient stock
	goodStockProduct := &Product{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		Code:         "PRD-GOOD",
		Name:         "Good Stock Product",
		ProductType:  ProductTypeGoods,
		Unit:         "pcs",
		SalesPrice:   decimal.NewFromFloat(100.00),
		VATRate:      decimal.NewFromFloat(20.00),
		CurrentStock: decimal.NewFromInt(100),
		ReorderPoint: decimal.NewFromInt(10),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, goodStockProduct); err != nil {
		t.Fatalf("CreateProduct (good) failed: %v", err)
	}

	// Filter for low stock
	filter := &ProductFilter{LowStock: true}
	products, err := repo.ListProducts(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListProducts with low stock filter failed: %v", err)
	}

	if len(products) != 1 {
		t.Errorf("expected 1 low stock product, got %d", len(products))
	}
}

func TestRepository_ListProducts_Search(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create products
	product1 := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "LAPTOP-001",
		Name:        "Business Laptop Pro",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(1000.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	product2 := &Product{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "MOUSE-001",
		Name:        "Wireless Mouse",
		ProductType: ProductTypeGoods,
		Unit:        "pcs",
		SalesPrice:  decimal.NewFromFloat(50.00),
		VATRate:     decimal.NewFromFloat(20.00),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.CreateProduct(ctx, tenant.SchemaName, product1); err != nil {
		t.Fatalf("CreateProduct (1) failed: %v", err)
	}
	if err := repo.CreateProduct(ctx, tenant.SchemaName, product2); err != nil {
		t.Fatalf("CreateProduct (2) failed: %v", err)
	}

	// Search by name
	filter := &ProductFilter{Search: "laptop"}
	products, err := repo.ListProducts(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListProducts with search filter failed: %v", err)
	}

	if len(products) != 1 {
		t.Errorf("expected 1 product matching 'laptop', got %d", len(products))
	}
	if products[0].Code != "LAPTOP-001" {
		t.Errorf("expected code 'LAPTOP-001', got '%s'", products[0].Code)
	}

	// Search by code
	filter2 := &ProductFilter{Search: "MOUSE"}
	products2, err := repo.ListProducts(ctx, tenant.SchemaName, tenant.ID, filter2)
	if err != nil {
		t.Fatalf("ListProducts with code search failed: %v", err)
	}

	if len(products2) != 1 {
		t.Errorf("expected 1 product matching 'MOUSE', got %d", len(products2))
	}
}
