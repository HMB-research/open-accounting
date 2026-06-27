package inventory

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestQualifiedInventoryTableBuildsQualifiedTableReference(t *testing.T) {
	table, err := qualifiedInventoryTable("tenant_demo", "products")
	if err != nil {
		t.Fatalf("qualifiedInventoryTable returned error: %v", err)
	}

	expected := `"tenant_demo"."products"`
	if table != expected {
		t.Fatalf("expected %q, got %q", expected, table)
	}
}

func TestGenerateCodeRejectsInvalidSchemaName(t *testing.T) {
	repo := &GORMRepository{}

	_, err := repo.GenerateCode(context.Background(), "tenant-demo", "tenant-1")
	if err == nil {
		t.Fatal("expected invalid schema error")
	}
}

func TestTenantTableRejectsInvalidSchemaName(t *testing.T) {
	repo := &GORMRepository{}
	ctx := context.Background()

	if _, err := repo.tenantTable(ctx, "tenant-demo", "products"); err == nil {
		t.Fatal("expected tenantTable to reject invalid schema")
	}
}

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repository := NewGORMRepository(nil)
	repo, ok := repository.(*GORMRepository)
	if !ok {
		t.Fatalf("NewGORMRepository(nil) = %T, want *GORMRepository", repository)
	}
	if repo.db != nil {
		t.Fatalf("NewGORMRepository(nil).db = %#v, want nil", repo.db)
	}

	ctx := context.Background()
	schemaName := "tenant_demo"
	tenantID := "tenant-1"
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "products")
				if table != nil {
					t.Fatalf("tenantTable() table = %#v, want nil", table)
				}
				return err
			},
		},
		{
			name: "WithInventoryLedgerTransaction",
			run: func(t *testing.T) error {
				return repo.WithInventoryLedgerTransaction(ctx, nil, func(repo Repository, ledger accountingPoster) error {
					t.Fatal("transaction callback should not run without database")
					return nil
				})
			},
		},
		{
			name: "CreateProduct",
			run: func(t *testing.T) error {
				return repo.CreateProduct(ctx, schemaName, &Product{TenantID: tenantID})
			},
		},
		{
			name: "GetProductByID",
			run: func(t *testing.T) error {
				product, err := repo.GetProductByID(ctx, schemaName, tenantID, "product-1")
				if product != nil {
					t.Fatalf("GetProductByID() product = %#v, want nil", product)
				}
				return err
			},
		},
		{
			name: "ListProducts",
			run: func(t *testing.T) error {
				products, err := repo.ListProducts(ctx, schemaName, tenantID, &ProductFilter{
					ProductType: ProductTypeGoods,
					Status:      ProductStatusActive,
					CategoryID:  "category-1",
					Search:      "widget",
					LowStock:    true,
				})
				if products != nil {
					t.Fatalf("ListProducts() products = %#v, want nil", products)
				}
				return err
			},
		},
		{
			name: "UpdateProduct",
			run: func(t *testing.T) error {
				return repo.UpdateProduct(ctx, schemaName, &Product{ID: "product-1", TenantID: tenantID})
			},
		},
		{
			name: "DeleteProduct",
			run: func(t *testing.T) error {
				return repo.DeleteProduct(ctx, schemaName, tenantID, "product-1")
			},
		},
		{
			name: "GenerateCode",
			run: func(t *testing.T) error {
				code, err := repo.GenerateCode(ctx, schemaName, tenantID)
				if code != "" {
					t.Fatalf("GenerateCode() code = %q, want empty", code)
				}
				return err
			},
		},
		{
			name: "CreateCategory",
			run: func(t *testing.T) error {
				return repo.CreateCategory(ctx, schemaName, &ProductCategory{TenantID: tenantID})
			},
		},
		{
			name: "GetCategoryByID",
			run: func(t *testing.T) error {
				category, err := repo.GetCategoryByID(ctx, schemaName, tenantID, "category-1")
				if category != nil {
					t.Fatalf("GetCategoryByID() category = %#v, want nil", category)
				}
				return err
			},
		},
		{
			name: "ListCategories",
			run: func(t *testing.T) error {
				categories, err := repo.ListCategories(ctx, schemaName, tenantID)
				if categories != nil {
					t.Fatalf("ListCategories() categories = %#v, want nil", categories)
				}
				return err
			},
		},
		{
			name: "DeleteCategory",
			run: func(t *testing.T) error {
				return repo.DeleteCategory(ctx, schemaName, tenantID, "category-1")
			},
		},
		{
			name: "CreateWarehouse",
			run: func(t *testing.T) error {
				return repo.CreateWarehouse(ctx, schemaName, &Warehouse{TenantID: tenantID})
			},
		},
		{
			name: "GetWarehouseByID",
			run: func(t *testing.T) error {
				warehouse, err := repo.GetWarehouseByID(ctx, schemaName, tenantID, "warehouse-1")
				if warehouse != nil {
					t.Fatalf("GetWarehouseByID() warehouse = %#v, want nil", warehouse)
				}
				return err
			},
		},
		{
			name: "ListWarehouses",
			run: func(t *testing.T) error {
				warehouses, err := repo.ListWarehouses(ctx, schemaName, tenantID, true)
				if warehouses != nil {
					t.Fatalf("ListWarehouses() warehouses = %#v, want nil", warehouses)
				}
				return err
			},
		},
		{
			name: "UpdateWarehouse",
			run: func(t *testing.T) error {
				return repo.UpdateWarehouse(ctx, schemaName, &Warehouse{ID: "warehouse-1", TenantID: tenantID})
			},
		},
		{
			name: "DeleteWarehouse",
			run: func(t *testing.T) error {
				return repo.DeleteWarehouse(ctx, schemaName, tenantID, "warehouse-1")
			},
		},
		{
			name: "GetStockLevel",
			run: func(t *testing.T) error {
				level, err := repo.GetStockLevel(ctx, schemaName, tenantID, "product-1", "warehouse-1")
				if level != nil {
					t.Fatalf("GetStockLevel() level = %#v, want nil", level)
				}
				return err
			},
		},
		{
			name: "GetStockLevelsByProduct",
			run: func(t *testing.T) error {
				levels, err := repo.GetStockLevelsByProduct(ctx, schemaName, tenantID, "product-1")
				if levels != nil {
					t.Fatalf("GetStockLevelsByProduct() levels = %#v, want nil", levels)
				}
				return err
			},
		},
		{
			name: "UpsertStockLevel",
			run: func(t *testing.T) error {
				return repo.UpsertStockLevel(ctx, schemaName, &StockLevel{TenantID: tenantID})
			},
		},
		{
			name: "ListLotReservations",
			run: func(t *testing.T) error {
				reservations, err := repo.ListLotReservations(ctx, schemaName, tenantID, "product-1", "warehouse-1")
				if reservations != nil {
					t.Fatalf("ListLotReservations() reservations = %#v, want nil", reservations)
				}
				return err
			},
		},
		{
			name: "UpsertLotReservation",
			run: func(t *testing.T) error {
				return repo.UpsertLotReservation(ctx, schemaName, &InventoryLotReservation{TenantID: tenantID})
			},
		},
		{
			name: "ReleaseLotReservation",
			run: func(t *testing.T) error {
				reservation, err := repo.ReleaseLotReservation(ctx, schemaName, tenantID, "product-1", "warehouse-1", "lot-1", "serial-1", "2026-12-31", decimal.NewFromInt(1), "release", "user-1")
				if reservation != nil {
					t.Fatalf("ReleaseLotReservation() reservation = %#v, want nil", reservation)
				}
				return err
			},
		},
		{
			name: "CreateMovement",
			run: func(t *testing.T) error {
				return repo.CreateMovement(ctx, schemaName, &InventoryMovement{TenantID: tenantID})
			},
		},
		{
			name: "ListMovements",
			run: func(t *testing.T) error {
				movements, err := repo.ListMovements(ctx, schemaName, tenantID, "product-1")
				if movements != nil {
					t.Fatalf("ListMovements() movements = %#v, want nil", movements)
				}
				return err
			},
		},
		{
			name: "UpdateProductStock",
			run: func(t *testing.T) error {
				return repo.UpdateProductStock(ctx, schemaName, tenantID, "product-1", decimal.NewFromInt(10))
			},
		},
		{
			name: "qualified helper valid schema through method setup",
			run: func(t *testing.T) error {
				product := &Product{
					ID:                 "product-1",
					TenantID:           tenantID,
					ProductType:        ProductTypeGoods,
					CategoryID:         "category-1",
					SaleAccountID:      "sale-account-1",
					PurchaseAccountID:  "purchase-account-1",
					InventoryAccountID: "inventory-account-1",
					SupplierID:         "supplier-1",
					CreatedAt:          now,
					UpdatedAt:          now,
				}
				_ = productCreateValues(product)
				return repo.CreateProduct(ctx, schemaName, product)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); got != "inventory repository database is not configured" {
				t.Fatalf("error = %q, want inventory repository database is not configured", got)
			}
		})
	}
}

func TestInventoryRepositoryMappingHelpers(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	categoryID := "category-1"
	saleAccountID := "sale-account-1"
	purchaseAccountID := "purchase-account-1"
	inventoryAccountID := "inventory-account-1"
	supplierID := "supplier-1"

	productRow := productRow{
		ID:                 "product-1",
		TenantID:           "tenant-1",
		Code:               "PRD-00001",
		Name:               "Widget",
		Description:        "Tracked widget",
		ProductType:        ProductTypeGoods,
		CategoryID:         &categoryID,
		Unit:               "pcs",
		PurchasePrice:      decimal.RequireFromString("10.50"),
		SalesPrice:         decimal.RequireFromString("15.75"),
		VATRate:            decimal.RequireFromString("22"),
		MinStockLevel:      decimal.RequireFromString("2"),
		CurrentStock:       decimal.RequireFromString("5"),
		ReorderPoint:       decimal.RequireFromString("3"),
		SaleAccountID:      &saleAccountID,
		PurchaseAccountID:  &purchaseAccountID,
		InventoryAccountID: &inventoryAccountID,
		TrackInventory:     true,
		IsActive:           true,
		Barcode:            "123456789",
		SupplierID:         &supplierID,
		LeadTimeDays:       7,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	product := productFromRow(productRow)
	if product.ID != productRow.ID ||
		product.CategoryID != categoryID ||
		product.SaleAccountID != saleAccountID ||
		product.PurchaseAccountID != purchaseAccountID ||
		product.InventoryAccountID != inventoryAccountID ||
		product.SupplierID != supplierID ||
		!product.SalesPrice.Equal(productRow.SalesPrice) {
		t.Fatalf("productFromRow() = %#v, want fields from %#v", product, productRow)
	}

	createValues := productCreateValues(product)
	if createValues["category_id"] != categoryID ||
		createValues["sale_account_id"] != saleAccountID ||
		createValues["purchase_account_id"] != purchaseAccountID ||
		createValues["inventory_account_id"] != inventoryAccountID ||
		createValues["supplier_id"] != supplierID {
		t.Fatalf("productCreateValues() = %#v, want account/category/supplier ids", createValues)
	}

	updateValues := productUpdateValues(product)
	if updateValues["category_id"] != categoryID ||
		updateValues["sale_account_id"] != saleAccountID ||
		updateValues["purchase_account_id"] != purchaseAccountID ||
		updateValues["inventory_account_id"] != inventoryAccountID ||
		updateValues["supplier_id"] != supplierID {
		t.Fatalf("productUpdateValues() = %#v, want account/category/supplier ids", updateValues)
	}

	if nullableString("") != nil {
		t.Fatal("nullableString(\"\") should return nil")
	}
	if nullableString("value") != "value" {
		t.Fatal("nullableString(\"value\") should return value")
	}
	if stringValue(nil) != "" {
		t.Fatal("stringValue(nil) should return empty string")
	}

	parentID := "parent-1"
	category := productCategoryFromRow(productCategoryRow{
		ID:          "category-1",
		TenantID:    "tenant-1",
		Name:        "Parts",
		Description: "Spare parts",
		ParentID:    &parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if category.ParentID != parentID || category.Name != "Parts" {
		t.Fatalf("productCategoryFromRow() = %#v, want parent/name mapped", category)
	}

	toWarehouseID := "warehouse-2"
	movement := inventoryMovementFromRow(inventoryMovementRow{
		ID:            "movement-1",
		TenantID:      "tenant-1",
		ProductID:     "product-1",
		WarehouseID:   "warehouse-1",
		MovementType:  MovementTypeTransfer,
		Quantity:      decimal.RequireFromString("4"),
		UnitCost:      decimal.RequireFromString("10.50"),
		TotalCost:     decimal.RequireFromString("42"),
		LotNumber:     "lot-1",
		SerialNumber:  "serial-1",
		ExpiryDate:    "2026-12-31",
		Reference:     "REF-1",
		SourceType:    "IMPORT",
		SourceID:      "source-1",
		ToWarehouseID: &toWarehouseID,
		Notes:         "transfer",
		MovementDate:  now,
		CreatedAt:     now,
		CreatedBy:     "user-1",
	})
	if movement.ToWarehouseID != toWarehouseID ||
		movement.MovementType != MovementTypeTransfer ||
		!movement.TotalCost.Equal(decimal.RequireFromString("42")) {
		t.Fatalf("inventoryMovementFromRow() = %#v, want movement fields mapped", movement)
	}
}
