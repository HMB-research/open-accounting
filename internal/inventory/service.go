package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides inventory operations
type Service struct {
	repo Repository
}

// NewService creates a new inventory service with an ORM-backed repository.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		repo: NewGORMRepository(db),
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

// GetInventoryValuation returns inventory valuation for tracked goods.
func (s *Service) GetInventoryValuation(ctx context.Context, tenantID, schemaName, warehouseID, method string) (*InventoryValuationReport, error) {
	warehouseID = strings.TrimSpace(warehouseID)
	valuationMethod, err := normalizeInventoryValuationMethod(method)
	if err != nil {
		return nil, err
	}
	if warehouseID != "" {
		if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID); err != nil {
			return nil, fmt.Errorf("get warehouse: %w", err)
		}
	}

	products, err := s.repo.ListProducts(ctx, schemaName, tenantID, &ProductFilter{ProductType: ProductTypeGoods})
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}

	warehouses, err := s.repo.ListWarehouses(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	warehouseByID := make(map[string]Warehouse, len(warehouses))
	for _, warehouse := range warehouses {
		warehouseByID[warehouse.ID] = warehouse
	}

	report := &InventoryValuationReport{
		TenantID:        tenantID,
		WarehouseID:     warehouseID,
		ValuationMethod: valuationMethod,
		Lines:           []InventoryValuationLine{},
		TotalQuantity:   decimal.Zero,
		TotalReserved:   decimal.Zero,
		TotalAvailable:  decimal.Zero,
		TotalValue:      decimal.Zero,
		GeneratedAt:     time.Now(),
	}

	for _, product := range products {
		if product.ProductType != ProductTypeGoods || !product.TrackInventory {
			continue
		}

		levels, err := s.repo.GetStockLevelsByProduct(ctx, schemaName, tenantID, product.ID)
		if err != nil {
			return nil, fmt.Errorf("get stock levels for product %s: %w", product.ID, err)
		}
		valuationQuantity := product.CurrentStock
		if len(levels) > 0 {
			valuationQuantity = decimal.Zero
			for _, level := range levels {
				if level.TenantID != tenantID {
					continue
				}
				if warehouseID != "" && level.WarehouseID != warehouseID {
					continue
				}
				valuationQuantity = valuationQuantity.Add(level.Quantity)
			}
		}

		unitCost, err := s.inventoryValuationUnitCost(ctx, tenantID, schemaName, product, valuationMethod, valuationQuantity)
		if err != nil {
			return nil, err
		}
		if len(levels) == 0 {
			if warehouseID == "" && !product.CurrentStock.IsZero() {
				report.addValuationLine(inventoryValuationLine(product, StockLevel{
					TenantID:     tenantID,
					ProductID:    product.ID,
					Quantity:     product.CurrentStock,
					ReservedQty:  decimal.Zero,
					AvailableQty: product.CurrentStock,
				}, Warehouse{}, unitCost))
			}
			continue
		}

		for _, level := range levels {
			if level.TenantID != tenantID {
				continue
			}
			if warehouseID != "" && level.WarehouseID != warehouseID {
				continue
			}
			report.addValuationLine(inventoryValuationLine(product, level, warehouseByID[level.WarehouseID], unitCost))
		}
	}

	sort.SliceStable(report.Lines, func(i, j int) bool {
		left := report.Lines[i]
		right := report.Lines[j]
		leftKey := strings.Join([]string{left.ProductCode, left.ProductName, left.ProductID, left.WarehouseCode, left.WarehouseName, left.WarehouseID}, "\x00")
		rightKey := strings.Join([]string{right.ProductCode, right.ProductName, right.ProductID, right.WarehouseCode, right.WarehouseName, right.WarehouseID}, "\x00")
		return leftKey < rightKey
	})

	return report, nil
}

func normalizeInventoryValuationMethod(method string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(method), "-", "_"))
	switch normalized {
	case "", "STANDARD", InventoryValuationMethodStandardCost:
		return InventoryValuationMethodStandardCost, nil
	case "WEIGHTED", "AVERAGE_COST", InventoryValuationMethodWeightedAverage:
		return InventoryValuationMethodWeightedAverage, nil
	case InventoryValuationMethodFIFO, "FIFO_LAYERED":
		return InventoryValuationMethodFIFO, nil
	default:
		return "", fmt.Errorf("invalid valuation method: %s", method)
	}
}

func (s *Service) inventoryValuationUnitCost(ctx context.Context, tenantID, schemaName string, product Product, method string, valuationQuantity decimal.Decimal) (decimal.Decimal, error) {
	if method == InventoryValuationMethodStandardCost {
		return product.PurchasePrice, nil
	}

	movements, err := s.repo.ListMovements(ctx, schemaName, tenantID, product.ID)
	if err != nil {
		return decimal.Zero, fmt.Errorf("list movements for product %s: %w", product.ID, err)
	}
	if method == InventoryValuationMethodFIFO {
		return fifoInventoryUnitCost(product, movements, tenantID, valuationQuantity), nil
	}

	totalQuantity := decimal.Zero
	totalCost := decimal.Zero
	for _, movement := range movements {
		if movement.TenantID != tenantID {
			continue
		}
		if movement.MovementType != MovementTypeIn && movement.MovementType != MovementTypeAdjustment {
			continue
		}
		if movement.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}

		movementCost := movement.TotalCost
		if movementCost.IsZero() && movement.UnitCost.GreaterThan(decimal.Zero) {
			movementCost = movement.Quantity.Mul(movement.UnitCost)
		}
		if movementCost.LessThanOrEqual(decimal.Zero) {
			continue
		}

		totalQuantity = totalQuantity.Add(movement.Quantity)
		totalCost = totalCost.Add(movementCost)
	}

	if totalQuantity.IsZero() {
		return product.PurchasePrice, nil
	}
	return totalCost.Div(totalQuantity), nil
}

func fifoInventoryUnitCost(product Product, movements []InventoryMovement, tenantID string, quantity decimal.Decimal) decimal.Decimal {
	if quantity.LessThanOrEqual(decimal.Zero) {
		return product.PurchasePrice
	}

	sort.SliceStable(movements, func(i, j int) bool {
		left := movements[i].MovementDate
		if left.IsZero() {
			left = movements[i].CreatedAt
		}
		right := movements[j].MovementDate
		if right.IsZero() {
			right = movements[j].CreatedAt
		}
		return left.After(right)
	})

	remaining := quantity
	totalValue := decimal.Zero
	for _, movement := range movements {
		if movement.TenantID != tenantID {
			continue
		}
		if movement.MovementType != MovementTypeIn && movement.MovementType != MovementTypeAdjustment {
			continue
		}
		if movement.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}

		unitCost := movement.UnitCost
		if unitCost.LessThanOrEqual(decimal.Zero) && movement.TotalCost.GreaterThan(decimal.Zero) {
			unitCost = movement.TotalCost.Div(movement.Quantity)
		}
		if unitCost.LessThanOrEqual(decimal.Zero) {
			continue
		}

		layerQty := movement.Quantity
		if layerQty.GreaterThan(remaining) {
			layerQty = remaining
		}
		totalValue = totalValue.Add(layerQty.Mul(unitCost))
		remaining = remaining.Sub(layerQty)
		if remaining.IsZero() {
			break
		}
	}

	if remaining.GreaterThan(decimal.Zero) {
		totalValue = totalValue.Add(remaining.Mul(product.PurchasePrice))
	}
	if totalValue.IsZero() {
		return product.PurchasePrice
	}
	return totalValue.Div(quantity)
}

func inventoryValuationLine(product Product, level StockLevel, warehouse Warehouse, unitCost decimal.Decimal) InventoryValuationLine {
	line := InventoryValuationLine{
		ProductID:      product.ID,
		ProductCode:    product.Code,
		ProductName:    product.Name,
		WarehouseID:    level.WarehouseID,
		WarehouseCode:  warehouse.Code,
		WarehouseName:  warehouse.Name,
		Quantity:       level.Quantity,
		ReservedQty:    level.ReservedQty,
		AvailableQty:   level.AvailableQty,
		UnitCost:       unitCost,
		InventoryValue: level.Quantity.Mul(unitCost),
	}
	if line.WarehouseID == "" {
		line.WarehouseCode = "UNASSIGNED"
		line.WarehouseName = "Unassigned"
	}
	return line
}

func (r *InventoryValuationReport) addValuationLine(line InventoryValuationLine) {
	r.Lines = append(r.Lines, line)
	r.TotalQuantity = r.TotalQuantity.Add(line.Quantity)
	r.TotalReserved = r.TotalReserved.Add(line.ReservedQty)
	r.TotalAvailable = r.TotalAvailable.Add(line.AvailableQty)
	r.TotalValue = r.TotalValue.Add(line.InventoryValue)
}

// AdjustStock adjusts stock level for a product
func (s *Service) AdjustStock(ctx context.Context, tenantID, schemaName string, req *AdjustStockRequest) (*InventoryMovement, error) {
	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("invalid quantity: %w", err)
	}

	unitCost := decimal.Zero
	if req.UnitCost != "" {
		unitCost, err = decimal.NewFromString(req.UnitCost)
		if err != nil {
			return nil, fmt.Errorf("invalid unit cost: %w", err)
		}
	}

	lotNumber, serialNumber, expiryDate, err := normalizeMovementTrackingMetadata(req)
	if err != nil {
		return nil, err
	}

	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, req.WarehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	currentLevel, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, req.ProductID, req.WarehouseID)
	if err != nil {
		return nil, fmt.Errorf("get stock level: %w", err)
	}
	newWarehouseStock := currentLevel.Quantity.Add(quantity)
	if newWarehouseStock.IsNegative() {
		return nil, fmt.Errorf("stock adjustment would make warehouse stock negative")
	}
	if newWarehouseStock.LessThan(currentLevel.ReservedQty) {
		return nil, fmt.Errorf("stock adjustment would reduce warehouse stock below reserved quantity")
	}

	newProductStock := product.CurrentStock.Add(quantity)
	if newProductStock.IsNegative() {
		return nil, fmt.Errorf("stock adjustment would make product stock negative")
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
		LotNumber:    lotNumber,
		SerialNumber: serialNumber,
		ExpiryDate:   expiryDate,
		Reference:    "Stock Adjustment",
		Notes:        req.Reason,
		MovementDate: time.Now(),
		CreatedAt:    time.Now(),
		CreatedBy:    req.UserID,
	}

	if err := s.repo.CreateMovement(ctx, schemaName, movement); err != nil {
		return nil, fmt.Errorf("create movement: %w", err)
	}

	if err := s.repo.UpdateProductStock(ctx, schemaName, tenantID, req.ProductID, newProductStock); err != nil {
		return nil, fmt.Errorf("update product stock: %w", err)
	}

	currentLevel.Quantity = newWarehouseStock
	currentLevel.AvailableQty = newWarehouseStock.Sub(currentLevel.ReservedQty)
	currentLevel.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, currentLevel); err != nil {
		return nil, fmt.Errorf("update stock level: %w", err)
	}

	return movement, nil
}

func normalizeMovementTrackingMetadata(req *AdjustStockRequest) (string, string, string, error) {
	lotNumber := strings.TrimSpace(req.LotNumber)
	serialNumber := strings.TrimSpace(req.SerialNumber)
	expiryDate := strings.TrimSpace(req.ExpiryDate)
	if expiryDate != "" {
		if _, err := time.Parse("2006-01-02", expiryDate); err != nil {
			return "", "", "", fmt.Errorf("expiry_date must use YYYY-MM-DD")
		}
	}
	return lotNumber, serialNumber, expiryDate, nil
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
	if req.FromWarehouseID == req.ToWarehouseID {
		return fmt.Errorf("source and destination warehouses must differ")
	}
	if _, err := s.repo.GetProductByID(ctx, schemaName, tenantID, req.ProductID); err != nil {
		return fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, req.FromWarehouseID); err != nil {
		return fmt.Errorf("get source warehouse: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, req.ToWarehouseID); err != nil {
		return fmt.Errorf("get destination warehouse: %w", err)
	}

	sourceLevel, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, req.ProductID, req.FromWarehouseID)
	if err != nil {
		return fmt.Errorf("get source stock level: %w", err)
	}
	destinationLevel, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, req.ProductID, req.ToWarehouseID)
	if err != nil {
		return fmt.Errorf("get destination stock level: %w", err)
	}
	if sourceLevel.AvailableQty.LessThan(quantity) {
		return fmt.Errorf("insufficient available stock in source warehouse")
	}

	outMovement := &InventoryMovement{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		ProductID:     req.ProductID,
		WarehouseID:   req.FromWarehouseID,
		MovementType:  MovementTypeOut,
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

	sourceLevel.Quantity = sourceLevel.Quantity.Sub(quantity)
	sourceLevel.AvailableQty = sourceLevel.AvailableQty.Sub(quantity)
	sourceLevel.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, sourceLevel); err != nil {
		return fmt.Errorf("update source stock level: %w", err)
	}

	destinationLevel.Quantity = destinationLevel.Quantity.Add(quantity)
	destinationLevel.AvailableQty = destinationLevel.AvailableQty.Add(quantity)
	destinationLevel.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, destinationLevel); err != nil {
		return fmt.Errorf("update destination stock level: %w", err)
	}

	return nil
}

// ReserveStock reserves available stock in a warehouse without changing on-hand quantity.
func (s *Service) ReserveStock(ctx context.Context, tenantID, schemaName string, req *StockReservationRequest) (*StockLevel, error) {
	quantity, err := parsePositiveStockQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetProductByID(ctx, schemaName, tenantID, req.ProductID); err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, req.WarehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	level, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, req.ProductID, req.WarehouseID)
	if err != nil {
		return nil, fmt.Errorf("get stock level: %w", err)
	}
	if level.AvailableQty.LessThan(quantity) {
		return nil, fmt.Errorf("insufficient available stock to reserve")
	}

	level.ReservedQty = level.ReservedQty.Add(quantity)
	level.AvailableQty = level.AvailableQty.Sub(quantity)
	level.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, level); err != nil {
		return nil, fmt.Errorf("update stock level: %w", err)
	}

	return level, nil
}

// ReleaseStock releases previously reserved stock back to available quantity.
func (s *Service) ReleaseStock(ctx context.Context, tenantID, schemaName string, req *StockReservationRequest) (*StockLevel, error) {
	quantity, err := parsePositiveStockQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetProductByID(ctx, schemaName, tenantID, req.ProductID); err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, req.WarehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	level, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, req.ProductID, req.WarehouseID)
	if err != nil {
		return nil, fmt.Errorf("get stock level: %w", err)
	}
	if level.ReservedQty.LessThan(quantity) {
		return nil, fmt.Errorf("cannot release more than reserved stock")
	}

	level.ReservedQty = level.ReservedQty.Sub(quantity)
	level.AvailableQty = level.AvailableQty.Add(quantity)
	level.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, level); err != nil {
		return nil, fmt.Errorf("update stock level: %w", err)
	}

	return level, nil
}

func parsePositiveStockQuantity(value string) (decimal.Decimal, error) {
	quantity, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid quantity: %w", err)
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("quantity must be positive")
	}
	return quantity, nil
}

func (s *Service) stockLevelForWarehouse(ctx context.Context, tenantID, schemaName, productID, warehouseID string) (*StockLevel, error) {
	levels, err := s.repo.GetStockLevelsByProduct(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, err
	}

	for _, level := range levels {
		if level.WarehouseID == warehouseID {
			levelCopy := level
			return &levelCopy, nil
		}
	}

	return &StockLevel{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		ProductID:    productID,
		WarehouseID:  warehouseID,
		Quantity:     decimal.Zero,
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.Zero,
		LastUpdated:  time.Now(),
	}, nil
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
