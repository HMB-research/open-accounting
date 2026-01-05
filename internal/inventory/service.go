package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides inventory operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new inventory service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new inventory service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateProduct creates a new product
func (s *Service) CreateProduct(ctx context.Context, tenantID, schemaName string, req *CreateProductRequest) (*Product, error) {
	purchasePrice := decimal.Zero
	if req.PurchasePrice != "" {
		var err error
		purchasePrice, err = decimal.NewFromString(req.PurchasePrice)
		if err != nil {
			return nil, fmt.Errorf("invalid purchase price: %w", err)
		}
	}

	salesPrice, err := decimal.NewFromString(req.SalesPrice)
	if err != nil {
		return nil, fmt.Errorf("invalid sales price: %w", err)
	}

	vatRate := decimal.NewFromInt(22) // Default VAT rate
	if req.VATRate != "" {
		vatRate, err = decimal.NewFromString(req.VATRate)
		if err != nil {
			return nil, fmt.Errorf("invalid VAT rate: %w", err)
		}
	}

	minStockLevel := decimal.Zero
	if req.MinStockLevel != "" {
		minStockLevel, _ = decimal.NewFromString(req.MinStockLevel)
	}

	reorderPoint := decimal.Zero
	if req.ReorderPoint != "" {
		reorderPoint, _ = decimal.NewFromString(req.ReorderPoint)
	}

	code := req.Code
	if code == "" {
		code, err = s.repo.GenerateCode(ctx, schemaName, tenantID)
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
	}

	productType := ProductType(req.ProductType)
	if productType == "" {
		productType = ProductTypeGoods
	}

	unit := req.Unit
	if unit == "" {
		unit = "pcs"
	}

	product := &Product{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		Code:               code,
		Name:               req.Name,
		Description:        req.Description,
		ProductType:        productType,
		CategoryID:         req.CategoryID,
		Unit:               unit,
		PurchasePrice:      purchasePrice,
		SalesPrice:         salesPrice,
		VATRate:            vatRate,
		MinStockLevel:      minStockLevel,
		CurrentStock:       decimal.Zero,
		ReorderPoint:       reorderPoint,
		SaleAccountID:      req.SaleAccountID,
		PurchaseAccountID:  req.PurchaseAccountID,
		InventoryAccountID: req.InventoryAccountID,
		TrackInventory:     req.TrackInventory,
		IsActive:           true,
		Barcode:            req.Barcode,
		SupplierID:         req.SupplierID,
		LeadTimeDays:       req.LeadTimeDays,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := product.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.CreateProduct(ctx, schemaName, product); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}

	return product, nil
}

// GetProductByID retrieves a product by ID
func (s *Service) GetProductByID(ctx context.Context, tenantID, schemaName, productID string) (*Product, error) {
	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return product, nil
}

// ListProducts retrieves products with optional filtering
func (s *Service) ListProducts(ctx context.Context, tenantID, schemaName string, filter *ProductFilter) ([]Product, error) {
	products, err := s.repo.ListProducts(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return products, nil
}

// UpdateProduct updates a product
func (s *Service) UpdateProduct(ctx context.Context, tenantID, schemaName, productID string, req *UpdateProductRequest) (*Product, error) {
	existing, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}

	existing.Name = req.Name
	existing.Description = req.Description
	existing.CategoryID = req.CategoryID
	existing.Unit = req.Unit
	existing.Barcode = req.Barcode
	existing.SupplierID = req.SupplierID
	existing.LeadTimeDays = req.LeadTimeDays
	existing.SaleAccountID = req.SaleAccountID
	existing.PurchaseAccountID = req.PurchaseAccountID
	existing.InventoryAccountID = req.InventoryAccountID
	existing.TrackInventory = req.TrackInventory
	existing.IsActive = req.IsActive
	existing.UpdatedAt = time.Now()

	if req.PurchasePrice != "" {
		existing.PurchasePrice, _ = decimal.NewFromString(req.PurchasePrice)
	}
	if req.SalesPrice != "" {
		existing.SalesPrice, _ = decimal.NewFromString(req.SalesPrice)
	}
	if req.VATRate != "" {
		existing.VATRate, _ = decimal.NewFromString(req.VATRate)
	}
	if req.MinStockLevel != "" {
		existing.MinStockLevel, _ = decimal.NewFromString(req.MinStockLevel)
	}
	if req.ReorderPoint != "" {
		existing.ReorderPoint, _ = decimal.NewFromString(req.ReorderPoint)
	}

	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.UpdateProduct(ctx, schemaName, existing); err != nil {
		return nil, fmt.Errorf("update product: %w", err)
	}

	return existing, nil
}

// DeleteProduct deletes a product
func (s *Service) DeleteProduct(ctx context.Context, tenantID, schemaName, productID string) error {
	if err := s.repo.DeleteProduct(ctx, schemaName, tenantID, productID); err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	return nil
}

// CreateCategory creates a new category
func (s *Service) CreateCategory(ctx context.Context, tenantID, schemaName string, req *CreateCategoryRequest) (*ProductCategory, error) {
	cat := &ProductCategory{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, schemaName, cat); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}

	return cat, nil
}

// GetCategoryByID retrieves a category by ID
func (s *Service) GetCategoryByID(ctx context.Context, tenantID, schemaName, categoryID string) (*ProductCategory, error) {
	cat, err := s.repo.GetCategoryByID(ctx, schemaName, tenantID, categoryID)
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return cat, nil
}

// ListCategories retrieves all categories for a tenant
func (s *Service) ListCategories(ctx context.Context, tenantID, schemaName string) ([]ProductCategory, error) {
	categories, err := s.repo.ListCategories(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return categories, nil
}

// DeleteCategory deletes a category
func (s *Service) DeleteCategory(ctx context.Context, tenantID, schemaName, categoryID string) error {
	if err := s.repo.DeleteCategory(ctx, schemaName, tenantID, categoryID); err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

// CreateWarehouse creates a new warehouse
func (s *Service) CreateWarehouse(ctx context.Context, tenantID, schemaName string, req *CreateWarehouseRequest) (*Warehouse, error) {
	warehouse := &Warehouse{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Code:      req.Code,
		Name:      req.Name,
		Address:   req.Address,
		IsDefault: req.IsDefault,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateWarehouse(ctx, schemaName, warehouse); err != nil {
		return nil, fmt.Errorf("create warehouse: %w", err)
	}

	return warehouse, nil
}

// GetWarehouseByID retrieves a warehouse by ID
func (s *Service) GetWarehouseByID(ctx context.Context, tenantID, schemaName, warehouseID string) (*Warehouse, error) {
	warehouse, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}
	return warehouse, nil
}

// ListWarehouses retrieves all warehouses for a tenant
func (s *Service) ListWarehouses(ctx context.Context, tenantID, schemaName string, activeOnly bool) ([]Warehouse, error) {
	warehouses, err := s.repo.ListWarehouses(ctx, schemaName, tenantID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	return warehouses, nil
}

// UpdateWarehouse updates a warehouse
func (s *Service) UpdateWarehouse(ctx context.Context, tenantID, schemaName, warehouseID string, req *UpdateWarehouseRequest) (*Warehouse, error) {
	existing, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	existing.Name = req.Name
	existing.Address = req.Address
	existing.IsDefault = req.IsDefault
	existing.IsActive = req.IsActive
	existing.UpdatedAt = time.Now()

	if err := s.repo.UpdateWarehouse(ctx, schemaName, existing); err != nil {
		return nil, fmt.Errorf("update warehouse: %w", err)
	}

	return existing, nil
}

// DeleteWarehouse deletes a warehouse
func (s *Service) DeleteWarehouse(ctx context.Context, tenantID, schemaName, warehouseID string) error {
	if err := s.repo.DeleteWarehouse(ctx, schemaName, tenantID, warehouseID); err != nil {
		return fmt.Errorf("delete warehouse: %w", err)
	}
	return nil
}

// AdjustStock adjusts stock level for a product
func (s *Service) AdjustStock(ctx context.Context, tenantID, schemaName string, req *AdjustStockRequest) (*InventoryMovement, error) {
	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	unitCost := decimal.Zero
	if req.UnitCost != "" {
		unitCost, _ = decimal.NewFromString(req.UnitCost)
	}

	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}

	movementType := MovementTypeAdjustment
	if quantity.GreaterThan(decimal.Zero) {
		movementType = MovementTypeIn
	} else if quantity.LessThan(decimal.Zero) {
		movementType = MovementTypeOut
	}

	movement := &InventoryMovement{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		ProductID:    req.ProductID,
		WarehouseID:  req.WarehouseID,
		MovementType: movementType,
		Quantity:     quantity.Abs(),
		UnitCost:     unitCost,
		TotalCost:    quantity.Abs().Mul(unitCost),
		Reference:    "Stock Adjustment",
		Notes:        req.Reason,
		MovementDate: time.Now(),
		CreatedAt:    time.Now(),
		CreatedBy:    req.UserID,
	}

	if err := s.repo.CreateMovement(ctx, schemaName, movement); err != nil {
		return nil, fmt.Errorf("create movement: %w", err)
	}

	// Update product's current stock
	newStock := product.CurrentStock.Add(quantity)
	if err := s.repo.(*PostgresRepository).UpdateProductStock(ctx, schemaName, tenantID, req.ProductID, newStock); err != nil {
		return nil, fmt.Errorf("update product stock: %w", err)
	}

	// Update stock level for warehouse
	stockLevel := &StockLevel{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		ProductID:    req.ProductID,
		WarehouseID:  req.WarehouseID,
		Quantity:     newStock,
		ReservedQty:  decimal.Zero,
		AvailableQty: newStock,
		LastUpdated:  time.Now(),
	}
	if err := s.repo.UpsertStockLevel(ctx, schemaName, stockLevel); err != nil {
		return nil, fmt.Errorf("update stock level: %w", err)
	}

	return movement, nil
}

// TransferStock transfers stock between warehouses
func (s *Service) TransferStock(ctx context.Context, tenantID, schemaName string, req *TransferStockRequest) error {
	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return fmt.Errorf("invalid quantity: %w", err)
	}

	if quantity.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("quantity must be positive")
	}

	// Create OUT movement for source warehouse
	outMovement := &InventoryMovement{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		ProductID:     req.ProductID,
		WarehouseID:   req.FromWarehouseID,
		MovementType:  MovementTypeTransfer,
		Quantity:      quantity,
		UnitCost:      decimal.Zero,
		TotalCost:     decimal.Zero,
		Reference:     "Transfer to " + req.ToWarehouseID,
		ToWarehouseID: req.ToWarehouseID,
		Notes:         req.Notes,
		MovementDate:  time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     req.UserID,
	}

	if err := s.repo.CreateMovement(ctx, schemaName, outMovement); err != nil {
		return fmt.Errorf("create out movement: %w", err)
	}

	// Create IN movement for destination warehouse
	inMovement := &InventoryMovement{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		ProductID:    req.ProductID,
		WarehouseID:  req.ToWarehouseID,
		MovementType: MovementTypeIn,
		Quantity:     quantity,
		UnitCost:     decimal.Zero,
		TotalCost:    decimal.Zero,
		Reference:    "Transfer from " + req.FromWarehouseID,
		Notes:        req.Notes,
		MovementDate: time.Now(),
		CreatedAt:    time.Now(),
		CreatedBy:    req.UserID,
	}

	if err := s.repo.CreateMovement(ctx, schemaName, inMovement); err != nil {
		return fmt.Errorf("create in movement: %w", err)
	}

	return nil
}

// GetStockLevels retrieves stock levels for a product
func (s *Service) GetStockLevels(ctx context.Context, tenantID, schemaName, productID string) ([]StockLevel, error) {
	levels, err := s.repo.GetStockLevelsByProduct(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("get stock levels: %w", err)
	}
	return levels, nil
}

// GetMovements retrieves inventory movements for a product
func (s *Service) GetMovements(ctx context.Context, tenantID, schemaName, productID string) ([]InventoryMovement, error) {
	movements, err := s.repo.ListMovements(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("list movements: %w", err)
	}
	return movements, nil
}
