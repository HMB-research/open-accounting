package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository defines the interface for inventory data operations
type Repository interface {
	// Products
	CreateProduct(ctx context.Context, schemaName string, product *Product) error
	GetProductByID(ctx context.Context, schemaName, tenantID, productID string) (*Product, error)
	ListProducts(ctx context.Context, schemaName, tenantID string, filter *ProductFilter) ([]Product, error)
	UpdateProduct(ctx context.Context, schemaName string, product *Product) error
	DeleteProduct(ctx context.Context, schemaName, tenantID, productID string) error
	GenerateCode(ctx context.Context, schemaName, tenantID string) (string, error)

	// Categories
	CreateCategory(ctx context.Context, schemaName string, category *ProductCategory) error
	GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*ProductCategory, error)
	ListCategories(ctx context.Context, schemaName, tenantID string) ([]ProductCategory, error)
	DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error

	// Warehouses
	CreateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error
	GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*Warehouse, error)
	ListWarehouses(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Warehouse, error)
	UpdateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error
	DeleteWarehouse(ctx context.Context, schemaName, tenantID, warehouseID string) error

	// Stock Levels
	GetStockLevel(ctx context.Context, schemaName, tenantID, productID, warehouseID string) (*StockLevel, error)
	GetStockLevelsByProduct(ctx context.Context, schemaName, tenantID, productID string) ([]StockLevel, error)
	UpsertStockLevel(ctx context.Context, schemaName string, level *StockLevel) error

	// Movements
	CreateMovement(ctx context.Context, schemaName string, movement *InventoryMovement) error
	ListMovements(ctx context.Context, schemaName, tenantID, productID string) ([]InventoryMovement, error)

	// Stock updates
	UpdateProductStock(ctx context.Context, schemaName, tenantID, productID string, newStock decimal.Decimal) error
}

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates an ORM-backed inventory repository.
func NewGORMRepository(pool *pgxpool.Pool) Repository {
	if pool == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create inventory GORM repository: %w", err))
	}
	return &GORMRepository{db: gormDB}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	qualifiedTable, err := qualifiedInventoryTable(schemaName, tableName)
	if err != nil {
		return nil, err
	}
	if r.db == nil {
		return nil, fmt.Errorf("inventory repository database is not configured")
	}
	return r.db.WithContext(ctx).Table(qualifiedTable), nil
}

func qualifiedInventoryTable(schemaName, tableName string) (string, error) {
	return database.QualifiedTable(schemaName, tableName)
}

// CreateProduct creates a new product
func (r *GORMRepository) CreateProduct(ctx context.Context, schemaName string, product *Product) error {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return err
	}
	return db.Create(productCreateValues(product)).Error
}

// GetProductByID retrieves a product by ID
func (r *GORMRepository) GetProductByID(ctx context.Context, schemaName, tenantID, productID string) (*Product, error) {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return nil, err
	}

	var row productRow
	err = db.Select(productSelectColumns()).
		Where("id = ? AND tenant_id = ?", productID, tenantID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("product not found")
	}
	if err != nil {
		return nil, err
	}
	return productFromRow(row), nil
}

// ListProducts retrieves products with optional filtering
func (r *GORMRepository) ListProducts(ctx context.Context, schemaName, tenantID string, filter *ProductFilter) ([]Product, error) {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return nil, err
	}

	query := db.Select(productSelectColumns()).Where("tenant_id = ?", tenantID)
	if filter != nil {
		if filter.ProductType != "" {
			query = query.Where("product_type = ?", filter.ProductType)
		}
		if filter.Status != "" {
			query = query.Where("is_active = ?", filter.Status == ProductStatusActive)
		}
		if filter.CategoryID != "" {
			query = query.Where("category_id = ?", filter.CategoryID)
		}
		if filter.Search != "" {
			searchPattern := "%" + filter.Search + "%"
			query = query.Where("name ILIKE ? OR code ILIKE ?", searchPattern, searchPattern)
		}
		if filter.LowStock {
			query = query.Where("current_stock <= reorder_point")
		}
	}

	var rows []productRow
	if err := query.Order("name ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}

	products := make([]Product, 0, len(rows))
	for _, row := range rows {
		products = append(products, *productFromRow(row))
	}
	return products, nil
}

// UpdateProduct updates a product
func (r *GORMRepository) UpdateProduct(ctx context.Context, schemaName string, product *Product) error {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return err
	}
	return db.Where("id = ? AND tenant_id = ?", product.ID, product.TenantID).
		Updates(productUpdateValues(product)).Error
}

// DeleteProduct deletes a product
func (r *GORMRepository) DeleteProduct(ctx context.Context, schemaName, tenantID, productID string) error {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return err
	}
	return db.Where("id = ? AND tenant_id = ?", productID, tenantID).Delete(map[string]interface{}{}).Error
}

// GenerateCode generates a unique product code
func (r *GORMRepository) GenerateCode(ctx context.Context, schemaName, tenantID string) (string, error) {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return "", err
	}

	var row struct {
		NextNum int `gorm:"column:next_num"`
	}
	if err := db.
		Select("COALESCE(MAX(CAST(SUBSTRING(code FROM 'PRD-([0-9]+)') AS INTEGER)), 0) + 1 AS next_num").
		Where("tenant_id = ? AND code LIKE ?", tenantID, "PRD-%").
		Scan(&row).Error; err != nil {
		return "", err
	}
	if row.NextNum == 0 {
		row.NextNum = 1
	}
	return fmt.Sprintf("PRD-%05d", row.NextNum), nil
}

// CreateCategory creates a new category
func (r *GORMRepository) CreateCategory(ctx context.Context, schemaName string, category *ProductCategory) error {
	db, err := r.tenantTable(ctx, schemaName, "product_categories")
	if err != nil {
		return err
	}
	return db.Create(map[string]interface{}{
		"id":          category.ID,
		"tenant_id":   category.TenantID,
		"name":        category.Name,
		"description": category.Description,
		"parent_id":   nullableString(category.ParentID),
		"created_at":  category.CreatedAt,
		"updated_at":  category.UpdatedAt,
	}).Error
}

// GetCategoryByID retrieves a category by ID
func (r *GORMRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*ProductCategory, error) {
	db, err := r.tenantTable(ctx, schemaName, "product_categories")
	if err != nil {
		return nil, err
	}

	var row productCategoryRow
	err = db.Select("id, tenant_id, name, COALESCE(description, '') AS description, parent_id, created_at, updated_at").
		Where("id = ? AND tenant_id = ?", categoryID, tenantID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("category not found")
	}
	if err != nil {
		return nil, err
	}
	return productCategoryFromRow(row), nil
}

// ListCategories retrieves all categories for a tenant
func (r *GORMRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]ProductCategory, error) {
	db, err := r.tenantTable(ctx, schemaName, "product_categories")
	if err != nil {
		return nil, err
	}

	var rows []productCategoryRow
	if err := db.Select("id, tenant_id, name, COALESCE(description, '') AS description, parent_id, created_at, updated_at").
		Where("tenant_id = ?", tenantID).
		Order("name ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	categories := make([]ProductCategory, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, *productCategoryFromRow(row))
	}
	return categories, nil
}

// DeleteCategory deletes a category
func (r *GORMRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	db, err := r.tenantTable(ctx, schemaName, "product_categories")
	if err != nil {
		return err
	}
	return db.Where("id = ? AND tenant_id = ?", categoryID, tenantID).Delete(map[string]interface{}{}).Error
}

// CreateWarehouse creates a new warehouse
func (r *GORMRepository) CreateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error {
	db, err := r.tenantTable(ctx, schemaName, "warehouses")
	if err != nil {
		return err
	}
	return db.Create(map[string]interface{}{
		"id":         warehouse.ID,
		"tenant_id":  warehouse.TenantID,
		"code":       warehouse.Code,
		"name":       warehouse.Name,
		"address":    warehouse.Address,
		"is_default": warehouse.IsDefault,
		"is_active":  warehouse.IsActive,
		"created_at": warehouse.CreatedAt,
		"updated_at": warehouse.UpdatedAt,
	}).Error
}

// GetWarehouseByID retrieves a warehouse by ID
func (r *GORMRepository) GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*Warehouse, error) {
	db, err := r.tenantTable(ctx, schemaName, "warehouses")
	if err != nil {
		return nil, err
	}

	var warehouse Warehouse
	err = db.Select("id, tenant_id, code, name, COALESCE(address, '') AS address, is_default, is_active, created_at, updated_at").
		Where("id = ? AND tenant_id = ?", warehouseID, tenantID).
		Take(&warehouse).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("warehouse not found")
	}
	if err != nil {
		return nil, err
	}
	return &warehouse, nil
}

// ListWarehouses retrieves all warehouses for a tenant
func (r *GORMRepository) ListWarehouses(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Warehouse, error) {
	db, err := r.tenantTable(ctx, schemaName, "warehouses")
	if err != nil {
		return nil, err
	}

	query := db.Select("id, tenant_id, code, name, COALESCE(address, '') AS address, is_default, is_active, created_at, updated_at").
		Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var warehouses []Warehouse
	if err := query.Order("name ASC").Scan(&warehouses).Error; err != nil {
		return nil, err
	}
	return warehouses, nil
}

// UpdateWarehouse updates a warehouse
func (r *GORMRepository) UpdateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error {
	db, err := r.tenantTable(ctx, schemaName, "warehouses")
	if err != nil {
		return err
	}
	return db.Where("id = ? AND tenant_id = ?", warehouse.ID, warehouse.TenantID).
		Updates(map[string]interface{}{
			"name":       warehouse.Name,
			"address":    warehouse.Address,
			"is_default": warehouse.IsDefault,
			"is_active":  warehouse.IsActive,
			"updated_at": warehouse.UpdatedAt,
		}).Error
}

// DeleteWarehouse deletes a warehouse
func (r *GORMRepository) DeleteWarehouse(ctx context.Context, schemaName, tenantID, warehouseID string) error {
	db, err := r.tenantTable(ctx, schemaName, "warehouses")
	if err != nil {
		return err
	}
	return db.Where("id = ? AND tenant_id = ?", warehouseID, tenantID).Delete(map[string]interface{}{}).Error
}

// GetStockLevel retrieves stock level for a product in a warehouse
func (r *GORMRepository) GetStockLevel(ctx context.Context, schemaName, tenantID, productID, warehouseID string) (*StockLevel, error) {
	db, err := r.tenantTable(ctx, schemaName, "stock_levels")
	if err != nil {
		return nil, err
	}

	var level StockLevel
	err = db.Select("id, tenant_id, product_id, warehouse_id, quantity, reserved_qty, available_qty, last_updated").
		Where("product_id = ? AND warehouse_id = ? AND tenant_id = ?", productID, warehouseID, tenantID).
		Take(&level).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("stock level not found")
	}
	if err != nil {
		return nil, err
	}
	return &level, nil
}

// GetStockLevelsByProduct retrieves all stock levels for a product
func (r *GORMRepository) GetStockLevelsByProduct(ctx context.Context, schemaName, tenantID, productID string) ([]StockLevel, error) {
	db, err := r.tenantTable(ctx, schemaName, "stock_levels")
	if err != nil {
		return nil, err
	}

	var levels []StockLevel
	if err := db.Select("id, tenant_id, product_id, warehouse_id, quantity, reserved_qty, available_qty, last_updated").
		Where("product_id = ? AND tenant_id = ?", productID, tenantID).
		Scan(&levels).Error; err != nil {
		return nil, err
	}
	return levels, nil
}

// UpsertStockLevel creates or updates a stock level
func (r *GORMRepository) UpsertStockLevel(ctx context.Context, schemaName string, level *StockLevel) error {
	db, err := r.tenantTable(ctx, schemaName, "stock_levels")
	if err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "product_id"},
			{Name: "warehouse_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":      level.Quantity,
			"reserved_qty":  level.ReservedQty,
			"available_qty": level.AvailableQty,
			"last_updated":  level.LastUpdated,
		}),
	}).Create(map[string]interface{}{
		"id":            level.ID,
		"tenant_id":     level.TenantID,
		"product_id":    level.ProductID,
		"warehouse_id":  nullableString(level.WarehouseID),
		"quantity":      level.Quantity,
		"reserved_qty":  level.ReservedQty,
		"available_qty": level.AvailableQty,
		"last_updated":  level.LastUpdated,
	}).Error
}

// CreateMovement creates a new inventory movement
func (r *GORMRepository) CreateMovement(ctx context.Context, schemaName string, movement *InventoryMovement) error {
	db, err := r.tenantTable(ctx, schemaName, "inventory_movements")
	if err != nil {
		return err
	}
	return db.Create(map[string]interface{}{
		"id":              movement.ID,
		"tenant_id":       movement.TenantID,
		"product_id":      movement.ProductID,
		"warehouse_id":    movement.WarehouseID,
		"movement_type":   movement.MovementType,
		"quantity":        movement.Quantity,
		"unit_cost":       movement.UnitCost,
		"total_cost":      movement.TotalCost,
		"lot_number":      nullableString(movement.LotNumber),
		"serial_number":   nullableString(movement.SerialNumber),
		"expiry_date":     nullableString(movement.ExpiryDate),
		"reference":       movement.Reference,
		"source_type":     nullableString(movement.SourceType),
		"source_id":       nullableString(movement.SourceID),
		"to_warehouse_id": nullableString(movement.ToWarehouseID),
		"notes":           movement.Notes,
		"movement_date":   movement.MovementDate,
		"created_at":      movement.CreatedAt,
		"created_by":      nullableString(movement.CreatedBy),
	}).Error
}

// ListMovements retrieves inventory movements for a product
func (r *GORMRepository) ListMovements(ctx context.Context, schemaName, tenantID, productID string) ([]InventoryMovement, error) {
	db, err := r.tenantTable(ctx, schemaName, "inventory_movements")
	if err != nil {
		return nil, err
	}

	var rows []inventoryMovementRow
	if err := db.Select(`
			id, tenant_id, product_id, warehouse_id, movement_type, quantity, unit_cost, total_cost,
			COALESCE(lot_number, '') AS lot_number,
			COALESCE(serial_number, '') AS serial_number,
			COALESCE(expiry_date::text, '') AS expiry_date,
			COALESCE(reference, '') AS reference,
			COALESCE(source_type, '') AS source_type,
			COALESCE(source_id::text, '') AS source_id,
			to_warehouse_id,
			COALESCE(notes, '') AS notes,
			movement_date,
			created_at,
			COALESCE(created_by::text, '') AS created_by
		`).
		Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("movement_date DESC, created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	movements := make([]InventoryMovement, 0, len(rows))
	for _, row := range rows {
		movements = append(movements, *inventoryMovementFromRow(row))
	}
	return movements, nil
}

// UpdateProductStock updates the current stock of a product
func (r *GORMRepository) UpdateProductStock(ctx context.Context, schemaName, tenantID, productID string, newStock decimal.Decimal) error {
	db, err := r.tenantTable(ctx, schemaName, "products")
	if err != nil {
		return err
	}
	return db.Where("id = ? AND tenant_id = ?", productID, tenantID).
		Updates(map[string]interface{}{
			"current_stock": newStock,
			"updated_at":    time.Now(),
		}).Error
}

func productSelectColumns() string {
	return `
		id, tenant_id, code, name, COALESCE(description, '') AS description, product_type, category_id,
		COALESCE(unit, 'pcs') AS unit,
		COALESCE(purchase_price, 0) AS purchase_price,
		COALESCE(sale_price, 0) AS sales_price,
		vat_rate,
		COALESCE(min_stock_level, 0) AS min_stock_level,
		COALESCE(current_stock, 0) AS current_stock,
		COALESCE(reorder_point, 0) AS reorder_point,
		sale_account_id,
		purchase_account_id,
		inventory_account_id,
		COALESCE(track_inventory, false) AS track_inventory,
		COALESCE(is_active, true) AS is_active,
		COALESCE(barcode, '') AS barcode,
		supplier_id,
		COALESCE(lead_time_days, 0) AS lead_time_days,
		created_at,
		updated_at
	`
}

func productCreateValues(product *Product) map[string]interface{} {
	return map[string]interface{}{
		"id":                   product.ID,
		"tenant_id":            product.TenantID,
		"code":                 product.Code,
		"name":                 product.Name,
		"description":          product.Description,
		"product_type":         product.ProductType,
		"category_id":          nullableString(product.CategoryID),
		"unit":                 product.Unit,
		"purchase_price":       product.PurchasePrice,
		"sale_price":           product.SalesPrice,
		"vat_rate":             product.VATRate,
		"min_stock_level":      product.MinStockLevel,
		"current_stock":        product.CurrentStock,
		"reorder_point":        product.ReorderPoint,
		"sale_account_id":      nullableString(product.SaleAccountID),
		"purchase_account_id":  nullableString(product.PurchaseAccountID),
		"inventory_account_id": nullableString(product.InventoryAccountID),
		"track_inventory":      product.TrackInventory,
		"is_active":            product.IsActive,
		"barcode":              product.Barcode,
		"supplier_id":          nullableString(product.SupplierID),
		"lead_time_days":       product.LeadTimeDays,
		"created_at":           product.CreatedAt,
		"updated_at":           product.UpdatedAt,
	}
}

func productUpdateValues(product *Product) map[string]interface{} {
	return map[string]interface{}{
		"name":                 product.Name,
		"description":          product.Description,
		"category_id":          nullableString(product.CategoryID),
		"unit":                 product.Unit,
		"purchase_price":       product.PurchasePrice,
		"sale_price":           product.SalesPrice,
		"vat_rate":             product.VATRate,
		"min_stock_level":      product.MinStockLevel,
		"reorder_point":        product.ReorderPoint,
		"sale_account_id":      nullableString(product.SaleAccountID),
		"purchase_account_id":  nullableString(product.PurchaseAccountID),
		"inventory_account_id": nullableString(product.InventoryAccountID),
		"track_inventory":      product.TrackInventory,
		"is_active":            product.IsActive,
		"barcode":              product.Barcode,
		"supplier_id":          nullableString(product.SupplierID),
		"lead_time_days":       product.LeadTimeDays,
		"updated_at":           product.UpdatedAt,
	}
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type productRow struct {
	ID                 string
	TenantID           string
	Code               string
	Name               string
	Description        string
	ProductType        ProductType
	CategoryID         *string
	Unit               string
	PurchasePrice      decimal.Decimal
	SalesPrice         decimal.Decimal `gorm:"column:sales_price"`
	VATRate            decimal.Decimal
	MinStockLevel      decimal.Decimal
	CurrentStock       decimal.Decimal
	ReorderPoint       decimal.Decimal
	SaleAccountID      *string
	PurchaseAccountID  *string
	InventoryAccountID *string
	TrackInventory     bool
	IsActive           bool
	Barcode            string
	SupplierID         *string
	LeadTimeDays       int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func productFromRow(row productRow) *Product {
	return &Product{
		ID:                 row.ID,
		TenantID:           row.TenantID,
		Code:               row.Code,
		Name:               row.Name,
		Description:        row.Description,
		ProductType:        row.ProductType,
		CategoryID:         stringValue(row.CategoryID),
		Unit:               row.Unit,
		PurchasePrice:      row.PurchasePrice,
		SalesPrice:         row.SalesPrice,
		VATRate:            row.VATRate,
		MinStockLevel:      row.MinStockLevel,
		CurrentStock:       row.CurrentStock,
		ReorderPoint:       row.ReorderPoint,
		SaleAccountID:      stringValue(row.SaleAccountID),
		PurchaseAccountID:  stringValue(row.PurchaseAccountID),
		InventoryAccountID: stringValue(row.InventoryAccountID),
		TrackInventory:     row.TrackInventory,
		IsActive:           row.IsActive,
		Barcode:            row.Barcode,
		SupplierID:         stringValue(row.SupplierID),
		LeadTimeDays:       row.LeadTimeDays,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

type productCategoryRow struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	ParentID    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func productCategoryFromRow(row productCategoryRow) *ProductCategory {
	return &ProductCategory{
		ID:          row.ID,
		TenantID:    row.TenantID,
		Name:        row.Name,
		Description: row.Description,
		ParentID:    stringValue(row.ParentID),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

type inventoryMovementRow struct {
	ID            string
	TenantID      string
	ProductID     string
	WarehouseID   string
	MovementType  MovementType
	Quantity      decimal.Decimal
	UnitCost      decimal.Decimal
	TotalCost     decimal.Decimal
	LotNumber     string
	SerialNumber  string
	ExpiryDate    string
	Reference     string
	SourceType    string
	SourceID      string
	ToWarehouseID *string
	Notes         string
	MovementDate  time.Time
	CreatedAt     time.Time
	CreatedBy     string
}

func inventoryMovementFromRow(row inventoryMovementRow) *InventoryMovement {
	return &InventoryMovement{
		ID:            row.ID,
		TenantID:      row.TenantID,
		ProductID:     row.ProductID,
		WarehouseID:   row.WarehouseID,
		MovementType:  row.MovementType,
		Quantity:      row.Quantity,
		UnitCost:      row.UnitCost,
		TotalCost:     row.TotalCost,
		LotNumber:     row.LotNumber,
		SerialNumber:  row.SerialNumber,
		ExpiryDate:    row.ExpiryDate,
		Reference:     row.Reference,
		SourceType:    row.SourceType,
		SourceID:      row.SourceID,
		ToWarehouseID: stringValue(row.ToWarehouseID),
		Notes:         row.Notes,
		MovementDate:  row.MovementDate,
		CreatedAt:     row.CreatedAt,
		CreatedBy:     row.CreatedBy,
	}
}
