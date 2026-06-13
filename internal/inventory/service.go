package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type accountingLister interface {
	ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.Account, error)
}

type accountingPoster interface {
	accountingLister
	CreateJournalEntry(ctx context.Context, schemaName, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error)
	PostJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID string) error
}

type inventoryLedgerTransactioner interface {
	WithInventoryLedgerTransaction(ctx context.Context, ledger accountingPoster, fn func(repo Repository, ledger accountingPoster) error) error
}

const (
	inventoryIssueSourceTypeDefault     = "INVENTORY_ISSUE"
	inventoryIssueAccountingRoleCOGS    = "COST_OF_GOODS_SOLD"
	inventoryIssueAccountingRoleAsset   = "INVENTORY"
	inventoryIssueAccountingCurrencyEUR = "EUR"
)

// Service provides inventory operations
type Service struct {
	repo     Repository
	accounts accountingLister
	ledger   accountingPoster
}

// NewService creates a new inventory service with an ORM-backed repository.
func NewService(db *pgxpool.Pool) *Service {
	accountingService := accounting.NewService(db)
	return &Service{
		repo:     NewGORMRepository(db),
		accounts: accountingService,
		ledger:   accountingService,
	}
}

// NewServiceWithRepository creates a new inventory service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return NewServiceWithRepositoryAndAccounting(repo, nil)
}

// NewServiceWithRepositoryAndAccounting creates a new inventory service with a custom repository and accounting account lister.
func NewServiceWithRepositoryAndAccounting(repo Repository, accounts accountingLister) *Service {
	service := &Service{
		repo:     repo,
		accounts: accounts,
	}
	if ledger, ok := accounts.(accountingPoster); ok {
		service.ledger = ledger
	}
	return service
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
	categoryID, err := normalizeOptionalInventoryUUIDString(req.CategoryID, "category_id")
	if err != nil {
		return nil, err
	}
	saleAccountID, err := normalizeOptionalInventoryUUIDString(req.SaleAccountID, "sale_account_id")
	if err != nil {
		return nil, err
	}
	purchaseAccountID, err := normalizeOptionalInventoryUUIDString(req.PurchaseAccountID, "purchase_account_id")
	if err != nil {
		return nil, err
	}
	inventoryAccountID, err := normalizeOptionalInventoryUUIDString(req.InventoryAccountID, "inventory_account_id")
	if err != nil {
		return nil, err
	}
	supplierID, err := normalizeOptionalInventoryUUIDString(req.SupplierID, "supplier_id")
	if err != nil {
		return nil, err
	}

	product := &Product{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		Code:               code,
		Name:               req.Name,
		Description:        req.Description,
		ProductType:        productType,
		CategoryID:         categoryID,
		Unit:               unit,
		PurchasePrice:      purchasePrice,
		SalesPrice:         salesPrice,
		VATRate:            vatRate,
		MinStockLevel:      minStockLevel,
		CurrentStock:       decimal.Zero,
		ReorderPoint:       reorderPoint,
		SaleAccountID:      saleAccountID,
		PurchaseAccountID:  purchaseAccountID,
		InventoryAccountID: inventoryAccountID,
		TrackInventory:     req.TrackInventory,
		IsActive:           true,
		Barcode:            req.Barcode,
		SupplierID:         supplierID,
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
	if filter != nil {
		categoryID, err := normalizeOptionalInventoryUUIDString(filter.CategoryID, "category_id")
		if err != nil {
			return nil, err
		}
		filterCopy := *filter
		filterCopy.CategoryID = categoryID
		filter = &filterCopy
	}
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
	categoryID, err := normalizeOptionalInventoryUUIDString(req.CategoryID, "category_id")
	if err != nil {
		return nil, err
	}
	saleAccountID, err := normalizeOptionalInventoryUUIDString(req.SaleAccountID, "sale_account_id")
	if err != nil {
		return nil, err
	}
	purchaseAccountID, err := normalizeOptionalInventoryUUIDString(req.PurchaseAccountID, "purchase_account_id")
	if err != nil {
		return nil, err
	}
	inventoryAccountID, err := normalizeOptionalInventoryUUIDString(req.InventoryAccountID, "inventory_account_id")
	if err != nil {
		return nil, err
	}
	supplierID, err := normalizeOptionalInventoryUUIDString(req.SupplierID, "supplier_id")
	if err != nil {
		return nil, err
	}

	existing.Name = req.Name
	existing.Description = req.Description
	existing.CategoryID = categoryID
	existing.Unit = req.Unit
	existing.Barcode = req.Barcode
	existing.SupplierID = supplierID
	existing.LeadTimeDays = req.LeadTimeDays
	existing.SaleAccountID = saleAccountID
	existing.PurchaseAccountID = purchaseAccountID
	existing.InventoryAccountID = inventoryAccountID
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
	parentID, err := normalizeOptionalInventoryUUIDString(req.ParentID, "parent_id")
	if err != nil {
		return nil, err
	}

	cat := &ProductCategory{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		ParentID:    parentID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, schemaName, cat); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}

	return cat, nil
}

func normalizeOptionalInventoryUUIDString(value string, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%s must be a valid UUID", field)
	}
	return parsedID.String(), nil
}

func normalizeRequiredInventoryUUIDString(value string, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return normalizeOptionalInventoryUUIDString(trimmed, field)
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

	return weightedAverageInventoryUnitCost(product, movements, tenantID), nil
}

func weightedAverageInventoryUnitCost(product Product, movements []InventoryMovement, tenantID string) decimal.Decimal {
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
		return product.PurchasePrice
	}
	return totalCost.Div(totalQuantity)
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

// GetInventoryLotReport returns on-hand stock grouped by lot, serial, and expiry metadata.
func (s *Service) GetInventoryLotReport(ctx context.Context, tenantID, schemaName, productID, warehouseID string, includeEmpty bool) (*InventoryLotReport, error) {
	productID = strings.TrimSpace(productID)
	warehouseID = strings.TrimSpace(warehouseID)

	if warehouseID != "" {
		if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID); err != nil {
			return nil, fmt.Errorf("get warehouse: %w", err)
		}
	}

	products, err := s.inventoryLotReportProducts(ctx, tenantID, schemaName, productID)
	if err != nil {
		return nil, err
	}

	warehouses, err := s.repo.ListWarehouses(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	warehouseByID := make(map[string]Warehouse, len(warehouses))
	for _, warehouse := range warehouses {
		warehouseByID[warehouse.ID] = warehouse
	}

	report := &InventoryLotReport{
		TenantID:      tenantID,
		ProductID:     productID,
		WarehouseID:   warehouseID,
		IncludeEmpty:  includeEmpty,
		Lines:         []InventoryLotLine{},
		TotalQuantity: decimal.Zero,
		TotalValue:    decimal.Zero,
		GeneratedAt:   time.Now(),
	}

	positions := make(map[inventoryLotKey]*inventoryLotAccumulator)
	for _, product := range products {
		if product.ProductType != ProductTypeGoods || !product.TrackInventory {
			continue
		}

		movements, err := s.repo.ListMovements(ctx, schemaName, tenantID, product.ID)
		if err != nil {
			return nil, fmt.Errorf("list movements for product %s: %w", product.ID, err)
		}
		for _, movement := range movements {
			if movement.TenantID != tenantID {
				continue
			}
			addInventoryLotReportMovement(positions, product, warehouseByID, movement, warehouseID)
		}
	}

	lines := make([]InventoryLotLine, 0, len(positions))
	for _, position := range positions {
		line := position.line
		if !includeEmpty && line.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}
		line.UnitCost = position.product.PurchasePrice
		if position.costQuantity.GreaterThan(decimal.Zero) {
			line.UnitCost = position.costTotal.Div(position.costQuantity)
		}
		line.InventoryValue = line.Quantity.Mul(line.UnitCost)
		lines = append(lines, line)
	}

	sort.SliceStable(lines, func(i, j int) bool {
		left := lines[i]
		right := lines[j]
		leftKey := strings.Join([]string{left.ProductCode, left.ProductName, left.ProductID, left.WarehouseCode, left.WarehouseName, left.WarehouseID, left.ExpiryDate, left.LotNumber, left.SerialNumber}, "\x00")
		rightKey := strings.Join([]string{right.ProductCode, right.ProductName, right.ProductID, right.WarehouseCode, right.WarehouseName, right.WarehouseID, right.ExpiryDate, right.LotNumber, right.SerialNumber}, "\x00")
		return leftKey < rightKey
	})

	for _, line := range lines {
		report.addLotLine(line)
	}

	return report, nil
}

func (s *Service) inventoryLotReportProducts(ctx context.Context, tenantID, schemaName, productID string) ([]Product, error) {
	if productID != "" {
		product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
		if err != nil {
			return nil, fmt.Errorf("get product: %w", err)
		}
		return []Product{*product}, nil
	}

	products, err := s.repo.ListProducts(ctx, schemaName, tenantID, &ProductFilter{ProductType: ProductTypeGoods})
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return products, nil
}

type inventoryLotKey struct {
	productID    string
	warehouseID  string
	lotNumber    string
	serialNumber string
	expiryDate   string
}

type inventoryLotAccumulator struct {
	product      Product
	line         InventoryLotLine
	costQuantity decimal.Decimal
	costTotal    decimal.Decimal
}

func addInventoryLotReportMovement(positions map[inventoryLotKey]*inventoryLotAccumulator, product Product, warehouseByID map[string]Warehouse, movement InventoryMovement, warehouseIDFilter string) {
	quantity := movement.Quantity
	if movement.MovementType != MovementTypeAdjustment {
		quantity = quantity.Abs()
	}

	switch movement.MovementType {
	case MovementTypeOut:
		addInventoryLotReportQuantity(positions, product, warehouseByID, movement, warehouseIDFilter, quantity.Neg())
	case MovementTypeTransfer:
		if strings.TrimSpace(movement.ToWarehouseID) == "" {
			addInventoryLotReportQuantity(positions, product, warehouseByID, movement, warehouseIDFilter, quantity)
			return
		}
		addInventoryLotReportQuantity(positions, product, warehouseByID, movement, warehouseIDFilter, quantity.Neg())
		destinationMovement := movement
		destinationMovement.WarehouseID = movement.ToWarehouseID
		addInventoryLotReportQuantity(positions, product, warehouseByID, destinationMovement, warehouseIDFilter, quantity)
	case MovementTypeIn, MovementTypeAdjustment:
		addInventoryLotReportQuantity(positions, product, warehouseByID, movement, warehouseIDFilter, quantity)
	}
}

func addInventoryLotReportQuantity(positions map[inventoryLotKey]*inventoryLotAccumulator, product Product, warehouseByID map[string]Warehouse, movement InventoryMovement, warehouseIDFilter string, quantity decimal.Decimal) {
	if quantity.IsZero() {
		return
	}

	movementWarehouseID := strings.TrimSpace(movement.WarehouseID)
	if warehouseIDFilter != "" && movementWarehouseID != warehouseIDFilter {
		return
	}

	key := inventoryLotKey{
		productID:    product.ID,
		warehouseID:  movementWarehouseID,
		lotNumber:    strings.TrimSpace(movement.LotNumber),
		serialNumber: strings.TrimSpace(movement.SerialNumber),
		expiryDate:   strings.TrimSpace(movement.ExpiryDate),
	}

	position := positions[key]
	if position == nil {
		warehouse := warehouseByID[movementWarehouseID]
		line := InventoryLotLine{
			ProductID:     product.ID,
			ProductCode:   product.Code,
			ProductName:   product.Name,
			WarehouseID:   movementWarehouseID,
			WarehouseCode: warehouse.Code,
			WarehouseName: warehouse.Name,
			LotNumber:     key.lotNumber,
			SerialNumber:  key.serialNumber,
			ExpiryDate:    key.expiryDate,
			Quantity:      decimal.Zero,
			UnitCost:      decimal.Zero,
		}
		if line.WarehouseID == "" {
			line.WarehouseCode = "UNASSIGNED"
			line.WarehouseName = "Unassigned"
		}
		position = &inventoryLotAccumulator{product: product, line: line, costQuantity: decimal.Zero, costTotal: decimal.Zero}
		positions[key] = position
	}

	position.line.Quantity = position.line.Quantity.Add(quantity)
	movementDate := inventoryLotMovementDate(movement)
	if position.line.LastMovementDate.IsZero() || movementDate.After(position.line.LastMovementDate) {
		position.line.LastMovementDate = movementDate
	}

	if quantity.LessThanOrEqual(decimal.Zero) {
		return
	}

	movementCost := movement.TotalCost
	if movementCost.IsZero() && movement.UnitCost.GreaterThan(decimal.Zero) {
		movementCost = quantity.Mul(movement.UnitCost)
	}
	if movementCost.LessThanOrEqual(decimal.Zero) {
		return
	}

	position.costQuantity = position.costQuantity.Add(quantity)
	position.costTotal = position.costTotal.Add(movementCost)
}

func inventoryLotMovementDate(movement InventoryMovement) time.Time {
	if !movement.MovementDate.IsZero() {
		return movement.MovementDate
	}
	return movement.CreatedAt
}

func (r *InventoryLotReport) addLotLine(line InventoryLotLine) {
	r.Lines = append(r.Lines, line)
	r.TotalQuantity = r.TotalQuantity.Add(line.Quantity)
	r.TotalValue = r.TotalValue.Add(line.InventoryValue)
}

func transferInventoryUnitCost(product Product, movements []InventoryMovement, tenantID, sourceWarehouseID, lotNumber, serialNumber, expiryDate string, quantity decimal.Decimal) (decimal.Decimal, error) {
	if hasInventoryTrackingMetadata(lotNumber, serialNumber, expiryDate) {
		position := inventoryLotPositionFromMovements(product, movements, tenantID, sourceWarehouseID, lotNumber, serialNumber, expiryDate)
		if position == nil || position.line.Quantity.LessThan(quantity) {
			return decimal.Zero, fmt.Errorf("insufficient tracked lot stock in source warehouse")
		}
		if position.costQuantity.GreaterThan(decimal.Zero) {
			return position.costTotal.Div(position.costQuantity), nil
		}
	}

	return weightedAverageInventoryUnitCost(product, movements, tenantID), nil
}

func hasInventoryTrackingMetadata(lotNumber, serialNumber, expiryDate string) bool {
	return strings.TrimSpace(lotNumber) != "" || strings.TrimSpace(serialNumber) != "" || strings.TrimSpace(expiryDate) != ""
}

func inventoryLotKeyHasMetadata(key inventoryLotKey) bool {
	return hasInventoryTrackingMetadata(key.lotNumber, key.serialNumber, key.expiryDate)
}

func inventoryLotPositionFromMovements(product Product, movements []InventoryMovement, tenantID, warehouseID, lotNumber, serialNumber, expiryDate string) *inventoryLotAccumulator {
	positions := inventoryLotPositionsFromMovements(product, movements, tenantID, warehouseID)

	return positions[inventoryLotKey{
		productID:    product.ID,
		warehouseID:  strings.TrimSpace(warehouseID),
		lotNumber:    strings.TrimSpace(lotNumber),
		serialNumber: strings.TrimSpace(serialNumber),
		expiryDate:   strings.TrimSpace(expiryDate),
	}]
}

func inventoryLotPositionsFromMovements(product Product, movements []InventoryMovement, tenantID, warehouseID string) map[inventoryLotKey]*inventoryLotAccumulator {
	positions := make(map[inventoryLotKey]*inventoryLotAccumulator)
	for _, movement := range movements {
		if movement.TenantID != tenantID {
			continue
		}
		addInventoryLotReportMovement(positions, product, nil, movement, warehouseID)
	}
	return positions
}

func inventoryLotKeyFromReservation(reservation InventoryLotReservation) inventoryLotKey {
	return inventoryLotKey{
		productID:    reservation.ProductID,
		warehouseID:  reservation.WarehouseID,
		lotNumber:    strings.TrimSpace(reservation.LotNumber),
		serialNumber: strings.TrimSpace(reservation.SerialNumber),
		expiryDate:   strings.TrimSpace(reservation.ExpiryDate),
	}
}

func inventoryLotReservationQuantities(reservations []InventoryLotReservation) map[inventoryLotKey]decimal.Decimal {
	quantities := make(map[inventoryLotKey]decimal.Decimal, len(reservations))
	for _, reservation := range reservations {
		if reservation.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}
		key := inventoryLotKeyFromReservation(reservation)
		quantities[key] = quantities[key].Add(reservation.Quantity)
	}
	return quantities
}

func unallocatedReservedQuantity(totalReserved decimal.Decimal, reservations []InventoryLotReservation) decimal.Decimal {
	trackedReserved := decimal.Zero
	for _, reservation := range reservations {
		if reservation.Quantity.GreaterThan(decimal.Zero) {
			trackedReserved = trackedReserved.Add(reservation.Quantity)
		}
	}
	unallocated := totalReserved.Sub(trackedReserved)
	if unallocated.IsNegative() {
		return decimal.Zero
	}
	return unallocated
}

func sortedInventoryLotKeys(positions map[inventoryLotKey]*inventoryLotAccumulator) []inventoryLotKey {
	keys := make([]inventoryLotKey, 0, len(positions))
	for key, position := range positions {
		if !inventoryLotKeyHasMetadata(key) || position == nil || position.line.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left := keys[i]
		right := keys[j]
		leftKey := strings.Join([]string{left.expiryDate, left.lotNumber, left.serialNumber, left.productID, left.warehouseID}, "\x00")
		rightKey := strings.Join([]string{right.expiryDate, right.lotNumber, right.serialNumber, right.productID, right.warehouseID}, "\x00")
		return leftKey < rightKey
	})
	return keys
}

func minDecimal(left, right decimal.Decimal) decimal.Decimal {
	if left.LessThan(right) {
		return left
	}
	return right
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

	productID, err := normalizeRequiredInventoryUUIDString(req.ProductID, "product_id")
	if err != nil {
		return nil, err
	}
	warehouseID, err := normalizeRequiredInventoryUUIDString(req.WarehouseID, "warehouse_id")
	if err != nil {
		return nil, err
	}

	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	currentLevel, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, productID, warehouseID)
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
		ProductID:    productID,
		WarehouseID:  warehouseID,
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

	if err := s.repo.UpdateProductStock(ctx, schemaName, tenantID, productID, newProductStock); err != nil {
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
	return normalizeMovementTrackingMetadataValues(req.LotNumber, req.SerialNumber, req.ExpiryDate)
}

func normalizeMovementTrackingMetadataValues(lotNumberValue, serialNumberValue, expiryDateValue string) (string, string, string, error) {
	lotNumber := strings.TrimSpace(lotNumberValue)
	serialNumber := strings.TrimSpace(serialNumberValue)
	expiryDate := strings.TrimSpace(expiryDateValue)
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
	productID, err := normalizeRequiredInventoryUUIDString(req.ProductID, "product_id")
	if err != nil {
		return err
	}
	fromWarehouseID, err := normalizeRequiredInventoryUUIDString(req.FromWarehouseID, "from_warehouse_id")
	if err != nil {
		return err
	}
	toWarehouseID, err := normalizeRequiredInventoryUUIDString(req.ToWarehouseID, "to_warehouse_id")
	if err != nil {
		return err
	}
	if fromWarehouseID == toWarehouseID {
		return fmt.Errorf("source and destination warehouses must differ")
	}
	lotNumber, serialNumber, expiryDate, err := normalizeMovementTrackingMetadataValues(req.LotNumber, req.SerialNumber, req.ExpiryDate)
	if err != nil {
		return err
	}
	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
	if err != nil {
		return fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, fromWarehouseID); err != nil {
		return fmt.Errorf("get source warehouse: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, toWarehouseID); err != nil {
		return fmt.Errorf("get destination warehouse: %w", err)
	}

	sourceLevel, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, productID, fromWarehouseID)
	if err != nil {
		return fmt.Errorf("get source stock level: %w", err)
	}
	destinationLevel, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, productID, toWarehouseID)
	if err != nil {
		return fmt.Errorf("get destination stock level: %w", err)
	}
	if sourceLevel.AvailableQty.LessThan(quantity) {
		return fmt.Errorf("insufficient available stock in source warehouse")
	}

	movements, err := s.repo.ListMovements(ctx, schemaName, tenantID, productID)
	if err != nil {
		return fmt.Errorf("list movements for transfer costing: %w", err)
	}
	unitCost, err := transferInventoryUnitCost(*product, movements, tenantID, fromWarehouseID, lotNumber, serialNumber, expiryDate, quantity)
	if err != nil {
		return err
	}
	totalCost := quantity.Mul(unitCost)

	outMovement := &InventoryMovement{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		ProductID:     productID,
		WarehouseID:   fromWarehouseID,
		MovementType:  MovementTypeOut,
		Quantity:      quantity,
		UnitCost:      unitCost,
		TotalCost:     totalCost,
		LotNumber:     lotNumber,
		SerialNumber:  serialNumber,
		ExpiryDate:    expiryDate,
		Reference:     "Transfer to " + toWarehouseID,
		ToWarehouseID: toWarehouseID,
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
		ProductID:    productID,
		WarehouseID:  toWarehouseID,
		MovementType: MovementTypeIn,
		Quantity:     quantity,
		UnitCost:     unitCost,
		TotalCost:    totalCost,
		LotNumber:    lotNumber,
		SerialNumber: serialNumber,
		ExpiryDate:   expiryDate,
		Reference:    "Transfer from " + fromWarehouseID,
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

// IssueStock consumes positive available stock from a warehouse and returns costed movements plus optional accounting lines.
func (s *Service) IssueStock(ctx context.Context, tenantID, schemaName string, req *IssueStockRequest) (*IssueStockResult, error) {
	if req.PostToLedger {
		if transactioner, ok := s.repo.(inventoryLedgerTransactioner); ok {
			var result *IssueStockResult
			err := transactioner.WithInventoryLedgerTransaction(ctx, s.ledger, func(txRepo Repository, txLedger accountingPoster) error {
				if txLedger == nil {
					return fmt.Errorf("accounting transaction is unavailable for issue ledger posting")
				}
				txService := *s
				txService.repo = txRepo
				txService.accounts = txLedger
				txService.ledger = txLedger
				var err error
				result, err = txService.issueStock(ctx, tenantID, schemaName, req)
				return err
			})
			if err != nil {
				return nil, err
			}
			return result, nil
		}
	}
	return s.issueStock(ctx, tenantID, schemaName, req)
}

func (s *Service) issueStock(ctx context.Context, tenantID, schemaName string, req *IssueStockRequest) (*IssueStockResult, error) {
	quantity, err := parsePositiveStockQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	productID, err := normalizeRequiredInventoryUUIDString(req.ProductID, "product_id")
	if err != nil {
		return nil, err
	}
	warehouseID, err := normalizeRequiredInventoryUUIDString(req.WarehouseID, "warehouse_id")
	if err != nil {
		return nil, err
	}
	sourceID, err := normalizeOptionalInventoryUUIDString(req.SourceID, "source_id")
	if err != nil {
		return nil, err
	}
	if req.PostToLedger {
		if strings.TrimSpace(req.UserID) == "" {
			return nil, fmt.Errorf("user id is required to post issue accounting")
		}
		if sourceID == "" {
			sourceID = uuid.New().String()
		}
	}
	lotNumber, serialNumber, expiryDate, err := normalizeMovementTrackingMetadataValues(req.LotNumber, req.SerialNumber, req.ExpiryDate)
	if err != nil {
		return nil, err
	}

	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	level, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, productID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("get stock level: %w", err)
	}
	if level.AvailableQty.LessThan(quantity) {
		return nil, fmt.Errorf("insufficient available stock to issue")
	}
	if product.CurrentStock.LessThan(quantity) {
		return nil, fmt.Errorf("insufficient product stock to issue")
	}

	movements, err := s.repo.ListMovements(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("list movements for issue costing: %w", err)
	}
	allocations, err := s.issueStockAllocations(ctx, tenantID, schemaName, *product, level, movements, quantity, lotNumber, serialNumber, expiryDate)
	if err != nil {
		return nil, err
	}

	totalCost := decimal.Zero
	for _, allocation := range allocations {
		totalCost = totalCost.Add(allocation.totalCost)
	}
	accountingLines, err := s.inventoryIssueAccounting(ctx, schemaName, tenantID, *product, req, sourceID, totalCost)
	if err != nil {
		return nil, err
	}
	if req.PostToLedger {
		if s.ledger == nil {
			return nil, fmt.Errorf("accounting service is unavailable for issue ledger posting")
		}
		if totalCost.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("positive issue cost is required to post issue accounting")
		}
		if accountingLines == nil {
			return nil, fmt.Errorf("cost_of_goods_sold_account_id and inventory_account_id are required to post issue accounting")
		}
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = "Inventory Issue"
	}
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = inventoryIssueSourceTypeDefault
	}
	now := time.Now()
	createdMovements := make([]InventoryMovement, 0, len(allocations))
	for _, allocation := range allocations {
		movement := &InventoryMovement{
			ID:           uuid.New().String(),
			TenantID:     tenantID,
			ProductID:    productID,
			WarehouseID:  warehouseID,
			MovementType: MovementTypeOut,
			Quantity:     allocation.quantity,
			UnitCost:     allocation.unitCost,
			TotalCost:    allocation.totalCost,
			LotNumber:    allocation.key.lotNumber,
			SerialNumber: allocation.key.serialNumber,
			ExpiryDate:   allocation.key.expiryDate,
			Reference:    reference,
			SourceType:   sourceType,
			SourceID:     sourceID,
			Notes:        strings.TrimSpace(req.Reason),
			MovementDate: now,
			CreatedAt:    now,
			CreatedBy:    strings.TrimSpace(req.UserID),
		}
		if err := s.repo.CreateMovement(ctx, schemaName, movement); err != nil {
			return nil, fmt.Errorf("create issue movement: %w", err)
		}
		createdMovements = append(createdMovements, *movement)
	}

	newProductStock := product.CurrentStock.Sub(quantity)
	if err := s.repo.UpdateProductStock(ctx, schemaName, tenantID, productID, newProductStock); err != nil {
		return nil, fmt.Errorf("update product stock: %w", err)
	}

	level.Quantity = level.Quantity.Sub(quantity)
	level.AvailableQty = level.AvailableQty.Sub(quantity)
	level.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, level); err != nil {
		return nil, fmt.Errorf("update stock level: %w", err)
	}
	if err := s.postInventoryIssueAccounting(ctx, schemaName, tenantID, req, accountingLines, now); err != nil {
		return nil, err
	}

	unitCost := decimal.Zero
	if !quantity.IsZero() && totalCost.GreaterThan(decimal.Zero) {
		unitCost = totalCost.Div(quantity)
	}

	return &IssueStockResult{
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    quantity,
		UnitCost:    unitCost,
		TotalCost:   totalCost,
		Movements:   createdMovements,
		StockLevel:  level,
		Accounting:  accountingLines,
	}, nil
}

type inventoryIssueAllocation struct {
	key       inventoryLotKey
	quantity  decimal.Decimal
	unitCost  decimal.Decimal
	totalCost decimal.Decimal
}

func (s *Service) issueStockAllocations(
	ctx context.Context,
	tenantID, schemaName string,
	product Product,
	level *StockLevel,
	movements []InventoryMovement,
	quantity decimal.Decimal,
	lotNumber, serialNumber, expiryDate string,
) ([]inventoryIssueAllocation, error) {
	positions := inventoryLotPositionsFromMovements(product, movements, tenantID, level.WarehouseID)
	reservations, err := s.repo.ListLotReservations(ctx, schemaName, tenantID, product.ID, level.WarehouseID)
	if err != nil {
		return nil, fmt.Errorf("list lot reservations: %w", err)
	}
	reservedByLot := inventoryLotReservationQuantities(reservations)

	if hasInventoryTrackingMetadata(lotNumber, serialNumber, expiryDate) {
		key := inventoryLotKey{
			productID:    product.ID,
			warehouseID:  level.WarehouseID,
			lotNumber:    lotNumber,
			serialNumber: serialNumber,
			expiryDate:   expiryDate,
		}
		position := positions[key]
		available := decimal.Zero
		if position != nil {
			available = position.line.Quantity.Sub(reservedByLot[key])
			available = available.Sub(unallocatedReservedQuantity(level.ReservedQty, reservations))
		}
		if available.LessThan(quantity) {
			return nil, fmt.Errorf("insufficient available tracked lot stock to issue")
		}
		return []inventoryIssueAllocation{newInventoryIssueAllocation(product, key, quantity, inventoryPositionUnitCost(product, position))}, nil
	}

	remaining := quantity
	allocations := make([]inventoryIssueAllocation, 0)
	blockedUnallocated := unallocatedReservedQuantity(level.ReservedQty, reservations)
	for _, key := range sortedInventoryLotKeys(positions) {
		position := positions[key]
		available := position.line.Quantity.Sub(reservedByLot[key])
		if blockedUnallocated.GreaterThan(decimal.Zero) {
			blocked := minDecimal(available, blockedUnallocated)
			available = available.Sub(blocked)
			blockedUnallocated = blockedUnallocated.Sub(blocked)
		}
		if available.LessThanOrEqual(decimal.Zero) {
			continue
		}
		issueQty := minDecimal(available, remaining)
		allocations = append(allocations, newInventoryIssueAllocation(product, key, issueQty, inventoryPositionUnitCost(product, position)))
		remaining = remaining.Sub(issueQty)
		if remaining.IsZero() {
			break
		}
	}

	if remaining.GreaterThan(decimal.Zero) {
		key := inventoryLotKey{productID: product.ID, warehouseID: level.WarehouseID}
		allocations = append(allocations, newInventoryIssueAllocation(product, key, remaining, weightedAverageInventoryUnitCost(product, movements, tenantID)))
	}
	return allocations, nil
}

func inventoryPositionUnitCost(product Product, position *inventoryLotAccumulator) decimal.Decimal {
	if position != nil && position.costQuantity.GreaterThan(decimal.Zero) {
		return position.costTotal.Div(position.costQuantity)
	}
	return product.PurchasePrice
}

func newInventoryIssueAllocation(product Product, key inventoryLotKey, quantity, unitCost decimal.Decimal) inventoryIssueAllocation {
	if unitCost.LessThan(decimal.Zero) {
		unitCost = decimal.Zero
	}
	return inventoryIssueAllocation{
		key:       key,
		quantity:  quantity,
		unitCost:  unitCost,
		totalCost: quantity.Mul(unitCost),
	}
}

func (s *Service) inventoryIssueAccounting(
	ctx context.Context,
	schemaName, tenantID string,
	product Product,
	req *IssueStockRequest,
	sourceID string,
	totalCost decimal.Decimal,
) (*InventoryIssueAccounting, error) {
	cogsAccountID, err := normalizeOptionalInventoryUUIDString(req.CostOfGoodsSoldAccountID, "cost_of_goods_sold_account_id")
	if err != nil {
		return nil, err
	}
	inventoryAccountIDValue := firstInventoryNonEmpty(req.InventoryAccountID, product.InventoryAccountID)
	inventoryAccountID, err := normalizeOptionalInventoryUUIDString(inventoryAccountIDValue, "inventory_account_id")
	if err != nil {
		return nil, err
	}
	if cogsAccountID == "" && strings.TrimSpace(req.InventoryAccountID) == "" && !req.PostToLedger {
		return nil, nil
	}
	if cogsAccountID == "" || inventoryAccountID == "" {
		return nil, fmt.Errorf("cost_of_goods_sold_account_id and inventory_account_id are both required for issue accounting")
	}
	if err := s.validateInventoryIssueAccounts(ctx, schemaName, tenantID, cogsAccountID, inventoryAccountID); err != nil {
		return nil, err
	}
	if totalCost.LessThanOrEqual(decimal.Zero) {
		return nil, nil
	}

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		reference = "Inventory Issue"
	}
	sourceType := strings.TrimSpace(req.SourceType)
	if sourceType == "" {
		sourceType = inventoryIssueSourceTypeDefault
	}
	description := fmt.Sprintf("Issue stock for %s", product.Name)
	return &InventoryIssueAccounting{
		SourceType:  sourceType,
		SourceID:    sourceID,
		Reference:   reference,
		Description: description,
		Lines: []InventoryIssueAccountingLine{
			{
				Role:        inventoryIssueAccountingRoleCOGS,
				AccountID:   cogsAccountID,
				Description: description,
				DebitAmount: totalCost,
				Currency:    inventoryIssueAccountingCurrencyEUR,
			},
			{
				Role:         inventoryIssueAccountingRoleAsset,
				AccountID:    inventoryAccountID,
				Description:  description,
				CreditAmount: totalCost,
				Currency:     inventoryIssueAccountingCurrencyEUR,
			},
		},
	}, nil
}

func (s *Service) postInventoryIssueAccounting(
	ctx context.Context,
	schemaName, tenantID string,
	req *IssueStockRequest,
	issueAccounting *InventoryIssueAccounting,
	issuedAt time.Time,
) error {
	if !req.PostToLedger {
		return nil
	}
	if issueAccounting == nil {
		return fmt.Errorf("issue accounting lines are required for ledger posting")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return fmt.Errorf("user id is required to post issue accounting")
	}

	sourceID := strings.TrimSpace(issueAccounting.SourceID)
	var sourceIDPtr *string
	if sourceID != "" {
		sourceIDPtr = &sourceID
	}
	lines := make([]accounting.CreateJournalEntryLineReq, 0, len(issueAccounting.Lines))
	for _, line := range issueAccounting.Lines {
		lines = append(lines, accounting.CreateJournalEntryLineReq{
			AccountID:    line.AccountID,
			Description:  line.Description,
			DebitAmount:  line.DebitAmount,
			CreditAmount: line.CreditAmount,
			Currency:     line.Currency,
			ExchangeRate: decimal.NewFromInt(1),
		})
	}

	entry, err := s.ledger.CreateJournalEntry(ctx, schemaName, tenantID, &accounting.CreateJournalEntryRequest{
		EntryDate:   issuedAt,
		Description: issueAccounting.Description,
		Reference:   issueAccounting.Reference,
		SourceType:  issueAccounting.SourceType,
		SourceID:    sourceIDPtr,
		UserID:      userID,
		Lines:       lines,
	})
	if err != nil {
		return fmt.Errorf("create inventory issue journal entry: %w", err)
	}
	if err := s.ledger.PostJournalEntry(ctx, schemaName, tenantID, entry.ID, userID); err != nil {
		return fmt.Errorf("post inventory issue journal entry: %w", err)
	}
	issueAccounting.Posted = true
	issueAccounting.JournalID = entry.ID
	issueAccounting.JournalNo = entry.EntryNumber
	return nil
}

func (s *Service) validateInventoryIssueAccounts(ctx context.Context, schemaName, tenantID, cogsAccountID, inventoryAccountID string) error {
	if s.accounts == nil {
		return nil
	}
	accounts, err := s.accounts.ListAccounts(ctx, schemaName, tenantID, false)
	if err != nil {
		return fmt.Errorf("list accounts for issue accounting: %w", err)
	}
	byID := make(map[string]accounting.Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}
	cogsAccount, ok := byID[cogsAccountID]
	if !ok {
		return fmt.Errorf("cost_of_goods_sold_account_id was not found")
	}
	if cogsAccount.AccountType != accounting.AccountTypeExpense {
		return fmt.Errorf("cost_of_goods_sold_account_id must reference an EXPENSE account")
	}
	inventoryAccount, ok := byID[inventoryAccountID]
	if !ok {
		return fmt.Errorf("inventory_account_id was not found")
	}
	if inventoryAccount.AccountType != accounting.AccountTypeAsset {
		return fmt.Errorf("inventory_account_id must reference an ASSET account")
	}
	return nil
}

func firstInventoryNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// ReserveStock reserves available stock in a warehouse without changing on-hand quantity.
func (s *Service) ReserveStock(ctx context.Context, tenantID, schemaName string, req *StockReservationRequest) (*StockLevel, error) {
	quantity, err := parsePositiveStockQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	productID, err := normalizeRequiredInventoryUUIDString(req.ProductID, "product_id")
	if err != nil {
		return nil, err
	}
	warehouseID, err := normalizeRequiredInventoryUUIDString(req.WarehouseID, "warehouse_id")
	if err != nil {
		return nil, err
	}
	req.ProductID = productID
	req.WarehouseID = warehouseID
	product, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	level, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, productID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("get stock level: %w", err)
	}
	if level.AvailableQty.LessThan(quantity) {
		return nil, fmt.Errorf("insufficient available stock to reserve")
	}
	if err := s.reserveLotAllocations(ctx, tenantID, schemaName, *product, level, req, quantity); err != nil {
		return nil, err
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
	productID, err := normalizeRequiredInventoryUUIDString(req.ProductID, "product_id")
	if err != nil {
		return nil, err
	}
	warehouseID, err := normalizeRequiredInventoryUUIDString(req.WarehouseID, "warehouse_id")
	if err != nil {
		return nil, err
	}
	req.ProductID = productID
	req.WarehouseID = warehouseID
	if _, err := s.repo.GetProductByID(ctx, schemaName, tenantID, productID); err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if _, err := s.repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID); err != nil {
		return nil, fmt.Errorf("get warehouse: %w", err)
	}

	level, err := s.stockLevelForWarehouse(ctx, tenantID, schemaName, productID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("get stock level: %w", err)
	}
	if level.ReservedQty.LessThan(quantity) {
		return nil, fmt.Errorf("cannot release more than reserved stock")
	}
	if err := s.releaseLotAllocations(ctx, tenantID, schemaName, req, quantity); err != nil {
		return nil, err
	}

	level.ReservedQty = level.ReservedQty.Sub(quantity)
	level.AvailableQty = level.AvailableQty.Add(quantity)
	level.LastUpdated = time.Now()
	if err := s.repo.UpsertStockLevel(ctx, schemaName, level); err != nil {
		return nil, fmt.Errorf("update stock level: %w", err)
	}

	return level, nil
}

func (s *Service) reserveLotAllocations(ctx context.Context, tenantID, schemaName string, product Product, level *StockLevel, req *StockReservationRequest, quantity decimal.Decimal) error {
	movements, err := s.repo.ListMovements(ctx, schemaName, tenantID, product.ID)
	if err != nil {
		return fmt.Errorf("list movements for product %s: %w", product.ID, err)
	}
	positions := inventoryLotPositionsFromMovements(product, movements, tenantID, level.WarehouseID)
	if len(positions) == 0 {
		if hasInventoryTrackingMetadata(req.LotNumber, req.SerialNumber, req.ExpiryDate) {
			return fmt.Errorf("insufficient available tracked lot stock to reserve")
		}
		return nil
	}

	reservations, err := s.repo.ListLotReservations(ctx, schemaName, tenantID, product.ID, level.WarehouseID)
	if err != nil {
		return fmt.Errorf("list lot reservations: %w", err)
	}
	reservedByLot := inventoryLotReservationQuantities(reservations)

	if hasInventoryTrackingMetadata(req.LotNumber, req.SerialNumber, req.ExpiryDate) {
		key := inventoryLotKey{
			productID:    product.ID,
			warehouseID:  level.WarehouseID,
			lotNumber:    strings.TrimSpace(req.LotNumber),
			serialNumber: strings.TrimSpace(req.SerialNumber),
			expiryDate:   strings.TrimSpace(req.ExpiryDate),
		}
		position := positions[key]
		available := decimal.Zero
		if position != nil {
			available = position.line.Quantity.Sub(reservedByLot[key])
			available = available.Sub(unallocatedReservedQuantity(level.ReservedQty, reservations))
		}
		if available.LessThan(quantity) {
			return fmt.Errorf("insufficient available tracked lot stock to reserve")
		}
		return s.createLotReservation(ctx, schemaName, tenantID, product.ID, level.WarehouseID, key.lotNumber, key.serialNumber, key.expiryDate, quantity, req.Reason, req.UserID)
	}

	remaining := quantity
	for _, key := range sortedInventoryLotKeys(positions) {
		position := positions[key]
		available := position.line.Quantity.Sub(reservedByLot[key])
		if available.LessThanOrEqual(decimal.Zero) {
			continue
		}
		reserveQty := minDecimal(available, remaining)
		if err := s.createLotReservation(ctx, schemaName, tenantID, product.ID, level.WarehouseID, key.lotNumber, key.serialNumber, key.expiryDate, reserveQty, req.Reason, req.UserID); err != nil {
			return err
		}
		remaining = remaining.Sub(reserveQty)
		if remaining.IsZero() {
			break
		}
	}

	return nil
}

func (s *Service) createLotReservation(ctx context.Context, schemaName, tenantID, productID, warehouseID, lotNumber, serialNumber, expiryDate string, quantity decimal.Decimal, reason, userID string) error {
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	now := time.Now()
	reservation := &InventoryLotReservation{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		ProductID:    productID,
		WarehouseID:  warehouseID,
		LotNumber:    strings.TrimSpace(lotNumber),
		SerialNumber: strings.TrimSpace(serialNumber),
		ExpiryDate:   strings.TrimSpace(expiryDate),
		Quantity:     quantity,
		Reason:       strings.TrimSpace(reason),
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    strings.TrimSpace(userID),
	}
	if err := s.repo.UpsertLotReservation(ctx, schemaName, reservation); err != nil {
		return fmt.Errorf("reserve tracked lot stock: %w", err)
	}
	return nil
}

func (s *Service) releaseLotAllocations(ctx context.Context, tenantID, schemaName string, req *StockReservationRequest, quantity decimal.Decimal) error {
	reservations, err := s.repo.ListLotReservations(ctx, schemaName, tenantID, req.ProductID, req.WarehouseID)
	if err != nil {
		return fmt.Errorf("list lot reservations: %w", err)
	}

	if hasInventoryTrackingMetadata(req.LotNumber, req.SerialNumber, req.ExpiryDate) {
		key := inventoryLotKey{
			productID:    req.ProductID,
			warehouseID:  req.WarehouseID,
			lotNumber:    strings.TrimSpace(req.LotNumber),
			serialNumber: strings.TrimSpace(req.SerialNumber),
			expiryDate:   strings.TrimSpace(req.ExpiryDate),
		}
		reservedByLot := inventoryLotReservationQuantities(reservations)
		if reservedByLot[key].LessThan(quantity) {
			return fmt.Errorf("cannot release more than reserved tracked lot stock")
		}
		_, err := s.repo.ReleaseLotReservation(ctx, schemaName, tenantID, req.ProductID, req.WarehouseID, key.lotNumber, key.serialNumber, key.expiryDate, quantity, req.Reason, req.UserID)
		if err != nil {
			return fmt.Errorf("release tracked lot stock: %w", err)
		}
		return nil
	}

	remaining := quantity
	for _, reservation := range reservations {
		if remaining.IsZero() {
			break
		}
		releaseQty := minDecimal(reservation.Quantity, remaining)
		_, err := s.repo.ReleaseLotReservation(ctx, schemaName, tenantID, req.ProductID, req.WarehouseID, reservation.LotNumber, reservation.SerialNumber, reservation.ExpiryDate, releaseQty, req.Reason, req.UserID)
		if err != nil {
			return fmt.Errorf("release tracked lot stock: %w", err)
		}
		remaining = remaining.Sub(releaseQty)
	}

	return nil
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
