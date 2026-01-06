package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
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

// PostgresRepository implements Repository for PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

// execInSchema executes a query in the specified schema
func (r *PostgresRepository) execInSchema(ctx context.Context, schemaName, query string, args ...interface{}) error {
	setSchemaQuery := fmt.Sprintf("SET search_path TO %s", schemaName)
	_, err := r.db.Exec(ctx, setSchemaQuery)
	if err != nil {
		return fmt.Errorf("set schema: %w", err)
	}
	_, err = r.db.Exec(ctx, query, args...)
	return err
}

// queryInSchema executes a query in the specified schema and returns rows
func (r *PostgresRepository) queryInSchema(ctx context.Context, schemaName, query string, args ...interface{}) (pgx.Rows, error) {
	setSchemaQuery := fmt.Sprintf("SET search_path TO %s", schemaName)
	_, err := r.db.Exec(ctx, setSchemaQuery)
	if err != nil {
		return nil, fmt.Errorf("set schema: %w", err)
	}
	return r.db.Query(ctx, query, args...)
}

// queryRowInSchema executes a query in the specified schema and returns a single row
func (r *PostgresRepository) queryRowInSchema(ctx context.Context, schemaName, query string, args ...interface{}) pgx.Row {
	setSchemaQuery := fmt.Sprintf("SET search_path TO %s", schemaName)
	_, _ = r.db.Exec(ctx, setSchemaQuery)
	return r.db.QueryRow(ctx, query, args...)
}

// CreateProduct creates a new product
func (r *PostgresRepository) CreateProduct(ctx context.Context, schemaName string, product *Product) error {
	query := `
		INSERT INTO products (
			id, tenant_id, code, name, description, product_type, category_id, unit,
			purchase_price, sale_price, vat_rate, min_stock_level, current_stock,
			reorder_point, sale_account_id, purchase_account_id, inventory_account_id,
			track_inventory, is_active, barcode, supplier_id, lead_time_days, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)`

	return r.execInSchema(ctx, schemaName, query,
		product.ID, product.TenantID, product.Code, product.Name, product.Description,
		product.ProductType, nullIfEmpty(product.CategoryID), product.Unit,
		product.PurchasePrice, product.SalesPrice, product.VATRate,
		product.MinStockLevel, product.CurrentStock, product.ReorderPoint,
		nullIfEmpty(product.SaleAccountID), nullIfEmpty(product.PurchaseAccountID),
		nullIfEmpty(product.InventoryAccountID), product.TrackInventory, product.IsActive, product.Barcode,
		nullIfEmpty(product.SupplierID), product.LeadTimeDays, product.CreatedAt, product.UpdatedAt,
	)
}

// GetProductByID retrieves a product by ID
func (r *PostgresRepository) GetProductByID(ctx context.Context, schemaName, tenantID, productID string) (*Product, error) {
	query := `
		SELECT id, tenant_id, code, name, COALESCE(description, ''), product_type, category_id, COALESCE(unit, 'pcs'),
			COALESCE(purchase_price, 0), COALESCE(sale_price, 0), vat_rate,
			COALESCE(min_stock_level, 0), COALESCE(current_stock, 0), COALESCE(reorder_point, 0),
			sale_account_id, purchase_account_id, inventory_account_id,
			COALESCE(track_inventory, false), COALESCE(is_active, true), COALESCE(barcode, ''),
			supplier_id, COALESCE(lead_time_days, 0), created_at, updated_at
		FROM products
		WHERE id = $1 AND tenant_id = $2`

	row := r.queryRowInSchema(ctx, schemaName, query, productID, tenantID)

	var p Product
	var categoryID, saleAcctID, purchaseAcctID, inventoryAcctID, supplierID *string
	err := row.Scan(
		&p.ID, &p.TenantID, &p.Code, &p.Name, &p.Description, &p.ProductType,
		&categoryID, &p.Unit, &p.PurchasePrice, &p.SalesPrice, &p.VATRate,
		&p.MinStockLevel, &p.CurrentStock, &p.ReorderPoint,
		&saleAcctID, &purchaseAcctID, &inventoryAcctID, &p.TrackInventory, &p.IsActive, &p.Barcode,
		&supplierID, &p.LeadTimeDays, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("product not found")
		}
		return nil, err
	}

	if categoryID != nil {
		p.CategoryID = *categoryID
	}
	if saleAcctID != nil {
		p.SaleAccountID = *saleAcctID
	}
	if purchaseAcctID != nil {
		p.PurchaseAccountID = *purchaseAcctID
	}
	if inventoryAcctID != nil {
		p.InventoryAccountID = *inventoryAcctID
	}
	if supplierID != nil {
		p.SupplierID = *supplierID
	}

	return &p, nil
}

// ListProducts retrieves products with optional filtering
func (r *PostgresRepository) ListProducts(ctx context.Context, schemaName, tenantID string, filter *ProductFilter) ([]Product, error) {
	query := `
		SELECT id, tenant_id, code, name, COALESCE(description, ''), product_type, category_id, COALESCE(unit, 'pcs'),
			COALESCE(purchase_price, 0), COALESCE(sale_price, 0), vat_rate,
			COALESCE(min_stock_level, 0), COALESCE(current_stock, 0), COALESCE(reorder_point, 0),
			sale_account_id, purchase_account_id, inventory_account_id,
			COALESCE(track_inventory, false), COALESCE(is_active, true), COALESCE(barcode, ''),
			supplier_id, COALESCE(lead_time_days, 0), created_at, updated_at
		FROM products
		WHERE tenant_id = $1`

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.ProductType != "" {
			query += fmt.Sprintf(" AND product_type = $%d", argNum)
			args = append(args, filter.ProductType)
			argNum++
		}
		if filter.Status != "" {
			// Map status to is_active
			isActive := filter.Status == "ACTIVE"
			query += fmt.Sprintf(" AND is_active = $%d", argNum)
			args = append(args, isActive)
			argNum++
		}
		if filter.CategoryID != "" {
			query += fmt.Sprintf(" AND category_id = $%d", argNum)
			args = append(args, filter.CategoryID)
			argNum++
		}
		if filter.Search != "" {
			query += fmt.Sprintf(" AND (name ILIKE $%d OR code ILIKE $%d)", argNum, argNum)
			args = append(args, "%"+filter.Search+"%")
			// argNum not incremented as it's the last filter
		}
		if filter.LowStock {
			query += " AND current_stock <= reorder_point"
		}
	}

	query += " ORDER BY name"

	rows, err := r.queryInSchema(ctx, schemaName, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		var categoryID, saleAcctID, purchaseAcctID, inventoryAcctID, supplierID *string
		err := rows.Scan(
			&p.ID, &p.TenantID, &p.Code, &p.Name, &p.Description, &p.ProductType,
			&categoryID, &p.Unit, &p.PurchasePrice, &p.SalesPrice, &p.VATRate,
			&p.MinStockLevel, &p.CurrentStock, &p.ReorderPoint,
			&saleAcctID, &purchaseAcctID, &inventoryAcctID, &p.TrackInventory, &p.IsActive, &p.Barcode,
			&supplierID, &p.LeadTimeDays, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if categoryID != nil {
			p.CategoryID = *categoryID
		}
		if saleAcctID != nil {
			p.SaleAccountID = *saleAcctID
		}
		if purchaseAcctID != nil {
			p.PurchaseAccountID = *purchaseAcctID
		}
		if inventoryAcctID != nil {
			p.InventoryAccountID = *inventoryAcctID
		}
		if supplierID != nil {
			p.SupplierID = *supplierID
		}

		products = append(products, p)
	}

	return products, nil
}

// UpdateProduct updates a product
func (r *PostgresRepository) UpdateProduct(ctx context.Context, schemaName string, product *Product) error {
	query := `
		UPDATE products SET
			name = $1, description = $2, category_id = $3, unit = $4,
			purchase_price = $5, sale_price = $6, vat_rate = $7,
			min_stock_level = $8, reorder_point = $9,
			sale_account_id = $10, purchase_account_id = $11, inventory_account_id = $12,
			track_inventory = $13, is_active = $14, barcode = $15, supplier_id = $16, lead_time_days = $17, updated_at = $18
		WHERE id = $19 AND tenant_id = $20`

	return r.execInSchema(ctx, schemaName, query,
		product.Name, product.Description, nullIfEmpty(product.CategoryID), product.Unit,
		product.PurchasePrice, product.SalesPrice, product.VATRate,
		product.MinStockLevel, product.ReorderPoint,
		nullIfEmpty(product.SaleAccountID), nullIfEmpty(product.PurchaseAccountID),
		nullIfEmpty(product.InventoryAccountID), product.TrackInventory, product.IsActive, product.Barcode,
		nullIfEmpty(product.SupplierID), product.LeadTimeDays, product.UpdatedAt,
		product.ID, product.TenantID,
	)
}

// DeleteProduct deletes a product
func (r *PostgresRepository) DeleteProduct(ctx context.Context, schemaName, tenantID, productID string) error {
	query := `DELETE FROM products WHERE id = $1 AND tenant_id = $2`
	return r.execInSchema(ctx, schemaName, query, productID, tenantID)
}

// GenerateCode generates a unique product code
func (r *PostgresRepository) GenerateCode(ctx context.Context, schemaName, tenantID string) (string, error) {
	query := `SELECT COALESCE(MAX(CAST(SUBSTRING(code FROM 'PRD-([0-9]+)') AS INTEGER)), 0) + 1 FROM products WHERE tenant_id = $1 AND code LIKE 'PRD-%'`
	row := r.queryRowInSchema(ctx, schemaName, query, tenantID)

	var nextNum int
	if err := row.Scan(&nextNum); err != nil {
		nextNum = 1
	}

	return fmt.Sprintf("PRD-%05d", nextNum), nil
}

// CreateCategory creates a new category
func (r *PostgresRepository) CreateCategory(ctx context.Context, schemaName string, category *ProductCategory) error {
	query := `INSERT INTO product_categories (id, tenant_id, name, description, parent_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	return r.execInSchema(ctx, schemaName, query,
		category.ID, category.TenantID, category.Name, category.Description,
		nullIfEmpty(category.ParentID), category.CreatedAt, category.UpdatedAt,
	)
}

// GetCategoryByID retrieves a category by ID
func (r *PostgresRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*ProductCategory, error) {
	query := `SELECT id, tenant_id, name, COALESCE(description, ''), parent_id, created_at, updated_at FROM product_categories WHERE id = $1 AND tenant_id = $2`
	row := r.queryRowInSchema(ctx, schemaName, query, categoryID, tenantID)

	var c ProductCategory
	var parentID *string
	err := row.Scan(&c.ID, &c.TenantID, &c.Name, &c.Description, &parentID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, err
	}

	if parentID != nil {
		c.ParentID = *parentID
	}

	return &c, nil
}

// ListCategories retrieves all categories for a tenant
func (r *PostgresRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]ProductCategory, error) {
	query := `SELECT id, tenant_id, name, COALESCE(description, ''), parent_id, created_at, updated_at FROM product_categories WHERE tenant_id = $1 ORDER BY name`
	rows, err := r.queryInSchema(ctx, schemaName, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []ProductCategory
	for rows.Next() {
		var c ProductCategory
		var parentID *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Name, &c.Description, &parentID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if parentID != nil {
			c.ParentID = *parentID
		}
		categories = append(categories, c)
	}

	return categories, nil
}

// DeleteCategory deletes a category
func (r *PostgresRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	query := `DELETE FROM product_categories WHERE id = $1 AND tenant_id = $2`
	return r.execInSchema(ctx, schemaName, query, categoryID, tenantID)
}

// CreateWarehouse creates a new warehouse
func (r *PostgresRepository) CreateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error {
	query := `INSERT INTO warehouses (id, tenant_id, code, name, address, is_default, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	return r.execInSchema(ctx, schemaName, query,
		warehouse.ID, warehouse.TenantID, warehouse.Code, warehouse.Name, warehouse.Address,
		warehouse.IsDefault, warehouse.IsActive, warehouse.CreatedAt, warehouse.UpdatedAt,
	)
}

// GetWarehouseByID retrieves a warehouse by ID
func (r *PostgresRepository) GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*Warehouse, error) {
	query := `SELECT id, tenant_id, code, name, COALESCE(address, ''), is_default, is_active, created_at, updated_at FROM warehouses WHERE id = $1 AND tenant_id = $2`
	row := r.queryRowInSchema(ctx, schemaName, query, warehouseID, tenantID)

	var w Warehouse
	err := row.Scan(&w.ID, &w.TenantID, &w.Code, &w.Name, &w.Address, &w.IsDefault, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("warehouse not found")
		}
		return nil, err
	}

	return &w, nil
}

// ListWarehouses retrieves all warehouses for a tenant
func (r *PostgresRepository) ListWarehouses(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Warehouse, error) {
	query := `SELECT id, tenant_id, code, name, COALESCE(address, ''), is_default, is_active, created_at, updated_at FROM warehouses WHERE tenant_id = $1`
	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY name"

	rows, err := r.queryInSchema(ctx, schemaName, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var warehouses []Warehouse
	for rows.Next() {
		var w Warehouse
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Code, &w.Name, &w.Address, &w.IsDefault, &w.IsActive, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		warehouses = append(warehouses, w)
	}

	return warehouses, nil
}

// UpdateWarehouse updates a warehouse
func (r *PostgresRepository) UpdateWarehouse(ctx context.Context, schemaName string, warehouse *Warehouse) error {
	query := `UPDATE warehouses SET name = $1, address = $2, is_default = $3, is_active = $4, updated_at = $5 WHERE id = $6 AND tenant_id = $7`
	return r.execInSchema(ctx, schemaName, query,
		warehouse.Name, warehouse.Address, warehouse.IsDefault, warehouse.IsActive, warehouse.UpdatedAt,
		warehouse.ID, warehouse.TenantID,
	)
}

// DeleteWarehouse deletes a warehouse
func (r *PostgresRepository) DeleteWarehouse(ctx context.Context, schemaName, tenantID, warehouseID string) error {
	query := `DELETE FROM warehouses WHERE id = $1 AND tenant_id = $2`
	return r.execInSchema(ctx, schemaName, query, warehouseID, tenantID)
}

// GetStockLevel retrieves stock level for a product in a warehouse
func (r *PostgresRepository) GetStockLevel(ctx context.Context, schemaName, tenantID, productID, warehouseID string) (*StockLevel, error) {
	query := `SELECT id, tenant_id, product_id, warehouse_id, quantity, reserved_qty, available_qty, last_updated FROM stock_levels WHERE product_id = $1 AND warehouse_id = $2 AND tenant_id = $3`
	row := r.queryRowInSchema(ctx, schemaName, query, productID, warehouseID, tenantID)

	var s StockLevel
	err := row.Scan(&s.ID, &s.TenantID, &s.ProductID, &s.WarehouseID, &s.Quantity, &s.ReservedQty, &s.AvailableQty, &s.LastUpdated)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("stock level not found")
		}
		return nil, err
	}

	return &s, nil
}

// GetStockLevelsByProduct retrieves all stock levels for a product
func (r *PostgresRepository) GetStockLevelsByProduct(ctx context.Context, schemaName, tenantID, productID string) ([]StockLevel, error) {
	query := `SELECT id, tenant_id, product_id, warehouse_id, quantity, reserved_qty, available_qty, last_updated FROM stock_levels WHERE product_id = $1 AND tenant_id = $2`
	rows, err := r.queryInSchema(ctx, schemaName, query, productID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []StockLevel
	for rows.Next() {
		var s StockLevel
		if err := rows.Scan(&s.ID, &s.TenantID, &s.ProductID, &s.WarehouseID, &s.Quantity, &s.ReservedQty, &s.AvailableQty, &s.LastUpdated); err != nil {
			return nil, err
		}
		levels = append(levels, s)
	}

	return levels, nil
}

// UpsertStockLevel creates or updates a stock level
func (r *PostgresRepository) UpsertStockLevel(ctx context.Context, schemaName string, level *StockLevel) error {
	query := `
		INSERT INTO stock_levels (id, tenant_id, product_id, warehouse_id, quantity, reserved_qty, available_qty, last_updated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, product_id, warehouse_id) DO UPDATE SET
			quantity = EXCLUDED.quantity,
			reserved_qty = EXCLUDED.reserved_qty,
			available_qty = EXCLUDED.available_qty,
			last_updated = EXCLUDED.last_updated`

	return r.execInSchema(ctx, schemaName, query,
		level.ID, level.TenantID, level.ProductID, level.WarehouseID,
		level.Quantity, level.ReservedQty, level.AvailableQty, level.LastUpdated,
	)
}

// CreateMovement creates a new inventory movement
func (r *PostgresRepository) CreateMovement(ctx context.Context, schemaName string, movement *InventoryMovement) error {
	query := `
		INSERT INTO inventory_movements (
			id, tenant_id, product_id, warehouse_id, movement_type, quantity, unit_cost, total_cost,
			reference, to_warehouse_id, notes, movement_date, created_at, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	return r.execInSchema(ctx, schemaName, query,
		movement.ID, movement.TenantID, movement.ProductID, movement.WarehouseID,
		movement.MovementType, movement.Quantity, movement.UnitCost, movement.TotalCost,
		movement.Reference, nullIfEmpty(movement.ToWarehouseID), movement.Notes, movement.MovementDate,
		movement.CreatedAt, movement.CreatedBy,
	)
}

// ListMovements retrieves inventory movements for a product
func (r *PostgresRepository) ListMovements(ctx context.Context, schemaName, tenantID, productID string) ([]InventoryMovement, error) {
	query := `
		SELECT id, tenant_id, product_id, warehouse_id, movement_type, quantity, unit_cost, total_cost,
			COALESCE(reference, ''), to_warehouse_id, COALESCE(notes, ''), movement_date, created_at, COALESCE(created_by::text, '')
		FROM inventory_movements
		WHERE tenant_id = $1 AND product_id = $2
		ORDER BY movement_date DESC, created_at DESC`

	rows, err := r.queryInSchema(ctx, schemaName, query, tenantID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []InventoryMovement
	for rows.Next() {
		var m InventoryMovement
		var toWarehouseID *string
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ProductID, &m.WarehouseID, &m.MovementType,
			&m.Quantity, &m.UnitCost, &m.TotalCost, &m.Reference,
			&toWarehouseID, &m.Notes, &m.MovementDate, &m.CreatedAt, &m.CreatedBy,
		); err != nil {
			return nil, err
		}
		if toWarehouseID != nil {
			m.ToWarehouseID = *toWarehouseID
		}
		movements = append(movements, m)
	}

	return movements, nil
}

// nullIfEmpty returns nil if the string is empty
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// UpdateProductStock updates the current stock of a product
func (r *PostgresRepository) UpdateProductStock(ctx context.Context, schemaName, tenantID, productID string, newStock decimal.Decimal) error {
	query := `UPDATE products SET current_stock = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`
	return r.execInSchema(ctx, schemaName, query, newStock, productID, tenantID)
}
